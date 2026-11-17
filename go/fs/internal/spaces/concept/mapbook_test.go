package concept_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces/concept"
)

func TestConceptMapbook_GenerateLayers(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	// Create docs with and without frontmatter, plus a chats/ directory that must be skipped
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "chats", "chat-1"), 0755); err != nil {
		t.Fatalf("MkdirAll(chats) failed: %v", err)
	}

	archDoc := `---
title: System Architecture
overview: High-level design overview
tags: [zeta, alpha]
---
# Architecture
`
	if err := os.WriteFile(filepath.Join(root, "docs", "arch.md"), []byte(archDoc), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "chats_notes.md"), []byte("# Root Notes"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "chats", "chat-1", "ignored.md"), []byte("# Ignored"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	mb := concept.NewConceptMapbook()
	layers, err := mb.GenerateLayers(ctx, root)
	if err != nil {
		t.Fatalf("GenerateLayers failed: %v", err)
	}

	structural := layers[spaces.LayerStructural]
	if !strings.Contains(structural, "- docs/arch.md (System Architecture)") {
		t.Errorf("expected frontmatter title in structural layer, got:\n%s", structural)
	}
	if !strings.Contains(structural, "Overview: High-level design overview") {
		t.Errorf("expected overview in structural layer, got:\n%s", structural)
	}
	if !strings.Contains(structural, "- chats_notes.md (chats_notes.md)") {
		t.Errorf("expected root file prefixed with 'chats_' not to be skipped, got:\n%s", structural)
	}
	if strings.Contains(structural, "ignored.md") {
		t.Errorf("expected internal chats/ directory to be skipped, got:\n%s", structural)
	}

	conceptual := layers[spaces.LayerConceptual]
	alphaIdx := strings.Index(conceptual, "#alpha -> docs/arch.md")
	zetaIdx := strings.Index(conceptual, "#zeta -> docs/arch.md")
	if alphaIdx == -1 || zetaIdx == -1 || alphaIdx > zetaIdx {
		t.Errorf("expected deterministically sorted tags #alpha before #zeta, got:\n%s", conceptual)
	}
}

func TestConceptMapbook_InvalidFrontmatterReturnsError(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()

	badDoc := "---\ntitle: [unclosed\n---\nBody"
	if err := os.WriteFile(filepath.Join(root, "bad.md"), []byte(badDoc), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	mb := concept.NewConceptMapbook()
	if _, err := mb.GenerateLayers(ctx, root); err == nil {
		t.Fatalf("expected error on malformed YAML frontmatter, got nil")
	}
}
