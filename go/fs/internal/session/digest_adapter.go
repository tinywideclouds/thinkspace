package session

import (
	"context"

	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

// DigestAdapter bridges the CreateDigestTool to the DigestManager.
type DigestAdapter struct {
	digestManager    *DigestManager
	workspaceService *workspace.Service
	workerModel      string
	workerPrompt     string
}

func NewDigestAdapter(
	digestManager *DigestManager,
	workspaceService *workspace.Service,
	workerModel string,
	workerPrompt string,
) *DigestAdapter {
	return &DigestAdapter{
		digestManager:    digestManager,
		workspaceService: workspaceService,
		workerModel:      workerModel,
		workerPrompt:     workerPrompt,
	}
}

func (d *DigestAdapter) CommitDigest(ctx context.Context, address *thinkgraph.ChatAddress, topicName string, tags []string, instruction string, contextData string) error {
	return d.digestManager.CreateManualDigest(ctx, d.workspaceService, address, topicName, tags, instruction, contextData, d.workerModel, d.workerPrompt)
}
