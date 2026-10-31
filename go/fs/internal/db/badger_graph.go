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

const (
	nsEvent        = "node:event"
	nsTimeline     = "idx:timeline"
	nsLens         = "idx:lens"
	nsDigest       = "node:digest"
	nsSpaceContext = "node:space_context"
	nsSyncState    = "meta:sync_state"
)

var chatCollections = []string{
	nsEvent, nsTimeline, nsLens, nsDigest, nsSpaceContext,
}
var chatSingletons = []string{nsSyncState}

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

func scanPrefix[T any](txn *badger.Txn, prefix []byte, filter func(T) bool) ([]T, error) {
	var results []T
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		_ = it.Item().Value(func(val []byte) error {
			var item T
			if json.Unmarshal(val, &item) == nil {
				if tombstonable, ok := any(item).(interface{ IsTombstoned() bool }); ok && tombstonable.IsTombstoned() {
					return nil
				}
				if filter == nil || filter(item) {
					results = append(results, item)
				}
			}
			return nil
		})
	}
	return results, nil
}

func (g *BadgerGraph) AppendNode(ctx context.Context, c *thinkgraph.Chat, node thinkgraph.Node) error {
	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, node.ID.String())

	branchID := node.BranchID
	if branchID == "" {
		branchID = "main"
	}

	timelineKey := fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.SpaceID, c.ID, branchID, node.Timestamp.UnixNano(), node.ID.String())

	nodeData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	slog.DebugContext(ctx, "BadgerDB appending node", "key", string(nodeKey))

	return g.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nodeKey, nodeData); err != nil {
			return err
		}
		if err := txn.Set(timelineKey, nil); err != nil {
			return err
		}
		for _, tag := range node.Tags {
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.SpaceID, c.ID, tag, node.ID.String())
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

	slog.InfoContext(ctx, "BadgerDB saving digest", "key", string(nodeKey), "promoted", digest.Promoted)

	return g.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nodeKey, nodeData); err != nil {
			return err
		}
		for _, tag := range digest.Tags {
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.SpaceID, c.ID, tag, digest.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveSpaceContext(ctx context.Context, c *thinkgraph.Chat, spaceCtx thinkgraph.SpaceContext) error {
	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.SpaceID, c.ID, spaceCtx.ID.String())
	nodeData, err := json.Marshal(spaceCtx)
	if err != nil {
		return fmt.Errorf("failed to marshal space context: %w", err)
	}

	slog.InfoContext(ctx, "BadgerDB saving space context", "key", string(nodeKey))

	return g.db.Update(func(txn *badger.Txn) error {
		if err := txn.Set(nodeKey, nodeData); err != nil {
			return err
		}
		for _, tag := range spaceCtx.Tags {
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.SpaceID, c.ID, tag, spaceCtx.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveMutation(ctx context.Context, c *thinkgraph.Chat, mutation thinkgraph.MutationEvent) error {
	slog.InfoContext(ctx, "BadgerDB applying mutation", "type", mutation.Type, "target_id", mutation.TargetID.String())

	return g.db.Update(func(txn *badger.Txn) error {
		evKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, mutation.TargetID.String())
		if item, err := txn.Get(evKey); err == nil {
			return item.Value(func(val []byte) error {
				var node thinkgraph.Node
				if json.Unmarshal(val, &node) != nil {
					return nil
				}

				if mutation.Type == thinkgraph.MutationTombstone {
					node.Tombstoned = true
				}
				if mutation.Type == thinkgraph.MutationRestore {
					node.Tombstoned = false
				}
				if mutation.Type == thinkgraph.MutationUpdate {
					if mutation.Payload != "" {
						node.Content = mutation.Payload
					}
					if len(mutation.Data) > 0 {
						node.Data = mutation.Data
					}
				}

				data, _ := json.Marshal(node)
				return txn.Set(evKey, data)
			})
		}

		dKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.SpaceID, c.ID, mutation.TargetID.String())
		if item, err := txn.Get(dKey); err == nil {
			return item.Value(func(val []byte) error {
				var d thinkgraph.DigestMeta
				if json.Unmarshal(val, &d) != nil {
					return nil
				}

				if mutation.Type == thinkgraph.MutationTombstone {
					d.Tombstoned = true
				}
				if mutation.Type == thinkgraph.MutationRestore {
					d.Tombstoned = false
				}
				if mutation.Type == thinkgraph.MutationUpdate && mutation.Payload != "" {
					d.Summary = mutation.Payload
				}

				data, _ := json.Marshal(d)
				return txn.Set(dKey, data)
			})
		}

		scKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.SpaceID, c.ID, mutation.TargetID.String())
		if item, err := txn.Get(scKey); err == nil {
			return item.Value(func(val []byte) error {
				var sc thinkgraph.SpaceContext
				if json.Unmarshal(val, &sc) != nil {
					return nil
				}

				if mutation.Type == thinkgraph.MutationTombstone {
					sc.Tombstoned = true
				}
				if mutation.Type == thinkgraph.MutationRestore {
					sc.Tombstoned = false
				}
				if mutation.Type == thinkgraph.MutationUpdate && mutation.Payload != "" {
					sc.Content = mutation.Payload
				}

				data, _ := json.Marshal(sc)
				return txn.Set(scKey, data)
			})
		}

		return fmt.Errorf("target node %s not found in read graph for mutation", mutation.TargetID)
	})
}

func (g *BadgerGraph) ClearChatGraph(ctx context.Context, c *thinkgraph.Chat) error {
	slog.WarnContext(ctx, "BadgerDB wiping chat graph", "chat_id", c.ID, "space_id", c.SpaceID)
	for _, ns := range chatCollections {
		prefix := fmt.Appendf(nil, "%s:%s:%s:", ns, c.SpaceID, c.ID)
		if err := g.db.DropPrefix(prefix); err != nil {
			return fmt.Errorf("failed to drop prefix %s: %w", string(prefix), err)
		}
	}

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

func (g *BadgerGraph) GetBranchTimeline(ctx context.Context, c *thinkgraph.Chat, branchID string) ([]thinkgraph.Node, error) {
	var nodes []thinkgraph.Node
	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.SpaceID, c.ID, branchID)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 7 {
				continue
			}
			eventID := parts[6]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, eventID)
			if nodeItem, err := txn.Get(nodeKey); err == nil {
				_ = nodeItem.Value(func(val []byte) error {
					var node thinkgraph.Node
					if json.Unmarshal(val, &node) == nil && !node.Tombstoned {
						nodes = append(nodes, node)
					}
					return nil
				})
			}
		}
		return nil
	})
	return nodes, err
}

func (g *BadgerGraph) GetBranchTimelinePage(ctx context.Context, c *thinkgraph.Chat, branchID string, cursor string, limit int) ([]thinkgraph.Node, string, bool, error) {
	var nodes []thinkgraph.Node
	var hasMore bool
	var nextCursor string

	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.SpaceID, c.ID, branchID)

	err := g.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		var seekPrefix []byte
		if cursor != "" {
			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, cursor)
			item, err := txn.Get(nodeKey)
			if err != nil {
				return fmt.Errorf("cursor event not found: %w", err)
			}
			var node thinkgraph.Node
			_ = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &node)
			})
			seekPrefix = fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.SpaceID, c.ID, branchID, node.Timestamp.UnixNano(), node.ID.String())
		} else {
			seekPrefix = append(prefix, 0xFF)
		}

		count := 0
		for it.Seek(seekPrefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 7 {
				continue
			}
			eventID := parts[6]

			if count >= limit {
				hasMore = true
				nextCursor = eventID
				break
			}

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, eventID)
			if nodeItem, err := txn.Get(nodeKey); err == nil {
				_ = nodeItem.Value(func(val []byte) error {
					var node thinkgraph.Node
					if json.Unmarshal(val, &node) == nil && !node.Tombstoned {
						nodes = append([]thinkgraph.Node{node}, nodes...)
						count++
					}
					return nil
				})
			}
		}
		return nil
	})

	return nodes, nextCursor, hasMore, err
}

func (g *BadgerGraph) WalkBranchReverse(ctx context.Context, c *thinkgraph.Chat, branchID string, startCursor string, fn func(thinkgraph.Node) bool) error {
	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.SpaceID, c.ID, branchID)

	return g.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		var seekPrefix []byte
		if startCursor != "" {
			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, startCursor)
			item, err := txn.Get(nodeKey)
			if err == nil {
				var node thinkgraph.Node
				_ = item.Value(func(val []byte) error { return json.Unmarshal(val, &node) })
				seekPrefix = fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.SpaceID, c.ID, branchID, node.Timestamp.UnixNano(), node.ID.String())
			} else {
				seekPrefix = append(prefix, 0xFF)
			}
		} else {
			seekPrefix = append(prefix, 0xFF)
		}

		for it.Seek(seekPrefix); it.ValidForPrefix(prefix); it.Next() {
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 7 {
				continue
			}
			eventID := parts[6]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, eventID)
			if nodeItem, err := txn.Get(nodeKey); err == nil {
				var node thinkgraph.Node
				_ = nodeItem.Value(func(val []byte) error {
					return json.Unmarshal(val, &node)
				})

				if !node.Tombstoned {
					if !fn(node) {
						break
					}
				}
			}
		}
		return nil
	})
}

func (g *BadgerGraph) GetLenses(ctx context.Context, c *thinkgraph.Chat, tag string) (thinkgraph.LensResult, error) {
	var result thinkgraph.LensResult
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
			eventID := parts[5]

			if item, err := txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, eventID)); err == nil {
				_ = item.Value(func(val []byte) error {
					var node thinkgraph.Node
					if json.Unmarshal(val, &node) == nil && !node.Tombstoned {
						result.Nodes = append(result.Nodes, node)
					}
					return nil
				})
				continue
			}

			if item, err := txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.SpaceID, c.ID, eventID)); err == nil {
				_ = item.Value(func(val []byte) error {
					var d thinkgraph.DigestMeta
					if json.Unmarshal(val, &d) == nil && !d.Tombstoned {
						result.Digests = append(result.Digests, d)
					}
					return nil
				})
				continue
			}

			if item, err := txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.SpaceID, c.ID, eventID)); err == nil {
				_ = item.Value(func(val []byte) error {
					var sc thinkgraph.SpaceContext
					if json.Unmarshal(val, &sc) == nil && !sc.Tombstoned {
						result.SpaceContexts = append(result.SpaceContexts, sc)
					}
					return nil
				})
			}
		}
		return nil
	})
	return result, err
}

func (g *BadgerGraph) GetStickyDigests(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.DigestMeta, error) {
	slog.DebugContext(ctx, "BadgerDB querying sticky digests")
	var digests []thinkgraph.DigestMeta
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		digests, err = scanPrefix[thinkgraph.DigestMeta](txn, fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.SpaceID, c.ID), func(d thinkgraph.DigestMeta) bool {
			return d.Promoted
		})
		return err
	})
	return digests, err
}

func (g *BadgerGraph) GetDigests(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.DigestMeta, error) {
	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.SpaceID, c.ID)
	slog.InfoContext(ctx, "BadgerDB querying digests", "prefix", string(prefix))

	var digests []thinkgraph.DigestMeta
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		digests, err = scanPrefix[thinkgraph.DigestMeta](txn, prefix, nil)
		return err
	})
	return digests, err
}

func (g *BadgerGraph) GetSpaceContexts(ctx context.Context, c *thinkgraph.Chat) ([]thinkgraph.SpaceContext, error) {
	slog.DebugContext(ctx, "BadgerDB querying space contexts")
	var contexts []thinkgraph.SpaceContext
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		contexts, err = scanPrefix[thinkgraph.SpaceContext](txn, fmt.Appendf(nil, "%s:%s:%s:", nsSpaceContext, c.SpaceID, c.ID), nil)
		return err
	})
	return contexts, err
}

func (g *BadgerGraph) GetNodes(ctx context.Context, c *thinkgraph.Chat, ids []string) ([]thinkgraph.Node, error) {
	var nodes []thinkgraph.Node
	err := g.db.View(func(txn *badger.Txn) error {
		for _, id := range ids {
			key := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.SpaceID, c.ID, id)
			item, err := txn.Get(key)
			if err != nil {
				continue
			}

			_ = item.Value(func(val []byte) error {
				var node thinkgraph.Node
				if json.Unmarshal(val, &node) == nil && !node.Tombstoned {
					nodes = append(nodes, node)
				}
				return nil
			})
		}
		return nil
	})
	return nodes, err
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
