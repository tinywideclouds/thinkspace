package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"text/template"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type DigestConfig struct {
	ManualPrompt     string `yaml:"manual_prompt"`
	BackgroundPrompt string `yaml:"background_prompt"`
}

type DigestManager struct {
	logger *slog.Logger
	client llm.ModelClient
	config DigestConfig
}

func NewDigestManager(logger *slog.Logger, client llm.ModelClient, config DigestConfig) *DigestManager {
	return &DigestManager{
		logger: logger,
		client: client,
		config: config,
	}
}

func normalizeDigestTags(raw []string) []string {
	if len(raw) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	var out []string
	for _, t := range raw {
		norm := strings.ToLower(strings.TrimSpace(t))
		norm = strings.TrimPrefix(norm, "#")
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		out = append(out, norm)
	}
	return out
}

func (m *DigestManager) existingCanonicalTags(ctx context.Context, svc *workspace.Service, chatSession *thinkgraph.ChatAddress) []string {
	if svc == nil || svc.Graph() == nil || chatSession == nil {
		return nil
	}
	digests, err := svc.Graph().GetDigests(ctx, chatSession)
	if err != nil {
		if m.logger != nil {
			m.logger.WarnContext(ctx, "failed fetching existing digests for tag reuse", "error", err)
		}
		return nil
	}
	seen := make(map[string]bool)
	var tags []string
	for _, d := range digests {
		for _, t := range d.Tags {
			norm := strings.ToLower(strings.TrimSpace(t))
			if norm != "" && !seen[norm] {
				seen[norm] = true
				tags = append(tags, norm)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

func (m *DigestManager) CreateManualDigest(
	ctx context.Context,
	svc *workspace.Service,
	chatSession *thinkgraph.ChatAddress,
	topicName string,
	tags []string,
	isSticky bool,
	instruction string,
	contextData string,
	model string,
	basePersona string,
) error {
	if m.client == nil {
		return fmt.Errorf("model client is nil")
	}
	if svc == nil {
		return fmt.Errorf("workspace service is nil")
	}
	if strings.TrimSpace(m.config.ManualPrompt) == "" {
		return fmt.Errorf("digest manual_prompt configuration is empty")
	}

	if m.logger != nil {
		m.logger.InfoContext(ctx, "Creating manual context digest", "topic", topicName, "is_sticky", isSticky)
	}

	tmpl, err := template.New("manual").Parse(m.config.ManualPrompt)
	if err != nil {
		return fmt.Errorf("failed to parse manual digest prompt: %w", err)
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, map[string]string{
		"TopicName":   topicName,
		"Instruction": instruction,
		"Context":     contextData,
	}); err != nil {
		return fmt.Errorf("failed to execute manual digest prompt: %w", err)
	}

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: basePersona,
		},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: promptBuf.String(),
			},
		},
	}

	stream := m.client.GenerateContentStream(ctx, model, bundle, nil, spaces.ThinkStateConfig{})
	var summaryBuilder strings.Builder

	for chunk, err := range stream {
		if err != nil {
			return fmt.Errorf("manual digest generation failed: %w", err)
		}
		summaryBuilder.WriteString(chunk.Text)
	}

	summary := strings.TrimSpace(summaryBuilder.String())
	if summary == "" {
		return fmt.Errorf("manual digest generation returned empty string")
	}

	return svc.AppendDigest(ctx, chatSession, summary, normalizeDigestTags(tags), isSticky, nil)
}

type generatedDigest struct {
	TopicName string   `json:"topic_name"`
	Tags      []string `json:"tags"`
	Summary   string   `json:"summary"`
}

type backgroundDigestResponse struct {
	Thought string            `json:"thought,omitempty"`
	Digests []generatedDigest `json:"digests"`
}

func (m *DigestManager) CreateBackgroundDigest(
	ctx context.Context,
	svc *workspace.Service,
	chatSession *thinkgraph.ChatAddress,
	targetWindow []thinkgraph.Event,
	activeBuffer []thinkgraph.Event,
	force bool,
	model string,
	basePersona string,
) (int, error) {
	if len(targetWindow) == 0 {
		return 0, nil
	}
	if m.client == nil {
		return 0, fmt.Errorf("model client is nil")
	}
	if svc == nil {
		return 0, fmt.Errorf("workspace service is nil")
	}
	if strings.TrimSpace(m.config.BackgroundPrompt) == "" {
		return 0, fmt.Errorf("digest background_prompt configuration is empty")
	}

	if m.logger != nil {
		m.logger.InfoContext(ctx, "Creating background context digest", "events", len(targetWindow), "forced", force)
	}

	var targetSb strings.Builder
	var targetIDs []uuid.UUID
	for _, ev := range targetWindow {
		targetSb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Type, ev.Content))
		targetIDs = append(targetIDs, ev.ID)
	}

	var bufferSb strings.Builder
	for _, ev := range activeBuffer {
		bufferSb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Type, ev.Content))
	}

	existingTags := m.existingCanonicalTags(ctx, svc, chatSession)
	existingTagsStr := "None yet"
	if len(existingTags) > 0 {
		existingTagsStr = strings.Join(existingTags, ", ")
	}

	tmpl, err := template.New("background").Parse(m.config.BackgroundPrompt)
	if err != nil {
		return 0, fmt.Errorf("failed to parse background digest prompt: %w", err)
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, map[string]string{
		"TargetWindow": targetSb.String(),
		"ActiveBuffer": bufferSb.String(),
		"ExistingTags": existingTagsStr,
	}); err != nil {
		return 0, fmt.Errorf("failed to execute background digest prompt: %w", err)
	}

	promptStr := promptBuf.String()
	if len(existingTags) > 0 && !strings.Contains(m.config.BackgroundPrompt, ".ExistingTags") {
		promptStr += fmt.Sprintf("\n\nExisting Canonical Tags (reuse whenever applicable): %s", existingTagsStr)
	}

	schemaInstruction := "\n\n[FORMATTING]: You MUST reply with ONLY a JSON object matching this schema: {\"thought\": \"...\", \"digests\": [{\"topic_name\": \"...\", \"tags\": [\"...\"], \"summary\": \"...\"}]}"

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: basePersona + schemaInstruction,
		},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: promptStr,
			},
		},
	}

	stream := m.client.GenerateContentStream(ctx, model, bundle, nil, spaces.ThinkStateConfig{})
	var jsonBuilder strings.Builder

	for chunk, err := range stream {
		if err != nil {
			return 0, fmt.Errorf("background digest generation failed: %w", err)
		}
		jsonBuilder.WriteString(chunk.Text)
	}

	rawResponse := strings.TrimSpace(jsonBuilder.String())

	healer := llm.GetHealerForModel(model)
	if healer != nil {
		healed, modified := healer.Heal(rawResponse)
		if modified {
			rawResponse = healed
		}
	}

	var response backgroundDigestResponse
	if err := json.Unmarshal([]byte(rawResponse), &response); err != nil {
		return 0, fmt.Errorf("failed to parse background digest JSON: %w (raw: %s)", err, rawResponse)
	}

	count := 0
	for _, d := range response.Digests {
		if strings.TrimSpace(d.Summary) == "" {
			continue
		}
		if err := svc.AppendDigest(ctx, chatSession, d.Summary, normalizeDigestTags(d.Tags), false, targetIDs); err != nil {
			return count, fmt.Errorf("failed to append background digest: %w", err)
		}
		count++
	}

	return count, nil
}
