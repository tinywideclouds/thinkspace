package api_test

import (
	"testing"
	"time"
	"uuid"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestProjectTimeline(t *testing.T) {
	ev1 := uuid.NewV7()
	ev2 := uuid.NewV7()

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
	}

	pbEvents := api.ProjectTimeline(nodes)

	if len(pbEvents) != 2 {
		t.Fatalf("expected 2 projected events, got %d", len(pbEvents))
	}

	// Verify standard message mapping
	msgPayload, ok := pbEvents[0].Payload.(*pb.GraphEvent_Message)
	if !ok {
		t.Fatalf("expected first event to map to Message payload")
	}
	if msgPayload.Message.Text != "Hello from user" {
		t.Errorf("expected text 'Hello from user', got %s", msgPayload.Message.Text)
	}

	// Verify strict candidate proposal unpacking (Data + MaterializedState)
	propPayload, ok := pbEvents[1].Payload.(*pb.GraphEvent_CandidateProposal)
	if !ok {
		t.Fatalf("expected second event to map to CandidateProposal payload")
	}
	if propPayload.CandidateProposal.CandidateId != "c-123" {
		t.Errorf("expected candidate_id 'c-123', got %s", propPayload.CandidateProposal.CandidateId)
	}
	if propPayload.CandidateProposal.DiffSnippet != "+ added line" {
		t.Errorf("expected diff snippet '+ added line', got %s", propPayload.CandidateProposal.DiffSnippet)
	}
}
