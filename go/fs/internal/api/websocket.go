package api

import (
	"context"
	"sync"

	"github.com/coder/websocket"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type WebSocketUI struct {
	ctx          context.Context
	conn         *websocket.Conn
	facade       *EventFacade
	writeMu      sync.Mutex
	strategyChan chan flows.FlowStrategy
	reviewChan   chan bool
	approveChan  chan bool
	tokenChan    chan<- workspace.AgentToken
}

func NewWebSocketUI(ctx context.Context, conn *websocket.Conn, tokenChan chan<- workspace.AgentToken) *WebSocketUI {
	return &WebSocketUI{
		ctx:          ctx,
		conn:         conn,
		facade:       NewEventFacade(),
		strategyChan: make(chan flows.FlowStrategy),
		reviewChan:   make(chan bool),
		approveChan:  make(chan bool),
		tokenChan:    tokenChan,
	}
}

func (ui *WebSocketUI) ConfirmTransition(targetDomain string) bool {
	// STUB: We will wire up the explicit Protobuf payloads for the frontend later.
	// For now, auto-approve so the server compiles and can execute transitions.
	return true
}

func (ui *WebSocketUI) sendBytes(data []byte) {
	ui.writeMu.Lock()
	defer ui.writeMu.Unlock()
	_ = ui.conn.Write(ui.ctx, websocket.MessageText, data)
}

func (ui *WebSocketUI) SendSyncHistory(data []byte) {
	ui.sendBytes(data)
}

func (ui *WebSocketUI) OnTextChunk(text string) {
	if data, err := ui.facade.MarshalChatStream(text); err == nil {
		ui.sendBytes(data)
	}
}

func (ui *WebSocketUI) ReviewContext(ctxData session.TurnContext) bool {
	if data, err := ui.facade.MarshalPreviewContext(ctxData); err == nil {
		ui.sendBytes(data)
	}
	// Blocks until the frontend responds (either instantly or after user review)
	return <-ui.approveChan
}

func (ui *WebSocketUI) ChooseNextStep() flows.FlowStrategy {
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

func (ui *WebSocketUI) PushStrategy(strategy flows.FlowStrategy) {
	ui.strategyChan <- strategy
}

func (ui *WebSocketUI) PushReviewDecision(accepted bool) {
	ui.reviewChan <- accepted
}

func (ui *WebSocketUI) PushContextApproval(approved bool) {
	ui.approveChan <- approved
}
