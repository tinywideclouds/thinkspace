package session_test

import (
	"strings"
	"testing"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func TestProjectTurnContext(t *testing.T) {
	spaceID := "space-1"
	chatID := "chat-1"
	responseID := uuid.NewV7()
	focus := thinkgraph.AttentionFocus{
		PrimaryFocus: []string{"backend"},
	}

	bundle := &cognition.ThinkBundle{
		Rules: cognition.SystemRules{Persona: "System instructions"},
		Mapbook: cognition.Mapbook{
			Maps: map[string]string{
				"structural": "├── go.mod\n├── main.go\n└── internal/handler/hello.go",
				"skeleton":   "// File: internal/handler/hello.go\npackage handler\nfunc Hello() {}",
			},
		},
		Trajectory: cognition.Trajectory{
			LongTermMemory: []string{"Sticky digest entry"},
			WorkingMemory: []cognition.TrajectoryEvent{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: "Build the server",
			},
		},
		Workbench: []cognition.WorkbenchItem{
			{Path: "main.go", TokenCount: 150},
		},
	}

	turnContext := session.ProjectTurnContext(spaceID, chatID, responseID, bundle, focus)

	if turnContext.SpaceID != spaceID {
		t.Errorf("expected SpaceID %s, got %s", spaceID, turnContext.SpaceID)
	}
	if turnContext.ChatID != chatID {
		t.Errorf("expected ChatID %s, got %s", chatID, turnContext.ChatID)
	}
	if turnContext.ResponseID != responseID {
		t.Errorf("expected ResponseID %s, got %s", responseID, turnContext.ResponseID)
	}
	if turnContext.ManagerSystemPrompt != "System instructions" {
		t.Errorf("expected ManagerSystemPrompt 'System instructions', got %s", turnContext.ManagerSystemPrompt)
	}
	if len(turnContext.StickyDigests) != 1 || turnContext.StickyDigests[0] != "Sticky digest entry" {
		t.Errorf("unexpected StickyDigests: %v", turnContext.StickyDigests)
	}
	if len(turnContext.RecentHistory) != 3 {
		t.Fatalf("expected 3 history entries (including catalyst), got %d", len(turnContext.RecentHistory))
	}
	if turnContext.RecentHistory[0] != "[user] Hello" || turnContext.RecentHistory[1] != "[assistant] Hi there" {
		t.Errorf("unexpected WorkingMemory formatting: %v", turnContext.RecentHistory)
	}
	if turnContext.RecentHistory[2] != "[catalyst] Build the server" {
		t.Errorf("unexpected Catalyst formatting: %v", turnContext.RecentHistory[2])
	}

	// Expect main.go (injected, 150t) + internal/handler/hello.go (skeleton, 0t) + go.mod (structural, 0t)
	if len(turnContext.FileContexts) != 3 {
		t.Fatalf("expected 3 deduplicated file contexts, got %d: %+v", len(turnContext.FileContexts), turnContext.FileContexts)
	}
	if turnContext.FileContexts[0].Path != "main.go" || turnContext.FileContexts[0].Level != "injected" || turnContext.FileContexts[0].TokenCount != 150 {
		t.Errorf("unexpected first FileContext: %+v", turnContext.FileContexts[0])
	}
	if turnContext.FileContexts[1].Path != "internal/handler/hello.go" || turnContext.FileContexts[1].Level != "skeleton" {
		t.Errorf("unexpected second FileContext: %+v", turnContext.FileContexts[1])
	}
	if turnContext.FileContexts[2].Path != "go.mod" || turnContext.FileContexts[2].Level != "structural" {
		t.Errorf("unexpected third FileContext: %+v", turnContext.FileContexts[2])
	}
	if len(turnContext.Focus.PrimaryFocus) != 1 || turnContext.Focus.PrimaryFocus[0] != "backend" {
		t.Errorf("unexpected Focus: %v", turnContext.Focus)
	}
}

func TestBuildBundledWorkerThinkBundleAndFormatMarkdown(t *testing.T) {
	tasks := []flows.SubAgentTask{
		{AgentID: "agent-1", Instruction: "Iterative approach"},
		{AgentID: "agent-2", Instruction: "Recursive approach"},
	}
	bundles := []cognition.ThinkBundle{
		{
			Rules: cognition.SystemRules{Persona: "Worker rules", FormattingConstraints: "JSON patch"},
			Trajectory: cognition.Trajectory{
				LongTermMemory: []string{"Shared project digest"},
				Catalyst:       cognition.TrajectoryEvent{Role: "user", Content: "Iterative approach"},
			},
			Workbench: []cognition.WorkbenchItem{
				{Path: "src/main.go", Content: "package main", TokenCount: 3},
			},
		},
		{
			Rules: cognition.SystemRules{Persona: "Worker rules"},
			Trajectory: cognition.Trajectory{
				LongTermMemory: []string{"Shared project digest"},
				Catalyst:       cognition.TrajectoryEvent{Role: "user", Content: "Recursive approach"},
			},
			Workbench: []cognition.WorkbenchItem{
				{Path: "src/main.go", Content: "package main", TokenCount: 3},
				{Path: "src/helper.go", Content: "package main\nfunc helper() {}", TokenCount: 7},
			},
		},
	}

	combined := session.BuildBundledWorkerThinkBundle(tasks, bundles)
	if len(combined.Trajectory.LongTermMemory) != 1 {
		t.Errorf("expected deduplicated LongTermMemory across bundled workers, got %d", len(combined.Trajectory.LongTermMemory))
	}
	if len(combined.Workbench) != 2 {
		t.Fatalf("expected 2 deduplicated workbench files across bundled workers, got %d", len(combined.Workbench))
	}
	if len(combined.Trajectory.WorkingMemory) != 1 || combined.Trajectory.WorkingMemory[0].Role != "agent-1" {
		t.Errorf("expected agent-1 instruction in WorkingMemory, got %+v", combined.Trajectory.WorkingMemory)
	}
	if combined.Trajectory.Catalyst.Role != "agent-2" || combined.Trajectory.Catalyst.Content != "Recursive approach" {
		t.Errorf("expected agent-2 instruction as Catalyst, got %+v", combined.Trajectory.Catalyst)
	}

	md := session.FormatBundlePreviewMarkdown("Sub-Agent Bundle Preview", &combined)
	for _, want := range []string{"# Sub-Agent Bundle Preview", "Worker rules", "JSON patch", "Shared project digest", "src/main.go", "src/helper.go", "Recursive approach"} {
		if !strings.Contains(md, want) {
			t.Errorf("expected preview markdown to contain %q, got:\n%s", want, md)
		}
	}
}
