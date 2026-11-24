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

// TestProbeV14BidirectionalTraceEndToEnd verifies:
// 1) Nginx X-Real-IP populates Probe.PublicIP when query param is empty;
// 2) Bidirectional trace dispatches both User Agent outbound and Server Agent return trace;
// 3) AgentTraceReport ("completed" status) unblocks ProbeBidirectionalTrace with parsed return hops.
func TestProbeV14BidirectionalTraceEndToEnd(t *testing.T) {
	setupTestDB(t)
	if err := models.DB.AutoMigrate(
		&models.Probe{},
		&models.Node{},
		&models.ProxyServer{},
		&models.ProxyInbound{},
		&models.NetworkTraceRun{},
		&models.NetworkTraceView{},
		&models.TraceAgentCapability{},
	); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	plainProbeToken := "bi-probe-token"
	probe := models.Probe{
		Name:       "福州",
		TokenHash:  models.HashProbeToken(plainProbeToken),
		Enabled:    true,
		LastSeenAt: &now,
	}
	if err := models.DB.Create(&probe).Error; err != nil {
		t.Fatal(err)
	}

	agentToken := "bi-agent-token"
	srv := models.ProxyServer{
		Name:       "DMIT_USA",
		Kind:       models.ServerKindAgent,
		PublicHost: "179.255.148.213",
		AgentToken: models.HashAgentToken(agentToken),
		Enabled:    true,
		LastSeenAt: &now,
	}
	if err := models.DB.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}

	nd := models.Node{
		Name: "DMIT_USA",
		Link: "vless://11111111-1111-1111-1111-111111111111@179.255.148.213:38614?security=reality#DMIT_USA",
	}
	if err := models.DB.Create(&nd).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middlewares.AuthorToken)
	r.GET("/api/v1/probe/wait", ProbeWait)
	r.POST("/api/v1/probe/trace-report", UserProbeTraceReport)
	r.GET("/api/v1/agent/trace/jobs", AgentTraceJobs)
	r.POST("/api/v1/agent/trace/report", AgentTraceReport)

	// 1. User Agent polls behind Nginx (RemoteAddr 127.0.0.1, X-Real-IP 211.80.216.99) without ?publicIp=.
	outDone := make(chan struct{}, 1)
	go func() {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/probe/wait?timeout=5&version=1.5", nil)
		req.RemoteAddr = "127.0.0.1:54321"
		req.Header.Set("X-Real-IP", "211.80.216.99")
		req.Header.Set("Authorization", "Probe "+plainProbeToken)
		r.ServeHTTP(w, req)

		var env struct {
			Data struct {
				JobID  string `json:"jobId"`
				Action string `json:"action"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &env)
		if env.Data.Action == "trace" && env.Data.JobID != "" {
			payload, _ := json.Marshal(map[string]any{
				"jobId": env.Data.JobID,
				"host":  "179.255.148.213",
				"hops": []userTraceHop{
					{TTL: 1, IP: "10.255.251.17", RTT: 1},
					{TTL: 2, IP: "59.43.80.1", RTT: 28},
					{TTL: 3, IP: "179.255.148.213", RTT: 145},
				},
			})
			rw := httptest.NewRecorder()
			rreq := httptest.NewRequest(http.MethodPost, "/api/v1/probe/trace-report", bytes.NewReader(payload))
			rreq.Header.Set("Authorization", "Probe "+plainProbeToken)
			rreq.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(rw, rreq)
			outDone <- struct{}{}
		}
	}()

	// 2. Simulated Server Agent polls /api/v1/agent/trace/jobs and reports completed traceroute output.
	retDone := make(chan struct{}, 1)
	go func() {
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			jw := httptest.NewRecorder()
			jreq := httptest.NewRequest(http.MethodGet, "/api/v1/agent/trace/jobs?version=2&tool=nexttrace", nil)
			jreq.Header.Set("Authorization", "Agent "+agentToken)
			r.ServeHTTP(jw, jreq)
			var jenv struct {
				Data []struct {
					ID     uint   `json:"id"`
					Target string `json:"target"`
				} `json:"data"`
			}
			_ = json.Unmarshal(jw.Body.Bytes(), &jenv)
			if len(jenv.Data) > 0 {
				rep, _ := json.Marshal(map[string]any{
					"id":     jenv.Data[0].ID,
					"tool":   "traceroute",
					"output": "1  59.43.130.1  12.4 ms\n2  211.80.216.99  142.1 ms\n",
				})
				rw := httptest.NewRecorder()
				rreq := httptest.NewRequest(http.MethodPost, "/api/v1/agent/trace/report", bytes.NewReader(rep))
				rreq.Header.Set("Authorization", "Agent "+agentToken)
				rreq.Header.Set("Content-Type", "application/json")
				r.ServeHTTP(rw, rreq)
				retDone <- struct{}{}
				return
			}
			time.Sleep(80 * time.Millisecond)
		}
	}()

	time.Sleep(120 * time.Millisecond)
	biW := httptest.NewRecorder()
	bc, _ := gin.CreateTestContext(biW)
	body, _ := json.Marshal(map[string]any{"nodeId": nd.ID})
	bc.Request = httptest.NewRequest(http.MethodPost, "/api/v1/probes/trace-bidirectional", bytes.NewReader(body))
	bc.Request.Header.Set("Content-Type", "application/json")
	ProbeBidirectionalTrace(bc)

	if biW.Code != http.StatusOK {
		t.Fatalf("ProbeBidirectionalTrace failed: %d %s", biW.Code, biW.Body.String())
	}
	var resp struct {
		Data struct {
			ClientIP     string `json:"clientIp"`
			ReturnServer string `json:"returnServer"`
			Verdict      string `json:"verdict"`
			Outbound     struct {
				Hops  []userTraceHop `json:"hops"`
				Line  string         `json:"line"`
				Error string         `json:"error"`
			} `json:"outbound"`
			ReturnPath struct {
				Hops  []userTraceHop `json:"hops"`
				Line  string         `json:"line"`
				Error string         `json:"error"`
			} `json:"returnPath"`
		} `json:"data"`
	}
	if err := json.Unmarshal(biW.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v (%s)", err, biW.Body.String())
	}
	if resp.Data.ClientIP != "211.80.216.99" {
		t.Fatalf("expected clientIp 211.80.216.99 from X-Real-IP, got %q", resp.Data.ClientIP)
	}
	if resp.Data.ReturnServer != "DMIT_USA" {
		t.Fatalf("expected returnServer DMIT_USA, got %q", resp.Data.ReturnServer)
	}
	if resp.Data.ReturnPath.Error != "" || len(resp.Data.ReturnPath.Hops) < 2 {
		t.Fatalf("expected returnPath hops without error, got err=%q hops=%+v", resp.Data.ReturnPath.Error, resp.Data.ReturnPath.Hops)
	}
	if resp.Data.Outbound.Line != "CN2 GIA" || resp.Data.ReturnPath.Line != "CN2 GIA" {
		t.Fatalf("expected CN2 GIA both ways, got out=%q ret=%q verdict=%q", resp.Data.Outbound.Line, resp.Data.ReturnPath.Line, resp.Data.Verdict)
	}
	<-outDone
	<-retDone
}
