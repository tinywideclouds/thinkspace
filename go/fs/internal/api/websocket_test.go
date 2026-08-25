package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestServer_WebSocketHandshake(t *testing.T) {
	srv, expectedSpaceID := setupTestServer(t)
	httpServer := httptest.NewServer(srv.Handler())
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read raw bytes instead of using wsjson
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read handshake event: %v", err)
	}

	// Mirror the @bufbuild/protobuf JSON output for AvailableSpaces
	var payload struct {
		AvailableSpaces struct {
			Spaces []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"spaces"`
		} `json:"availableSpaces"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v (data: %s)", err, string(data))
	}

	if len(payload.AvailableSpaces.Spaces) != 1 || payload.AvailableSpaces.Spaces[0].ID != expectedSpaceID {
		t.Errorf("expected 1 space with ID %s, got %+v", expectedSpaceID, payload.AvailableSpaces.Spaces)
	}
}

func TestWebSocketUI_RoutingAndBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Create a raw WebSocket handler to isolate the UI struct
	var ui *api.WebSocketUI
	uiReady := make(chan struct{})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("failed to accept ws: %v", err)
		}

		tokenChan := make(chan workspace.AgentToken, 10)
		ui = api.NewWebSocketUI(r.Context(), conn, tokenChan)
		close(uiReady)

		// Keep connection alive for test
		<-r.Context().Done()
	})

	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()

	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial websocket: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	<-uiReady

	// 2. Test Outbound Serialization (Server -> Client)
	go func() {
		ui.OnTextChunk("hello world")
	}()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

	// Mirror the @bufbuild/protobuf JSON output for ChatStream
	var chatPayload struct {
		ChatStream struct {
			Text string `json:"text"`
		} `json:"chatStream"`
	}

	if err := json.Unmarshal(data, &chatPayload); err != nil {
		t.Fatalf("failed to unmarshal payload: %v (data: %s)", err, string(data))
	}
	if chatPayload.ChatStream.Text != "hello world" {
		t.Errorf("expected text 'hello world', got '%s'", chatPayload.ChatStream.Text)
	}

	// 3. Test Inbound Routing & Channel Blocking (Client -> Server)
	strategyChosen := make(chan session.DelegationStrategy)
	go func() {
		// This should block until PushStrategy is called
		strategyChosen <- ui.ChooseNextStep()
	}()

	// Read the outbound request event
	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read strategy request: %v", err)
	}

	// Mirror the @bufbuild/protobuf JSON output for RequestStrategy
	var reqPayload struct {
		RequestStrategy struct {
			Active bool `json:"active"`
		} `json:"requestStrategy"`
	}

	if err := json.Unmarshal(data, &reqPayload); err != nil {
		t.Fatalf("failed to unmarshal strategy request: %v (data: %s)", err, string(data))
	}
	if !reqPayload.RequestStrategy.Active {
		t.Errorf("expected active true, got false")
	}

	// Simulate receiving the inbound choice from the web router
	ui.PushStrategy(session.StrategyRefine)

	select {
	case strategy := <-strategyChosen:
		if strategy != session.StrategyRefine {
			t.Errorf("expected StrategyRefine, got %v", strategy)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for strategy channel to unblock")
	}
}
