package workspace

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Service orchestrates the workspace lifecycle using the StateEngine.
type Service struct {
	logger        *slog.Logger
	state         StateEngine
	workspaceRoot string
}

func NewService(logger *slog.Logger, state StateEngine, workspaceRoot string) *Service {
	return &Service{
		logger:        logger,
		state:         state,
		workspaceRoot: workspaceRoot,
	}
}

// StartThread initializes a new logical workspace and establishes its state.
func (s *Service) StartThread(ctx context.Context, id string) (*Thread, error) {
	threadDir := filepath.Join(s.workspaceRoot, "chats", id)
	ledgerPath := filepath.Join(threadDir, "conversation.jsonl")

	s.logger.InfoContext(ctx, "starting thread", slog.String("id", id))

	if err := s.state.InitThread(ctx, s.workspaceRoot, id); err != nil {
		return nil, fmt.Errorf("initializing thread state: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(threadDir, "docs"), 0755); err != nil {
		return nil, fmt.Errorf("creating thread directories: %w", err)
	}

	f, err := os.Create(ledgerPath)
	if err != nil {
		return nil, fmt.Errorf("creating ledger file: %w", err)
	}
	f.Close()

	return &Thread{
		ID:         id,
		Branch:     "chat/" + id,
		LedgerPath: ledgerPath,
		Dir:        threadDir,
	}, nil
}

// ProposeCandidate isolates a sandbox for tool outputs, writes the files securely,
// and saves the proposal into the state engine.
func (s *Service) ProposeCandidate(ctx context.Context, thread *Thread, toolName string, files map[string][]byte) (*Candidate, error) {
	now := time.Now().UTC()
	candidateIdentifier := fmt.Sprintf("%s-%d", toolName, now.Unix())

	s.logger.InfoContext(ctx, "proposing candidate",
		slog.String("thread_id", thread.ID),
		slog.String("proposal_id", candidateIdentifier),
	)

	commitHash, err := s.state.Propose(ctx, s.workspaceRoot, thread.ID, candidateIdentifier, files, toolName)
	if err != nil {
		return nil, fmt.Errorf("proposing candidate state: %w", err)
	}

	var fileList []string
	for relativeFilePath := range files {
		fileList = append(fileList, filepath.Clean(relativeFilePath))
	}

	if err := s.LogProposal(ctx, thread, candidateIdentifier, fileList); err != nil {
		s.logger.ErrorContext(ctx, "failed to log proposal to ledger", "error", err)
	}

	return &Candidate{
		ID:        candidateIdentifier,
		ThreadID:  thread.ID,
		Branch:    "candidate/" + candidateIdentifier,
		CommitSHA: commitHash,
		Status:    StatusPending,
		CreatedAt: now,
	}, nil
}

// ResolveCandidate handles the human or automated decision to accept or reject a proposal.
func (s *Service) ResolveCandidate(ctx context.Context, thread *Thread, candidate *Candidate, accept bool) error {
	reason := "User manually rejected via CLI"

	if accept {
		reason = "User manually accepted via CLI"
		if err := s.state.Accept(ctx, s.workspaceRoot, thread.ID, candidate.ID); err != nil {
			return fmt.Errorf("accepting candidate: %w", err)
		}
	} else {
		if err := s.state.Reject(ctx, s.workspaceRoot, thread.ID, candidate.ID); err != nil {
			return fmt.Errorf("rejecting candidate: %w", err)
		}
	}

	if err := s.LogResolution(ctx, thread, candidate.ID, accept, reason); err != nil {
		s.logger.ErrorContext(ctx, "failed to log resolution to ledger", "error", err)
	}

	return nil
}

// AppendEvent writes an interaction to the append-only ledger on disk.
func (s *Service) AppendEvent(ctx context.Context, thread *Thread, ev Event) error {
	b, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	f, err := os.OpenFile(thread.LedgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening ledger: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	return nil
}

// Checkpoint safely commits the current state of the Thread branch.
func (s *Service) Checkpoint(ctx context.Context, thread *Thread, reason string) (string, error) {
	s.logger.DebugContext(ctx, "checkpointing thread state", slog.String("thread_id", thread.ID))

	msg := fmt.Sprintf("chore(ledger): %s", reason)
	sha, err := s.state.Snapshot(ctx, s.workspaceRoot, thread.ID, msg)
	if err != nil {
		return "", fmt.Errorf("snapshotting checkpoint: %w", err)
	}

	return sha, nil
}

// LoadEvents parses the append-only ledger and returns the full event history.
func (s *Service) LoadEvents(ctx context.Context, thread *Thread) ([]Event, error) {
	var events []Event

	file, err := os.Open(thread.LedgerPath)
	if err != nil {
		return nil, fmt.Errorf("opening ledger: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			s.logger.WarnContext(ctx, "skipping malformed ledger entry", slog.String("error", err.Error()))
			continue
		}
		events = append(events, ev)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning ledger: %w", err)
	}

	return events, nil
}

// LogUserPrompt creates and appends a user prompt event to the ledger.
func (s *Service) LogUserPrompt(ctx context.Context, thread *Thread, content string) error {
	ev := Event{
		ID:        fmt.Sprintf("usr-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Type:      EventPrompt,
		Content:   content,
	}
	return s.AppendEvent(ctx, thread, ev)
}

// LogModelResponse creates and appends a model output event to the ledger.
func (s *Service) LogModelResponse(ctx context.Context, thread *Thread, content string) error {
	ev := Event{
		ID:        fmt.Sprintf("mdl-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Type:      EventModel,
		Content:   content,
	}
	return s.AppendEvent(ctx, thread, ev)
}

// LogProposal records the generation of a candidate.
func (s *Service) LogProposal(ctx context.Context, thread *Thread, candidateID string, files []string) error {
	ev := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Type:      EventCandidate,
		Content:   fmt.Sprintf("Proposed files: %s", strings.Join(files, ", ")),
		Metadata: map[string]string{
			"proposal_uid": candidateID,
			"files":        strings.Join(files, ","),
		},
	}
	return s.AppendEvent(ctx, thread, ev)
}

// LogResolution records the outcome of a candidate.
func (s *Service) LogResolution(ctx context.Context, thread *Thread, candidateID string, accepted bool, reason string) error {
	status := "REJECTED"
	if accepted {
		status = "ACCEPTED"
	}
	ev := Event{
		ID:        fmt.Sprintf("evt-%d", time.Now().UnixNano()),
		Timestamp: time.Now().UTC(),
		Type:      EventResolution,
		Content:   fmt.Sprintf("Proposal %s %s. Reason: %s", candidateID, status, reason),
		Metadata: map[string]string{
			"proposal_uid": candidateID,
			"status":       status,
			"reason":       reason,
		},
	}
	return s.AppendEvent(ctx, thread, ev)
}
