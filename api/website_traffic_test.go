package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http/httptest"
	"net/url"
	"ppeelink/models"
	"ppeelink/xraycore"
	"testing"
	"time"
)

func setupWebsiteDB(t *testing.T) models.ProxyServer {
	t.Helper()
	s, _ := setupProxyDB(t)
	if err := models.DB.AutoMigrate(&models.WebsiteTraffic{}, &models.WebsiteTrafficReceipt{}); err != nil {
		t.Fatal(err)
	}
	return s
}
func websiteRequest(t *testing.T, query string, status int) map[string]json.RawMessage {
	t.Helper()
	r := gin.New()
	r.GET("/traffic", ProxyWebsiteTraffic)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/traffic?"+query, nil))
	if w.Code != status {
		t.Fatalf("status %d want %d: %s", w.Code, status, w.Body.String())
	}
	var envelope struct {
		Data map[string]json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}
func TestWebsiteTrafficIdempotentAndUnknown(t *testing.T) {
	s := setupWebsiteDB(t)
	hour := time.Now().UTC().Truncate(time.Hour)
	report := xraycore.Report{WebsiteBatchID: "0123456789abcdef0123456789abcdef", WebsiteTraffic: []xraycore.WebsiteTrafficBucket{
		{BucketStart: hour.Unix(), InboundTag: "in-1", Domain: "EXAMPLE.com.", Up: 10, Down: 20},
		{BucketStart: hour.Unix(), Domain: "192.0.2.2", Up: 3, Down: 4},
		{BucketStart: hour.Unix(), Domain: "https://fake.example/path", Up: 5, Down: 6},
	}}
	for i := 0; i < 2; i++ {
		if err := ApplyServerReport(s, report); err != nil {
			t.Fatal(err)
		}
	}
	var rows []models.WebsiteTraffic
	models.DB.Order("domain").Find(&rows)
	if len(rows) != 2 || rows[0].Domain != "example.com" || rows[0].Up != 10 || rows[1].Domain != "unknown" || rows[1].Down != 10 {
		t.Fatalf("bad rows %+v", rows)
	}
	q := url.Values{"start": {hour.Format(time.RFC3339)}, "end": {hour.Add(time.Hour).Format(time.RFC3339)}}
	data := websiteRequest(t, q.Encode(), 200)
	var items []websiteTrafficRow
	json.Unmarshal(data["items"], &items)
	if len(items) != 2 || items[0].Domain != "example.com" || items[0].Rank != 1 || items[0].ServerName != "JP" {
		t.Fatalf("bad rank %s", data["items"])
	}
	var sum struct{ Up, Down, Total int64 }
	json.Unmarshal(data["summary"], &sum)
	if sum.Up != 18 || sum.Down != 30 || sum.Total != 48 {
		t.Fatalf("bad sum %s", data["summary"])
	}
	// Summary is for all filtered records, not only the top-N ranking.
	q.Set("limit", "1")
	data = websiteRequest(t, q.Encode(), 200)
	json.Unmarshal(data["summary"], &sum)
	if sum.Total != 48 {
		t.Fatalf("top N truncated summary %s", data["summary"])
	}
	q.Set("domain", "unknown")
	data = websiteRequest(t, q.Encode(), 200)
	json.Unmarshal(data["items"], &items)
	if len(items) != 1 || items[0].Total != 18 {
		t.Fatalf("unknown filter failed %s", data["items"])
	}
}
func TestWebsiteTrafficRangeNodeAndOrder(t *testing.T) {
	s := setupWebsiteDB(t)
	h := time.Now().UTC().Truncate(time.Hour)
	s2 := models.ProxyServer{Name: "Other", Kind: models.ServerKindAgent}
	models.DB.Create(&s2)
	for _, r := range []models.WebsiteTraffic{{ServerID: s.ID, BucketStart: h, Domain: "a.test", Up: 50, Down: 1}, {ServerID: s.ID, BucketStart: h, Domain: "b.test", Up: 2, Down: 100}, {ServerID: s2.ID, BucketStart: h, Domain: "a.test", Up: 900, Down: 900}, {ServerID: s.ID, BucketStart: h.Add(time.Hour), Domain: "outside.test", Up: 9, Down: 9}} {
		if err := models.DB.Create(&r).Error; err != nil {
			t.Fatal(err)
		}
	}
	q := url.Values{"start": {h.Format(time.RFC3339)}, "end": {h.Add(time.Hour).Format(time.RFC3339)}, "serverId": {"1"}, "order": {"up"}}
	data := websiteRequest(t, q.Encode(), 200)
	var rows []websiteTrafficRow
	json.Unmarshal(data["items"], &rows)
	if len(rows) != 2 || rows[0].Domain != "a.test" || rows[1].Domain != "b.test" {
		t.Fatalf("filter/order %s", data["items"])
	}
	for _, extra := range []string{"order=up%3BDELETE", "limit=0", "limit=101", "serverId=-1", "domain=https%3A%2F%2Fx.test", "start=2026-01-01T00%3A30%3A00Z", "end=bad"} {
		websiteRequest(t, extra, 400)
	}
}
func TestWebsiteTrafficValidationRollbackAndMigration(t *testing.T) {
	s := setupWebsiteDB(t)
	h := time.Now().UTC().Truncate(time.Hour)
	// Running AutoMigrate again on an existing schema preserves rows.
	row := models.WebsiteTraffic{ServerID: s.ID, BucketStart: h, Domain: "keep.test", Up: 1}
	models.DB.Create(&row)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.WebsiteTraffic{}, &models.WebsiteTrafficReceipt{}); err != nil {
		t.Fatal(err)
	}
	var n int64
	models.DB.Model(&models.WebsiteTraffic{}).Count(&n)
	if n != 1 {
		t.Fatal("migration lost data")
	}
	report := xraycore.Report{WebsiteBatchID: "0123456789abcdef", WebsiteTraffic: []xraycore.WebsiteTrafficBucket{{BucketStart: h.Unix(), Domain: "x.test", Up: -1}}}
	if err := ApplyServerReport(s, report); err == nil {
		t.Fatal("negative traffic accepted")
	}
	models.DB.Model(&models.WebsiteTrafficReceipt{}).Count(&n)
	if n != 0 {
		t.Fatal("invalid batch got receipt")
	}
	report.WebsiteTraffic[0].Up = 2
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := applyWebsiteTraffic(tx, s.ID, report, time.Now()); err != nil {
			return err
		}
		return gorm.ErrInvalidData
	}); err == nil {
		t.Fatal("expected rollback")
	}
	models.DB.Model(&models.WebsiteTrafficReceipt{}).Count(&n)
	if n != 0 {
		t.Fatal("receipt escaped rollback")
	}
	if err := ApplyServerReport(s, report); err != nil {
		t.Fatal(err)
	}
}
func TestDisabledMetricsKeepsAgentHeartbeat(t *testing.T) {
	s := setupWebsiteDB(t)
	m := models.Monitor{Name: "linked", ProxyServerID: &s.ID, Enabled: true}
	if err := models.DB.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	response := jsonRequest(t, ProxyServerSave, serverSaveRequest{ID: s.ID, Name: s.Name, PublicHost: s.PublicHost, Enabled: false, MonitorEnabled: false, WebsiteTrafficEnabled: true}, nil)
	if response.Code != 200 {
		t.Fatalf("save failed: %s", response.Body.String())
	}
	models.DB.First(&s, s.ID)
	if s.Enabled || s.MonitorEnabled || !s.WebsiteTrafficEnabled {
		t.Fatalf("switches not independent: %+v", s)
	}
	if err := ApplyServerReport(s, xraycore.Report{AgentVersion: "real-build", AgentHash: "real-hash", Arch: "amd64"}); err != nil {
		t.Fatal(err)
	}
	models.DB.First(&m, m.ID)
	if m.Enabled || m.LastSeenAt == nil || m.Version != "real-build" || m.AgentHash != "real-hash" {
		t.Fatalf("metrics disabled broke management state: %+v", m)
	}
	var count int64
	models.DB.Model(&models.MonitorMetric{}).Count(&count)
	if count != 0 {
		t.Fatal("disabled metrics were persisted")
	}
}

func TestWebsiteCollectionSwitchReachesAgent(t *testing.T) {
	s := setupWebsiteDB(t)
	spec, err := BuildServerSpec(s)
	if err != nil {
		t.Fatal(err)
	}
	if spec.WebsiteTraffic == nil || spec.WebsiteTraffic.Enabled {
		t.Fatal("collection must default off")
	}
	s.WebsiteTrafficEnabled = true
	spec, err = BuildServerSpec(s)
	if err != nil {
		t.Fatal(err)
	}
	if !spec.WebsiteTraffic.Enabled || spec.WebsiteTraffic.BucketSeconds != 3600 {
		t.Fatal("website switch not sent to agent")
	}
}
