package flows

import (
	"fmt"
	"log/slog"
)

type ResearchConfig struct {
	MaxSteps       int    `yaml:"max_steps"`
	ResearchPrompt string `yaml:"research_prompt"`
}

type ResearchFlow struct {
	logger *slog.Logger
	config ResearchConfig
}

func NewResearchFlow(logger *slog.Logger, config ResearchConfig) *ResearchFlow {
	return &ResearchFlow{
		logger: logger,
		config: config,
	}
}

func (f *ResearchFlow) Name() string {
	return "ResearchFlow"
}

func (f *ResearchFlow) MaxSteps() int {
	if f.config.MaxSteps <= 0 {
		return 10 // Safe fallback for research bounds
	}
	return f.config.MaxSteps
}

func (f *ResearchFlow) Next(state FlowState) FlowDirective {
	if len(state.Candidates) == 0 && state.TargetBranch == "" {
		topics, ok := state.LLMArguments["topics"].([]any)
		if !ok || len(topics) == 0 {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("manager provided no topics to research"),
			}
		}

		var tasks []SubAgentTask
		for i, rawTopic := range topics {
			tasks = append(tasks, SubAgentTask{
				AgentID:     fmt.Sprintf("researcher-%d", i+1),
				Instruction: fmt.Sprintf("%s\nTopic: %s", f.config.ResearchPrompt, rawTopic),
				TargetFiles: []string{},
			})
		}

		return FlowDirective{
			Action: ActionSpawnAgents,
			Tasks:  tasks,
		}
	}

	if len(state.Candidates) > 0 && state.TargetBranch == "" {
		hasPass := false
		for _, c := range state.Candidates {
			if c.Passed {
				hasPass = true
				break
			}
		}
		if !hasPass {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("all research agents failed to compile their notes"),
			}
		}

		return FlowDirective{Action: ActionManualReview}
	}

	if state.TargetBranch != "" {
		return FlowDirective{
			Action:       ActionCommit,
			TargetBranch: state.TargetBranch,
		}
	}

	return FlowDirective{Action: ActionFinish}
}
