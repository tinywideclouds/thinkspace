package session

import (
	"context"
	"log/slog"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// GraphStreamer is the minimal UI capability needed to push graph node updates.
type GraphStreamer = signals.GraphStreamer

// GraphBroadcaster coordinates dual-write graph persistence (Ledger + BadgerGraph)
// with immediate UI node streaming.
type GraphBroadcaster struct {
	logger     *slog.Logger
	dispatcher *thinkgraph.Dispatcher
	graph      thinkgraph.GraphRepository
}

func NewGraphBroadcaster(
	logger *slog.Logger,
	dispatcher *thinkgraph.Dispatcher,
	graph thinkgraph.GraphRepository,
) *GraphBroadcaster {
	return &GraphBroadcaster{
		logger:     logger,
		dispatcher: dispatcher,
		graph:      graph,
	}
}

func NewGraphBroadcasterFromService(logger *slog.Logger, svc *workspace.Service) *GraphBroadcaster {
	if svc == nil {
		return &GraphBroadcaster{logger: logger}
	}
	return &GraphBroadcaster{
		logger:     logger,
		dispatcher: svc.Dispatcher(),
		graph:      svc.Graph(),
	}
}

func asGraphStreamer(ui any) GraphStreamer {
	if ui == nil {
		return nil
	}
	if streamer, ok := ui.(GraphStreamer); ok {
		return streamer
	}
	return nil
}

// AppendAndStream persists an event via the dispatcher and immediately streams the resulting node to the UI.
func (b *GraphBroadcaster) AppendAndStream(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	ev thinkgraph.Event,
	ui any,
) error {
	if ev.GraphID.IsZero() && address != nil && !address.GraphID.IsZero() {
		ev.GraphID = address.GraphID
	}

	if b.dispatcher != nil {
		if err := b.dispatcher.AppendEvent(ctx, address, ev); err != nil {
			return err
		}
	}

	if streamer := asGraphStreamer(ui); streamer != nil {
		streamer.StreamGraphEvent(address, thinkgraph.Node{Event: ev})
	}
	return nil
}

// MutateAndStream applies a graph mutation, re-fetches the materialized node, and streams it to the UI.
func (b *GraphBroadcaster) MutateAndStream(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	mut thinkgraph.Mutator,
	targetID uuid.UUID,
	ui any,
) error {
	if b.dispatcher != nil {
		if err := b.dispatcher.SaveMutation(ctx, address, mut); err != nil {
			if b.logger != nil {
				b.logger.ErrorContext(ctx, "failed to save graph mutation", "error", err, "target_id", targetID)
			}
			return err
		}
	}

	b.StreamNode(ctx, address, targetID, ui)
	return nil
}

// StreamNode fetches the current materialized node from the read graph and forwards it to the UI.
func (b *GraphBroadcaster) StreamNode(
	ctx context.Context,
	address *thinkgraph.ChatAddress,
	nodeID uuid.UUID,
	ui any,
) {
	streamer := asGraphStreamer(ui)
	if streamer == nil || b.graph == nil || nodeID == uuid.Nil() {
		return
	}

	nodes, err := b.graph.GetNodes(ctx, address, []string{nodeID.String()})
	if err != nil {
		if b.logger != nil {
			b.logger.WarnContext(ctx, "failed to fetch node for UI streaming", "node_id", nodeID, "error", err)
		}
		return
	}
	if len(nodes) > 0 {
		streamer.StreamGraphEvent(address, nodes[0])
	}
}
