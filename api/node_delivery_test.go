package api

import (
	"testing"
	"time"

	"ppeelink/models"
)

func deliveryFixture(t *testing.T) (models.Subcription, models.GroupNode, []models.Node) {
	t.Helper()
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.NodeHealthEvent{}, &models.AlertSetting{}, &models.ProxyServer{}, &models.ProxyTraffic{}, &models.ProxyInbound{}, &models.ProxyAccount{}); err != nil {
		t.Fatal(err)
	}
	group := models.GroupNode{Name: "G"}
	models.DB.Create(&group)
	nodes := []models.Node{{Name: "A", Link: testLinkA}, {Name: "B", Link: testLinkB}}
	for i := range nodes {
		models.DB.Create(&nodes[i])
		_ = models.DB.Model(&nodes[i]).Association("GroupNodes").Append(&group)
	}
	sub := models.Subcription{Name: "S", Token: "tok"}
	models.DB.Create(&sub)
	_ = models.DB.Model(&sub).Association("GroupRefs").Append(&group)
	return sub, group, nodes
}

func deliveredNames(t *testing.T, sub models.Subcription) ([]string, []excludedNode) {
	t.Helper()
	excluded, err := mergeGroupNodesDetailed(&sub)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{}
	for _, n := range sub.Nodes {
		names = append(names, n.Name)
	}
	return names, excluded
}

func TestPausedNodeIsNotDeliveredUntilResumeDate(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	future := time.Now().Add(48 * time.Hour)
	models.DB.Model(&nodes[0]).Updates(map[string]any{"paused_until": future, "pause_note": "本月流量用完"})
	names, excluded := deliveredNames(t, sub)
	if len(names) != 1 || names[0] != "B" || len(excluded) != 1 || excluded[0].Kind != blockPaused {
		t.Fatalf("paused node must be held back: %v %+v", names, excluded)
	}
	past := time.Now().Add(-time.Hour)
	models.DB.Model(&nodes[0]).Update("paused_until", past)
	if names, _ := deliveredNames(t, sub); len(names) != 2 {
		t.Fatalf("pause must end on its date: %v", names)
	}
}

func TestHealthDownExcludedButNeverEmptiesSubscription(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	models.DB.Create(&models.NodeHealthEvent{NodeID: nodes[0].ID, Type: "down", CreatedAt: time.Now()})
	if names, _ := deliveredNames(t, sub); len(names) != 1 || names[0] != "B" {
		t.Fatalf("node down for consecutive checks must be held back: %v", names)
	}
	models.DB.Create(&models.NodeHealthEvent{NodeID: nodes[1].ID, Type: "down", CreatedAt: time.Now()})
	if names, _ := deliveredNames(t, sub); len(names) != 2 {
		t.Fatalf("if every node looks down the panel's own network is suspect; keep them: %v", names)
	}
	models.DB.Create(&models.NodeHealthEvent{NodeID: nodes[0].ID, Type: "recovery", CreatedAt: time.Now()})
	if names, _ := deliveredNames(t, sub); len(names) != 1 || names[0] != "A" {
		t.Fatalf("recovered node must come back: %v", names)
	}
}

func TestServerQuotaHoldsNodesAndStopsXray(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, PublicHost: "jp.example.com", Enabled: true,
		TrafficLimit: 1000, ResetDay: 1}
	models.DB.Create(&server)
	selfGroup := models.GroupNode{Name: selfHostedGroupPrefix + "JP"}
	models.DB.Create(&selfGroup)
	_ = models.DB.Model(&nodes[1]).Association("GroupNodes").Append(&selfGroup)
	today := time.Now().Format("2006-01-02")
	models.DB.Create(&models.ProxyTraffic{Day: today, UserID: 1, ServerID: server.ID, Up: 400, Down: 700})
	spec, err := BuildServerSpec(server)
	if err != nil || spec.Enabled {
		t.Fatalf("over-quota server must stop xray: %+v %v", spec, err)
	}
	names, excluded := deliveredNames(t, sub)
	if len(names) != 1 || names[0] != "A" || excluded[0].Kind != blockServer || excluded[0].Until == nil {
		t.Fatalf("self-hosted nodes over quota must be held back until reset: %v %+v", names, excluded)
	}
}

func TestQuotaPeriod(t *testing.T) {
	loc := time.Local
	start, next := quotaPeriod(time.Date(2026, 10, 19, 12, 0, 0, 0, loc), 15)
	if start != time.Date(2026, 10, 15, 0, 0, 0, 0, loc) || next != time.Date(2026, 11, 15, 0, 0, 0, 0, loc) {
		t.Fatalf("after reset day: %v %v", start, next)
	}
	start, next = quotaPeriod(time.Date(2026, 10, 3, 12, 0, 0, 0, loc), 15)
	if start != time.Date(2026, 9, 15, 0, 0, 0, 0, loc) || next != time.Date(2026, 10, 15, 0, 0, 0, 0, loc) {
		t.Fatalf("before reset day: %v %v", start, next)
	}
}

func disableAutoBlock(t *testing.T) {
	t.Helper()
	if err := models.DB.Create(&models.AlertSetting{FailureThreshold: 3}).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Model(&models.AlertSetting{}).Where("1 = 1").Update("auto_block_invalid", false).Error; err != nil {
		t.Fatal(err)
	}
}

func TestHealthDownIgnoredWhenAutoBlockDisabled(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	disableAutoBlock(t)
	models.DB.Create(&models.NodeHealthEvent{NodeID: nodes[0].ID, Type: "down", CreatedAt: time.Now()})
	names, excluded := deliveredNames(t, sub)
	if len(names) != 2 || len(excluded) != 0 {
		t.Fatalf("auto block disabled must keep every node: %v %+v", names, excluded)
	}
}

func TestForceDeliverOverridesHealthBlockOnly(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	models.DB.Create(&models.NodeHealthEvent{NodeID: nodes[0].ID, Type: "down", CreatedAt: time.Now()})
	if err := models.DB.Model(&nodes[0]).Update("force_deliver", true).Error; err != nil {
		t.Fatal(err)
	}
	if names, _ := deliveredNames(t, sub); len(names) != 2 {
		t.Fatalf("force-deliver node must stay: %v", names)
	}
	future := time.Now().Add(48 * time.Hour)
	models.DB.Model(&nodes[0]).Updates(map[string]any{"paused_until": future, "pause_note": "手动暂停"})
	names, excluded := deliveredNames(t, sub)
	if len(names) != 1 || names[0] != "B" || len(excluded) != 1 || excluded[0].Kind != blockPaused {
		t.Fatalf("manual pause must still win over force-deliver: %v %+v", names, excluded)
	}
}
