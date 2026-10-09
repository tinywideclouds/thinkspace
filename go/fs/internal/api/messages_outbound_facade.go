package api

import (
	"encoding/json"
	"time"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"uuid"
)

func MarshalSyncHistory(recentEvents []chat.Event, digests map[uuid.UUID]chat.DigestMeta) ([]byte, error) {
	var pbEvents []*pb.LedgerEvent
	for _, e := range recentEvents {
		pbEvents = append(pbEvents, &pb.LedgerEvent{
			Id:        e.ID.String(),
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Type:      string(e.Type),
			Content:   e.Content,
			Metadata:  e.Metadata,
		})
	}

	pbDigests := make(map[string]*pb.DigestMeta)
	for id, d := range digests {
		pbDigests[id.String()] = &pb.DigestMeta{
			Id:       d.ID.String(),
			Summary:  d.Summary,
			IsSticky: d.IsSticky,
		}
	}

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_SyncHistory{
			SyncHistory: &pb.SyncHistoryPayload{
				RecentEvents: pbEvents,
				Digests:      pbDigests,
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
			AvailableSpaces: &pb.AvailableSpacesPayload{Spaces: pbSpaces},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalChatStream(text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_ChatStream{
			ChatStream: &pb.ChatStreamPayload{Text: text},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestStrategy() ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestStrategy{
			RequestStrategy: &pb.RequestStrategyPayload{Active: true},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestReview(branch string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestReview{
			RequestReview: &pb.RequestReviewPayload{Branch: branch},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalRequestTransition(targetDomain string, reasoning string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestTransition{
			RequestTransition: &pb.RequestTransitionPayload{
				TargetDomain: targetDomain,
				Reasoning:    reasoning,
			},
		},
	}
	return protoMarshaler.Marshal(event)
}

func MarshalAgentStream(agentID int, text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_AgentStream{
			AgentStream: &pb.AgentStreamPayload{AgentId: int32(agentID), Text: text},
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

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_PreviewContext{
			PreviewContext: &pb.PreviewContextPayload{
				SpaceId:             ctxData.SpaceID,
				ChatId:              ctxData.ChatID,
				PreviewId:           ctxData.PreviewID,
				ManagerSystemPrompt: ctxData.ManagerSystemPrompt,
				StickyDigests:       ctxData.StickyDigests,
				RecentHistory:       ctxData.RecentHistory,
				FileContexts:        fileContexts,
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
			TelemetryEvent: &pb.TelemetryEventPayload{
				Channel: channel,
				Event: &pb.TelemetryEvent{
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
