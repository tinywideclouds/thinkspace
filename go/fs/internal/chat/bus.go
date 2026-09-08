package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Subscriber defines a listener on the Chat Event Bus.
type Subscriber interface {
	OnEvent(ctx context.Context, thread *Thread, event Event) error
}

// EventBus distributes chat events to registered subscribers.
type EventBus struct {
	mu          sync.RWMutex
	subscribers []Subscriber
}

func NewEventBus() *EventBus {
	return &EventBus{}
}

func (b *EventBus) Subscribe(sub Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = append(b.subscribers, sub)
}

func (b *EventBus) Publish(ctx context.Context, thread *Thread, event Event) error {
	b.mu.RLock()
	subs := b.subscribers
	b.mu.RUnlock()

	var errs []error
	for _, sub := range subs {
		if err := sub.OnEvent(ctx, thread, event); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("event bus publish errors: %v", errs)
	}
	return nil
}

// LedgerSubscriber appends events to the physical conversation.jsonl file.
type LedgerSubscriber struct{}

func NewLedgerSubscriber() *LedgerSubscriber {
	return &LedgerSubscriber{}
}

func (l *LedgerSubscriber) OnEvent(ctx context.Context, thread *Thread, event Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	f, err := os.OpenFile(thread.LedgerPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening ledger: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	return nil
}
