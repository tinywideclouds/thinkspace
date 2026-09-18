package session

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"text/template"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type Delegator struct {
	logger            *slog.Logger
	workspaceService  *workspace.Service
	llmAdapter        *llm.Adapter
	executor          workspace.SubAgentExecutor
	fanOutFlow        flows.Flow
	emitter           telemetry.Emitter
	spaceID           string
	baseAgentRules    string
	flowConfiguration flows.FlowConfig
}

func NewDelegator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	llmAdapter *llm.Adapter,
	executor workspace.SubAgentExecutor,
	fanOutFlow flows.Flow,
	emitter telemetry.Emitter,
	spaceID string,
	baseAgentRules string,
	flowConfiguration flows.FlowConfig,
) *Delegator {
	return &Delegator{
		logger:            logger,
		workspaceService:  workspaceService,
		llmAdapter:        llmAdapter,
		executor:          executor,
		fanOutFlow:        fanOutFlow,
		emitter:           emitter,
		spaceID:           spaceID,
		baseAgentRules:    baseAgentRules,
		flowConfiguration: flowConfiguration,
	}
}

func (d *Delegator) ExecuteProposeChange(
	turnContext context.Context,
	thread *chat.Thread,
	thinkSpace spaces.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
	call llm.ToolCall,
	managerModel string,
) error {
	var flowSummary strings.Builder
	uniqueFlowID := fmt.Sprintf("flow-%s", uuid.NewV7())

	managerSummary, _ := call.Args["flow_summary"].(string)
	if managerSummary == "" {
		managerSummary = "Executing delegated sub-agent tasks"
	}

	flowContext := flows.FlowContext{
		FlowID:         uniqueFlowID,
		SpaceID:        d.spaceID,
		BaseAgentRules: d.baseAgentRules,
		FlowSummary:    managerSummary,
	}

	var assignedTags []string
	if rawTags, ok := call.Args["assigned_tags"].([]any); ok {
		for _, rt := range rawTags {
			if s, isStr := rt.(string); isStr {
				assignedTags = append(assignedTags, s)
			}
		}
	}

	activeFlowConfig := d.flowConfiguration
	if thinkSpace.Config().Roles.Worker.RetryPrompt != "" {
		activeFlowConfig.RetryPrompt = thinkSpace.Config().Roles.Worker.RetryPrompt
	}

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Initiating parallel FanOut Flow: %s", uniqueFlowID),
	})

	result, err := d.fanOutFlow.Execute(
		turnContext,
		d.workspaceService,
		thread,
		thinkSpace,
		call.Args,
		activeFlowConfig,
		flowContext,
		d.emitter,
		d.executor,
		thinkSpace.Verifier(),
	)

	if err != nil {
		d.logger.ErrorContext(turnContext, "delegation flow failed", "error", err)
		return err
	}

	if len(result.Branches) == 0 {
		d.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "FanOut flow completed with zero valid branches.",
		})
		return nil
	}

	var branchNames []string
	for _, branch := range result.Branches {
		candidateID := strings.TrimPrefix(branch.CandidateID, "candidate/")
		statusMsg := "Proposed branch"
		if !branch.Passed {
			statusMsg = "Proposed FAILED branch"
		}
		branchNames = append(branchNames, branch.CandidateID)
		_ = d.workspaceService.LogProposal(turnContext, thread, candidateID, fmt.Sprintf("%s %s via sub-agent fan-out.", statusMsg, branch.CandidateID), assignedTags)
	}

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Awaiting user triage strategy selection...",
	})
	branchesToReview := branchNames
	strategy := userInterface.ChooseNextStep()

	strategyNames := map[DelegationStrategy]string{
		StrategySkip:   "Skip / Abort",
		StrategyManual: "Manual Review",
		StrategyReview: "Assisted Review",
		StrategyRefine: "Refine (Manager synthesized final version)",
	}

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("User selected strategy: %s", strategyNames[strategy]),
		Payload:   ManagerPayload{Strategy: strategyNames[strategy]}.ToMap(),
	})

	_ = d.workspaceService.LogUserMessage(turnContext, thread, fmt.Sprintf("[User selected triage strategy: %s]", strategyNames[strategy]))
	flowSummary.WriteString(fmt.Sprintf("- User selected triage strategy: %s\n", strategyNames[strategy]))

	if strategy == StrategySkip {
		for _, branch := range branchesToReview {
			candidateID := strings.TrimPrefix(branch, "candidate/")
			_ = d.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Review Skipped)")
			flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected.\n", candidateID))
		}
	} else {
		if strategy == StrategyReview || strategy == StrategyRefine {
			branchesToReview = d.executeLLMReviewPhase(turnContext, thread, thinkSpace, history, userInterface, strategy, result.Branches)
		}

		hasAcceptedAny := false
		for _, branch := range branchesToReview {
			candidateID := strings.TrimPrefix(branch, "candidate/")

			if hasAcceptedAny {
				_ = d.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Another candidate was accepted)")
				flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected (conflict).\n", candidateID))
				continue
			}

			if err := d.workspaceService.PreviewCandidate(turnContext, thread, candidateID); err != nil {
				_ = d.workspaceService.ResolveCandidate(turnContext, thread, candidateID, false, "Auto-rejected (Preview checkout failed)")
				flowSummary.WriteString(fmt.Sprintf("- Candidate %s was Auto-Rejected (checkout failed).\n", candidateID))
				continue
			}

			d.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Awaiting user review for candidate: %s", branch),
				Payload:   ManagerPayload{Branch: branch}.ToMap(),
			})
			accept := userInterface.ReviewCandidate(branch)
			reason, status := "Rejected via triage", "REJECTED"
			if accept {
				reason, status = "Accepted via triage", "ACCEPTED"
				hasAcceptedAny = true
			}

			flowSummary.WriteString(fmt.Sprintf("- Candidate %s was %s by the user.\n", candidateID, status))
			_ = d.workspaceService.ResolveCandidate(turnContext, thread, candidateID, accept, reason)
		}
	}

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Turn complete. Manager is providing final summary.",
	})
	userInterface.OnTextChunk("\n\n🤖 **Manager summarizing turn...**\n")
	wrapUpPrompt := fmt.Sprintf(
		"The sub-agent orchestration flow is now complete. Here is the system trace of the outcome:\n\n%s\n\n"+
			"Please provide a friendly, conversational wrap-up to the human user.", flowSummary.String(),
	)

	history = append(history, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: wrapUpPrompt}},
	})

	wrapUpStream := d.llmAdapter.GenerateStream(turnContext, managerModel, thinkSpace.Config().ManagerSystemPrompt(), nil, history)
	var wrapUpResponse strings.Builder
	
	for chunk, err := range wrapUpStream {
		if err != nil {
			d.logger.ErrorContext(turnContext, "wrap-up stream failed", "error", err)
			break
		}
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					userInterface.OnTextChunk(part.Text)
					wrapUpResponse.WriteString(part.Text)
				}
			}
		}
	}

	if wrapUpResponse.Len() > 0 {
		_ = d.workspaceService.LogModelResponse(turnContext, thread, wrapUpResponse.String())
	}
	return nil
}

type BranchTemplateData struct {
	CandidateID string
	Passed      bool
	Trace       string
	Diff        string
}

type ReviewTemplateData struct {
	Branches []BranchTemplateData
}

func (d *Delegator) executeLLMReviewPhase(
	ctx context.Context,
	thread *chat.Thread,
	thinkSpace spaces.ThinkSpace,
	history []*genai.Content,
	userInterface UserInterface,
	strategy DelegationStrategy,
	candidateBranches []flows.BranchResult,
) []string {

	var data ReviewTemplateData
	var outputBranches []string

	for _, branch := range candidateBranches {
		candidateID := strings.TrimPrefix(branch.CandidateID, "candidate/")
		outputBranches = append(outputBranches, branch.CandidateID)

		diff, err := d.workspaceService.ReadCandidate(ctx, thread, candidateID)
		if err != nil || diff == "" {
			diff = "// No readable diff generated or error fetching diff."
		}

		data.Branches = append(data.Branches, BranchTemplateData{
			CandidateID: branch.CandidateID,
			Passed:      branch.Passed,
			Trace:       branch.Trace,
			Diff:        diff,
		})
	}

	templateStr := d.flowConfiguration.ReviewPrompt
	if strategy == StrategyRefine {
		templateStr = d.flowConfiguration.RefinePrompt
	}

	// Fallback definitions if the YAML is missing the prompt fields
	if templateStr == "" {
		if strategy == StrategyRefine {
			templateStr = "The automated sub-agents have completed their proposals. Please evaluate the following candidate diffs. Write a detailed comparative summary. Based on your evaluation, your entire response will then be used as the architectural instruction for a single, final sub-agent to synthesize the ultimate best version.\n\n### Candidate Branches\n\n{{ range .Branches }}* [`{{ .CandidateID }}`](#branch:{{ .CandidateID }})\n#### {{ .CandidateID }}\n{{ if not .Passed }}⚠️ **VERIFICATION FAILED**:\n```\n{{ .Trace }}\n```\n{{ end }}\n```diff\n{{ .Diff }}\n```\n{{ end }}"
		} else {
			templateStr = "The automated sub-agents have completed their proposals. Please evaluate the following candidate diffs. Write a comparative summary and recommend the best approach to the human user.\n\n### Candidate Branches\n\n{{ range .Branches }}* [`{{ .CandidateID }}`](#branch:{{ .CandidateID }})\n#### {{ .CandidateID }}\n{{ if not .Passed }}⚠️ **VERIFICATION FAILED**:\n```\n{{ .Trace }}\n```\n{{ end }}\n```diff\n{{ .Diff }}\n```\n{{ end }}"
		}
	}

	tmpl, err := template.New("review").Parse(templateStr)
	if err != nil {
		d.logger.ErrorContext(ctx, "failed to parse review template", "error", err)
		tmpl, _ = template.New("fallback").Parse("Evaluate the following candidate diffs:\n{{ range .Branches }}\n#### {{ .CandidateID }}\n```diff\n{{ .Diff }}\n```\n{{ end }}")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		d.logger.ErrorContext(ctx, "failed to execute review template", "error", err)
		buf.WriteString("Error formatting prompt.")
	}

	llmPrompt := buf.String()

	if err := d.workspaceService.LogUserMessage(ctx, thread, llmPrompt); err != nil {
		d.logger.ErrorContext(ctx, "failed to log evaluation prompt to ledger", "error", err)
	}

	evaluationHistory := make([]*genai.Content, len(history))
	copy(evaluationHistory, history)
	evaluationHistory = append(evaluationHistory, &genai.Content{
		Role:  "user",
		Parts: []*genai.Part{{Text: llmPrompt}},
	})

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Manager is analyzing candidate diffs...",
	})
	userInterface.OnTextChunk("\n\n🤖 **Manager evaluating candidates...**\n")

	managerModel := thinkSpace.Config().Roles.Manager.Model
	evaluationStream := d.llmAdapter.GenerateStream(ctx, managerModel, thinkSpace.Config().ManagerSystemPrompt(), nil, evaluationHistory)

	var evaluationResponse strings.Builder
	for chunk, err := range evaluationStream {
		if err != nil {
			d.logger.ErrorContext(ctx, "evaluation stream failed", "error", err)
			return outputBranches
		}
		if len(chunk.Candidates) > 0 && chunk.Candidates[0].Content != nil {
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part.Text != "" {
					userInterface.OnTextChunk(part.Text)
					evaluationResponse.WriteString(part.Text)
				}
			}
		}
	}

	if evaluationResponse.Len() > 0 {
		_ = d.workspaceService.LogModelResponse(ctx, thread, evaluationResponse.String())
	}

	if strategy == StrategyReview {
		return outputBranches
	}

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Manager synthesizing final refinement candidate...",
	})
	userInterface.OnTextChunk("\n\n🚀 **Synthesizing final candidate based on evaluation...**\n")

	refinementArguments := map[string]any{
		"agent_count": float64(1),
		"agent_tasks": []any{
			map[string]any{
				"context_digest": "Synthesize a final implementation based on the following architectural evaluation.",
				"instruction":    evaluationResponse.String(),
			},
		},
	}

	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("refine-%s", thread.ID),
		SpaceID:        d.spaceID,
		BaseAgentRules: d.baseAgentRules,
	}

	activeFlowConfig := d.flowConfiguration
	if thinkSpace.Config().Roles.Worker.RetryPrompt != "" {
		activeFlowConfig.RetryPrompt = thinkSpace.Config().Roles.Worker.RetryPrompt
	}

	refinementResult, err := d.fanOutFlow.Execute(
		ctx,
		d.workspaceService,
		thread,
		thinkSpace,
		refinementArguments,
		activeFlowConfig,
		flowContext,
		d.emitter,
		d.executor,
		thinkSpace.Verifier(),
	)

	if err != nil {
		d.logger.ErrorContext(ctx, "refinement flow failed", "error", err)
		d.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "error",
			Timestamp: time.Now(),
			Message:   fmt.Sprintf("Refinement failed: %v", err),
		})
		userInterface.OnTextChunk("\n\n⚠️ Refinement orchestration failed. Falling back to original candidates.\n")
		return outputBranches
	}

	var finalBranches []string
	for _, b := range refinementResult.Branches {
		finalBranches = append(finalBranches, b.CandidateID)
	}
	finalBranches = append(finalBranches, outputBranches...)

	return finalBranches
}