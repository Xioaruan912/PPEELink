package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"ppeelink/throughputcore"
	"sync/atomic"
	"testing"
	"time"
)

func TestThroughputFetchDesiredAndAuth(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Agent secret" {
			t.Error("token missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": "00000", "data": map[string]any{"jobs": []throughputcore.Job{{ID: 7, Role: "client"}}}})
	}))
	defer s.Close()
	jobs, e := fetchAgentThroughputJobs(context.Background(), s.Client(), s.URL, "secret")
	if e != nil || len(jobs) != 1 || jobs[0].ID != 7 {
		t.Fatalf("%+v %v", jobs, e)
	}
}
func TestThroughputReportRetriesTransientFailure(t *testing.T) {
	var attempts atomic.Int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(200)
	}))
	defer s.Close()
	e := postAgentThroughput(context.Background(), s.Client(), s.URL, "secret", "report", throughputcore.Report{ID: 1, Role: "server", Status: "ready"})
	if e != nil || attempts.Load() != 3 {
		t.Fatalf("report not retried: attempts=%d error=%v", attempts.Load(), e)
	}
}
func TestThroughputFinishedReportPersistsUntilAcknowledged(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
 --version) echo 'iperf 3.22';exit 0;;
 --help) echo '--rsa-private-key-path --rsa-public-key-path --authorized-users-path --bidir --json-stream';exit 0;;
esac
echo '{"event":"end","data":{"sum_received":{"bits_per_second":98000000,"bytes":100}}}'
`
	if e := os.WriteFile(filepath.Join(dir, "iperf3"), []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	var completed, running atomic.Int32
	received := make(chan struct{}, 1)
	j := throughputcore.Job{ID: 9, Role: "client", Host: "127.0.0.1", Port: 35219, Protocol: "tcp", Direction: "forward", Family: 4, Duration: 5, Streams: 1, RateMbps: 100, Username: "runner", Password: "secret", PublicKey: "test", ExpiresAt: time.Now().Add(time.Minute)}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/agent/throughput/jobs" {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": "00000", "data": map[string]any{"jobs": []throughputcore.Job{j}}})
			return
		}
		if r.URL.Path == "/api/v1/agent/throughput/report" {
			var v throughputcore.Report
			_ = json.NewDecoder(r.Body).Decode(&v)
			if v.Status == "running" {
				running.Add(1)
			}
			if v.Status == "completed" {
				if completed.Add(1) <= 3 {
					w.WriteHeader(503)
					return
				}
				select {
				case received <- struct{}{}:
				default:
				}
			}
		}
		w.WriteHeader(200)
	}))
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { runAgentThroughputLoop(ctx, s.Client(), s.URL, "secret"); close(done) }()
	select {
	case <-received:
	case <-time.After(6 * time.Second):
		cancel()
		<-done
		t.Fatal("completed result was lost after initial retry budget")
	}
	cancel()
	<-done
	if running.Load() != 1 {
		t.Fatalf("finished job executed %d times", running.Load())
	}
}
