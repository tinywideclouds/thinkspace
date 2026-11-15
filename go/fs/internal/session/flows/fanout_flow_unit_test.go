package flows

import (
	"log/slog"
	"os"
	"testing"
)

func TestFanOutFlow_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewFanOutFlow(logger, FanOutConfig{RefinePrompt: "Synthesize this"})

	t.Run("Initial State -> Spawn Agents", func(t *testing.T) {
		state := FlowState{
			InitialTasks: []SubAgentTask{{Instruction: "task 1"}},
		}
		directive := flow.Next(state)
		if directive.Action != ActionSpawnAgents {
			t.Errorf("Expected ActionSpawnAgents, got %s", directive.Action)
		}
		if len(directive.Tasks) != 1 {
			t.Errorf("Expected 1 task parsed, got %d", len(directive.Tasks))
		}
	})

	t.Run("Missing Tasks -> Error", func(t *testing.T) {
		state := FlowState{
			InitialTasks: nil,
		}
		directive := flow.Next(state)
		if directive.Action != ActionError {
			t.Errorf("Expected ActionError, got %s", directive.Action)
		}
	})

	t.Run("Agents Finished Successfully -> Request Strategy", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionRequestStrategy {
			t.Errorf("Expected ActionRequestStrategy, got %s", directive.Action)
		}
	})

	t.Run("All Agents Failed -> Still Requests Strategy For User Review/Accept", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: false, Trace: "go.mod version mismatch"},
				{CandidateID: "cand-2", Passed: false, Trace: "test failed"},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionRequestStrategy {
			t.Errorf("Expected ActionRequestStrategy so user can review/accept failed candidates, got %s", directive.Action)
		}
	})

	t.Run("User Chose Manual Strategy -> Manual Review", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			UserStrategy: StrategyManual,
		}
		directive := flow.Next(state)
		if directive.Action != ActionManualReview {
			t.Errorf("Expected ActionManualReview, got %s", directive.Action)
		}
	})

	t.Run("User Chose Skip Strategy -> Finish", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			UserStrategy: StrategySkip,
		}
		directive := flow.Next(state)
		if directive.Action != ActionFinish {
			t.Errorf("Expected ActionFinish, got %s", directive.Action)
		}
	})

	t.Run("User Chose Refine Strategy -> Synthesize", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			UserStrategy: StrategyRefine,
		}
		directive := flow.Next(state)
		if directive.Action != ActionSynthesize {
			t.Errorf("Expected ActionSynthesize, got %s", directive.Action)
		}
		if len(directive.Tasks) != 1 || directive.Tasks[0].Instruction != "Synthesize this" {
			t.Errorf("Expected RefinePrompt in task instruction")
		}
	})
}
