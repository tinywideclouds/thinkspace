package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type SynthesizeEngine struct {
	logger       *slog.Logger
	registry     *config.Registry
	service      *workspace.Service
	turnExecutor *TurnExecutor
	session      *ChatSession
}

func NewSynthesizeEngine(
	logger *slog.Logger,
	registry *config.Registry,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	session *ChatSession,
) *SynthesizeEngine {
	return &SynthesizeEngine{
		logger:       logger,
		registry:     registry,
		service:      service,
		turnExecutor: turnExecutor,
		session:      session,
	}
}

type refineBranchView struct {
	CandidateID string
	Passed      bool
	Trace       string
	Diff        string
}

func (e *SynthesizeEngine) Execute(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	turnID uuid.UUID,
	catalystID uuid.UUID,
	signal signals.EngineSignal,
	ui UserInterface,
	emitter telemetry.Emitter,
) (signals.EngineSignal, error) {
	sig, ok := signal.(*signals.SignalSynthesize)
	if !ok {
		return nil, fmt.Errorf("SynthesizeEngine expected SignalSynthesize, got %T", signal)
	}

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	e.session.mu.RUnlock()

	space, exists := e.registry.GetThinkState(activeThinkState)
	if !exists {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Synthesizing refined solution from %d candidates...", len(sig.Candidates)),
		})
	}

	// 1. Render refine_prompt template with candidate diffs and verification traces
	var branchViews []refineBranchView
	for _, cand := range sig.Candidates {
		branchName := strings.TrimPrefix(cand.CandidateID, "candidate/")
		diff, diffErr := e.service.ReadCandidate(ctx, chatSession, branchName)
		if diffErr != nil {
			e.logger.WarnContext(ctx, "failed reading candidate diff for synthesis", "candidate", branchName, "error", diffErr)
			diff = cand.RawPayload
		}
		branchViews = append(branchViews, refineBranchView{
			CandidateID: cand.CandidateID,
			Passed:      cand.Passed,
			Trace:       cand.Trace,
			Diff:        diff,
		})
	}

	renderedInstruction := sig.OriginalInstruction
	if tmpl, parseErr := template.New("refine").Parse(sig.OriginalInstruction); parseErr == nil {
		var buf bytes.Buffer
		if execErr := tmpl.Execute(&buf, map[string]any{"Branches": branchViews}); execErr == nil {
			renderedInstruction = buf.String()
		} else {
			e.logger.WarnContext(ctx, "failed executing refine prompt template", "error", execErr)
		}
	} else {
		e.logger.WarnContext(ctx, "failed parsing refine prompt template", "error", parseErr)
	}

	synthPromptID := uuid.NewV7()
	promptNode := thinkgraph.NewEvent(thinkgraph.EventPrompt, "[Synthesis Phase]: "+renderedInstruction)
	promptNode.ID = synthPromptID
	promptNode.Edges = map[string]string{"parent_id": sig.ParentNodeID.String()}

	if err := e.service.Dispatcher().AppendEvent(ctx, sig.FlowAddress, promptNode); err != nil {
		e.logger.ErrorContext(ctx, "failed to append synthesis prompt node", "error", err, "node_id", synthPromptID)
	}

	if ui != nil {
		ui.StreamGraphEvent(sig.FlowAddress, thinkgraph.Node{Event: promptNode})
	}

	sandbox, err := e.service.SpawnSandbox(ctx, chatSession.ChatID, "synthesis-refine")
	if err != nil {
		return nil, fmt.Errorf("failed to spawn synthesis sandbox: %w", err)
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil {
			e.logger.WarnContext(ctx, "failed to tear down synthesis sandbox", "error", tdErr)
		}
	}()

	projectWorkspaceDir := e.service.ProjectWorkspaceDir(chatSession.ProjectID)
	rawMapbook, err := space.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
	if err != nil {
		e.logger.WarnContext(ctx, "failed to generate mapbook layers for synthesis", "error", err)
	}

	stringMaps := make(map[string]string)
	for k, v := range rawMapbook {
		stringMaps[string(k)] = v
	}

	formattingConstraints := ""
	if patcher := space.Patcher(); patcher != nil {
		formattingConstraints = patcher.SystemInstructions()
	}

	// 2. Subgraph Worker Turn: Generate and apply the synthesized JSON patch
	workerBundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona:               space.Config().WorkerSystemPrompt(),
			FormattingConstraints: formattingConstraints,
		},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			LongTermMemory: []string{sig.OriginalContext},
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: "[Synthesis Phase]: Combine the best approaches from the candidate branches:\n\n" + renderedInstruction,
			},
		},
	}

	silentMux := &streamMultiplexer{ui: nil}
	e.turnExecutor.emitter = emitter

	responseID := uuid.NewV7()
	_, err = e.turnExecutor.Execute(ctx, sig.FlowAddress, responseID, synthPromptID, false, space, workerBundle, tools.NewRegistry(), silentMux)
	if err != nil {
		e.updateMainTimelineFlowCard(ctx, chatSession, sig.FlowAddress.GraphID.UUID, flows.FlowStatusFailed, "Synthesis generation failed: "+err.Error(), ui)
		return nil, fmt.Errorf("synthesis execution failed: %w", err)
	}

	lastPayload := silentMux.buffer.String()
	finalTrace := "Synthesis successful."
	passed := true

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Applying synthesized patches and verifying workspace...",
		})
	}

	if patcher := space.Patcher(); patcher != nil {
		if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
			passed = false
			finalTrace = fmt.Sprintf("Failed to apply synthesis patch: %v", applyErr)
			e.logger.WarnContext(ctx, "failed to apply synthesis patch", "error", applyErr)
		}
	}

	if passed {
		if verifier := space.Verifier(); verifier != nil {
			if verifyErr := verifier.Verify(ctx, sandbox); verifyErr != nil {
				passed = false
				finalTrace = fmt.Sprintf("Verification failed: %v", verifyErr)
			}
		}
	}

	statusStr := "REJECTED"
	synthesizedDiff := ""
	if passed {
		statusStr = "ACCEPTED"
		if err := sandbox.ApplyDraft(ctx, "Applied synthesized golden candidate"); err != nil {
			e.logger.ErrorContext(ctx, "failed to apply draft for synthesized candidate", "error", err)
		}
		if err := sandbox.DeliverForReview(ctx); err != nil {
			e.logger.ErrorContext(ctx, "failed to deliver synthesized candidate for review", "error", err)
		}
		if diff, diffErr := e.service.ReadCandidate(ctx, chatSession, "synthesis-refine"); diffErr != nil {
			e.logger.WarnContext(ctx, "failed reading synthesized candidate diff", "error", diffErr)
		} else {
			synthesizedDiff = diff
		}
		if err := e.service.ResolveCandidate(ctx, chatSession, "synthesis-refine", true, "Auto-accepted by Refine strategy"); err != nil {
			e.logger.ErrorContext(ctx, "failed to resolve synthesized candidate", "error", err)
		}
	}

	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Synthesis Proposal %s. Reason: %s", statusStr, finalTrace))
	resEvent.Edges = map[string]string{"parent_id": responseID.String()}
	resPayload := workspace.CandidateResolution{
		CandidateID: "candidate/synthesis-refine",
		Accepted:    passed,
		Reason:      finalTrace,
	}
	resBytes, marshalErr := json.Marshal(resPayload)
	if marshalErr != nil {
		e.logger.ErrorContext(ctx, "failed to marshal synthesis candidate resolution payload", "error", marshalErr)
	} else {
		resEvent.Data = resBytes
	}

	if err := e.service.Dispatcher().AppendEvent(ctx, sig.FlowAddress, resEvent); err != nil {
		e.logger.ErrorContext(ctx, "failed to append candidate resolution event", "error", err)
	}

	if ui != nil {
		ui.StreamGraphEvent(sig.FlowAddress, thinkgraph.Node{Event: resEvent})
	}

	flowStatus := flows.FlowStatusCompleted
	flowSummary := "Refined solution synthesized and applied."
	if !passed {
		flowStatus = flows.FlowStatusFailed
		flowSummary = "Synthesis failed: " + finalTrace
	}

	e.updateMainTimelineFlowCard(ctx, chatSession, sig.FlowAddress.GraphID.UUID, flowStatus, flowSummary, ui)

	// 3. Main Timeline Manager Review Turn: Summarize the flow outcome in plain Markdown (no patcher constraints)
	var reviewPrompt strings.Builder
	reviewPrompt.WriteString("The parallel sub-agent flow and synthesis phase have concluded. Provide a concise, human-readable summary of what was accomplished, comparing the candidate approaches and explaining the final synthesized changes.\n")
	reviewPrompt.WriteString("Do NOT output raw JSON patches. Respond in clean Markdown for the user.\n\n")
	reviewPrompt.WriteString(fmt.Sprintf("### Goal Context\n%s\n\n", sig.OriginalContext))
	reviewPrompt.WriteString(fmt.Sprintf("### Synthesis Outcome\n- **Status:** %s\n- **Verification:** %s\n\n", statusStr, finalTrace))
	if synthesizedDiff != "" {
		reviewPrompt.WriteString(fmt.Sprintf("### Applied Diff\n```diff\n%s\n```\n", synthesizedDiff))
	}

	managerBundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: space.Config().ManagerSystemPrompt(),
		},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			LongTermMemory: []string{sig.OriginalContext},
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: reviewPrompt.String(),
			},
		},
	}

	reviewNodeID := uuid.NewV7()
	if _, reviewErr := e.turnExecutor.Execute(ctx, chatSession, reviewNodeID, sig.ParentNodeID, true, space, managerBundle, tools.NewRegistry(), ui); reviewErr != nil {
		e.logger.ErrorContext(ctx, "failed executing Manager flow review turn", "error", reviewErr)
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_complete",
			Timestamp: time.Now(),
			Message:   flowSummary,
		})
	}

	return &signals.SignalTerminal{}, nil
}

func (e *SynthesizeEngine) updateMainTimelineFlowCard(
	ctx context.Context,
	chatSession *thinkgraph.ChatAddress,
	flowCardID uuid.UUID,
	status flows.FlowStatus,
	summary string,
	ui UserInterface,
) {
	contentMut := thinkgraph.ContentMutation{Content: summary}
	contentMut.TargetID = flowCardID
	contentMut.Type = thinkgraph.MutationContent
	contentMut.Timestamp = time.Now().UTC()

	if err := e.service.Dispatcher().SaveMutation(ctx, chatSession, contentMut); err != nil {
		e.logger.ErrorContext(ctx, "failed to save content mutation for flow card", "error", err, "flow_card_id", flowCardID)
	}

	var currentState flows.FlowExecutionState
	nodes, err := e.service.Graph().GetNodes(ctx, chatSession, []string{flowCardID.String()})
	if err == nil && len(nodes) > 0 && len(nodes[0].MaterializedState) > 0 {
		if unmarshalErr := json.Unmarshal(nodes[0].MaterializedState, &currentState); unmarshalErr != nil {
			e.logger.WarnContext(ctx, "failed to unmarshal flow execution state for mutation", "error", unmarshalErr)
		}
	}

	currentState.Status = status
	currentState.Summary = summary

	stateBytes, err := json.Marshal(currentState)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed to marshal updated flow state", "error", err)
		return
	}

	stateMut := thinkgraph.StateMutation{State: stateBytes}
	stateMut.TargetID = flowCardID
	stateMut.Type = thinkgraph.MutationState
	stateMut.Timestamp = time.Now().UTC()

	if err := e.service.Dispatcher().SaveMutation(ctx, chatSession, stateMut); err != nil {
		e.logger.ErrorContext(ctx, "failed to save state mutation for flow card", "error", err, "flow_card_id", flowCardID)
	}

	if ui != nil && len(nodes) > 0 {
		updatedNode := nodes[0]
		updatedNode.Content = summary
		updatedNode.MaterializedState = stateBytes
		ui.StreamGraphEvent(chatSession, updatedNode)
	}
}
