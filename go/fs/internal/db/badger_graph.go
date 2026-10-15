package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

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
	nodeKey := fmt.Appendf(nil, "node:event:%s:%s:%s", c.SpaceID, c.ID, event.ID.String())
	timelineKey := fmt.Appendf(nil, "idx:timeline:%s:%s:%019d:%s", c.SpaceID, c.ID, event.Timestamp.UnixNano(), event.ID.String())

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
			lensKey := fmt.Appendf(nil, "idx:lens:%s:%s:%s:%s", c.SpaceID, c.ID, tag, event.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveDigest(ctx context.Context, c *thinkgraph.Chat, digest thinkgraph.DigestMeta) error {
	nodeKey := fmt.Appendf(nil, "node:digest:%s:%s:%s", c.SpaceID, c.ID, digest.ID.String())
	nodeData, err := json.Marshal(digest)
	if err != nil {
		return fmt.Errorf("failed to marshal digest: %w", err)
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set(nodeKey, nodeData)
	})
}

func (g *BadgerGraph) ClearChatGraph(ctx context.Context, c *thinkgraph.Chat) error {
	prefix := fmt.Appendf(nil, "node:event:%s:%s:", c.SpaceID, c.ID)
	return g.db.DropPrefix(prefix)
}

func (g *BadgerGraph) GetTimeline(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.Event, error) {
	var events []thinkgraph.Event
	prefix := fmt.Appendf(nil, "idx:timeline:%s:%s:", c.SpaceID, c.ID)

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

			nodeItem, err := txn.Get(fmt.Appendf(nil, "node:event:%s:%s:%s", c.SpaceID, c.ID, uuidStr))
			if err != nil {
				continue
			}

			err = nodeItem.Value(func(val []byte) error {
				var ev thinkgraph.Event
				if err := json.Unmarshal(val, &ev); err != nil {
					return err
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

func (g *BadgerGraph) GetLenses(ctx context.Context, c *thinkgraph.Chat, tag string) ([]thinkgraph.Event, error) {
	var events []thinkgraph.Event
	prefix := fmt.Appendf(nil, "idx:lens:%s:%s:%s:", c.SpaceID, c.ID, tag)

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

			nodeItem, err := txn.Get(fmt.Appendf(nil, "node:event:%s:%s:%s", c.SpaceID, c.ID, uuidStr))
			if err != nil {
				continue
			}

			err = nodeItem.Value(func(val []byte) error {
				var ev thinkgraph.Event
				if err := json.Unmarshal(val, &ev); err != nil {
					return err
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
	prefix := fmt.Appendf(nil, "node:digest:%s:%s:", c.SpaceID, c.ID)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			err := it.Item().Value(func(val []byte) error {
				var digest thinkgraph.DigestMeta
				if err := json.Unmarshal(val, &digest); err != nil {
					return err
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

func (g *BadgerGraph) GetSyncState(ctx context.Context, c *thinkgraph.Chat) (*thinkgraph.LedgerManifest, error) {
	var manifest thinkgraph.LedgerManifest
	key := fmt.Appendf(nil, "meta:sync_state:%s:%s", c.SpaceID, c.ID)

	err := g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &manifest)
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
	key := fmt.Appendf(nil, "meta:sync_state:%s:%s", c.SpaceID, c.ID)
	data, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
