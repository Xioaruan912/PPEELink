package main

import (
	"context"
	"github.com/Xioaruan912/PPEELink/terminalcore"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"testing"
)

// Used only by the panel's cross-process integration suite, never included in releases.
func TestAgentTerminalTransportHelper(t *testing.T) {
	if os.Getenv("PPEELINK_AGENT_TRANSPORT_HELPER") != "1" {
		return
	}
	socket := os.Getenv("PPEELINK_TEST_SOCKET")
	terminalcore.Default = &terminalcore.Manager{Socket: socket, Directory: filepath.Dir(socket), Unit: os.Getenv("PPEELINK_TEST_TMUX_UNIT")}
	if err := terminalcore.Default.Prepare(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runAgentTerminal(ctx, os.Getenv("PPEELINK_TEST_PANEL"), os.Getenv("PPEELINK_TEST_TOKEN"))
}
