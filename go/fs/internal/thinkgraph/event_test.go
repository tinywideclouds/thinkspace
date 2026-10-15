package thinkgraph_test

import (
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestEvent_TimeOrderingV7(t *testing.T) {
	firstEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, "Hello")
	time.Sleep(2 * time.Millisecond)
	secondEvent := thinkgraph.NewEvent(thinkgraph.EventTool, "Doing work")

	if !(firstEvent.ID.String() < secondEvent.ID.String()) {
		t.Errorf("Expected firstEvent ID to sort before secondEvent ID. Got: %s >= %s", firstEvent.ID, secondEvent.ID)
	}
}
