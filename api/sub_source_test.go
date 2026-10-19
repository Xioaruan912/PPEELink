package api

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"ppeelink/models"
)

func TestParseSubscriptionUserinfo(t *testing.T) {
	up, down, total, expire, ok := parseSubscriptionUserinfo("upload=1024; download=2048;total=10737418240; expire=1767225600")
	if !ok || up != 1024 || down != 2048 || total != 10737418240 || expire == nil || expire.Unix() != 1767225600 {
		t.Fatalf("unexpected parse: %d %d %d %v %v", up, down, total, expire, ok)
	}
	if _, _, _, _, ok := parseSubscriptionUserinfo(""); ok {
		t.Fatal("empty header must not be ok")
	}
}

// upstream 返回 base64 链接列表的测试上游，body 可在测试中替换。
func upstream(t *testing.T, links *atomic.Value) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Subscription-Userinfo", "upload=5; download=7; total=100; expire=0")
		_, _ = w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(links.Load().(string)))))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("SUBLINKX_ALLOW_PRIVATE_FETCH", "1")
	return srv.URL
}

func TestSubscriptionSourceSyncKeepsIDsAndRemovesStale(t *testing.T) {
	setupTestDB(t)
	var links atomic.Value
	links.Store(testLinkA + "\n" + testLinkB)
	url := upstream(t, &links)

	pull, err := pullSubscriptionSource("http://127.0.0.1:1/dead\n" + url)
	if err != nil || len(pull.Nodes) != 2 {
		t.Fatalf("fallback source must be used: %v %d", err, len(pull.Nodes))
	}
	sub := models.Subcription{Name: "Up", SourceURLs: url}
	models.DB.Create(&sub)
	if err := applySubscriptionSource(sub.ID, pull); err != nil {
		t.Fatal(err)
	}
	var first models.Subcription
	models.DB.Preload("Nodes").First(&first, sub.ID)
	if len(first.Nodes) != 2 || first.SourceTotal != 100 || first.SourceTrafficAt == nil {
		t.Fatalf("first sync wrong: %d nodes, total %d", len(first.Nodes), first.SourceTotal)
	}
	keepID := 0
	for _, n := range first.Nodes {
		if n.Link == testLinkA {
			keepID = n.ID
		}
	}

	// 上游只剩 A：A 沿用原 ID，B 不再被引用而被删除
	links.Store(testLinkA)
	if n, err := SyncSubscriptionSource(sub.ID); err != nil || n != 1 {
		t.Fatalf("resync: %v %d", err, n)
	}
	var second models.Subcription
	models.DB.Preload("Nodes").First(&second, sub.ID)
	if len(second.Nodes) != 1 || second.Nodes[0].ID != keepID {
		t.Fatalf("node identity not kept: %+v", second.Nodes)
	}
	var total int64
	models.DB.Model(&models.Node{}).Count(&total)
	if total != 1 {
		t.Fatalf("stale node must be removed, %d left", total)
	}
	var group models.GroupNode
	models.DB.Preload("Nodes").Where("name = ?", "Up").First(&group)
	if len(group.Nodes) != 1 {
		t.Fatalf("group must mirror upstream: %d", len(group.Nodes))
	}

	// 上游失败：保留节点并记录错误
	models.DB.Model(&sub).Update("source_urls", "http://127.0.0.1:1/dead")
	if _, err := SyncSubscriptionSource(sub.ID); err == nil {
		t.Fatal("dead upstream must fail")
	}
	var third models.Subcription
	models.DB.Preload("Nodes").First(&third, sub.ID)
	if len(third.Nodes) != 1 || !strings.Contains(third.LastSyncError, "127.0.0.1") {
		t.Fatalf("failure must keep nodes and record error: %d %q", len(third.Nodes), third.LastSyncError)
	}
}
