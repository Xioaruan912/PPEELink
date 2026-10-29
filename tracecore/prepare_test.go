package tracecore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNextTracePinnedAssets(t *testing.T) {
	for arch, want := range map[string]string{"amd64": "25100992ad8924d6bfc7724eb6cc98547d87ec13828aa9b354516687331e170c", "arm64": "84e8dee2115263e3627580382dcb355cf364d5cc4fd4a2a106ac2fd989476834"} {
		a, err := nextTraceAsset("linux", arch)
		if err != nil || a.SHA256 != want || !strings.Contains(a.URL, "/v1.7.4/nexttrace_linux_"+arch) {
			t.Fatalf("%+v %v", a, err)
		}
	}
	if _, err := nextTraceAsset("linux", "mips"); err == nil {
		t.Fatal("unsupported architecture accepted")
	}
}
func TestNextTracePrepareChecksumAtomicAndVersion(t *testing.T) {
	payload := []byte("#!/bin/sh\necho NextTrace v1.7.4\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload) }))
	defer server.Close()
	dir := t.TempDir()
	dest := filepath.Join(dir, "nexttrace")
	os.WriteFile(dest, []byte("old"), 0755)
	hash := sha256.Sum256(payload)
	a := traceAsset{URL: server.URL, SHA256: strings.Repeat("0", 64)}
	if err := prepareNextTrace(context.Background(), server.Client(), a, dest); err == nil {
		t.Fatal("checksum mismatch accepted")
	}
	if b, _ := os.ReadFile(dest); string(b) != "old" {
		t.Fatal("old binary replaced before verification")
	}
	a.SHA256 = hex.EncodeToString(hash[:])
	if err := prepareNextTrace(context.Background(), server.Client(), a, dest); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dest); string(b) != string(payload) {
		t.Fatal("verified binary not installed")
	}
	payload = []byte("#!/bin/sh\necho NextTrace v1.7.3\n")
	hash = sha256.Sum256(payload)
	a.SHA256 = hex.EncodeToString(hash[:])
	if err := prepareNextTrace(context.Background(), server.Client(), a, dest); err == nil {
		t.Fatal("wrong version installed")
	}
	files, _ := os.ReadDir(dir)
	if len(files) != 1 {
		t.Fatal("download temporary files leaked")
	}
}

func TestNextTracePrepareOfficialAPIFallback(t *testing.T) {
	payload := []byte("#!/bin/sh\necho NextTrace v1.7.4\n")
	hash := sha256.Sum256(payload)
	apiCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/release" {
			http.Error(w, "release download unavailable", 502)
			return
		}
		apiCalled = true
		if r.Header.Get("Accept") != "application/octet-stream" || r.URL.Query().Get("subl_download") == "" {
			t.Error("API fallback missing octet-stream/cache bust")
		}
		w.Write(payload)
	}))
	defer server.Close()
	asset := traceAsset{URL: server.URL + "/release", APIURL: server.URL + "/asset", SHA256: hex.EncodeToString(hash[:])}
	if err := prepareNextTrace(context.Background(), server.Client(), asset, filepath.Join(t.TempDir(), "nexttrace")); err != nil {
		t.Fatal(err)
	}
	if !apiCalled {
		t.Fatal("API fallback not used")
	}
}
