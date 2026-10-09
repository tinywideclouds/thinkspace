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
	ctx            context.Context
	conn           *websocket.Conn
	writeMu        sync.Mutex
	strategyChan   chan flows.FlowStrategy
	reviewChan     chan bool
	approveChan    chan session.ContextApprovalDecision
	transitionChan chan bool
	tokenChan      chan<- workspace.AgentToken
}

func NewWebSocketUI(ctx context.Context, conn *websocket.Conn, tokenChan chan<- workspace.AgentToken) *WebSocketUI {
	return &WebSocketUI{
		ctx:            ctx,
		conn:           conn,
		strategyChan:   make(chan flows.FlowStrategy),
		reviewChan:     make(chan bool),
		approveChan:    make(chan session.ContextApprovalDecision),
		transitionChan: make(chan bool),
		tokenChan:      tokenChan,
	}
}

func (ui *WebSocketUI) ConfirmTransition(targetDomain string, reasoning string) bool {
	if data, err := MarshalRequestTransition(targetDomain, reasoning); err == nil {
		ui.sendBytes(data)
	}
	return <-ui.transitionChan
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
	if data, err := MarshalChatStream(text); err == nil {
		ui.sendBytes(data)
	}
}

func (ui *WebSocketUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	if data, err := MarshalPreviewContext(ctxData); err == nil {
		ui.sendBytes(data)
	}
	return <-ui.approveChan
}

func (ui *WebSocketUI) ChooseNextStep() flows.FlowStrategy {
	if data, err := MarshalRequestStrategy(); err == nil {
		ui.sendBytes(data)
	}
	return <-ui.strategyChan
}

func (ui *WebSocketUI) ReviewCandidate(branch string) bool {
	if data, err := MarshalRequestReview(branch); err == nil {
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

func (ui *WebSocketUI) PushContextApproval(decision session.ContextApprovalDecision) {
	ui.approveChan <- decision
}

func (ui *WebSocketUI) PushTransitionDecision(approved bool) {
	ui.transitionChan <- approved
}
