package api

import (
	"encoding/json"
	"testing"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/xraycore"
)

// Removing the topology inventory must not disable the shared agent control path.
// Disabled servers must still authenticate so the agent can receive a stop spec
// and continue reporting its state (and final counters).
func TestNetworkRemovalPreservesDisabledAgentControl(t *testing.T) {
	server, _ := setupProxyDB(t)
	if err := models.DB.Model(&server).Updates(map[string]any{"enabled": false, "monitor_enabled": true}).Error; err != nil {
		t.Fatal(err)
	}
	rec := jsonRequest(t, AgentSpec, nil, func(c *gin.Context) {
		c.Request.Header.Set("Authorization", "Agent agent-secret")
	})
	if rec.Code != 200 {
		t.Fatalf("disabled agent cannot fetch control state: %d %s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data xraycore.Spec `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Enabled || payload.Data.Monitor == nil || !payload.Data.Monitor.Enabled {
		t.Fatalf("proxy stop must not disable independent monitoring: %+v", payload.Data)
	}
	if len(payload.Data.Inbounds) != 1 || payload.Data.APIPort == 0 {
		t.Fatalf("shared xray connection configuration lost: %+v", payload.Data)
	}
}
