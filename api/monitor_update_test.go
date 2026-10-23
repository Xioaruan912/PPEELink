package api

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"ppeelink/models"
	"testing"
)

func TestDisabledLegacyMonitorKeepsManagementButSkipsMetrics(t *testing.T) {
	setupUnifiedAgentDB(t)
	monitor := models.Monitor{Name: "monitor", TokenHash: models.HashMonitorToken("secret")}
	models.DB.Create(&monitor)
	models.DB.Model(&monitor).Update("enabled", false)
	r := gin.New()
	r.GET("/config", MonitorConfig)
	r.POST("/report", MonitorReport)
	for _, path := range []string{"/config", "/report"} {
		method := "GET"
		if path == "/report" {
			method = "POST"
		}
		request := httptest.NewRequest(method, path, bytes.NewBufferString(`{"version":"legacy"}`))
		request.Header.Set("Authorization", "Monitor secret")
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		r.ServeHTTP(response, request)
		if response.Code != 200 {
			t.Fatalf("disabled monitor lost management: %s", response.Body.String())
		}
	}
	var count int64
	models.DB.Model(&models.MonitorMetric{}).Count(&count)
	if count != 0 {
		t.Fatal("disabled monitor still stored metrics")
	}
	models.DB.First(&monitor, monitor.ID)
	if monitor.LastSeenAt == nil {
		t.Fatal("heartbeat lost")
	}
}

func TestMonitorOwnedAgentUninstallKeepsCredentialUntilAcknowledged(t *testing.T) {
	setupUnifiedAgentDB(t)
	monitor := models.Monitor{Name: "monitor", TokenHash: models.HashMonitorToken("secret")}
	models.DB.Create(&monitor)
	server, err := monitorAgent("secret")
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/", MonitorDelete)
	body, _ := json.Marshal(map[string]any{"id": monitor.ID})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatalf("delete rejected: %s", response.Body.String())
	}
	if response = agentIdentityRequest(AgentSpec, "secret", nil); response.Code != 200 {
		t.Fatal("uninstall credential revoked prematurely")
	}
	models.DB.First(server, server.ID)
	if !server.Removing {
		t.Fatal("uninstall not scheduled")
	}
	if response = agentIdentityRequest(AgentReport, "secret", map[string]any{"uninstalled": true}); response.Code != 200 {
		t.Fatalf("uninstall acknowledgment failed: %s", response.Body.String())
	}
	var count int64
	models.DB.Model(&models.Monitor{}).Where("id = ?", monitor.ID).Count(&count)
	if count != 0 {
		t.Fatal("monitor not purged after acknowledgement")
	}
	models.DB.Model(&models.ProxyServer{}).Where("id = ?", server.ID).Count(&count)
	if count != 0 {
		t.Fatal("agent not purged after acknowledgement")
	}
}

func TestMonitorUpgradeQueuesOnDisabledUnifiedAgent(t *testing.T) {
	setupUnifiedAgentDB(t)
	monitor := models.Monitor{Name: "monitor", TokenHash: models.HashMonitorToken("secret")}
	models.DB.Create(&monitor)
	server, err := monitorAgent("secret")
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.POST("/", MonitorUpdateAgent)
	body, _ := json.Marshal(map[string]any{"id": monitor.ID})
	req := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	r.ServeHTTP(response, req)
	if response.Code != 200 {
		t.Fatalf("upgrade rejected: %s", response.Body.String())
	}
	models.DB.First(server, server.ID)
	if server.Enabled || !server.AgentUpdateRequested {
		t.Fatalf("upgrade changed proxy enabled or lost request: %+v", server)
	}
}
