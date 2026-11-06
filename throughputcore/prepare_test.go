package throughputcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assetDigest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func fixtureIPerf(version string) []byte {
	return []byte("#!/bin/sh\ncase \"$1\" in\n --version) printf 'iperf " + version + "\\n';;\n --help) printf '%s\\n' '--rsa-private-key-path --rsa-public-key-path --authorized-users-path --bidir --json-stream';;\nesac\n")
}
func TestPrepareStaticWithoutCompiler(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	payload := fixtureIPerf("3.22")
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
	defer s.Close()
	dest := filepath.Join(t.TempDir(), "iperf3")
	if e := prepareStaticIPerf(context.Background(), s.Client(), iperfAsset{URL: s.URL, SHA256: assetDigest(payload)}, dest); e != nil {
		t.Fatal(e)
	}
	if c := inspectPath(context.Background(), dest); !c.AuthSupported || c.Version != "3.22" {
		t.Fatal(c)
	}
	if data, e := os.ReadFile(dest + ".LICENSE"); e != nil || !strings.Contains(string(data), "OpenSSL") || !strings.Contains(string(data), "musl") {
		t.Fatal("missing licenses", e)
	}
}
func TestPrepareRejectsUnverifiedOrOldBinaryAndPreservesExisting(t *testing.T) {
	for _, version := range []string{"bad-digest", "3.20"} {
		t.Run(version, func(t *testing.T) {
			payload := fixtureIPerf(version)
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
			defer s.Close()
			dest := filepath.Join(t.TempDir(), "iperf3")
			os.WriteFile(dest, []byte("existing tool"), 0755)
			digest := assetDigest(payload)
			if version == "bad-digest" {
				digest = strings.Repeat("0", 64)
			}
			if e := prepareStaticIPerf(context.Background(), s.Client(), iperfAsset{URL: s.URL, SHA256: digest}, dest); e == nil {
				t.Fatal("invalid binary accepted")
			}
			data, _ := os.ReadFile(dest)
			if string(data) != "existing tool" {
				t.Fatal("existing tool replaced")
			}
		})
	}
}
func TestPrepareStaticAPIFallback(t *testing.T) {
	payload := fixtureIPerf("3.22")
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path == "/release" {
			w.WriteHeader(503)
			return
		}
		if r.Header.Get("Accept") != "application/octet-stream" {
			t.Error("missing API binary accept")
		}
		w.Write(payload)
	}))
	defer s.Close()
	if e := prepareStaticIPerf(context.Background(), s.Client(), iperfAsset{URL: s.URL + "/release", APIURL: s.URL + "/api", SHA256: assetDigest(payload)}, filepath.Join(t.TempDir(), "iperf3")); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal(calls)
	}
}
func TestPreparePinnedAssets(t *testing.T) {
	for _, arch := range []string{"amd64", "arm64", "armv7"} {
		a, e := staticIPerfAsset("linux", arch)
		if e != nil || len(a.SHA256) != 64 || !strings.Contains(a.URL, "/3.22/") {
			t.Fatalf("%s: %+v %v", arch, a, e)
		}
	}
	for _, v := range [][2]string{{"windows", "amd64"}, {"linux", "arm"}, {"linux", "386"}} {
		if _, e := staticIPerfAsset(v[0], v[1]); e == nil {
			t.Fatal("unsupported platform accepted", v)
		}
	}
}

func TestPrepareRejectsOversizeAndCancelledDownloads(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(maxIPerfDownload+1))
		w.WriteHeader(200)
	}))
	defer s.Close()
	dest := filepath.Join(t.TempDir(), "iperf3")
	asset := iperfAsset{URL: s.URL, SHA256: strings.Repeat("0", 64)}
	if err := prepareStaticIPerf(context.Background(), s.Client(), asset, dest); err == nil {
		t.Fatal("oversize accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := prepareStaticIPerf(ctx, s.Client(), asset, dest); err == nil {
		t.Fatal("cancelled download accepted")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("failed download installed", err)
	}
}
