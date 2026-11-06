package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FileLedger struct {
	MaxEventsPerShard int
	workspaceRoot     string
}

func NewFileLedger(maxEvents int, workspaceRoot string) *FileLedger {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	return &FileLedger{MaxEventsPerShard: maxEvents, workspaceRoot: workspaceRoot}
}

func (l *FileLedger) AppendEvent(ctx context.Context, c *ChatAddress, event Event) error {
	name := c.Name
	if name == "" {
		name = "main"
	}

	ledgerDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger")
	manifest, err := LoadOrRebuildManifest(ledgerDir, name)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if manifest.EventCount >= l.MaxEventsPerShard {
		var i int
		fmt.Sscanf(manifest.ActiveShard, "%d.jsonl", &i)
		manifest.ActiveShard = fmt.Sprintf("%05d.jsonl", i+1)
		manifest.EventCount = 0
	}

	// Lock in the mathematical DAG identity
	event.GraphID = c.GraphID

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	// Route to the semantic directory
	logsDir := filepath.Join(ledgerDir, "logs", name)
	_ = os.MkdirAll(logsDir, 0755)

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
	return manifest.Save(ledgerDir, name)
}

func (l *FileLedger) SaveDigest(ctx context.Context, c *ChatAddress, digest DigestMeta) error {
	digestsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "digests")
	_ = os.MkdirAll(digestsDir, 0755)

	isSticky := "false"
	if digest.Promoted {
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
	contextsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "contexts")
	_ = os.MkdirAll(contextsDir, 0755)

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

func (l *FileLedger) SaveMutation(ctx context.Context, c *ChatAddress, mutation MutationEvent) error {
	mutationsDir := filepath.Join(l.workspaceRoot, c.ProjectID, "chats", c.ChatID, "ledger", "mutations")
	_ = os.MkdirAll(mutationsDir, 0755)

	b, err := json.MarshalIndent(mutation, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling mutation: %w", err)
	}

	filename := fmt.Sprintf("%019d_%s.json", mutation.Timestamp.UnixNano(), mutation.TargetID.String())
	filePath := filepath.Join(mutationsDir, filename)
	return os.WriteFile(filePath, b, 0644)
}
