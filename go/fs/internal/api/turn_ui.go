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
}

func NewTurnUI(connectionManager *ConnectionManager) *TurnUI {
	return &TurnUI{
		ConnectionManager: connectionManager,
	}
}

// StreamGraphEvent satisfies the final requirement of session.UserInterface.
// It intercepts the pure node and address from the domain, attaches the transport coordinates,
// marshals the envelope, and dispatches it via the embedded ConnectionManager.
func (ui *TurnUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {
	if ui == nil || ui.ConnectionManager == nil || address == nil {
		return
	}

	coordinate := ProjectionCoordinate{
		SpaceID:      address.ProjectID,
		ChatID:       address.ChatID,
		ProjectionID: address.SubGraph.Name,
	}

	data, err := MarshalStreamedGraphEvent(node, coordinate)
	if err != nil {
		ui.logger.Error("failed to marshal streamed graph event envelope", "error", err, "node_id", node.ID)
		return
	}
	ui.SendBytes(data) // Promoted directly from ConnectionManager
}
