package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

var ErrGraphMismatch = errors.New("physical ledger mismatch detected (Git history was altered). The fast-graph index must be rebuilt")

type Service struct {
	logger        *slog.Logger
	engine        ChatVersioner
	workspaceRoot string
	dispatcher    *thinkgraph.Dispatcher
	emitter       telemetry.Emitter
}

func NewService(
	logger *slog.Logger,
	engine ChatVersioner,
	workspaceRoot string,
	dispatcher *thinkgraph.Dispatcher,
	emitter telemetry.Emitter,
) *Service {
	return &Service{
		logger:        logger,
		engine:        engine,
		workspaceRoot: workspaceRoot,
		dispatcher:    dispatcher,
		emitter:       emitter,
	}
}

func (s *Service) WorkspaceRoot() string {
	return s.workspaceRoot
}

func (s *Service) Graph() thinkgraph.GraphRepository {
	if s.dispatcher != nil && s.dispatcher.GraphStore() != nil {
		return s.dispatcher.GraphStore().Repository()
	}
	return nil
}

func (s *Service) InitializeChat(ctx context.Context, spaceID, chatID string) (*thinkgraph.ChatAddress, error) {
	chat := &thinkgraph.ChatAddress{
		ChatID:    chatID,
		ProjectID: spaceID,
		SubGraph: thinkgraph.SubGraph{
			GraphID: thinkgraph.NewGraphID(thinkgraph.PrefixChat),
			Name:    "main",
		},
	}

	chatDir := filepath.Join(s.workspaceRoot, spaceID, "chats", chatID)
	ledgerPath := filepath.Join(chatDir, "ledger")

	s.logger.InfoContext(ctx, "initializing chat", slog.String("id", chatID))

	if err := s.engine.InitChat(ctx, chatID); err != nil {
		return nil, fmt.Errorf("initializing chat state: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(chatDir, "docs"), 0755); err != nil {
		return nil, fmt.Errorf("creating chat directories: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(ledgerPath, "logs"), 0755); err != nil {
		return nil, fmt.Errorf("creating ledger logs dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(ledgerPath, "digests"), 0755); err != nil {
		return nil, fmt.Errorf("creating ledger digests dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(ledgerPath, "mutations"), 0755); err != nil {
		return nil, fmt.Errorf("creating ledger mutations dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(ledgerPath, "contexts"), 0755); err != nil {
		return nil, fmt.Errorf("creating ledger contexts dir: %w", err)
	}

	if s.Graph() != nil {
		rehydrator := thinkgraph.NewRehydrator(s.dispatcher.GraphStore(), s.workspaceRoot)
		status, diskManifest, graphManifest, err := rehydrator.CheckSync(ctx, chat)
		if err != nil {
			return nil, fmt.Errorf("sync check failed: %w", err)
		}

		if status == thinkgraph.SyncStatusBehind {
			s.logger.InfoContext(ctx, "Fast-forwarding graph index", "target_events", diskManifest.TotalEvents)
			if err := rehydrator.FastForward(ctx, chat, graphManifest, diskManifest); err != nil {
				return nil, fmt.Errorf("fast-forward failed: %w", err)
			}
		} else if status == thinkgraph.SyncStatusMismatch {
			s.logger.WarnContext(ctx, "History divergence detected between BadgerDB and disk; rebuilding graph index from ledger", "chat_id", chatID)
			if err := rehydrator.Rebuild(ctx, chat); err != nil {
				s.logger.ErrorContext(ctx, "failed rebuilding graph index from ledger", "chat_id", chatID, "error", err)
				return nil, fmt.Errorf("rebuilding graph index: %w", err)
			}
			s.logger.InfoContext(ctx, "successfully rebuilt graph index from ledger", "chat_id", chatID)
		}
	} else {
		_, _ = thinkgraph.LoadOrRebuildManifest(ledgerPath, "main")
	}

	return chat, nil
}

func (s *Service) PreviewCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string) error {
	return s.engine.PreviewCandidate(ctx, c.ChatID, candidateID)
}

func (s *Service) ResolveCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, accept bool, reason string) error {
	if accept {
		if err := s.engine.Accept(ctx, c.ChatID, candidateID, reason); err != nil {
			return fmt.Errorf("accepting candidate: %w", err)
		}
	} else {
		if err := s.engine.Reject(ctx, c.ChatID, candidateID, reason); err != nil {
			return fmt.Errorf("rejecting candidate: %w", err)
		}
	}

	if err := s.LogResolution(ctx, c, candidateID, accept, reason); err != nil {
		s.logger.ErrorContext(ctx, "failed to log resolution to ledger", "error", err)
	}

	return nil
}

func (s *Service) ProposeCandidate(ctx context.Context, c *thinkgraph.ChatAddress, toolName string, files map[string][]byte) (*Candidate, error) {
	now := time.Now().UTC()
	candidateID := fmt.Sprintf("%s-%d", toolName, now.Unix())

	s.logger.InfoContext(ctx, "proposing candidate", slog.String("candidate_id", candidateID))

	sandbox, err := s.engine.SpawnCandidateSandbox(ctx, c.ChatID, candidateID)
	if err != nil {
		return nil, fmt.Errorf("spawning sandbox: %w", err)
	}
	defer sandbox.TearDown(ctx)

	var fileList []string
	for relativePath, content := range files {
		cleanPath := filepath.Clean(relativePath)
		if err := sandbox.WriteFile(ctx, filepath.Join("chats", c.ChatID, "docs", cleanPath), content); err != nil {
			return nil, fmt.Errorf("writing file %s to sandbox: %w", cleanPath, err)
		}
		fileList = append(fileList, cleanPath)
	}

	commitMessage := fmt.Sprintf("auto(candidate): generated by %s", toolName)
	if err := sandbox.ApplyDraft(ctx, commitMessage); err != nil {
		return nil, fmt.Errorf("applying sandbox draft: %w", err)
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		return nil, fmt.Errorf("delivering sandbox for review: %w", err)
	}

	if err := s.engine.PreviewCandidate(ctx, c.ChatID, candidateID); err != nil {
		return nil, fmt.Errorf("previewing candidate: %w", err)
	}

	description := fmt.Sprintf("Proposed files: %s", strings.Join(fileList, ", "))
	if err := s.LogProposal(ctx, c, candidateID, description, nil, fileList); err != nil {
		s.logger.ErrorContext(ctx, "failed to log proposal", "error", err)
	}

	return &Candidate{
		ID:        candidateID,
		ThreadID:  c.ChatID,
		Branch:    "candidate/" + candidateID,
		CommitSHA: "head",
		Status:    StatusPending,
		CreatedAt: now,
	}, nil
}

func (s *Service) Checkpoint(ctx context.Context, c *thinkgraph.ChatAddress, reason string) (string, error) {
	s.logger.DebugContext(ctx, "checkpointing chat state", slog.String("chat_id", c.ChatID))

	msg := fmt.Sprintf("chore(ledger): %s", reason)
	sha, err := s.engine.Snapshot(ctx, c.ChatID, msg)
	if err != nil {
		return "", fmt.Errorf("snapshotting checkpoint: %w", err)
	}

	return sha, nil
}

func (s *Service) LogUserPrompt(ctx context.Context, c *thinkgraph.ChatAddress, content string) error {
	event := thinkgraph.NewEvent(thinkgraph.EventPrompt, content)
	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) LogModelResponse(ctx context.Context, c *thinkgraph.ChatAddress, content string, usage *telemetry.TokenUsage) error {
	event := thinkgraph.NewEvent(thinkgraph.EventModel, content)

	if usage != nil && usage.TotalTokens > 0 {
		event.Metadata = map[string]string{
			"prompt_tokens":    strconv.Itoa(usage.PromptTokens),
			"candidate_tokens": strconv.Itoa(usage.CandidateTokens),
			"total_tokens":     strconv.Itoa(usage.TotalTokens),
		}
	}

	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) LogProposal(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, description string, tags []string, fileList []string) error {
	event := thinkgraph.NewEvent(EventCandidateProposal, description)
	event.Tags = tags

	payload := CandidateProposal{
		CandidateID: candidateID,
		Description: description,
		Files:       fileList,
	}
	event.Data, _ = json.Marshal(payload)

	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) LogResolution(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, accepted bool, reason string) error {
	status := "REJECTED"
	if accepted {
		status = "ACCEPTED"
	}

	event := thinkgraph.NewEvent(EventCandidateResolution, fmt.Sprintf("Proposal %s %s. Reason: %s", candidateID, status, reason))

	payload := CandidateResolution{
		CandidateID: candidateID,
		Accepted:    accepted,
		Reason:      reason,
	}
	event.Data, _ = json.Marshal(payload)

	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) ReadCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string) (string, error) {
	s.logger.DebugContext(ctx, "reading candidate contents", slog.String("candidate_id", candidateID))

	diff, err := s.engine.ReadCandidateDiff(ctx, c.ChatID, candidateID)
	if err != nil {
		return "", fmt.Errorf("reading candidate diff: %w", err)
	}

	return diff, nil
}

func (s *Service) SpawnSandbox(ctx context.Context, chatID, candidateID string) (CandidateSandbox, error) {
	return s.engine.SpawnCandidateSandbox(ctx, chatID, candidateID)
}

func (s *Service) SaveContextPreview(ctx context.Context, c *thinkgraph.ChatAddress, content string) (string, error) {
	id := uuid.NewV7()
	previewID := id.String()
	previewsDir := filepath.Join(s.workspaceRoot, c.ProjectID, "chats", c.ChatID, "previews")

	if err := os.MkdirAll(previewsDir, 0755); err != nil {
		return "", fmt.Errorf("creating previews directory: %w", err)
	}

	path := filepath.Join(previewsDir, fmt.Sprintf("%s.md", previewID))
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing preview file: %w", err)
	}

	if entries, err := os.ReadDir(previewsDir); err == nil && len(entries) > 15 {
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				files = append(files, entry.Name())
			}
		}

		sort.Strings(files)

		for i := 0; i < len(files)-15; i++ {
			_ = os.Remove(filepath.Join(previewsDir, files[i]))
		}
	}

	return previewID, nil
}

func (s *Service) GetContextPreview(ctx context.Context, c *thinkgraph.ChatAddress, previewID string) ([]byte, error) {
	path := filepath.Join(s.workspaceRoot, c.ProjectID, "chats", c.ChatID, "previews", fmt.Sprintf("%s.md", previewID))
	return os.ReadFile(path)
}

func (s *Service) LogDigest(ctx context.Context, c *thinkgraph.ChatAddress, summary string, tags []string, isSticky bool, summarized []uuid.UUID) error {
	event := thinkgraph.NewEvent(thinkgraph.EventDigest, summary)
	event.Tags = tags

	payload := thinkgraph.Digest{
		Summary:          summary,
		Promoted:         isSticky,
		SummarizedEvents: summarized,
	}
	event.Data, _ = json.Marshal(payload)

	return s.dispatcher.SaveDigest(ctx, c, thinkgraph.DigestMeta{
		ID:               event.ID,
		Summary:          summary,
		Tags:             tags,
		Promoted:         isSticky,
		SummarizedEvents: summarized,
	})
}

func (s *Service) SaveMutation(ctx context.Context, c *thinkgraph.ChatAddress, mut thinkgraph.MutationEvent) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	return s.dispatcher.SaveMutation(ctx, c, mut)
}

func (s *Service) SaveSpaceContext(ctx context.Context, c *thinkgraph.ChatAddress, sc thinkgraph.SpaceContext) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	return s.dispatcher.SaveSpaceContext(ctx, c, sc)
}

func (s *Service) Dispatcher() *thinkgraph.Dispatcher {
	return s.dispatcher
}
