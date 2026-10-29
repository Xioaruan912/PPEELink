package api

import (
	"ppeelink/models"
	"ppeelink/relaycore"
	"ppeelink/xraycore"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func setupRelayDB(t *testing.T) (models.ProxyServer, models.Node) {
	s, _ := setupProxyDB(t)
	if e := models.DB.AutoMigrate(&models.AgentRelay{}, &models.AgentRelayDay{}); e != nil {
		t.Fatal(e)
	}
	s.PublicHost = "8.8.8.8"
	s.ProxyFeatures = 5
	models.DB.Save(&s)
	n := models.Node{Name: "JP", Link: "vless://11111111-1111-4111-8111-111111111111@1.1.1.1:443?security=reality&sni=example.com&pbk=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA#JP"}
	models.DB.Create(&n)
	return s, n
}
func TestAgentRelaySaveGeneratesEntryAndPreservesUpstream(t *testing.T) {
	s, n := setupRelayDB(t)
	req := relaySaveRequest{ServerID: s.ID, Name: "日本入口", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	rec := jsonRequest(t, AgentRelaySave, req, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var rr models.AgentRelay
	models.DB.First(&rr)
	var entry models.Node
	models.DB.First(&entry, rr.NodeID)
	if entry.Name != req.Name {
		t.Fatalf("relay node name should use the line name exactly: %q", entry.Name)
	}
	if !strings.Contains(entry.Link, "@8.8.8.8:25001") || !strings.Contains(entry.Link, "sni=example.com") {
		t.Fatalf("invalid forwarded link %s", entry.Link)
	}
	var upstream models.Node
	models.DB.First(&upstream, n.ID)
	if upstream.Link != n.Link {
		t.Fatal("upstream mutated")
	}
	spec, e := BuildServerSpec(s)
	if e != nil || len(spec.Relays) != 1 || spec.Relays[0].TargetHost != "1.1.1.1" {
		t.Fatalf("relay not dispatched %+v %v", spec.Relays, e)
	}
	req.ID = rr.ID
	req.ListenPort = 24443
	if rec = jsonRequest(t, AgentRelaySave, req, nil); rec.Code != 400 {
		t.Fatal("Xray port collision accepted")
	}
	s.ProxyFeatures = 4
	models.DB.Save(&s)
	req.ListenPort = 25001
	if rec = jsonRequest(t, AgentRelaySave, req, nil); rec.Code != 400 {
		t.Fatal("old agent accepted")
	}
}
func TestRelayTrafficReportIdempotent(t *testing.T) {
	s, n := setupRelayDB(t)
	r := models.AgentRelay{ServerID: s.ID, Name: "test", ListenPort: 26000, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&r)
	report := xraycore.Report{Relays: map[string]relaycore.Status{strconv.Itoa(int(r.ID)): {Session: "boot-1", Up: 100, Down: 200, Running: true, Revision: "a"}}}
	for i := 0; i < 2; i++ {
		if e := ApplyServerReport(s, report); e != nil {
			t.Fatal(e)
		}
	}
	models.DB.First(&r, r.ID)
	if r.Up != 100 || r.Down != 200 {
		t.Fatalf("report replay doubled %+v", r)
	}
	report.Relays[strconv.Itoa(int(r.ID))] = relaycore.Status{Session: "boot-1", Up: 90, Down: 150}
	if e := ApplyServerReport(s, report); e != nil {
		t.Fatal(e)
	}
	models.DB.First(&r, r.ID)
	if r.Up != 100 {
		t.Fatal("stale counter replayed")
	}
	report.Relays[strconv.Itoa(int(r.ID))] = relaycore.Status{Session: "boot-2", Up: 10, Down: 20}
	if e := ApplyServerReport(s, report); e != nil {
		t.Fatal(e)
	}
	models.DB.First(&r, r.ID)
	if r.Up != 110 || r.Down != 220 {
		t.Fatal("restart counters lost")
	}
}
func TestRelayRejectPrivateAndPhysicalLoops(t *testing.T) {
	s, n := setupRelayDB(t)
	req := relaySaveRequest{ServerID: s.ID, Name: "bad", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	for _, link := range []string{strings.Replace(n.Link, "1.1.1.1", "127.0.0.1", 1), strings.Replace(n.Link, "1.1.1.1:443", "8.8.8.8:25001", 1)} {
		models.DB.Model(&n).Update("link", link)
		rec := jsonRequest(t, AgentRelaySave, req, nil)
		if rec.Code != 400 {
			t.Fatalf("invalid target saved: %s", rec.Body.String())
		}
	}
	var cnt int64
	models.DB.Model(&models.AgentRelay{}).Count(&cnt)
	if cnt != 0 {
		t.Fatal("invalid save left partial data")
	}
}

func TestRelayDispatchRejectsLoopIntroducedByTargetEdit(t *testing.T) {
	a, n := setupRelayDB(t)
	b := models.ProxyServer{Name: "B", Kind: models.ServerKindAgent, PublicHost: "9.9.9.9", Enabled: true, ProxyFeatures: 5}
	models.DB.Create(&b)
	toA := models.Node{Name: "manual A", Link: strings.Replace(n.Link, "1.1.1.1:443", "8.8.8.8:25001", 1)}
	models.DB.Create(&toA)
	ra := models.AgentRelay{ServerID: a.ID, Name: "a", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	rb := models.AgentRelay{ServerID: b.ID, Name: "b", ListenPort: 25002, Network: "tcp", TargetNodeID: toA.ID, Enabled: true}
	models.DB.Create(&ra)
	models.DB.Create(&rb)
	good, e := BuildServerSpec(a)
	if e != nil || len(good.Relays) != 1 {
		t.Fatalf("valid path rejected: %v", e)
	}
	models.DB.Model(&n).Update("link", strings.Replace(n.Link, "1.1.1.1:443", "9.9.9.9:25002", 1))
	stopped, e := BuildServerSpec(a)
	if e != nil || len(stopped.Relays) != 0 {
		t.Fatalf("loop dispatched %+v %v", stopped.Relays, e)
	}
	models.DB.First(&ra, ra.ID)
	if !strings.Contains(ra.LastError, "循环") {
		t.Fatal("no actionable cycle error")
	}
}
func TestConcurrentRelayNodeSyncKeepsOneStableNode(t *testing.T) {
	s, n := setupRelayDB(t)
	r := models.AgentRelay{ServerID: s.ID, Name: "stable", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&r)
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- SyncAgentRelayNodes() }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var count int64
	models.DB.Model(&models.Node{}).Where("source = ?", "agent-relay:"+strconv.Itoa(int(r.ID))).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate generated nodes %d", count)
	}
	models.DB.First(&r, r.ID)
	var before models.Node
	models.DB.First(&before, r.NodeID)
	if !before.Hidden {
		t.Fatal("unconfirmed rule published")
	}
	sp, e := relayRuleSpec(r)
	if e != nil {
		t.Fatal(e)
	}
	report := xraycore.Report{Relays: map[string]relaycore.Status{strconv.Itoa(int(r.ID)): {Session: "confirmed", Revision: sp.Rev(), Running: true}}}
	if e = ApplyServerReport(s, report); e != nil {
		t.Fatal(e)
	}
	var after models.Node
	models.DB.First(&after, r.NodeID)
	if after.Hidden || after.ID != before.ID {
		t.Fatal("applied rule did not publish stable node")
	}
	if e = SyncAgentRelayNodes(); e != nil {
		t.Fatal(e)
	}
	var same models.Node
	models.DB.First(&same, r.NodeID)
	if !same.UpdatedAt.Equal(after.UpdatedAt) {
		t.Fatal("unchanged sync rewrote node on every poll")
	}
}
func TestRelayHY2LinkRetainsImplicitSNI(t *testing.T) {
	n := models.Node{Link: "hy2://secret@upstream.example:443"}
	link, e := relayEntryLink(n, "8.8.8.8", 25001)
	if e != nil || !strings.Contains(link, "sni=upstream.example") {
		t.Fatalf("upstream TLS identity changed %s %v", link, e)
	}
}

func TestRelayDisableDoesNotNeedValidTarget(t *testing.T) {
	s, n := setupRelayDB(t)
	r := models.AgentRelay{ServerID: s.ID, Name: "disable", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&r)
	models.DB.Delete(&n)
	req := relaySaveRequest{ID: r.ID, ServerID: s.ID, Name: r.Name, ListenPort: r.ListenPort, Network: r.Network, TargetNodeID: n.ID, Enabled: false}
	res := jsonRequest(t, AgentRelaySave, req, nil)
	if res.Code != 200 {
		t.Fatalf("cannot disable invalid upstream: %d %s", res.Code, res.Body.String())
	}
	models.DB.First(&r, r.ID)
	if r.Enabled {
		t.Fatal("disable not persisted")
	}
}

func TestDeletingServerRelaysRemovesOnlyGeneratedNodesAndKeepsTraffic(t *testing.T) {
	s, n := setupRelayDB(t)
	r := models.AgentRelay{ServerID: s.ID, Name: "remove", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	if err := models.DB.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	if err := SyncAgentRelayNodes(); err != nil {
		t.Fatal(err)
	}
	models.DB.First(&r, r.ID)
	if r.NodeID == 0 {
		t.Fatal("no generated relay node")
	}
	day := models.AgentRelayDay{RuleID: r.ID, ServerID: s.ID, Day: time.Now().Format("2006-01-02"), Up: 123}
	models.DB.Create(&day)
	if err := deleteServerAgentRelays(s.ID); err != nil {
		t.Fatal(err)
	}
	if err := SyncAgentRelayNodes(); err != nil {
		t.Fatal(err)
	}
	var generated models.Node
	if err := models.DB.First(&generated, r.NodeID).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("orphan relay node after server removal: %v", err)
	}
	if err := models.DB.First(&n, n.ID).Error; err != nil {
		t.Fatal("upstream deleted", err)
	}
	var count int64
	models.DB.Table("group_node_nodes").Where("node_id = ?", r.NodeID).Count(&count)
	if count != 0 {
		t.Fatal("orphan group membership")
	}
	if err := models.DB.First(&day, day.ID).Error; err != nil || day.Up != 123 {
		t.Fatal("historical traffic lost", err)
	}
}

func TestRelayConfirmationInvalidatesHiddenOverviewButUnchangedReportsKeepCache(t *testing.T) {
	s, n := setupRelayDB(t)
	t.Cleanup(InvalidateOverview)
	r := models.AgentRelay{ServerID: s.ID, Name: "publish", ListenPort: 25001, Network: "tcp", TargetNodeID: n.ID, Enabled: true}
	models.DB.Create(&r)
	if err := SyncAgentRelayNodes(); err != nil {
		t.Fatal(err)
	}
	models.DB.First(&r, r.ID)
	spec, err := relayRuleSpec(r)
	if err != nil {
		t.Fatal(err)
	}
	// The browser requested overview while the Agent had not applied the rule.
	oCache.mu.Lock()
	oCache.data, oCache.expires = []NodeOverviewItem{{ID: n.ID}}, time.Now().Add(5*time.Minute)
	oCache.mu.Unlock()
	report := xraycore.Report{Relays: map[string]relaycore.Status{strconv.Itoa(int(r.ID)): {Session: "publish", Revision: spec.Rev(), Running: true}}}
	if err := ApplyServerReport(s, report); err != nil {
		t.Fatal(err)
	}
	oCache.mu.Lock()
	invalidated := oCache.data == nil
	oCache.mu.Unlock()
	if !invalidated {
		t.Fatal("newly published relay remains missing from cached node overview")
	}
	// An unchanged heartbeat must not cause another network scan on every poll.
	oCache.mu.Lock()
	oCache.data, oCache.expires = []NodeOverviewItem{{ID: r.NodeID}}, time.Now().Add(5*time.Minute)
	oCache.mu.Unlock()
	if err := ApplyServerReport(s, report); err != nil {
		t.Fatal(err)
	}
	oCache.mu.Lock()
	kept := len(oCache.data) == 1 && oCache.data[0].ID == r.NodeID
	oCache.mu.Unlock()
	if !kept {
		t.Fatal("unchanged relay heartbeat unnecessarily invalidated overview")
	}
}
