package session

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// ProjectTurnContext projects a domain ThinkBundle and attention focus into a presentation-layer TurnContext.
func ProjectTurnContext(spaceID, chatID string, responseID uuid.UUID, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus) TurnContext {
	var recentHistory []string
	if bundle != nil {
		for _, memory := range bundle.Trajectory.WorkingMemory {
			recentHistory = append(recentHistory, fmt.Sprintf("[%s] %s", memory.Role, memory.Content))
		}
		if bundle.Trajectory.Catalyst.Content != "" {
			recentHistory = append(recentHistory, fmt.Sprintf("[catalyst] %s", bundle.Trajectory.Catalyst.Content))
		}
	}

	var fileContexts []FileContext
	seenPaths := make(map[string]bool)

	if bundle != nil {
		for _, item := range bundle.Workbench {
			seenPaths[item.Path] = true
			fileContexts = append(fileContexts, FileContext{
				Path:       item.Path,
				Level:      "injected",
				TokenCount: item.TokenCount,
			})
		}

		for _, mf := range extractMapbookFileContexts(bundle.Mapbook.Maps, seenPaths) {
			seenPaths[mf.Path] = true
			fileContexts = append(fileContexts, mf)
		}
	}

	turnContext := TurnContext{
		SpaceID:       spaceID,
		ChatID:        chatID,
		ResponseID:    responseID,
		RecentHistory: recentHistory,
		FileContexts:  fileContexts,
		Focus:         focus,
	}

	if bundle != nil {
		turnContext.ManagerSystemPrompt = bundle.Rules.Persona
		turnContext.StickyDigests = bundle.Trajectory.LongTermMemory
	}

	return turnContext
}

func extractMapbookFileContexts(maps map[string]string, alreadySeen map[string]bool) []FileContext {
	if len(maps) == 0 {
		return nil
	}

	layers := make([]string, 0, len(maps))
	for k := range maps {
		layers = append(layers, k)
	}
	// Prefer skeleton/ast classification before structural if a file appears in both
	sort.Slice(layers, func(i, j int) bool {
		li, lj := strings.ToLower(layers[i]), strings.ToLower(layers[j])
		if li != lj {
			if strings.Contains(li, "skeleton") || strings.Contains(li, "ast") {
				return true
			}
			if strings.Contains(lj, "skeleton") || strings.Contains(lj, "ast") {
				return false
			}
		}
		return li < lj
	})

	var results []FileContext
	localSeen := make(map[string]bool)

	for _, layer := range layers {
		content := maps[layer]
		if strings.TrimSpace(content) == "" {
			continue
		}

		level := "structural"
		lowerLayer := strings.ToLower(layer)
		if strings.Contains(lowerLayer, "skeleton") || strings.Contains(lowerLayer, "ast") {
			level = "skeleton"
		}

		for _, rawLine := range strings.Split(content, "\n") {
			candidate := parseFilePathCandidate(rawLine)
			if candidate == "" || alreadySeen[candidate] || localSeen[candidate] {
				continue
			}
			localSeen[candidate] = true
			results = append(results, FileContext{
				Path:       candidate,
				Level:      level,
				TokenCount: 0,
			})
		}
	}

	return results
}

func parseFilePathCandidate(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	// Strip common section headers (e.g., "// File: pkg/foo.go", "=== pkg/foo.go ===", "--- pkg/foo.go ---")
	for _, prefix := range []string{"// File:", "// file:", "File:", "file:", "===", "---"} {
		if strings.HasPrefix(trimmed, prefix) {
			trimmed = strings.TrimPrefix(trimmed, prefix)
			trimmed = strings.TrimSuffix(trimmed, "===")
			trimmed = strings.TrimSuffix(trimmed, "---")
			trimmed = strings.TrimSpace(trimmed)
			break
		}
	}

	// Strip ASCII/Unicode tree connectors
	trimmed = strings.TrimLeft(trimmed, "│├└─ \t")
	trimmed = strings.TrimSpace(trimmed)

	if trimmed == "" || strings.HasSuffix(trimmed, "/") || strings.ContainsAny(trimmed, " \t(){};=") {
		return ""
	}

	base := filepath.Base(trimmed)
	ext := filepath.Ext(base)
	if ext != "" || base == "Dockerfile" || base == "Makefile" {
		return filepath.ToSlash(trimmed)
	}

	return ""
}
