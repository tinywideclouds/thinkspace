package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"text/template"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// SubAgentRunRequest encapsulates all inputs needed to execute a single sub-agent
// inside an isolated candidate sandbox.
type SubAgentRunRequest struct {
	ChatSession  *thinkgraph.ChatAddress
	FlowAddress  *thinkgraph.ChatAddress
	Space        spaces.ThinkState
	FlowContext  flows.FlowContext
	Task         flows.SubAgentTask
	AgentIndex   int
	ParentNodeID uuid.UUID
	MaxAttempts  int
	UI           UserInterface
	Emitter      telemetry.Emitter
}

// SubAgentResult reports the outcome, candidate branch ID, verification trace,
// and leaf graph nodes produced by a SubAgentRunner execution.
type SubAgentResult struct {
	CandidateID  string
	Err          error
	Record       flows.AgentRecord
	PatchApplied bool
	Passed       bool
	Trace        string
	Diff         string
	GenNodeID    uuid.UUID
	LeafEventIDs []uuid.UUID
}

// CandidateExecutionRequest runs a single-pass candidate generation + patch + verify + deliver
// inside an isolated sandbox without appending task/resolution nodes (used by SynthesizeEngine).
type CandidateExecutionRequest struct {
	ChatSession    *thinkgraph.ChatAddress
	FlowAddress    *thinkgraph.ChatAddress
	Space          spaces.ThinkState
	CandidateID    string
	BaseAgentRules string
	ContextDigest  string
	Instruction    string
	ParentNodeID   uuid.UUID
	DraftMessage   string
	UI             tools.UI
	Emitter        telemetry.Emitter
}

// SubAgentRunner executes an isolated sub-agent task inside a CandidateSandbox,
// handling context assembly, patch application, verification retries, and candidate delivery.
type SubAgentRunner struct {
	logger          *slog.Logger
	service         *workspace.Service
	turnExecutor    *TurnExecutor
	bundleAssembler *BundleAssembler
	broadcaster     *GraphBroadcaster
}

func NewSubAgentRunner(
	logger *slog.Logger,
	service *workspace.Service,
	turnExecutor *TurnExecutor,
	bundleAssembler *BundleAssembler,
	broadcaster *GraphBroadcaster,
) *SubAgentRunner {
	if broadcaster == nil {
		broadcaster = NewGraphBroadcasterFromService(logger, service)
	}
	if bundleAssembler == nil && service != nil {
		bundleAssembler = NewBundleAssembler(logger, service.Graph(), thinkgraph.NewContextProjecter(), nil, service.WorkspaceRoot())
	}
	return &SubAgentRunner{
		logger:          logger,
		service:         service,
		turnExecutor:    turnExecutor,
		bundleAssembler: bundleAssembler,
		broadcaster:     broadcaster,
	}
}

// ExecuteCandidate executes a single generation, patch, and verification pass into a named candidate sandbox.
// If the patch applies, the candidate branch is always drafted and delivered for review even if verification fails.
func (r *SubAgentRunner) ExecuteCandidate(ctx context.Context, req CandidateExecutionRequest) SubAgentResult {
	sandbox, err := r.service.SpawnSandbox(ctx, req.ChatSession.ChatID, req.CandidateID)
	if err != nil {
		return SubAgentResult{CandidateID: req.CandidateID, Err: err}
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil && r.logger != nil {
			r.logger.WarnContext(ctx, "failed to tear down candidate sandbox", "error", tdErr)
		}
	}()

	bundle := r.bundleAssembler.AssembleWorkerBundle(
		ctx,
		req.ChatSession.ProjectID,
		req.Space,
		req.BaseAgentRules,
		req.ContextDigest,
		req.Instruction,
		nil,
	)

	mux := &streamMultiplexer{ui: req.UI}
	responseID := uuid.NewV7()

	_, err = r.turnExecutor.Execute(ctx, req.FlowAddress, responseID, req.ParentNodeID, false, req.Space, bundle, tools.NewRegistry(), mux, req.Emitter)
	if err != nil {
		return SubAgentResult{CandidateID: req.CandidateID, Err: err, GenNodeID: responseID}
	}

	lastPayload := mux.buffer.String()
	patchApplied := true
	verified := true
	verifyTrace := "Verification passed."

	if patcher := req.Space.Patcher(); patcher != nil {
		if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
			patchApplied = false
			verified = false
			verifyTrace = fmt.Sprintf("Failed to apply patch: %v", applyErr)
			if r.logger != nil {
				r.logger.WarnContext(ctx, "failed to apply candidate patch", "error", applyErr)
			}
		}
	}

	if patchApplied {
		if verifier := req.Space.Verifier(); verifier != nil {
			if verifyErr := verifier.Verify(ctx, sandbox); verifyErr != nil {
				verified = false
				verifyTrace = fmt.Sprintf("Verification failed: %v", verifyErr)
			}
		}

		draftMsg := req.DraftMessage
		if draftMsg == "" {
			draftMsg = "Applied candidate draft"
		}
		if err := sandbox.ApplyDraft(ctx, draftMsg); err != nil && r.logger != nil {
			r.logger.ErrorContext(ctx, "failed to apply draft for candidate", "error", err)
		}
		if err := sandbox.DeliverForReview(ctx); err != nil && r.logger != nil {
			r.logger.ErrorContext(ctx, "failed to deliver candidate for review", "error", err)
		}
	}

	var diff string
	if patchApplied {
		if d, diffErr := r.service.ReadCandidate(ctx, req.ChatSession, req.CandidateID); diffErr != nil {
			if r.logger != nil {
				r.logger.WarnContext(ctx, "failed reading candidate diff", "error", diffErr)
			}
		} else {
			diff = d
		}
	}

	return SubAgentResult{
		CandidateID:  req.CandidateID,
		PatchApplied: patchApplied,
		Passed:       verified,
		Trace:        verifyTrace,
		Diff:         diff,
		GenNodeID:    responseID,
		Record: flows.AgentRecord{
			AgentID:           req.CandidateID,
			Passed:            verified,
			Instruction:       req.Instruction,
			RawPayload:        lastPayload,
			VerificationTrace: verifyTrace,
			StateDelta:        diff,
		},
	}
}

func (r *SubAgentRunner) Run(ctx context.Context, req SubAgentRunRequest) SubAgentResult {
	if req.Emitter != nil {
		req.Emitter.Emit(req.Task.AgentID, telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_status",
			Timestamp: time.Now(),
			Message:   "Agent spawning and parsing context",
			Payload: flows.AgentTelemetry{
				FlowID:      req.FlowContext.FlowID,
				AgentID:     req.Task.AgentID,
				AgentIndex:  req.AgentIndex,
				Instruction: req.Task.Instruction,
			},
		})
	}

	candidateID := fmt.Sprintf("%s-%s-%s", req.ChatSession.ChatID, req.FlowContext.FlowID, req.Task.AgentID)
	sandbox, err := r.service.SpawnSandbox(ctx, req.ChatSession.ChatID, candidateID)
	if err != nil {
		return SubAgentResult{CandidateID: candidateID, Err: err}
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil && r.logger != nil {
			r.logger.WarnContext(ctx, "failed to teardown sandbox", "error", tdErr)
		}
	}()

	workbench := workspace.NewWorkbench()
	workbenchItems, err := workbench.Build(ctx, sandbox, req.Task.TargetFiles)
	if err != nil {
		return SubAgentResult{CandidateID: candidateID, Err: err}
	}

	bundle := r.bundleAssembler.AssembleWorkerBundle(
		ctx,
		req.ChatSession.ProjectID,
		req.Space,
		req.FlowContext.BaseAgentRules,
		req.Task.ContextDigest,
		req.Task.Instruction,
		workbenchItems,
	)

	taskEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, req.Task.Instruction)
	taskEvent.Metadata["agent_id"] = req.Task.AgentID
	if taskEvent.Edges == nil {
		taskEvent.Edges = make(map[string]string)
	}
	taskEvent.Edges["parent_id"] = req.ParentNodeID.String()

	if dataBytes, marshalErr := json.Marshal(req.Task); marshalErr != nil {
		if r.logger != nil {
			r.logger.WarnContext(ctx, "failed to marshal sub-agent task data", "error", marshalErr)
		}
	} else {
		taskEvent.Data = dataBytes
	}

	if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, taskEvent, req.UI); appendErr != nil && r.logger != nil {
		r.logger.ErrorContext(ctx, "failed to append agent task event", "error", appendErr)
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}

	passed, finalTrace, rawPayload, genNodeID := r.attemptLoop(ctx, req, sandbox, bundle, taskEvent.ID, maxAttempts)

	record := flows.AgentRecord{
		AgentID:           req.Task.AgentID,
		Passed:            passed,
		Instruction:       req.Task.Instruction,
		RawPayload:        rawPayload,
		VerificationTrace: finalTrace,
	}

	if err := sandbox.DeliverForReview(ctx); err != nil {
		return SubAgentResult{CandidateID: candidateID, Err: err}
	}

	diff, err := r.service.ReadCandidate(ctx, req.ChatSession, candidateID)
	if err != nil {
		if r.logger != nil {
			r.logger.WarnContext(ctx, "failed to read candidate diff", "error", err)
		}
	} else {
		record.StateDelta = diff
	}

	if req.Emitter != nil {
		req.Emitter.Emit("manager", telemetry.Event{
			ID:        uuid.NewV7(),
			Type:      "flow_complete",
			Timestamp: time.Now(),
			Message:   "Flow pipeline completed",
			Payload: flows.AgentTelemetry{
				FlowID:      req.FlowContext.FlowID,
				AgentID:     req.Task.AgentID,
				AgentIndex:  req.AgentIndex,
				CandidateID: candidateID,
				Passed:      passed,
			},
		})
	}

	statusStr := "REJECTED"
	if passed {
		statusStr = "ACCEPTED"
	}
	resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Proposal %s. Reason: %s", statusStr, finalTrace))
	resEvent.Metadata["agent_id"] = req.Task.AgentID
	if resEvent.Edges == nil {
		resEvent.Edges = make(map[string]string)
	}
	resEvent.Edges["parent_id"] = genNodeID.String()

	resPayload := workspace.CandidateResolution{
		CandidateID: candidateID,
		Accepted:    passed,
		Reason:      finalTrace,
	}
	if resBytes, marshalErr := json.Marshal(resPayload); marshalErr != nil {
		if r.logger != nil {
			r.logger.WarnContext(ctx, "failed to marshal resolution payload", "error", marshalErr)
		}
	} else {
		resEvent.Data = resBytes
	}

	if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, resEvent, req.UI); appendErr != nil && r.logger != nil {
		r.logger.ErrorContext(ctx, "failed to append agent candidate resolution event", "error", appendErr)
	}

	return SubAgentResult{
		CandidateID:  fmt.Sprintf("candidate/%s", candidateID),
		PatchApplied: true,
		Passed:       passed,
		Trace:        finalTrace,
		Diff:         diff,
		GenNodeID:    genNodeID,
		Record:       record,
		LeafEventIDs: []uuid.UUID{resEvent.ID},
	}
}

func (r *SubAgentRunner) attemptLoop(
	ctx context.Context,
	req SubAgentRunRequest,
	sandbox workspace.CandidateSandbox,
	bundle cognition.ThinkBundle,
	taskEventID uuid.UUID,
	maxAttempts int,
) (bool, string, string, uuid.UUID) {
	var finalTrace string
	var lastPayload string
	var genNodeID uuid.UUID

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && req.Task.RetryPrompt != "" {
			tmpl, err := template.New("retry").Parse(req.Task.RetryPrompt)
			if err == nil {
				var buf bytes.Buffer
				if execErr := tmpl.Execute(&buf, map[string]string{"ErrorTrace": finalTrace}); execErr != nil {
					if r.logger != nil {
						r.logger.WarnContext(ctx, "failed to execute retry template", "error", execErr)
					}
				} else {
					bundle.Trajectory.Catalyst.Content = buf.String()

					retry := thinkgraph.NewEvent(thinkgraph.EventPrompt, buf.String())
					retry.Metadata["agent_id"] = req.Task.AgentID
					if retry.Edges == nil {
						retry.Edges = make(map[string]string)
					}
					retry.Edges["parent_id"] = genNodeID.String()

					if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, retry, req.UI); appendErr != nil && r.logger != nil {
						r.logger.ErrorContext(ctx, "failed to append retry prompt event", "error", appendErr)
					}

					taskEventID = retry.ID
				}
			}
		}

		if req.Emitter != nil {
			req.Emitter.Emit(req.Task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Executing instructions...",
				Payload: flows.AgentTelemetry{
					FlowID:     req.FlowContext.FlowID,
					AgentID:    req.Task.AgentID,
					AgentIndex: req.AgentIndex,
					Attempt:    attempt,
				},
			})
		}

		mux := &streamMultiplexer{ui: req.UI}
		taskResponseID := uuid.NewV7()

		_, err := r.turnExecutor.Execute(ctx, req.FlowAddress, taskResponseID, taskEventID, false, req.Space, bundle, tools.NewRegistry(), mux, req.Emitter)
		if err != nil {
			return false, fmt.Errorf("Execution Error: %v", err).Error(), "", uuid.Nil()
		}
		genNodeID = taskResponseID

		mut := thinkgraph.MetadataMutation{Metadata: map[string]string{"agent_id": req.Task.AgentID}}
		mut.TargetID = taskResponseID
		mut.Type = thinkgraph.MutationMetadata
		mut.Timestamp = time.Now().UTC()
		_ = r.broadcaster.MutateAndStream(ctx, req.FlowAddress, mut, taskResponseID, req.UI)

		lastPayload = mux.buffer.String()

		if patcher := req.Space.Patcher(); patcher != nil {
			if applyErr := patcher.Apply(ctx, sandbox, lastPayload); applyErr != nil {
				finalTrace = fmt.Sprintf("Patch Error: %v", applyErr)
				continue
			}
		}

		if applyErr := sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt)); applyErr != nil && r.logger != nil {
			r.logger.ErrorContext(ctx, "failed to apply sandbox draft", "error", applyErr)
		}

		if req.Emitter != nil {
			req.Emitter.Emit(req.Task.AgentID, telemetry.Event{
				ID:        uuid.NewV7(),
				Type:      "flow_status",
				Timestamp: time.Now(),
				Message:   "Running tests and verification...",
				Payload: flows.AgentTelemetry{
					FlowID:     req.FlowContext.FlowID,
					AgentID:    req.Task.AgentID,
					AgentIndex: req.AgentIndex,
					Attempt:    attempt,
				},
			})
		}

		if verifier := req.Space.Verifier(); verifier != nil {
			err = verifier.Verify(ctx, sandbox)
			if err == nil {
				return true, "", lastPayload, genNodeID
			}
			finalTrace = err.Error()

			if req.Emitter != nil {
				req.Emitter.Emit(req.Task.AgentID, telemetry.Event{
					ID:        uuid.NewV7(),
					Type:      "flow_error",
					Timestamp: time.Now(),
					Message:   "Verification failed",
					Payload: flows.AgentTelemetry{
						FlowID:     req.FlowContext.FlowID,
						AgentID:    req.Task.AgentID,
						AgentIndex: req.AgentIndex,
						Attempt:    attempt,
						Trace:      finalTrace,
					},
				})
			}
		} else {
			return true, "", lastPayload, genNodeID
		}
	}

	return false, finalTrace, lastPayload, genNodeID
}
