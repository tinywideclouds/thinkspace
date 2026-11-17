package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type trackedCandidate struct {
	cand     flows.Candidate
	resolved bool
}

// FlowScope is a transferable, thread-safe lifecycle owner for a flow execution.
// It tracks all candidate branches produced across stages, updates the flow lifecycle
// state on the parent timeline, and guarantees every unresolved candidate branch is rejected on Finalize.
type FlowScope struct {
	mu          sync.Mutex
	logger      *slog.Logger
	service     *workspace.Service
	broadcaster *GraphBroadcaster
	chatAddress *thinkgraph.ChatAddress
	flowAddress *thinkgraph.ChatAddress
	flowNodeID  uuid.UUID

	order      []string
	candidates map[string]*trackedCandidate
}

func NewFlowScope(
	logger *slog.Logger,
	service *workspace.Service,
	broadcaster *GraphBroadcaster,
	chatAddress *thinkgraph.ChatAddress,
	flowAddress *thinkgraph.ChatAddress,
	flowNodeID uuid.UUID,
) *FlowScope {
	if broadcaster == nil {
		broadcaster = NewGraphBroadcasterFromService(logger, service)
	}
	return &FlowScope{
		logger:      logger,
		service:     service,
		broadcaster: broadcaster,
		chatAddress: chatAddress,
		flowAddress: flowAddress,
		flowNodeID:  flowNodeID,
		candidates:  make(map[string]*trackedCandidate),
	}
}

func normalizeCandidateID(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "candidate/")
}

func (s *FlowScope) RegisterCandidate(cand flows.Candidate) {
	id := normalizeCandidateID(cand.ID)
	if id == "" {
		return
	}
	cand.ID = id

	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.candidates[id]; ok {
		existing.cand = cand
		return
	}
	s.order = append(s.order, id)
	s.candidates[id] = &trackedCandidate{cand: cand, resolved: false}
}

func (s *FlowScope) Candidates() []flows.Candidate {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]flows.Candidate, 0, len(s.order))
	for _, id := range s.order {
		if tc, ok := s.candidates[id]; ok {
			result = append(result, tc.cand)
		}
	}
	return result
}

func (s *FlowScope) ResolveCandidate(ctx context.Context, candidateID string, accept bool, reason string) error {
	id := normalizeCandidateID(candidateID)
	if id == "" {
		return nil
	}

	s.mu.Lock()
	if tc, ok := s.candidates[id]; ok {
		if tc.resolved {
			s.mu.Unlock()
			return nil
		}
		tc.resolved = true
	} else {
		s.order = append(s.order, id)
		s.candidates[id] = &trackedCandidate{
			cand:     flows.Candidate{ID: id},
			resolved: true,
		}
	}
	s.mu.Unlock()

	if s.service == nil {
		return nil
	}
	return s.service.ResolveCandidate(ctx, s.chatAddress, id, accept, reason)
}

func (s *FlowScope) Finalize(ctx context.Context, fallbackReason string) error {
	s.mu.Lock()
	var unresolved []string
	for _, id := range s.order {
		if tc, ok := s.candidates[id]; ok && !tc.resolved {
			tc.resolved = true
			unresolved = append(unresolved, id)
		}
	}
	s.mu.Unlock()

	if s.service == nil || len(unresolved) == 0 {
		return nil
	}

	reason := strings.TrimSpace(fallbackReason)
	if reason == "" {
		reason = "Flow finalized"
	}

	var errs []error
	for _, id := range unresolved {
		if err := s.service.CleanupCandidate(ctx, s.chatAddress, id, reason); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (s *FlowScope) SetRunning(ctx context.Context, summary string, agents []flows.AgentRecord, streamer signals.GraphStreamer) error {
	return s.updateFlowState(ctx, flows.FlowStatusRunning, summary, "", agents, false, streamer)
}

func (s *FlowScope) Complete(ctx context.Context, summary string, agents []flows.AgentRecord, streamer signals.GraphStreamer) error {
	return s.updateFlowState(ctx, flows.FlowStatusCompleted, summary, "", agents, true, streamer)
}

func (s *FlowScope) Fail(ctx context.Context, summary string, errTrace string, agents []flows.AgentRecord, streamer signals.GraphStreamer) error {
	return s.updateFlowState(ctx, flows.FlowStatusFailed, summary, errTrace, agents, true, streamer)
}

func (s *FlowScope) updateFlowState(
	ctx context.Context,
	status flows.FlowStatus,
	summary string,
	errTrace string,
	agents []flows.AgentRecord,
	updateContent bool,
	streamer signals.GraphStreamer,
) error {
	if s.broadcaster == nil || s.chatAddress == nil || s.flowNodeID == uuid.Nil() {
		return nil
	}

	var currentState flows.FlowExecutionState
	if s.broadcaster.graph != nil {
		nodes, err := s.broadcaster.graph.GetNodes(ctx, s.chatAddress, []string{s.flowNodeID.String()})
		if err != nil {
			return fmt.Errorf("reading flow node %s: %w", s.flowNodeID, err)
		}
		if len(nodes) > 0 && len(nodes[0].MaterializedState) > 0 {
			if unmarshalErr := json.Unmarshal(nodes[0].MaterializedState, &currentState); unmarshalErr != nil {
				return fmt.Errorf("unmarshaling flow execution state: %w", unmarshalErr)
			}
		}
	}

	currentState.Status = status
	currentState.Summary = summary
	currentState.Error = errTrace
	if agents != nil {
		currentState.Agents = agents
	}

	stateBytes, err := json.Marshal(currentState)
	if err != nil {
		return fmt.Errorf("marshaling updated flow state: %w", err)
	}

	stateMut := thinkgraph.StateMutation{State: stateBytes}
	if updateContent && summary != "" {
		stateMut.Content = summary
	}
	stateMut.TargetID = s.flowNodeID
	stateMut.Type = thinkgraph.MutationState
	stateMut.Timestamp = time.Now().UTC()

	if err := s.broadcaster.MutateAndStream(ctx, s.chatAddress, stateMut, s.flowNodeID, streamer); err != nil {
		return fmt.Errorf("updating flow state: %w", err)
	}
	return nil
}
