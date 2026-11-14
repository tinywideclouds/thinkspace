package api

import (
	"context"
	"log/slog"
	"sync"

	"github.com/coder/websocket"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type strategySelection struct {
	strategy   flows.FlowStrategy
	autoAccept bool
}

type reviewSelection struct {
	accepted bool
	reason   string
}

// ConnectionManager maintains the long-lived WebSocket connection and routes bytes.
type ConnectionManager struct {
	ctx            context.Context
	conn           *websocket.Conn
	writeMu        sync.Mutex
	logger         *slog.Logger
	strategyChan   chan strategySelection
	reviewChan     chan reviewSelection
	approveChan    chan session.ContextApprovalDecision
	transitionChan chan bool
	chunkChan      chan<- workspace.NodeChunk
}

func NewConnectionManager(ctx context.Context, conn *websocket.Conn, chunkChan chan<- workspace.NodeChunk, logger *slog.Logger) *ConnectionManager {
	if logger == nil {
		logger = slog.Default()
	}
	return &ConnectionManager{
		ctx:            ctx,
		conn:           conn,
		logger:         logger,
		strategyChan:   make(chan strategySelection),
		reviewChan:     make(chan reviewSelection),
		approveChan:    make(chan session.ContextApprovalDecision),
		transitionChan: make(chan bool),
		chunkChan:      chunkChan,
	}
}

func (cm *ConnectionManager) SendBytes(data []byte) {
	cm.writeMu.Lock()
	defer cm.writeMu.Unlock()
	if err := cm.conn.Write(cm.ctx, websocket.MessageText, data); err != nil {
		cm.logger.WarnContext(cm.ctx, "failed writing bytes to websocket connection", "error", err)
	}
}

func (cm *ConnectionManager) OnTextChunk(chunk workspace.NodeChunk) {
	if cm.chunkChan != nil {
		cm.chunkChan <- chunk
	}
}

func (cm *ConnectionManager) GetChunkChannel() chan<- workspace.NodeChunk {
	return cm.chunkChan
}

func (cm *ConnectionManager) ConfirmTransition(targetThinkState string, reasoning string) bool {
	data, err := MarshalRequestTransition(targetThinkState, reasoning)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request transition", "error", err)
	} else {
		cm.SendBytes(data)
	}
	return <-cm.transitionChan
}

func (cm *ConnectionManager) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	data, err := MarshalPreviewContext(ctxData)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling preview context for review", "error", err)
	} else {
		cm.SendBytes(data)
	}
	return <-cm.approveChan
}

func (cm *ConnectionManager) ChooseNextStep() (flows.FlowStrategy, bool) {
	data, err := MarshalRequestStrategy()
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request strategy", "error", err)
	} else {
		cm.SendBytes(data)
	}
	sel := <-cm.strategyChan
	return sel.strategy, sel.autoAccept
}

func (cm *ConnectionManager) ReviewCandidate(branch string) (bool, string) {
	data, err := MarshalRequestReview(branch)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request review", "error", err)
	} else {
		cm.SendBytes(data)
	}
	sel := <-cm.reviewChan
	return sel.accepted, sel.reason
}

func (cm *ConnectionManager) PushStrategy(strategy flows.FlowStrategy, autoAccept bool) {
	cm.strategyChan <- strategySelection{strategy: strategy, autoAccept: autoAccept}
}

func (cm *ConnectionManager) PushReviewDecision(accepted bool, reason string) {
	cm.reviewChan <- reviewSelection{accepted: accepted, reason: reason}
}

func (cm *ConnectionManager) PushContextApproval(decision session.ContextApprovalDecision) {
	cm.approveChan <- decision
}

func (cm *ConnectionManager) PushTransitionDecision(approved bool) {
	cm.transitionChan <- approved
}
