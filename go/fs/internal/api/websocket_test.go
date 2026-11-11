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

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/api"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

func TestWebSocketUI_RoutingAndBlocking(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var ui *api.TurnUI
	var connectionManager *api.ConnectionManager
	uiReady := make(chan struct{})
	chunkChan := make(chan workspace.NodeChunk, 10)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Fatalf("failed to accept ws: %v", err)
		}

		connectionManager = api.NewConnectionManager(r.Context(), conn, chunkChan, nil)

		coordinate := api.ProjectionCoordinate{SpaceID: "space-1", ChatID: "chat-1", ProjectionID: "main"}
		ui = api.NewTurnUI(connectionManager, coordinate)

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

	// Test 1: OnTextChunk routing pushes to chunkChan via embedded struct
	nodeID := uuid.NewV7()
	go func() {
		ui.OnTextChunk(workspace.NodeChunk{NodeID: nodeID, Text: "hello world"})
	}()

	select {
	case chunk := <-chunkChan:
		if chunk.Text != "hello world" {
			t.Errorf("expected text 'hello world', got '%s'", chunk.Text)
		}
		if chunk.NodeID != nodeID {
			t.Errorf("expected nodeId %s, got '%s'", nodeID.String(), chunk.NodeID.String())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for chunkChan push")
	}

	// Test 2: StreamGraphEvent routing via TurnUI with projection coordinates
	graphNodeID := uuid.NewV7()
	node := thinkgraph.Node{
		Event: thinkgraph.Event{
			ID:      graphNodeID,
			Type:    thinkgraph.EventModel,
			Content: "Durable node test",
		},
	}
	go func() {
		ui.StreamGraphEvent(node)
	}()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read streamed graph event: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	streamedEvent, ok := result["streamedGraphEvent"].(map[string]any)
	if !ok {
		t.Fatalf("expected streamedGraphEvent wrapper")
	}

	if streamedEvent["projectionId"] != "main" {
		t.Errorf("expected scoped ProjectionID 'main' to be attached to envelope, got: %v", streamedEvent["projectionId"])
	}

	graphEvent, ok := streamedEvent["event"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested 'event' object")
	}
	if graphEvent["id"] != graphNodeID.String() {
		t.Errorf("expected node ID %s, got: %v", graphNodeID.String(), graphEvent["id"])
	}

	// Test 3: ChooseNextStep Blocking and Channel Push via embedded struct
	strategyChosen := make(chan flows.FlowStrategy)
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
		t.Fatalf("failed to unmarshal strategy request: %v", err)
	}
	if !reqPayload.RequestStrategy.Active {
		t.Errorf("expected active true, got false")
	}

	connectionManager.PushStrategy(flows.StrategyRefine)

	select {
	case strategy := <-strategyChosen:
		if strategy != flows.StrategyRefine {
			t.Errorf("expected StrategyRefine, got %v", strategy)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for strategy channel to unblock")
	}

	// Test 4: ConfirmTransition Blocking and Channel Push via embedded struct
	transitionChosen := make(chan bool)
	go func() {
		transitionChosen <- ui.ConfirmTransition(signals.SignalTransition{
			TargetThinkState: "golang",
			Reasoning:        "need code",
		})
	}()

	_, data, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("failed to read transition request: %v", err)
	}

	var transPayload struct {
		RequestTransition struct {
			TargetThinkState string `json:"targetThinkState"`
			Reasoning        string `json:"reasoning"`
		} `json:"requestTransition"`
	}

	if err := json.Unmarshal(data, &transPayload); err != nil {
		t.Fatalf("failed to unmarshal transition request: %v", err)
	}
	if transPayload.RequestTransition.TargetThinkState != "golang" {
		t.Errorf("expected golang, got %s", transPayload.RequestTransition.TargetThinkState)
	}

	connectionManager.PushTransitionDecision(true)

	select {
	case decision := <-transitionChosen:
		if !decision {
			t.Errorf("expected transition decision true, got false")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for transition channel to unblock")
	}
}
