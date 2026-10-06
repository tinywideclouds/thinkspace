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
	digestFlow       flows.Flow
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
	digestFlow flows.Flow,
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
		digestFlow:       digestFlow,
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
		return fmt.Errorf("all %d sub-agents failed to produce verifiable code", len(result.Branches))
	}

	strategy := userInterface.ChooseNextStep()

	if strategy == StrategySkip {
		for _, branch := range validBranches {
			_ = d.workspaceService.ResolveCandidate(ctx, thread, branch.CandidateID, false, "Skipped by user")
		}
		return nil
	}

	var selectedBranch string

	if strategy == StrategyManual {
		for _, branch := range validBranches {
			accepted := userInterface.ReviewCandidate(branch.CandidateID)
			if accepted {
				selectedBranch = branch.CandidateID
				break
			}
		}
	}

	if selectedBranch != "" {
		for _, branch := range validBranches {
			if branch.CandidateID == selectedBranch {
				_ = d.workspaceService.ResolveCandidate(ctx, thread, branch.CandidateID, true, "Manually accepted by user")
			} else {
				_ = d.workspaceService.ResolveCandidate(ctx, thread, branch.CandidateID, false, "Discarded in favor of another candidate")
			}
		}
	} else {
		for _, branch := range validBranches {
			_ = d.workspaceService.ResolveCandidate(ctx, thread, branch.CandidateID, false, "Rejected by user")
		}
	}

	return nil
}

func (d *Delegator) ExecuteCreateDigest(
	ctx context.Context,
	thread *chat.Thread,
	space spaces.ThinkSpace,
	bundle cognition.ThinkBundle,
	call llm.ToolCall,
) error {
	flowContext := flows.FlowContext{
		FlowID:         fmt.Sprintf("digest-%d", time.Now().Unix()),
		SpaceID:        d.spaceID,
		BaseAgentRules: d.baseAgentRules,
	}

	// We push the manager's recent working memory as part of the context instruction
	// so the background agent actually has something to summarize.
	workingMemoryStr := ""
	for _, memory := range bundle.Trajectory.WorkingMemory {
		workingMemoryStr += fmt.Sprintf("[%s]: %s\n\n", memory.Role, memory.Content)
	}

	call.Args["instruction"] = fmt.Sprintf("Recent Conversation Context:\n%s\n\nTask: %s", workingMemoryStr, call.Args["instruction"])

	_, err := d.digestFlow.Execute(
		ctx, d.workspaceService, thread, space, call.Args, d.flowConfig, flowContext, d.emitter, d.executor, space.Verifier(),
	)
	return err
}
