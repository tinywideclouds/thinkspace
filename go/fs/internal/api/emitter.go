package api

import (
	"context"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

type WebSocketEmitter struct {
	ctx  context.Context
	conn *websocket.Conn
}

func NewWebSocketEmitter(ctx context.Context, conn *websocket.Conn) *WebSocketEmitter {
	return &WebSocketEmitter{
		ctx:  ctx,
		conn: conn,
	}
}

func (w *WebSocketEmitter) Emit(channel string, event telemetry.Event) {
	bytes, err := MarshalTelemetryEvent(channel, event)
	if err == nil {
		if writeErr := w.conn.Write(w.ctx, websocket.MessageText, bytes); writeErr != nil {
			return
		}
	}
}
