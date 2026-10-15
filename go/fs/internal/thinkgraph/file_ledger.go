package thinkgraph

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileLedger implements the Ledger interface, writing directly to the file system.
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

func (l *FileLedger) AppendEvent(ctx context.Context, c *Chat, event Event) error {
	ledgerDir := filepath.Join(l.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")
	manifest, err := LoadOrRebuildManifest(ledgerDir)
	if err != nil {
		return fmt.Errorf("loading manifest: %w", err)
	}

	if manifest.EventCount >= l.MaxEventsPerShard {
		var currentNum int
		fmt.Sscanf(manifest.ActiveShard, "%d.jsonl", &currentNum)
		manifest.ActiveShard = fmt.Sprintf("%05d.jsonl", currentNum+1)
		manifest.EventCount = 0
	}

	b, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	logsDir := filepath.Join(ledgerDir, "logs")
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
	return manifest.Save(ledgerDir)
}

func (l *FileLedger) SaveDigest(ctx context.Context, c *Chat, digest DigestMeta) error {
	digestsDir := filepath.Join(l.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger", "digests")
	_ = os.MkdirAll(digestsDir, 0755)

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

	content := fmt.Sprintf("---\nid: \"%s\"\nsticky: %s\nsummarizes: %s\n---\n%s\n",
		digest.ID.String(), isSticky, summarizesStr, digest.Summary)

	filePath := filepath.Join(digestsDir, digest.ID.String()+".md")
	return os.WriteFile(filePath, []byte(content), 0644)
}
