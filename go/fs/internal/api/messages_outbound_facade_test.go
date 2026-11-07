package api_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"uuid"
)

func TestMarshalPreviewContext(t *testing.T) {
	ctxData := session.TurnContext{
		SpaceID:             "golang",
		ChatID:              "chat-123",
		PreviewID:           "uuid-v7",
		ManagerSystemPrompt: "Test rules",
		StickyDigests:       []string{"Sticky memory"},
		RecentHistory:       []string{"[prompt] Test"},
		FileContexts: []session.FileContext{
			{Path: "src/main.go", Level: "structural", TokenCount: 100},
		},
	}

	data, err := api.MarshalPreviewContext(ctxData)
	if err != nil {
		t.Fatalf("failed to marshal preview context: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse protojson output: %v", err)
	}

	previewCtxRaw, ok := result["previewContext"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'previewContext' wrapper in JSON output, got: %s", string(data))
	}

	if previewCtxRaw["spaceId"] != "golang" {
		t.Errorf("expected spaceId 'golang', got %v", previewCtxRaw["spaceId"])
	}
	if previewCtxRaw["previewId"] != "uuid-v7" {
		t.Errorf("expected previewId 'uuid-v7', got %v", previewCtxRaw["previewId"])
	}
	if previewCtxRaw["managerSystemPrompt"] != "Test rules" {
		t.Errorf("expected prompt 'Test rules', got %v", previewCtxRaw["managerSystemPrompt"])
	}
}

func TestMarshalTelemetryEvent(t *testing.T) {
	testTime := time.Date(2026, 8, 28, 10, 47, 45, 0, time.UTC)
	eventID := uuid.NewV7()

	domainEvent := telemetry.Event{
		ID:        eventID,
		Type:      "flow_error",
		Message:   "verification failed",
		Summary:   "Flow summary",
		Timestamp: testTime,
		Payload: map[string]any{
			"agent_id": "agent-1",
			"attempt":  2,
		},
	}

	data, err := api.MarshalTelemetryEvent("agent-1", domainEvent)
	if err != nil {
		t.Fatalf("failed to marshal telemetry event: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse protojson output: %v", err)
	}

	payloadRaw, ok := result["telemetryEvent"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'telemetryEvent' wrapper in JSON output, got: %s", string(data))
	}

	if payloadRaw["channel"] != "agent-1" {
		t.Errorf("expected channel 'agent-1', got %v", payloadRaw["channel"])
	}

	eventRaw, ok := payloadRaw["event"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested 'event' struct in JSON output")
	}

	if eventRaw["id"] != eventID.String() {
		t.Errorf("expected id %s, got %v", eventID.String(), eventRaw["id"])
	}
	if eventRaw["type"] != "flow_error" {
		t.Errorf("expected type 'flow_error', got %v", eventRaw["type"])
	}
	if eventRaw["summary"] != "Flow summary" {
		t.Errorf("expected summary, got %v", eventRaw["summary"])
	}

	expectedTime := testTime.Format(time.RFC3339)
	if eventRaw["timestamp"] != expectedTime {
		t.Errorf("expected timestamp %s, got %v", expectedTime, eventRaw["timestamp"])
	}

	payloadJSON, ok := eventRaw["payloadJson"].(string)
	if !ok {
		t.Fatalf("expected payloadJson string, got %v", eventRaw["payloadJson"])
	}
	if !strings.Contains(payloadJSON, `"agent_id":"agent-1"`) {
		t.Errorf("expected JSON payload to contain nested agent_id, got %s", payloadJSON)
	}
}

func TestMarshalSyncHistory(t *testing.T) {
	evID := uuid.NewV7()
	digestID := uuid.NewV7()

	nodes := []thinkgraph.Node{
		{
			Event: thinkgraph.Event{
				ID:        evID,
				Timestamp: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				Type:      thinkgraph.EventPrompt,
				Content:   "Standard message",
				Metadata:  map[string]string{"user": "test"},
			},
		},
	}

	digests := []thinkgraph.DigestMeta{
		{
			ID:       digestID,
			Summary:  "Sticky context",
			Promoted: true,
		},
	}

	focus := thinkgraph.AttentionFocus{
		PrimaryFocus: []string{"test-focus"},
	}

	data, err := api.MarshalSyncHistory(nodes, digests, focus, "cursor-123", true)
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
	if !strings.Contains(jsonStr, `"message"`) {
		t.Errorf("expected oneof payload 'message' to be present, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, "Sticky context") {
		t.Errorf("expected digest summary to be present")
	}
	if !strings.Contains(jsonStr, "test-focus") {
		t.Errorf("expected focus to be present")
	}
	if !strings.Contains(jsonStr, `"nextCursor":"cursor-123"`) {
		t.Errorf("expected nextCursor to be present")
	}
	if !strings.Contains(jsonStr, `"hasMore":true`) {
		t.Errorf("expected hasMore to be true")
	}
}

func TestMarshalSyncSubGraph(t *testing.T) {
	evID := uuid.NewV7()

	nodes := []thinkgraph.Node{
		{
			Event: thinkgraph.Event{
				ID:        evID,
				Timestamp: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				Type:      thinkgraph.EventModel,
				Content:   "SubGraph message",
			},
		},
	}

	data, err := api.MarshalSyncSubGraph(nodes)
	if err != nil {
		t.Fatalf("failed to marshal sync subgraph: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, "syncSubGraph") {
		t.Errorf("expected wrapper 'syncSubGraph', got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, evID.String()) {
		t.Errorf("expected event ID %s to be present", evID.String())
	}
	if !strings.Contains(jsonStr, `"message"`) {
		t.Errorf("expected oneof payload 'message' to be present, got: %s", jsonStr)
	}
}

func TestMarshalRequestTransition(t *testing.T) {
	data, err := api.MarshalRequestTransition("golang", "need to compile")
	if err != nil {
		t.Fatalf("failed to marshal request transition: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse protojson output: %v", err)
	}

	reqRaw, ok := result["requestTransition"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected 'requestTransition' wrapper in JSON output, got: %s", string(data))
	}

	if reqRaw["targetThinkState"] != "golang" {
		t.Errorf("expected targetThinkState 'golang', got %v", reqRaw["targetThinkState"])
	}
	if reqRaw["reasoning"] != "need to compile" {
		t.Errorf("expected reasoning 'need to compile', got %v", reqRaw["reasoning"])
	}
}
