package api

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/node"
)

type unlockCache struct {
	mu      sync.Mutex
	entries map[string]*unlockCacheEntry
}
type unlockCacheEntry struct {
	result  *node.UnlockResult
	expires time.Time
}

var uCache = &unlockCache{entries: make(map[string]*unlockCacheEntry)}

func NodeUnlockServices(c *gin.Context) {
	c.JSON(200, gin.H{"code": "00000", "data": node.ListUnlockServices()})
}

func unlockStream(c *gin.Context, event string, data any) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.SSEvent(event, data)
	c.Writer.Flush()
}
func writeUnlockResult(c *gin.Context, result *node.UnlockResult, stream bool) {
	if stream {
		unlockStream(c, "done", result)
	} else {
		c.JSON(200, gin.H{"code": "00000", "data": result, "msg": "解锁检测完成"})
	}
}

// NodeUnlock retains the JSON contract; Accept: text/event-stream receives item/done events.
func NodeUnlock(c *gin.Context) {
	nodeUnlock(c, node.RunUnlockTest)
}

func nodeUnlock(c *gin.Context, run func(context.Context, node.UnlockTestConfig) (*node.UnlockResult, error)) {
	if c.Request.Context().Err() != nil {
		return
	}
	idStr := c.PostForm("id")
	link := c.PostForm("link")
	service := strings.TrimSpace(c.PostForm("service"))
	group := strings.TrimSpace(c.PostForm("group"))
	targets, err := node.SelectUnlockServices(service, group)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	if idStr == "" && link == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "需要提供节点 id 或 link"})
		return
	}
	nodeName := "自定义节点"
	nodeID := 0
	if idStr != "" {
		id, err := strconv.Atoi(idStr)
		if err != nil || id <= 0 {
			c.JSON(400, gin.H{"code": "40000", "msg": "节点 id 格式错误"})
			return
		}
		var n models.Node
		n.ID = id
		if err := n.Find(); err != nil {
			c.JSON(404, gin.H{"code": "40400", "msg": "节点不存在"})
			return
		}
		nodeName = n.Name
		nodeID = n.ID
		link = n.Link
	}
	if link == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点链接为空"})
		return
	}
	stream := strings.Contains(c.GetHeader("Accept"), "text/event-stream")
	cacheKey := link + "|" + service + "|" + group
	uCache.mu.Lock()
	entry := uCache.entries[cacheKey]
	if entry != nil && time.Now().Before(entry.expires) {
		uCache.mu.Unlock()
		cached := *entry.result
		cached.NodeName = nodeName
		writeUnlockResult(c, &cached, stream)
		return
	}
	uCache.mu.Unlock()
	ctx, _, ok := node.BeginTest(nodeName, nodeID, "unlock", c.Request.Context())
	if !ok {
		c.JSON(429, gin.H{"code": "42900", "msg": "已有测试进行中，请稍候"})
		return
	}
	defer node.EndTest()
	var onResult func(node.UnlockCheckResult)
	if stream {
		unlockStream(c, "start", gin.H{"source": "node", "nodeName": nodeName, "total": len(targets)})
		onResult = func(row node.UnlockCheckResult) { unlockStream(c, "item", row) }
	}
	result, err := run(ctx, node.UnlockTestConfig{Link: link, Timeout: 8 * time.Second, ServiceFilter: service, GroupFilter: group, OnResult: onResult})
	if ctx.Err() == context.Canceled || c.Request.Context().Err() != nil {
		return
	}
	if err != nil {
		if stream {
			unlockStream(c, "error", gin.H{"msg": "解锁测试失败: " + err.Error()})
		} else {
			c.JSON(500, gin.H{"code": "50000", "msg": "解锁测试失败: " + err.Error()})
		}
		return
	}
	result.NodeName = nodeName
	if result.Complete && result.Error == "" {
		if nodeID > 0 {
			observations := make([]models.UnlockObservation, 0, len(result.Results))
			for _, check := range result.Results {
				observations = append(observations, models.UnlockObservation{NodeID: nodeID, Service: check.Key, Available: check.Ok, Status: check.Status, Region: check.Region, Rtt: check.Rtt, CheckedAt: check.CheckedAt})
			}
			if len(observations) > 0 {
				_ = models.DB.Create(&observations).Error
			}
		}
		uCache.mu.Lock()
		for key, e := range uCache.entries {
			if time.Now().After(e.expires) {
				delete(uCache.entries, key)
			}
		}
		uCache.entries[cacheKey] = &unlockCacheEntry{result: result, expires: time.Now().Add(60 * time.Second)}
		uCache.mu.Unlock()
	}
	writeUnlockResult(c, result, stream)
}
