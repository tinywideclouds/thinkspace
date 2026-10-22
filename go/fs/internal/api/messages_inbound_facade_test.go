package api_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
)

func TestUnmarshalInbound_SelectStrategy(t *testing.T) {
	tests := []struct {
		name         string
		protoEnumVal int
		expected     flows.FlowStrategy
	}{
		{"UNSPECIFIED fallback", 0, flows.StrategyManual},
		{"SKIP mapping", 1, flows.StrategySkip},
		{"MANUAL mapping", 2, flows.StrategyManual},
		{"REVIEW mapping", 3, flows.StrategyReview},
		{"REFINE mapping", 4, flows.StrategyRefine},
		{"UNKNOWN fallback", 99, flows.StrategyManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := `{"selectStrategy": {"strategyId": ` + string(rune('0'+tt.protoEnumVal)) + `}}`
			if tt.protoEnumVal > 9 {
				payload = `{"selectStrategy": {"strategyId": 99}}`
			}

			inbound, err := api.UnmarshalInbound([]byte(payload))
			if err != nil {
				t.Fatalf("unexpected error unmarshaling: %v", err)
			}

			if inbound.Type != "select_strategy" {
				t.Errorf("expected type 'select_strategy', got '%s'", inbound.Type)
			}

			if inbound.SelectStrategy == nil {
				t.Fatalf("expected SelectStrategy payload to be populated")
			}

			if inbound.SelectStrategy.StrategyID != tt.expected {
				t.Errorf("enum drift detected: proto %d mapped to %v, expected %v", tt.protoEnumVal, inbound.SelectStrategy.StrategyID, tt.expected)
			}
		})
	}
}

func TestUnmarshalInbound_SubmitPrompt(t *testing.T) {
	payload := []byte(`{
		"submitPrompt": {
			"text": "build a server",
			"spaceId": "golang",
			"chatId": "chat-123"
		}
	}`)

	inbound, err := api.UnmarshalInbound(payload)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if inbound.Type != "submit_prompt" {
		t.Errorf("expected 'submit_prompt', got '%s'", inbound.Type)
	}
	if inbound.SubmitPrompt.Text != "build a server" {
		t.Errorf("expected text 'build a server', got '%s'", inbound.SubmitPrompt.Text)
	}
	if inbound.SubmitPrompt.SpaceID != "golang" {
		t.Errorf("expected spaceId 'golang', got '%s'", inbound.SubmitPrompt.SpaceID)
	}
	if inbound.SubmitPrompt.ChatID != "chat-123" {
		t.Errorf("expected chatId 'chat-123', got '%s'", inbound.SubmitPrompt.ChatID)
	}
}

func TestUnmarshalInbound_ReviewDecision(t *testing.T) {
	payload := []byte(`{
		"reviewDecision": {
			"branch": "candidate/123",
			"accepted": true
		}
	}`)

	inbound, err := api.UnmarshalInbound(payload)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if inbound.Type != "review_decision" {
		t.Errorf("expected 'review_decision', got '%s'", inbound.Type)
	}
	if inbound.ReviewDecision.Branch != "candidate/123" {
		t.Errorf("expected branch 'candidate/123', got '%s'", inbound.ReviewDecision.Branch)
	}
	if !inbound.ReviewDecision.Accepted {
		t.Errorf("expected accepted to be true")
	}
}

func TestUnmarshalInbound_ApproveContext(t *testing.T) {
	payload := []byte(`{
		"approveContext": {
			"approved": true,
			"retainedFilePaths": ["src/main.go"],
			"retainedHistoryIndices": [0, 2]
		}
	}`)

	inbound, err := api.UnmarshalInbound(payload)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if inbound.Type != "approve_context" {
		t.Errorf("expected 'approve_context', got '%s'", inbound.Type)
	}
	if !inbound.ApproveContext.Approved {
		t.Errorf("expected approved to be true")
	}
	if len(inbound.ApproveContext.RetainedFilePaths) != 1 || inbound.ApproveContext.RetainedFilePaths[0] != "src/main.go" {
		t.Errorf("expected retained file paths to be parsed correctly")
	}
	if len(inbound.ApproveContext.RetainedHistoryIndices) != 2 || inbound.ApproveContext.RetainedHistoryIndices[0] != 0 || inbound.ApproveContext.RetainedHistoryIndices[1] != 2 {
		t.Errorf("expected retained history indices to be parsed correctly")
	}
}

func TestUnmarshalInbound_TransitionDecision(t *testing.T) {
	payload := []byte(`{
		"transitionDecision": {
			"approved": true
		}
	}`)

	inbound, err := api.UnmarshalInbound(payload)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if inbound.Type != "transition_decision" {
		t.Errorf("expected 'transition_decision', got '%s'", inbound.Type)
	}
	if !inbound.TransitionDecision.Approved {
		t.Errorf("expected approved to be true")
	}
}

func TestUnmarshalInbound_LoadHistory(t *testing.T) {
	payload := []byte(`{
		"loadHistory": {
			"spaceId": "golang",
			"chatId": "chat-123",
			"cursor": "some-uuid-v7",
			"limit": 50
		}
	}`)

	inbound, err := api.UnmarshalInbound(payload)
	if err != nil {
		t.Fatalf("unexpected error unmarshaling: %v", err)
	}

	if inbound.Type != "load_history" {
		t.Errorf("expected 'load_history', got '%s'", inbound.Type)
	}
	if inbound.LoadHistory.SpaceID != "golang" {
		t.Errorf("expected spaceId 'golang', got '%s'", inbound.LoadHistory.SpaceID)
	}
	if inbound.LoadHistory.ChatID != "chat-123" {
		t.Errorf("expected chatId 'chat-123', got '%s'", inbound.LoadHistory.ChatID)
	}
	if inbound.LoadHistory.Cursor != "some-uuid-v7" {
		t.Errorf("expected cursor 'some-uuid-v7', got '%s'", inbound.LoadHistory.Cursor)
	}
	if inbound.LoadHistory.Limit != 50 {
		t.Errorf("expected limit 50, got %d", inbound.LoadHistory.Limit)
	}
}

func TestInvalidInbound(t *testing.T) {
	_, err := api.UnmarshalInbound([]byte(`{"unknownPayload": {}}`))
	if err == nil {
		t.Errorf("expected error for unknown or invalid inbound type")
	}

	_, err = api.UnmarshalInbound([]byte(`{bad json`))
	if err == nil {
		t.Errorf("expected error for malformed json")
	}
}
