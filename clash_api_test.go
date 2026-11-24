package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDetectClashStatusReports401AndConnectsWithSecret(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer 123" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/version":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"version":"v1.19.33"}`))
		case "/proxies":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"proxies":{"日常使用":{"name":"日常使用","type":"Selector","now":"JP-01","all":["JP-01","US-01","DIRECT"]},"JP-01":{"name":"JP-01","type":"Vless"},"US-01":{"name":"US-01","type":"Vless"}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// 1. Without secret: must report explicit 401 message + detected controller URL
	setClashConfig(srv.URL, "")
	st := detectClashStatus(context.Background())
	if st.Connected {
		t.Fatalf("expected connected=false without secret, got %+v", st)
	}
	if st.Controller != srv.URL || !strings.Contains(st.Error, "401") {
		t.Fatalf("expected 401 auth error with controller=%s, got %+v", srv.URL, st)
	}

	// 2. With correct secret: must connect and list real proxies
	setClashConfig(srv.URL, "123")
	st = detectClashStatus(context.Background())
	if !st.Connected || st.Group != "日常使用" || st.Current != "JP-01" || len(st.Proxies) != 2 {
		t.Fatalf("expected connected Clash status with 2 proxies, got %+v", st)
	}
}
