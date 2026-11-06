package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"ppeelink/models"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTerminalSlowBrowserDoesNotBlockAgentReplies(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	ready := make(chan *agentConn, 1)
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := newAgentConn(987654, conn)
		agentConnMu.Lock()
		agentConns[ac.serverID] = ac
		agentConnMu.Unlock()
		if !ac.openTerminal("slow", 100, 30, func([]byte) { <-block }, func() {}) {
			return
		}
		ready <- ac
		go ac.writeLoop()
		ac.readLoop()
	}))
	defer panel.Close()
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ac := <-ready
	done := make(chan error, 1)
	go func() { _, err := agentRequest(ac.serverID, "files", nil, 500*time.Millisecond); done <- err }()
	var msg agentWireMessage
	for {
		if err := client.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type == "op" {
			break
		}
	}
	client.WriteJSON(agentWireMessage{Type: "term.output", ID: "slow", Data: encodeBase64([]byte("blocked"))})
	client.WriteJSON(agentWireMessage{Type: "op.result", ID: msg.ID, OK: true, Payload: json.RawMessage(`{}`)})
	if err := <-done; err != nil {
		t.Fatal("slow terminal blocked unrelated reply:", err)
	}
}
func TestTerminalTicketRejectsRevokedOwnerAndIsSingleUse(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.User{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "terminal-admin", Password: "irrelevant", Role: models.RoleAdmin}
	models.DB.Create(&user)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("authType", "jwt")
	c.Set("userId", user.ID)
	owner, err := terminalOwner(c)
	if err != nil || owner <= 0 {
		t.Fatalf("owner %d %v", owner, err)
	}
	monitorSSHTickets.Store("bound", monitorSSHTicket{MonitorID: 1, Owner: owner, SessionID: strings.Repeat("a", 24), ExpiresAt: time.Now().Add(time.Minute)})
	ticket, ok := takeTerminalTicket("bound")
	if !ok || ticket.Owner != owner {
		t.Fatal("bound ticket rejected")
	}
	if _, ok := takeTerminalTicket("bound"); ok {
		t.Fatal("ticket replay accepted")
	}
	models.DB.Model(&user).Update("disabled", true)
	monitorSSHTickets.Store("revoked", monitorSSHTicket{MonitorID: 1, Owner: owner, SessionID: strings.Repeat("a", 24), ExpiresAt: time.Now().Add(time.Minute)})
	if _, ok := takeTerminalTicket("revoked"); ok {
		t.Fatal("disabled owner retained access")
	}
}

func TestTerminalDetachDeliveredWhenDataQueueIsFull(t *testing.T) {
	ready := make(chan *agentConn, 1)
	start := make(chan struct{})
	var startOnce sync.Once
	panel := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		ac := newAgentConn(987655, conn)
		ready <- ac
		<-start
		go ac.writeLoop()
		ac.readLoop()
	}))
	defer panel.Close()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	defer startOnce.Do(func() { close(start) })
	ac := <-ready
	for i := 0; i < cap(ac.send); i++ {
		if !ac.sendMessage(agentWireMessage{Type: "term.input", ID: "busy"}) {
			t.Fatal("queue filled early")
		}
	}
	if !ac.openTerminal("owned", 100, 30, func([]byte) {}, func() {}) {
		t.Fatal("control open cannot get past full data queue")
	}
	ac.closeTerminal("owned")
	startOnce.Do(func() { close(start) })
	c.SetReadDeadline(time.Now().Add(time.Second))
	for _, want := range []string{"term.open", "term.close"} {
		var msg agentWireMessage
		if err := c.ReadJSON(&msg); err != nil {
			t.Fatal(err)
		}
		if msg.Type != want || msg.ID != "owned" {
			t.Fatalf("control ordering lost: %+v want %s", msg, want)
		}
	}
}
