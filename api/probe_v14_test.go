package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"ppeelink/middlewares"
	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

// TestProbeV14UnlockAndSpeedtestThroughJWTMiddleware verifies both routes bypass
// JWT under /api/v1/probe/, enforce Probe token auth, and complete end-to-end.
func TestProbeV14UnlockAndSpeedtestThroughJWTMiddleware(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(&models.Probe{}, &models.ProbeResult{}, &models.Node{}); err != nil {
		t.Fatal(err)
	}
	plainToken := "test-probe-token-v14"
	now := time.Now()
	probe := models.Probe{
		Name:       "福州测试机",
		TokenHash:  models.HashProbeToken(plainToken),
		Enabled:    true,
		LastSeenAt: &now,
	}
	if err := models.DB.Create(&probe).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.AuthorToken)
	r.GET("/api/v1/probe/wait", ProbeWait)
	r.POST("/api/v1/probe/unlock-report", UserProbeUnlockReport)
	r.POST("/api/v1/probe/speedtest-report", UserProbeSpeedtestReport)

	// 1. Verify unauthorized request to /api/v1/probe/unlock-report returns 401 (not 400 JWT missing).
	unauth := httptest.NewRecorder()
	req0 := httptest.NewRequest(http.MethodPost, "/api/v1/probe/unlock-report", bytes.NewBufferString(`{}`))
	r.ServeHTTP(unauth, req0)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 from Probe token check, got %d: %s", unauth.Code, unauth.Body.String())
	}

	// 2. End-to-end Unlock roundtrip through real probeHub + middleware.
	waitDone := make(chan string, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/probe/wait?timeout=5&version=1.4&publicIp=1.2.3.4", nil)
		req.Header.Set("Authorization", "Probe "+plainToken)
		r.ServeHTTP(w, req)
		var env struct {
			Data struct {
				JobID  string `json:"jobId"`
				Action string `json:"action"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Data.Action == "unlock" && env.Data.JobID != "" {
			payload, _ := json.Marshal(userUnlockReport{
				JobID:  env.Data.JobID,
				Egress: userUnlockEgress{IP: "8.8.8.8", Loc: "US", DirectIP: "1.2.3.4"},
				Items:  []userUnlockItem{{Key: "netflix", Name: "Netflix", Status: "available", Region: "US", OK: true}},
			})
			rw := httptest.NewRecorder()
			rreq := httptest.NewRequest(http.MethodPost, "/api/v1/probe/unlock-report", bytes.NewReader(payload))
			rreq.Header.Set("Authorization", "Probe "+plainToken)
			rreq.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(rw, rreq)
			waitDone <- env.Data.JobID
		}
	}()

	time.Sleep(60 * time.Millisecond)
	adminW := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(adminW)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/probes/unlock", bytes.NewBufferString(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")
	ProbeUnlock(c)

	if adminW.Code != http.StatusOK {
		t.Fatalf("ProbeUnlock failed: %d %s", adminW.Code, adminW.Body.String())
	}
	select {
	case <-waitDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for probe wait goroutine")
	}

	// 3. End-to-end Speedtest roundtrip.
	stDone := make(chan string, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/probe/wait?timeout=5&version=1.4", nil)
		req.Header.Set("Authorization", "Probe "+plainToken)
		r.ServeHTTP(w, req)
		var env struct {
			Data struct {
				JobID     string `json:"jobId"`
				Action    string `json:"action"`
				SpeedMode string `json:"speedMode"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Data.Action == "speedtest" && env.Data.SpeedMode == "both" {
			payload, _ := json.Marshal(userSpeedtestReport{
				JobID:  env.Data.JobID,
				Mode:   "both",
				Proxy:  &userSpeedChannel{Mode: "proxy", DownloadMbps: 180, UploadMbps: 40, TTFBMs: 65},
				Direct: &userSpeedChannel{Mode: "direct", DownloadMbps: 200, UploadMbps: 50, TTFBMs: 12},
			})
			rw := httptest.NewRecorder()
			rreq := httptest.NewRequest(http.MethodPost, "/api/v1/probe/speedtest-report", bytes.NewReader(payload))
			rreq.Header.Set("Authorization", "Probe "+plainToken)
			rreq.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(rw, rreq)
			stDone <- env.Data.JobID
		}
	}()

	time.Sleep(60 * time.Millisecond)
	stW := httptest.NewRecorder()
	sc, _ := gin.CreateTestContext(stW)
	sc.Request = httptest.NewRequest(http.MethodPost, "/api/v1/probes/speedtest", bytes.NewBufferString(`{"mode":"both"}`))
	sc.Request.Header.Set("Content-Type", "application/json")
	ProbeSpeedtest(sc)

	if stW.Code != http.StatusOK || !bytes.Contains(stW.Body.Bytes(), []byte(`"retentionPct":90`)) {
		t.Fatalf("ProbeSpeedtest unexpected response: %d %s", stW.Code, stW.Body.String())
	}
	<-stDone
}
