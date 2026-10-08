package session

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type ManagerPayload struct {
	Branch   string
	Strategy string
}

func (p ManagerPayload) ToMap() map[string]any {
	m := make(map[string]any)
	if p.Branch != "" {
		m["branch"] = p.Branch
	}
	if p.Strategy != "" {
		m["strategy"] = p.Strategy
	}
	return m
}

type TurnContext struct {
	SpaceID             string
	ChatID              string
	PreviewID           string
	ManagerSystemPrompt string
	StickyDigests       []string
	RecentHistory       []string
	FileContexts        []FileContext
}

type FileContext struct {
	Path       string
	Level      string
	TokenCount int
}

type UserInterface interface {
	OnTextChunk(text string)
	ReviewContext(ctxData TurnContext) bool
	ChooseNextStep() flows.FlowStrategy
	ReviewCandidate(branch string) (accepted bool)
	ConfirmTransition(targetDomain string) bool
}

type TransitionRequest struct {
	TargetDomain string
}

func (e *TransitionRequest) Error() string {
	return "domain transition requested to: " + e.TargetDomain
}

type Coordinator struct {
	logger           *slog.Logger
	workspaceService *workspace.Service
	modelClient      llm.ModelClient
	emitter          telemetry.Emitter
	flowEngine       *FlowEngine
	fanOutFlow       flows.Flow
	digestManager    *DigestManager
	spaceID          string
	workerPrompt     string
}

func NewCoordinator(
	logger *slog.Logger,
	workspaceService *workspace.Service,
	modelClient llm.ModelClient,
	emitter telemetry.Emitter,
	flowEngine *FlowEngine,
	fanOutFlow flows.Flow,
	digestManager *DigestManager,
	spaceID string,
	workerPrompt string,
) *Coordinator {
	return &Coordinator{
		logger:           logger,
		workspaceService: workspaceService,
		modelClient:      modelClient,
		emitter:          emitter,
		flowEngine:       flowEngine,
		fanOutFlow:       fanOutFlow,
		digestManager:    digestManager,
		spaceID:          spaceID,
		workerPrompt:     workerPrompt,
	}
}

func (c *Coordinator) RunAgentLoop(
	ctx context.Context,
	thread *chat.Thread,
	manifest *chat.ThreadManifest,
	thinkSpace spaces.ThinkSpace,
	bundle cognition.ThinkBundle,
	userInterface UserInterface,
	availableTools []llm.ToolDef,
) error {
	turnContext, cancel := context.WithTimeout(ctx, thinkSpace.Config().TurnTimeout())
	defer cancel()

	managerModel := thinkSpace.Config().Roles.Manager.Model
	maxIterations := 3

	for iteration := 0; iteration < maxIterations; iteration++ {
		c.emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "log",
			Timestamp: time.Now(),
			Message:   "Manager is analyzing context and formulating a plan...",
		})

		stream := c.modelClient.GenerateContentStream(
			turnContext,
			managerModel,
			bundle,
			availableTools,
		)

		var fullModelResponse strings.Builder
		var interceptedTools []llm.ToolCall

		for chunk, err := range stream {
			if err != nil {
				return fmt.Errorf("generation stream failed: %w", err)
			}

			if chunk.Text != "" {
				userInterface.OnTextChunk(chunk.Text)
				fullModelResponse.WriteString(chunk.Text)
			}

			if len(chunk.ToolCalls) > 0 {
				interceptedTools = append(interceptedTools, chunk.ToolCalls...)
			}
		}

		if fullModelResponse.Len() > 0 {
			_ = c.workspaceService.LogModelResponse(turnContext, thread, fullModelResponse.String())
			bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "model",
				Content: fullModelResponse.String(),
			})
		}

		if len(interceptedTools) == 0 {
			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   "Manager has concluded its response.",
			})
			break
		}

		call := interceptedTools[0]

		if call.Name == "transition_workspace" {
			targetDomain, _ := call.Args["target_domain"].(string)

			c.logger.InfoContext(turnContext, "intercepted transition request", "target", targetDomain)
			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Manager requested workspace transition to '%s'", targetDomain),
			})

			if userInterface.ConfirmTransition(targetDomain) {
				c.emitter.Emit("manager", telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "log",
					Timestamp: time.Now(),
					Message:   "Transition approved. Rebooting workspace...",
				})
				return &TransitionRequest{TargetDomain: targetDomain}
			}

			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   "Transition denied by user. Forcing manager to self-correct...",
			})
			userInterface.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** Transition to '%s' denied.\nAsking Manager to continue in current domain...\n", targetDomain))

			bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "user",
				Content: fmt.Sprintf("[System: Transition to '%s' was denied by the user. You must continue your task within the current domain.]", targetDomain),
			})
			continue
		}

		if call.Name == "query_lens" {
			tag, _ := call.Args["tag"].(string)
			c.logger.InfoContext(turnContext, "executing query_lens", "tag", tag)
			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   fmt.Sprintf("Manager invoked query_lens for tag: %s", tag),
			})
			userInterface.OnTextChunk(fmt.Sprintf("\n\n🔍 **Searching memory for #%s...**\n", tag))

			var toolResponse strings.Builder
			toolResponse.WriteString(fmt.Sprintf("[System Tool Response: query_lens(%s)]\n", tag))

			var idsToFetch []string
			if mappedUUIDs, exists := manifest.Lenses[tag]; exists {
				for _, u := range mappedUUIDs {
					idsToFetch = append(idsToFetch, u.String())
				}
			}

			if len(idsToFetch) == 0 {
				toolResponse.WriteString("No historical events found for this tag.\n")
			} else {
				events, err := c.workspaceService.FetchEvents(turnContext, thread, idsToFetch)
				if err != nil {
					toolResponse.WriteString(fmt.Sprintf("Error fetching events: %v\n", err))
				} else {
					for _, ev := range events {
						toolResponse.WriteString(fmt.Sprintf("- [%s] %s: %s\n", ev.Timestamp.Format("15:04:05"), ev.Type, ev.Content))
					}
				}
			}

			bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
				Role:    "user",
				Content: toolResponse.String(),
			})
			continue
		}

		if call.Name == "create_digest" {
			instruction, _ := call.Args["instruction"].(string)
			topicName, _ := call.Args["topic_name"].(string)

			var tags []string
			if rawTags, ok := call.Args["tags"].([]any); ok {
				for _, rt := range rawTags {
					if tStr, ok := rt.(string); ok {
						tags = append(tags, tStr)
					}
				}
			}

			var sb strings.Builder
			for _, ev := range bundle.Trajectory.WorkingMemory {
				sb.WriteString(fmt.Sprintf("[%s]: %s\n", ev.Role, ev.Content))
			}

			err := c.digestManager.CreateManualDigest(
				turnContext,
				c.workspaceService,
				thread,
				topicName,
				tags,
				instruction,
				sb.String(),
				thinkSpace.Config().Roles.Worker.Model,
				c.workerPrompt,
			)

			if err != nil {
				c.logger.WarnContext(turnContext, "create_digest failed", "error", err)
				c.emitter.Emit("manager", telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "error",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Digest creation failed: %v", err),
				})
			}
			break
		}

		if call.Name == "propose_change" {
			c.emitter.Emit("manager", telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "log",
				Timestamp: time.Now(),
				Message:   "Manager is proposing changes and delegating to sub-agents...",
			})

			flowContext := flows.FlowContext{
				FlowID:         fmt.Sprintf("flow-%d", time.Now().Unix()),
				SpaceID:        c.spaceID,
				BaseAgentRules: c.workerPrompt,
				FlowSummary:    "Executing requested changes.",
			}

			err := c.flowEngine.Run(turnContext, thread, thinkSpace, c.fanOutFlow, call.Args, flowContext, userInterface)

			if err != nil {
				c.logger.WarnContext(turnContext, "tool execution failed, bouncing back to manager", "error", err)
				c.emitter.Emit("manager", telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "error",
					Timestamp: time.Now(),
					Message:   fmt.Sprintf("Delegation failed: %v", err),
				})
				userInterface.OnTextChunk(fmt.Sprintf("\n\n⚠️ **System Intercept:** %v\nAsking Manager to self-correct...\n", err))

				bundle.Trajectory.WorkingMemory = append(bundle.Trajectory.WorkingMemory, cognition.TrajectoryEvent{
					Role:    "user",
					Content: fmt.Sprintf("[System Tool Error: propose_change failed with: %v. Please review your requested target_files and parameters, and retry.]", err),
				})
				continue
			}
			break
		}
	}

	_, err := c.workspaceService.Checkpoint(turnContext, thread, "End of session turn")
	return err
}
