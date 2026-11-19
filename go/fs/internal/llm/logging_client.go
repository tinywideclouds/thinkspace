package llm

import (
	"context"
	"fmt"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
)

// LoggingModelClient is a transport-level ModelClient decorator that records the full
// compiled ThinkBundle to a temp Markdown file on every generation request and logs
// only a concise file pointer via slog.
type LoggingModelClient struct {
	logger   *slog.Logger
	debugDir string
	inner    ModelClient
}

func NewLoggingModelClient(logger *slog.Logger, debugDir string, inner ModelClient) *LoggingModelClient {
	if logger == nil {
		logger = slog.Default()
	}
	if strings.TrimSpace(debugDir) == "" {
		debugDir = filepath.Join(os.TempDir(), "thinkspace-llm")
	}
	return &LoggingModelClient{
		logger:   logger,
		debugDir: debugDir,
		inner:    inner,
	}
}

func sanitizeModelPrefix(model string) string {
	trimmed := strings.TrimSpace(model)
	if trimmed == "" {
		return "unknown"
	}
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}, trimmed)
}

func formatThinkBundleMarkdown(model string, bundle cognition.ThinkBundle) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# LLM Request Context\n\n")
	fmt.Fprintf(&b, "- **Timestamp:** %s\n", time.Now().UTC().Format(time.RFC3339Nano))
	fmt.Fprintf(&b, "- **Model:** `%s`\n", model)
	if len(bundle.ActiveTags) > 0 {
		fmt.Fprintf(&b, "- **Active Tags:** `%s`\n", strings.Join(bundle.ActiveTags, ", "))
	}
	b.WriteString("\n")

	b.WriteString("## 1. System Rules (Persona)\n\n```text\n")
	b.WriteString(bundle.Rules.Persona)
	b.WriteString("\n```\n\n")

	if bundle.Rules.FormattingConstraints != "" {
		b.WriteString("## 2. Formatting Constraints\n\n```text\n")
		b.WriteString(bundle.Rules.FormattingConstraints)
		b.WriteString("\n```\n\n")
	}

	b.WriteString("## 3. Mapbook Layers\n\n")
	if len(bundle.Mapbook.Maps) == 0 {
		b.WriteString("_None_\n\n")
	} else {
		layerNames := make([]string, 0, len(bundle.Mapbook.Maps))
		for k := range bundle.Mapbook.Maps {
			layerNames = append(layerNames, k)
		}
		sort.Strings(layerNames)
		for _, name := range layerNames {
			fmt.Fprintf(&b, "### Layer: `%s`\n\n```text\n%s\n```\n\n", name, bundle.Mapbook.Maps[name])
		}
	}

	fmt.Fprintf(&b, "## 4. Long-Term Memory (%d items)\n\n", len(bundle.Trajectory.LongTermMemory))
	if len(bundle.Trajectory.LongTermMemory) == 0 {
		b.WriteString("_None_\n\n")
	} else {
		for i, mem := range bundle.Trajectory.LongTermMemory {
			fmt.Fprintf(&b, "### Item %d\n\n```text\n%s\n```\n\n", i+1, mem)
		}
	}

	fmt.Fprintf(&b, "## 5. Working Memory (%d events)\n\n", len(bundle.Trajectory.WorkingMemory))
	if len(bundle.Trajectory.WorkingMemory) == 0 {
		b.WriteString("_None_\n\n")
	} else {
		for i, ev := range bundle.Trajectory.WorkingMemory {
			fmt.Fprintf(&b, "### Event %d (`%s`)\n\n```text\n%s\n```\n\n", i+1, ev.Role, ev.Content)
		}
	}

	fmt.Fprintf(&b, "## 6. Materialized Workbench (%d files)\n\n", len(bundle.Workbench))
	if len(bundle.Workbench) == 0 {
		b.WriteString("_None_\n\n")
	} else {
		for _, item := range bundle.Workbench {
			fmt.Fprintf(&b, "### `%s` (~%d tokens)\n\n```\n%s\n```\n\n", item.Path, item.TokenCount, item.Content)
		}
	}

	fmt.Fprintf(&b, "## 7. Catalyst (`%s`)\n\n```text\n%s\n```\n", bundle.Trajectory.Catalyst.Role, bundle.Trajectory.Catalyst.Content)
	return b.String()
}

func (c *LoggingModelClient) writeContextTempFile(model string, bundle cognition.ThinkBundle) (string, error) {
	if err := os.MkdirAll(c.debugDir, 0755); err != nil {
		return "", fmt.Errorf("creating llm debug temp directory %s: %w", c.debugDir, err)
	}

	pattern := fmt.Sprintf("llm-%s-*.md", sanitizeModelPrefix(model))
	tmpFile, err := os.CreateTemp(c.debugDir, pattern)
	if err != nil {
		return "", fmt.Errorf("creating llm debug temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	content := formatThinkBundleMarkdown(model, bundle)
	if _, writeErr := tmpFile.WriteString(content); writeErr != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("writing llm debug temp file: %w", writeErr)
	}
	if closeErr := tmpFile.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("closing llm debug temp file: %w", closeErr)
	}

	return tmpPath, nil
}

func (c *LoggingModelClient) GenerateContentStream(
	ctx context.Context,
	model string,
	bundle cognition.ThinkBundle,
	registry *tools.Registry,
	config spaces.ThinkStateConfig,
) iter.Seq2[StreamChunk, error] {
	if c == nil || c.inner == nil {
		return func(yield func(StreamChunk, error) bool) {
			yield(StreamChunk{}, fmt.Errorf("inner model client is nil"))
		}
	}

	logger := c.logger
	if logger == nil {
		logger = slog.Default()
	}

	filePath, err := c.writeContextTempFile(model, bundle)
	if err != nil {
		logger.WarnContext(ctx, "Failed recording LLM request context to temp file",
			"model", model,
			"error", err,
		)
	} else {
		logger.InfoContext(ctx, "Recorded LLM request context",
			"model", model,
			"file", filePath,
			"active_tags", bundle.ActiveTags,
			"workbench_files", len(bundle.Workbench),
			"working_memory_events", len(bundle.Trajectory.WorkingMemory),
		)
	}

	return c.inner.GenerateContentStream(ctx, model, bundle, registry, config)
}
