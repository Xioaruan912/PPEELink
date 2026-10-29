package api

import (
	"github.com/gin-gonic/gin"
	"ppeelink/node"
	"strings"
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

func TestDeliverySwitchDoesNotHideOrLoseNodeAndCannotBeOverridden(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	t.Cleanup(InvalidateOverview)
	models.DB.Model(&nodes[0]).Update("force_deliver", true)
	res := jsonRequest(t, NodeDelivery, map[string]any{"id": nodes[0].ID, "enabled": false}, nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	listed, err := models.GetNodeList()
	if err != nil || len(listed) != 2 {
		t.Fatalf("switch hid node: %d %v", len(listed), err)
	}
	items := withBlocks([]NodeOverviewItem{{ID: nodes[0].ID}, {ID: nodes[1].ID}})
	if items[0].DeliveryEnabled || items[0].Blocked == nil || items[0].Blocked.Kind != blockDisabled || !items[1].DeliveryEnabled {
		t.Fatalf("overview does not reflect switch: %+v", items)
	}
	names, excluded := deliveredNames(t, sub)
	if len(names) != 1 || names[0] != "B" || len(excluded) != 1 || excluded[0].Kind != blockDisabled {
		t.Fatalf("disabled node delivered: %v %+v", names, excluded)
	}
	jsonRequest(t, NodeDelivery, map[string]any{"id": nodes[1].ID, "enabled": false}, nil)
	if names, _ := deliveredNames(t, sub); len(names) != 0 {
		t.Fatal("all-disabled fallback revived nodes", names)
	}
	res = jsonRequest(t, NodeDelivery, map[string]any{"id": nodes[0].ID, "enabled": true}, nil)
	if res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if names, _ := deliveredNames(t, sub); len(names) != 1 || names[0] != "A" {
		t.Fatal("re-enabling failed", names)
	}
	var saved models.Node
	models.DB.First(&saved, nodes[0].ID)
	if saved.Hidden || saved.Link != nodes[0].Link || saved.ID != nodes[0].ID || !saved.ForceDeliver {
		t.Fatal("delivery switch changed node identity or other settings")
	}
}

func TestDeliverySwitchRequiresAnExplicitState(t *testing.T) {
	_, _, nodes := deliveryFixture(t)
	for _, req := range []map[string]any{{"id": nodes[0].ID}, {"id": nodes[0].ID, "enabled": "false"}, {"id": 0, "enabled": true}} {
		if res := jsonRequest(t, NodeDelivery, req, nil); res.Code != 400 {
			t.Fatal("invalid request accepted")
		}
	}
	if res := jsonRequest(t, NodeDelivery, map[string]any{"id": 999999, "enabled": false}, nil); res.Code != 404 {
		t.Fatal("missing node accepted")
	}
}

func TestDisabledNodeDoesNotLeakThroughTemporaryOrOldFallbackSubscription(t *testing.T) {
	sub, _, nodes := deliveryFixture(t)
	sub.Config = "{}"
	models.DB.Model(&sub).Update("config", sub.Config)
	oldBody := []byte(node.Base64Encode(nodes[0].Link + "\n" + nodes[1].Link))
	rememberGoodArtifact(sub.ID, "v2ray", oldBody, "")
	if res := jsonRequest(t, NodeDelivery, map[string]any{"id": nodes[0].ID, "enabled": false}, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	token := signTempLink(nodes[0].ID, time.Now().Add(time.Hour).Unix())
	res := jsonRequest(t, GetTempLink, nil, func(c *gin.Context) { c.Request.URL.RawQuery = "token=" + token })
	if res.Code != 403 {
		t.Fatal("disabled node leaked through temp link", res.Code)
	}
	res = jsonRequest(t, func(c *gin.Context) { serveSubscriptionClient(c, sub, "v2ray") }, nil, nil)
	decoded := node.Base64Decode(res.Body.String())
	if res.Code != 200 || strings.Contains(decoded, nodes[0].Link) || !strings.Contains(decoded, nodes[1].Link) {
		t.Fatalf("live subscription ignored switch: HTTP %d", res.Code)
	}
	// Recreate a stale cache and fail generation: it must not bypass the switch.
	models.DB.Where("subscription_id = ?", sub.ID).Delete(&models.SubscriptionArtifactPointer{})
	rememberGoodArtifact(sub.ID, "v2ray", oldBody, "")
	sub.Name = "missing-subscription"
	res = jsonRequest(t, func(c *gin.Context) { serveSubscriptionClient(c, sub, "v2ray") }, nil, nil)
	if res.Code != 502 || res.Header().Get("X-SubLinkX-Artifact") != "" {
		t.Fatal("old artifact bypassed delivery settings", res.Code)
	}
}

func TestPersonalDeliverySwitchSurvivesManagedNodeSync(t *testing.T) {
	_, _, user := selfHostedFixture(t)
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	var n models.Node
	models.DB.Where("source = ?", selfHostedSource).First(&n)
	var account models.ProxyAccount
	models.DB.Where("user_id = ?", user.ID).First(&account)
	if len(AccountNodes(account)) != 1 {
		t.Fatal("fixture missing account node")
	}
	if res := jsonRequest(t, NodeDelivery, map[string]any{"id": n.ID, "enabled": false}, nil); res.Code != 200 {
		t.Fatal(res.Body.String())
	}
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	if len(AccountNodes(account)) != 0 {
		t.Fatal("personal subscription bypassed node switch")
	}
	var same models.Node
	if err := models.DB.First(&same, n.ID).Error; err != nil || !same.DeliveryDisabled {
		t.Fatal("managed sync overwrote switch", err)
	}
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
	models.DB.Create(&models.ProxyInbound{ServerID: server.ID, Remark: "B", Template: "vless-reality-vision", Port: 443, Enabled: true})
	nodes[1].Name = "JP B"
	nodes[1].Source = selfHostedSource
	models.DB.Model(&nodes[1]).Updates(map[string]any{"name": "JP B", "source": selfHostedSource})
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
