package api

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type WebSocketUI struct {
	ctx            context.Context
	conn           *websocket.Conn
	writeMu        sync.Mutex
	logger         *slog.Logger
	strategyChan   chan flows.FlowStrategy
	reviewChan     chan bool
	approveChan    chan session.ContextApprovalDecision
	transitionChan chan bool
	tokenChan      chan<- workspace.AgentToken
}

func NewWebSocketUI(ctx context.Context, conn *websocket.Conn, tokenChan chan<- workspace.AgentToken, logger *slog.Logger) *WebSocketUI {
	if logger == nil {
		logger = slog.Default()
	}
	return &WebSocketUI{
		ctx:            ctx,
		conn:           conn,
		logger:         logger,
		strategyChan:   make(chan flows.FlowStrategy),
		reviewChan:     make(chan bool),
		approveChan:    make(chan session.ContextApprovalDecision),
		transitionChan: make(chan bool),
		tokenChan:      tokenChan,
	}
}

func (ui *WebSocketUI) ConfirmTransition(req signals.SignalTransition) bool {
	data, err := MarshalRequestTransition(req.TargetThinkState, req.Reasoning)
	if err != nil {
		ui.logger.ErrorContext(ui.ctx, "failed marshaling request transition", "error", err)
	} else {
		ui.sendBytes(data)
	}
	return <-ui.transitionChan
}

func (ui *WebSocketUI) sendBytes(data []byte) {
	ui.writeMu.Lock()
	defer ui.writeMu.Unlock()
	if err := ui.conn.Write(ui.ctx, websocket.MessageText, data); err != nil {
		ui.logger.WarnContext(ui.ctx, "failed writing bytes to websocket connection", "error", err)
	}
}

func (ui *WebSocketUI) SendSyncHistory(data []byte) {
	ui.sendBytes(data)
}

func (ui *WebSocketUI) SendSyncSubGraph(data []byte) {
	ui.sendBytes(data)
}

func (ui *WebSocketUI) OnTextChunk(text string) {
	data, err := MarshalChatStream(text)
	if err != nil {
		ui.logger.ErrorContext(ui.ctx, "failed marshaling chat stream text chunk", "error", err)
		return
	}
	ui.sendBytes(data)
}

func (ui *WebSocketUI) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	data, err := MarshalPreviewContext(ctxData)
	if err != nil {
		ui.logger.ErrorContext(ui.ctx, "failed marshaling preview context for review", "error", err)
	} else {
		ui.sendBytes(data)
	}
	return <-ui.approveChan
}

func (ui *WebSocketUI) ChooseNextStep() flows.FlowStrategy {
	data, err := MarshalRequestStrategy()
	if err != nil {
		ui.logger.ErrorContext(ui.ctx, "failed marshaling request strategy", "error", err)
	} else {
		ui.sendBytes(data)
	}
	return <-ui.strategyChan
}

func (ui *WebSocketUI) ReviewCandidate(branch string) bool {
	data, err := MarshalRequestReview(branch)
	if err != nil {
		ui.logger.ErrorContext(ui.ctx, "failed marshaling request review", "error", err)
	} else {
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
