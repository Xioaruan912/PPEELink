package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/xraycore"
)

func setupUnifiedAgentDB(t *testing.T) {
	t.Helper()
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyTraffic{}, &models.ProxyAccount{}, &models.Monitor{}, &models.MonitorMetric{}); err != nil {
		t.Fatal(err)
	}
}

func agentIdentityRequest(handler gin.HandlerFunc, token string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	recorder := httptest.NewRecorder()
	// Use an engine to initialize Gin's response writer normally.
	r := gin.New()
	r.POST("/", handler)
	req := httptest.NewRequest("POST", "/", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Agent "+token)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(recorder, req)
	return recorder
}

func TestMonitorTokenUsesUnifiedAgentLifecycle(t *testing.T) {
	setupUnifiedAgentDB(t)
	monitor := models.Monitor{Name: "monitor-only", TokenHash: models.HashMonitorToken("monitor-secret"), Enabled: true}
	if err := models.DB.Create(&monitor).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		response := agentIdentityRequest(AgentSpec, "monitor-secret", nil)
		if response.Code != 200 {
			t.Fatalf("spec: %s", response.Body.String())
		}
		var envelope struct {
			Data xraycore.Spec `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Data.Enabled || envelope.Data.Monitor == nil || !envelope.Data.Monitor.Enabled {
			t.Fatalf("monitor must start proxy-disabled: %+v", envelope.Data)
		}
	}
	if err := models.DB.First(&monitor, monitor.ID).Error; err != nil {
		t.Fatal(err)
	}
	if monitor.ProxyServerID == nil {
		t.Fatal("monitor not bound")
	}
	var count int64
	models.DB.Model(&models.ProxyServer{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate agent bindings: %d", count)
	}
	if err := models.DB.Model(&models.ProxyServer{}).Where("id = ?", *monitor.ProxyServerID).Updates(map[string]any{"enabled": false, "agent_update_requested": true}).Error; err != nil {
		t.Fatal(err)
	}
	response := agentIdentityRequest(AgentSpec, "monitor-secret", nil)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"agentUpdate":true`) {
		t.Fatalf("disabled proxy lost update: %s", response.Body.String())
	}
	response = agentIdentityRequest(AgentReport, "monitor-secret", xraycore.Report{AgentVersion: "test-real-version", AgentHash: xraycore.ExecutableHash()})
	if response.Code != 200 {
		t.Fatalf("report: %s", response.Body.String())
	}
	var server models.ProxyServer
	models.DB.First(&server, *monitor.ProxyServerID)
	if server.AgentUpdateRequested || server.AgentVersion != "test-real-version" || server.LastSeenAt == nil {
		t.Fatalf("upgrade/heartbeat not acknowledged: %+v", server)
	}
	// Proxy can be enabled in Web without reinstalling or changing the token.
	models.DB.Model(&server).Update("enabled", true)
	response = agentIdentityRequest(AgentSpec, "monitor-secret", nil)
	if !strings.Contains(response.Body.String(), `"enabled":true`) {
		t.Fatalf("proxy enable not delivered: %s", response.Body.String())
	}
	// Disabling metric collection never invalidates management authentication.
	models.DB.Model(&monitor).Update("enabled", false)
	if response = agentIdentityRequest(AgentSpec, "monitor-secret", nil); response.Code != 200 {
		t.Fatalf("monitor disable revoked management: %s", response.Body.String())
	}
	models.DB.Model(&monitor).Update("token_hash", models.HashMonitorToken("replacement"))
	if response = agentIdentityRequest(AgentSpec, "monitor-secret", nil); response.Code != 401 {
		t.Fatal("reset token still authorized")
	}
	if response = agentIdentityRequest(AgentSpec, "replacement", nil); response.Code != 200 {
		t.Fatal("replacement token cannot access binding")
	}
}

func TestUnifiedTokenRotationRevokesBothRoles(t *testing.T) {
	setupUnifiedAgentDB(t)
	monitor := models.Monitor{Name: "monitor", TokenHash: models.HashMonitorToken("monitor-old")}
	models.DB.Create(&monitor)
	server, err := monitorAgent("monitor-old")
	if err != nil {
		t.Fatal(err)
	}
	if err := resetServerAgentToken(server.ID, models.HashAgentToken("agent-new")); err != nil {
		t.Fatal(err)
	}
	if response := agentIdentityRequest(AgentSpec, "monitor-old", nil); response.Code != 401 {
		t.Fatal("server rotation did not revoke old monitor identity")
	}
	if response := agentIdentityRequest(AgentSpec, "agent-new", nil); response.Code != 200 {
		t.Fatal("new agent identity invalid")
	}
	models.DB.First(&monitor, monitor.ID)
	if err := resetMonitorAgentToken(monitor, models.HashMonitorToken("monitor-new")); err != nil {
		t.Fatal(err)
	}
	if response := agentIdentityRequest(AgentSpec, "agent-new", nil); response.Code != 401 {
		t.Fatal("monitor rotation did not revoke old agent identity")
	}
	if response := agentIdentityRequest(AgentSpec, "monitor-new", nil); response.Code != 200 {
		t.Fatal("new monitor identity invalid")
	}
}

func TestMonitorTokenCannotCrossServerBinding(t *testing.T) {
	setupUnifiedAgentDB(t)
	local := models.ProxyServer{Name: "local", Kind: models.ServerKindLocal}
	models.DB.Create(&local)
	monitor := models.Monitor{Name: "bad binding", TokenHash: models.HashMonitorToken("secret"), ProxyServerID: &local.ID}
	models.DB.Create(&monitor)
	if response := agentIdentityRequest(AgentSpec, "secret", nil); response.Code != 401 {
		t.Fatalf("monitor token authorized local server: %s", response.Body.String())
	}
}
