package api

import (
	"encoding/json"
	"errors"
	"time"
	"uuid"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/chat"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
)

// --- Domain Types ---

type SpaceState struct {
	ID           string
	Name         string
	IsConfigured bool
}

type InboundEvent struct {
	Type           string
	SubmitPrompt   *SubmitPromptPayload
	SelectStrategy *SelectStrategyPayload
	ReviewDecision *ReviewDecisionPayload
}

type SubmitPromptPayload struct {
	Text    string
	SpaceID string
	ChatID  string
}

type SelectStrategyPayload struct {
	StrategyID session.DelegationStrategy
}

type ReviewDecisionPayload struct {
	Branch   string
	Accepted bool
}

// --- Facade Implementation ---

type EventFacade struct {
	marshaler   protojson.MarshalOptions
	unmarshaler protojson.UnmarshalOptions
}

func NewEventFacade() *EventFacade {
	return &EventFacade{
		marshaler: protojson.MarshalOptions{
			EmitUnpopulated: true,
		},
		unmarshaler: protojson.UnmarshalOptions{
			DiscardUnknown: true,
		},
	}
}

func (f *EventFacade) MarshalSyncHistory(recentEvents []chat.Event, digests map[uuid.UUID]chat.DigestMeta) ([]byte, error) {
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
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalRESTSpaces(spaces []SpaceState) ([]byte, error) {
	var rawSpaces []json.RawMessage

	for _, s := range spaces {
		pbSpace := &pb.SpaceInfo{
			Id:           s.ID,
			Name:         s.Name,
			IsConfigured: s.IsConfigured,
		}

		b, err := f.marshaler.Marshal(pbSpace)
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

// UnmarshalRESTSpaces completes the Facade symmetry, hiding the protojson array parsing from tests
func (f *EventFacade) UnmarshalRESTSpaces(data []byte) ([]SpaceState, error) {
	var rawSpaces []json.RawMessage
	if err := json.Unmarshal(data, &rawSpaces); err != nil {
		return nil, err
	}

	var spaces []SpaceState
	for _, raw := range rawSpaces {
		var pbSpace pb.SpaceInfo
		if err := f.unmarshaler.Unmarshal(raw, &pbSpace); err != nil {
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

func (f *EventFacade) MarshalAvailableSpaces(spaces []SpaceState) ([]byte, error) {
	var pbSpaces []*pb.SpaceInfo
	for _, s := range spaces {
		pbSpaces = append(pbSpaces, &pb.SpaceInfo{
			Id:           s.ID,
			Name:         s.Name,
			IsConfigured: s.IsConfigured,
		})
	}

	event := &pb.WSEvent{
		Payload: &pb.WSEvent_AvailableSpaces{
			AvailableSpaces: &pb.AvailableSpacesPayload{
				Spaces: pbSpaces,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalChatStream(text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_ChatStream{
			ChatStream: &pb.ChatStreamPayload{
				Text: text,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalRequestStrategy() ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestStrategy{
			RequestStrategy: &pb.RequestStrategyPayload{
				Active: true,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalRequestReview(branch string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_RequestReview{
			RequestReview: &pb.RequestReviewPayload{
				Branch: branch,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalAgentStream(agentID int, text string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_AgentStream{
			AgentStream: &pb.AgentStreamPayload{
				AgentId: int32(agentID),
				Text:    text,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalFlowEvent(event flows.FlowEvent) ([]byte, error) {
	pbEvent := &pb.WSEvent{
		Payload: &pb.WSEvent_FlowEvent{
			FlowEvent: &pb.FlowEventPayload{
				FlowId:      event.FlowID,
				Type:        string(event.Type),
				Timestamp:   event.Timestamp.Format(time.RFC3339),
				TaskId:      event.TaskID,
				AgentCount:  int32(event.AgentCount),
				AgentId:     event.AgentID,
				AgentIndex:  int32(event.AgentIndex),
				Instruction: event.Instruction,
				Status:      event.Status,
				Attempt:     int32(event.Attempt),
				Trace:       event.Trace,
				CandidateId: event.CandidateID,
				Passed:      event.Passed,
			},
		},
	}
	return f.marshaler.Marshal(pbEvent)
}

func (f *EventFacade) UnmarshalInbound(data []byte) (*InboundEvent, error) {
	var pbEvent pb.WSEvent
	if err := f.unmarshaler.Unmarshal(data, &pbEvent); err != nil {
		return nil, err
	}

	domainEvent := &InboundEvent{}

	switch payload := pbEvent.Payload.(type) {
	case *pb.WSEvent_SubmitPrompt:
		domainEvent.Type = "submit_prompt"
		domainEvent.SubmitPrompt = &SubmitPromptPayload{
			Text:    payload.SubmitPrompt.Text,
			SpaceID: payload.SubmitPrompt.SpaceId,
			ChatID:  payload.SubmitPrompt.ChatId,
		}
	case *pb.WSEvent_SelectStrategy:
		domainEvent.Type = "select_strategy"

		// Explicit Mapping to prevent off-by-one enum drift
		var strategy session.DelegationStrategy
		switch payload.SelectStrategy.StrategyId {
		case pb.DelegationStrategy_DELEGATION_STRATEGY_SKIP:
			strategy = session.StrategySkip
		case pb.DelegationStrategy_DELEGATION_STRATEGY_MANUAL:
			strategy = session.StrategyManual
		case pb.DelegationStrategy_DELEGATION_STRATEGY_REVIEW:
			strategy = session.StrategyReview
		case pb.DelegationStrategy_DELEGATION_STRATEGY_REFINE:
			strategy = session.StrategyRefine
		default:
			strategy = session.StrategyManual // Safe fallback
		}

		domainEvent.SelectStrategy = &SelectStrategyPayload{
			StrategyID: strategy,
		}
	case *pb.WSEvent_ReviewDecision:
		domainEvent.Type = "review_decision"
		domainEvent.ReviewDecision = &ReviewDecisionPayload{
			Branch:   payload.ReviewDecision.Branch,
			Accepted: payload.ReviewDecision.Accepted,
		}
	default:
		return nil, errors.New("unsupported or unknown inbound event type")
	}

	return domainEvent, nil
}
