package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type FileLedger struct {
	mu                sync.Mutex
	MaxEventsPerShard int
	workspaceRoot     string
}

func NewFileLedger(maxEvents int, workspaceRoot string) *FileLedger {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	return &FileLedger{MaxEventsPerShard: maxEvents, workspaceRoot: workspaceRoot}
}

func ResolveLogDirName(subgraphName string) string {
	if subgraphName == "" {
		return "main"
	}
	if strings.HasPrefix(subgraphName, "flow-") {
		parts := strings.Split(subgraphName, "/")
		return parts[0]
	}
	return strings.ReplaceAll(subgraphName, "/", "_")
}

func (l *FileLedger) AppendEvent(ctx context.Context, c *ChatAddress, event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	name := c.Name
	if name == "" {
		name = "main"
	}

	ledgerDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger")
	dirName := ResolveLogDirName(name)

	manifest, err := LoadOrRebuildManifest(ledgerDir, dirName)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if manifest.EventCount >= l.MaxEventsPerShard {
		var i int
		if _, scanErr := fmt.Sscanf(manifest.ActiveShard, "%d.jsonl", &i); scanErr != nil {
			return fmt.Errorf("parsing active shard filename %q: %w", manifest.ActiveShard, scanErr)
		}
		manifest.ActiveShard = fmt.Sprintf("%05d.jsonl", i+1)
		manifest.EventCount = 0
	}

	if event.GraphID.IsZero() {
		event.GraphID = c.GraphID
	}

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	logsDir := filepath.Join(ledgerDir, "logs", dirName)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("creating logs directory %s: %w", logsDir, err)
	}

	shardPath := filepath.Join(logsDir, manifest.ActiveShard)
	f, err := os.OpenFile(shardPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening ledger shard: %w", err)
	}

	if _, err := f.Write(append(b, '\n')); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing event: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing ledger shard: %w", err)
	}

	manifest.EventCount++
	manifest.TotalEvents++
	return manifest.Save(ledgerDir, dirName)
}

func (l *FileLedger) SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	digestsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "digests")
	if err := os.MkdirAll(digestsDir, 0755); err != nil {
		return fmt.Errorf("creating digests directory %s: %w", digestsDir, err)
	}

	isSticky := "false"
	if digest.IsSticky {
		isSticky = "true"
	}

	var sumStrs []string
	for _, id := range digest.SummarizedEvents {
		sumStrs = append(sumStrs, fmt.Sprintf(`"%s"`, id.String()))
	}
	summarizesStr := "[]"
	if len(sumStrs) > 0 {
		summarizesStr = "[" + strings.Join(sumStrs, ", ") + "]"
	}

	var tagStrs []string
	for _, tag := range digest.Tags {
		tagStrs = append(tagStrs, fmt.Sprintf(`"%s"`, tag))
	}
	tagsStr := "[]"
	if len(tagStrs) > 0 {
		tagsStr = "[" + strings.Join(tagStrs, ", ") + "]"
	}

	content := fmt.Sprintf("---\nid: \"%s\"\nsticky: %s\ntags: %s\nsummarizes: %s\ntombstoned: %v\n---\n%s\n",
		digest.ID.String(), isSticky, tagsStr, summarizesStr, digest.Tombstoned, digest.Summary)

	filePath := filepath.Join(digestsDir, digest.ID.String()+".md")
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (l *FileLedger) SaveSpaceContext(ctx context.Context, c *ChatAddress, spaceCtx SpaceContext) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	contextsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "contexts")
	if err := os.MkdirAll(contextsDir, 0755); err != nil {
		return fmt.Errorf("creating contexts directory %s: %w", contextsDir, err)
	}

	var tagStrs []string
	for _, tag := range spaceCtx.Tags {
		tagStrs = append(tagStrs, fmt.Sprintf(`"%s"`, tag))
	}
	tagsStr := "[]"
	if len(tagStrs) > 0 {
		tagsStr = "[" + strings.Join(tagStrs, ", ") + "]"
	}

	content := fmt.Sprintf("---\nid: \"%s\"\ntags: %s\ntombstoned: %v\n---\n%s\n",
		spaceCtx.ID.String(), tagsStr, spaceCtx.Tombstoned, spaceCtx.Content)

	filePath := filepath.Join(contextsDir, spaceCtx.ID.String()+".md")
	return os.WriteFile(filePath, []byte(content), 0644)
}

func (l *FileLedger) SaveMutation(ctx context.Context, c *ChatAddress, mutation Mutator) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	mutationsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "mutations")
	if err := os.MkdirAll(mutationsDir, 0755); err != nil {
		return fmt.Errorf("creating mutations directory %s: %w", mutationsDir, err)
	}

	b, err := json.MarshalIndent(mutation, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mutation: %w", err)
	}

	filename := fmt.Sprintf("%019d_%s.json", mutation.Time().UnixNano(), mutation.Target().String())
	filePath := filepath.Join(mutationsDir, filename)
	return os.WriteFile(filePath, b, 0644)
}
