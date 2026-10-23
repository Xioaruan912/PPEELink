package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"ppeelink/models"
	"strings"
	"testing"
	"time"
)

func TestAgentReconnectReplacesConnection(t *testing.T) {
	if os.Getenv("SUBL_RECONNECT_CHILD") != "1" {
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(exe, "-test.run=^TestAgentReconnectReplacesConnection$", "-test.timeout=5s")
		cmd.Env = append(os.Environ(), "SUBL_RECONNECT_CHILD=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("agent reconnect failed: %v\n%s", err, out)
		}
		return
	}
	// A subprocess isolates a possible registry-lock deadlock from other tests.
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "reconnect", Kind: models.ServerKindAgent, AgentToken: models.HashAgentToken("reconnect-token")}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	router.GET("/ws", AgentWebSocket)
	panel := httptest.NewServer(router)
	header := http.Header{"Authorization": []string{"Agent reconnect-token"}}
	url := "ws" + strings.TrimPrefix(panel.URL, "http") + "/ws"
	old, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	first := agentConnForServer(server.ID)
	for deadline := time.Now().Add(time.Second); first == nil && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
		first = agentConnForServer(server.ID)
	}
	if first == nil {
		t.Fatal("initial connection not registered")
	}
	latest, _, err := websocket.DefaultDialer.Dial(url, header)
	if err != nil {
		t.Fatal(err)
	}
	defer latest.Close()
	for deadline := time.Now().Add(time.Second); agentConnForServer(server.ID) == first && time.Now().Before(deadline); {
		time.Sleep(time.Millisecond)
	}
	done := make(chan error, 1)
	go func() {
		_, err := agentRequest(server.ID, "files", map[string]string{"operation": "list", "path": "/tmp"}, time.Second)
		done <- err
	}()
	_ = latest.SetReadDeadline(time.Now().Add(time.Second))
	var msg agentWireMessage
	if err := latest.ReadJSON(&msg); err != nil {
		t.Fatalf("replacement connection unusable: %v", err)
	}
	if msg.Type != "op" {
		t.Fatalf("unexpected command: %+v", msg)
	}
	if err := latest.WriteJSON(agentWireMessage{Type: "op.result", ID: msg.ID, OK: true, Payload: json.RawMessage(`{"items":[]}`)}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("replacement reply was not delivered")
	}
	if agentConnForServer(server.ID) == first {
		t.Fatal("old connection still registered")
	}
}
