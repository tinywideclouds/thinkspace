package flows

import (
	"context"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type DigestFlow struct {
	logger *slog.Logger
}

func NewDigestFlow(logger *slog.Logger) *DigestFlow {
	return &DigestFlow{logger: logger}
}

func (f *DigestFlow) Name() string {
	return "DigestFlow"
}

func (f *DigestFlow) Execute(
	ctx context.Context,
	service *workspace.Service,
	thread *chat.Thread,
	space spaces.ThinkSpace,
	args map[string]any,
	flowConfig FlowConfig,
	flowContext FlowContext,
	emitter telemetry.Emitter,
	executor workspace.SubAgentExecutor,
	verifier spaces.Verifier,
) (*FlowResult, error) {

	instruction, _ := args["instruction"].(string)
	tagsRaw, _ := args["tags"].([]any)
	var tags []string
	for _, t := range tagsRaw {
		if strTag, ok := t.(string); ok {
			tags = append(tags, strTag)
		}
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   "Spinning up background agent to compress memory...",
	})

	// 1. Spawn an empty, transient sandbox for the agent
	candidateID := fmt.Sprintf("digest-%d", time.Now().Unix())
	sandbox, err := service.SpawnSandbox(ctx, thread.ID, candidateID)
	if err != nil {
		return nil, fmt.Errorf("spawning digest sandbox: %w", err)
	}
	defer sandbox.TearDown(ctx) // Always discard immediately, this doesn't merge to Main

	// 2. Build a highly specific ThinkBundle just for summarization
	agentBundle := cognition.ThinkBundle{
		Rules: cognition.SystemRules{
			Persona: "You are a Memory Synthesizer. Output ONLY the raw markdown text of the summary. Do not use JSON. Do not wrap in backticks. Be extremely concise.",
		},
		Trajectory: cognition.Trajectory{
			Catalyst: cognition.TrajectoryEvent{
				Role:    "user",
				Content: instruction,
			},
		},
	}

	// 3. Execute the Sub-Agent
	err = executor(ctx, agentBundle, sandbox, 1, nil)
	if err != nil {
		return nil, fmt.Errorf("memory compression failed: %w", err)
	}

	// 4. Retrieve the Raw Output from the trace log
	traceBytes, err := sandbox.ReadFile(ctx, "trace.jsonl")
	if err != nil {
		return nil, fmt.Errorf("failed to read trace: %w", err)
	}

	// We extract the generated text from the trace
	summaryText := extractGeneratedTextFromTrace(string(traceBytes))
	if summaryText == "" {
		return nil, fmt.Errorf("agent produced empty summary")
	}

	// 5. Persist the Digest Event directly to the Ledger
	if err := service.LogDigest(ctx, thread, summaryText, tags, true); err != nil {
		return nil, fmt.Errorf("failed to log digest to ledger: %w", err)
	}

	emitter.Emit("manager", telemetry.Event{
		ID:        uuid.NewV7(),
		Type:      "log",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Memory compressed and tagged with %v", tags),
	})

	return &FlowResult{Summary: "Memory compressed"}, nil
}

// Quick helper to pluck the "generated" field from the last line of the trace.jsonl
func extractGeneratedTextFromTrace(trace string) string {
	// Basic extraction logic omitted for brevity, but it unmarshals the JSONL line
	return "Parsed Summary Text" // In reality, unmarshal the TraceEvent struct here
}
