package concept

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	err := filepath.Walk(workspaceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}

		relPath, _ := filepath.Rel(workspaceRoot, path)

		// Skip internal chat histories
		if strings.HasPrefix(relPath, "chats") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		var fm Frontmatter
		if fmData := m.extractFrontmatter(string(content)); fmData != "" {
			_ = yaml.Unmarshal([]byte(fmData), &fm)
		}

		// Fallbacks if frontmatter is missing
		title := fm.Title
		if title == "" {
			title = filepath.Base(relPath)
		}

		tocBuilder.WriteString(fmt.Sprintf("- %s (%s)\n", filepath.ToSlash(relPath), title))
		if fm.Overview != "" {
			tocBuilder.WriteString(fmt.Sprintf("  Overview: %s\n", fm.Overview))
		}

		for _, tag := range fm.Tags {
			conceptIndex[tag] = append(conceptIndex[tag], filepath.ToSlash(relPath))
		}

		return nil
	})

	if err == nil {
		layers[spaces.LayerStructural] = tocBuilder.String()
	}

	var indexBuilder strings.Builder
	indexBuilder.WriteString("Conceptual Index (Tags to Files):\n")
	if len(conceptIndex) == 0 {
		indexBuilder.WriteString("No tagged files found yet.\n")
	} else {
		for tag, files := range conceptIndex {
			indexBuilder.WriteString(fmt.Sprintf("#%s -> %s\n", tag, strings.Join(files, ", ")))
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
