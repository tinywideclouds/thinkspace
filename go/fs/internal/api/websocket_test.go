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

func TestWebSocketUI_RoutingAndBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	// Test 1: OnTextChunk routing
	go func() {
		ui.OnTextChunk("hello world")
	}()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read event: %v", err)
	}

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

	// Test 2: ChooseNextStep Blocking and Channel Push
	strategyChosen := make(chan session.DelegationStrategy)
	go func() {
		strategyChosen <- ui.ChooseNextStep()
	}()

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read strategy request: %v", err)
	}

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

	ui.PushStrategy(session.StrategyRefine)

	select {
	case strategy := <-strategyChosen:
		if strategy != session.StrategyRefine {
			t.Errorf("expected StrategyRefine, got %v", strategy)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for strategy channel to unblock")
	}

	// Test 3: SendSyncHistory Direct Payload Routing
	go func() {
		ui.SendSyncHistory([]byte(`{"type":"sync_history"}`))
	}()

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read sync history: %v", err)
	}

	if string(data) != `{"type":"sync_history"}` {
		t.Errorf("expected sync history payload, got '%s'", string(data))
	}
}
