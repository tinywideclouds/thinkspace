package session

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"uuid"
)

type Delegator struct {
	logger           *slog.Logger
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	executor         workspace.SubAgentExecutor
	fanOutFlow       flows.Flow
	emitter          telemetry.Emitter
	spaceID          string
	baseAgentRules   string
	flowConfig       flows.FlowConfig
}

func NewDelegator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	modelClient llm.ModelClient,
	executor workspace.SubAgentExecutor,
	fanOutFlow flows.Flow,
	emitter telemetry.Emitter,
	spaceID string,
	baseAgentRules string,
	flowConfig flows.FlowConfig,
) *Delegator {
	return &Delegator{
		logger:           logger,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		executor:         executor,
		fanOutFlow:       fanOutFlow,
		emitter:          emitter,
		spaceID:          spaceID,
		baseAgentRules:   baseAgentRules,
		flowConfig:       flowConfig,
	}
}

func (d *Delegator) ExecuteProposeChange(
	ctx context.Context,
	thread *chat.Thread,
	space spaces.ThinkSpace,
	bundle cognition.ThinkBundle,
	userInterface UserInterface,
	call llm.ToolCall,
	managerModel string,
) error {
	flowID := fmt.Sprintf("flow-%d", time.Now().Unix())

	d.emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Validating delegation parameters...",
	})

	assignedTags := make([]string, 0)
	if rawTags, ok := call.Args["assigned_tags"].([]any); ok {
		for _, t := range rawTags {
			if strTag, ok := t.(string); ok {
				assignedTags = append(assignedTags, strTag)
			}
		}
	}

	description := "Delegated Task"
	if tasks, ok := call.Args["agent_tasks"].([]any); ok && len(tasks) > 0 {
		if firstTask, ok := tasks[0].(map[string]any); ok {
			if instr, ok := firstTask["instruction"].(string); ok {
				description = fmt.Sprintf("Proposed: %s", instr)
			}
		}
	}

	_ = d.workspaceService.LogProposal(ctx, thread, flowID, description, assignedTags)

	flowContext := flows.FlowContext{
		FlowID:         flowID,
		SpaceID:        d.spaceID,
		BaseAgentRules: d.baseAgentRules,
		FlowSummary:    description,
	}

	result, err := d.fanOutFlow.Execute(
		ctx, d.workspaceService, thread, space, call.Args, d.flowConfig, flowContext, d.emitter, d.executor, space.Verifier(),
	)
	if err != nil {
		return err
	}

	var validBranches []flows.BranchResult
	for _, branch := range result.Branches {
		if branch.Passed {
			validBranches = append(validBranches, branch)
		}
	}

	if len(validBranches) == 0 {
		userInterface.OnTextChunk("\n\n❌ **All sub-agents failed verification.** Check the trace logs for details.\n")
		_ = d.workspaceService.LogResolution(ctx, thread, flowID, false, "All candidates failed verification")
		return nil
	}

	var branchOptions []string
	for _, branch := range validBranches {
		branchOptions = append(branchOptions, branch.CandidateID)
	}

	userInterface.OnTextChunk(fmt.Sprintf("\n\n✅ Sub-agents have produced %d valid candidate branches.\n", len(validBranches)))

	strategy := userInterface.ChooseNextStep()

	switch strategy {
	case StrategySkip:
		userInterface.OnTextChunk("\nSkipping candidates. The conversation will continue without merging.\n")
		_ = d.workspaceService.LogResolution(ctx, thread, flowID, false, "Skipped by user")
		return nil

	case StrategyManual:
		for _, branchID := range branchOptions {
			if userInterface.ReviewCandidate(branchID) {
				userInterface.OnTextChunk(fmt.Sprintf("\nMerging %s...\n", branchID))
				_ = d.workspaceService.ResolveCandidate(ctx, thread, branchID, true, "Manually accepted by user")
				return nil
			}
		}
		_ = d.workspaceService.LogResolution(ctx, thread, flowID, false, "All candidates manually rejected")
		userInterface.OnTextChunk("\nNo branches selected. Continuing.\n")
		return nil

	default:
		userInterface.OnTextChunk("\nUnsupported strategy. Defaulting to skip.\n")
		_ = d.workspaceService.LogResolution(ctx, thread, flowID, false, "Unsupported strategy fallback")
		return nil
	}
}
