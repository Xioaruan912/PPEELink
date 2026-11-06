package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentCLIHelper(t *testing.T) {
	if os.Getenv("PPEELINK_AGENT_CLI_HELPER") != "1" {
		return
	}
	for i, v := range os.Args {
		if v == "--" {
			os.Args = append([]string{"ppeelink-agent"}, os.Args[i+1:]...)
			main()
			return
		}
	}
}
func TestAgentVersionDoesNotInitializePanelState(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestAgentCLIHelper$", "--", "--version")
	cmd.Env = append(os.Environ(), "PPEELINK_AGENT_CLI_HELPER=1")
	dir := t.TempDir()
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), version) {
		t.Fatalf("%s %v", out, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "db")); !os.IsNotExist(err) {
		t.Fatal("Agent initialized panel database")
	}
}
