package flows

import (
	"log/slog"
	"os"
	"testing"
)

func TestFanOutFlow_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewFanOutFlow(logger, FanOutConfig{})

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

	t.Run("All Agents Failed -> Error", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: false},
				{CandidateID: "cand-2", Passed: false},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionError {
			t.Errorf("Expected ActionError because all candidates failed, got %s", directive.Action)
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

	t.Run("User Accepted Branch -> Commit", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			UserStrategy: StrategyManual,
			TargetBranch: "candidate/cand-1",
		}
		directive := flow.Next(state)
		if directive.Action != ActionCommit {
			t.Errorf("Expected ActionCommit, got %s", directive.Action)
		}
		if directive.TargetBranch != "candidate/cand-1" {
			t.Errorf("Expected target branch candidate/cand-1, got %s", directive.TargetBranch)
		}
	})
}
