package api

import (
	"context"

	"github.com/coder/websocket"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
)

type WebSocketEmitter struct {
	sendFn func([]byte)
}

func NewWebSocketEmitter(ctx context.Context, conn *websocket.Conn) *WebSocketEmitter {
	return &WebSocketEmitter{
		sendFn: func(data []byte) {
			if conn != nil {
				_ = conn.Write(ctx, websocket.MessageText, data)
			}
		},
	}
}

func NewConnectionEmitter(cm *ConnectionManager) *WebSocketEmitter {
	return &WebSocketEmitter{
		sendFn: func(data []byte) {
			if cm != nil {
				cm.SendBytes(data)
			}
		},
	}
}

func (w *WebSocketEmitter) Emit(channel string, event telemetry.Event) {
	if w == nil || w.sendFn == nil {
		return
	}
	bytes, err := MarshalTelemetryEvent(channel, event)
	if err == nil {
		w.sendFn(bytes)
	}
}
