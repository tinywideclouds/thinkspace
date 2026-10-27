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

func (r *Rehydrator) CheckSync(ctx context.Context, c *Chat) (SyncStatus, *LedgerManifest, *LedgerManifest, error) {
	branchID := c.ActiveBranch
	if branchID == "" {
		branchID = "main"
	}
	ledgerDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")

	diskManifest, err := LoadOrRebuildManifest(ledgerDir, branchID)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to load disk manifest: %w", err)
	}

	graphManifest, err := r.graph.GetSyncState(ctx, c)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to get graph sync state: %w", err)
	}

	if graphManifest == nil {
		if diskManifest.TotalEvents == 0 {
			_ = r.graph.SetSyncState(ctx, c, *diskManifest)
			return SyncStatusOK, diskManifest, diskManifest, nil
		}
		return SyncStatusMismatch, diskManifest, nil, nil
	}

	if graphManifest.TotalEvents == diskManifest.TotalEvents {
		return SyncStatusOK, diskManifest, graphManifest, nil
	}

	if graphManifest.TotalEvents < diskManifest.TotalEvents {
		return SyncStatusBehind, diskManifest, graphManifest, nil
	}

	return SyncStatusMismatch, diskManifest, graphManifest, nil
}

func (r *Rehydrator) FastForward(ctx context.Context, c *Chat, graphManifest, diskManifest *LedgerManifest) error {
	branchID := c.ActiveBranch
	if branchID == "" {
		branchID = "main"
	}
	logsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "logs", branchID)
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

	targetSkip := graphManifest.TotalEvents
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

			if skipped < targetSkip {
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

func (r *Rehydrator) Rebuild(ctx context.Context, c *Chat) error {
	if err := r.graph.ClearChatGraph(ctx, c); err != nil {
		return fmt.Errorf("failed to clear graph: %w", err)
	}

	// 1. Re-ingest SpaceContexts (Markdown -> Badger)
	contextsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "contexts")
	entries, err := os.ReadDir(contextsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(contextsDir, e.Name()))
				if err == nil {
					spaceCtx := r.parseSpaceContextMarkdown(e.Name(), string(content))
					if spaceCtx.ID != (uuid.UUID{}) {
						_ = r.graph.SaveSpaceContext(ctx, c, spaceCtx)
					}
				}
			}
		}
	}

	// 2. Re-ingest Digests (Markdown -> Badger)
	digestsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "digests")
	entries, err = os.ReadDir(digestsDir)
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

	// 3. Re-ingest Event Shards for ALL branches (.jsonl -> Badger)
	ledgerDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")
	logsBaseDir := filepath.Join(ledgerDir, "logs")
	branches, err := os.ReadDir(logsBaseDir)
	if err == nil {
		for _, branchDir := range branches {
			if branchDir.IsDir() {
				branchID := branchDir.Name()
				diskManifest, err := LoadOrRebuildManifest(ledgerDir, branchID)
				if err == nil {
					branchChat := &Chat{ID: c.ID, SpaceID: c.SpaceID, ActiveBranch: branchID}
					emptyGraphManifest := &LedgerManifest{TotalEvents: 0}
					if err := r.FastForward(ctx, branchChat, emptyGraphManifest, diskManifest); err != nil {
						return fmt.Errorf("fast-forwarding branch %s: %w", branchID, err)
					}
				}
			}
		}
	}

	// 4. Replay Mutations chronologically to re-establish tombstone and update states
	mutationsDir := filepath.Join(r.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "mutations")
	entries, err = os.ReadDir(mutationsDir)
	if err == nil {
		var mutFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				mutFiles = append(mutFiles, e.Name())
			}
		}
		sort.Strings(mutFiles) // Chronological execution based on unixnano prefixes

		for _, filename := range mutFiles {
			if data, err := os.ReadFile(filepath.Join(mutationsDir, filename)); err == nil {
				var mut MutationEvent
				if json.Unmarshal(data, &mut) == nil {
					_ = r.graph.SaveMutation(ctx, c, mut)
				}
			}
		}
	}

	return nil
}

func (r *Rehydrator) parseDigestMarkdown(filename, content string) DigestMeta {
	idStr := strings.TrimSuffix(filename, ".md")
	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.UUID{}
	}

	meta := DigestMeta{ID: id}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			meta.Summary = strings.TrimSpace(parts[2])

			if strings.Contains(frontmatter, "sticky: true") {
				meta.Promoted = true
			}
			if strings.Contains(frontmatter, "tombstoned: true") {
				meta.Tombstoned = true
			}

			meta.Tags = r.extractTags(frontmatter)

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

func (r *Rehydrator) parseSpaceContextMarkdown(filename, content string) SpaceContext {
	idStr := strings.TrimSuffix(filename, ".md")
	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.UUID{}
	}

	sc := SpaceContext{ID: id}

	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			sc.Content = strings.TrimSpace(parts[2])
			sc.Tags = r.extractTags(frontmatter)

			if strings.Contains(frontmatter, "tombstoned: true") {
				sc.Tombstoned = true
			}
		}
	} else {
		sc.Content = content
	}

	return sc
}

func (r *Rehydrator) extractTags(frontmatter string) []string {
	var tags []string
	if strings.Contains(frontmatter, "tags: [") {
		start := strings.Index(frontmatter, "tags: [") + 7
		end := strings.Index(frontmatter[start:], "]")
		if end != -1 {
			arr := frontmatter[start : start+end]
			if strings.TrimSpace(arr) != "" {
				for _, p := range strings.Split(arr, ",") {
					p = strings.Trim(strings.TrimSpace(p), `"`)
					tags = append(tags, p)
				}
			}
		}
	}
	return tags
}
