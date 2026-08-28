package api

import (
	"errors"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace/flows"
)

// --- Domain Types ---
// These ensure the rest of the app never imports the Protobuf package directly.

type SpaceInfo struct {
	ID   string
	Name string
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

// Outbound Serialization

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

func (f *EventFacade) MarshalDelegationStart(agentCount int, instructions string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_DelegationStart{
			DelegationStart: &pb.DelegationStartPayload{
				AgentCount:   int32(agentCount),
				Instructions: instructions,
			},
		},
	}
	return f.marshaler.Marshal(event)
}

func (f *EventFacade) MarshalDelegationComplete(summary string) ([]byte, error) {
	event := &pb.WSEvent{
		Payload: &pb.WSEvent_DelegationComplete{
			DelegationComplete: &pb.DelegationCompletePayload{
				Summary: summary,
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

func (f *EventFacade) MarshalAvailableSpaces(spaces []SpaceInfo) ([]byte, error) {
	var pbSpaces []*pb.SpaceInfo
	for _, s := range spaces {
		pbSpaces = append(pbSpaces, &pb.SpaceInfo{
			Id:   s.ID,
			Name: s.Name,
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

// Inbound Deserialization

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
		}
	case *pb.WSEvent_SelectStrategy:
		domainEvent.Type = "select_strategy"
		domainEvent.SelectStrategy = &SelectStrategyPayload{
			StrategyID: session.DelegationStrategy(payload.SelectStrategy.StrategyId),
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
