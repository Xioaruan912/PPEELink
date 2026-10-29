package throughputcore

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func authenticatedJob(t *testing.T) Job {
	t.Helper()
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	pub, e := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if e != nil {
		t.Fatal(e)
	}
	j := testJob()
	j.Streams = 1
	j.PrivateKey = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	j.PublicKey = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}))
	j.RateMbps = 10
	return j
}
func freePort(t *testing.T) int {
	t.Helper()
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	p := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return p
}
func TestRealAuthenticatedMeasurements(t *testing.T) {
	if os.Getenv("THROUGHPUT_REAL") != "1" {
		t.Skip("set THROUGHPUT_REAL=1 with iperf3 >=3.22")
	}
	c := Inspect(context.Background())
	if !c.AuthSupported {
		t.Fatal(c)
	}
	for _, protocol := range []string{"tcp", "udp"} {
		for _, direction := range []string{"forward", "reverse", "bidir"} {
			t.Run(protocol+"/"+direction, func(t *testing.T) {
				j := authenticatedJob(t)
				j.Port = freePort(t)
				j.Protocol = protocol
				j.Direction = direction
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
				defer cancel()
				server := j
				server.Role = "server"
				ready := make(chan Report, 4)
				serverDone := make(chan struct{})
				go func() { Run(ctx, server, func(r Report) { ready <- r }); close(serverDone) }()
				r := <-ready
				if r.Status != "ready" {
					t.Fatalf("listener: %+v", r)
				}
				var result Report
				Run(ctx, j, func(r Report) { result = r })
				if result.Status != "completed" || result.Result == nil {
					t.Fatalf("client: %+v", result)
				}
				if (direction != "reverse" && (result.Result.Forward == nil || result.Result.Forward.BitsPerSecond <= 0 || len(result.Result.Forward.Intervals) == 0)) || (direction != "forward" && (result.Result.Reverse == nil || result.Result.Reverse.BitsPerSecond <= 0 || len(result.Result.Reverse.Intervals) == 0)) {
					t.Fatalf("receiver result: %+v forward=%+v reverse=%+v", result.Result, result.Result.Forward, result.Result.Reverse)
				}
				select {
				case <-serverDone:
				case <-ctx.Done():
					t.Fatal("listener retained after one-off measurement")
				}
				t.Logf("%s/%s cap=%v", protocol, direction, result.Result.CapReached)
			})
		}
	}
}
func TestRealRejectCredentialsAndCancel(t *testing.T) {
	if os.Getenv("THROUGHPUT_REAL") != "1" {
		t.Skip("explicit integration test")
	}
	j := authenticatedJob(t)
	j.Port = freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := j
	server.Role = "server"
	ready := make(chan Report, 4)
	done := make(chan struct{})
	go func() { Run(ctx, server, func(r Report) { ready <- r }); close(done) }()
	if r := <-ready; r.Status != "ready" {
		t.Fatal(r)
	}
	j.Password = "invalid"
	var result Report
	Run(ctx, j, func(r Report) { result = r })
	if result.Status != "failed" {
		t.Fatal("invalid password was accepted")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("server ignored cancellation")
	}
	l, e := net.Listen("tcp4", "127.0.0.1:"+strings.TrimSpace(portString(j.Port)))
	if e != nil {
		t.Fatal("listener not released", e)
	}
	l.Close()
}
func portString(p int) string { return strconv.Itoa(p) }
func TestChildCancelRemovesSecrets(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/sh
case "$1" in
 --version) echo 'iperf 3.22'; exit 0;;
 --help) echo '--rsa-private-key-path --rsa-public-key-path --authorized-users-path --bidir --json-stream'; exit 0;;
esac
sleep 30
`
	p := filepath.Join(dir, "iperf3")
	if e := os.WriteFile(p, []byte(script), 0700); e != nil {
		t.Fatal(e)
	}
	if _, e := exec.LookPath("sleep"); e != nil {
		t.Skip("shell sleep unavailable")
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	j := testJob()
	ctx, cancel := context.WithCancel(context.Background())
	var report Report
	done := make(chan struct{})
	go func() {
		Run(ctx, j, func(r Report) {
			if r.Status == "running" {
				time.Sleep(100 * time.Millisecond)
				cancel()
			}
			report = r
		})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("child not reaped")
	}
	if report.Status != "failed" {
		t.Fatal(report)
	}
	entries, e := os.ReadDir(tmp)
	if e != nil || len(entries) != 0 {
		t.Fatalf("private files retained: %v %v", entries, e)
	}
}
func TestRealIPv6AuthenticatedMeasurement(t *testing.T) {
	if os.Getenv("THROUGHPUT_REAL") != "1" {
		t.Skip("explicit integration test")
	}
	l, e := net.Listen("tcp6", "[::1]:0")
	if e != nil {
		t.Skip("IPv6 loopback unavailable")
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	j := authenticatedJob(t)
	j.Host = "::1"
	j.Family = 6
	j.Port = port
	server := j
	server.Role = "server"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ready := make(chan Report, 4)
	done := make(chan struct{})
	go func() { Run(ctx, server, func(r Report) { ready <- r }); close(done) }()
	if r := <-ready; r.Status != "ready" {
		t.Fatal(r)
	}
	var result Report
	Run(ctx, j, func(r Report) { result = r })
	if result.Status != "completed" || result.Result == nil || result.Result.Forward == nil || result.Result.Forward.BitsPerSecond <= 0 {
		t.Fatalf("%+v", result)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}
