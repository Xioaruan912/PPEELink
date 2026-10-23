package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestAgentCancelledChecksDoNotReportZeroNodes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nodes := make([]agentCheckNode, 2000)
	for i := range nodes {
		nodes[i] = agentCheckNode{ID: i + 1, Host: "127.0.0.1", Port: 1}
	}
	for _, r := range checkAgentNodes(ctx, nodes, 8, time.Second) {
		if r.NodeID == 0 {
			t.Fatal("cancelled work produced a zero-node result")
		}
	}
}

func TestAgentCheckLoopCancelsPendingHTTP(t *testing.T) {
	entered := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(entered); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { runAgentCheckLoop(ctx, &http.Client{Timeout: time.Second}, server.URL, "test"); close(done) }()
	<-entered
	cancel()
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Error("cancel did not stop the in-flight jobs request")
	}
}

func TestAgentCheckReportRejectsFailedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":"50000","msg":"database busy"}`))
	}))
	defer server.Close()
	if err := reportAgentChecks(context.Background(), server.Client(), server.URL, "test", []agentCheckResult{{NodeID: 1}}); err == nil {
		t.Fatal("HTTP 200 with a failed envelope was treated as acknowledged")
	}
}

func TestAgentPermanentReportErrorDoesNotStallJobRefresh(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var fetches atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if fetches.Add(1) == 2 {
			cancel()
		}
		w.Write([]byte(`{"code":"00000","data":{"nodes":[{"id":1,"host":"127.0.0.1","port":1}],"intervalSeconds":1}}`))
	}))
	defer server.Close()
	done := make(chan struct{})
	go func() { runAgentCheckLoop(ctx, server.Client(), server.URL, "test"); close(done) }()
	select {
	case <-done:
	case <-time.After(1500 * time.Millisecond):
		cancel()
		<-done
		t.Fatal("permanent 400 report error stalled future jobs")
	}
	if fetches.Load() < 2 {
		t.Fatal("did not refresh jobs")
	}
}
