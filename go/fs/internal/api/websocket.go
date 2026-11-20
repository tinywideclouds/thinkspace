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

// ConnectionManager maintains the long-lived WebSocket connection and serializes outbound writes.
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
		strategyChan:   make(chan strategySelection, 1),
		reviewChan:     make(chan reviewSelection, 1),
		approveChan:    make(chan session.ContextApprovalDecision, 1),
		transitionChan: make(chan bool, 1),
		chunkChan:      chunkChan,
	}
}

func (cm *ConnectionManager) SendBytes(data []byte) {
	if cm == nil || cm.conn == nil || cm.ctx.Err() != nil {
		return
	}
	cm.writeMu.Lock()
	defer cm.writeMu.Unlock()
	if err := cm.conn.Write(cm.ctx, websocket.MessageText, data); err != nil {
		cm.logger.WarnContext(cm.ctx, "failed writing bytes to websocket connection", "error", err)
	}
}

func (cm *ConnectionManager) OnTextChunk(chunk workspace.NodeChunk) {
	if cm.chunkChan == nil {
		return
	}
	select {
	case cm.chunkChan <- chunk:
	case <-cm.ctx.Done():
	}
}

func (cm *ConnectionManager) GetChunkChannel() chan<- workspace.NodeChunk {
	return cm.chunkChan
}

func (cm *ConnectionManager) ConfirmTransition(targetThinkState string, reasoning string) bool {
	data, err := MarshalRequestTransition(targetThinkState, reasoning)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request transition", "error", err)
		return false
	}
	select {
	case <-cm.transitionChan:
	default:
	}
	cm.SendBytes(data)

	select {
	case approved := <-cm.transitionChan:
		return approved
	case <-cm.ctx.Done():
		return false
	}
}

func (cm *ConnectionManager) ReviewContext(ctxData session.TurnContext) session.ContextApprovalDecision {
	data, err := MarshalPreviewContext(ctxData)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling preview context for review", "error", err)
		return session.ContextApprovalDecision{Approved: false}
	}
	select {
	case <-cm.approveChan:
	default:
	}
	cm.SendBytes(data)

	select {
	case decision := <-cm.approveChan:
		return decision
	case <-cm.ctx.Done():
		return session.ContextApprovalDecision{Approved: false}
	}
}

func (cm *ConnectionManager) ChooseNextStep() (flows.FlowStrategy, bool) {
	data, err := MarshalRequestStrategy()
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request strategy", "error", err)
		return flows.StrategySkip, false
	}
	select {
	case <-cm.strategyChan:
	default:
	}
	cm.SendBytes(data)

	select {
	case sel := <-cm.strategyChan:
		return sel.strategy, sel.autoAccept
	case <-cm.ctx.Done():
		return flows.StrategySkip, false
	}
}

func (cm *ConnectionManager) ReviewCandidate(branch string) (bool, string) {
	data, err := MarshalRequestReview(branch)
	if err != nil {
		cm.logger.ErrorContext(cm.ctx, "failed marshaling request review", "error", err)
		return false, "Failed to marshal review request"
	}
	select {
	case <-cm.reviewChan:
	default:
	}
	cm.SendBytes(data)

	select {
	case sel := <-cm.reviewChan:
		return sel.accepted, sel.reason
	case <-cm.ctx.Done():
		return false, "Client disconnected during review"
	}
}

func (cm *ConnectionManager) PushStrategy(strategy flows.FlowStrategy, autoAccept bool) {
	select {
	case cm.strategyChan <- strategySelection{strategy: strategy, autoAccept: autoAccept}:
	case <-cm.ctx.Done():
	default:
		cm.logger.WarnContext(cm.ctx, "dropped unsolicited strategy selection")
	}
}

func (cm *ConnectionManager) PushReviewDecision(accepted bool, reason string) {
	select {
	case cm.reviewChan <- reviewSelection{accepted: accepted, reason: reason}:
	case <-cm.ctx.Done():
	default:
		cm.logger.WarnContext(cm.ctx, "dropped unsolicited review decision")
	}
}

func (cm *ConnectionManager) PushContextApproval(decision session.ContextApprovalDecision) {
	select {
	case cm.approveChan <- decision:
	case <-cm.ctx.Done():
	default:
		cm.logger.WarnContext(cm.ctx, "dropped unsolicited context approval")
	}
}

func (cm *ConnectionManager) PushTransitionDecision(approved bool) {
	select {
	case cm.transitionChan <- approved:
	case <-cm.ctx.Done():
	default:
		cm.logger.WarnContext(cm.ctx, "dropped unsolicited transition decision")
	}
}
