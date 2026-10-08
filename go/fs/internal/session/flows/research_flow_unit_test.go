package flows

import (
	"log/slog"
	"os"
	"testing"
)

func TestResearchFlow_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewResearchFlow(logger, ResearchConfig{ResearchPrompt: "Gather data on:"})

	t.Run("Stage 1 -> Spawn Agents", func(t *testing.T) {
		state := FlowState{
			LLMArguments: map[string]any{
				"topics": []any{"1960s perspective", "Feminist perspective"},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionSpawnAgents {
			t.Errorf("Expected ActionSpawnAgents, got %s", directive.Action)
		}
		if len(directive.Tasks) != 2 {
			t.Errorf("Expected 2 research tasks, got %d", len(directive.Tasks))
		}
	})

	t.Run("Stage 2 -> Bypass Strategy and Force Manual Review", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			// Notice UserStrategy is completely empty/ignored
		}
		directive := flow.Next(state)
		if directive.Action != ActionManualReview {
			t.Errorf("Expected ActionManualReview, got %s", directive.Action)
		}
	})

	t.Run("Stage 3 -> Commit Accepted Research", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: true},
			},
			TargetBranch: "candidate/cand-1",
		}
		directive := flow.Next(state)
		if directive.Action != ActionCommit {
			t.Errorf("Expected ActionCommit, got %s", directive.Action)
		}
	})
}
