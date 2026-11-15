package flows

import (
	"fmt"
	"log/slog"
)

type FanOutConfig struct {
	MaxSteps     int    `yaml:"max_steps"`
	RetryPrompt  string `yaml:"retry_prompt"`
	ReviewPrompt string `yaml:"review_prompt"`
	RefinePrompt string `yaml:"refine_prompt"`
}

type FanOutFlow struct {
	logger *slog.Logger
	config FanOutConfig
}

func NewFanOutFlow(logger *slog.Logger, config FanOutConfig) *FanOutFlow {
	return &FanOutFlow{
		logger: logger,
		config: config,
	}
}

func (f *FanOutFlow) Name() string {
	return "FanOutFlow"
}

func (f *FanOutFlow) MaxSteps() int {
	if f.config.MaxSteps <= 0 {
		return 15 // Safe fallback for a standard FanOut loop
	}
	return f.config.MaxSteps
}

func (f *FanOutFlow) Next(state FlowState) FlowDirective {
	if len(state.Candidates) == 0 && state.UserStrategy == "" {
		if len(state.InitialTasks) == 0 {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("supervisor provided no valid agent_tasks to execute"),
			}
		}

		for i := range state.InitialTasks {
			state.InitialTasks[i].RetryPrompt = f.config.RetryPrompt
		}

		return FlowDirective{
			Action: ActionSpawnAgents,
			Tasks:  state.InitialTasks,
		}
	}

	if len(state.Candidates) > 0 && state.UserStrategy == "" {
		return FlowDirective{Action: ActionRequestStrategy}
	}

	if state.UserStrategy != "" {
		switch state.UserStrategy {
		case StrategySkip:
			return FlowDirective{Action: ActionFinish}
		case StrategyManual, StrategyReview:
			return FlowDirective{Action: ActionManualReview}
		case StrategyRefine:
			return FlowDirective{
				Action:     ActionSynthesize,
				AutoAccept: state.AutoAccept,
				Tasks: []SubAgentTask{
					{Instruction: f.config.RefinePrompt},
				},
			}
		default:
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("unknown flow strategy: %s", state.UserStrategy),
			}
		}
	}

	return FlowDirective{Action: ActionFinish}
}
