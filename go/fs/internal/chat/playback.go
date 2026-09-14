package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// PlaybackEngine reads the immutable ledger and reconstructs the working state graph.
type PlaybackEngine struct{}

func NewPlaybackEngine() *PlaybackEngine {
	return &PlaybackEngine{}
}

// LoadState streams the ledger from disk, separating heavy semantic digests from the raw recent tail.
func (engine *PlaybackEngine) LoadState(ctx context.Context, thread *Thread) (*ChatGraph, *ThreadManifest, error) {
	file, err := os.Open(thread.LedgerPath)
	if err != nil {
		if os.IsNotExist(err) {
			// A brand new chat thread simply starts with an empty graph
			return &ChatGraph{}, NewThreadManifest(thread.ID), nil
		}
		return nil, nil, fmt.Errorf("opening ledger: %w", err)
	}
	defer file.Close()

	manifest := NewThreadManifest(thread.ID)
	graph := &ChatGraph{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			// In a production system we would log this, but we gracefully skip malformed legacy lines
			continue
		}

		if event.Type == EventDigest {
			isSticky := false
			if event.Metadata != nil && event.Metadata["is_sticky"] == "true" {
				isSticky = true
			}

			digest := DigestMeta{
				ID:       event.ID,
				Summary:  event.Content,
				IsSticky: isSticky,
			}

			manifest.Digests[event.ID] = digest
			graph.Digests = append(graph.Digests, digest)
		} else {
			graph.RecentEvents = append(graph.RecentEvents, event)
		}

		// Map historical tags to their Event IDs so they can be retrieved via the query_lens tool
		for _, tag := range event.Tags {
			manifest.Lenses[tag] = append(manifest.Lenses[tag], event.ID)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("scanning ledger: %w", err)
	}

	return graph, manifest, nil
}
