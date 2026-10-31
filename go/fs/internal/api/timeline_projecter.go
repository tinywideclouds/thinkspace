package api

import (
	"encoding/json"
	"time"

	pb "github.com/tinywideclouds.com/thinkspace/api/v1"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

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
			var payload workspace.CandidateProposalPayload
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
			var payload workspace.CandidateResolutionPayload
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
		case flows.EventFlowCompleted:
			var payload flows.FlowCompletedPayload
			if err := json.Unmarshal(n.Data, &payload); err != nil {
				continue
			}

			var agents []*pb.AgentRecordPayload
			for _, a := range payload.Agents {
				agents = append(agents, &pb.AgentRecordPayload{
					AgentId:           a.AgentID,
					Passed:            a.Passed,
					Instruction:       a.Instruction,
					RawPayload:        a.RawPayload,
					VerificationTrace: a.VerificationTrace,
					StateDelta:        a.StateDelta,
				})
			}

			ev.Payload = &pb.GraphEvent_FlowCompleted{
				FlowCompleted: &pb.FlowCompleted{
					FlowId:  payload.FlowID,
					TaskId:  payload.TaskID,
					Task:    payload.Task,
					Summary: payload.Summary,
					Agents:  agents,
				},
			}
		default:
			// thinkgraph.EventDigest, flows.EventFlowStarted, flows.EventAgentSpawned, EventFileUpdate
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
