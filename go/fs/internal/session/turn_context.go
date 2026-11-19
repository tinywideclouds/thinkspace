package session

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
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

// BuildBundledWorkerThinkBundle merges multiple parallel sub-agent ThinkBundles into a single
// aggregate ThinkBundle for unified context review and token estimation.
func BuildBundledWorkerThinkBundle(tasks []flows.SubAgentTask, bundles []cognition.ThinkBundle) cognition.ThinkBundle {
	if len(bundles) == 0 {
		return cognition.ThinkBundle{}
	}

	combined := cognition.ThinkBundle{
		Rules:   bundles[0].Rules,
		Mapbook: bundles[0].Mapbook,
	}

	seenDigests := make(map[string]bool)
	seenPaths := make(map[string]bool)

	for i, b := range bundles {
		for _, ltm := range b.Trajectory.LongTermMemory {
			if ltm != "" && !seenDigests[ltm] {
				seenDigests[ltm] = true
				combined.Trajectory.LongTermMemory = append(combined.Trajectory.LongTermMemory, ltm)
			}
		}

		agentLabel := fmt.Sprintf("agent-%d", i+1)
		if i < len(tasks) && strings.TrimSpace(tasks[i].AgentID) != "" {
			agentLabel = tasks[i].AgentID
		}
		if b.Trajectory.Catalyst.Content != "" {
			combined.Trajectory.WorkingMemory = append(combined.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    agentLabel,
				Content: b.Trajectory.Catalyst.Content,
			})
		}

		for _, item := range b.Workbench {
			if !seenPaths[item.Path] {
				seenPaths[item.Path] = true
				combined.Workbench = append(combined.Workbench, item)
			}
		}
	}

	if len(combined.Trajectory.WorkingMemory) > 0 {
		last := combined.Trajectory.WorkingMemory[len(combined.Trajectory.WorkingMemory)-1]
		combined.Trajectory.WorkingMemory = combined.Trajectory.WorkingMemory[:len(combined.Trajectory.WorkingMemory)-1]
		combined.Trajectory.Catalyst = last
	}

	return combined
}

// FormatBundlePreviewMarkdown renders a human-readable Markdown snapshot of a ThinkBundle
// for persistence via Service.SaveContextPreview.
func FormatBundlePreviewMarkdown(title string, bundle *cognition.ThinkBundle) string {
	if bundle == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)

	b.WriteString("## System Persona\n\n```text\n")
	b.WriteString(bundle.Rules.Persona)
	b.WriteString("\n```\n\n")

	if bundle.Rules.FormattingConstraints != "" {
		b.WriteString("## Formatting Constraints\n\n```text\n")
		b.WriteString(bundle.Rules.FormattingConstraints)
		b.WriteString("\n```\n\n")
	}

	if len(bundle.Trajectory.LongTermMemory) > 0 {
		b.WriteString("## Long-Term Memory / Context Digests\n\n")
		for i, item := range bundle.Trajectory.LongTermMemory {
			fmt.Fprintf(&b, "### Digest %d\n\n%s\n\n", i+1, item)
		}
	}

	if len(bundle.Trajectory.WorkingMemory) > 0 {
		b.WriteString("## Working Memory\n\n")
		for _, ev := range bundle.Trajectory.WorkingMemory {
			fmt.Fprintf(&b, "- **[%s]**: %s\n", ev.Role, ev.Content)
		}
		b.WriteString("\n")
	}

	if len(bundle.Workbench) > 0 {
		b.WriteString("## Materialized Workbench Files\n\n")
		for _, item := range bundle.Workbench {
			fmt.Fprintf(&b, "### `%s` (~%d tokens)\n\n```\n%s\n```\n\n", item.Path, item.TokenCount, item.Content)
		}
	}

	if bundle.Trajectory.Catalyst.Content != "" {
		fmt.Fprintf(&b, "## Catalyst (`%s`)\n\n%s\n", bundle.Trajectory.Catalyst.Role, bundle.Trajectory.Catalyst.Content)
	}

	return b.String()
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
