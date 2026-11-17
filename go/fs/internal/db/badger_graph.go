package db

import (
	"context"
	"encoding/json"
	"errors"
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
	nsEvent, nsTimeline, nsLens, nsDigest, nsSpaceContext, nsSyncState,
}

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
	if g == nil || g.db == nil {
		return nil
	}
	return g.db.Close()
}

func syncStateKey(c *thinkgraph.ChatAddress) []byte {
	dirName := thinkgraph.ResolveLogDirName(c.Name)
	return fmt.Appendf(nil, "%s:%s:%s:%s", nsSyncState, c.ProjectID, c.ChatID, dirName)
}

func scanPrefix[T any](ctx context.Context, txn *badger.Txn, prefix []byte, filter func(T) bool) ([]T, error) {
	var results []T
	it := txn.NewIterator(badger.DefaultIteratorOptions)
	defer it.Close()

	for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err := it.Item().Value(func(val []byte) error {
			var item T
			if unmarshalErr := json.Unmarshal(val, &item); unmarshalErr != nil {
				return fmt.Errorf("unmarshaling item at key %s: %w", string(it.Item().Key()), unmarshalErr)
			}
			if tombstonable, ok := any(item).(interface{ IsTombstoned() bool }); ok && tombstonable.IsTombstoned() {
				return nil
			}
			if filter == nil || filter(item) {
				results = append(results, item)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (g *BadgerGraph) AppendNode(ctx context.Context, c *thinkgraph.ChatAddress, node thinkgraph.Node) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, node.ID.String())

	graphIDStr := node.GraphID.String()
	if node.GraphID.IsZero() {
		graphIDStr = "c:legacy"
	}

	timelineKey := fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.ProjectID, c.ChatID, graphIDStr, node.Timestamp.UnixNano(), node.ID.String())

	nodeData, err := json.Marshal(node)
	if err != nil {
		return fmt.Errorf("failed to marshal node: %w", err)
	}

	slog.DebugContext(ctx, "BadgerDB appending node", "key", string(nodeKey))

	sKey := syncStateKey(c)

	return g.db.Update(func(txn *badger.Txn) error {
		_, getErr := txn.Get(nodeKey)
		if getErr != nil && !errors.Is(getErr, badger.ErrKeyNotFound) {
			return fmt.Errorf("checking existing node %s: %w", node.ID, getErr)
		}
		isNewNode := errors.Is(getErr, badger.ErrKeyNotFound)

		if err := txn.Set(nodeKey, nodeData); err != nil {
			return err
		}
		if err := txn.Set(timelineKey, nil); err != nil {
			return err
		}
		for _, tag := range node.Tags {
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.ProjectID, c.ChatID, tag, node.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}

		// Keep branch sync state aligned with the physical ledger inside the same transaction
		// so live appends do not trigger redundant FastForward re-reads on SyncGraphContext.
		if isNewNode {
			manifest := thinkgraph.LedgerManifest{ActiveShard: "00000.jsonl"}
			syncItem, syncErr := txn.Get(sKey)
			if syncErr == nil {
				if valErr := syncItem.Value(func(val []byte) error {
					return json.Unmarshal(val, &manifest)
				}); valErr != nil {
					return fmt.Errorf("unmarshaling sync state for %s: %w", string(sKey), valErr)
				}
			} else if !errors.Is(syncErr, badger.ErrKeyNotFound) {
				return fmt.Errorf("reading sync state for %s: %w", string(sKey), syncErr)
			}

			manifest.EventCount++
			manifest.TotalEvents++
			syncData, marshalErr := json.Marshal(manifest)
			if marshalErr != nil {
				return fmt.Errorf("marshaling sync state: %w", marshalErr)
			}
			if err := txn.Set(sKey, syncData); err != nil {
				return fmt.Errorf("updating sync state: %w", err)
			}
		}

		return nil
	})
}

func (g *BadgerGraph) SaveDigest(ctx context.Context, c *thinkgraph.ChatAddress, digest thinkgraph.DigestMeta) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.ProjectID, c.ChatID, digest.ID.String())
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
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.ProjectID, c.ChatID, tag, digest.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveSpaceContext(ctx context.Context, c *thinkgraph.ChatAddress, spaceCtx thinkgraph.SpaceContext) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.ProjectID, c.ChatID, spaceCtx.ID.String())
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
			lensKey := fmt.Appendf(nil, "%s:%s:%s:%s:%s", nsLens, c.ProjectID, c.ChatID, tag, spaceCtx.ID.String())
			if err := txn.Set(lensKey, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *BadgerGraph) SaveMutation(ctx context.Context, c *thinkgraph.ChatAddress, mutation thinkgraph.Mutator) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	slog.InfoContext(ctx, "BadgerDB applying mutation", "type", mutation.EventType(), "target_id", mutation.Target().String())

	return g.db.Update(func(txn *badger.Txn) error {
		evKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, mutation.Target().String())
		item, err := txn.Get(evKey)
		if err == nil {
			return item.Value(func(val []byte) error {
				var node thinkgraph.Node
				if unmarshalErr := json.Unmarshal(val, &node); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling target node %s: %w", mutation.Target(), unmarshalErr)
				}

				switch m := mutation.(type) {
				case thinkgraph.TombstoneMutation:
					node.Tombstoned = true
				case thinkgraph.RestoreMutation:
					node.Tombstoned = false
				case thinkgraph.ContentMutation:
					node.Content = m.Content
				case thinkgraph.StateMutation:
					node.MaterializedState = m.State
					if m.Content != "" {
						node.Content = m.Content
					}
				case thinkgraph.MetadataMutation:
					if node.Metadata == nil {
						node.Metadata = make(map[string]string)
					}
					for k, v := range m.Metadata {
						node.Metadata[k] = v
					}
				}

				data, marshalErr := json.Marshal(node)
				if marshalErr != nil {
					return fmt.Errorf("marshaling mutated node %s: %w", mutation.Target(), marshalErr)
				}
				return txn.Set(evKey, data)
			})
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("reading event node %s: %w", mutation.Target(), err)
		}

		dKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.ProjectID, c.ChatID, mutation.Target().String())
		item, err = txn.Get(dKey)
		if err == nil {
			return item.Value(func(val []byte) error {
				var d thinkgraph.DigestMeta
				if unmarshalErr := json.Unmarshal(val, &d); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling target digest %s: %w", mutation.Target(), unmarshalErr)
				}

				switch m := mutation.(type) {
				case thinkgraph.TombstoneMutation:
					d.Tombstoned = true
				case thinkgraph.RestoreMutation:
					d.Tombstoned = false
				case thinkgraph.ContentMutation:
					d.Summary = m.Content
				}

				data, marshalErr := json.Marshal(d)
				if marshalErr != nil {
					return fmt.Errorf("marshaling mutated digest %s: %w", mutation.Target(), marshalErr)
				}
				return txn.Set(dKey, data)
			})
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("reading digest %s: %w", mutation.Target(), err)
		}

		scKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.ProjectID, c.ChatID, mutation.Target().String())
		item, err = txn.Get(scKey)
		if err == nil {
			return item.Value(func(val []byte) error {
				var sc thinkgraph.SpaceContext
				if unmarshalErr := json.Unmarshal(val, &sc); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling target space context %s: %w", mutation.Target(), unmarshalErr)
				}

				switch m := mutation.(type) {
				case thinkgraph.TombstoneMutation:
					sc.Tombstoned = true
				case thinkgraph.RestoreMutation:
					sc.Tombstoned = false
				case thinkgraph.ContentMutation:
					sc.Content = m.Content
				}

				data, marshalErr := json.Marshal(sc)
				if marshalErr != nil {
					return fmt.Errorf("marshaling mutated space context %s: %w", mutation.Target(), marshalErr)
				}
				return txn.Set(scKey, data)
			})
		}
		if !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("reading space context %s: %w", mutation.Target(), err)
		}

		return fmt.Errorf("target node %s not found in read graph for mutation", mutation.Target())
	})
}

func (g *BadgerGraph) ClearChatGraph(ctx context.Context, c *thinkgraph.ChatAddress) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	slog.WarnContext(ctx, "BadgerDB wiping chat graph", "chat_id", c.ChatID, "space_id", c.ProjectID)
	for _, ns := range chatCollections {
		prefix := fmt.Appendf(nil, "%s:%s:%s:", ns, c.ProjectID, c.ChatID)
		if err := g.db.DropPrefix(prefix); err != nil {
			return fmt.Errorf("failed to drop prefix %s: %w", string(prefix), err)
		}
	}

	// Clean up any legacy chat-wide sync state key if present.
	return g.db.Update(func(txn *badger.Txn) error {
		legacyKey := fmt.Appendf(nil, "%s:%s:%s", nsSyncState, c.ProjectID, c.ChatID)
		if err := txn.Delete(legacyKey); err != nil && !errors.Is(err, badger.ErrKeyNotFound) {
			return fmt.Errorf("failed to delete legacy singleton %s: %w", string(legacyKey), err)
		}
		return nil
	})
}

func (g *BadgerGraph) GetSubGraphTimeline(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID) ([]thinkgraph.Node, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	var nodes []thinkgraph.Node

	graphIDStr := graphID.String()
	if graphID.IsZero() {
		graphIDStr = "c:legacy"
	}

	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.ProjectID, c.ChatID, graphIDStr)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 2 {
				continue
			}
			eventID := parts[len(parts)-1]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, eventID)
			nodeItem, getErr := txn.Get(nodeKey)
			if getErr != nil {
				if errors.Is(getErr, badger.ErrKeyNotFound) {
					continue
				}
				return fmt.Errorf("reading timeline node %s: %w", eventID, getErr)
			}
			if valErr := nodeItem.Value(func(val []byte) error {
				var node thinkgraph.Node
				if unmarshalErr := json.Unmarshal(val, &node); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling timeline node %s: %w", eventID, unmarshalErr)
				}
				if !node.Tombstoned {
					nodes = append(nodes, node)
				}
				return nil
			}); valErr != nil {
				return valErr
			}
		}
		return nil
	})
	return nodes, err
}

func (g *BadgerGraph) GetSubGraphTimelinePage(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID, cursor string, limit int) ([]thinkgraph.Node, string, bool, error) {
	if c == nil {
		return nil, "", false, fmt.Errorf("chat address is nil")
	}

	var nodes []thinkgraph.Node
	var hasMore bool
	var nextCursor string

	graphIDStr := graphID.String()
	if graphID.IsZero() {
		graphIDStr = "c:legacy"
	}

	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.ProjectID, c.ChatID, graphIDStr)

	err := g.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		var seekPrefix []byte
		if cursor != "" {
			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, cursor)
			item, err := txn.Get(nodeKey)
			if err != nil {
				return fmt.Errorf("cursor event not found: %w", err)
			}
			var node thinkgraph.Node
			if valErr := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &node)
			}); valErr != nil {
				return fmt.Errorf("decoding cursor event %s: %w", cursor, valErr)
			}
			seekPrefix = fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.ProjectID, c.ChatID, graphIDStr, node.Timestamp.UnixNano(), node.ID.String())
		} else {
			seekPrefix = append(prefix, 0xFF)
		}

		count := 0
		for it.Seek(seekPrefix); it.ValidForPrefix(prefix); it.Next() {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 2 {
				continue
			}
			eventID := parts[len(parts)-1]

			if count >= limit {
				hasMore = true
				nextCursor = eventID
				break
			}

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, eventID)
			nodeItem, getErr := txn.Get(nodeKey)
			if getErr != nil {
				if errors.Is(getErr, badger.ErrKeyNotFound) {
					continue
				}
				return fmt.Errorf("reading timeline page node %s: %w", eventID, getErr)
			}
			if valErr := nodeItem.Value(func(val []byte) error {
				var node thinkgraph.Node
				if unmarshalErr := json.Unmarshal(val, &node); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling timeline page node %s: %w", eventID, unmarshalErr)
				}
				if !node.Tombstoned {
					nodes = append(nodes, node)
					count++
				}
				return nil
			}); valErr != nil {
				return valErr
			}
		}
		return nil
	})
	if err != nil {
		return nil, "", false, err
	}

	// Reverse into chronological order in-place (O(N) vs O(N^2) head-prepending)
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}

	return nodes, nextCursor, hasMore, nil
}

func (g *BadgerGraph) WalkSubGraphReverse(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID, startCursor string, fn func(thinkgraph.Node) bool) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	graphIDStr := graphID.String()
	if graphID.IsZero() {
		graphIDStr = "c:legacy"
	}
	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsTimeline, c.ProjectID, c.ChatID, graphIDStr)

	return g.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Reverse = true
		it := txn.NewIterator(opts)
		defer it.Close()

		var seekPrefix []byte
		if startCursor != "" {
			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, startCursor)
			item, err := txn.Get(nodeKey)
			if err == nil {
				var node thinkgraph.Node
				if valErr := item.Value(func(val []byte) error { return json.Unmarshal(val, &node) }); valErr != nil {
					return fmt.Errorf("decoding start cursor node %s: %w", startCursor, valErr)
				}
				seekPrefix = fmt.Appendf(nil, "%s:%s:%s:%s:%019d:%s", nsTimeline, c.ProjectID, c.ChatID, graphIDStr, node.Timestamp.UnixNano(), node.ID.String())
			} else if errors.Is(err, badger.ErrKeyNotFound) {
				seekPrefix = append(prefix, 0xFF)
			} else {
				return fmt.Errorf("reading start cursor node %s: %w", startCursor, err)
			}
		} else {
			seekPrefix = append(prefix, 0xFF)
		}

		for it.Seek(seekPrefix); it.ValidForPrefix(prefix); it.Next() {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 2 {
				continue
			}
			eventID := parts[len(parts)-1]

			nodeKey := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, eventID)
			nodeItem, getErr := txn.Get(nodeKey)
			if getErr != nil {
				if errors.Is(getErr, badger.ErrKeyNotFound) {
					continue
				}
				return fmt.Errorf("reading walked node %s: %w", eventID, getErr)
			}

			var node thinkgraph.Node
			if valErr := nodeItem.Value(func(val []byte) error {
				return json.Unmarshal(val, &node)
			}); valErr != nil {
				return fmt.Errorf("unmarshaling walked node %s: %w", eventID, valErr)
			}

			if !node.Tombstoned {
				if !fn(node) {
					break
				}
			}
		}
		return nil
	})
}

func (g *BadgerGraph) GetLenses(ctx context.Context, c *thinkgraph.ChatAddress, tag string) (thinkgraph.LensResult, error) {
	if c == nil {
		return thinkgraph.LensResult{}, fmt.Errorf("chat address is nil")
	}

	var result thinkgraph.LensResult
	prefix := fmt.Appendf(nil, "%s:%s:%s:%s:", nsLens, c.ProjectID, c.ChatID, tag)

	err := g.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			if ctx != nil && ctx.Err() != nil {
				return ctx.Err()
			}
			keyStr := string(it.Item().Key())
			parts := strings.Split(keyStr, ":")
			if len(parts) < 2 {
				continue
			}
			eventID := parts[len(parts)-1]

			item, getErr := txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, eventID))
			if getErr == nil {
				if valErr := item.Value(func(val []byte) error {
					var node thinkgraph.Node
					if unmarshalErr := json.Unmarshal(val, &node); unmarshalErr != nil {
						return fmt.Errorf("unmarshaling lens node %s: %w", eventID, unmarshalErr)
					}
					if !node.Tombstoned {
						result.Nodes = append(result.Nodes, node)
					}
					return nil
				}); valErr != nil {
					return valErr
				}
				continue
			} else if !errors.Is(getErr, badger.ErrKeyNotFound) {
				return fmt.Errorf("reading lens node %s: %w", eventID, getErr)
			}

			item, getErr = txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsDigest, c.ProjectID, c.ChatID, eventID))
			if getErr == nil {
				if valErr := item.Value(func(val []byte) error {
					var d thinkgraph.DigestMeta
					if unmarshalErr := json.Unmarshal(val, &d); unmarshalErr != nil {
						return fmt.Errorf("unmarshaling lens digest %s: %w", eventID, unmarshalErr)
					}
					if !d.Tombstoned {
						result.Digests = append(result.Digests, d)
					}
					return nil
				}); valErr != nil {
					return valErr
				}
				continue
			} else if !errors.Is(getErr, badger.ErrKeyNotFound) {
				return fmt.Errorf("reading lens digest %s: %w", eventID, getErr)
			}

			item, getErr = txn.Get(fmt.Appendf(nil, "%s:%s:%s:%s", nsSpaceContext, c.ProjectID, c.ChatID, eventID))
			if getErr == nil {
				if valErr := item.Value(func(val []byte) error {
					var sc thinkgraph.SpaceContext
					if unmarshalErr := json.Unmarshal(val, &sc); unmarshalErr != nil {
						return fmt.Errorf("unmarshaling lens space context %s: %w", eventID, unmarshalErr)
					}
					if !sc.Tombstoned {
						result.SpaceContexts = append(result.SpaceContexts, sc)
					}
					return nil
				}); valErr != nil {
					return valErr
				}
			} else if !errors.Is(getErr, badger.ErrKeyNotFound) {
				return fmt.Errorf("reading lens space context %s: %w", eventID, getErr)
			}
		}
		return nil
	})
	return result, err
}

func (g *BadgerGraph) GetStickyDigests(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.DigestMeta, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	slog.DebugContext(ctx, "BadgerDB querying sticky digests")
	var digests []thinkgraph.DigestMeta
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		digests, err = scanPrefix[thinkgraph.DigestMeta](ctx, txn, fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.ProjectID, c.ChatID), func(d thinkgraph.DigestMeta) bool {
			return d.Promoted
		})
		return err
	})
	return digests, err
}

func (g *BadgerGraph) GetDigests(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.DigestMeta, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	prefix := fmt.Appendf(nil, "%s:%s:%s:", nsDigest, c.ProjectID, c.ChatID)
	slog.InfoContext(ctx, "BadgerDB querying digests", "prefix", string(prefix))

	var digests []thinkgraph.DigestMeta
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		digests, err = scanPrefix[thinkgraph.DigestMeta](ctx, txn, prefix, nil)
		return err
	})
	return digests, err
}

func (g *BadgerGraph) GetSpaceContexts(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.SpaceContext, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	slog.DebugContext(ctx, "BadgerDB querying space contexts")
	var contexts []thinkgraph.SpaceContext
	err := g.db.View(func(txn *badger.Txn) error {
		var err error
		contexts, err = scanPrefix[thinkgraph.SpaceContext](ctx, txn, fmt.Appendf(nil, "%s:%s:%s:", nsSpaceContext, c.ProjectID, c.ChatID), nil)
		return err
	})
	return contexts, err
}

func (g *BadgerGraph) GetNodes(ctx context.Context, c *thinkgraph.ChatAddress, ids []string) ([]thinkgraph.Node, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	var nodes []thinkgraph.Node
	err := g.db.View(func(txn *badger.Txn) error {
		for _, id := range ids {
			key := fmt.Appendf(nil, "%s:%s:%s:%s", nsEvent, c.ProjectID, c.ChatID, id)
			item, err := txn.Get(key)
			if err != nil {
				if errors.Is(err, badger.ErrKeyNotFound) {
					continue
				}
				return fmt.Errorf("reading node %s: %w", id, err)
			}

			if valErr := item.Value(func(val []byte) error {
				var node thinkgraph.Node
				if unmarshalErr := json.Unmarshal(val, &node); unmarshalErr != nil {
					return fmt.Errorf("unmarshaling node %s: %w", id, unmarshalErr)
				}
				if !node.Tombstoned {
					nodes = append(nodes, node)
				}
				return nil
			}); valErr != nil {
				return valErr
			}
		}
		return nil
	})
	return nodes, err
}

func (g *BadgerGraph) GetSyncState(ctx context.Context, c *thinkgraph.ChatAddress) (*thinkgraph.LedgerManifest, error) {
	if c == nil {
		return nil, fmt.Errorf("chat address is nil")
	}

	var manifest thinkgraph.LedgerManifest
	key := syncStateKey(c)

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

	if errors.Is(err, badger.ErrKeyNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &manifest, nil
}

func (g *BadgerGraph) SetSyncState(ctx context.Context, c *thinkgraph.ChatAddress, manifest thinkgraph.LedgerManifest) error {
	if c == nil {
		return fmt.Errorf("chat address is nil")
	}

	key := syncStateKey(c)
	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal sync state: %w", err)
	}
	return g.db.Update(func(txn *badger.Txn) error {
		return txn.Set(key, data)
	})
}
