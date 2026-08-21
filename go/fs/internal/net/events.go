package net

import "encoding/json"
import "github.com/tinywideclouds.com/thinkspace/internal/session"

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
	// Tells the UI to render the routing choice buttons
	Active bool `json:"active"`
}

type RequestReviewPayload struct {
	Branch string `json:"branch"`
}

// --- Inbound Payload Structs ---

type SubmitPromptPayload struct {
	Text string `json:"text"`
}

type SelectStrategyPayload struct {
	StrategyID session.DelegationStrategy `json:"strategy_id"`
}

type ReviewDecisionPayload struct {
	Branch   string `json:"branch"`
	Accepted bool   `json:"accepted"`
}
