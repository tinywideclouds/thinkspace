package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"google.golang.org/genai"
)

// ContextAssembler is the master prompt compiler that fuses Spatial Reality (Mapbook)
// with Temporal Memory (Chat Assembler).
type ContextAssembler struct {
	chatAssembler *chat.Assembler
}

func NewContextAssembler() *ContextAssembler {
	return &ContextAssembler{
		chatAssembler: chat.NewAssembler(),
	}
}

// Build fuses the workspace physical map and the chat history into a final system prompt and message array.
func (ca *ContextAssembler) Build(
	ctx context.Context,
	thinkSpace spaces.ThinkSpace,
	workspaceRoot string,
	chatReq chat.AssemblyRequest,
) (string, []*genai.Content, error) {

	// 1. Build Temporal Memory (History)
	history, err := ca.chatAssembler.Build(chatReq)
	if err != nil {
		return "", nil, fmt.Errorf("assembling temporal memory: %w", err)
	}

	// 2. Build Spatial Reality (Mapbook)
	var promptBuilder strings.Builder
	promptBuilder.WriteString("### THE MAPBOOK (WORKSPACE CONTEXT)\n")

	if layers, err := thinkSpace.Mapbook().GenerateLayers(ctx, workspaceRoot); err == nil {
		for layerName, content := range layers {
			promptBuilder.WriteString(fmt.Sprintf("--- %s Map ---\n%s\n\n", layerName, content))
		}
	}

	// 3. Fuse into dynamic System Prompt
	dynamicSystemPrompt := thinkSpace.Config().ManagerSystemPrompt() + "\n\n" + promptBuilder.String()

	return dynamicSystemPrompt, history, nil
}

// dynamicThinkSpaceWrapper intercepts the ManagerSystemPrompt to inject Mapbook contexts at runtime.
type dynamicThinkSpaceWrapper struct {
	spaces.ThinkSpace
	systemPrompt string
}

func (d *dynamicThinkSpaceWrapper) Config() spaces.ThinkSpaceConfig {
	cfg := d.ThinkSpace.Config()
	cfg.Roles.Manager.SystemPrompt = d.systemPrompt
	return cfg
}

// WrapSpace injects the dynamic system prompt into a ThinkSpace wrapper for the Coordinator.
func WrapSpace(base spaces.ThinkSpace, dynamicPrompt string) spaces.ThinkSpace {
	return &dynamicThinkSpaceWrapper{
		ThinkSpace:   base,
		systemPrompt: dynamicPrompt,
	}
}
