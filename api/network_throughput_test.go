package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"ppeelink/models"
	"strings"
	"testing"
	"time"
)

func throughputDB(t *testing.T) {
	traceDB(t)
	if err := models.DB.AutoMigrate(&models.ProxyServer{}, &models.ThroughputCapability{}, &models.ThroughputRun{}, &models.ThroughputJob{}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 3; i++ {
		s := models.ProxyServer{ID: uint(i), Name: fmt.Sprintf("VPS %d", i), Kind: models.ServerKindAgent, PublicHost: fmt.Sprintf("192.0.2.%d", i), Enabled: true, AgentToken: models.HashAgentToken(fmt.Sprintf("token%d", i))}
		models.DB.Create(&s)
		models.DB.Create(&models.ThroughputCapability{ServerID: s.ID, Available: true, AuthSupported: true, BidirSupported: true, Version: "3.20", SeenAt: time.Now()})
	}
}
func throughputReq(h gin.HandlerFunc, method, path, body, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if token != "" {
		c.Request.Header.Set("Authorization", "Agent "+token)
	}
	h(c)
	return w
}
func createThroughput(t *testing.T) models.ThroughputRun {
	t.Helper()
	w := throughputReq(NetworkThroughputCreate, "POST", "/", `{"landingId":1,"sourceIds":[2,3],"protocol":"tcp","direction":"both","family":4,"duration":5,"streams":1,"rateMbps":10,"port":5201}`, "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var payload struct {
		Data models.ThroughputRun `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data
}
func TestThroughputValidationAndEndpointReservation(t *testing.T) {
	throughputDB(t)
	for _, body := range []string{`{"landingId":1,"sourceIds":[1]}`, `{"landingId":1,"sourceIds":[2],"protocol":"udp","rateMbps":0}`, `{"landingId":1,"sourceIds":[2],"duration":999}`, `{"landingId":1,"sourceIds":[2],"streams":99}`, `{"landingId":1,"sourceIds":[2],"port":443}`} {
		if w := throughputReq(NetworkThroughputCreate, "POST", "/", body, ""); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	run := createThroughput(t)
	if run.ID == 0 {
		t.Fatal("missing run")
	}
	if w := throughputReq(NetworkThroughputCreate, "POST", "/", `{"landingId":2,"sourceIds":[3],"protocol":"tcp","direction":"forward","family":4,"duration":5,"streams":1,"rateMbps":10}`, ""); w.Code != 409 {
		t.Fatal("overlapping endpoint", w.Code, w.Body.String())
	}
	var jobs []models.ThroughputJob
	models.DB.Order("sequence").Find(&jobs)
	if len(jobs) != 4 || jobs[0].Status != "starting" || jobs[1].Status != "queued" {
		t.Fatalf("jobs %+v", jobs)
	}
	detail := throughputReq(NetworkThroughputDetail, "GET", fmt.Sprintf("/?id=%d", run.ID), "", "")
	if detail.Code != 200 || strings.Contains(detail.Body.String(), "PRIVATE KEY") || strings.Contains(detail.Body.String(), "password") {
		t.Fatal("secret exposed", detail.Body.String())
	}
}
func TestThroughputReadinessOwnershipAndSequentialDispatch(t *testing.T) {
	throughputDB(t)
	run := createThroughput(t)
	var job models.ThroughputJob
	models.DB.Order("sequence").First(&job)
	before := throughputReq(AgentThroughputJobs, "GET", "/", "", "token2")
	if strings.Contains(before.Body.String(), `"role":"client"`) {
		t.Fatal("client dispatched before server ready")
	}
	listener := throughputReq(AgentThroughputJobs, "GET", "/", "", "token1")
	if !strings.Contains(listener.Body.String(), `"role":"server"`) || !strings.Contains(listener.Body.String(), "PRIVATE KEY") {
		t.Fatal(listener.Body.String())
	}
	wrong := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"server","status":"ready"}`, job.ID), "token3")
	if wrong.Code != 409 {
		t.Fatal(wrong.Code)
	}
	ready := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"server","status":"ready"}`, job.ID), "token1")
	if ready.Code != 200 {
		t.Fatal(ready.Body.String())
	}
	client := throughputReq(AgentThroughputJobs, "GET", "/", "", "token2")
	if !strings.Contains(client.Body.String(), `"role":"client"`) || strings.Contains(client.Body.String(), "PRIVATE KEY") {
		t.Fatal(client.Body.String())
	}
	complete := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"client","status":"completed","result":{"protocol":"tcp","direction":"forward","duration":5,"streams":1,"rateMbps":10,"forward":{"bitsPerSecond":9000000,"bytes":5625000}}}`, job.ID), "token2")
	if complete.Code != 200 {
		t.Fatal(complete.Body.String())
	}
	var jobs []models.ThroughputJob
	models.DB.Where("run_id = ?", run.ID).Order("sequence").Find(&jobs)
	if jobs[0].Status != "completed" || jobs[1].Status != "starting" || jobs[2].Status != "queued" {
		t.Fatalf("wrong sequencing %+v", jobs)
	}
	if w := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"client","status":"completed"}`, job.ID), "token2"); w.Code != 409 {
		t.Fatal("accepted duplicate report", w.Code)
	}
}
func TestThroughputCancelExpiryAndCapability(t *testing.T) {
	throughputDB(t)
	run := createThroughput(t)
	var job models.ThroughputJob
	models.DB.Order("sequence").First(&job)
	if w := throughputReq(NetworkThroughputCancel, "POST", "/", fmt.Sprintf(`{"id":%d}`, run.ID), ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, token := range []string{"token1", "token2"} {
		w := throughputReq(AgentThroughputJobs, "GET", "/", "", token)
		if !strings.Contains(w.Body.String(), `"jobs":[]`) {
			t.Fatal(w.Body.String())
		}
	}
	if w := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"server","status":"ready"}`, job.ID), "token1"); w.Code != 409 {
		t.Fatal("late report accepted", w.Code)
	}
	models.DB.Model(&models.ThroughputJob{}).Where("run_id = ?", run.ID).Update("finished_at", time.Now().Add(-20*time.Second))
	run = createThroughput(t)
	job = models.ThroughputJob{}
	models.DB.Where("run_id = ? AND status = ?", run.ID, "starting").First(&job)
	models.DB.Model(&models.ThroughputJob{}).Where("id = ?", job.ID).Update("expires_at", time.Now().Add(-time.Second))
	throughputReq(AgentThroughputJobs, "GET", "/", "", "token1")
	models.DB.First(&job, job.ID)
	if job.Status != "timeout" {
		t.Fatal(job.Status)
	}
	if w := throughputReq(AgentThroughputJobs, "GET", "/", "", ""); w.Code != 401 {
		t.Fatal("missing auth", w.Code)
	}
	if w := throughputReq(AgentThroughputCapabilities, "POST", "/", `{"available":true,"authSupported":true,"version":"3.20"}`, "token2"); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
}

func TestThroughputProgressAndPreparationCannotInterruptMeasurement(t *testing.T) {
	throughputDB(t)
	run := createThroughput(t)
	var job models.ThroughputJob
	models.DB.Where("run_id = ?", run.ID).Order("sequence").First(&job)
	if w := throughputReq(NetworkThroughputPrepare, "POST", "/", `{"serverIds":[1]}`, ""); w.Code != 409 {
		t.Fatal("prepare interrupted active endpoint", w.Code, w.Body.String())
	}
	throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"server","status":"ready"}`, job.ID), "token1")
	heartbeat := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"client","status":"running"}`, job.ID), "token2")
	if heartbeat.Code != 200 {
		t.Fatal(heartbeat.Body.String())
	}
	progress := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"client","status":"running","result":{"protocol":"tcp","direction":"forward","duration":5,"streams":1,"rateMbps":10,"progressObservation":"sender","progressIntervals":[{"start":0,"end":1,"bitsPerSecond":10000000}]}}`, job.ID), "token2")
	if progress.Code != 200 {
		t.Fatal(progress.Body.String())
	}
	var after models.ThroughputJob
	models.DB.First(&after, job.ID)
	if after.Status != "running" || !strings.Contains(string(after.Result), "progressIntervals") {
		t.Fatal("progress finished job or was not persisted")
	}
	detail := throughputReq(NetworkThroughputDetail, "GET", fmt.Sprintf("/?id=%d", run.ID), "", "")
	if !strings.Contains(detail.Body.String(), `"progressObservation":"sender"`) {
		t.Fatal(detail.Body.String())
	}
}

func TestThroughputImmediateReuseWaitsForCancelledAgentCleanup(t *testing.T) {
	throughputDB(t)
	run := createThroughput(t)
	throughputReq(NetworkThroughputCancel, "POST", "/", fmt.Sprintf(`{"id":%d}`, run.ID), "")
	w := throughputReq(NetworkThroughputCreate, "POST", "/", `{"landingId":1,"sourceIds":[2],"protocol":"tcp","direction":"forward","family":4,"duration":5,"streams":1,"rateMbps":10}`, "")
	if w.Code != 409 {
		t.Fatal("reused port before agents could observe cancellation", w.Code, w.Body.String())
	}
}

func TestThroughputPreparationUsesUniqueAcknowledgement(t *testing.T) {
	throughputDB(t)
	request := func() {
		w := throughputReq(NetworkThroughputPrepare, "POST", "/", `{"serverIds":[1]}`, "")
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	request()
	var first models.ThroughputCapability
	models.DB.First(&first, "server_id = ?", 1)
	if first.PrepareID == 0 {
		t.Fatal("missing preparation nonce")
	}
	w := throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"prepare","status":"completed"}`, first.PrepareID), "token1")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	request()
	var second models.ThroughputCapability
	models.DB.First(&second, "server_id = ?", 1)
	if first.PrepareID == second.PrepareID {
		t.Fatal("preparation nonce reused")
	}
	w = throughputReq(AgentThroughputReport, "POST", "/", fmt.Sprintf(`{"id":%d,"role":"prepare","status":"failed","error":"stale result"}`, first.PrepareID), "token1")
	if w.Code != 409 {
		t.Fatal("late prepare report cleared newer request", w.Code)
	}
}
