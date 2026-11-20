package session_test

import (
	"strings"
	"testing"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestTokenThresholdPolicy_ShouldReview(t *testing.T) {
	policy := session.NewTokenThresholdPolicy()
	focus := thinkgraph.AttentionFocus{}
	responseID := uuid.NewV7()

	tests := []struct {
		name     string
		policy   workspace.ReviewPolicySettings
		bundle   *cognition.ThinkBundle
		expected bool
	}{
		{
			name: "Review active, large persona excluded -> below threshold bypasses",
			policy: workspace.ReviewPolicySettings{
				Active:         true,
				MinTokens:      200,
				IncludePersona: false,
			},
			bundle: &cognition.ThinkBundle{
				Rules: cognition.SystemRules{Persona: strings.Repeat("a", 2000)},
				Trajectory: cognition.Trajectory{
					WorkingMemory: []cognition.TrajectoryEvent{
						{Role: "user", Content: "hi"},
					},
				},
			},
			expected: false,
		},
		{
			name: "Review active, large persona included -> exceeds threshold triggers review",
			policy: workspace.ReviewPolicySettings{
				Active:         true,
				MinTokens:      200,
				IncludePersona: true,
			},
			bundle: &cognition.ThinkBundle{
				Rules: cognition.SystemRules{Persona: strings.Repeat("a", 2000)},
				Trajectory: cognition.Trajectory{
					WorkingMemory: []cognition.TrajectoryEvent{
						{Role: "user", Content: "hi"},
					},
				},
			},
			expected: true,
		},
		{
			name: "Disabled globally overrides high dynamic token count",
			policy: workspace.ReviewPolicySettings{
				Active:         false,
				MinTokens:      10,
				IncludePersona: false,
			},
			bundle: &cognition.ThinkBundle{
				Workbench: []cognition.WorkbenchItem{
					{Path: "main.go", TokenCount: 500},
				},
			},
			expected: false,
		},
		{
			name: "Review active, persona excluded, file tokens push over threshold",
			policy: workspace.ReviewPolicySettings{
				Active:         true,
				MinTokens:      150,
				IncludePersona: false,
			},
			bundle: &cognition.ThinkBundle{
				Workbench: []cognition.WorkbenchItem{
					{Path: "main.go", TokenCount: 200},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			turnContext := session.ProjectTurnContext("space-1", "chat-1", responseID, session.ContextStageManager, tt.bundle, focus)
			result := policy.ShouldReview(tt.policy, turnContext)
			if result != tt.expected {
				t.Errorf("expected ShouldReview to be %v, got %v", tt.expected, result)
			}
		})
	}
}
