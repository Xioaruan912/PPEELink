package api

import (
	"testing"

	"ppeelink/models"
)

func selfHostedFixture(t *testing.T) (models.ProxyServer, models.ProxyInbound, models.User) {
	t.Helper()
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ProxyInbound{}, &models.ProxyAccount{}); err != nil {
		t.Fatal(err)
	}
	user := models.User{Username: "group-owner", Password: "test-password", Role: models.RoleAdmin, Nickname: "owner"}
	if err := user.Create(); err != nil {
		t.Fatal(err)
	}
	account := models.ProxyAccount{UserID: user.ID, Enabled: true, UUID: "11111111-1111-4111-8111-111111111111"}
	if err := models.DB.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	server := models.ProxyServer{Name: "JP", Kind: models.ServerKindAgent, PublicHost: "203.0.113.10", Enabled: true}
	if err := models.DB.Create(&server).Error; err != nil {
		t.Fatal(err)
	}
	inbound := models.ProxyInbound{ServerID: server.ID, Remark: "Reality", Template: "vless-reality-vision", Port: 443, SNI: "www.apple.com", PublicKey: "pk", Enabled: true}
	if err := models.DB.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	return server, inbound, user
}

func TestSyncSelfHostedNodesMigratesToSingleGroup(t *testing.T) {
	_, inbound, _ := selfHostedFixture(t)
	name := "JP Reality"
	generated := models.Node{Name: name, Link: "vless://old@example:443", ProbePolicy: "panel"}
	if err := models.DB.Create(&generated).Error; err != nil {
		t.Fatal(err)
	}
	legacy := models.GroupNode{Name: "自建·JP", Nodes: []models.Node{generated}}
	if err := models.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	sub := models.Subcription{Name: "existing", GroupRefs: []models.GroupNode{legacy}}
	if err := sub.Add(); err != nil {
		t.Fatal(err)
	}

	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}

	var groups []models.GroupNode
	models.DB.Order("id").Find(&groups)
	if len(groups) != 1 || groups[0].Name != selfHostedGroupName {
		t.Fatalf("groups = %#v", groups)
	}
	var nodes []models.Node
	models.DB.Find(&nodes)
	if len(nodes) != 1 || nodes[0].Name != name {
		t.Fatalf("nodes = %#v", nodes)
	}
	if nodes[0].Source != selfHostedSource || nodes[0].ProbePolicy != "any" {
		t.Fatalf("managed node not normalized: %#v", nodes[0])
	}
	var refs []struct {
		GroupNodeID int
	}
	models.DB.Table("subcription_groups").Where("subcription_id = ?", sub.ID).Scan(&refs)
	if len(refs) != 1 || refs[0].GroupNodeID != groups[0].ID {
		t.Fatalf("subscription refs = %#v", refs)
	}
	if inbound.Port != 443 {
		t.Fatal("fixture mutated")
	}
}

func TestSyncSelfHostedNodesMergesDuplicateAndMovesDirectNodeRef(t *testing.T) {
	_, _, _ = selfHostedFixture(t)
	name := "JP Reality"
	keep := models.Node{Name: name, Link: "vless://new@example:443", Source: "self-hosted"}
	if err := models.DB.Create(&keep).Error; err != nil {
		t.Fatal(err)
	}
	target := models.GroupNode{Name: selfHostedGroupName, Nodes: []models.Node{keep}}
	if err := models.DB.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := models.Node{Name: name, Link: "vless://old@example:443", ProbePolicy: "panel"}
	if err := models.DB.Create(&duplicate).Error; err != nil {
		t.Fatal(err)
	}
	legacy := models.GroupNode{Name: "自建·JP", Nodes: []models.Node{duplicate}}
	if err := models.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	sub := models.Subcription{Name: "direct", Nodes: []models.Node{duplicate}}
	if err := sub.Add(); err != nil {
		t.Fatal(err)
	}
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	var nodes []models.Node
	models.DB.Find(&nodes)
	if len(nodes) != 1 || nodes[0].ID != int(keep.ID) {
		t.Fatalf("duplicates not merged: %#v", nodes)
	}
	var direct []struct{ NodeID int }
	models.DB.Table("subcription_nodes").Where("subcription_id = ?", sub.ID).Scan(&direct)
	if len(direct) != 1 || direct[0].NodeID != int(keep.ID) {
		t.Fatalf("direct refs = %#v", direct)
	}
}
