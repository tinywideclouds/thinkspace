package api

import (
	"encoding/json"
	"time"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

func MarshalSyncHistory(recentEvents []thinkgraph.Node, digests []thinkgraph.DigestMeta, focus thinkgraph.AttentionFocus, nextCursor string, hasMore bool) ([]byte, error) {
	pbEvents := ProjectTimeline(recentEvents)

	pbDigests := make(map[string]*pb.DigestMeta)
	for _, d := range digests {
		pbDigests[d.ID.String()] = &pb.DigestMeta{
			Id:       d.ID.String(),
			Summary:  d.Summary,
			IsSticky: d.Promoted,
		}
	}

	pbFocus := &pb.AttentionFocus{
		PrimaryFocus:   focus.PrimaryFocus,
		RelatedContext: focus.RelatedContext,
		DormantLenses:  focus.DormantLenses,
	}

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_SyncHistory{
			SyncHistory: &pb.SyncHistory{
				RecentEvents: pbEvents,
				Digests:      pbDigests,
				Focus:        pbFocus,
				NextCursor:   nextCursor,
				HasMore:      hasMore,
			},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalSyncSubGraph(events []thinkgraph.Node) ([]byte, error) {
	pbEvents := ProjectTimeline(events)

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_SyncSubGraph{
			SyncSubGraph: &pb.SyncSubGraph{
				Events: pbEvents,
			},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRESTSpaces(spaces []SpaceState) ([]byte, error) {
	var rawSpaces []json.RawMessage
	for _, s := range spaces {
		pbSpace := &pb.SpaceInfo{Id: s.ID, Name: s.Name, IsConfigured: s.IsConfigured}
		b, err := protoMarshaler.Marshal(pbSpace)
		if err != nil {
			return nil, err
		}
		rawSpaces = append(rawSpaces, b)
	}
	if rawSpaces == nil {
		rawSpaces = make([]json.RawMessage, 0)
	}
	return json.Marshal(rawSpaces)
}

func UnmarshalRESTSpaces(data []byte) ([]SpaceState, error) {
	var rawSpaces []json.RawMessage
	if err := json.Unmarshal(data, &rawSpaces); err != nil {
		return nil, err
	}

	var spaces []SpaceState
	for _, raw := range rawSpaces {
		var pbSpace pb.SpaceInfo
		if err := protoUnmarshaler.Unmarshal(raw, &pbSpace); err != nil {
			return nil, err
		}
		spaces = append(spaces, SpaceState{
			ID:           pbSpace.Id,
			Name:         pbSpace.Name,
			IsConfigured: pbSpace.IsConfigured,
		})
	}
	return spaces, nil
}

func MarshalAvailableSpaces(spaces []SpaceState) ([]byte, error) {
	var pbSpaces []*pb.SpaceInfo
	for _, s := range spaces {
		pbSpaces = append(pbSpaces, &pb.SpaceInfo{Id: s.ID, Name: s.Name, IsConfigured: s.IsConfigured})
	}

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_AvailableSpaces{
			AvailableSpaces: &pb.AvailableSpaces{Spaces: pbSpaces},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalChatStream(text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_ChatStream{
			ChatStream: &pb.ChatStream{Text: text},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestStrategy() ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestStrategy{
			RequestStrategy: &pb.RequestStrategy{Active: true},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestReview(branch string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestReview{
			RequestReview: &pb.RequestReview{Branch: branch},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestTransition(targetThinkState string, reasoning string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestTransition{
			RequestTransition: &pb.RequestTransition{
				TargetThinkState: targetThinkState,
				Reasoning:        reasoning,
			},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalAgentStream(agentID int, text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_AgentStream{
			AgentStream: &pb.AgentStream{AgentId: int32(agentID), Text: text},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalPreviewContext(ctxData session.TurnContext) ([]byte, error) {
	var fileContexts []*pb.FileContextMetaData
	for _, fc := range ctxData.FileContexts {
		fileContexts = append(fileContexts, &pb.FileContextMetaData{
			Path:       fc.Path,
			Level:      fc.Level,
			TokenCount: int32(fc.TokenCount),
		})
	}

	pbFocus := &pb.AttentionFocus{
		PrimaryFocus:   ctxData.Focus.PrimaryFocus,
		RelatedContext: ctxData.Focus.RelatedContext,
		DormantLenses:  ctxData.Focus.DormantLenses,
	}

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_PreviewContext{
			PreviewContext: &pb.PreviewContext{
				SpaceId:             ctxData.SpaceID,
				ChatId:              ctxData.ChatID,
				PreviewId:           ctxData.PreviewID,
				ManagerSystemPrompt: ctxData.ManagerSystemPrompt,
				StickyDigests:       ctxData.StickyDigests,
				RecentHistory:       ctxData.RecentHistory,
				FileContexts:        fileContexts,
				Focus:               pbFocus,
			},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalTelemetryEvent(channel string, event telemetry.Event) ([]byte, error) {
	var payloadJSON string
	if len(event.Payload) > 0 {
		b, err := json.Marshal(event.Payload)
		if err != nil {
			return nil, err
		}
		payloadJSON = string(b)
	}

	pbEvent := &pb.WSEvent{
		Payload: &pb.WSEvent_TelemetryEvent{
			TelemetryEvent: &pb.TelemetryEvent{
				Channel: channel,
				Event: &pb.TelemetryData{
					Id:          event.ID.String(),
					Type:        event.Type,
					Timestamp:   event.Timestamp.Format(time.RFC3339),
					Message:     event.Message,
					Summary:     event.Summary,
					PayloadJson: payloadJSON,
				},
			},
		},
	}
	return protoMarshaler.Marshal(pbEvent)
}
