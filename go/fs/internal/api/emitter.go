package api

import (
	"context"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

// WebSocketEmitter pushes flow events directly to a connected frontend client.
type WebSocketEmitter struct {
	ctx    context.Context
	conn   *websocket.Conn
	facade *EventFacade
}

func NewWebSocketEmitter(ctx context.Context, conn *websocket.Conn, facade *EventFacade) *WebSocketEmitter {
	return &WebSocketEmitter{
		ctx:    ctx,
		conn:   conn,
		facade: facade,
	}
}

func (w *WebSocketEmitter) Emit(event flows.FlowEvent) {
	bytes, err := w.facade.MarshalFlowEvent(event)
	if err == nil {
		_ = w.conn.Write(w.ctx, websocket.MessageText, bytes)
	}
}
