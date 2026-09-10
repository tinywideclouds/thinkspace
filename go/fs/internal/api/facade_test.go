package api_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
)

func TestEventFacade_UnmarshalInbound_SelectStrategy(t *testing.T) {
	facade := api.NewEventFacade()

	tests := []struct {
		name         string
		protoEnumVal int
		expected     session.DelegationStrategy
	}{
		{"UNSPECIFIED fallback", 0, session.StrategyManual},
		{"SKIP mapping", 1, session.StrategySkip},
		{"MANUAL mapping", 2, session.StrategyManual},
		{"REVIEW mapping", 3, session.StrategyReview},
		{"REFINE mapping", 4, session.StrategyRefine},
		{"UNKNOWN fallback", 99, session.StrategyManual},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct the raw JSON that protojson expects for the WSEvent oneof
			payload := `{"selectStrategy": {"strategyId": ` + string(rune('0'+tt.protoEnumVal)) + `}}`
			if tt.protoEnumVal > 9 {
				payload = `{"selectStrategy": {"strategyId": 99}}`
			}

			inbound, err := facade.UnmarshalInbound([]byte(payload))
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

func TestEventFacade_UnmarshalInbound_SubmitPrompt(t *testing.T) {
	facade := api.NewEventFacade()
	payload := []byte(`{
		"submitPrompt": {
			"text": "build a server",
			"spaceId": "golang",
			"chatId": "chat-123"
		}
	}`)

	inbound, err := facade.UnmarshalInbound(payload)
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

func TestEventFacade_UnmarshalInbound_ReviewDecision(t *testing.T) {
	facade := api.NewEventFacade()
	payload := []byte(`{
		"reviewDecision": {
			"branch": "candidate/123",
			"accepted": true
		}
	}`)

	inbound, err := facade.UnmarshalInbound(payload)
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

func TestEventFacade_MarshalFlowEvent(t *testing.T) {
	facade := api.NewEventFacade()
	testTime := time.Date(2026, 8, 28, 10, 47, 45, 0, time.UTC)

	domainEvent := flows.FlowEvent{
		FlowID:      "flow-123",
		Type:        flows.FlowError,
		Timestamp:   testTime,
		AgentID:     "agent-1",
		Attempt:     2,
		Trace:       "compile error: undefined variable",
		CandidateID: "cand-456",
		Passed:      false,
	}

	data, err := facade.MarshalFlowEvent(domainEvent)
	if err != nil {
		t.Fatalf("failed to marshal flow event: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse protojson output: %v", err)
	}

	flowEventRaw, ok := result["flowEvent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'flowEvent' wrapper in JSON output, got: %s", string(data))
	}

	if flowEventRaw["flowId"] != "flow-123" {
		t.Errorf("expected flowId 'flow-123', got %v", flowEventRaw["flowId"])
	}
	if flowEventRaw["type"] != string(flows.FlowError) {
		t.Errorf("expected type 'flow_error', got %v", flowEventRaw["type"])
	}
	if flowEventRaw["trace"] != "compile error: undefined variable" {
		t.Errorf("expected trace payload, got %v", flowEventRaw["trace"])
	}

	expectedTime := testTime.Format(time.RFC3339)
	if flowEventRaw["timestamp"] != expectedTime {
		t.Errorf("expected timestamp %s, got %v", expectedTime, flowEventRaw["timestamp"])
	}
}

func TestEventFacade_MarshalSyncHistory(t *testing.T) {
	facade := api.NewEventFacade()

	evID := uuid.NewV7()
	digestID := uuid.NewV7()

	events := []chat.Event{
		{
			ID:        evID,
			Timestamp: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
			Type:      chat.EventPrompt,
			Content:   "Hello",
			Metadata:  map[string]string{"user": "test"},
		},
	}

	digests := map[uuid.UUID]chat.DigestMeta{
		digestID: {
			ID:       digestID,
			Summary:  "Sticky context",
			IsSticky: true,
		},
	}

	data, err := facade.MarshalSyncHistory(events, digests)
	if err != nil {
		t.Fatalf("failed to marshal sync history: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, "syncHistory") {
		t.Errorf("expected wrapper 'syncHistory', got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, evID.String()) {
		t.Errorf("expected event ID %s to be present", evID.String())
	}
	if !strings.Contains(jsonStr, "Sticky context") {
		t.Errorf("expected digest summary to be present")
	}
}

func TestEventFacade_InvalidInbound(t *testing.T) {
	facade := api.NewEventFacade()

	// Missing wrapper
	_, err := facade.UnmarshalInbound([]byte(`{"unknownPayload": {}}`))
	if err == nil {
		t.Errorf("expected error for unknown or invalid inbound type")
	}

	// Malformed JSON
	_, err = facade.UnmarshalInbound([]byte(`{bad json`))
	if err == nil {
		t.Errorf("expected error for malformed json")
	}
}
