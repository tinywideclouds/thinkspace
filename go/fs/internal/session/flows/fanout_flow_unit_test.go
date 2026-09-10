package flows

import (
	"log/slog"
	"os"
	"reflect"
	"testing"
)

func TestFanOutFlow_ParseTasks(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	flow := NewFanOutFlow(logger)

	tests := []struct {
		name     string
		args     map[string]any
		expected []SubAgentTask
	}{
		{
			name: "Structured Object Tasks",
			args: map[string]any{
				"agent_tasks": []any{
					map[string]any{
						"context_digest": "System uses Go 1.22",
						"instruction":    "Build a mux",
					},
					map[string]any{
						"context_digest": "System uses Go 1.22",
						"instruction":    "Build a gin router",
					},
				},
			},
			expected: []SubAgentTask{
				{AgentID: "agent-1", ContextDigest: "System uses Go 1.22", Instruction: "Build a mux"},
				{AgentID: "agent-2", ContextDigest: "System uses Go 1.22", Instruction: "Build a gin router"},
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
				{AgentID: "agent-1", ContextDigest: "", Instruction: "Just build a mux"},
				{AgentID: "agent-2", ContextDigest: "", Instruction: "Just build a gin router"},
			},
		},
		{
			name:     "Missing or Empty Tasks",
			args:     map[string]any{},
			expected: nil,
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
