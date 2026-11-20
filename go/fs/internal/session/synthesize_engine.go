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

type synthesisReviewView struct {
	CandidateID string
	Passed      bool
	Trace       string
	Diff        string
	Reasoning   string
}

func (e *SynthesizeEngine) loadFanOutConfig() (config.FanOutFlowConfig, error) {
	if e.registry == nil {
		return config.FanOutFlowConfig{}, fmt.Errorf("config registry is nil; cannot load fanout flow config")
	}
	fanoutCfg, err := e.registry.GetFanOutConfig()
	if err != nil {
		return config.FanOutFlowConfig{}, fmt.Errorf("loading fanout flow config: %w", err)
	}
	return fanoutCfg, nil
}

func (e *SynthesizeEngine) appendSubgraphResolution(
	ctx context.Context,
	flowAddress *thinkgraph.ChatAddress,
	parentNodeID uuid.UUID,
	candidateID string,
	accepted bool,
	reason string,
	ui UserInterface,
) error {
	statusStr := "REJECTED"
	if accepted {
		statusStr = "ACCEPTED"
	}
	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Proposal %s. Reason: %s", statusStr, reason))
	resEvent.Metadata["agent_id"] = "synthesis"
	if resEvent.Edges == nil {
		resEvent.Edges = make(map[string]string)
	}
	if parentNodeID != uuid.Nil() {
		resEvent.Edges["parent_id"] = parentNodeID.String()
	}

	resPayload := workspace.CandidateResolution{
		CandidateID: candidateID,
		Accepted:    accepted,
		Reason:      reason,
	}
	resBytes, err := json.Marshal(resPayload)
	if err != nil {
		return fmt.Errorf("marshaling synthesis resolution payload: %w", err)
	}
	resEvent.Data = resBytes

	if err := e.broadcaster.AppendAndStream(ctx, flowAddress, resEvent, ui); err != nil {
		return fmt.Errorf("appending synthesis candidate resolution event: %w", err)
	}
	return nil
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

	fanoutCfg, err := e.loadFanOutConfig()
	if err != nil {
		return nil, err
	}
	synthesizePrompt := strings.TrimSpace(fanoutCfg.SynthesizePrompt)
	synthesisReviewPromptTmpl := strings.TrimSpace(fanoutCfg.SynthesisReviewPrompt)

	scope := sig.Flow
	if scope == nil {
		var flowNodeID uuid.UUID
		if sig.FlowAddress != nil {
			flowNodeID = sig.FlowAddress.GraphID.UUID
		}
		localScope := NewFlowScope(e.logger, e.service, e.broadcaster, chatSession, sig.FlowAddress, flowNodeID)
		for _, c := range sig.Candidates {
			localScope.RegisterCandidate(c)
		}
		scope = localScope
	}

	handedOff := false
	defer func() {
		if !handedOff {
			if finalizeErr := scope.Finalize(ctx, "Superseded by synthesis"); finalizeErr != nil && e.logger != nil {
				e.logger.ErrorContext(ctx, "failed to finalize flow scope after synthesis", "error", finalizeErr)
			}
		}
	}()

	e.session.mu.RLock()
	activeThinkState := e.session.Meta.ActiveThinkState
	e.session.mu.RUnlock()

	space, exists := e.registry.GetThinkState(activeThinkState)
	if !exists {
		return nil, fmt.Errorf("think state configuration not found: %s", activeThinkState)
	}

	candidates := sig.Candidates
	if len(candidates) == 0 {
		candidates = scope.Candidates()
	}

	if emitter != nil {
		emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Preparing %d candidate proposal(s) for synthesis...", len(candidates)),
		})
	}

	// Build domain-projected candidate blueprints (Go AST symbols & declarations for Go spaces,
	// clean fenced files for prose/generic spaces) so the Synthesis Worker sees clean source.
	var digestBuilder strings.Builder
	if strings.TrimSpace(sig.OriginalContext) != "" {
		digestBuilder.WriteString(strings.TrimSpace(sig.OriginalContext))
	}

	if len(candidates) > 0 {
		if digestBuilder.Len() > 0 {
			digestBuilder.WriteString("\n\n")
		}
		digestBuilder.WriteString("### Candidate Proposals to Merge\n")
		for _, cand := range candidates {
			if cand.Diff == "" && e.service != nil {
				if readDiff, diffErr := e.service.ReadCandidate(ctx, chatSession, cand.ID); diffErr == nil {
					cand.Diff = readDiff
				} else if e.logger != nil {
					e.logger.WarnContext(ctx, "failed reading candidate diff for synthesis", "candidate", cand.ID, "error", diffErr)
				}
			}

			statusBadge := "PASSED"
			if !cand.Passed {
				statusBadge = "VERIFICATION FAILED"
			}
			fmt.Fprintf(&digestBuilder, "\n### Candidate `%s` (`%s` — %s)\n", cand.AgentID, cand.BranchRef(), statusBadge)
			if !cand.Passed && strings.TrimSpace(cand.Trace) != "" {
				fmt.Fprintf(&digestBuilder, "Verification Trace:\n```\n%s\n```\n", strings.TrimSpace(cand.Trace))
			}
			digestBuilder.WriteString(ProjectCandidateForSpace(space, cand))
			digestBuilder.WriteString("\n")
		}
	}

	// Materialize any pre-existing base TargetFiles from the workbench without failing fast on new files.
	var baseWorkbenchItems []cognition.WorkbenchItem
	if e.service != nil && len(sig.TargetFiles) > 0 {
		reader := e.service.WorkbenchReader(chatSession)
		var existingBaseFiles []string
		for _, target := range sig.TargetFiles {
			if _, readErr := reader.ReadFile(ctx, target); readErr == nil {
				existingBaseFiles = append(existingBaseFiles, target)
			}
		}
		if len(existingBaseFiles) > 0 {
			items, readErr := workspace.ReadFileContext(ctx, reader, existingBaseFiles)
			if readErr != nil {
				return nil, readErr
			}
			baseWorkbenchItems = items
		}
	}

	flowName := ""
	if sig.FlowAddress != nil && sig.FlowAddress.SubGraph.Name != "" {
		flowName = sig.FlowAddress.SubGraph.Name
	} else {
		flowName = fmt.Sprintf("flow-%s", uuid.NewV7().String())
	}

	flowContext := flows.FlowContext{
		FlowID:         flowName,
		SpaceID:        chatSession.ProjectID,
		BaseAgentRules: space.Config().WorkerSystemPrompt(),
		FlowSummary:    "Synthesizing refined solution",
	}

	synthTask := flows.SubAgentTask{
		AgentID:       "synthesis",
		ContextDigest: digestBuilder.String(),
		Instruction:   synthesizePrompt,
		RetryPrompt:   space.Config().Roles.Worker.RetryPrompt,
		TargetFiles:   sig.TargetFiles,
	}

	synthBundle := e.bundleAssembler.AssembleWorkerBundle(
		ctx,
		chatSession.ProjectID,
		space,
		flowContext.BaseAgentRules,
		synthTask.ContextDigest,
		synthTask.Instruction,
		baseWorkbenchItems,
	)

	// Gate synthesis context BEFORE spawning the synthesis sandbox.
	if ui != nil && e.session != nil && e.session.workspaceManager != nil {
		if st, stErr := e.session.workspaceManager.GetSpaceSettings(chatSession.ProjectID); stErr == nil && st.WorkerReviewPolicy.Active {
			policy := e.session.WorkerReviewPolicy()
			reviewID := uuid.NewV7()
			turnContext := ProjectTurnContext(chatSession.ProjectID, chatSession.ChatID, reviewID, ContextStageWorker, &synthBundle, thinkgraph.AttentionFocus{})
			if policy.ShouldReview(st.WorkerReviewPolicy, turnContext) {
				previewMarkdown := FormatBundlePreviewMarkdown("Sub-Agent Context Preview (synthesis)", &synthBundle)
				if previewID, saveErr := e.service.SaveContextPreview(ctx, chatSession, previewMarkdown); saveErr != nil {
					if e.logger != nil {
						e.logger.WarnContext(ctx, "failed saving synthesis context preview", "error", saveErr)
					}
				} else {
					turnContext.PreviewID = previewID
				}

				decision := ui.ReviewContext(turnContext)
				if !decision.Approved {
					if failErr := scope.Fail(ctx, "Synthesis aborted by user during context review.", "", nil, ui); failErr != nil && e.logger != nil {
						e.logger.ErrorContext(ctx, "failed recording synthesis abort state", "error", failErr)
					}
					return &signals.SignalTerminal{}, nil
				}

				if decision.RetainedFilePaths != nil {
					retainedPaths := make(map[string]bool, len(decision.RetainedFilePaths))
					for _, p := range decision.RetainedFilePaths {
						retainedPaths[p] = true
					}
					var prunedWorkbench []cognition.WorkbenchItem
					for _, item := range synthBundle.Workbench {
						if retainedPaths[item.Path] {
							prunedWorkbench = append(prunedWorkbench, item)
						}
					}
					synthBundle.Workbench = prunedWorkbench
				}
			}
		}
	}

	// Execute the single-pass Synthesis Worker directly linked from optionsNodeID on the subgraph.
	synthResult := e.runner.Run(ctx, SubAgentRunRequest{
		ChatSession:          chatSession,
		FlowAddress:          sig.FlowAddress,
		Scope:                scope,
		Space:                space,
		FlowContext:          flowContext,
		Task:                 synthTask,
		AgentIndex:           len(candidates) + 1,
		ParentNodeID:         sig.ParentNodeID,
		DeferResolutionEvent: true,
		PrebuiltBundle:       &synthBundle,
		SkipContextReview:    true,
		UI:                   ui,
		Emitter:              emitter,
	})

	if synthResult.Err != nil || !synthResult.PatchApplied {
		errMsg := "Synthesis failed to produce a patch."
		if synthResult.Err != nil {
			errMsg = synthResult.Err.Error()
		} else if synthResult.Trace != "" {
			errMsg = synthResult.Trace
		}

		if synthResult.GenNodeID != uuid.Nil() {
			if resErr := e.appendSubgraphResolution(ctx, sig.FlowAddress, synthResult.GenNodeID, synthResult.CandidateID, false, errMsg, ui); resErr != nil {
				return nil, resErr
			}
		}

		// Never abandon the user: if surviving worker candidates exist, allow manual review on them
		if len(candidates) > 0 && ui != nil {
			acceptedCount := 0
			for _, cand := range candidates {
				accepted, userReason := ui.ReviewCandidate(cand.BranchRef())
				if accepted {
					acceptedCount++
				}
				reason := strings.TrimSpace(userReason)
				if reason == "" {
					if accepted {
						reason = "Accepted by user after synthesis failure"
					} else {
						reason = "Rejected by user after synthesis failure"
					}
				}
				if err := scope.ResolveCandidate(ctx, cand.ID, accepted, reason); err != nil {
					return nil, fmt.Errorf("resolving surviving candidate %s: %w", cand.ID, err)
				}
			}

			if acceptedCount > 0 {
				if err := scope.Complete(ctx, fmt.Sprintf("Synthesis failed (%s); accepted %d surviving candidate(s).", errMsg, acceptedCount), nil, ui); err != nil {
					return nil, err
				}
			} else {
				if err := scope.Fail(ctx, "Synthesis failed and all surviving candidates were rejected.", errMsg, nil, ui); err != nil {
					return nil, err
				}
			}
			return &signals.SignalTerminal{}, nil
		}

		if failErr := scope.Fail(ctx, "Synthesis failed: "+errMsg, errMsg, nil, ui); failErr != nil && e.logger != nil {
			e.logger.ErrorContext(ctx, "failed recording synthesis failure state", "error", failErr)
		}
		if synthResult.Err != nil {
			return nil, fmt.Errorf("synthesis execution failed: %w", synthResult.Err)
		}
		return &signals.SignalTerminal{}, nil
	}

	// Main Timeline Manager Review Turn (presents merge rationale + resulting diff on main before Accept/Reject)
	mergeReasoning := extractSynthesisReasoning(synthResult.RawPayload)
	renderedReviewPrompt := synthesisReviewPromptTmpl
	if tmpl, parseErr := template.New("synthesis_review").Parse(synthesisReviewPromptTmpl); parseErr == nil {
		var buf bytes.Buffer
		reviewView := synthesisReviewView{
			CandidateID: synthResult.Candidate.BranchRef(),
			Passed:      synthResult.Passed,
			Trace:       synthResult.Trace,
			Diff:        synthResult.Diff,
			Reasoning:   mergeReasoning,
		}
		if execErr := tmpl.Execute(&buf, reviewView); execErr == nil {
			renderedReviewPrompt = buf.String()
		} else if e.logger != nil {
			e.logger.WarnContext(ctx, "failed executing synthesis_review_prompt template", "error", execErr)
		}
	} else if e.logger != nil {
		e.logger.WarnContext(ctx, "failed parsing synthesis_review_prompt template", "error", parseErr)
	}

	reviewBundle := e.bundleAssembler.AssembleWorkerBundle(
		ctx,
		chatSession.ProjectID,
		space,
		space.Config().ManagerSystemPrompt(),
		sig.OriginalContext,
		renderedReviewPrompt,
		nil,
	)
	reviewBundle.Rules.FormattingConstraints = ""

	managerModel := space.Config().Roles.Manager.Model
	reviewNodeID := uuid.NewV7()
	if _, reviewErr := e.turnExecutor.Execute(ctx, chatSession, reviewNodeID, sig.ParentNodeID, managerModel, true, space, reviewBundle, tools.NewRegistry(), ui, emitter); reviewErr != nil {
		return nil, fmt.Errorf("executing main timeline synthesis review: %w", reviewErr)
	}

	// User Review & Resolution of the synthesized candidate
	candidateID := synthResult.CandidateID
	verified := synthResult.Passed
	verifyTrace := synthResult.Trace

	accepted := false
	resolutionReason := "Synthesis verification passed."
	if !verified {
		resolutionReason = verifyTrace
	}
	var userDecisionNodeContent string

	if verified && sig.AutoAccept {
		accepted = true
		resolutionReason = "Auto-accepted by Refine strategy"
		if err := scope.ResolveCandidate(ctx, candidateID, true, resolutionReason); err != nil {
			return nil, fmt.Errorf("auto-accepting synthesized candidate: %w", err)
		}
	} else if ui != nil {
		var userReason string
		accepted, userReason = ui.ReviewCandidate(synthResult.Candidate.BranchRef())
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

		if err := scope.ResolveCandidate(ctx, candidateID, accepted, resolutionReason); err != nil {
			return nil, fmt.Errorf("resolving synthesized candidate: %w", err)
		}
	}

	if err := e.appendSubgraphResolution(ctx, sig.FlowAddress, synthResult.GenNodeID, candidateID, accepted, resolutionReason, ui); err != nil {
		return nil, err
	}

	if userDecisionNodeContent != "" {
		decisionEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, userDecisionNodeContent)
		decisionEvent.Edges = map[string]string{"parent_id": reviewNodeID.String()}

		if err := e.broadcaster.AppendAndStream(ctx, chatSession, decisionEvent, ui); err != nil {
			return nil, fmt.Errorf("appending user decision node to main timeline: %w", err)
		}
	}

	flowSummary := "Refined solution synthesized and accepted."
	if accepted {
		if !verified {
			flowSummary = "Refined solution accepted despite verification failure."
		}
		if err := scope.Complete(ctx, flowSummary, nil, ui); err != nil {
			return nil, err
		}
	} else if !verified {
		flowSummary = "Synthesis verification failed and candidate was rejected."
		if err := scope.Fail(ctx, flowSummary, verifyTrace, nil, ui); err != nil {
			return nil, err
		}
	} else {
		flowSummary = "Refined solution rejected: " + resolutionReason
		if err := scope.Complete(ctx, flowSummary, nil, ui); err != nil {
			return nil, err
		}
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
