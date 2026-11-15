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

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type SynthesizeEngine struct {
	logger          *slog.Logger
	registry        *config.Registry
	service         *workspace.Service
	turnExecutor    *TurnExecutor
	session         *ChatSession
	bundleAssembler *BundleAssembler
	broadcaster     *GraphBroadcaster
	runner          *SubAgentRunner
}

func NewSynthesizeEngine(
	logger *slog.Logger,
	registry *config.Registry,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	session *ChatSession,
) *SynthesizeEngine {
	var graphRepo thinkgraph.GraphRepository
	var workspaceRoot string
	if service != nil {
		graphRepo = service.Graph()
		workspaceRoot = service.WorkspaceRoot()
	}

	broadcaster := NewGraphBroadcasterFromService(logger, service)

	var assembler *BundleAssembler
	if session != nil && session.bundleAssembler != nil {
		assembler = session.bundleAssembler
	} else {
		assembler = NewBundleAssembler(logger, graphRepo, thinkgraph.NewContextProjecter(), registry, workspaceRoot)
	}

	runner := NewSubAgentRunner(logger, service, turnExecutor, assembler, broadcaster)

	return &SynthesizeEngine{
		logger:          logger,
		registry:        registry,
		service:         service,
		turnExecutor:    turnExecutor,
		session:         session,
		bundleAssembler: assembler,
		broadcaster:     broadcaster,
		runner:          runner,
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

	if err := e.broadcaster.AppendAndStream(ctx, sig.FlowAddress, promptNode, ui); err != nil {
		e.logger.ErrorContext(ctx, "failed to append synthesis prompt node", "error", err, "node_id", synthPromptID)
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Applying synthesized patches and verifying workspace...",
		})
	}

	// 2. Subgraph Worker Turn via SubAgentRunner: Generate, patch, verify, and deliver golden candidate
	synthResult := e.runner.ExecuteCandidate(ctx, CandidateExecutionRequest{
		ChatSession:    chatSession,
		FlowAddress:    sig.FlowAddress,
		Space:          space,
		CandidateID:    "synthesis-refine",
		BaseAgentRules: space.Config().WorkerSystemPrompt(),
		ContextDigest:  sig.OriginalContext,
		Instruction:    "[Synthesis Phase]: Combine the best approaches from the candidate branches:\n\n" + renderedInstruction,
		ParentNodeID:   synthPromptID,
		DraftMessage:   "Applied synthesized golden candidate",
		UI:             nil,
		Emitter:        emitter,
	})
	if synthResult.Err != nil {
		e.updateMainTimelineFlowCard(ctx, chatSession, sig.FlowAddress.GraphID.UUID, flows.FlowStatusFailed, "Synthesis generation failed: "+synthResult.Err.Error(), ui)
		return nil, fmt.Errorf("synthesis execution failed: %w", synthResult.Err)
	}

	patchApplied := synthResult.PatchApplied
	verified := synthResult.Passed
	verifyTrace := "Synthesis verification passed."
	if !verified {
		verifyTrace = synthResult.Trace
	}
	synthesizedDiff := synthResult.Diff

	// 3. Main Timeline Manager Review Turn: Summarize the flow outcome in plain Markdown
	var reviewPrompt strings.Builder
	reviewPrompt.WriteString("The parallel sub-agent flow and synthesis phase have concluded. Provide a concise, human-readable summary of what was accomplished, comparing the candidate approaches and explaining the final synthesized changes.\n")
	reviewPrompt.WriteString("Do NOT output raw JSON patches. Respond in clean Markdown for the user.\n\n")
	reviewPrompt.WriteString(fmt.Sprintf("### Goal Context\n%s\n\n", sig.OriginalContext))
	reviewPrompt.WriteString(fmt.Sprintf("### Synthesis Verification\n- **Verified:** %v\n- **Trace:** %s\n\n", verified, verifyTrace))
	if synthesizedDiff != "" {
		reviewPrompt.WriteString(fmt.Sprintf("### Applied Diff\n```diff\n%s\n```\n", synthesizedDiff))
	}

	managerBundle := e.bundleAssembler.AssembleWorkerBundle(
		ctx,
		chatSession.ProjectID,
		space,
		space.Config().ManagerSystemPrompt(),
		sig.OriginalContext,
		reviewPrompt.String(),
		nil,
	)
	managerBundle.Rules.FormattingConstraints = ""

	reviewNodeID := uuid.NewV7()
	if _, reviewErr := e.turnExecutor.Execute(ctx, chatSession, reviewNodeID, sig.ParentNodeID, true, space, managerBundle, tools.NewRegistry(), ui, emitter); reviewErr != nil {
		e.logger.ErrorContext(ctx, "failed executing Manager flow review turn", "error", reviewErr)
	}

	// 4. Final Resolution: Auto-accept only when verified + AutoAccept; otherwise always prompt user for [Accept]/[Reject]
	accepted := false
	resolutionReason := verifyTrace
	var userDecisionNodeContent string

	if patchApplied {
		if verified && sig.AutoAccept {
			accepted = true
			resolutionReason = "Auto-accepted by Refine strategy"
			if err := e.service.ResolveCandidate(ctx, chatSession, "synthesis-refine", true, resolutionReason); err != nil {
				e.logger.ErrorContext(ctx, "failed to auto-accept synthesized candidate", "error", err)
			}
		} else if ui != nil {
			var userReason string
			accepted, userReason = ui.ReviewCandidate("candidate/synthesis-refine")
			trimmedReason := strings.TrimSpace(userReason)

			if accepted {
				resolutionReason = trimmedReason
				if resolutionReason == "" {
					resolutionReason = "Accepted by user"
				}
				userDecisionNodeContent = fmt.Sprintf("accepted flow: %s", resolutionReason)
			} else {
				resolutionReason = trimmedReason
				if resolutionReason == "" {
					resolutionReason = "Rejected by user"
				}
				userDecisionNodeContent = fmt.Sprintf("rejected flow: %s", resolutionReason)
			}

			if err := e.service.ResolveCandidate(ctx, chatSession, "synthesis-refine", accepted, resolutionReason); err != nil {
				e.logger.ErrorContext(ctx, "failed to resolve synthesized candidate", "error", err)
			}
		}
	}

	// 5. Append formal EventCandidateResolution node to the subgraph after final decision
	statusStr := "REJECTED"
	if accepted {
		statusStr = "ACCEPTED"
	}
	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Synthesis Proposal %s. Reason: %s", statusStr, resolutionReason))
	resEvent.Edges = map[string]string{"parent_id": synthResult.GenNodeID.String()}
	resPayload := workspace.CandidateResolution{
		CandidateID: "candidate/synthesis-refine",
		Accepted:    accepted,
		Reason:      resolutionReason,
	}
	resBytes, marshalErr := json.Marshal(resPayload)
	if marshalErr != nil {
		e.logger.ErrorContext(ctx, "failed to marshal synthesis candidate resolution payload", "error", marshalErr)
	} else {
		resEvent.Data = resBytes
	}

	if err := e.broadcaster.AppendAndStream(ctx, sig.FlowAddress, resEvent, ui); err != nil {
		e.logger.ErrorContext(ctx, "failed to append candidate resolution event", "error", err)
	}

	// 6. In manual review mode, append user decision + reason as a prompt node on the Main Timeline
	// so ContextProjecter naturally collapses it with the user's next prompt.
	if userDecisionNodeContent != "" {
		decisionEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, userDecisionNodeContent)
		decisionEvent.Edges = map[string]string{"parent_id": reviewNodeID.String()}

		if err := e.broadcaster.AppendAndStream(ctx, chatSession, decisionEvent, ui); err != nil {
			e.logger.ErrorContext(ctx, "failed to append user decision node to main timeline", "error", err)
		}
	}

	flowStatus := flows.FlowStatusCompleted
	flowSummary := "Refined solution synthesized and accepted."
	if !patchApplied {
		flowStatus = flows.FlowStatusFailed
		flowSummary = "Synthesis failed: " + verifyTrace
	} else if accepted {
		flowStatus = flows.FlowStatusCompleted
		if !verified {
			flowSummary = "Refined solution accepted despite verification failure."
		}
	} else if !verified {
		flowStatus = flows.FlowStatusFailed
		flowSummary = "Synthesis failed: " + verifyTrace
	} else {
		flowStatus = flows.FlowStatusCompleted
		flowSummary = "Refined solution rejected: " + resolutionReason
	}

	e.updateMainTimelineFlowCard(ctx, chatSession, sig.FlowAddress.GraphID.UUID, flowStatus, flowSummary, ui)

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

	_ = e.broadcaster.MutateAndStream(ctx, chatSession, contentMut, flowCardID, nil)

	var currentState flows.FlowExecutionState
	if graph := e.service.Graph(); graph != nil {
		nodes, err := graph.GetNodes(ctx, chatSession, []string{flowCardID.String()})
		if err == nil && len(nodes) > 0 && len(nodes[0].MaterializedState) > 0 {
			if unmarshalErr := json.Unmarshal(nodes[0].MaterializedState, &currentState); unmarshalErr != nil {
				e.logger.WarnContext(ctx, "failed to unmarshal flow execution state for mutation", "error", unmarshalErr)
			}
		}
	}

	currentState.Status = status
	currentState.Summary = summary
	if status == flows.FlowStatusFailed {
		currentState.Error = summary
	}

	stateBytes, err := json.Marshal(currentState)
	if err != nil {
		e.logger.ErrorContext(ctx, "failed to marshal updated flow state", "error", err)
		return
	}

	stateMut := thinkgraph.StateMutation{State: stateBytes}
	stateMut.TargetID = flowCardID
	stateMut.Type = thinkgraph.MutationState
	stateMut.Timestamp = time.Now().UTC()

	_ = e.broadcaster.MutateAndStream(ctx, chatSession, stateMut, flowCardID, ui)
}
