package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"text/template"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type DigestConfig struct {
	ManualPrompt     string `yaml:"manual_prompt"`
	BackgroundPrompt string `yaml:"background_prompt"`
}

// DigestManager handles the compression of historical chat events into semantic summaries.
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

// CreateManualDigest handles explicit digest requests triggered by the LLM Manager.
func (m *DigestManager) CreateManualDigest(
	ctx context.Context,
	svc *workspace.Service,
	thread *chat.Thread,
	topicName string,
	tags []string,
	instruction string,
	contextData string,
	model string,
	basePersona string,
) error {
	m.logger.InfoContext(ctx, "Creating manual context digest", "topic", topicName)

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

	stream := m.client.GenerateContentStream(ctx, model, bundle, nil)
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

	return svc.LogDigest(ctx, thread, summary, tags, true)
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

// CreateBackgroundDigest handles the automated sliding window compression.
func (m *DigestManager) CreateBackgroundDigest(
	ctx context.Context,
	svc *workspace.Service,
	thread *chat.Thread,
	targetWindow []chat.Event,
	activeBuffer []chat.Event,
	force bool,
	model string,
	basePersona string,
) (int, error) {
	if len(targetWindow) == 0 {
		return 0, nil
	}

	m.logger.InfoContext(ctx, "Creating background context digest", "events", len(targetWindow), "forced", force)

	// Format the window we actually want to summarize
	var targetSb strings.Builder
	for _, ev := range targetWindow {
		targetSb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Type, ev.Content))
	}

	// Format the recent buffer so the LLM knows what we already remember
	var bufferSb strings.Builder
	for _, ev := range activeBuffer {
		bufferSb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Type, ev.Content))
	}

	tmpl, err := template.New("background").Parse(m.config.BackgroundPrompt)
	if err != nil {
		return 0, fmt.Errorf("failed to parse background digest prompt: %w", err)
	}

	var promptBuf bytes.Buffer
	if err := tmpl.Execute(&promptBuf, map[string]string{
		"TargetWindow": targetSb.String(),
		"ActiveBuffer": bufferSb.String(),
	}); err != nil {
		return 0, fmt.Errorf("failed to execute background digest prompt: %w", err)
	}

	schemaInstruction := "\n\n[FORMATTING]: You MUST reply with ONLY a JSON object matching this schema: {\"thought\": \"...\", \"digests\": [{\"topic_name\": \"...\", \"tags\": [\"...\"], \"summary\": \"...\"}]}"

	bundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: basePersona + schemaInstruction,
		},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: promptBuf.String(),
			},
		},
	}

	stream := m.client.GenerateContentStream(ctx, model, bundle, nil)
	var jsonBuilder strings.Builder

	for chunk, err := range stream {
		if err != nil {
			return 0, fmt.Errorf("background digest generation failed: %w", err)
		}
		jsonBuilder.WriteString(chunk.Text)
	}

	rawResponse := strings.TrimSpace(jsonBuilder.String())

	// Apply Model-Specific Output Healers to extract JSON safely
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
		if d.Summary == "" {
			continue
		}
		// Utilizing the tags provided by the LLM
		if err := svc.LogDigest(ctx, thread, d.Summary, d.Tags, false); err != nil {
			return count, fmt.Errorf("failed to log background digest: %w", err)
		}
		count++
	}

	return count, nil
}
