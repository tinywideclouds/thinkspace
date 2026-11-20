package session

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// --- Mocks for Assembly Testing ---

type baMockGraphRepo struct {
	thinkgraph.GraphRepository
	nodes   []thinkgraph.Node
	digests []thinkgraph.DigestMeta
}

func (m *baMockGraphRepo) WalkSubGraphReverse(ctx context.Context, c *thinkgraph.ChatAddress, graphID thinkgraph.GraphID, startCursor string, fn func(thinkgraph.Node) bool) error {
	for i := len(m.nodes) - 1; i >= 0; i-- {
		if !fn(m.nodes[i]) {
			break
		}
	}
	return nil
}

func (m *baMockGraphRepo) GetStickyDigests(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.DigestMeta, error) {
	var sticky []thinkgraph.DigestMeta
	for _, d := range m.digests {
		if d.IsSticky {
			sticky = append(sticky, d)
		}
	}
	return sticky, nil
}

func (m *baMockGraphRepo) GetDigests(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.DigestMeta, error) {
	return m.digests, nil
}

func (m *baMockGraphRepo) GetSpaceContexts(ctx context.Context, c *thinkgraph.ChatAddress) ([]thinkgraph.SpaceContext, error) {
	return nil, nil
}

func (m *baMockGraphRepo) GetNodes(ctx context.Context, c *thinkgraph.ChatAddress, ids []string) ([]thinkgraph.Node, error) {
	var result []thinkgraph.Node
	for _, id := range ids {
		for _, n := range m.nodes {
			if n.ID.String() == id {
				result = append(result, n)
			}
		}
	}
	return result, nil
}

type baMockThinkSpace struct {
	lastMapbookRoot string
}

func (s *baMockThinkSpace) Config() spaces.ThinkStateConfig {
	cfg := spaces.ThinkStateConfig{
		Roles: spaces.RolesConfig{
			Manager: spaces.ManagerConfig{SystemPrompt: "test prompt"},
		},
	}
	cfg.ApplyDefaults()
	return cfg
}
func (s *baMockThinkSpace) Verifier() spaces.Verifier { return nil }
func (s *baMockThinkSpace) Mapbook() spaces.Mapbook   { return s }
func (s *baMockThinkSpace) GenerateLayers(ctx context.Context, root string) (map[spaces.MapbookLayer]string, error) {
	s.lastMapbookRoot = root
	return map[spaces.MapbookLayer]string{
		spaces.LayerStructural: "├── src/go.mod",
	}, nil
}
func (s *baMockThinkSpace) Patcher() workspace.Patcher                             { return nil }
func (s *baMockThinkSpace) Initialize(ctx context.Context, directory string) error { return nil }
func (s *baMockThinkSpace) EnrichBundle(ctx context.Context, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus, workspaceRoot string) error {
	return nil
}

// --- Tests ---

func TestBundleAssembler_ResolvesWorkbenchLocationForMapbook(t *testing.T) {
	ctx := context.Background()
	baseRoot := t.TempDir()
	repo := &baMockGraphRepo{
		nodes: []thinkgraph.Node{
			{Event: thinkgraph.Event{ID: uuid.NewV7(), Type: thinkgraph.EventPrompt, Content: "Add auth", Timestamp: time.Now()}},
		},
	}

	assembler := NewBundleAssembler(nil, repo, thinkgraph.NewContextProjecter(), config.NewRegistry(), baseRoot)
	chatObj := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}
	space := &baMockThinkSpace{}

	expectedWorkbenchDir := filepath.Join(baseRoot, "space-1", workspace.WorkbenchLocation)

	bundle, _, err := assembler.Assemble(ctx, chatObj, uuid.Nil(), uuid.Nil(), "golang", space)
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}
	if space.lastMapbookRoot != expectedWorkbenchDir {
		t.Errorf("expected Assemble to generate Mapbook from %q, got %q", expectedWorkbenchDir, space.lastMapbookRoot)
	}
	if bundle.Mapbook.Maps[string(spaces.LayerStructural)] != "├── src/go.mod" {
		t.Errorf("expected structural Mapbook layer to be populated, got %+v", bundle.Mapbook.Maps)
	}

	space.lastMapbookRoot = ""
	workerBundle := assembler.AssembleWorkerBundle(ctx, "space-1", space, "rules", "digest", "instruction", nil)
	if space.lastMapbookRoot != expectedWorkbenchDir {
		t.Errorf("expected AssembleWorkerBundle to generate Mapbook from %q, got %q", expectedWorkbenchDir, space.lastMapbookRoot)
	}
	if workerBundle.Mapbook.Maps[string(spaces.LayerStructural)] != "├── src/go.mod" {
		t.Errorf("expected worker structural Mapbook layer to be populated, got %+v", workerBundle.Mapbook.Maps)
	}
}

func TestBundleAssembler_Assemble_CatalystResolution(t *testing.T) {
	ctx := context.Background()

	// Create nodes to simulate a timeline
	catalystID := uuid.NewV7()
	oldPromptID := uuid.NewV7()

	mockNodes := []thinkgraph.Node{
		{Event: thinkgraph.Event{ID: oldPromptID, Type: thinkgraph.EventPrompt, Content: "Old prompt", Timestamp: time.Now().Add(-2 * time.Minute)}},
		{Event: thinkgraph.Event{ID: uuid.NewV7(), Type: thinkgraph.EventModel, Content: "Old response", Timestamp: time.Now().Add(-1 * time.Minute)}},
		{Event: thinkgraph.Event{ID: catalystID, Type: thinkgraph.EventPrompt, Content: "Active explicit catalyst", Timestamp: time.Now()}},
	}

	repo := &baMockGraphRepo{nodes: mockNodes}
	registry := config.NewRegistry()
	projecter := thinkgraph.NewContextProjecter()
	assembler := NewBundleAssembler(nil, repo, projecter, registry, "/tmp")

	chatObj := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}
	thinkState := &baMockThinkSpace{}

	t.Run("Resolves Explicit CatalystID and Deduplicates Timeline", func(t *testing.T) {
		bundle, graphData, err := assembler.Assemble(ctx, chatObj, uuid.Nil(), catalystID, "golang", thinkState)
		if err != nil {
			t.Fatalf("Assemble failed: %v", err)
		}

		// 1. Ensure the catalyst was explicitly assigned
		if bundle.Trajectory.Catalyst.Content != "Active explicit catalyst" {
			t.Errorf("Expected catalyst content 'Active explicit catalyst', got %q", bundle.Trajectory.Catalyst.Content)
		}

		// 2. Ensure working memory doesn't redundantly include the catalyst
		for _, mem := range bundle.Trajectory.WorkingMemory {
			if mem.Content == "Active explicit catalyst" {
				t.Errorf("Catalyst node was not filtered out of working memory")
			}
		}

		// 3. Ensure prior history remains intact
		if len(bundle.Trajectory.WorkingMemory) != 2 {
			t.Errorf("Expected 2 historical nodes in working memory, got %d", len(bundle.Trajectory.WorkingMemory))
		}

		// 4. Ensure graphData returned to the caller also excludes the catalyst
		for _, n := range graphData.Timeline {
			if n.ID == catalystID {
				t.Errorf("Catalyst node leaked into graphData.Timeline")
			}
		}
	})

	t.Run("Falls Back to Timeline Tail When CatalystID is Nil", func(t *testing.T) {
		bundle, _, err := assembler.Assemble(ctx, chatObj, uuid.Nil(), uuid.Nil(), "golang", thinkState)
		if err != nil {
			t.Fatalf("Assemble failed: %v", err)
		}

		// Without an explicit ID, it should naturally peel the final node off the timeline (Catalyst fallback)
		if bundle.Trajectory.Catalyst.Content != "Active explicit catalyst" {
			t.Errorf("Expected fallback catalyst content 'Active explicit catalyst', got %q", bundle.Trajectory.Catalyst.Content)
		}
	})
}

func TestBundleAssembler_Assemble_DynamicTagSensitiveDigestsAndDormantLenses(t *testing.T) {
	ctx := context.Background()

	pinnedDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Pinned architecture invariant",
		Tags:     []string{"core-invariant"},
		IsSticky: true,
	}
	time.Sleep(2 * time.Millisecond)
	authDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Bearer auth middleware implemented",
		Tags:     []string{"auth", "middleware"},
		IsSticky: false,
	}
	time.Sleep(2 * time.Millisecond)
	mathDigest := thinkgraph.DigestMeta{
		ID:       uuid.NewV7(),
		Summary:  "Internal math package handles arithmetic",
		Tags:     []string{"internal/math", "routing"},
		IsSticky: false,
	}

	mockNodes := []thinkgraph.Node{
		{Event: thinkgraph.Event{
			ID:        uuid.NewV7(),
			Type:      thinkgraph.EventPrompt,
			Content:   "Refactor internal/math endpoints",
			Tags:      []string{"internal/math"},
			Timestamp: time.Now(),
		}},
	}

	repo := &baMockGraphRepo{
		nodes:   mockNodes,
		digests: []thinkgraph.DigestMeta{pinnedDigest, authDigest, mathDigest},
	}

	assembler := NewBundleAssembler(nil, repo, thinkgraph.NewContextProjecter(), config.NewRegistry(), "/tmp")
	chatObj := &thinkgraph.ChatAddress{ChatID: "chat-1", ProjectID: "space-1"}

	bundle, graphData, err := assembler.Assemble(ctx, chatObj, uuid.Nil(), uuid.Nil(), "golang", &baMockThinkSpace{})
	if err != nil {
		t.Fatalf("Assemble failed: %v", err)
	}

	// Active digests must include sticky pinnedDigest + focus-matched mathDigest, while excluding out-of-focus authDigest
	if len(graphData.Digests) != 2 {
		t.Fatalf("expected 2 active digests (pinned + math), got %d: %+v", len(graphData.Digests), graphData.Digests)
	}
	if graphData.Digests[0].ID != pinnedDigest.ID || graphData.Digests[1].ID != mathDigest.ID {
		t.Errorf("unexpected active digests selected: %+v", graphData.Digests)
	}

	// Unselected authDigest tags must appear in DormantLenses
	if !slices.Contains(graphData.Focus.DormantLenses, "auth") || !slices.Contains(graphData.Focus.DormantLenses, "middleware") {
		t.Errorf("expected unselected digest tags [auth, middleware] in DormantLenses, got %v", graphData.Focus.DormantLenses)
	}

	// LongTermMemory must include metadata + pinnedDigest + mathDigest
	joinedLTM := strings.Join(bundle.Trajectory.LongTermMemory, "\n")
	if !strings.Contains(joinedLTM, "Pinned architecture invariant") {
		t.Errorf("expected sticky digest in LongTermMemory")
	}
	if !strings.Contains(joinedLTM, "Internal math package handles arithmetic") {
		t.Errorf("expected focus-matched non-sticky digest in LongTermMemory")
	}
	if strings.Contains(joinedLTM, "Bearer auth middleware implemented") {
		t.Errorf("expected out-of-focus non-sticky digest to be excluded from LongTermMemory")
	}
}

func TestBundleAssembler_SelectActiveDigests(t *testing.T) {
	primaryTags := map[string]bool{"backend": true, "database": true}
	relatedTags := map[string]bool{"ui": true}

	globalDigest := thinkgraph.DigestMeta{ID: uuid.NewV7(), Tags: []string{}}
	backendDigest := thinkgraph.DigestMeta{ID: uuid.NewV7(), Tags: []string{"backend", "performance"}}
	uiDigest := thinkgraph.DigestMeta{ID: uuid.NewV7(), Tags: []string{"ui"}}
	unrelatedDigest := thinkgraph.DigestMeta{ID: uuid.NewV7(), Tags: []string{"marketing"}}
	stickyUnrelated := thinkgraph.DigestMeta{ID: uuid.NewV7(), Tags: []string{"policy"}, IsSticky: true}

	allDigests := []thinkgraph.DigestMeta{globalDigest, backendDigest, uiDigest, unrelatedDigest, stickyUnrelated}

	active, dormant := selectActiveDigests(allDigests, primaryTags, relatedTags)

	if len(active) != 4 {
		t.Fatalf("Expected exactly 4 active digests (global + backend + ui + sticky), got %d", len(active))
	}

	foundGlobal := false
	foundUnrelated := false
	foundSticky := false
	for _, d := range active {
		if d.ID == globalDigest.ID {
			foundGlobal = true
		}
		if d.ID == unrelatedDigest.ID {
			foundUnrelated = true
		}
		if d.ID == stickyUnrelated.ID {
			foundSticky = true
		}
	}

	if !foundGlobal {
		t.Errorf("Global digest (no tags) should always be active")
	}
	if !foundSticky {
		t.Errorf("Sticky digest should always be active regardless of tags")
	}
	if foundUnrelated {
		t.Errorf("Unrelated non-sticky digest should have been filtered out")
	}
	if !slices.Contains(dormant, "marketing") {
		t.Errorf("Expected 'marketing' to appear in dormant lenses, got %v", dormant)
	}
}

func TestBundleAssembler_FilterActiveContexts(t *testing.T) {
	primaryTags := map[string]bool{"api": true}
	relatedTags := map[string]bool{}

	globalContext := thinkgraph.SpaceContext{ID: uuid.NewV7(), Tags: []string{}}
	apiContext := thinkgraph.SpaceContext{ID: uuid.NewV7(), Tags: []string{"api"}}
	dbContext := thinkgraph.SpaceContext{ID: uuid.NewV7(), Tags: []string{"database"}}

	allContexts := []thinkgraph.SpaceContext{globalContext, apiContext, dbContext}

	active := filterActiveContexts(allContexts, primaryTags, relatedTags)

	if len(active) != 2 {
		t.Fatalf("Expected exactly 2 active contexts, got %d", len(active))
	}
}

func TestBundleAssembler_FormatPersona(t *testing.T) {
	t.Run("Executes Template Successfully", func(t *testing.T) {
		rawPrompt := "Welcome. Here are your routing options:\n{{.AvailableThinkStates}}"
		available := "- golang: Go dev\n- concept: Ideation"
		activeState := "intake"

		result := formatPersona(nil, rawPrompt, available, activeState)

		if !strings.Contains(result, available) {
			t.Errorf("Expected injected template variables to be present in prompt. Got:\n%s", result)
		}
		if !strings.Contains(result, "operating in the 'intake' think state") {
			t.Errorf("Expected global rule to append active state.")
		}
	})

	t.Run("Handles Fallback for Missing Summary", func(t *testing.T) {
		rawPrompt := "{{.AvailableThinkStates}}"

		result := formatPersona(nil, rawPrompt, "", "intake")

		if !strings.Contains(result, "No specialized think states configured") {
			t.Errorf("Expected fallback string for empty summary, got:\n%s", result)
		}
	})

	t.Run("Fails Gracefully on Invalid Template", func(t *testing.T) {
		rawPrompt := "Invalid {{.Missing closing bracket"

		result := formatPersona(nil, rawPrompt, "summary", "golang")

		if !strings.Contains(result, "Invalid {{.Missing") {
			t.Errorf("Expected raw prompt to survive template parse failure, got:\n%s", result)
		}
		if !strings.Contains(result, "operating in the 'golang' think state") {
			t.Errorf("Expected global rules to still append safely.")
		}
	})
}
