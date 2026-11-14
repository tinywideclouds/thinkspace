package thinkgraph

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	store         *GraphStore
	workspaceRoot string
}

func NewRehydrator(store *GraphStore, workspaceRoot string) *Rehydrator {
	return &Rehydrator{store: store, workspaceRoot: workspaceRoot}
}

func (r *Rehydrator) CheckSync(ctx context.Context, c *ChatAddress) (SyncStatus, *LedgerManifest, *LedgerManifest, error) {
	subGraphName := c.Name
	if subGraphName == "" {
		subGraphName = "main"
	}
	ledgerDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger")

	diskManifest, err := LoadOrRebuildManifest(ledgerDir, subGraphName)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to load disk manifest: %w", err)
	}

	graphManifest, err := r.store.Repository().GetSyncState(ctx, c)
	if err != nil {
		return SyncStatusMismatch, nil, nil, fmt.Errorf("failed to get graph sync state: %w", err)
	}

	if graphManifest == nil {
		if diskManifest.TotalEvents == 0 {
			_ = r.store.Repository().SetSyncState(ctx, c, *diskManifest)
			return SyncStatusOK, diskManifest, diskManifest, nil
		}
		slog.InfoContext(ctx, "CheckSync: graph state empty but disk has events", "disk_events", diskManifest.TotalEvents)
		return SyncStatusMismatch, diskManifest, nil, nil
	}

	if graphManifest.TotalEvents == diskManifest.TotalEvents {
		return SyncStatusOK, diskManifest, graphManifest, nil
	}

	if graphManifest.TotalEvents < diskManifest.TotalEvents {
		return SyncStatusBehind, diskManifest, graphManifest, nil
	}

	slog.WarnContext(ctx, "CheckSync: graph has more events than disk (ledger truncated or altered)",
		"disk_events", diskManifest.TotalEvents,
		"graph_events", graphManifest.TotalEvents,
	)
	return SyncStatusMismatch, diskManifest, graphManifest, nil
}

func (r *Rehydrator) FastForward(ctx context.Context, c *ChatAddress, graphManifest, diskManifest *LedgerManifest) error {
	dirName := ResolveLogDirName(c.Name)
	logsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "logs", dirName)
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

			saveAddr := *c
			if strings.HasPrefix(c.Name, "flow-") {
				saveAddr.SubGraph.Name = dirName
			}
			if !event.GraphID.IsZero() {
				saveAddr.SubGraph.GraphID = event.GraphID
			}

			if err := r.store.AppendEvent(ctx, &saveAddr, event); err != nil {
				file.Close()
				return err
			}

			graphManifest.TotalEvents++
			_ = r.store.Repository().SetSyncState(ctx, c, *graphManifest)
		}
		file.Close()
	}

	return r.store.Repository().SetSyncState(ctx, c, *diskManifest)
}

func (r *Rehydrator) Rebuild(ctx context.Context, c *ChatAddress) error {
	if err := r.store.Repository().ClearChatGraph(ctx, c); err != nil {
		return fmt.Errorf("failed to clear graph: %w", err)
	}

	contextsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "contexts")
	entries, err := os.ReadDir(contextsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(contextsDir, e.Name()))
				if err == nil {
					spaceCtx := r.parseSpaceContextMarkdown(e.Name(), string(content))
					if spaceCtx.ID != (uuid.UUID{}) {
						_ = r.store.SaveSpaceContext(ctx, c, spaceCtx)
					}
				}
			}
		}
	}

	digestsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "digests")
	entries, err = os.ReadDir(digestsDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				content, err := os.ReadFile(filepath.Join(digestsDir, e.Name()))
				if err == nil {
					digestMeta := r.parseDigestMarkdown(e.Name(), string(content))
					if digestMeta.ID != (uuid.UUID{}) {
						_ = r.store.SaveDigest(ctx, c, digestMeta)
					}
				}
			}
		}
	}

	ledgerDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger")
	logsBaseDir := filepath.Join(ledgerDir, "logs")
	subGraphs, err := os.ReadDir(logsBaseDir)
	if err == nil {
		for _, branchDir := range subGraphs {
			if branchDir.IsDir() {
				subGraphName := branchDir.Name()
				diskManifest, err := LoadOrRebuildManifest(ledgerDir, subGraphName)
				if err == nil {
					subGraphChat := &ChatAddress{ChatID: c.ChatID, ProjectID: c.ProjectID, SubGraph: SubGraph{Name: subGraphName}}
					emptyGraphManifest := &LedgerManifest{TotalEvents: 0}
					if err := r.FastForward(ctx, subGraphChat, emptyGraphManifest, diskManifest); err != nil {
						return fmt.Errorf("fast-forwarding subgraph %s: %w", subGraphName, err)
					}
				}
			}
		}
	}

	mutationsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "mutations")
	entries, err = os.ReadDir(mutationsDir)
	if err == nil {
		var mutFiles []string
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				mutFiles = append(mutFiles, e.Name())
			}
		}
		sort.Strings(mutFiles)

		for _, filename := range mutFiles {
			if data, err := os.ReadFile(filepath.Join(mutationsDir, filename)); err == nil {
				if mut, unmarshalErr := UnmarshalMutation(data); unmarshalErr == nil {
					_ = r.store.SaveMutation(ctx, c, mut)
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
