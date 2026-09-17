package api

import (
	"context"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

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

func (w *WebSocketEmitter) Emit(channel string, event telemetry.Event) {
	bytes, err := w.facade.MarshalTelemetryEvent(channel, event)
	if err == nil {
		if writeErr := w.conn.Write(w.ctx, websocket.MessageText, bytes); writeErr != nil {
			// Client disconnected, swallow and return cleanly
			return
		}
	}
}
