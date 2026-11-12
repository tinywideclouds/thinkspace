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
			InitialTasks: []SubAgentTask{
				{Instruction: "1960s perspective"},
				{Instruction: "Feminist perspective"},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionSpawnAgents {
			t.Errorf("Expected ActionSpawnAgents, got %s", directive.Action)
		}
		if len(directive.Tasks) != 2 {
			t.Errorf("Expected 2 research tasks, got %d", len(directive.Tasks))
		}
		if directive.Tasks[0].Instruction != "Gather data on:\nTopic: 1960s perspective" {
			t.Errorf("Expected prompt injection, got %q", directive.Tasks[0].Instruction)
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

	t.Run("All Agents Failed -> Error", func(t *testing.T) {
		state := FlowState{
			Candidates: []BranchResult{
				{CandidateID: "cand-1", Passed: false},
			},
		}
		directive := flow.Next(state)
		if directive.Action != ActionError {
			t.Errorf("Expected ActionError, got %s", directive.Action)
		}
	})
}
