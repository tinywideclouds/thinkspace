package thinkgraph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type DigestSubscriber struct {
	workspaceRoot string
}

func NewDigestSubscriber(workspaceRoot string) *DigestSubscriber {
	return &DigestSubscriber{workspaceRoot: workspaceRoot}
}

func (d *DigestSubscriber) OnEvent(ctx context.Context, c *Chat, event Event) error {
	if event.Type != EventDigest {
		return nil
	}

	ledgerDir := filepath.Join(d.workspaceRoot, c.SpaceID, "chats", c.ID, "ledger")
	digestsDir := filepath.Join(ledgerDir, "digests")
	if err := os.MkdirAll(digestsDir, 0755); err != nil {
		return fmt.Errorf("creating digests directory: %w", err)
	}

	isSticky := "false"
	if val, ok := event.Metadata["is_sticky"]; ok && val == "true" {
		isSticky = "true"
	}

	var formattedTags []string
	for _, t := range event.Tags {
		formattedTags = append(formattedTags, fmt.Sprintf("%q", t))
	}
	tagsStr := "[" + strings.Join(formattedTags, ", ") + "]"

	var formattedSummarizes []string
	if val, ok := event.Metadata["summarizes"]; ok && val != "" {
		for _, id := range strings.Split(val, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				formattedSummarizes = append(formattedSummarizes, fmt.Sprintf("%q", id))
			}
		}
	}
	summarizesStr := "[" + strings.Join(formattedSummarizes, ", ") + "]"

	markdownContent := fmt.Sprintf("---\nid: %q\nsticky: %s\ntags: %s\nsummarizes: %s\n---\n%s\n",
		event.ID.String(), isSticky, tagsStr, summarizesStr, event.Content)

	filePath := filepath.Join(digestsDir, event.ID.String()+".md")
	return os.WriteFile(filePath, []byte(markdownContent), 0644)
}
