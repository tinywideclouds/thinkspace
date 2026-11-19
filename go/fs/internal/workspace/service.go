package workspace

import (
	"bufio"
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
	}
}

func (s *Service) WorkspaceRoot() string {
	return s.workspaceRoot
}

func (s *Service) ProjectWorkspaceDir(projectID string) string {
	return filepath.Join(s.workspaceRoot, projectID, "workspace")
}

func (s *Service) Graph() thinkgraph.GraphRepository {
	if s.dispatcher != nil && s.dispatcher.GraphStore() != nil {
		return s.dispatcher.GraphStore().Repository()
	}
	return nil
}

// peekParentBranch natively reads the graph branch anchor from physical disk.
func (s *Service) peekParentBranch(projectID, chatID, branchName string) (string, error) {
	logsDir := filepath.Join(s.workspaceRoot, projectID, "chats", chatID, "ledger", "logs", branchName)
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("reading branch logs directory %s: %w", logsDir, err)
	}
	if len(entries) == 0 {
		return "", nil
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}
	if len(shards) == 0 {
		return "", nil
	}
	sort.Strings(shards)

	firstShardPath := filepath.Join(logsDir, shards[0])
	file, err := os.Open(firstShardPath)
	if err != nil {
		return "", fmt.Errorf("opening first shard %s: %w", firstShardPath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 10*1024*1024)
	if scanner.Scan() {
		var ev thinkgraph.Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			return "", fmt.Errorf("unmarshaling initial branch event in %s: %w", firstShardPath, err)
		}
		if ev.Type == thinkgraph.EventBranchAnchor {
			var anchor thinkgraph.BranchAnchor
			if err := json.Unmarshal(ev.Data, &anchor); err != nil {
				return "", fmt.Errorf("unmarshaling branch anchor data in %s: %w", firstShardPath, err)
			}
			return anchor.ParentBranch, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanning initial branch event in %s: %w", firstShardPath, err)
	}
	return "", nil
}

// SyncGraphContext is completely decoupled from the API layer's topology maps.
func (s *Service) SyncGraphContext(ctx context.Context, chat *thinkgraph.ChatAddress) error {
	if chat == nil {
		return fmt.Errorf("chat address is nil")
	}

	if s.Graph() == nil {
		ledgerPath := filepath.Join(s.workspaceRoot, chat.ProjectID, "chats", chat.ChatID, "ledger")
		if _, err := thinkgraph.LoadOrRebuildManifest(ledgerPath, chat.SubGraph.Name); err != nil {
			return fmt.Errorf("loading or rebuilding ledger manifest: %w", err)
		}
		return nil
	}

	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}

	var branchesToSync []string
	current := chat.SubGraph.Name

	if chat.SubGraph.GraphID.Prefix == thinkgraph.PrefixFlow || strings.HasPrefix(current, "flow-") {
		// DETACHABLE SUB-GRAPH: No topology walk required. Lazy-load isolated file.
		branchesToSync = append(branchesToSync, current)
	} else {
		// Graph-Native Ancestry Walk
		visited := make(map[string]bool)
		for current != "" {
			if visited[current] {
				break
			}
			visited[current] = true
			branchesToSync = append([]string{current}, branchesToSync...)

			parentBranch, err := s.peekParentBranch(chat.ProjectID, chat.ChatID, current)
			if err != nil {
				return fmt.Errorf("peeking parent branch for %s: %w", current, err)
			}
			current = parentBranch
		}
	}

	rehydrator := thinkgraph.NewRehydrator(s.dispatcher.GraphStore(), s.workspaceRoot)

	for _, branchName := range branchesToSync {
		branchAddr := *chat
		branchAddr.SubGraph.Name = branchName

		status, diskManifest, graphManifest, err := rehydrator.CheckSync(ctx, &branchAddr)
		if err != nil {
			return fmt.Errorf("sync check failed for %s: %w", branchName, err)
		}

		if status == thinkgraph.SyncStatusBehind {
			if s.logger != nil {
				s.logger.InfoContext(ctx, "Fast-forwarding graph index (JIT)", "subgraph", branchName)
			}
			if err := rehydrator.FastForward(ctx, &branchAddr, graphManifest, diskManifest); err != nil {
				return fmt.Errorf("fast-forward failed for %s: %w", branchName, err)
			}
		} else if status == thinkgraph.SyncStatusMismatch {
			if s.logger != nil {
				s.logger.WarnContext(ctx, "History divergence detected; rebuilding graph index (JIT)", "subgraph", branchName)
			}
			if err := rehydrator.Rebuild(ctx, &branchAddr); err != nil {
				return fmt.Errorf("rebuilding graph index for %s: %w", branchName, err)
			}
		}
	}

	return nil
}

func (s *Service) InitializeChat(ctx context.Context, chat *thinkgraph.ChatAddress) error {
	if s.engine == nil {
		return fmt.Errorf("chat versioner is not initialized")
	}

	chatDir := filepath.Join(s.workspaceRoot, chat.ProjectID, "chats", chat.ChatID)
	ledgerPath := filepath.Join(chatDir, "ledger")

	if s.logger != nil {
		s.logger.InfoContext(ctx, "initializing chat", slog.String("id", chat.ChatID))
	}

	if err := s.engine.InitChat(ctx, chat.ChatID); err != nil {
		return fmt.Errorf("initializing chat state: %w", err)
	}

	for _, dir := range []string{
		filepath.Join(chatDir, "docs"),
		filepath.Join(ledgerPath, "logs"),
		filepath.Join(ledgerPath, "digests"),
		filepath.Join(ledgerPath, "mutations"),
		filepath.Join(ledgerPath, "contexts"),
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating chat directory %s: %w", dir, err)
		}
	}

	// At boot, sync the active moving head (no topology walk needed just for boot)
	if err := s.SyncGraphContext(ctx, chat); err != nil {
		return fmt.Errorf("failed to sync main timeline graph at boot: %w", err)
	}

	return nil
}

func (s *Service) PreviewCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string) error {
	if s.engine == nil {
		return fmt.Errorf("chat versioner is not initialized")
	}
	return s.engine.PreviewCandidate(ctx, c.ChatID, candidateID)
}

func (s *Service) ResolveCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, accept bool, reason string) error {
	if s.engine == nil {
		return fmt.Errorf("chat versioner is not initialized")
	}

	if accept {
		if err := s.engine.Accept(ctx, c.ChatID, candidateID, reason); err != nil {
			return fmt.Errorf("accepting candidate: %w", err)
		}
	} else {
		if err := s.engine.Reject(ctx, c.ChatID, candidateID, reason); err != nil {
			return fmt.Errorf("rejecting candidate: %w", err)
		}
	}

	if err := s.AppendResolution(ctx, c, candidateID, accept, reason); err != nil {
		return fmt.Errorf("appending candidate resolution: %w", err)
	}

	return nil
}

// CleanupCandidate tags and deletes an unaccepted candidate branch in the VCS layer
// without appending additional resolution events to the conversation graph.
func (s *Service) CleanupCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, reason string) error {
	if s.engine == nil {
		return fmt.Errorf("chat versioner is not initialized")
	}
	if err := s.engine.Reject(ctx, c.ChatID, candidateID, reason); err != nil {
		return fmt.Errorf("cleaning up candidate %s: %w", candidateID, err)
	}
	return nil
}

func (s *Service) Checkpoint(ctx context.Context, c *thinkgraph.ChatAddress, reason string) (string, error) {
	if s.engine == nil {
		return "", fmt.Errorf("chat versioner is not initialized")
	}
	if s.logger != nil {
		s.logger.DebugContext(ctx, "checkpointing chat state", slog.String("chat_id", c.ChatID))
	}

	msg := fmt.Sprintf("chore(ledger): %s", reason)
	sha, err := s.engine.Snapshot(ctx, c.ChatID, msg)
	if err != nil {
		return "", fmt.Errorf("snapshotting checkpoint: %w", err)
	}

	return sha, nil
}

func (s *Service) AppendUserPrompt(ctx context.Context, c *thinkgraph.ChatAddress, content string) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	event := thinkgraph.NewEvent(thinkgraph.EventPrompt, content)
	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) AppendModelResponse(ctx context.Context, c *thinkgraph.ChatAddress, eventID uuid.UUID, content string, usage *telemetry.TokenUsage, parentID uuid.UUID) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	event := thinkgraph.NewEvent(thinkgraph.EventModel, content)
	event.ID = eventID

	if parentID != uuid.Nil() {
		if event.Edges == nil {
			event.Edges = make(map[string]string)
		}
		event.Edges["parent_id"] = parentID.String()
	}

	if usage != nil && usage.TotalTokens > 0 {
		event.Metadata = map[string]string{
			"prompt_tokens":    strconv.Itoa(usage.PromptTokens),
			"candidate_tokens": strconv.Itoa(usage.CandidateTokens),
			"total_tokens":     strconv.Itoa(usage.TotalTokens),
		}
	}

	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) AppendResolution(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string, accepted bool, reason string) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
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
	dataBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling candidate resolution payload: %w", err)
	}
	event.Data = dataBytes

	return s.dispatcher.AppendEvent(ctx, c, event)
}

func (s *Service) AppendBranchAnchor(ctx context.Context, address *thinkgraph.ChatAddress, parentBranch string, forkPoint uuid.UUID) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	anchorEvent := thinkgraph.NewEvent(thinkgraph.EventBranchAnchor, fmt.Sprintf("Branched from %s", parentBranch))

	anchorData := thinkgraph.BranchAnchor{
		ParentBranch: parentBranch,
		ForkPoint:    forkPoint,
	}

	dataBytes, err := json.Marshal(anchorData)
	if err != nil {
		return fmt.Errorf("failed to marshal branch anchor data: %w", err)
	}
	anchorEvent.Data = dataBytes

	return s.dispatcher.AppendEvent(ctx, address, anchorEvent)
}

func (s *Service) ReadCandidate(ctx context.Context, c *thinkgraph.ChatAddress, candidateID string) (string, error) {
	if s.engine == nil {
		return "", fmt.Errorf("chat versioner is not initialized")
	}
	if s.logger != nil {
		s.logger.DebugContext(ctx, "reading candidate contents", slog.String("candidate_id", candidateID))
	}

	diff, err := s.engine.ReadCandidateDiff(ctx, c.ChatID, candidateID)
	if err != nil {
		return "", fmt.Errorf("reading candidate diff: %w", err)
	}

	return diff, nil
}

func (s *Service) SpawnSandbox(ctx context.Context, chatID, candidateID string) (CandidateSandbox, error) {
	if s.engine == nil {
		return nil, fmt.Errorf("chat versioner is not initialized")
	}
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

	entries, err := os.ReadDir(previewsDir)
	if err != nil {
		return "", fmt.Errorf("reading previews directory for pruning: %w", err)
	}

	if len(entries) > 15 {
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				files = append(files, entry.Name())
			}
		}

		sort.Strings(files)

		for i := 0; i < len(files)-15; i++ {
			oldPreview := filepath.Join(previewsDir, files[i])
			if rmErr := os.Remove(oldPreview); rmErr != nil && s.logger != nil {
				s.logger.WarnContext(ctx, "failed to prune old preview file", "path", oldPreview, "error", rmErr)
			}
		}
	}

	return previewID, nil
}

func (s *Service) GetContextPreview(ctx context.Context, c *thinkgraph.ChatAddress, previewID string) ([]byte, error) {
	if previewID == "" || filepath.Base(previewID) != previewID {
		return nil, fmt.Errorf("invalid preview ID: %q", previewID)
	}
	path := filepath.Join(s.workspaceRoot, c.ProjectID, "chats", c.ChatID, "previews", fmt.Sprintf("%s.md", previewID))
	return os.ReadFile(path)
}

func (s *Service) AppendDigest(ctx context.Context, c *thinkgraph.ChatAddress, summary string, tags []string, isSticky bool, summarized []uuid.UUID) error {
	if s.dispatcher == nil {
		return fmt.Errorf("dispatcher is not initialized")
	}
	return s.dispatcher.SaveDigest(ctx, c, thinkgraph.DigestMeta{
		ID:               uuid.NewV7(),
		Summary:          summary,
		Tags:             tags,
		IsSticky:         isSticky,
		SummarizedEvents: summarized,
	})
}

func (s *Service) SaveMutation(ctx context.Context, c *thinkgraph.ChatAddress, mut thinkgraph.Mutator) error {
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

func (s *Service) SaveLLMContextDump(ctx context.Context, c *thinkgraph.ChatAddress, label string, content string) (string, error) {
	if c == nil || c.ProjectID == "" || c.ChatID == "" {
		return "", fmt.Errorf("invalid chat address for LLM context dump")
	}

	debugDir := filepath.Join(s.workspaceRoot, c.ProjectID, "chats", c.ChatID, "debug_llm")
	if err := os.MkdirAll(debugDir, 0755); err != nil {
		return "", fmt.Errorf("creating debug_llm directory: %w", err)
	}

	safeLabel := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, label)
	if safeLabel == "" {
		safeLabel = "turn"
	}

	filename := fmt.Sprintf("%s_%s.md", uuid.NewV7().String(), safeLabel)
	path := filepath.Join(debugDir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing LLM context dump: %w", err)
	}

	entries, err := os.ReadDir(debugDir)
	if err != nil {
		return path, fmt.Errorf("reading debug_llm directory for pruning: %w", err)
	}

	if len(entries) > 50 {
		var files []string
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				files = append(files, entry.Name())
			}
		}
		sort.Strings(files)
		for i := 0; i < len(files)-50; i++ {
			oldDump := filepath.Join(debugDir, files[i])
			if rmErr := os.Remove(oldDump); rmErr != nil && s.logger != nil {
				s.logger.WarnContext(ctx, "failed to prune old LLM context dump", "path", oldDump, "error", rmErr)
			}
		}
	}

	return path, nil
}
