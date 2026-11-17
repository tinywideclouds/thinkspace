package thinkgraph

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
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
			if setErr := r.store.Repository().SetSyncState(ctx, c, *diskManifest); setErr != nil {
				return SyncStatusMismatch, nil, nil, fmt.Errorf("initializing graph sync state: %w", setErr)
			}
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
		return fmt.Errorf("reading logs directory %s: %w", logsDir, err)
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
		shardPath := filepath.Join(logsDir, shard)
		file, openErr := os.Open(shardPath)
		if openErr != nil {
			return fmt.Errorf("opening shard %s: %w", shardPath, openErr)
		}

		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 64*1024), maxLedgerLineBytes)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			if len(scanner.Bytes()) == 0 {
				continue
			}

			if skipped < targetSkip {
				skipped++
				continue
			}

			var event Event
			if unmarshalErr := json.Unmarshal(scanner.Bytes(), &event); unmarshalErr != nil {
				_ = file.Close()
				return fmt.Errorf("unmarshaling event in %s at line %d: %w", shardPath, lineNum, unmarshalErr)
			}

			saveAddr := *c
			if strings.HasPrefix(c.Name, "flow-") {
				saveAddr.SubGraph.Name = dirName
			}
			if !event.GraphID.IsZero() {
				saveAddr.SubGraph.GraphID = event.GraphID
			}

			if appendErr := r.store.AppendEvent(ctx, &saveAddr, event); appendErr != nil {
				_ = file.Close()
				return fmt.Errorf("appending event %s during fast-forward: %w", event.ID, appendErr)
			}

			graphManifest.TotalEvents++
		}

		scanErr := scanner.Err()
		_ = file.Close()
		if scanErr != nil {
			return fmt.Errorf("scanning shard %s: %w", shardPath, scanErr)
		}
	}

	if err := r.store.Repository().SetSyncState(ctx, c, *diskManifest); err != nil {
		return fmt.Errorf("updating graph sync state: %w", err)
	}
	return nil
}

func (r *Rehydrator) Rebuild(ctx context.Context, c *ChatAddress) error {
	if err := r.store.Repository().ClearChatGraph(ctx, c); err != nil {
		return fmt.Errorf("failed to clear graph: %w", err)
	}

	contextsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "contexts")
	entries, err := os.ReadDir(contextsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading contexts directory %s: %w", contextsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			filePath := filepath.Join(contextsDir, e.Name())
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return fmt.Errorf("reading space context %s: %w", filePath, readErr)
			}
			spaceCtx, parseErr := r.parseSpaceContextMarkdown(e.Name(), string(content))
			if parseErr != nil {
				return fmt.Errorf("parsing space context %s: %w", filePath, parseErr)
			}
			if saveErr := r.store.SaveSpaceContext(ctx, c, spaceCtx); saveErr != nil {
				return fmt.Errorf("saving space context %s: %w", spaceCtx.ID, saveErr)
			}
		}
	}

	digestsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "digests")
	entries, err = os.ReadDir(digestsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading digests directory %s: %w", digestsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			filePath := filepath.Join(digestsDir, e.Name())
			content, readErr := os.ReadFile(filePath)
			if readErr != nil {
				return fmt.Errorf("reading digest %s: %w", filePath, readErr)
			}
			digestMeta, parseErr := r.parseDigestMarkdown(e.Name(), string(content))
			if parseErr != nil {
				return fmt.Errorf("parsing digest %s: %w", filePath, parseErr)
			}
			if saveErr := r.store.SaveDigest(ctx, c, digestMeta); saveErr != nil {
				return fmt.Errorf("saving digest %s: %w", digestMeta.ID, saveErr)
			}
		}
	}

	ledgerDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger")
	logsBaseDir := filepath.Join(ledgerDir, "logs")
	subGraphs, err := os.ReadDir(logsBaseDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading logs base directory %s: %w", logsBaseDir, err)
	}
	for _, branchDir := range subGraphs {
		if branchDir.IsDir() {
			subGraphName := branchDir.Name()
			diskManifest, manifestErr := LoadOrRebuildManifest(ledgerDir, subGraphName)
			if manifestErr != nil {
				return fmt.Errorf("loading manifest for subgraph %s: %w", subGraphName, manifestErr)
			}
			subGraphChat := &ChatAddress{ChatID: c.ChatID, ProjectID: c.ProjectID, SubGraph: SubGraph{Name: subGraphName}}
			emptyGraphManifest := &LedgerManifest{TotalEvents: 0}
			if ffErr := r.FastForward(ctx, subGraphChat, emptyGraphManifest, diskManifest); ffErr != nil {
				return fmt.Errorf("fast-forwarding subgraph %s: %w", subGraphName, ffErr)
			}
		}
	}

	mutationsDir := filepath.Join(r.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "mutations")
	entries, err = os.ReadDir(mutationsDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reading mutations directory %s: %w", mutationsDir, err)
	}
	var mutFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			mutFiles = append(mutFiles, e.Name())
		}
	}
	sort.Strings(mutFiles)

	for _, filename := range mutFiles {
		filePath := filepath.Join(mutationsDir, filename)
		data, readErr := os.ReadFile(filePath)
		if readErr != nil {
			return fmt.Errorf("reading mutation %s: %w", filePath, readErr)
		}
		mut, unmarshalErr := UnmarshalMutation(data)
		if unmarshalErr != nil {
			return fmt.Errorf("unmarshaling mutation %s: %w", filePath, unmarshalErr)
		}
		if saveErr := r.store.SaveMutation(ctx, c, mut); saveErr != nil {
			return fmt.Errorf("saving mutation %s: %w", filePath, saveErr)
		}
	}

	return nil
}

func (r *Rehydrator) parseDigestMarkdown(filename, content string) (DigestMeta, error) {
	idStr := strings.TrimSuffix(filename, ".md")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return DigestMeta{}, fmt.Errorf("invalid digest UUID filename %q: %w", filename, err)
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
							if evID, parseErr := uuid.Parse(p); parseErr == nil {
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

	return meta, nil
}

func (r *Rehydrator) parseSpaceContextMarkdown(filename, content string) (SpaceContext, error) {
	idStr := strings.TrimSuffix(filename, ".md")
	id, err := uuid.Parse(idStr)
	if err != nil {
		return SpaceContext{}, fmt.Errorf("invalid space context UUID filename %q: %w", filename, err)
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

	return sc, nil
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
