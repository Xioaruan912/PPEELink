package api

import (
	"testing"

	"ppeelink/models"
	"ppeelink/xraycore"
)

func TestAgentUpdateSpecAndAcknowledgement(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyAccount{}, &models.ProxyTraffic{}, &models.Monitor{}, &models.MonitorMetric{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, PublicHost: "example.com", Enabled: true, AgentUpdateRequested: true}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	spec, err := BuildServerSpec(server)
	if err != nil {
		t.Fatal(err)
	}
	if !spec.AgentUpdate || spec.AgentHash == "" {
		t.Fatalf("spec missing update fields: %#v", spec)
	}
	if err := ApplyServerReport(server, xraycore.Report{AgentHash: xraycore.ExecutableHash()}); err != nil {
		t.Fatal(err)
	}
	models.DB.First(&server, server.ID)
	if server.AgentUpdateRequested {
		t.Fatal("matching agent hash must clear update request")
	}
	if server.AgentRevision != 1 {
		t.Fatalf("successful update must increment revision, got %d", server.AgentRevision)
	}
	if err := ApplyServerReport(server, xraycore.Report{AgentHash: xraycore.ExecutableHash()}); err != nil {
		t.Fatal(err)
	}
	models.DB.First(&server, server.ID)
	if server.AgentRevision != 1 {
		t.Fatalf("ordinary reports must not increment revision, got %d", server.AgentRevision)
	}
}
