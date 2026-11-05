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
		tasks := f.parseTasks(state.LLMArguments)
		if len(tasks) == 0 {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("manager provided no valid agent_tasks to execute"),
			}
		}
		return FlowDirective{
			Action: ActionSpawnAgents,
			Tasks:  tasks,
		}
	}

	if len(state.Candidates) > 0 && state.UserStrategy == "" {
		allFailed := true
		for _, c := range state.Candidates {
			if c.Passed {
				allFailed = false
				break
			}
		}
		if allFailed {
			return FlowDirective{
				Action: ActionError,
				Error:  fmt.Errorf("all sub-agents failed verification"),
			}
		}
		return FlowDirective{Action: ActionRequestStrategy}
	}

	if state.UserStrategy != "" && state.TargetBranch == "" {
		switch state.UserStrategy {
		case StrategySkip:
			return FlowDirective{Action: ActionFinish}
		case StrategyManual, StrategyReview:
			return FlowDirective{Action: ActionManualReview}
		case StrategyRefine:
			return FlowDirective{
				Action: ActionSynthesize,
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

	if state.TargetBranch != "" {
		return FlowDirective{
			Action:       ActionCommit,
			TargetBranch: state.TargetBranch,
		}
	}

	return FlowDirective{Action: ActionFinish}
}

func (f *FanOutFlow) parseTasks(args map[string]any) []SubAgentTask {
	var tasks []SubAgentTask
	if rawTasks, ok := args["agent_tasks"].([]any); ok {
		for i, raw := range rawTasks {
			taskMap, isMap := raw.(map[string]any)
			if isMap {
				digest, _ := taskMap["context_digest"].(string)
				instruction, _ := taskMap["instruction"].(string)

				targetFiles := make([]string, 0)
				if rawFiles, hasFiles := taskMap["target_files"].([]any); hasFiles {
					for _, fileRaw := range rawFiles {
						if strFile, isStr := fileRaw.(string); isStr {
							targetFiles = append(targetFiles, strFile)
						}
					}
				}

				tasks = append(tasks, SubAgentTask{
					AgentID:       fmt.Sprintf("agent-%d", i+1),
					ContextDigest: digest,
					Instruction:   instruction,
					TargetFiles:   targetFiles,
					RetryPrompt:   f.config.RetryPrompt,
				})
			} else {
				tasks = append(tasks, SubAgentTask{
					AgentID:     fmt.Sprintf("agent-%d", i+1),
					Instruction: fmt.Sprintf("%v", raw),
					TargetFiles: make([]string, 0),
					RetryPrompt: f.config.RetryPrompt,
				})
			}
		}
	}
	return tasks
}
