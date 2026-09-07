package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/workspace"
)

type TerminalUI struct {
	reader *bufio.Reader
}

func NewTerminalUI() *TerminalUI {
	return &TerminalUI{
		reader: bufio.NewReader(os.Stdin),
	}
}

func (ui *TerminalUI) OnTextChunk(text string) {
	fmt.Print(text)
}

func (ui *TerminalUI) ChooseNextStep() session.DelegationStrategy {
	fmt.Println("\nSelect Next Step:")
	fmt.Println("[1] Manual Review (I will read the generated code)")
	fmt.Println("[2] Assisted Review (Manager evaluates diffs, I decide)")
	fmt.Println("[3] Auto-Refine (Manager evaluates, synthesizes a final branch, I approve)")
	fmt.Println("[0] Skip / Abort (Reject all and continue)")
	fmt.Print("Choice [1]: ")

	input, _ := ui.reader.ReadString('\n')
	input = strings.TrimSpace(input)

	switch input {
	case "0":
		return session.StrategySkip
	case "2":
		return session.StrategyReview
	case "3":
		return session.StrategyRefine
	default:
		return session.StrategyManual
	}
}

func (ui *TerminalUI) ReviewCandidate(branch string) bool {
	fmt.Printf("\n👀 Previewing %s...\n", branch)
	fmt.Println("--------------------------------------------------")
	fmt.Printf("📂 Files have been successfully checked out.\n")
	fmt.Printf("💻 Open your IDE to inspect the code for %s.\n", branch)
	fmt.Println("--------------------------------------------------")

	fmt.Print("Accept candidate? (y/n): ")
	accept, _ := ui.reader.ReadString('\n')
	return strings.ToLower(strings.TrimSpace(accept)) == "y"
}

func (ui *TerminalUI) GetAgentTokenChannel() chan<- workspace.AgentToken {
	return nil
}
