package thinkgraph

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"uuid"
)

type SyncStatus string

const (
	SyncStatusOK       SyncStatus = "ok"
	SyncStatusBehind   SyncStatus = "behind"
	SyncStatusMismatch SyncStatus = "mismatch"
)

type Rehydrator struct {
	graph         GraphRepository
	workspaceRoot string
}

func NewRehydrator(graph GraphRepository, workspaceRoot string) *Rehydrator {
	return &Rehydrator{graph: graph, workspaceRoot: workspaceRoot}
}

// CheckSync compares the physical disk truth against the BadgerDB index.
func (r *Rehydrator) CheckSync(ctx context.Context, c *Chat) (SyncStatus, *LedgerManifest, *LedgerManifest, error) {
	ledgerDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")

	diskManifest, err := LoadOrRebuildManifest(ledgerDir)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to load disk manifest: %w", err)
	}

	graphManifest, err := r.graph.GetSyncState(ctx, c)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to get graph sync state: %w", err)
	}

	// Cold Boot: New Chat or Wiped DB
	if graphManifest == nil {
		if diskManifest.TotalEvents == 0 {
			// Fast-path for brand new, empty chats. Just initialize empty sync state.
			_ = r.graph.SetSyncState(ctx, c, *diskManifest)
			return SyncStatusOK, diskManifest, diskManifest, nil
		}
		// Populated disk, but empty Badger. (e.g. server restart on a new machine)
		return SyncStatusMismatch, diskManifest, nil, nil
	}

	if graphManifest.TotalEvents == diskManifest.TotalEvents {
		return SyncStatusOK, diskManifest, graphManifest, nil
	}

	if graphManifest.TotalEvents < diskManifest.TotalEvents {
		return SyncStatusBehind, diskManifest, graphManifest, nil
	}

	// The Graph has more events than Disk (User altered git history, e.g., git reset --hard)
	return SyncStatusMismatch, diskManifest, graphManifest, nil
}

// FastForward streams only the missing events from the tail of the disk shards into the graph.
func (r *Rehydrator) FastForward(ctx context.Context, c *Chat, graphManifest, diskManifest *LedgerManifest) error {
	logsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return err
	}

	var shards []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			shards = append(shards, e.Name())
		}
	}
	sort.Strings(shards)

	skipped := 0
	for _, shard := range shards {
		file, err := os.Open(filepath.Join(logsDir, shard))
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if len(scanner.Bytes()) == 0 {
				continue
			}

			if skipped < graphManifest.TotalEvents {
				skipped++
				continue
			}

			var event Event
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				continue
			}

			if err := r.graph.AppendEvent(ctx, c, event); err != nil {
				file.Close()
				return err
			}

			graphManifest.TotalEvents++
			_ = r.graph.SetSyncState(ctx, c, *graphManifest)
		}
		file.Close()
	}

	return r.graph.SetSyncState(ctx, c, *diskManifest)
}

// Rebuild clears the specific chat's graph prefix and streams the physical disk reality back in.
func (r *Rehydrator) Rebuild(ctx context.Context, c *Chat) error {
	if err := r.graph.ClearChatGraph(ctx, c); err != nil {
		return fmt.Errorf("failed to clear graph: %w", err)
	}

	// 1. Re-ingest Digests (Markdown -> Badger)
	digestsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "digests")
	entries, err := os.ReadDir(digestsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(digestsDir, e.Name()))
				if err == nil {
					digestMeta := r.parseDigestMarkdown(e.Name(), string(content))
					if digestMeta.ID != (uuid.UUID{}) {
						_ = r.graph.SaveDigest(ctx, c, digestMeta)
					}
				}
			}
		}
	}

	// 2. Re-ingest Event Shards (.jsonl -> Badger)
	ledgerDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")
	diskManifest, err := LoadOrRebuildManifest(ledgerDir)
	if err != nil {
		return err
	}

	emptyGraphManifest := &LedgerManifest{TotalEvents: 0}
	return r.FastForward(ctx, c, emptyGraphManifest, diskManifest)
}

func (r *Rehydrator) parseDigestMarkdown(filename, content string) DigestMeta {
	idStr := strings.TrimSuffix(filename, ".md")

	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.UUID{}
	}

	meta := DigestMeta{
		ID: id,
	}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			meta.Summary = strings.TrimSpace(parts[2])

			if strings.Contains(frontmatter, "sticky: true") {
				meta.IsSticky = true
			}

			if strings.Contains(frontmatter, "summarizes: [") {
				start := strings.Index(frontmatter, "summarizes: [") + 13
				end := strings.Index(frontmatter[start:], "]")
				if end != -1 {
					arr := frontmatter[start : start+end]
					if strings.TrimSpace(arr) != "" {
						for _, p := range strings.Split(arr, ",") {
							p = strings.Trim(strings.TrimSpace(p), `"`)
							if evID, err := uuid.Parse(p); err == nil {
								meta.SummarizedEvents = append(meta.SummarizedEvents, evID)
							}
						}
					}
				}
			}
		}
	} else {
		meta.Summary = content
	}

	return meta
}
