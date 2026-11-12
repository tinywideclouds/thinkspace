package session

import (
	"fmt"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// ProjectTurnContext projects a domain ThinkBundle and attention focus into a presentation-layer TurnContext.
func ProjectTurnContext(spaceID, chatID string, responseID uuid.UUID, bundle *cognition.ThinkBundle, focus thinkgraph.AttentionFocus) TurnContext {
	var recentHistory []string
	if bundle != nil {
		for _, memory := range bundle.Trajectory.WorkingMemory {
			recentHistory = append(recentHistory, fmt.Sprintf("[%s] %s", memory.Role, memory.Content))
		}
		if bundle.Trajectory.Catalyst.Content != "" {
			recentHistory = append(recentHistory, fmt.Sprintf("[catalyst] %s", bundle.Trajectory.Catalyst.Content))
		}
	}

	var fileContexts []FileContext
	if bundle != nil {
		for _, item := range bundle.Workbench {
			fileContexts = append(fileContexts, FileContext{
				Path:       item.Path,
				Level:      "injected",
				TokenCount: item.TokenCount,
			})
		}
	}

	turnContext := TurnContext{
		SpaceID:       spaceID,
		ChatID:        chatID,
		ResponseID:    responseID,
		RecentHistory: recentHistory,
		FileContexts:  fileContexts,
		Focus:         focus,
	}

	if bundle != nil {
		turnContext.ManagerSystemPrompt = bundle.Rules.Persona
		turnContext.StickyDigests = bundle.Trajectory.LongTermMemory
	}

	return turnContext
}
