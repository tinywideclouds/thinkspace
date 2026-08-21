package api

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type WebSocketUI struct {
	ctx          context.Context
	conn         *websocket.Conn
	writeMu      sync.Mutex
	strategyChan chan session.DelegationStrategy
	reviewChan   chan bool
	tokenChan    chan<- workspace.AgentToken
}

func NewWebSocketUI(ctx context.Context, conn *websocket.Conn, tokenChan chan<- workspace.AgentToken) *WebSocketUI {
	return &WebSocketUI{
		ctx:          ctx,
		conn:         conn,
		strategyChan: make(chan session.DelegationStrategy),
		reviewChan:   make(chan bool),
		tokenChan:    tokenChan,
	}
}

// sendEvent safely marshals and writes a JSON event to the WebSocket.
func (ui *WebSocketUI) sendEvent(eventType EventType, payload any) {
	ui.writeMu.Lock()
	defer ui.writeMu.Unlock()

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return // Silently drop failed marshals to avoid crashing the orchestrator
	}

	event := WSEvent{
		Type:    eventType,
		Payload: rawPayload,
	}

	_ = wsjson.Write(ui.ctx, ui.conn, event)
}

func (ui *WebSocketUI) OnTextChunk(text string) {
	ui.sendEvent(EventTypeChatStream, ChatStreamPayload{Text: text})
}

func (ui *WebSocketUI) OnDelegationStart(count int, instructions string) {
	ui.sendEvent(EventTypeDelegationStart, DelegationStartPayload{
		AgentCount:   count,
		Instructions: instructions,
	})
}

func (ui *WebSocketUI) OnDelegationComplete(summary string) {
	ui.sendEvent(EventTypeDelegationComplete, DelegationCompletePayload{
		Summary: summary,
	})
}

func (ui *WebSocketUI) ChooseNextStep() session.DelegationStrategy {
	ui.sendEvent(EventTypeRequestStrategy, RequestStrategyPayload{Active: true})
	return <-ui.strategyChan
}

func (ui *WebSocketUI) ReviewCandidate(branch string) bool {
	ui.sendEvent(EventTypeRequestReview, RequestReviewPayload{Branch: branch})
	return <-ui.reviewChan
}

func (ui *WebSocketUI) GetAgentTokenChannel() chan<- workspace.AgentToken {
	return ui.tokenChan
}

func (ui *WebSocketUI) PushStrategy(strategy session.DelegationStrategy) {
	ui.strategyChan <- strategy
}

func (ui *WebSocketUI) PushReviewDecision(accepted bool) {
	ui.reviewChan <- accepted
}
