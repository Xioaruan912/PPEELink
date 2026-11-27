package api

import (
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// specWakeHub 管理远端 Server Agent 的配置变更秒级唤醒（防滥用：每台服务器最多 1 个活跃长轮询连接，500ms 变更防抖合并）。
type specWakeHub struct {
	mu           sync.Mutex
	seq          atomic.Uint64
	waiters      map[uint]chan struct{}
	debounceOnce *time.Timer
}

var agentSpecWake = &specWakeHub{
	waiters: make(map[uint]chan struct{}),
}

// BroadcastSpecChange 在代理配置变更后以 500ms 防抖唤醒所有在线等待的 Server Agent。
func BroadcastSpecChange() {
	agentSpecWake.mu.Lock()
	defer agentSpecWake.mu.Unlock()
	if agentSpecWake.debounceOnce != nil {
		return
	}
	agentSpecWake.debounceOnce = time.AfterFunc(500*time.Millisecond, func() {
		agentSpecWake.mu.Lock()
		agentSpecWake.debounceOnce = nil
		agentSpecWake.seq.Add(1)
		for sid, ch := range agentSpecWake.waiters {
			select {
			case ch <- struct{}{}:
			default:
			}
			delete(agentSpecWake.waiters, sid)
		}
		agentSpecWake.mu.Unlock()
	})
}

// AgentSpecWait 供 Server Agent v1.26+ 长轮询等待配置变更。
// GET /api/v1/agent/spec/wait?since=<seq>&timeout=40
// 防滥用保护：单台服务器同时仅允许 1 个活跃等待连接，旧连接自动优雅释放；超时范围限制在 5~45 秒。
func AgentSpecWait(c *gin.Context) {
	s, ok := agentServer(c)
	if !ok {
		return
	}
	since, _ := strconv.ParseUint(c.Query("since"), 10, 64)
	timeoutSec, _ := strconv.Atoi(c.Query("timeout"))
	if timeoutSec < 5 {
		timeoutSec = 40
	}
	if timeoutSec > 45 {
		timeoutSec = 45
	}

	cur := agentSpecWake.seq.Load()
	if since > 0 && cur > since {
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changed": true, "seq": cur}})
		return
	}

	ch := make(chan struct{}, 1)
	agentSpecWake.mu.Lock()
	// 替换同服务器已有等待者，防止连接堆积滥用
	if prev, exists := agentSpecWake.waiters[s.ID]; exists {
		close(prev)
	}
	agentSpecWake.waiters[s.ID] = ch
	agentSpecWake.mu.Unlock()

	defer func() {
		agentSpecWake.mu.Lock()
		if agentSpecWake.waiters[s.ID] == ch {
			delete(agentSpecWake.waiters, s.ID)
		}
		agentSpecWake.mu.Unlock()
	}()

	timer := time.NewTimer(time.Duration(timeoutSec) * time.Second)
	defer timer.Stop()

	select {
	case _, open := <-ch:
		if !open {
			c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changed": false, "seq": agentSpecWake.seq.Load()}})
			return
		}
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changed": true, "seq": agentSpecWake.seq.Load()}})
	case <-timer.C:
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"changed": false, "seq": agentSpecWake.seq.Load()}})
	case <-c.Request.Context().Done():
		return
	}
}
