package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
	"github.com/tinywideclouds.com/thinkspace/internal/session/signals"
	"github.com/tinywideclouds.com/thinkspace/internal/thinkgraph"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TerminalUI struct{}

func NewTerminalUI() *TerminalUI {
	return &TerminalUI{}
}

func (ui *TerminalUI) OnTextChunk(chunk workspace.NodeChunk) {
	fmt.Print(chunk.Text)
}

func (ui *TerminalUI) StreamGraphEvent(node thinkgraph.Node) {
	// The standard CLI relies on raw text streams for output
	// and does not explicitly render structural graph nodes.
}

func (ui *TerminalUI) ConfirmTransition(req signals.SignalTransition) bool {
	fmt.Printf("\n\n⚠️  [SYSTEM] The agent has requested a transition to the '%s' domain.\n", req.TargetThinkState)
	fmt.Printf("💡 Reasoning: %s\n", req.Reasoning)
	fmt.Print("Allow transition? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "" || input == "y" || input == "yes"
}

func (ui *TerminalUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
	// For the CLI, we currently bypass manual context pruning and auto-approve.
	return session.ContextApprovalDecision{
		Approved: true,
	}
}

func (ui *TerminalUI) ChooseNextStep() flows.FlowStrategy {
	fmt.Println("\n\n[FLOW] Sub-agents have completed their tasks. How would you like to proceed?")
	fmt.Println("  1. Manual (I will review the branches myself)")
	fmt.Println("  2. Review (Manager will summarize the approaches)")
	fmt.Println("  3. Refine (Manager will synthesize the best candidate)")
	fmt.Println("  4. Skip   (Discard all candidates and continue)")
	fmt.Print("\nSelect strategy [1-4] (default 1): ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "2":
		return flows.StrategyReview
	case "3":
		return flows.StrategyRefine
	case "4":
		return flows.StrategySkip
	default:
		return flows.StrategyManual
	}
}

func (ui *TerminalUI) ReviewCandidate(branch string) bool {
	fmt.Printf("\n🔍 [REVIEW] Candidate branch ready: %s\n", branch)
	fmt.Print("Accept this candidate into the main timeline? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}

func (ui *TerminalUI) GetChunkChannel() chan<- workspace.NodeChunk {
	// The standard CLI does not currently multiplex concurrent agent text streams to the terminal.
	// It relies on the post-execution diffs and flow summaries.
	return nil
}
