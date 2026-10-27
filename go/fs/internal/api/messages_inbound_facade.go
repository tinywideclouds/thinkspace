package api

import (
	"errors"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
)

func UnmarshalInbound(data []byte) (*InboundEvent, error) {
	var pbEvent pb.WSEvent
	if err := protoUnmarshaler.Unmarshal(data, &pbEvent); err != nil {
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
		var strategy flows.FlowStrategy
		switch payload.SelectStrategy.StrategyId {
		case pb.DelegationStrategy_DELEGATION_STRATEGY_SKIP:
			strategy = flows.StrategySkip
		case pb.DelegationStrategy_DELEGATION_STRATEGY_MANUAL:
			strategy = flows.StrategyManual
		case pb.DelegationStrategy_DELEGATION_STRATEGY_REVIEW:
			strategy = flows.StrategyReview
		case pb.DelegationStrategy_DELEGATION_STRATEGY_REFINE:
			strategy = flows.StrategyRefine
		default:
			strategy = flows.StrategyManual
		}
		domainEvent.SelectStrategy = &SelectStrategyPayload{StrategyID: strategy}
	case *pb.WSEvent_ReviewDecision:
		domainEvent.Type = "review_decision"
		domainEvent.ReviewDecision = &ReviewDecisionPayload{
			Branch:   payload.ReviewDecision.Branch,
			Accepted: payload.ReviewDecision.Accepted,
		}
	case *pb.WSEvent_ApproveContext:
		domainEvent.Type = "approve_context"

		var indices []int
		for _, idx := range payload.ApproveContext.RetainedHistoryIndices {
			indices = append(indices, int(idx))
		}

		domainEvent.ApproveContext = &ApproveContextPayload{
			Approved:               payload.ApproveContext.Approved,
			RetainedFilePaths:      payload.ApproveContext.RetainedFilePaths,
			RetainedHistoryIndices: indices,
		}
	case *pb.WSEvent_TransitionDecision:
		domainEvent.Type = "transition_decision"
		domainEvent.TransitionDecision = &TransitionDecisionPayload{
			Approved: payload.TransitionDecision.Approved,
		}
	case *pb.WSEvent_LoadHistory:
		domainEvent.Type = "load_history"
		domainEvent.LoadHistory = &LoadHistoryPayload{
			SpaceID: payload.LoadHistory.SpaceId,
			ChatID:  payload.LoadHistory.ChatId,
			Cursor:  payload.LoadHistory.Cursor,
			Limit:   int(payload.LoadHistory.Limit),
		}
	case *pb.WSEvent_ForkChat:
		domainEvent.Type = "fork_chat"
		domainEvent.ForkChat = &ForkChatPayload{
			SpaceID:       payload.ForkChat.SpaceId,
			ChatID:        payload.ForkChat.ChatId,
			TargetEventID: payload.ForkChat.TargetEventId,
		}
	case *pb.WSEvent_InlineEdit:
		domainEvent.Type = "inline_edit"
		domainEvent.InlineEdit = &InlineEditPayload{
			SpaceID:       payload.InlineEdit.SpaceId,
			ChatID:        payload.InlineEdit.ChatId,
			TargetEventID: payload.InlineEdit.TargetEventId,
			NewText:       payload.InlineEdit.NewText,
		}
	case *pb.WSEvent_SwitchBranch:
		domainEvent.Type = "switch_branch"
		domainEvent.SwitchBranch = &SwitchBranchPayload{
			SpaceID:      payload.SwitchBranch.SpaceId,
			ChatID:       payload.SwitchBranch.ChatId,
			TargetBranch: payload.SwitchBranch.TargetBranch,
		}
	default:
		return nil, errors.New("unsupported or unknown inbound event type")
	}

	return domainEvent, nil
}
