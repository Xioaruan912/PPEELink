package api

import (
	"testing"

	"ppeelink/models"
	"ppeelink/monitorcore"
	"ppeelink/xraycore"
)

func TestApplyAgentMonitorCreatesLinkedMetric(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyTraffic{}, &models.ProxyAccount{}, &models.Monitor{}, &models.MonitorMetric{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP-01", Kind: models.ServerKindAgent, MonitorEnabled: true}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	report := xraycore.Report{
		Arch:    "amd64",
		Monitor: &monitorcore.Metric{CPUPercent: 12.5, MemTotal: 1024, MemUsed: 512},
	}
	if err := ApplyServerReport(server, report); err != nil {
		t.Fatal(err)
	}
	var monitor models.Monitor
	if err := models.DB.Where("proxy_server_id = ?", server.ID).First(&monitor).Error; err != nil {
		t.Fatal("linked monitor not created:", err)
	}
	var metric models.MonitorMetric
	if err := models.DB.Where("monitor_id = ?", monitor.ID).First(&metric).Error; err != nil {
		t.Fatal("metric not stored:", err)
	}
	if metric.CPUPercent != 12.5 || metric.MemUsed != 512 {
		t.Fatalf("unexpected metric: %+v", metric)
	}
}

func TestBuildServerSpecIncludesMonitor(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyAccount{}, &models.User{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP-01", Kind: models.ServerKindAgent, MonitorEnabled: true}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	spec, err := BuildServerSpec(server)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Monitor == nil || !spec.Monitor.Enabled {
		t.Fatalf("monitor spec missing: %+v", spec.Monitor)
	}
}
