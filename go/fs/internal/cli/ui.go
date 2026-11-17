package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
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

func readLine(reader *bufio.Reader) (string, bool) {
	input, err := reader.ReadString('\n')
	if err != nil && (!errors.Is(err, io.EOF) || input == "") {
		return "", false
	}
	return strings.TrimSpace(input), true
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
	input, ok := readLine(reader)
	if !ok {
		return false
	}
	lower := strings.ToLower(input)
	return lower == "" || lower == "y" || lower == "yes"
}

func (ui *TerminalUI) ReviewContext(turnContext session.TurnContext) session.ContextApprovalDecision {
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
	input, ok := readLine(reader)
	if !ok {
		return flows.StrategySkip, false
	}

	switch input {
	case "2":
		return flows.StrategyReview, false
	case "3":
		fmt.Print("Auto-accept synthesized result without manual review? [y/N]: ")
		autoInput, autoOK := readLine(reader)
		if !autoOK {
			return flows.StrategyRefine, false
		}
		lower := strings.ToLower(autoInput)
		return flows.StrategyRefine, lower == "y" || lower == "yes"
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
	input, ok := readLine(reader)
	if !ok {
		return false, ""
	}
	lower := strings.ToLower(input)
	accepted := lower == "y" || lower == "yes"

	fmt.Print("Optional reason/comment: ")
	reason, _ := readLine(reader)

	return accepted, reason
}

func (ui *TerminalUI) GetChunkChannel() chan<- workspace.NodeChunk {
	return nil
}
