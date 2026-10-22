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
