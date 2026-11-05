package api

import (
	"encoding/json"
	"time"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// mapFlowStatus exhaustively guarantees type safety across the architectural boundary,
// preventing silent drift between the domain enums and transport protobufs.
func mapFlowStatus(domainStatus flows.FlowStatus) pb.FlowStatus {
	switch domainStatus {
	case flows.FlowStatusRunning:
		return pb.FlowStatus_FLOW_STATUS_RUNNING
	case flows.FlowStatusCompleted:
		return pb.FlowStatus_FLOW_STATUS_COMPLETED
	case flows.FlowStatusFailed:
		return pb.FlowStatus_FLOW_STATUS_FAILED
	default:
		return pb.FlowStatus_FLOW_STATUS_UNSPECIFIED
	}
}

// ProjectTimeline iterates over raw generic graph nodes, extracts the strongly typed domain payloads,
// and maps them natively into Protobuf envelopes to maintain strict transport type safety.
func ProjectTimeline(nodes []thinkgraph.Node) []*pb.GraphEvent {
	var pbEvents []*pb.GraphEvent

	for _, n := range nodes {
		ev := &pb.GraphEvent{
			Id:         n.ID.String(),
			Timestamp:  n.Timestamp.Format(time.RFC3339),
			Tags:       n.Tags,
			Tombstoned: n.Tombstoned,
			BranchId:   n.BranchID,
			Metadata:   n.Metadata,
		}

		switch n.Type {
		case thinkgraph.EventPrompt:
			ev.Payload = &pb.GraphEvent_Message{
				Message: &pb.Message{
					Type: pb.MessageType_PROMPT,
					Text: n.Content,
				},
			}
		case thinkgraph.EventModel:
			ev.Payload = &pb.GraphEvent_Message{
				Message: &pb.Message{
					Type: pb.MessageType_MODEL,
					Text: n.Content,
				},
			}
		case workspace.EventCandidateProposal:
			var payload workspace.CandidateProposal
			if err := json.Unmarshal(n.Data, &payload); err != nil {
				continue
			}

			var state map[string]string
			diffSnippet := ""
			if len(n.MaterializedState) > 0 {
				if err := json.Unmarshal(n.MaterializedState, &state); err == nil {
					diffSnippet = state["diff"]
				}
			}

			ev.Payload = &pb.GraphEvent_CandidateProposal{
				CandidateProposal: &pb.CandidateProposal{
					CandidateId: payload.CandidateID,
					Description: payload.Description,
					Files:       payload.Files,
					DiffSnippet: diffSnippet,
				},
			}
		case workspace.EventCandidateResolution:
			var payload workspace.CandidateResolution
			if err := json.Unmarshal(n.Data, &payload); err != nil {
				continue
			}
			ev.Payload = &pb.GraphEvent_CandidateResolution{
				CandidateResolution: &pb.CandidateResolution{
					CandidateId: payload.CandidateID,
					Accepted:    payload.Accepted,
					Reason:      payload.Reason,
				},
			}
		case flows.EventFlowLifecycle:
			var config flows.FlowData
			if err := json.Unmarshal(n.Data, &config); err != nil {
				continue
			}

			var state flows.FlowExecutionState
			if len(n.MaterializedState) > 0 {
				_ = json.Unmarshal(n.MaterializedState, &state)
			}

			var agents []*pb.AgentRecordPayload
			for _, a := range state.Agents {
				agents = append(agents, &pb.AgentRecordPayload{
					AgentId:           a.AgentID,
					Passed:            a.Passed,
					Instruction:       a.Instruction,
					RawPayload:        a.RawPayload,
					VerificationTrace: a.VerificationTrace,
					StateDelta:        a.StateDelta,
				})
			}

			ev.Payload = &pb.GraphEvent_FlowExecution{
				FlowExecution: &pb.FlowExecution{
					FlowId:  config.FlowID,
					TaskId:  config.TaskID,
					Task:    config.TaskPrompt,
					Status:  mapFlowStatus(state.Status),
					Summary: state.Summary,
					Error:   state.Error,
					Agents:  agents,
				},
			}
		default:
			ev.Payload = &pb.GraphEvent_Message{
				Message: &pb.Message{
					Type: pb.MessageType_SYSTEM,
					Text: n.Content,
				},
			}
		}

		pbEvents = append(pbEvents, ev)
	}

	return pbEvents
}
