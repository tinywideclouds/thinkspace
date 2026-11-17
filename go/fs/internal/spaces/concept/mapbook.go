package concept

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"gopkg.in/yaml.v3"
)

type ConceptMapbook struct{}

func NewConceptMapbook() *ConceptMapbook {
	return &ConceptMapbook{}
}

type Frontmatter struct {
	Title    string   `yaml:"title"`
	Overview string   `yaml:"overview"`
	Tags     []string `yaml:"tags"`
}

func (m *ConceptMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[spaces.MapbookLayer]string, error) {
	layers := make(map[spaces.MapbookLayer]string)

	var tocBuilder strings.Builder
	tocBuilder.WriteString("Table of Contents:\n")

	conceptIndex := make(map[string][]string) // tag -> list of files

	err := filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if path == workspaceRoot && errors.Is(walkErr, os.ErrNotExist) {
				return filepath.SkipAll
			}
			return walkErr
		}

		if d.IsDir() {
			if path != workspaceRoot && (d.Name() == "chats" || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}

		relPath, relErr := filepath.Rel(workspaceRoot, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("reading markdown file %s: %w", relPath, readErr)
		}

		var fm Frontmatter
		if fmData := m.extractFrontmatter(string(content)); fmData != "" {
			if unmarshalErr := yaml.Unmarshal([]byte(fmData), &fm); unmarshalErr != nil {
				return fmt.Errorf("parsing frontmatter in %s: %w", filepath.ToSlash(relPath), unmarshalErr)
			}
		}

		title := fm.Title
		if title == "" {
			title = filepath.Base(relPath)
		}

		slashPath := filepath.ToSlash(relPath)
		tocBuilder.WriteString(fmt.Sprintf("- %s (%s)\n", slashPath, title))
		if fm.Overview != "" {
			tocBuilder.WriteString(fmt.Sprintf("  Overview: %s\n", fm.Overview))
		}

		for _, tag := range fm.Tags {
			if trimmed := strings.TrimSpace(tag); trimmed != "" {
				conceptIndex[trimmed] = append(conceptIndex[trimmed], slashPath)
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking workspace for concept mapbook: %w", err)
	}

	layers[spaces.LayerStructural] = tocBuilder.String()

	var indexBuilder strings.Builder
	indexBuilder.WriteString("Conceptual Index (Tags to Files):\n")
	if len(conceptIndex) == 0 {
		indexBuilder.WriteString("No tagged files found yet.\n")
	} else {
		tags := make([]string, 0, len(conceptIndex))
		for tag := range conceptIndex {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		for _, tag := range tags {
			indexBuilder.WriteString(fmt.Sprintf("#%s -> %s\n", tag, strings.Join(conceptIndex[tag], ", ")))
		}
	}
	layers[spaces.LayerConceptual] = indexBuilder.String()

	return layers, nil
}

func (m *ConceptMapbook) extractFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	parts := strings.SplitN(content, "---", 3)
	if len(parts) >= 3 {
		return strings.TrimSpace(parts[1])
	}
	return ""
}
