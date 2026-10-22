package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ppeelink/models"
)

func traceDB(t *testing.T) {
	t.Helper()
	old := models.DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	models.DB = db
	t.Cleanup(func() { sql, _ := db.DB(); _ = sql.Close(); models.DB = old })
	if err := db.AutoMigrate(&models.Probe{}, &models.NetworkTraceRun{}, &models.NetworkTraceView{}, &models.TraceProbeCapability{}); err != nil {
		t.Fatal(err)
	}
}
func traceRequest(handler gin.HandlerFunc, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if token != "" {
		c.Request.Header.Set("Authorization", "Probe "+token)
	}
	handler(c)
	return w
}
func TestNetworkTraceCreateAndCancel(t *testing.T) {
	traceDB(t)
	p := models.Probe{Name: "source", TokenHash: models.HashProbeToken("test"), Enabled: true}
	models.DB.Create(&p)
	models.DB.Create(&models.TraceProbeCapability{ProbeID: p.ID, SeenAt: time.Now(), Tool: "traceroute"})
	bad := traceRequest(NetworkTraceCreate, "POST", "/", `{"target":"x;whoami","probeIds":[1]}`, "")
	if bad.Code != 400 {
		t.Fatal(bad.Code)
	}
	w := traceRequest(NetworkTraceCreate, "POST", "/", `{"target":"example.com","includePanel":false,"probeIds":[1,1]}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var views []models.NetworkTraceView
	models.DB.Find(&views)
	if len(views) != 1 || views[0].Status != "queued" {
		t.Fatalf("%+v", views)
	}
	job := traceRequest(ProbeTraceJobs, "GET", "/?tool=traceroute", "", "test")
	if job.Code != 200 || !strings.Contains(job.Body.String(), "example.com") {
		t.Fatal(job.Body.String())
	}
	cancel := traceRequest(NetworkTraceCancel, "POST", "/", fmt.Sprintf(`{"id":%d}`, views[0].RunID), "")
	if cancel.Code != 200 {
		t.Fatal(cancel.Body.String())
	}
	report := traceRequest(ProbeTraceReport, "POST", "/", fmt.Sprintf(`{"id":%d,"tool":"traceroute","output":"1 1.1.1.1 2ms"}`, views[0].ID), "test")
	if report.Code != 409 {
		t.Fatal(report.Body.String())
	}
}
func TestNetworkTraceProbeIsolationAndBounds(t *testing.T) {
	traceDB(t)
	for _, token := range []string{"a", "b"} {
		models.DB.Create(&models.Probe{Name: token, TokenHash: models.HashProbeToken(token), Enabled: true})
	}
	run := models.NetworkTraceRun{Target: "1.1.1.1"}
	models.DB.Create(&run)
	view := models.NetworkTraceView{RunID: run.ID, ProbeID: 1, Status: "running"}
	models.DB.Create(&view)
	body := fmt.Sprintf(`{"id":%d,"tool":"traceroute","output":"1 1.1.1.1 2ms"}`, view.ID)
	if w := traceRequest(ProbeTraceReport, "POST", "/", body, "b"); w.Code != 409 {
		t.Fatal(w.Body.String())
	}
	if w := traceRequest(ProbeTraceReport, "POST", "/", body, "a"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	models.DB.First(&view, view.ID)
	if view.Status != "completed" {
		t.Fatal(view.Status)
	}
	large, _ := json.Marshal(map[string]any{"id": view.ID, "output": strings.Repeat("x", 33000)})
	if w := traceRequest(ProbeTraceReport, "POST", "/", string(large), "a"); w.Code != 400 {
		t.Fatal(w.Code)
	}
	if w := traceRequest(ProbeTraceJobs, "GET", "/", "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
}
func TestNetworkTraceUnsupportedAndExpiry(t *testing.T) {
	traceDB(t)
	models.DB.Create(&models.Probe{Name: "legacy", TokenHash: models.HashProbeToken("legacy"), Enabled: true})
	w := traceRequest(NetworkTraceCreate, "POST", "/", `{"target":"1.1.1.1","probeIds":[1]}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var view models.NetworkTraceView
	models.DB.First(&view)
	if view.Status != "unsupported" {
		t.Fatal(view.Status)
	}
	old := models.NetworkTraceView{RunID: view.RunID, Status: "running", CreatedAt: time.Now().Add(-4 * time.Minute)}
	models.DB.Create(&old)
	expireNetworkTraces()
	models.DB.First(&old, old.ID)
	if old.Status != "timeout" {
		t.Fatal(old.Status)
	}
}
