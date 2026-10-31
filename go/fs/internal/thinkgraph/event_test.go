package thinkgraph_test

import (
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestEvent_TimeOrderingV7(t *testing.T) {
	firstEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Hello")
	time.Sleep(2 * time.Millisecond)
	secondEvent := thinkgraph.NewEvent(thinkgraph.EventModel, "Doing work")

	if !(firstEvent.ID.String() < secondEvent.ID.String()) {
		t.Errorf("Expected firstEvent ID to sort before secondEvent ID. Got: %s >= %s", firstEvent.ID, secondEvent.ID)
	}
}

func TestEvent_BranchInitialization(t *testing.T) {
	ev := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Branch test")
	if ev.BranchID != "" {
		t.Errorf("Expected new event branch ID to be empty by default, got %s", ev.BranchID)
	}
}
