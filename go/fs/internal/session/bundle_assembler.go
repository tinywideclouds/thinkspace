package session

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/config"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const maxFallbackRecentDigests = 3

// BundleAssembler handles the compilation of the active trajectory, knowledge graph digests,
// and physical mapbook into a single ThinkBundle for the LLM.
type BundleAssembler struct {
	logger        *slog.Logger
	graphRepo     thinkgraph.GraphRepository
	projecter     *thinkgraph.ContextProjecter
	registry      *config.Registry
	workspaceRoot string
}

func NewBundleAssembler(
	logger *slog.Logger,
	graphRepo thinkgraph.GraphRepository,
	projecter *thinkgraph.ContextProjecter,
	registry *config.Registry,
	workspaceRoot string,
) *BundleAssembler {
	if projecter == nil {
		projecter = thinkgraph.NewContextProjecter()
	}
	return &BundleAssembler{
		logger:        logger,
		graphRepo:     graphRepo,
		projecter:     projecter,
		registry:      registry,
		workspaceRoot: workspaceRoot,
	}
}

// Assemble executes the graph traversal, tag-sensitive digest selection, and persona template injection.
func (a *BundleAssembler) Assemble(
	ctx context.Context,
	chatObj *thinkgraph.ChatAddress,
	digestionHorizon uuid.UUID,
	catalystID uuid.UUID,
	activeThinkState string,
	thinkState spaces.ThinkState,
) (cognition.ThinkBundle, *thinkgraph.GraphData, error) {
	if a.graphRepo == nil {
		if a.logger != nil {
			a.logger.ErrorContext(ctx, "cannot assemble bundle: graph repository is nil")
		}
		return cognition.ThinkBundle{}, nil, fmt.Errorf("graph repository is nil")
	}

	focusDepth := 3
	if thinkState != nil {
		focusDepth = thinkState.Config().Memory.FocusDepth
	}

	volatileEvents, primaryTags, relatedTags, err := a.gatherTimeline(ctx, chatObj, digestionHorizon, focusDepth)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("gathering timeline: %w", err)
	}

	allDigests, err := a.graphRepo.GetDigests(ctx, chatObj)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("fetching digests: %w", err)
	}

	allSpaceContexts, err := a.graphRepo.GetSpaceContexts(ctx, chatObj)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("fetching space contexts: %w", err)
	}

	activeDigests, dormantLenses := selectActiveDigests(allDigests, primaryTags, relatedTags)
	activeContexts := filterActiveContexts(allSpaceContexts, primaryTags, relatedTags)

	manifest := thinkgraph.NewChatManifest()
	for _, d := range activeDigests {
		manifest.Digests[d.ID] = d
	}
	for _, sc := range activeContexts {
		manifest.SpaceContexts[sc.ID] = sc
	}

	focus := thinkgraph.AttentionFocus{
		PrimaryFocus:   mapKeys(primaryTags),
		RelatedContext: mapKeys(relatedTags),
		DormantLenses:  dormantLenses,
	}

	var catalystNode *thinkgraph.Node
	if catalystID != uuid.Nil() {
		nodes, getErr := a.graphRepo.GetNodes(ctx, chatObj, []string{catalystID.String()})
		if getErr != nil {
			return cognition.ThinkBundle{}, nil, fmt.Errorf("fetching catalyst node %s: %w", catalystID, getErr)
		}
		if len(nodes) > 0 {
			catalystNode = &nodes[0]
		}
	} else if len(volatileEvents) > 0 {
		last := volatileEvents[len(volatileEvents)-1]
		catalystNode = &last
	}

	// Filter catalyst out of working memory events to prevent duplication
	var filteredTimeline []thinkgraph.Node
	for _, ev := range volatileEvents {
		if catalystNode == nil || ev.ID != catalystNode.ID {
			filteredTimeline = append(filteredTimeline, ev)
		}
	}
	volatileEvents = filteredTimeline

	graphData := &thinkgraph.GraphData{
		Timeline: volatileEvents,
		Digests:  activeDigests,
		Focus:    focus,
	}

	if thinkState == nil {
		return cognition.ThinkBundle{}, graphData, nil
	}

	projectWorkspaceDir := filepath.Join(a.workspaceRoot, chatObj.ProjectID, "workspace")
	stringMaps := make(map[string]string)
	if thinkState.Mapbook() != nil {
		rawMapbook, mbErr := thinkState.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
		if mbErr != nil {
			return cognition.ThinkBundle{}, nil, fmt.Errorf("generating mapbook layers: %w", mbErr)
		}
		for k, v := range rawMapbook {
			stringMaps[string(k)] = v
		}
	}

	trajectoryRequest := thinkgraph.ContextRequest{
		Manifest:     manifest,
		RecentEvents: volatileEvents,
		Catalyst:     catalystNode,
		Focus:        focus,
	}

	trajectory, err := a.projecter.Build(ctx, trajectoryRequest)
	if err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("assembling trajectory: %w", err)
	}

	rawSystemPrompt := thinkState.Config().ManagerSystemPrompt()
	var availableSummary string
	if a.registry != nil {
		availableSummary = a.registry.AvailableThinkStatesSummary()
	}

	finalSystemPrompt := formatPersona(a.logger, rawSystemPrompt, availableSummary, activeThinkState)

	bundle := cognition.ThinkBundle{
		Rules:      cognition.SystemRules{Persona: finalSystemPrompt},
		Mapbook:    cognition.Mapbook{Maps: stringMaps},
		Trajectory: trajectory,
		Workbench:  []cognition.WorkbenchItem{},
		ActiveTags: focus.PrimaryFocus,
	}

	if err := thinkState.EnrichBundle(ctx, &bundle, focus, a.workspaceRoot); err != nil {
		return cognition.ThinkBundle{}, nil, fmt.Errorf("enriching bundle: %w", err)
	}

	return bundle, graphData, nil
}

// AssembleWorkerBundle builds an isolated ThinkBundle for a sub-agent execution,
// including workspace Mapbook layers, patcher formatting rules, and materialized workbench files.
func (a *BundleAssembler) AssembleWorkerBundle(
	ctx context.Context,
	projectID string,
	space spaces.ThinkState,
	baseAgentRules string,
	contextDigest string,
	instruction string,
	workbenchItems []cognition.WorkbenchItem,
) cognition.ThinkBundle {
	projectWorkspaceDir := filepath.Join(a.workspaceRoot, projectID, "workspace")
	stringMaps := make(map[string]string)

	if space != nil && space.Mapbook() != nil {
		rawMapbook, err := space.Mapbook().GenerateLayers(ctx, projectWorkspaceDir)
		if err != nil {
			if a.logger != nil {
				a.logger.WarnContext(ctx, "failed to generate mapbook layers", "error", err)
			}
		} else {
			for k, v := range rawMapbook {
				stringMaps[string(k)] = v
			}
		}
	}

	formattingConstraints := ""
	if space != nil && space.Patcher() != nil {
		formattingConstraints = space.Patcher().SystemInstructions()
	}

	var longTermMemory []string
	if contextDigest != "" {
		longTermMemory = []string{contextDigest}
	}

	return cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona:               baseAgentRules,
			FormattingConstraints: formattingConstraints,
		},
		Mapbook: cognition.Mapbook{Maps: stringMaps},
		Trajectory: cognition.Trajectory{
			LongTermMemory: longTermMemory,
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: instruction,
			},
		},
		Workbench: workbenchItems,
	}
}

func (a *BundleAssembler) gatherTimeline(
	ctx context.Context,
	chatObj *thinkgraph.ChatAddress,
	digestionHorizon uuid.UUID,
	focusDepth int,
) ([]thinkgraph.Node, map[string]bool, map[string]bool, error) {
	traversalEngine := thinkgraph.NewTraversalEngine(a.graphRepo)
	var volatileEvents []thinkgraph.Node
	primaryTags := make(map[string]bool)
	relatedTags := make(map[string]bool)

	err := traversalEngine.WalkTimelineReverse(ctx, chatObj, func(node thinkgraph.Node) bool {
		if node.ID == digestionHorizon {
			return false
		}

		volatileEvents = append(volatileEvents, node)

		if len(volatileEvents) <= focusDepth {
			for _, tag := range node.Tags {
				if norm := strings.ToLower(strings.TrimSpace(tag)); norm != "" {
					primaryTags[norm] = true
				}
			}
		} else {
			for _, tag := range node.Tags {
				if norm := strings.ToLower(strings.TrimSpace(tag)); norm != "" {
					relatedTags[norm] = true
				}
			}
		}

		if len(volatileEvents) >= 50 {
			return false
		}
		return true
	})

	if err != nil {
		return nil, nil, nil, err
	}

	// Reverse to chronological order
	for i, j := 0, len(volatileEvents)-1; i < j; i, j = i+1, j-1 {
		volatileEvents[i], volatileEvents[j] = volatileEvents[j], volatileEvents[i]
	}

	return volatileEvents, primaryTags, relatedTags, nil
}

// selectActiveDigests partitions all chat digests into active digests for the current turn
// and dormant tags for query_lens discovery:
// - Sticky digests (Promoted: true) are always included regardless of active tags.
// - Global digests (len(Tags) == 0) are always included.
// - Non-sticky tagged digests are included if any tag matches primaryTags or relatedTags.
// - If no focus tags are active yet, up to maxFallbackRecentDigests most recent digests are included.
// - Tags belonging solely to unselected digests are returned as sorted dormant lenses.
func selectActiveDigests(allDigests []thinkgraph.DigestMeta, primaryTags, relatedTags map[string]bool) ([]thinkgraph.DigestMeta, []string) {
	if len(allDigests) == 0 {
		return nil, nil
	}

	sorted := append([]thinkgraph.DigestMeta(nil), allDigests...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].ID.String() < sorted[j].ID.String()
	})

	hasFocus := len(primaryTags) > 0 || len(relatedTags) > 0

	// When no focus tags exist yet on the timeline, allow the most recent non-sticky digests as a baseline window.
	recentFallbackIDs := make(map[uuid.UUID]bool)
	if !hasFocus {
		nonStickyCount := 0
		for i := len(sorted) - 1; i >= 0 && nonStickyCount < maxFallbackRecentDigests; i-- {
			if !sorted[i].Promoted && len(sorted[i].Tags) > 0 {
				recentFallbackIDs[sorted[i].ID] = true
				nonStickyCount++
			}
		}
	}

	var active []thinkgraph.DigestMeta
	activeTagSet := make(map[string]bool)
	for t := range primaryTags {
		activeTagSet[t] = true
	}
	for t := range relatedTags {
		activeTagSet[t] = true
	}

	var dormantCandidates []thinkgraph.DigestMeta

	for _, d := range sorted {
		if d.Promoted || len(d.Tags) == 0 {
			active = append(active, d)
			for _, tag := range d.Tags {
				if norm := strings.ToLower(strings.TrimSpace(tag)); norm != "" {
					activeTagSet[norm] = true
				}
			}
			continue
		}

		isActive := false
		if hasFocus {
			for _, tag := range d.Tags {
				norm := strings.ToLower(strings.TrimSpace(tag))
				if primaryTags[norm] || relatedTags[norm] {
					isActive = true
					break
				}
			}
		} else {
			isActive = recentFallbackIDs[d.ID]
		}

		if isActive {
			active = append(active, d)
			for _, tag := range d.Tags {
				if norm := strings.ToLower(strings.TrimSpace(tag)); norm != "" {
					activeTagSet[norm] = true
				}
			}
		} else {
			dormantCandidates = append(dormantCandidates, d)
		}
	}

	dormantMap := make(map[string]bool)
	for _, d := range dormantCandidates {
		for _, tag := range d.Tags {
			norm := strings.ToLower(strings.TrimSpace(tag))
			if norm != "" && !activeTagSet[norm] {
				dormantMap[norm] = true
			}
		}
	}

	return active, mapKeys(dormantMap)
}

func filterActiveContexts(allContexts []thinkgraph.SpaceContext, primaryTags, relatedTags map[string]bool) []thinkgraph.SpaceContext {
	var active []thinkgraph.SpaceContext
	for _, sc := range allContexts {
		if len(sc.Tags) == 0 {
			active = append(active, sc)
			continue
		}
		isActive := false
		for _, tag := range sc.Tags {
			norm := strings.ToLower(strings.TrimSpace(tag))
			if primaryTags[norm] || relatedTags[norm] {
				isActive = true
				break
			}
		}
		if isActive {
			active = append(active, sc)
		}
	}
	return active
}

func formatPersona(logger *slog.Logger, rawPrompt string, availableSummary string, activeThinkState string) string {
	tmpl, err := template.New("persona").Parse(rawPrompt)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to parse persona as template; falling back to raw string", "error", err)
		}
	} else {
		if availableSummary == "" {
			availableSummary = "No specialized think states configured."
		}

		var buf bytes.Buffer
		err = tmpl.Execute(&buf, map[string]any{
			"AvailableThinkStates": availableSummary,
		})
		if err != nil {
			if logger != nil {
				logger.Warn("failed to execute persona template; falling back to raw string", "error", err)
			}
		} else {
			rawPrompt = buf.String()
		}
	}

	rawPrompt += fmt.Sprintf("\n\n[GLOBAL SYSTEM RULE]: You are currently operating in the '%s' think state. You may use the `transition_workspace` tool to switch to a different think state if the user's task requires different tools or a different directory structure.", activeThinkState)
	rawPrompt += "\n[TAXONOMY RULE]: Use precise semantic tagging. Apply existing tags from the active context to maintain a cohesive knowledge graph over time."

	return rawPrompt
}

func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
