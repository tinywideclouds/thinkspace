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
	if len(state.Candidates) == 0 {
		if len(state.InitialTasks) == 0 {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("manager provided no topics to research"),
			}
		}

		// Inject the research prompt wrapper into the provided tasks
		for i := range state.InitialTasks {
			if state.InitialTasks[i].AgentID == "" {
				state.InitialTasks[i].AgentID = fmt.Sprintf("researcher-%d", i+1)
			}
			state.InitialTasks[i].Instruction = fmt.Sprintf("%s\nTopic: %s", f.config.ResearchPrompt, state.InitialTasks[i].Instruction)
		}

		return FlowDirective{
			Action: ActionSpawnAgents,
			Tasks:  state.InitialTasks,
		}
	}

	if len(state.Candidates) > 0 {
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

	return FlowDirective{Action: ActionFinish}
}
