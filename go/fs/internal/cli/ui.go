package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/tinywideclouds.com/thinkspace/internal/session"
	"github.com/tinywideclouds.com/thinkspace/internal/session/flows"
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

// ReviewContext fulfills the updated session.UserInterface contract, allowing the CLI
// to optionally block and display the JIT context before invoking the Manager LLM.
func (ui *TerminalUI) ReviewContext(ctxData session.TurnContext) bool {
	fmt.Printf("\n🧠 --- CONTEXT PREVIEW ---\n")
	fmt.Printf("Space: %s | Chat: %s\n", ctxData.SpaceID, ctxData.ChatID)

	if len(ctxData.StickyDigests) > 0 {
		fmt.Printf("Sticky Digests: %d\n", len(ctxData.StickyDigests))
	}

	if len(ctxData.FileContexts) > 0 {
		fmt.Printf("Files in Context:\n")
		for _, fc := range ctxData.FileContexts {
			fmt.Printf("  - %s [%s] (~%d tokens)\n", fc.Path, fc.Level, fc.TokenCount)
		}
	}

	fmt.Println("---------------------------")
	fmt.Print("Proceed with LLM execution? (Y/n): ")

	input, _ := ui.reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	// Default to yes unless explicitly aborted
	return input != "n"
}

func (ui *TerminalUI) ChooseNextStep() flows.FlowStrategy {
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
		return flows.StrategySkip
	case "2":
		return flows.StrategyReview
	case "3":
		return flows.StrategyRefine
	default:
		return flows.StrategyManual
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

func (ui *TerminalUI) ConfirmTransition(targetDomain string) bool {
	fmt.Printf("\n⚠️  The AI wants to transition the workspace to the '%s' domain.\n", targetDomain)
	fmt.Print("Allow this transition and scaffold the environment? (Y/n): ")

	input, _ := ui.reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))

	// Default to yes unless explicitly rejected
	return input != "n"
}
