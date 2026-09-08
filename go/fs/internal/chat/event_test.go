package chat_test

import (
	"testing"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
)

func TestEvent_TimeOrderingV7(t *testing.T) {
	firstEvent := chat.NewEvent(chat.EventPrompt, "Hello")
	time.Sleep(2 * time.Millisecond)
	secondEvent := chat.NewEvent(chat.EventTool, "Doing work")

	if !(firstEvent.ID.String() < secondEvent.ID.String()) {
		t.Errorf("Expected firstEvent ID to sort before secondEvent ID. Got: %s >= %s", firstEvent.ID, secondEvent.ID)
	}
}
