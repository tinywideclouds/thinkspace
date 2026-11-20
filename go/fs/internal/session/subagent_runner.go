package session

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"uuid"

	"github.com/tinywideclouds.com/thinkspace/internal/cognition"
	"github.com/tinywideclouds.com/thinkspace/internal/llm"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/session/tools"
	"github.com/tinywideclouds.com/thinkspace/internal/spaces"
	"github.com/tinywideclouds.com/thinkspace/internal/telemetry"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// SubAgentRunRequest encapsulates all inputs needed to execute a single sub-agent
// inside an isolated candidate sandbox.
type SubAgentRunRequest struct {
	ChatSession          *thinkgraph.ChatAddress
	FlowAddress          *thinkgraph.ChatAddress
	Scope                signals.FlowHandle
	Space                spaces.ThinkState
	FlowContext          flows.FlowContext
	Task                 flows.SubAgentTask
	AgentIndex           int
	ParentNodeID         uuid.UUID
	MaxAttempts          int
	DeferResolutionEvent bool
	PrebuiltSandbox      workspace.CandidateSandbox
	PrebuiltBundle       *cognition.ThinkBundle
	SkipContextReview    bool
	WorkerReviewPolicy   ContextReviewPolicy
	WorkerPolicySettings workspace.ReviewPolicySettings
	UI                   UserInterface
	Emitter              telemetry.Emitter
}

// SubAgentResult reports the outcome, canonical candidate value object, verification trace,
// and leaf graph nodes produced by a SubAgentRunner execution.
type SubAgentResult struct {
	Candidate    flows.Candidate
	CandidateID  string
	Err          error
	Record       flows.AgentRecord
	PatchApplied bool
	Passed       bool
	Trace        string
	Diff         string
	RawPayload   string
	GenNodeID    uuid.UUID
	LeafEventIDs []uuid.UUID
}

// SubAgentRunner executes an isolated sub-agent task inside a CandidateSandbox,
// handling context assembly, response healing, patch application, verification retries,
// subgraph node streaming, and candidate delivery.
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

func extractChangedFilesFromDiff(diff string) []string {
	if strings.TrimSpace(diff) == "" {
		return nil
	}
	seen := make(map[string]bool)
	var paths []string

	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+++ ") {
			continue
		}
		raw := strings.TrimSpace(strings.TrimPrefix(line, "+++ "))
		if raw == "" || raw == "/dev/null" {
			continue
		}
		raw = strings.TrimPrefix(raw, "b/")
		clean := filepath.ToSlash(filepath.Clean(raw))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			continue
		}
		if !seen[clean] {
			seen[clean] = true
			paths = append(paths, clean)
		}
	}
	return paths
}

func (r *SubAgentRunner) captureCandidateFiles(ctx context.Context, sandbox workspace.FileReader, diff string, fallbackTargets []string) map[string]string {
	if sandbox == nil {
		return nil
	}

	paths := extractChangedFilesFromDiff(diff)
	if len(paths) == 0 {
		paths = fallbackTargets
	}
	if len(paths) == 0 {
		return nil
	}

	files := make(map[string]string, len(paths))
	for _, p := range paths {
		data, err := sandbox.ReadFile(ctx, p)
		if err == nil {
			files[p] = string(data)
		}
	}
	if len(files) == 0 {
		return nil
	}
	return files
}

func (r *SubAgentRunner) Run(ctx context.Context, req SubAgentRunRequest) SubAgentResult {
	if r.service == nil {
		return SubAgentResult{Err: fmt.Errorf("workspace service is nil")}
	}
	if req.ChatSession == nil {
		return SubAgentResult{Err: fmt.Errorf("chat session address is nil")}
	}

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

	sandbox := req.PrebuiltSandbox
	if sandbox == nil {
		var err error
		sandbox, err = r.service.SpawnSandbox(ctx, req.ChatSession.ChatID, candidateID)
		if err != nil {
			return SubAgentResult{CandidateID: candidateID, Err: err}
		}
	}
	defer func() {
		if tdErr := sandbox.TearDown(ctx); tdErr != nil && r.logger != nil {
			r.logger.WarnContext(ctx, "failed to teardown sandbox", "error", tdErr)
		}
	}()

	var bundle cognition.ThinkBundle
	if req.PrebuiltBundle != nil {
		bundle = *req.PrebuiltBundle
	} else {
		workbenchItems, err := workspace.ReadFileContext(ctx, sandbox, req.Task.TargetFiles)
		if err != nil {
			return SubAgentResult{CandidateID: candidateID, Err: err}
		}

		bundle = r.bundleAssembler.AssembleWorkerBundle(
			ctx,
			req.ChatSession.ProjectID,
			req.Space,
			req.FlowContext.BaseAgentRules,
			req.Task.ContextDigest,
			req.Task.Instruction,
			workbenchItems,
		)
	}

	if !req.SkipContextReview && req.WorkerPolicySettings.Active && req.UI != nil {
		policy := req.WorkerReviewPolicy
		if policy == nil {
			policy = NewTokenThresholdPolicy()
		}
		reviewID := uuid.NewV7()
		turnContext := ProjectTurnContext(req.ChatSession.ProjectID, req.ChatSession.ChatID, reviewID, ContextStageWorker, &bundle, thinkgraph.AttentionFocus{})
		if policy.ShouldReview(req.WorkerPolicySettings, turnContext) {
			previewTitle := fmt.Sprintf("Sub-Agent Context Preview (%s)", req.Task.AgentID)
			previewMarkdown := FormatBundlePreviewMarkdown(previewTitle, &bundle)
			if previewID, err := r.service.SaveContextPreview(ctx, req.ChatSession, previewMarkdown); err != nil {
				if r.logger != nil {
					r.logger.WarnContext(ctx, "failed saving sub-agent context preview", "agent_id", req.Task.AgentID, "error", err)
				}
			} else {
				turnContext.PreviewID = previewID
			}

			decision := req.UI.ReviewContext(turnContext)
			if !decision.Approved {
				return SubAgentResult{CandidateID: candidateID, Err: fmt.Errorf("sub-agent %s execution aborted by user during context review", req.Task.AgentID)}
			}

			if decision.RetainedFilePaths != nil {
				retainedPaths := make(map[string]bool, len(decision.RetainedFilePaths))
				for _, p := range decision.RetainedFilePaths {
					retainedPaths[p] = true
				}
				var prunedWorkbench []cognition.WorkbenchItem
				for _, item := range bundle.Workbench {
					if retainedPaths[item.Path] {
						prunedWorkbench = append(prunedWorkbench, item)
					}
				}
				bundle.Workbench = prunedWorkbench
			}
		}
	}

	taskEvent := thinkgraph.NewEvent(thinkgraph.EventPrompt, req.Task.Instruction)
	taskEvent.Metadata["agent_id"] = req.Task.AgentID
	if taskEvent.Edges == nil {
		taskEvent.Edges = make(map[string]string)
	}
	taskEvent.Edges["parent_id"] = req.ParentNodeID.String()

	dataBytes, marshalErr := json.Marshal(req.Task)
	if marshalErr != nil {
		return SubAgentResult{CandidateID: candidateID, Err: fmt.Errorf("marshaling sub-agent task data: %w", marshalErr)}
	}
	taskEvent.Data = dataBytes

	if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, taskEvent, req.UI); appendErr != nil {
		return SubAgentResult{CandidateID: candidateID, Err: fmt.Errorf("appending agent task event: %w", appendErr)}
	}

	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}

	patchApplied, passed, finalTrace, rawPayload, genNodeID, loopErr := r.attemptLoop(ctx, req, sandbox, bundle, taskEvent.ID, maxAttempts)
	if loopErr != nil {
		return SubAgentResult{CandidateID: candidateID, Err: loopErr}
	}

	record := flows.AgentRecord{
		AgentID:           req.Task.AgentID,
		Passed:            passed,
		Instruction:       req.Task.Instruction,
		RawPayload:        rawPayload,
		VerificationTrace: finalTrace,
	}

	var diff string
	var files map[string]string
	if patchApplied {
		if err := sandbox.DeliverForReview(ctx); err != nil {
			return SubAgentResult{CandidateID: candidateID, Err: err}
		}

		readDiff, readErr := r.service.ReadCandidate(ctx, req.ChatSession, candidateID)
		if readErr != nil {
			if r.logger != nil {
				r.logger.WarnContext(ctx, "failed to read candidate diff", "error", readErr)
			}
		} else {
			diff = readDiff
			record.StateDelta = diff
		}

		files = r.captureCandidateFiles(ctx, sandbox, diff, req.Task.TargetFiles)
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

	leafNodeID := genNodeID
	if leafNodeID == uuid.Nil() {
		leafNodeID = taskEvent.ID
	}

	if !req.DeferResolutionEvent {
		statusStr := "REJECTED"
		if passed {
			statusStr = "ACCEPTED"
		}
		resEvent := thinkgraph.NewEvent(workspace.EventCandidateResolution, fmt.Sprintf("Proposal %s. Reason: %s", statusStr, finalTrace))
		resEvent.Metadata["agent_id"] = req.Task.AgentID
		if resEvent.Edges == nil {
			resEvent.Edges = make(map[string]string)
		}
		resEvent.Edges["parent_id"] = leafNodeID.String()

		resPayload := workspace.CandidateResolution{
			CandidateID: candidateID,
			Accepted:    passed,
			Reason:      finalTrace,
		}
		resBytes, marshalErr := json.Marshal(resPayload)
		if marshalErr != nil {
			return SubAgentResult{CandidateID: candidateID, Err: fmt.Errorf("marshaling resolution payload: %w", marshalErr)}
		}
		resEvent.Data = resBytes

		if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, resEvent, req.UI); appendErr != nil {
			return SubAgentResult{CandidateID: candidateID, Err: fmt.Errorf("appending candidate resolution event: %w", appendErr)}
		}
		leafNodeID = resEvent.ID
	}

	cand := flows.Candidate{
		ID:          candidateID,
		AgentID:     req.Task.AgentID,
		LeafNodeID:  leafNodeID,
		Passed:      passed,
		Trace:       finalTrace,
		Diff:        diff,
		Files:       files,
		RawPayload:  rawPayload,
		Instruction: req.Task.Instruction,
	}

	if req.Scope != nil {
		req.Scope.RegisterCandidate(cand)
	}

	return SubAgentResult{
		Candidate:    cand,
		CandidateID:  candidateID,
		PatchApplied: patchApplied,
		Passed:       passed,
		Trace:        finalTrace,
		Diff:         diff,
		RawPayload:   rawPayload,
		GenNodeID:    genNodeID,
		Record:       record,
		LeafEventIDs: []uuid.UUID{leafNodeID},
	}
}

func (r *SubAgentRunner) attemptLoop(
	ctx context.Context,
	req SubAgentRunRequest,
	sandbox workspace.CandidateSandbox,
	bundle cognition.ThinkBundle,
	taskEventID uuid.UUID,
	maxAttempts int,
) (bool, bool, string, string, uuid.UUID, error) {
	var finalTrace string
	var lastPayload string
	var genNodeID uuid.UUID
	anyPatchApplied := false

	workerModel := strings.TrimSpace(req.Space.Config().Roles.Worker.Model)
	if workerModel == "" {
		workerModel = req.Space.Config().Roles.Manager.Model
	}
	healer := llm.GetHealerForModel(workerModel)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && req.Task.RetryPrompt != "" {
			tmpl, err := template.New("retry").Parse(req.Task.RetryPrompt)
			if err != nil {
				if r.logger != nil {
					r.logger.WarnContext(ctx, "failed to parse retry template", "error", err)
				}
			} else {
				var buf bytes.Buffer
				if execErr := tmpl.Execute(&buf, map[string]string{
					"ErrorTrace":     finalTrace,
					"BaseAgentRules": req.FlowContext.BaseAgentRules,
				}); execErr != nil {
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
					if genNodeID != uuid.Nil() {
						retry.Edges["parent_id"] = genNodeID.String()
					} else {
						retry.Edges["parent_id"] = taskEventID.String()
					}

					if appendErr := r.broadcaster.AppendAndStream(ctx, req.FlowAddress, retry, req.UI); appendErr != nil {
						return false, false, "", "", uuid.Nil(), fmt.Errorf("appending retry prompt event: %w", appendErr)
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

		mux := &streamMultiplexer{ui: req.UI, streamTextChunks: false}
		taskResponseID := uuid.NewV7()

		_, err := r.turnExecutor.Execute(ctx, req.FlowAddress, taskResponseID, taskEventID, workerModel, false, req.Space, bundle, tools.NewRegistry(), mux, req.Emitter)
		if err != nil {
			return anyPatchApplied, false, fmt.Errorf("Execution Error: %v", err).Error(), "", uuid.Nil(), nil
		}
		genNodeID = taskResponseID

		mut := thinkgraph.MetadataMutation{Metadata: map[string]string{"agent_id": req.Task.AgentID}}
		mut.TargetID = taskResponseID
		mut.Type = thinkgraph.MutationMetadata
		mut.Timestamp = time.Now().UTC()
		if err := r.broadcaster.MutateAndStream(ctx, req.FlowAddress, mut, taskResponseID, req.UI); err != nil {
			return anyPatchApplied, false, "", "", uuid.Nil(), fmt.Errorf("mutating agent metadata on response node: %w", err)
		}

		lastPayload = mux.buffer.String()

		if patcher := req.Space.Patcher(); patcher != nil {
			healedPayload := lastPayload
			if healer != nil {
				if healed, modified := healer.Heal(lastPayload); modified {
					if r.logger != nil {
						r.logger.InfoContext(ctx, "healed worker LLM response before patching", "agent_id", req.Task.AgentID, "model", workerModel, "attempt", attempt)
					}
					healedPayload = healed
				}
			}

			if applyErr := patcher.Apply(ctx, sandbox, healedPayload); applyErr != nil {
				finalTrace = fmt.Sprintf("Patch Error: %v", applyErr)
				if req.Emitter != nil {
					req.Emitter.Emit(req.Task.AgentID, telemetry.Event{
						ID:        uuid.NewV7(),
						Type:      "flow_error",
						Timestamp: time.Now(),
						Message:   "Patch application failed",
						Payload: flows.AgentTelemetry{
							FlowID:     req.FlowContext.FlowID,
							AgentID:    req.Task.AgentID,
							AgentIndex: req.AgentIndex,
							Attempt:    attempt,
							Trace:      finalTrace,
						},
					})
				}
				continue
			}
		}

		anyPatchApplied = true

		if applyErr := sandbox.ApplyDraft(ctx, fmt.Sprintf("attempt %d", attempt)); applyErr != nil {
			return anyPatchApplied, false, "", "", uuid.Nil(), fmt.Errorf("applying sandbox draft on attempt %d: %w", attempt, applyErr)
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
				return true, true, "", lastPayload, genNodeID, nil
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
			return true, true, "", lastPayload, genNodeID, nil
		}
	}

	return anyPatchApplied, false, finalTrace, lastPayload, genNodeID, nil
}
