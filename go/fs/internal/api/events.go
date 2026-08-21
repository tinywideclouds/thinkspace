package api

import (
	"encoding/json"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
)

// EventType defines the strict string constants for our JSON protocol routing
type EventType string

const (
	// Server-to-Client (Outbound)
	EventTypeChatStream         EventType = "chat_stream"
	EventTypeLogMessage         EventType = "log_message"
	EventTypeDelegationStart    EventType = "delegation_start"
	EventTypeDelegationComplete EventType = "delegation_complete"
	EventTypeAgentStart         EventType = "agent_start"
	EventTypeAgentStream        EventType = "agent_stream"
	EventTypeAgentComplete      EventType = "agent_complete"
	EventTypeRequestStrategy    EventType = "request_strategy"
	EventTypeRequestReview      EventType = "request_review"
	EventTypeAvailableSpaces    EventType = "available_spaces" // NEW: Handshake event

	// Client-to-Server (Inbound)
	EventTypeSubmitPrompt   EventType = "submit_prompt"
	EventTypeSelectStrategy EventType = "select_strategy"
	EventTypeReviewDecision EventType = "review_decision"
)

// WSEvent is the standard envelope for all WebSocket messages
type WSEvent struct {
	Type    EventType       `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// --- Outbound Payload Structs ---

type ChatStreamPayload struct {
	Text string `json:"text"`
}

type LogMessagePayload struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type DelegationStartPayload struct {
	AgentCount   int    `json:"agent_count"`
	Instructions string `json:"instructions"`
}

type DelegationCompletePayload struct {
	Summary string `json:"summary"`
}

type AgentStartPayload struct {
	AgentID      int    `json:"agent_id"`
	Instructions string `json:"instructions"`
}

type AgentStreamPayload struct {
	AgentID int    `json:"agent_id"`
	Text    string `json:"text"`
}

type AgentCompletePayload struct {
	AgentID  int    `json:"agent_id"`
	Branch   string `json:"branch"`
	Verified bool   `json:"verified"`
}

type RequestStrategyPayload struct {
	Active bool `json:"active"`
}

type RequestReviewPayload struct {
	Branch string `json:"branch"`
}

// NEW: Tells the UI what domain plugins are loaded
type SpaceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"` // Assuming ThinkSpaceConfig will eventually have a 'Name' or we derive it
}

type AvailableSpacesPayload struct {
	Spaces []SpaceInfo `json:"spaces"`
}

// --- Inbound Payload Structs ---

type SubmitPromptPayload struct {
	Text    string `json:"text"`
	SpaceID string `json:"space_id"` // NEW: The UI tells the server which domain to use
}

type SelectStrategyPayload struct {
	StrategyID session.DelegationStrategy `json:"strategy_id"`
}

type ReviewDecisionPayload struct {
	Branch   string `json:"branch"`
	Accepted bool   `json:"accepted"`
}
