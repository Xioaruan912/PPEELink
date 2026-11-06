package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/node"
)

func unlockAPIRequest(form url.Values, accept string) (*httptest.ResponseRecorder, *gin.Context) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/v1/nodes/unlock", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Request.Header.Set("Accept", accept)
	return w, c
}
func TestUnlockAPICatalog(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/nodes/unlock/services", nil)
	NodeUnlockServices(c)
	var out struct {
		Data []node.UnlockService `json:"data"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &out); e != nil || w.Code != 200 || len(out.Data) != 18 {
		t.Fatalf("catalog: %d %s %v", w.Code, w.Body.String(), e)
	}
}
func TestUnlockAPIRejectsUnknownFilters(t *testing.T) {
	for _, form := range []url.Values{{"link": {"test://example"}, "service": {"missing"}}, {"link": {"test://example"}, "group": {"missing"}}, {"link": {"test://example"}, "group": {"ai"}, "service": {"netflix"}}} {
		w, c := unlockAPIRequest(form, "")
		NodeUnlock(c)
		if w.Code != 400 {
			t.Fatalf("invalid filter status %d: %s", w.Code, w.Body.String())
		}
	}
}
func TestUnlockAPICachedJSONAndStream(t *testing.T) {
	key := "test://cache-only|openai|ai"
	cached := &node.UnlockResult{NodeName: "Cached", Source: "node", Complete: true, Results: []node.UnlockCheckResult{{Key: "openai", Status: "unknown", Evidence: "api"}}}
	uCache.mu.Lock()
	uCache.entries[key] = &unlockCacheEntry{result: cached, expires: time.Now().Add(time.Minute)}
	uCache.mu.Unlock()
	defer func() { uCache.mu.Lock(); delete(uCache.entries, key); uCache.mu.Unlock() }()
	for _, accept := range []string{"application/json", "text/event-stream"} {
		w, c := unlockAPIRequest(url.Values{"link": {"test://cache-only"}, "service": {"openai"}, "group": {"ai"}}, accept)
		NodeUnlock(c)
		if w.Code != 200 {
			t.Fatalf("cache failed: %d %s", w.Code, w.Body.String())
		}
		if accept == "text/event-stream" && !strings.Contains(w.Body.String(), "event:done") {
			t.Fatalf("missing SSE completion: %s", w.Body.String())
		}
		if accept == "application/json" && !strings.Contains(w.Body.String(), `"nodeName":"自定义节点"`) {
			t.Fatalf("cached result kept another request identity: %s", w.Body.String())
		}
		if cached.NodeName != "Cached" {
			t.Fatal("shared cache result mutated")
		}
		if accept == "application/json" && !strings.Contains(w.Body.String(), `"code":"00000"`) {
			t.Fatalf("JSON client broken: %s", w.Body.String())
		}
	}
}
func TestUnlockAPICancelDoesNotCache(t *testing.T) {
	// An already cancelled request cannot start an expensive detector or write observations.
	w, c := unlockAPIRequest(url.Values{"link": {"test://cancelled"}}, "")
	ctx, cancel := context.WithCancel(c.Request.Context())
	cancel()
	c.Request = c.Request.WithContext(ctx)
	NodeUnlock(c)
	uCache.mu.Lock()
	_, exists := uCache.entries["test://cancelled||"]
	uCache.mu.Unlock()
	if exists || w.Body.Len() != 0 {
		t.Fatalf("cancelled request produced data: %s", w.Body.String())
	}
}

func TestUnlockAPIStreamsItemsAndDoesNotCacheIncompleteRun(t *testing.T) {
	w, c := unlockAPIRequest(url.Values{"link": {"test://partial"}, "group": {"ai"}}, "text/event-stream")
	nodeUnlock(c, func(ctx context.Context, cfg node.UnlockTestConfig) (*node.UnlockResult, error) {
		if cfg.GroupFilter != "ai" || cfg.OnResult == nil {
			t.Fatal("filter or callback lost")
		}
		row := node.UnlockCheckResult{Key: "openai", Name: "ChatGPT", Group: "ai", Status: "unknown", Evidence: "api"}
		cfg.OnResult(row)
		return &node.UnlockResult{Source: "node", Results: []node.UnlockCheckResult{row}, Complete: false, Error: "检测已超时"}, nil
	})
	if !strings.Contains(w.Body.String(), "event:item") || !strings.Contains(w.Body.String(), "event:done") || !w.Flushed {
		t.Fatalf("missing streaming events: %s", w.Body.String())
	}
	uCache.mu.Lock()
	_, exists := uCache.entries["test://partial||ai"]
	uCache.mu.Unlock()
	if exists {
		t.Fatal("incomplete run cached")
	}
	if node.GetTestStatus() != nil {
		t.Fatal("test session not released")
	}
}
