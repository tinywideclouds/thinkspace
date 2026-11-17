package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestScaffoldDefaults_FreshInstallAndVersionUpgrade(t *testing.T) {
	cfgDir := filepath.Join(t.TempDir(), "configs")

	// 1. Fresh scaffold creates all 5 default config files
	config.ScaffoldDefaults(nil, cfgDir)

	for _, name := range []string{"golang.yaml", "concept.yaml", "intake.yaml", "fanout.yaml", "digest.yaml"} {
		if _, err := os.Stat(filepath.Join(cfgDir, name)); err != nil {
			t.Fatalf("expected %s to be scaffolded: %v", name, err)
		}
	}

	// 2. Simulate an outdated version: 1 fanout.yaml on disk and verify ScaffoldDefaults upgrades it and writes .bak
	fanoutPath := filepath.Join(cfgDir, "fanout.yaml")
	oldContent := "fanout:\n  version: 1\n  type: flow\n  name: Old FanOut\n"
	if err := os.WriteFile(fanoutPath, []byte(oldContent), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	config.ScaffoldDefaults(nil, cfgDir)

	bakBytes, err := os.ReadFile(fanoutPath + ".bak")
	if err != nil {
		t.Fatalf("expected backup file fanout.yaml.bak to be created: %v", err)
	}
	if string(bakBytes) != oldContent {
		t.Errorf("expected backup to preserve old content, got %q", string(bakBytes))
	}

	upgradedBytes, err := os.ReadFile(fanoutPath)
	if err != nil {
		t.Fatalf("failed reading upgraded fanout.yaml: %v", err)
	}
	if !strings.Contains(string(upgradedBytes), "synthesis_review_prompt") {
		t.Errorf("expected upgraded fanout.yaml to contain version 3 synthesis_review_prompt")
	}
}

func TestApplyDevSkeleton(t *testing.T) {
	baseRoot := t.TempDir()
	mgr := workspace.NewServiceManager(nil, baseRoot, func(string) workspace.ChatVersioner { return nil }, nil, nil)

	config.ApplyDevSkeleton(nil, mgr)

	skeleton, err := config.GetEmbeddedSkeleton()
	if err != nil {
		t.Fatalf("GetEmbeddedSkeleton failed: %v", err)
	}

	meta, err := mgr.GetChatMeta(skeleton.Space, skeleton.Chat)
	if err != nil {
		t.Fatalf("expected skeleton chat meta to be created: %v", err)
	}
	if meta.ActiveThinkState != skeleton.ThinkState {
		t.Errorf("expected ActiveThinkState %q, got %q", skeleton.ThinkState, meta.ActiveThinkState)
	}

	state, err := mgr.GetSpaceState(skeleton.Space)
	if err != nil {
		t.Fatalf("expected skeleton space state to be created: %v", err)
	}
	if state.ThinkState != skeleton.ThinkState {
		t.Errorf("expected SpaceState.ThinkState %q, got %q", skeleton.ThinkState, state.ThinkState)
	}
}
