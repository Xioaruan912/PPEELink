package middlewares

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSecurityHeadersMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders)
	r.GET("/api/v1/ping", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/static/app.js", func(c *gin.Context) { c.String(http.StatusOK, "console.log(1)") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	r.ServeHTTP(w, req)
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("missing nosniff header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("missing DENY frame header")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("expected no-store on /api/ response, got %q", w.Header().Get("Cache-Control"))
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/static/app.js", nil)
	r.ServeHTTP(w2, req2)
	if w2.Header().Get("Cache-Control") == "no-store" {
		t.Fatalf("static asset should not inherit API no-store cache header")
	}
}

func TestTokenRevocationAndTouchThrottle(t *testing.T) {
	tok := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJhZG1pbiJ9.sig"
	if isTokenRevoked(tok) {
		t.Fatal("fresh token should not be revoked")
	}
	RevokeToken(tok, time.Now().Add(time.Minute))
	if !isTokenRevoked(tok) {
		t.Fatal("revoked token should be rejected")
	}

	now := time.Now()
	if !shouldTouchAPIToken(99901, now) {
		t.Fatal("first touch should write")
	}
	if shouldTouchAPIToken(99901, now.Add(15*time.Second)) {
		t.Fatal("second touch within 60s should be throttled")
	}
	if !shouldTouchAPIToken(99901, now.Add(65*time.Second)) {
		t.Fatal("touch after 60s should write")
	}
}

func TestStaticFSPooledGzipAndPathSafety(t *testing.T) {
	gin.SetMode(gin.TestMode)
	payload := strings.Repeat("export const hello = 'world';\n", 40)
	memFS := fstest.MapFS{
		"js/app.js": &fstest.MapFile{Data: []byte(payload)},
	}
	r := gin.New()
	r.Any("/static/*filepath", StaticFS(memFS))

	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/static/js/app.js", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		if w.Header().Get("Content-Encoding") != "gzip" || w.Header().Get("Vary") != "Accept-Encoding" {
			t.Fatalf("missing gzip/Vary headers: %#v", w.Header())
		}
		gr, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("invalid gzip body: %v", err)
		}
		raw, _ := io.ReadAll(gr)
		_ = gr.Close()
		if string(raw) != payload {
			t.Fatalf("decompressed mismatch on iter %d", i)
		}
	}

	wBad := httptest.NewRecorder()
	reqBad := httptest.NewRequest(http.MethodGet, "/static/../../etc/passwd", nil)
	r.ServeHTTP(wBad, reqBad)
	if wBad.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for traversal, got %d", wBad.Code)
	}
}
