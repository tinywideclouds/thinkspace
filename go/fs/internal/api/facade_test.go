package api_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

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

	// We use standard encoding/json here just to inspect the final raw string
	// outputted by protojson, ensuring the payload keys are formatted correctly.
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to parse protojson output: %v", err)
	}

	// WSEvent wrapper should use the oneof field name "flowEvent"
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
