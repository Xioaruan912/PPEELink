package terminalcore

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testManager(t *testing.T) *Manager {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux unavailable")
	}
	dir := t.TempDir()
	m := &Manager{Binary: os.Getenv("PPEELINK_TEST_TMUX"), Socket: filepath.Join(dir, "tmux.sock"), Unit: fmt.Sprintf("ppeelink-terminal-test-%d-%d.service", os.Getpid(), time.Now().UnixNano()), Directory: dir}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := m.Prepare(ctx, false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { exec.Command("systemctl", "stop", m.Unit).Run() })
	return m
}

func TestPersistentSessionSurvivesDetachAndAgentServiceStop(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()
	s, err := m.Create(ctx, 7, "long task")
	if err != nil {
		t.Fatal(err)
	}
	p, err := m.Attach(ctx, 7, s.ID, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	go io.Copy(io.Discard, p.File)
	progress := filepath.Join(t.TempDir(), "progress")
	fmt.Fprintf(p.File, "while :; do echo tick >> %s; sleep .1; done\r", progress)
	deadline := time.Now().Add(3 * time.Second)
	for {
		b, _ := os.ReadFile(progress)
		if len(b) > 10 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	p.Close()
	before, _ := os.ReadFile(progress)
	time.Sleep(250 * time.Millisecond)
	after, _ := os.ReadFile(progress)
	if len(after) <= len(before) {
		t.Fatal("long task died on detach")
	}
	// The tmux server must belong to its own unit, not the caller/agent service.
	pid, err := m.command(ctx, "display-message", "-p", "-t", m.target(7, s.ID), "#{pid}")
	if err != nil {
		t.Fatal(err)
	}
	group, err := os.ReadFile("/proc/" + strings.TrimSpace(string(pid)) + "/cgroup")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(group), m.Unit) {
		t.Fatalf("tmux escaped its independent unit: %s", group)
	}
	again, err := m.Attach(ctx, 7, s.ID, 100, 30)
	if err != nil {
		t.Fatal(err)
	}
	again.Close()
	if err := m.Terminate(ctx, 8, s.ID); err == nil {
		t.Fatal("cross-owner terminate accepted")
	}
	if err := m.Terminate(ctx, 7, s.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Attach(ctx, 7, s.ID, 100, 30); err == nil {
		t.Fatal("ended session was recreated")
	}
}
func TestPersistentSessionsAreOwnerScopedAndValidateInput(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()
	a, err := m.Create(ctx, 1, "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Create(ctx, 2, "second"); err != nil {
		t.Fatal(err)
	}
	list, err := m.List(ctx, 1)
	if err != nil || len(list) != 1 || list[0].ID != a.ID {
		t.Fatalf("owner leak: %v %v", list, err)
	}
	if _, err := m.Attach(ctx, 2, a.ID, 100, 30); err == nil {
		t.Fatal("cross-owner attach accepted")
	}
	if _, err := m.Create(ctx, 0, "bad"); err == nil {
		t.Fatal("zero owner accepted")
	}
	if _, err := m.Create(ctx, 1, "bad\nname"); err == nil {
		t.Fatal("control characters accepted")
	}
	if _, err := m.Attach(ctx, 1, ";kill-server", 100, 30); err == nil {
		t.Fatal("invalid ID accepted")
	}
}
func TestPersistentMissingTmuxDoesNotFallBackToShell(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m := &Manager{Socket: filepath.Join(t.TempDir(), "sock"), Unit: "ppeelink-test.service"}
	if err := m.Prepare(context.Background(), false); err == nil {
		t.Fatal("missing dependency must fail")
	}
}

func TestPersistentSessionUsesPanelSessionControls(t *testing.T) {
	m := testManager(t)
	out, err := m.command(context.Background(), "show-options", "-g", "-v", "prefix")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "None" {
		t.Fatal("tmux native session-switch prefix must be disabled; the panel owns session selection")
	}
}

func TestPersistentServiceLossNeverReplaysOldSessions(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()
	s, err := m.Create(ctx, 3, "lost-service")
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("systemctl", "stop", m.Unit).CombinedOutput(); err != nil {
		t.Fatalf("stop: %v %s", err, out)
	}
	if err := m.Prepare(ctx, false); err != nil {
		t.Fatal(err)
	}
	sessions, err := m.List(ctx, 3)
	if err != nil || len(sessions) != 0 {
		t.Fatalf("sessions resurrected: %v %v", sessions, err)
	}
	if _, err := m.Attach(ctx, 3, s.ID, 100, 30); err == nil {
		t.Fatal("lost session recreated on attach")
	}
}
func TestPersistentServerOwnershipRequiresExpectedUnit(t *testing.T) {
	m := testManager(t)
	ctx := context.Background()
	s, err := m.Create(ctx, 2, "unit-check")
	if err != nil {
		t.Fatal(err)
	}
	other := &Manager{Binary: m.Binary, Socket: m.Socket, Unit: "ppeelink-wrong-owner.service", Directory: m.Directory}
	if err := other.serverOwned(ctx, m.target(2, s.ID)); err == nil {
		t.Fatal("foreign cgroup accepted as protected")
	}
}
