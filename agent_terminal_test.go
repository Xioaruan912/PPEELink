package main

import (
	"context"
	"encoding/base64"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAgentTerminalCancellationClosesIdleConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = serveAgentTerminal(ctx, conn)
		close(done)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	cancel()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("idle websocket ignored cancellation")
		conn.Close()
		<-done
	}
}

func TestAgentTerminalReportsActualExitCode(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = serveAgentTerminal(ctx, conn)
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	if err := conn.WriteJSON(agentTermMessage{Type: "term.open", ID: "exit-test"}); err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteJSON(agentTermMessage{Type: "term.input", ID: "exit-test", Data: base64.StdEncoding.EncodeToString([]byte("exit 7\n"))}); err != nil {
		t.Fatal(err)
	}
	for {
		var msg agentTermMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type == "term.exit" {
			if msg.Code != 7 {
				t.Fatalf("exit status=%d, want 7", msg.Code)
			}
			break
		}
	}
}
