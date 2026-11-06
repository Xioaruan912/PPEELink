package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"ppeelink/api"
	"ppeelink/middlewares"
	"ppeelink/models"
	"strings"
	"testing"
	"time"
)

func TestAgentTerminalHTTPAuthorizationAndRecovery(t *testing.T) {
	if os.Getenv("PPEELINK_AGENT_TEST_BINARY") == "" {
		t.Skip("standalone Agent integration artifact not provided")
	}
	manager := persistentPanelTerminal(t)
	oldDB, oldSecret := models.DB, middlewares.Secret
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "panel.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	models.DB = db
	middlewares.Secret = []byte("isolated-terminal-integration-test-secret")
	t.Cleanup(func() { models.DB = oldDB; middlewares.Secret = oldSecret; sql, _ := db.DB(); sql.Close() })
	if err := db.AutoMigrate(&models.User{}, &models.ProxyServer{}, &models.Monitor{}, &models.APIToken{}); err != nil {
		t.Fatal(err)
	}
	users := []models.User{{Username: "admin-one", Role: models.RoleAdmin}, {Username: "admin-two", Role: models.RoleAdmin}, {Username: "ordinary", Role: models.RoleUser}}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	tokens := make([]string, len(users))
	for i, u := range users {
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, middlewares.JwtClaims{Username: u.Username, StandardClaims: jwt.StandardClaims{ExpiresAt: time.Now().Add(time.Hour).Unix()}}).SignedString(middlewares.Secret)
		if err != nil {
			t.Fatal(err)
		}
		tokens[i] = "Bearer " + token
	}
	server := models.ProxyServer{Name: "isolated", Kind: models.ServerKindAgent, AgentToken: models.HashAgentToken("test-agent-token")}
	db.Create(&server)
	monitor := models.Monitor{Name: "isolated", ProxyServerID: &server.ID}
	db.Create(&monitor)
	router := gin.New()
	router.Use(middlewares.AuthorToken)
	router.GET("/api/v1/agent/ws", api.AgentWebSocket)
	router.GET("/api/v1/monitors/ssh/sessions", api.MonitorTerminalSessions)
	router.POST("/api/v1/monitors/ssh/sessions", api.MonitorTerminalSessions)
	router.DELETE("/api/v1/monitors/ssh/sessions", api.MonitorTerminalSessions)
	router.POST("/api/v1/monitors/ssh/ticket", api.MonitorSSHTicket)
	router.GET("/api/v1/monitors/ssh/ws", api.MonitorAgentTerminal)
	panel := httptest.NewServer(router)
	t.Cleanup(panel.Close)
	testBinary := os.Getenv("PPEELINK_AGENT_TEST_BINARY")
	if testBinary == "" {
		t.Skip("set PPEELINK_AGENT_TEST_BINARY to the agent branch test executable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	child := exec.CommandContext(ctx, testBinary, "-test.run=^TestAgentTerminalTransportHelper$")
	child.Env = append(os.Environ(), "PPEELINK_AGENT_TRANSPORT_HELPER=1", "PPEELINK_TEST_SOCKET="+manager.Socket, "PPEELINK_TEST_TMUX_UNIT="+manager.Unit, "PPEELINK_TEST_PANEL="+panel.URL, "PPEELINK_TEST_TOKEN=test-agent-token")
	if err := child.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = child.Wait() })
	endpoint := fmt.Sprintf("/api/v1/monitors/ssh/sessions?id=%d", monitor.ID)
	request := func(method, path string, body any, token string) (int, json.RawMessage) {
		t.Helper()
		var b io.Reader
		if body != nil {
			j, _ := json.Marshal(body)
			b = bytes.NewReader(j)
		}
		req, _ := http.NewRequest(method, panel.URL+path, b)
		req.Header.Set("Authorization", token)
		req.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		raw, _ := io.ReadAll(res.Body)
		var result struct {
			Data json.RawMessage `json:"data"`
		}
		json.Unmarshal(raw, &result)
		return res.StatusCode, result.Data
	}
	for deadline := time.Now().Add(4 * time.Second); ; {
		_, raw := request("GET", endpoint, nil, tokens[0])
		var state struct {
			Ready bool `json:"ready"`
		}
		json.Unmarshal(raw, &state)
		if state.Ready {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("terminal not ready: %s", raw)
		}
		time.Sleep(30 * time.Millisecond)
	}
	if code, _ := request("GET", endpoint, nil, tokens[2]); code != 403 {
		t.Fatalf("ordinary user got %d", code)
	}
	code, raw := request("POST", endpoint, map[string]string{"name": "HTTP 长任务"}, tokens[0])
	if code != 200 {
		t.Fatalf("create status %d: %s", code, raw)
	}
	var session struct {
		ID string `json:"id"`
	}
	json.Unmarshal(raw, &session)
	if code, _ := request("POST", "/api/v1/monitors/ssh/ticket", map[string]any{"id": monitor.ID, "sessionId": session.ID}, tokens[1]); code == 200 {
		t.Fatal("cross-user ticket accepted")
	}
	ticket := func() string {
		code, raw := request("POST", "/api/v1/monitors/ssh/ticket", map[string]any{"id": monitor.ID, "sessionId": session.ID}, tokens[0])
		if code != 200 {
			t.Fatalf("ticket %d %s", code, raw)
		}
		var result struct {
			Ticket string `json:"ticket"`
		}
		json.Unmarshal(raw, &result)
		return result.Ticket
	}
	token := ticket()
	url := "ws" + strings.TrimPrefix(panel.URL, "http") + "/api/v1/monitors/ssh/ws?ticket=" + token
	browser, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if replay, response, err := websocket.DefaultDialer.Dial(url, nil); err == nil {
		replay.Close()
		t.Fatal("ticket replay accepted")
	} else if response.StatusCode != 401 {
		t.Fatal("wrong replay response")
	}
	waitReady := func(c *websocket.Conn) {
		t.Helper()
		c.SetReadDeadline(time.Now().Add(3 * time.Second))
		for {
			var msg struct {
				Type string `json:"type"`
			}
			if err := c.ReadJSON(&msg); err != nil {
				t.Fatal(err)
			}
			if msg.Type == "ready" {
				return
			}
		}
	}
	waitReady(browser)
	browser.Close()
	next, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(panel.URL, "http")+"/api/v1/monitors/ssh/ws?ticket="+ticket(), nil)
	if err != nil {
		t.Fatal(err)
	}
	waitReady(next)
	next.Close()
	if code, _ := request("DELETE", endpoint+"&sessionId="+session.ID, nil, tokens[0]); code != 200 {
		t.Fatalf("terminate %d", code)
	}
	if code, _ := request("POST", "/api/v1/monitors/ssh/ticket", map[string]any{"id": monitor.ID, "sessionId": session.ID}, tokens[0]); code != 410 {
		t.Fatalf("ended session got %d", code)
	}
}

type terminalFixture struct{ Socket, Directory, Unit string }

func persistentPanelTerminal(t *testing.T) *terminalFixture {
	t.Helper()
	dir := t.TempDir()
	manager := &terminalFixture{Socket: filepath.Join(dir, "sock"), Directory: dir, Unit: fmt.Sprintf("ppeelink-panel-agent-integration-%d.service", time.Now().UnixNano())}
	t.Cleanup(func() { exec.Command("systemctl", "stop", manager.Unit).Run() })
	return manager
}
