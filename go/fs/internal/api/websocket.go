package api

import (
	"context"
	"sync"

	"github.com/coder/websocket"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type WebSocketUI struct {
	ctx          context.Context
	conn         *websocket.Conn
	facade       *EventFacade
	writeMu      sync.Mutex
	strategyChan chan session.DelegationStrategy
	reviewChan   chan bool
	tokenChan    chan<- workspace.AgentToken
}

func NewWebSocketUI(ctx context.Context, conn *websocket.Conn, tokenChan chan<- workspace.AgentToken) *WebSocketUI {
	return &WebSocketUI{
		ctx:          ctx,
		conn:         conn,
		facade:       NewEventFacade(),
		strategyChan: make(chan session.DelegationStrategy),
		reviewChan:   make(chan bool),
		tokenChan:    tokenChan,
	}
}

// sendBytes safely writes raw JSON bytes to the WebSocket.
func (ui *WebSocketUI) sendBytes(data []byte) {
	ui.writeMu.Lock()
	defer ui.writeMu.Unlock()

	if err := ui.conn.Write(ui.ctx, websocket.MessageText, data); err != nil {
		return
	}
}

func (ui *WebSocketUI) OnTextChunk(text string) {
	if data, err := ui.facade.MarshalChatStream(text); err == nil {
		ui.sendBytes(data)
	}
}

func (ui *WebSocketUI) ChooseNextStep() session.DelegationStrategy {
	if data, err := ui.facade.MarshalRequestStrategy(); err == nil {
		ui.sendBytes(data)
	}
	return <-ui.strategyChan
}

func (ui *WebSocketUI) ReviewCandidate(branch string) bool {
	if data, err := ui.facade.MarshalRequestReview(branch); err == nil {
		ui.sendBytes(data)
	}
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
