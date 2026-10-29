package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
)

func jsonNumber(id uint) string { return fmt.Sprintf("%d", id) }

func agentAPIRequest(t *testing.T, handler gin.HandlerFunc, method, path, body, token string) (*httptest.ResponseRecorder, gin.H) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Authorization", "Agent "+token)
	handler(c)
	var payload gin.H
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	return w, payload
}

func TestAgentTraceDispatchAndReport(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.NetworkTraceRun{}, &models.NetworkTraceView{}, &models.TraceAgentCapability{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken("agent-token")}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	run := models.NetworkTraceRun{Target: "example.com"}
	if err := models.DB.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
	view := models.NetworkTraceView{RunID: run.ID, AgentServerID: server.ID, SourceName: server.Name, Status: "queued"}
	if err := models.DB.Create(&view).Error; err != nil {
		t.Fatal(err)
	}

	w, payload := agentAPIRequest(t, AgentTraceJobs, http.MethodGet, "/api/v1/agent/trace/jobs?tool=traceroute", "", "agent-token")
	if w.Code != 200 {
		t.Fatalf("jobs status %d: %s", w.Code, w.Body.String())
	}
	data, _ := payload["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("jobs = %#v", payload)
	}
	models.DB.First(&view, view.ID)
	if view.Status != "running" {
		t.Fatalf("view status = %s", view.Status)
	}

	body := `{"id":` + jsonNumber(view.ID) + `,"tool":"traceroute","output":" 1  1.1.1.1  2.5 ms"}`
	w, _ = agentAPIRequest(t, AgentTraceReport, http.MethodPost, "/api/v1/agent/trace/report", body, "agent-token")
	if w.Code != 200 {
		t.Fatalf("report status %d: %s", w.Code, w.Body.String())
	}
	models.DB.First(&view, view.ID)
	if view.Status != "completed" || !strings.Contains(view.Output, "1.1.1.1") {
		t.Fatalf("view = %#v", view)
	}
}

func TestAgentTraceRequiresOwnServer(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.NetworkTraceRun{}, &models.NetworkTraceView{}, &models.TraceAgentCapability{}); err != nil {
		t.Fatal(err)
	}
	owner := models.ProxyServer{Name: "A", Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken("a")}
	other := models.ProxyServer{Name: "B", Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken("b")}
	models.DB.Create(&owner)
	models.DB.Create(&other)
	run := models.NetworkTraceRun{Target: "example.com"}
	models.DB.Create(&run)
	view := models.NetworkTraceView{RunID: run.ID, AgentServerID: owner.ID, Status: "running"}
	models.DB.Create(&view)
	body := `{"id":` + jsonNumber(view.ID) + `,"tool":"traceroute","output":"1 1.1.1.1"}`
	w, _ := agentAPIRequest(t, AgentTraceReport, http.MethodPost, "/api/v1/agent/trace/report", body, "b")
	if w.Code != http.StatusConflict {
		t.Fatalf("other agent must not report: %d %s", w.Code, w.Body.String())
	}
}

func TestAgentTracePrepareOwnershipAndCapabilityUpgrade(t *testing.T) {
	setupTestDB(t)
	models.DB.AutoMigrate(&models.ProxyServer{}, &models.NetworkTraceRun{}, &models.NetworkTraceView{}, &models.TraceAgentCapability{})
	for _, token := range []string{"owner", "other"} {
		models.DB.Create(&models.ProxyServer{Name: token, Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken(token)})
	}
	agentAPIRequest(t, AgentTraceJobs, "GET", "/?version=2&tool=traceroute", "", "owner")
	w := traceRequest(NetworkTracePrepare, "POST", "/", `{"agentIds":[1]}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w, _ = agentAPIRequest(t, AgentTraceJobs, "GET", "/?version=2&tool=traceroute", "", "other")
	if strings.Contains(w.Body.String(), `"action":"prepare"`) {
		t.Fatal("prepare stolen")
	}
	w, payload := agentAPIRequest(t, AgentTraceJobs, "GET", "/?version=2&tool=traceroute", "", "owner")
	data, _ := payload["data"].([]any)
	if len(data) != 1 {
		t.Fatal(w.Body.String())
	}
	job := data[0].(map[string]any)
	nonce, _ := job["prepareId"].(string)
	if job["action"] != "prepare" || len(nonce) != 32 {
		t.Fatal(job)
	}
	body, _ := json.Marshal(map[string]any{"action": "prepare", "prepareId": nonce, "tool": "nexttrace"})
	w, _ = agentAPIRequest(t, AgentTraceReport, "POST", "/", string(body), "other")
	if w.Code != 409 {
		t.Fatal("other agent accepted", w.Body.String())
	}
	w, _ = agentAPIRequest(t, AgentTraceReport, "POST", "/", string(body), "owner")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var cap models.TraceAgentCapability
	models.DB.First(&cap, "server_id = ?", 1)
	if cap.Tool != "nexttrace" || cap.PrepareFinishedAt == nil || cap.PrepareError != "" {
		t.Fatalf("%+v", cap)
	}
	w, _ = agentAPIRequest(t, AgentTraceReport, "POST", "/", string(body), "owner")
	if w.Code != 409 {
		t.Fatal("replay accepted")
	}
}
