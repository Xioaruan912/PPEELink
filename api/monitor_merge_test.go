package api

import (
	"testing"

	"ppeelink/models"
)

func TestMergeDuplicateMonitorsKeepsStandaloneAndMovesMetrics(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.Monitor{}, &models.MonitorMetric{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "NoSLA_JP", Kind: models.ServerKindAgent, PublicHost: "216.23.84.93", Enabled: true, MonitorEnabled: true}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	standalone := models.Monitor{Name: "NoSLa_JP", SSHHost: "216.23.84.93", SSHPort: 22, SSHUser: "root", SSHPasswordEnc: "encrypted"}
	if err := models.DB.Create(&standalone).Error; err != nil {
		t.Fatal(err)
	}
	linked := models.Monitor{Name: "NoSLA_JP", ProxyServerID: &server.ID, Enabled: true}
	if err := models.DB.Create(&linked).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Create(&models.MonitorMetric{MonitorID: linked.ID, CPUPercent: 12.5}).Error; err != nil {
		t.Fatal(err)
	}
	if err := MergeDuplicateMonitors(); err != nil {
		t.Fatal(err)
	}
	var monitors []models.Monitor
	models.DB.Order("id").Find(&monitors)
	if len(monitors) != 1 || monitors[0].ID != standalone.ID {
		t.Fatalf("monitors = %#v", monitors)
	}
	if monitors[0].ProxyServerID == nil || *monitors[0].ProxyServerID != server.ID || monitors[0].SSHHost != "216.23.84.93" {
		t.Fatalf("standalone monitor not adopted: %#v", monitors[0])
	}
	var metrics []models.MonitorMetric
	models.DB.Find(&metrics)
	if len(metrics) != 1 || metrics[0].MonitorID != standalone.ID {
		t.Fatalf("metrics not moved: %#v", metrics)
	}
}

func TestMergeDuplicateMonitorsIgnoresDifferentHost(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.Monitor{}, &models.MonitorMetric{}); err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, PublicHost: "203.0.113.10", Enabled: true, MonitorEnabled: true}
	models.DB.Create(&server)
	standalone := models.Monitor{Name: "other", SSHHost: "203.0.113.11"}
	models.DB.Create(&standalone)
	linked := models.Monitor{Name: "JP", ProxyServerID: &server.ID, Enabled: true}
	models.DB.Create(&linked)
	if err := MergeDuplicateMonitors(); err != nil {
		t.Fatal(err)
	}
	var count int64
	models.DB.Model(&models.Monitor{}).Count(&count)
	if count != 2 {
		t.Fatalf("different hosts must not merge, count=%d", count)
	}
}
