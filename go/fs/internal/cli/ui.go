package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
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

func (ui *TerminalUI) StreamGraphEvent(address *thinkgraph.ChatAddress, node thinkgraph.Node) {
	// The standard CLI relies on raw text streams for output
	// and does not explicitly render structural graph nodes.
}

func (ui *TerminalUI) ConfirmTransition(targetThinkState string, reasoning string) bool {
	fmt.Printf("\n\n⚠️  [SYSTEM] The agent has requested a transition to the '%s' domain.\n", targetThinkState)
	fmt.Printf("💡 Reasoning: %s\n", reasoning)
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

func (ui *TerminalUI) ChooseNextStep() (flows.FlowStrategy, bool) {
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
		return flows.StrategyReview, false
	case "3":
		fmt.Print("Auto-accept synthesized result without manual review? [y/N]: ")
		autoInput, _ := reader.ReadString('\n')
		autoAccept := strings.TrimSpace(strings.ToLower(autoInput))
		return flows.StrategyRefine, autoAccept == "y" || autoAccept == "yes"
	case "4":
		return flows.StrategySkip, false
	default:
		return flows.StrategyManual, false
	}
}

func (ui *TerminalUI) ReviewCandidate(branch string) (bool, string) {
	fmt.Printf("\n🔍 [REVIEW] Candidate branch ready: %s\n", branch)
	fmt.Print("Accept this candidate into the main timeline? [y/N]: ")

	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))
	accepted := input == "y" || input == "yes"

	fmt.Print("Optional reason/comment: ")
	reasonInput, _ := reader.ReadString('\n')
	reason := strings.TrimSpace(reasonInput)

	return accepted, reason
}

func (ui *TerminalUI) GetChunkChannel() chan<- workspace.NodeChunk {
	// The standard CLI does not currently multiplex concurrent agent text streams to the terminal.
	// It relies on the post-execution diffs and flow summaries.
	return nil
}
