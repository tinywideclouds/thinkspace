package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
)

const CreateDigest ToolName = "create_digest"

const (
	TopicName         = "topic_name"
	DigestInstruction = "instruction"
	Tags              = "tags"
)

type CreateDigestRequest struct {
	TopicName   string   `json:"topic_name"`
	Instruction string   `json:"instruction"`
	Tags        []string `json:"tags"`
}

type DigestCommitter interface {
	CommitDigest(ctx context.Context, chatObj *thinkgraph.ChatAddress, topicName string, tags []string, instruction string, contextData string) error
}

type CreateDigestTool struct {
	committer DigestCommitter
}

func NewCreateDigestTool(committer DigestCommitter) *CreateDigestTool {
	return &CreateDigestTool{committer: committer}
}

func (t *CreateDigestTool) Name() ToolName {
	return CreateDigest
}

func (t *CreateDigestTool) DefaultDescription() string {
	return "Compress a concluded discussion or major architectural decision into durable memory."
}

func (t *CreateDigestTool) Execute(ctx context.Context, executionContext ExecutionContext) (bool, error) {
	var request CreateDigestRequest
	argumentBytes, err := json.Marshal(executionContext.RawArguments)
	if err != nil {
		return false, fmt.Errorf("marshaling create_digest arguments: %w", err)
	}
	if err := json.Unmarshal(argumentBytes, &request); err != nil {
		return false, fmt.Errorf("invalid create_digest schema: %w", err)
	}

	if request.TopicName == "" || request.Instruction == "" {
		if executionContext.UI != nil {
			executionContext.UI.OnTextChunk("\n\n⚠️ **System Intercept:** Manager provided incomplete digest parameters. Aborting.\n")
		}
		return true, fmt.Errorf("manager provided empty topic or instruction")
	}

	var sb strings.Builder
	if executionContext.Bundle != nil {
		for _, ev := range executionContext.Bundle.Trajectory.WorkingMemory {
			fmt.Fprintf(&sb, "[%s]: %s\n", ev.Role, ev.Content)
		}
	}

	err = t.committer.CommitDigest(
		ctx,
		executionContext.Address,
		request.TopicName,
		request.Tags,
		request.Instruction,
		sb.String(),
	)
	if err != nil {
		return false, fmt.Errorf("manual digest creation failed: %w", err)
	}

	return true, nil
}
