package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
)

func TestAgentCheckJobsAndReport(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProbeResult{}, &models.Node{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken("check-token")}
	models.DB.Create(&server)
	models.DB.Create(&models.Node{Name: "node", Link: "vless://uuid@203.0.113.1:443?type=tcp"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/agent/checks/jobs", nil)
	c.Request.Header.Set("Authorization", "Agent check-token")
	AgentCheckJobs(c)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "203.0.113.1") {
		t.Fatalf("jobs: %d %s", w.Code, w.Body.String())
	}

	body := `{"results":[{"nodeId":1,"kind":"tcp","success":true,"rtt":12,"ipVersion":4}]}`
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/agent/checks/report", strings.NewReader(body))
	c.Request.Header.Set("Authorization", "Agent check-token")
	c.Request.Header.Set("Content-Type", "application/json")
	AgentCheckReport(c)
	if w.Code != 200 {
		t.Fatalf("report: %d %s", w.Code, w.Body.String())
	}
	var result models.ProbeResult
	if err := models.DB.First(&result).Error; err != nil {
		t.Fatal(err)
	}
	if result.AgentServerID != server.ID || !result.Success || result.NodeID != 1 {
		t.Fatalf("result = %#v", result)
	}
}
