package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Subscriber interface {
	OnEvent(ctx context.Context, c *Chat, event Event) error
}

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

func (b *EventBus) Publish(ctx context.Context, c *Chat, event Event) error {
	b.mu.RLock()
	subs := b.subscribers
	b.mu.RUnlock()

	var errs []error
	for _, sub := range subs {
		if err := sub.OnEvent(ctx, c, event); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("event bus publish errors: %v", errs)
	}
	return nil
}

type LedgerSubscriber struct {
	MaxEventsPerShard int
	workspaceRoot     string
}

func NewLedgerSubscriber(maxEvents int, workspaceRoot string) *LedgerSubscriber {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	return &LedgerSubscriber{MaxEventsPerShard: maxEvents, workspaceRoot: workspaceRoot}
}

func (l *LedgerSubscriber) OnEvent(ctx context.Context, c *Chat, event Event) error {
	// Discrete digests are persisted to digests/*.md via DigestSubscriber
	if event.Type == EventDigest {
		return nil
	}

	ledgerDir := filepath.Join(l.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")
	manifest, err := LoadOrRebuildManifest(ledgerDir)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if manifest.EventCount >= l.MaxEventsPerShard {
		var currentNum int
		fmt.Sscanf(manifest.ActiveShard, "%d.jsonl", &currentNum)
		manifest.ActiveShard = fmt.Sprintf("%05d.jsonl", currentNum+1)
		manifest.EventCount = 0
	}

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	logsDir := filepath.Join(ledgerDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("creating logs directory: %w", err)
	}

	shardPath := filepath.Join(logsDir, manifest.ActiveShard)
	f, err := os.OpenFile(shardPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening ledger shard: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("writing event: %w", err)
	}

	manifest.EventCount++
	manifest.TotalEvents++
	return manifest.Save(ledgerDir)
}
