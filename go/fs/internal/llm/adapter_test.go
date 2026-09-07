package llm_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

func TestAdapter_BuildHistory(t *testing.T) {
	adapter := llm.NewAdapter(nil)

	events := []workspace.Event{
		{
			Type:    workspace.EventPrompt,
			Content: "Hello",
		},
		{
			Type:    workspace.EventModel,
			Content: "Hi there",
		},
		{
			Type: workspace.EventCandidate,
			Metadata: map[string]string{
				"files":        "main.go",
				"proposal_uid": "cand-123",
			},
		},
		{
			Type: workspace.EventResolution,
			Metadata: map[string]string{
				"proposal_uid": "cand-123",
				"status":       "ACCEPTED",
				"reason":       "Looks good",
			},
		},
	}

	history := adapter.BuildHistory(events)

	if len(history) != 4 {
		t.Fatalf("expected 4 history items, got %d", len(history))
	}

	if history[0].Role != "user" || history[0].Parts[0].Text != "Hello" {
		t.Errorf("history[0] mismatch, got role %s and text %s", history[0].Role, history[0].Parts[0].Text)
	}
	if history[1].Role != "model" || history[1].Parts[0].Text != "Hi there" {
		t.Errorf("history[1] mismatch")
	}
	if history[2].Role != "model" || history[2].Parts[0].Text != "[Action Taken: Proposed files (main.go) under proposal UID: cand-123]" {
		t.Errorf("history[2] mismatch, got: %s", history[2].Parts[0].Text)
	}
	if history[3].Role != "user" || history[3].Parts[0].Text != "[System Feedback: Proposal cand-123 was ACCEPTED. Reason: Looks good]" {
		t.Errorf("history[3] mismatch, got: %s", history[3].Parts[0].Text)
	}
}

func TestAdapter_InterceptToolCalls(t *testing.T) {
	adapter := llm.NewAdapter(nil)

	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "Thinking..."},
						{
							FunctionCall: &genai.FunctionCall{
								Name: "propose_change",
								Args: map[string]any{"file": "test.go"},
							},
						},
					},
				},
			},
		},
	}

	calls := adapter.InterceptToolCalls(resp)

	if len(calls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(calls))
	}

	if calls[0].Name != "propose_change" {
		t.Errorf("expected tool name 'propose_change', got '%s'", calls[0].Name)
	}

	if calls[0].Args["file"] != "test.go" {
		t.Errorf("expected arg file='test.go', got '%v'", calls[0].Args["file"])
	}
}
