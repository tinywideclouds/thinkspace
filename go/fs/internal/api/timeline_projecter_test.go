package api_test

import (
	"encoding/json"
	"testing"
	"time"
	"uuid"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestProjectTimeline(t *testing.T) {
	ev1 := uuid.NewV7()
	ev2 := uuid.NewV7()
	ev3 := uuid.NewV7()

	nodes := []thinkgraph.Node{
		{
			Event: thinkgraph.Event{
				ID:        ev1,
				Timestamp: time.Now(),
				Type:      thinkgraph.EventPrompt,
				Content:   "Hello from user",
			},
		},
		{
			Event: thinkgraph.Event{
				ID:        ev2,
				Timestamp: time.Now(),
				Type:      workspace.EventCandidateProposal,
				Data:      []byte(`{"candidate_id":"c-123","description":"A proposal","files":["main.go"]}`),
			},
			MaterializedState: []byte(`{"diff":"+ added line"}`),
		},
		{
			Event: thinkgraph.Event{
				ID:        ev3,
				Timestamp: time.Now(),
				Type:      flows.EventFlowLifecycle,
				Data:      []byte(`{"flow_id":"` + ev3.String() + `","task_id":"task-123","task_prompt":"Architect backend"}`),
			},
			MaterializedState: []byte(`{"status":1,"agents":[{"agent_id":"ag-1","instruction":"Write tests"}]}`),
		},
	}

	pbEvents := api.ProjectTimeline(nodes)

	if len(pbEvents) != 3 {
		t.Fatalf("expected 3 projected events, got %d", len(pbEvents))
	}

	msgPayload, ok := pbEvents[0].Payload.(*pb.GraphEvent_Message)
	if !ok {
		t.Fatalf("expected first event to map to Message payload")
	}
	if msgPayload.Message.Text != "Hello from user" {
		t.Errorf("expected text 'Hello from user', got %s", msgPayload.Message.Text)
	}

	propPayload, ok := pbEvents[1].Payload.(*pb.GraphEvent_CandidateProposal)
	if !ok {
		t.Fatalf("expected second event to map to CandidateProposal payload")
	}
	if propPayload.CandidateProposal.CandidateId != "c-123" {
		t.Errorf("expected candidate_id 'c-123', got %s", propPayload.CandidateProposal.CandidateId)
	}

	flowPayload, ok := pbEvents[2].Payload.(*pb.GraphEvent_FlowExecution)
	if !ok {
		t.Fatalf("expected third event to map to FlowExecution payload")
	}
	if flowPayload.FlowExecution.FlowId != ev3.String() {
		t.Errorf("expected flow_id %s, got %s", ev3.String(), flowPayload.FlowExecution.FlowId)
	}
	if flowPayload.FlowExecution.Status != pb.FlowStatus_FLOW_STATUS_RUNNING {
		t.Errorf("expected status RUNNING, got %v", flowPayload.FlowExecution.Status)
	}
	if len(flowPayload.FlowExecution.Agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(flowPayload.FlowExecution.Agents))
	}
}

// TestProjectTimeline_FlowStatusMapping exhaustively ensures that valid domain states
// successfully map to non-UNSPECIFIED Protobuf statuses to prevent cross-package drift.
func TestProjectTimeline_FlowStatusMapping(t *testing.T) {
	testCases := []struct {
		domainStatus   flows.FlowStatus
		expectedStatus pb.FlowStatus
	}{
		{flows.FlowStatusRunning, pb.FlowStatus_FLOW_STATUS_RUNNING},
		{flows.FlowStatusCompleted, pb.FlowStatus_FLOW_STATUS_COMPLETED},
		{flows.FlowStatusFailed, pb.FlowStatus_FLOW_STATUS_FAILED},
	}

	for _, tc := range testCases {
		state := flows.FlowExecutionState{
			Status: tc.domainStatus,
		}
		stateBytes, _ := json.Marshal(state)

		nodes := []thinkgraph.Node{
			{
				Event: thinkgraph.Event{
					ID:   uuid.NewV7(),
					Type: flows.EventFlowLifecycle,
					Data: []byte(`{"flow_id":"f-123"}`),
				},
				MaterializedState: stateBytes,
			},
		}

		pbEvents := api.ProjectTimeline(nodes)
		if len(pbEvents) != 1 {
			t.Fatalf("failed to project node for status %v", tc.domainStatus)
		}

		payload, ok := pbEvents[0].Payload.(*pb.GraphEvent_FlowExecution)
		if !ok {
			t.Fatalf("failed to cast projected node to FlowExecution")
		}

		if payload.FlowExecution.Status != tc.expectedStatus {
			t.Errorf("Expected domain status %v to map to %v, got %v",
				tc.domainStatus, tc.expectedStatus, payload.FlowExecution.Status)
		}
	}
}
