package session_test

import (
	"context"
	"strings"
	"testing"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/assembler"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
	"google.golang.org/genai"
)

type mockMapbook struct{}

func (m *mockMapbook) GenerateLayers(ctx context.Context, workspaceRoot string) (map[assembler.MapbookLayer]string, error) {
	return map[assembler.MapbookLayer]string{
		assembler.LayerStructural: "- src/\n  - main.go",
		assembler.LayerConceptual: "Strict Domain Rules Apply",
	}, nil
}

type mockSpaceForFusion struct{}

func (m *mockSpaceForFusion) Config() spaces.ThinkSpaceConfig {
	return spaces.ThinkSpaceConfig{
		SystemPrompt: "Base System Prompt",
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{
				SystemPrompt: "Manager Role Rules",
			},
		},
	}
}
func (m *mockSpaceForFusion) Tools() []*genai.Tool       { return nil }
func (m *mockSpaceForFusion) Verifier() spaces.Verifier  { return nil }
func (m *mockSpaceForFusion) Mapbook() assembler.Mapbook { return &mockMapbook{} }
func (m *mockSpaceForFusion) Patcher() workspace.Patcher { return nil }

func TestContextAssembler_Build(t *testing.T) {
	ca := session.NewContextAssembler()
	ctx := context.Background()

	manifest := chat.NewThreadManifest("test-thread")
	stickyID := uuid.NewV7()
	manifest.Digests[stickyID] = chat.DigestMeta{
		ID:       stickyID,
		Summary:  "Sticky Summary Context",
		IsSticky: true,
	}

	req := chat.AssemblyRequest{
		Manifest:     manifest,
		ActiveLenses: []string{},
		RecentEvents: []chat.Event{
			chat.NewEvent(chat.EventPrompt, "Recent human prompt"),
		},
	}

	space := &mockSpaceForFusion{}

	sysPrompt, history, err := ca.Build(ctx, space, "/tmp/workspace", req)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if !strings.Contains(sysPrompt, "Base System Prompt\n\nManager Role Rules") {
		t.Errorf("Expected base system prompt and manager role, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "### THE MAPBOOK (WORKSPACE CONTEXT)") {
		t.Errorf("Expected mapbook header, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "- src/\n  - main.go") {
		t.Errorf("Expected structural layer, got: %s", sysPrompt)
	}
	if !strings.Contains(sysPrompt, "Strict Domain Rules Apply") {
		t.Errorf("Expected conceptual layer, got: %s", sysPrompt)
	}

	if len(history) != 3 {
		t.Fatalf("Expected 3 history items (sticky, tags, recent), got %d", len(history))
	}
	if !strings.Contains(history[0].Parts[0].Text, "Sticky Summary Context") {
		t.Errorf("Expected sticky summary")
	}
	if !strings.Contains(history[1].Parts[0].Text, "[No historical tags currently defined in workspace]") {
		t.Errorf("Expected system tag warning")
	}
	if history[2].Parts[0].Text != "Recent human prompt" {
		t.Errorf("Expected recent prompt")
	}
}
