package flows

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
)

func TestFanOutFlow_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewFanOutFlow(logger, FanOutConfig{})

	t.Run("Initial State -> Spawn Agents", func(t *testing.T) {
		state := FlowState{
			LLMArguments: map[string]any{
				"agent_tasks": []any{"task 1"},
			},
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
			LLMArguments: map[string]any{},
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

func TestFanOutFlow_ParseTasks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewFanOutFlow(logger, FanOutConfig{})

	tests := []struct {
		name     string
		args     map[string]any
		expected []SubAgentTask
	}{
		{
			name: "Structured Object Tasks with Target Files",
			args: map[string]any{
				"agent_tasks": []any{
					map[string]any{
						"context_digest": "System uses Go 1.22",
						"instruction":    "Build a mux",
						"target_files":   []any{"src/main.go", "src/router.go"},
					},
					map[string]any{
						"context_digest": "System uses Go 1.22",
						"instruction":    "Build a gin router",
					},
				},
			},
			expected: []SubAgentTask{
				{
					AgentID:       "agent-1",
					ContextDigest: "System uses Go 1.22",
					Instruction:   "Build a mux",
					TargetFiles:   []string{"src/main.go", "src/router.go"},
				},
				{
					AgentID:       "agent-2",
					ContextDigest: "System uses Go 1.22",
					Instruction:   "Build a gin router",
					TargetFiles:   []string{},
				},
			},
		},
		{
			name: "Flat String Tasks (Fallback)",
			args: map[string]any{
				"agent_tasks": []any{
					"Just build a mux",
					"Just build a gin router",
				},
			},
			expected: []SubAgentTask{
				{AgentID: "agent-1", ContextDigest: "", Instruction: "Just build a mux", TargetFiles: []string{}},
				{AgentID: "agent-2", ContextDigest: "", Instruction: "Just build a gin router", TargetFiles: []string{}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := flow.parseTasks(tt.args)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("parseTasks() mismatch\nExpected: %+v\nActual: %+v", tt.expected, actual)
			}
		})
	}
}
