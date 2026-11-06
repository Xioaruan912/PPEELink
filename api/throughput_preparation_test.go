package api

import (
	"encoding/json"
	"ppeelink/models"
	"testing"
	"time"
)

func TestThroughputSourcesDistinguishCurrentAgentPreparationFromOldAgent(t *testing.T) {
	throughputDB(t)
	setupAgentRelease(t)
	models.DB.Model(&models.ProxyServer{}).Where("id = ?", 1).Update("agent_version", currentAgentReleaseVersion())
	models.DB.Model(&models.ProxyServer{}).Where("id = ?", 2).Update("agent_version", "1.16")
	w := throughputReq(AgentThroughputCapabilities, "POST", "/", `{"available":false,"authSupported":false,"preparing":true,"error":"正在自动准备测速工具…"}`, "token1")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = throughputReq(NetworkThroughputSources, "GET", "/", "", "")
	var result struct {
		Data []struct {
			ID            uint `json:"id"`
			AgentOutdated bool `json:"agentOutdated"`
			Preparing     bool `json:"preparing"`
			Available     bool `json:"available"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 3 || result.Data[0].AgentOutdated || !result.Data[0].Preparing || result.Data[0].Available || !result.Data[1].AgentOutdated || result.Data[2].AgentOutdated {
		t.Fatalf("inaccurate readiness: %s", w.Body.String())
	}
	models.DB.Model(&models.ThroughputCapability{}).Where("server_id = ?", 1).Updates(map[string]any{"preparing": false, "available": true, "auth_supported": true, "seen_at": time.Now()})
	w = throughputReq(NetworkThroughputSources, "GET", "/", "", "")
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Data[0].Preparing || !result.Data[0].Available {
		t.Fatalf("readiness not cleared: %s", w.Body.String())
	}
}
