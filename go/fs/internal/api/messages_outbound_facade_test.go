package api_test

import (
	"encoding/json"
	"testing"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestMarshalChatStream(t *testing.T) {
	chunk := workspace.NodeChunk{
		NodeID: uuid.NewV7(),
		Text:   "chunk test",
	}

	data, err := api.MarshalChatStream(chunk)
	if err != nil {
		t.Fatalf("failed to marshal chat stream: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	chatStream, ok := result["chatStream"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapper 'chatStream'")
	}

	if chatStream["nodeId"] != chunk.NodeID.String() {
		t.Errorf("expected node_id %s, got %v", chunk.NodeID.String(), chatStream["nodeId"])
	}
	if chatStream["text"] != "chunk test" {
		t.Errorf("expected text 'chunk test', got %v", chatStream["text"])
	}
}

func TestMarshalStreamedGraphEvent(t *testing.T) {
	eventID := uuid.NewV7()
	node := thinkgraph.Node{
		Event: thinkgraph.Event{
			ID:        eventID,
			Timestamp: time.Now().UTC(),
			Type:      thinkgraph.EventModel,
			Content:   "Durable payload",
		},
	}

	coordinate := api.ProjectionCoordinate{SpaceID: "space-1", ChatID: "chat-1", ProjectionID: "main"}
	data, err := api.MarshalStreamedGraphEvent(node, coordinate)
	if err != nil {
		t.Fatalf("failed to marshal streamed graph event: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	streamedEvent, ok := result["streamedGraphEvent"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapper 'streamedGraphEvent'")
	}

	graphEvent, ok := streamedEvent["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'event' object")
	}

	if graphEvent["id"] != eventID.String() {
		t.Errorf("expected node ID %s, got %v", eventID.String(), graphEvent["id"])
	}
	if streamedEvent["spaceId"] != "space-1" {
		t.Errorf("expected spaceId 'space-1', got %v", streamedEvent["spaceId"])
	}
	if streamedEvent["projectionId"] != "main" {
		t.Errorf("expected projectionId 'main', got %v", streamedEvent["projectionId"])
	}
}

func TestMarshalPreviewContext(t *testing.T) {
	ctxData := session.TurnContext{
		SpaceID:             "golang",
		ChatID:              "chat-123",
		PreviewID:           "uuid-v7",
		ManagerSystemPrompt: "Test rules",
		LongTermMemory:      []string{"Sticky memory"},
		RecentHistory:       []string{"[prompt] Test"},
		FileContexts: []session.FileContext{
			{Path: "src/main.go", Level: "structural", TokenCount: 100},
		},
	}

	data, err := api.MarshalPreviewContext(ctxData)
	if err != nil {
		t.Fatalf("failed to marshal preview context: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	previewContext, ok := result["previewContext"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'previewContext' wrapper")
	}

	if previewContext["spaceId"] != "golang" {
		t.Errorf("expected spaceId 'golang', got %v", previewContext["spaceId"])
	}
	if previewContext["previewId"] != "uuid-v7" {
		t.Errorf("expected previewId 'uuid-v7', got %v", previewContext["previewId"])
	}
	if previewContext["managerSystemPrompt"] != "Test rules" {
		t.Errorf("expected prompt 'Test rules', got %v", previewContext["managerSystemPrompt"])
	}
}

func TestMarshalTelemetryEvent_AgentPayload(t *testing.T) {
	testTime := time.Date(2026, 8, 28, 10, 47, 45, 0, time.UTC)
	eventID := uuid.NewV7()

	domainEvent := telemetry.Event{
		ID:        eventID,
		Type:      "flow_error",
		Message:   "verification failed",
		Timestamp: testTime,
		Payload: flows.AgentTelemetry{
			AgentID: "agent-1",
			Attempt: 2,
		},
	}

	data, err := api.MarshalTelemetryEvent("agent-1", domainEvent)
	if err != nil {
		t.Fatalf("failed to marshal telemetry event: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	payloadRaw, ok := result["telemetryEvent"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'telemetryEvent' wrapper")
	}

	if payloadRaw["channel"] != "agent-1" {
		t.Errorf("expected channel 'agent-1', got %v", payloadRaw["channel"])
	}

	eventRaw, ok := payloadRaw["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'event' struct")
	}

	if eventRaw["id"] != eventID.String() {
		t.Errorf("expected id %s, got %v", eventID.String(), eventRaw["id"])
	}
	if eventRaw["type"] != "flow_error" {
		t.Errorf("expected type 'flow_error', got %v", eventRaw["type"])
	}
	// Verify behavioral Summary() fallback populated the event summary automatically
	if eventRaw["summary"] != "Agent agent-1 [attempt 2]" {
		t.Errorf("expected derived summary 'Agent agent-1 [attempt 2]', got %v", eventRaw["summary"])
	}

	expectedTime := testTime.Format(time.RFC3339)
	if eventRaw["timestamp"] != expectedTime {
		t.Errorf("expected timestamp %s, got %v", expectedTime, eventRaw["timestamp"])
	}

	agentPayload, ok := eventRaw["agentPayload"].(map[string]any)
	if !ok {
		t.Fatalf("expected agentPayload oneof wrapper")
	}
	if agentPayload["agentId"] != "agent-1" {
		t.Errorf("expected agentId 'agent-1', got %v", agentPayload["agentId"])
	}
}

func TestMarshalTelemetryEvent_FlowPayload(t *testing.T) {
	testTime := time.Date(2026, 8, 28, 10, 47, 45, 0, time.UTC)
	eventID := uuid.NewV7()

	domainEvent := telemetry.Event{
		ID:        eventID,
		Type:      "flow_start",
		Message:   "Starting Flow",
		Summary:   "Explicit Flow Summary",
		Timestamp: testTime,
		Payload: flows.FlowTelemetry{
			FlowID:     "flow-123",
			AgentCount: 4,
		},
	}

	data, err := api.MarshalTelemetryEvent("manager", domainEvent)
	if err != nil {
		t.Fatalf("failed to marshal telemetry event: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	payloadRaw, ok := result["telemetryEvent"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'telemetryEvent' wrapper")
	}

	eventRaw, ok := payloadRaw["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'event' struct")
	}

	if eventRaw["summary"] != "Explicit Flow Summary" {
		t.Errorf("expected explicit summary 'Explicit Flow Summary', got %v", eventRaw["summary"])
	}

	flowPayload, ok := eventRaw["flowPayload"].(map[string]any)
	if !ok {
		t.Fatalf("expected flowPayload oneof wrapper")
	}
	if flowPayload["flowId"] != "flow-123" {
		t.Errorf("expected flowId 'flow-123', got %v", flowPayload["flowId"])
	}
	if float64(flowPayload["agentCount"].(float64)) != float64(4) {
		t.Errorf("expected agentCount 4, got %v", flowPayload["agentCount"])
	}
}

func TestMarshalSyncHistory(t *testing.T) {
	eventID := uuid.NewV7()
	digestID := uuid.NewV7()

	nodes := []thinkgraph.Node{
		{
			Event: thinkgraph.Event{
				ID:        eventID,
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
			IsSticky: true,
		},
	}

	focus := thinkgraph.AttentionFocus{
		PrimaryFocus: []string{"test-focus"},
	}

	coordinate := api.ProjectionCoordinate{SpaceID: "space-1", ChatID: "chat-1", ProjectionID: "main"}
	data, err := api.MarshalSyncHistory(nodes, digests, focus, "cursor-123", true, coordinate)
	if err != nil {
		t.Fatalf("failed to marshal sync history: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	syncHistory, ok := result["syncHistory"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapper 'syncHistory'")
	}

	recentEvents, ok := syncHistory["recentEvents"].([]any)
	if !ok || len(recentEvents) == 0 {
		t.Fatalf("expected recentEvents array")
	}

	firstEvent := recentEvents[0].(map[string]any)
	if firstEvent["id"] != eventID.String() {
		t.Errorf("expected event ID %s, got %v", eventID.String(), firstEvent["id"])
	}
	if _, hasMessage := firstEvent["message"]; !hasMessage {
		t.Errorf("expected oneof payload 'message' to be present")
	}

	focusData, ok := syncHistory["focus"].(map[string]any)
	if !ok {
		t.Fatalf("expected focus object")
	}

	primaryFocus := focusData["primaryFocus"].([]any)
	if len(primaryFocus) == 0 || primaryFocus[0] != "test-focus" {
		t.Errorf("expected primaryFocus 'test-focus'")
	}

	if syncHistory["nextCursor"] != "cursor-123" {
		t.Errorf("expected nextCursor 'cursor-123', got %v", syncHistory["nextCursor"])
	}
	if syncHistory["hasMore"] != true {
		t.Errorf("expected hasMore to be true")
	}
	if syncHistory["projectionId"] != "main" {
		t.Errorf("expected projectionId 'main', got %v", syncHistory["projectionId"])
	}
}

func TestMarshalSyncSubGraph(t *testing.T) {
	eventID := uuid.NewV7()

	nodes := []thinkgraph.Node{
		{
			Event: thinkgraph.Event{
				ID:        eventID,
				Timestamp: time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
				Type:      thinkgraph.EventModel,
				Content:   "SubGraph message",
			},
		},
	}

	coordinate := api.ProjectionCoordinate{SpaceID: "space-1", ChatID: "chat-1", ProjectionID: "flow-123"}
	data, err := api.MarshalSyncSubGraph(nodes, coordinate)
	if err != nil {
		t.Fatalf("failed to marshal sync subgraph: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	syncSubGraph, ok := result["syncSubGraph"].(map[string]any)
	if !ok {
		t.Fatalf("expected wrapper 'syncSubGraph'")
	}

	events, ok := syncSubGraph["events"].([]any)
	if !ok || len(events) == 0 {
		t.Fatalf("expected events array")
	}

	firstEvent := events[0].(map[string]any)
	if firstEvent["id"] != eventID.String() {
		t.Errorf("expected event ID %s, got %v", eventID.String(), firstEvent["id"])
	}
	if _, hasMessage := firstEvent["message"]; !hasMessage {
		t.Errorf("expected oneof payload 'message' to be present")
	}

	if syncSubGraph["projectionId"] != "flow-123" {
		t.Errorf("expected projectionId 'flow-123', got %v", syncSubGraph["projectionId"])
	}
}
