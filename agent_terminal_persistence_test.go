package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/Xioaruan912/PPEELink/terminalcore"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func persistentAgentTest(t *testing.T) *terminalcore.Manager {
	if os.Getenv("PPEELINK_AGENT_SYSTEMD_TESTS") != "1" {
		t.Skip("set PPEELINK_AGENT_SYSTEMD_TESTS=1 for isolated systemd/tmux integration tests")
	}
	t.Helper()
	dir := t.TempDir()
	old := terminalcore.Default
	m := &terminalcore.Manager{Socket: filepath.Join(dir, "sock"), Directory: dir, Unit: fmt.Sprintf("ppeelink-agent-terminal-test-%d.service", time.Now().UnixNano())}
	if err := m.Prepare(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	terminalcore.Default = m
	t.Cleanup(func() { terminalcore.Default = old; exec.Command("systemctl", "stop", m.Unit).Run() })
	return m
}
func TestAgentTerminalPersistentTaskSurvivesWireDisconnect(t *testing.T) {
	m := persistentAgentTest(t)
	ctx := context.Background()
	payload, _ := json.Marshal(map[string]any{"operation": "create", "owner": 2, "name": "task"})
	raw, err := handleAgentOp(ctx, "terminal", payload)
	if err != nil {
		t.Fatalf("persistent sessions unsupported: %v", err)
	}
	var s terminalcore.Session
	if err := json.Unmarshal(raw, &s); err != nil || s.ID == "" {
		t.Fatalf("invalid session: %s %v", raw, err)
	}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		_ = serveAgentTerminal(ctx, c)
	}))
	defer panel.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	send := func(v any) {
		t.Helper()
		if err := conn.WriteJSON(v); err != nil {
			t.Fatal(err)
		}
	}
	send(map[string]any{"type": "term.open", "id": "attach", "sessionId": s.ID, "owner": 2})
	progress := filepath.Join(t.TempDir(), "progress")
	send(map[string]any{"type": "term.input", "id": "attach", "data": base64.StdEncoding.EncodeToString([]byte("while :; do echo tick >> " + progress + "; sleep .1; done\r"))})
	go func() {
		for {
			if _, _, e := conn.ReadMessage(); e != nil {
				return
			}
		}
	}()
	for deadline := time.Now().Add(3 * time.Second); ; {
		b, _ := os.ReadFile(progress)
		if len(b) > 10 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task not started")
		}
		time.Sleep(30 * time.Millisecond)
	}
	conn.Close()
	before, _ := os.ReadFile(progress)
	time.Sleep(300 * time.Millisecond)
	after, _ := os.ReadFile(progress)
	if len(after) <= len(before) {
		t.Fatal("task died after wire disconnect")
	}
	if err := m.Exists(ctx, 2, s.ID); err != nil {
		t.Fatal(err)
	}
}

// This helper runs the production terminal engine under a disposable Agent unit.
func TestAgentTerminalSystemdHelper(t *testing.T) {
	if os.Getenv("PPEELINK_TERMINAL_TEST_HELPER") != "1" {
		return
	}
	terminalcore.Default = &terminalcore.Manager{Socket: os.Getenv("PPEELINK_TEST_SOCKET"), Directory: filepath.Dir(os.Getenv("PPEELINK_TEST_SOCKET")), Unit: os.Getenv("PPEELINK_TEST_TMUX_UNIT")}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	runAgentTerminal(ctx, os.Getenv("PPEELINK_TEST_PANEL"), "disposable-test-token")
}
func TestAgentTerminalSystemdRestartPreservesTask(t *testing.T) {
	m := persistentAgentTest(t)
	type wire struct {
		conn     *websocket.Conn
		messages chan agentTermMessage
	}
	connections := make(chan wire, 8)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var hello agentTermMessage
		if err := conn.ReadJSON(&hello); err != nil {
			return
		}
		incoming := wire{conn: conn, messages: make(chan agentTermMessage, 64)}
		connections <- incoming
		for {
			var msg agentTermMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			select {
			case incoming.messages <- msg:
			default:
			}
		}
	}))
	t.Cleanup(panel.Close)
	agentUnit := fmt.Sprintf("ppeelink-disposable-agent-%d.service", time.Now().UnixNano())
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--quiet", "--collect", "--unit=" + agentUnit, "--property=Type=exec", "--setenv=PPEELINK_TERMINAL_TEST_HELPER=1", "--setenv=PPEELINK_TEST_SOCKET=" + m.Socket, "--setenv=PPEELINK_TEST_TMUX_UNIT=" + m.Unit, "--setenv=PPEELINK_TEST_PANEL=" + panel.URL, exe, "-test.run=^TestAgentTerminalSystemdHelper$"}
	if out, err := exec.Command("systemd-run", args...).CombinedOutput(); err != nil {
		t.Fatalf("helper unit: %v %s", err, out)
	}
	t.Cleanup(func() { exec.Command("systemctl", "stop", agentUnit).Run() })
	receive := func() wire {
		t.Helper()
		select {
		case w := <-connections:
			return w
		case <-time.After(8 * time.Second):
			t.Fatal("Agent did not reconnect")
			return wire{}
		}
	}
	session, err := m.Create(context.Background(), 2, "restart-proof")
	if err != nil {
		t.Fatal(err)
	}
	attach := func(w wire) {
		t.Helper()
		if err := w.conn.WriteJSON(agentTermMessage{Type: "term.open", ID: "attach", SessionID: session.ID, Owner: 2}); err != nil {
			t.Fatal(err)
		}
		for {
			select {
			case msg := <-w.messages:
				if msg.Type == "term.ready" {
					return
				}
			case <-time.After(3 * time.Second):
				t.Fatal("attach not ready")
			}
		}
	}
	w := receive()
	attach(w)
	dir := t.TempDir()
	progress := filepath.Join(dir, "progress")
	pidFile := filepath.Join(dir, "pid")
	command := fmt.Sprintf("echo $$ > %s; while :; do echo tick >> %s; sleep .1; done\r", pidFile, progress)
	if err := w.conn.WriteJSON(agentTermMessage{Type: "term.input", ID: "attach", Data: base64.StdEncoding.EncodeToString([]byte(command))}); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(3 * time.Second); ; {
		b, _ := os.ReadFile(progress)
		if len(b) > 10 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("task did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	pidBefore, _ := os.ReadFile(pidFile)
	assertProgress := func() {
		t.Helper()
		before, _ := os.ReadFile(progress)
		time.Sleep(250 * time.Millisecond)
		after, _ := os.ReadFile(progress)
		if len(after) <= len(before) {
			t.Fatal("task stopped")
		}
		pidAfter, _ := os.ReadFile(pidFile)
		if string(pidAfter) != string(pidBefore) {
			t.Fatal("task was re-executed with a new PID")
		}
	}
	w.conn.WriteJSON(agentTermMessage{Type: "term.close", ID: "attach"})
	assertProgress()
	w.conn.Close()
	w = receive()
	attach(w)
	assertProgress()
	if out, err := exec.Command("systemctl", "restart", agentUnit).CombinedOutput(); err != nil {
		t.Fatalf("restart: %v %s", err, out)
	}
	w = receive()
	attach(w)
	assertProgress()
	if err := m.Terminate(context.Background(), 2, session.ID); err != nil {
		t.Fatal(err)
	}
}

func TestAgentTerminalAttachmentDetachIsNotSessionExit(t *testing.T) {
	m := persistentAgentTest(t)
	session, err := m.Create(context.Background(), 2, "detach")
	if err != nil {
		t.Fatal(err)
	}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		_ = serveAgentTerminal(context.Background(), c)
	}))
	defer panel.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	c.WriteJSON(agentTermMessage{Type: "term.open", ID: "attach", Owner: 2, SessionID: session.ID})
	for {
		var msg agentTermMessage
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type == "term.ready" {
			break
		}
	}
	c.WriteJSON(agentTermMessage{Type: "term.input", ID: "attach", Data: base64.StdEncoding.EncodeToString([]byte("tmux detach-client\r"))})
	for {
		var msg agentTermMessage
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type == "term.exit" {
			t.Fatal("retained session reported as ended")
		}
		if msg.Type == "term.detached" {
			break
		}
	}
	if err := m.Exists(context.Background(), 2, session.ID); err != nil {
		t.Fatal(err)
	}
}
func TestAgentTerminalStalledInputDoesNotBlockOperations(t *testing.T) {
	m := persistentAgentTest(t)
	session, err := m.Create(context.Background(), 2, "stalled-input")
	if err != nil {
		t.Fatal(err)
	}
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if e != nil {
			return
		}
		defer c.Close()
		_ = serveAgentTerminal(context.Background(), c)
	}))
	defer panel.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(4 * time.Second))
	c.WriteJSON(agentTermMessage{Type: "term.open", ID: "attach", Owner: 2, SessionID: session.ID})
	for {
		var msg agentTermMessage
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type == "term.ready" {
			break
		}
	}
	out, err := exec.Command("tmux", "-N", "-S", m.Socket, "display-message", "-p", "-t", "=p2_"+session.ID, "#{pid}").Output()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		t.Fatal(err)
	}

	if err := syscall.Kill(pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGCONT)
	c.SetWriteDeadline(time.Now().Add(2 * time.Second))
	for i := 0; i < 80; i++ {
		if err := c.WriteJSON(agentTermMessage{Type: "term.input", ID: "attach", Data: base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 8192)))}); err != nil {
			t.Fatal(err)
		}
	}
	c.WriteJSON(agentTermMessage{Type: "op", ID: "control", Op: "invalid-control-probe", Payload: json.RawMessage(`{}`)})
	for {
		var msg agentTermMessage
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal("blocked PTY input stalled Agent operations:", err)
		}
		if msg.Type == "op.result" && msg.ID == "control" {
			if msg.Error != "不支持的 agent 操作" {
				t.Fatal(msg.Error)
			}
			break
		}
	}
}
