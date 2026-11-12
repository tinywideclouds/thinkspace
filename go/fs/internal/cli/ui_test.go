package cli_test

import (
	"testing"

	"github.com/tinywideclouds.com/thinkspace/internal/cli"
	"github.com/tinywideclouds.com/thinkspace/internal/session"
)

// Ensure at compile-time that TerminalUI implements the required session.UserInterface contract.
var _ session.UserInterface = (*cli.TerminalUI)(nil)

func TestTerminalUI_ImplementsInterface(t *testing.T) {
	// The primary purpose of this test is the compiler check above.
	// We instantiate it here just to ensure it can be safely created.
	ui := cli.NewTerminalUI()
	if ui == nil {
		t.Fatal("Expected TerminalUI instance, got nil")
	}
}
