package db

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// --- Graph Taxonomy Registry ---

const (
	nsEvent     = "node:event"
	nsTimeline  = "idx:timeline"
	nsLens      = "idx:lens"
	nsDigest    = "node:digest"
	nsSyncState = "meta:sync_state"
)

// Collections require a trailing colon delimiter during prefix scans to prevent
// cross-contamination (e.g. wiping "chat-11" when targeting "chat-1").
var chatCollections = []string{
	nsEvent,
	nsTimeline,
	nsLens,
	nsDigest,
}

// Singletons are exact 1:1 key-value pairs for a chat.
var chatSingletons = []string{
	nsSyncState,
}

// -------------------------------

type BadgerGraph struct {
	db *badger.DB
}

func NewBadgerGraph(opts badger.Options) (*BadgerGraph, error) {
	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger db: %w", err)
	}
	return &BadgerGraph{db: db}, nil
}

func (g *BadgerGraph) Close() error {
	return g.db.Close()
}

func (g *BadgerGraph) AppendEvent(ctx context.Context, c *thinkgraph.Chat, event thinkgraph.Event) error {
	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, event.ID.String())
	timelineKey := fmt.Appendf(nil, "%s:%s:%s:%019d:%s", nsTimeline, c.SpaceID, c.ID, event.Timestamp.UnixNano(), event.ID.String())

	nodeData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	return g.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nodeKey, nodeData); err != nil {
			return err
		}
		if err := txn.Set(timelineKey, nil); err != nil {
			return err
		}
		for _, tag := range event.Tags {
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.SpaceID, c.ID, tag, event.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveDigest(ctx context.Context, c *thinkgraph.Chat, digest thinkgraph.DigestMeta) error {
	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.SpaceID, c.ID, digest.ID.String())
	nodeData, err := json.Marshal(digest)
	if err != nil {
		return fmt.Errorf("failed to marshal digest: %w", err)
	}

	slog.Info("BadgerDB saving digest", "key", string(nodeKey))

	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set(nodeKey, nodeData)
	})
}

func (g *BadgerGraph) ClearChatGraph(ctx context.Context, c *thinkgraph.Chat) error {
	// Safely drop all O(N) collections associated with the chat using strict trailing colons
	for _, ns := range chatCollections {
		prefix := fmt.Appendf(nil, "%s:%s:%s:", ns, c.SpaceID, c.ID)
		if err := g.db.DropPrefix(prefix); err != nil {
			return fmt.Errorf("failed to drop prefix %s: %w", string(prefix), err)
		}
	}

	// Safely delete all exact 1:1 singletons
	return g.db.Update(func(txn *badger.Txn) error {
		for _, ns := range chatSingletons {
			key := fmt.Appendf(nil, "%s:%s:%s", ns, c.SpaceID, c.ID)
			if err := txn.Delete(key); err != nil && err != badger.ErrKeyNotFound {
				return fmt.Errorf("failed to delete singleton %s: %w", string(key), err)
			}
		}
		return nil
	})
}

func (g *BadgerGraph) GetTimeline(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.Event, error) {
	var events []thinkgraph.Event
	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsTimeline, c.SpaceID, c.ID)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 6 {
				continue
			}
			uuidStr := parts[5]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, uuidStr)
			nodeItem, err := txn.Get(nodeKey)
			if err != nil {
				continue
			}

			err = nodeItem.Value(func(val []byte) error {
				var ev thinkgraph.Event
				if err := json.Unmarshal(val, &ev); err != nil {
					return fmt.Errorf("failed to unmarshal event %s: %w", uuidStr, err)
				}
				events = append(events, ev)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return events, err
}

func (g *BadgerGraph) WalkTimelineReverse(ctx context.Context, c *thinkgraph.Chat, fn func(thinkgraph.Event) bool) error {
	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsTimeline, c.SpaceID, c.ID)

	return g.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		seekPrefix := append(prefix, 0xFF)

		for it.Seek(seekPrefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 6 {
				continue
			}
			uuidStr := parts[5]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, uuidStr)
			nodeItem, err := txn.Get(nodeKey)
			if err != nil {
				continue
			}

			var ev thinkgraph.Event
			err = nodeItem.Value(func(val []byte) error {
				if err := json.Unmarshal(val, &ev); err != nil {
					return fmt.Errorf("failed to unmarshal reverse event %s: %w", uuidStr, err)
				}
				return nil
			})
			if err != nil {
				return err
			}

			if !fn(ev) {
				break
			}
		}
		return nil
	})
}

func (g *BadgerGraph) GetLenses(ctx context.Context, c *thinkgraph.Chat, tag string) ([]thinkgraph.Event, error) {
	var events []thinkgraph.Event
	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsLens, c.SpaceID, c.ID, tag)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 6 {
				continue
			}
			uuidStr := parts[5]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, uuidStr)
			nodeItem, err := txn.Get(nodeKey)
			if err != nil {
				continue
			}

			err = nodeItem.Value(func(val []byte) error {
				var ev thinkgraph.Event
				if err := json.Unmarshal(val, &ev); err != nil {
					return fmt.Errorf("failed to unmarshal lens event %s: %w", uuidStr, err)
				}
				events = append(events, ev)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return events, err
}

func (g *BadgerGraph) GetStickyDigests(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.DigestMeta, error) {
	var digests []thinkgraph.DigestMeta
	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.SpaceID, c.ID)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				var digest thinkgraph.DigestMeta
				if err := json.Unmarshal(val, &digest); err != nil {
					return fmt.Errorf("failed to unmarshal sticky digest: %w (raw: %s)", err, string(val))
				}
				if digest.IsSticky {
					digests = append(digests, digest)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return digests, err
}

func (g *BadgerGraph) GetDigests(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.DigestMeta, error) {
	var digests []thinkgraph.DigestMeta
	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.SpaceID, c.ID)

	slog.Info("BadgerDB querying digests", "prefix", string(prefix))

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				var digest thinkgraph.DigestMeta
				if err := json.Unmarshal(val, &digest); err != nil {
					return fmt.Errorf("failed to unmarshal digest: %w (raw: %s)", err, string(val))
				}
				digests = append(digests, digest)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return digests, err
}

func (g *BadgerGraph) GetEvents(ctx context.Context, c *thinkgraph.Chat, ids []string) ([]thinkgraph.Event, error) {
	var events []thinkgraph.Event

	err := g.db.View(func(txn *badger.Txn) error {
		for _, id := range ids {
			key := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, id)
			item, err := txn.Get(key)
			if err != nil {
				continue
			}

			err = item.Value(func(val []byte) error {
				var ev thinkgraph.Event
				if err := json.Unmarshal(val, &ev); err != nil {
					return fmt.Errorf("failed to unmarshal direct event %s: %w", id, err)
				}
				events = append(events, ev)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	return events, err
}

func (g *BadgerGraph) GetSyncState(ctx context.Context, c *thinkgraph.Chat) (*thinkgraph.LedgerManifest, error) {
	var manifest thinkgraph.LedgerManifest
	key := fmt.Appendf(nil, "%s:%s:%s", nsSyncState, c.SpaceID, c.ID)

	err := g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			if err := json.Unmarshal(val, &manifest); err != nil {
				return fmt.Errorf("failed to unmarshal sync state: %w", err)
			}
			return nil
		})
	})

	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (g *BadgerGraph) SetSyncState(ctx context.Context, c *thinkgraph.Chat, manifest thinkgraph.LedgerManifest) error {
	key := fmt.Appendf(nil, "%s:%s:%s", nsSyncState, c.SpaceID, c.ID)
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal sync state: %w", err)
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
