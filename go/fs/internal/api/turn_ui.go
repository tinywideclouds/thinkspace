package api

import (
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

// TurnUI implements session.UserInterface. It is a scoped decorator instantiated
// per-turn by the server to safely inject network routing coordinates into outgoing
// graph nodes without exposing those transport concepts to the cognitive engines.
type TurnUI struct {
	// Embedding automatically promotes all standard UI methods (OnTextChunk, ReviewContext, etc.)
	*ConnectionManager
	coordinate ProjectionCoordinate
}

func NewTurnUI(conn *ConnectionManager, coordinate ProjectionCoordinate) *TurnUI {
	return &TurnUI{
		ConnectionManager: conn,
		coordinate:        coordinate,
	}
}

// StreamGraphEvent satisfies the final requirement of session.UserInterface.
// It intercepts the pure node from the domain, attaches the transport coordinates,
// marshals the envelope, and dispatches it via the embedded ConnectionManager.
func (ui *TurnUI) StreamGraphEvent(node thinkgraph.Node) {
	data, err := MarshalStreamedGraphEvent(node, ui.coordinate)
	if err != nil {
		ui.logger.Error("failed to marshal streamed graph event envelope", "error", err, "node_id", node.ID)
		return
	}
	ui.SendBytes(data) // Promoted directly from ConnectionManager
}
