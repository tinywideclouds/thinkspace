package session

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type SynthesizeEngine struct {
	logger           *slog.Logger
	registry         *config.Registry
	workspaceService *workspace.Service
	turnExecutor     *TurnExecutor
	session          *ChatSession
}

func NewSynthesizeEngine(
	logger *slog.Logger,
	registry *config.Registry,
	workspaceService *workspace.Service,
	turnExecutor *TurnExecutor,
	session *ChatSession,
) *SynthesizeEngine {
	return &SynthesizeEngine{
		logger:           logger,
		registry:         registry,
		workspaceService: workspaceService,
		turnExecutor:     turnExecutor,
		session:          session,
	}
}

func (e *SynthesizeEngine) Execute(ctx context.Context, address *thinkgraph.ChatAddress, turnID uuid.UUID, catalystID uuid.UUID, signal signals.EngineSignal, ui UserInterface, emitter telemetry.Emitter) (signals.EngineSignal, error) {
	sig, ok := signal.(*signals.SignalSynthesize)
	if !ok {
		return nil, fmt.Errorf("SynthesizeEngine expected SignalSynthesize, got %T", signal)
	}

	e.turnExecutor.emitter = emitter

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	e.session.mu.RUnlock()

	thinkState, ok := e.registry.GetThinkState(activeThinkState)
	if !ok {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	var prompt strings.Builder
	prompt.WriteString(sig.OriginalInstruction + "\n\n")

	if sig.OriginalContext != "" {
		prompt.WriteString("### Original Task Context\n")
		prompt.WriteString(sig.OriginalContext + "\n\n")
	}

	prompt.WriteString("### Candidate Results\n")
	for _, c := range sig.Candidates {
		status := "PASSED"
		if !c.Passed {
			status = "FAILED"
		}
		prompt.WriteString(fmt.Sprintf("\n--- Candidate Branch: %s (Status: %s) ---\n", c.CandidateID, status))
		if c.Instruction != "" {
			prompt.WriteString(fmt.Sprintf("**Sub-Task Instruction:** %s\n", c.Instruction))
		}
		if c.Trace != "" {
			prompt.WriteString(fmt.Sprintf("**Verification Error Trace:**\n```\n%s\n```\n", c.Trace))
		}
		if c.RawPayload != "" {
			prompt.WriteString(fmt.Sprintf("**Raw Sub-Agent Output:**\n```\n%s\n```\n", c.RawPayload))
		}
	}

	synthAddress := sig.FlowAddress
	if synthAddress == nil {
		return nil, fmt.Errorf("SynthesizeEngine received nil FlowAddress in signal")
	}

	projectWorkspaceDir := e.workspaceService.ProjectWorkspaceDir(synthAddress.ProjectID)
	rawMapbook, _ := thinkState.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	bundle := cognition.ThinkBundle{
		Rules:   cognition.SystemRules{Persona: thinkState.Config().ManagerSystemPrompt()},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: prompt.String(),
			},
		},
		Workbench: []cognition.WorkbenchItem{},
	}

	// 1. Write the synthesis prompt node and bridge it back to the Flow Options Node on the Main Timeline
	synthPromptEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, prompt.String())
	if synthPromptEvent.Edges == nil {
		synthPromptEvent.Edges = make(map[string]string)
	}
	synthPromptEvent.Edges["parent_id"] = sig.ParentNodeID.String()

	_ = e.workspaceService.Dispatcher().AppendEvent(ctx, synthAddress, synthPromptEvent)
	if ui != nil {
		ui.StreamGraphEvent(synthAddress, thinkgraph.Node{Event: synthPromptEvent})
	}

	synthCandidateID := fmt.Sprintf("%s-synthesis-%s", synthAddress.ChatID, uuid.NewV7().String()[:8])
	sandbox, err := e.workspaceService.SpawnSandbox(ctx, synthAddress.ChatID, synthCandidateID)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn synthesis sandbox: %w", err)
	}
	defer sandbox.TearDown(ctx)

	mux := &streamMultiplexer{ui: ui}
	taskResponseID := uuid.NewV7()

	// 2. Execute LLM. TurnExecutor natively writes the parent_id edge onto the Model node.
	// Pass anticipateResponse: false
	_, err = e.turnExecutor.Execute(ctx, synthAddress, taskResponseID, synthPromptEvent.ID, false, thinkState, bundle, tools.NewRegistry(), mux)
	if err != nil {
		return nil, fmt.Errorf("synthesis execution failed: %w", err)
	}

	lastPayload := mux.buffer.String()
	if patcher := thinkState.Patcher(); patcher != nil {
		if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
			e.logger.WarnContext(ctx, "failed to apply synthesis patch", "error", applyErr)
		}
	}

	_ = sandbox.ApplyDraft(ctx, "synthesis")

	passed := true
	var finalTrace string
	if verifier := thinkState.Verifier(); verifier != nil {
		if vErr := verifier.Verify(ctx, sandbox); vErr != nil {
			passed = false
			finalTrace = vErr.Error()
		}
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		return nil, fmt.Errorf("failed to deliver synthesis sandbox for review: %w", err)
	}

	diff, _ := e.workspaceService.ReadCandidate(ctx, address, synthCandidateID)

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "flow_complete",
		Timestamp: time.Now(),
		Message:   "Synthesis pipeline completed",
		Payload:   map[string]any{"candidate_id": synthCandidateID, "passed": passed, "trace": finalTrace, "diff": diff},
	})

	uiBranchName := fmt.Sprintf("candidate/%s", synthCandidateID)
	accepted := false
	if ui != nil {
		accepted = ui.ReviewCandidate(uiBranchName)
	}

	reason := "Accepted by user"
	if !accepted {
		reason = "Rejected by user"
	}

	_ = e.workspaceService.ResolveCandidate(ctx, address, synthCandidateID, accepted, reason)

	// 3. Write the Resolution node and link it to the Model Response
	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Proposal %s. Reason: %s", "ACCEPTED", reason))
	if !accepted {
		resEvent.Content = fmt.Sprintf("Proposal %s. Reason: %s", "REJECTED", reason)
	}
	if resEvent.Edges == nil {
		resEvent.Edges = make(map[string]string)
	}
	resEvent.Edges["parent_id"] = taskResponseID.String()

	resPayload := workspace.CandidateResolution{
		CandidateID: synthCandidateID,
		Accepted:    accepted,
		Reason:      reason,
	}
	resEvent.Data, _ = json.Marshal(resPayload)
	_ = e.workspaceService.Dispatcher().AppendEvent(ctx, synthAddress, resEvent)
	if ui != nil {
		ui.StreamGraphEvent(synthAddress, thinkgraph.Node{Event: resEvent})
	}

	return &signals.SignalTerminal{}, err
}
