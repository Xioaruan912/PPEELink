package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

type serverOpsJob struct {
	JobID   string `json:"jobId"`
	Action  string `json:"action"` // tune-network | upgrade-core | fetch-logs | check-port
	Source  string `json:"source,omitempty"`
	Lines   int    `json:"lines,omitempty"`
	Filter  string `json:"filter,omitempty"`
	Version string `json:"version,omitempty"`
	Port    int    `json:"port,omitempty"`
	Proto   string `json:"proto,omitempty"`
}

type serverOpsResult struct {
	JobID  string `json:"jobId"`
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
	InUse  bool   `json:"inUse,omitempty"`
	BBR    string `json:"bbr,omitempty"`
	Qdisc  string `json:"qdisc,omitempty"`
	Rmem   int64  `json:"rmemMax,omitempty"`
	Wmem   int64  `json:"wmemMax,omitempty"`
}

var serverOpsHub = &struct {
	mu      sync.Mutex
	waiters map[uint]chan serverOpsJob
	pending map[uint][]serverOpsJob
	results map[string]chan serverOpsResult
}{
	waiters: map[uint]chan serverOpsJob{},
	pending: map[uint][]serverOpsJob{},
	results: map[string]chan serverOpsResult{},
}

func opsRegister(serverID uint) chan serverOpsJob {
	serverOpsHub.mu.Lock()
	defer serverOpsHub.mu.Unlock()
	ch := make(chan serverOpsJob, 1)
	if q := serverOpsHub.pending[serverID]; len(q) > 0 {
		ch <- q[0]
		if len(q) == 1 {
			delete(serverOpsHub.pending, serverID)
		} else {
			serverOpsHub.pending[serverID] = q[1:]
		}
		return ch
	}
	serverOpsHub.waiters[serverID] = ch
	return ch
}

func opsUnregister(serverID uint, ch chan serverOpsJob) {
	serverOpsHub.mu.Lock()
	defer serverOpsHub.mu.Unlock()
	if cur, ok := serverOpsHub.waiters[serverID]; ok && cur == ch {
		delete(serverOpsHub.waiters, serverID)
	}
}

func opsDispatch(serverID uint, job serverOpsJob, allowQueue bool) (string, chan serverOpsResult, bool) {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	job.JobID = hex.EncodeToString(b)
	resCh := make(chan serverOpsResult, 1)

	serverOpsHub.mu.Lock()
	ch, waiting := serverOpsHub.waiters[serverID]
	if waiting {
		delete(serverOpsHub.waiters, serverID)
		serverOpsHub.results[job.JobID] = resCh
		serverOpsHub.mu.Unlock()
		select {
		case ch <- job:
			return job.JobID, resCh, true
		default:
			serverOpsHub.mu.Lock()
			if allowQueue && len(serverOpsHub.pending[serverID]) < 4 {
				serverOpsHub.pending[serverID] = append(serverOpsHub.pending[serverID], job)
				serverOpsHub.mu.Unlock()
				return job.JobID, resCh, true
			}
			delete(serverOpsHub.results, job.JobID)
			serverOpsHub.mu.Unlock()
			return "", nil, false
		}
	}
	if allowQueue && len(serverOpsHub.pending[serverID]) < 4 {
		serverOpsHub.results[job.JobID] = resCh
		serverOpsHub.pending[serverID] = append(serverOpsHub.pending[serverID], job)
		serverOpsHub.mu.Unlock()
		return job.JobID, resCh, true
	}
	serverOpsHub.mu.Unlock()
	return "", nil, false
}

func opsCleanup(jobID string) {
	serverOpsHub.mu.Lock()
	delete(serverOpsHub.results, jobID)
	serverOpsHub.mu.Unlock()
}

func serverOpsOnline(server *models.ProxyServer) bool {
	serverOpsHub.mu.Lock()
	_, waiting := serverOpsHub.waiters[server.ID]
	serverOpsHub.mu.Unlock()
	if waiting {
		return true
	}
	return server.LastSeenAt != nil && time.Since(*server.LastSeenAt) < 90*time.Second
}

// AgentOpsWait 代理 Agent 长轮询等待按需运维指令（BBR 调优 / 内核升级 / 日志拉取 / 端口预检）。
func AgentOpsWait(c *gin.Context) {
	s, ok := agentServer(c)
	if !ok {
		return
	}
	waitSec, _ := strconv.Atoi(c.DefaultQuery("timeout", "25"))
	if waitSec < 5 {
		waitSec = 5
	}
	if waitSec > 30 {
		waitSec = 30
	}
	ch := opsRegister(s.ID)
	defer opsUnregister(s.ID, ch)

	timer := time.NewTimer(time.Duration(waitSec) * time.Second)
	defer timer.Stop()

	select {
	case job := <-ch:
		c.JSON(http.StatusOK, gin.H{"code": "00000", "data": job, "msg": "dispatched"})
	case <-timer.C:
		c.JSON(http.StatusOK, gin.H{"code": "00000", "data": serverOpsJob{}, "msg": "idle"})
	case <-c.Request.Context().Done():
		return
	}
}

// AgentOpsResult 代理 Agent 回报运维指令执行结果。
func AgentOpsResult(c *gin.Context) {
	if _, ok := agentServer(c); !ok {
		return
	}
	var res serverOpsResult
	if err := c.ShouldBindJSON(&res); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	serverOpsHub.mu.Lock()
	ch, ok := serverOpsHub.results[res.JobID]
	if ok {
		delete(serverOpsHub.results, res.JobID)
	}
	serverOpsHub.mu.Unlock()
	if ok {
		select {
		case ch <- res:
		default:
		}
		close(ch)
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

func runServerOpsRPC(c *gin.Context, serverID uint, job serverOpsJob, timeout time.Duration) (*models.ProxyServer, *serverOpsResult, bool) {
	var server models.ProxyServer
	if err := models.DB.First(&server, serverID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return nil, nil, false
	}
	if !serverOpsOnline(&server) {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 Agent 当前离线"})
		return nil, nil, false
	}
	jobID, resCh, ok := opsDispatch(server.ID, job, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "服务器运维通道繁忙，请稍后再试"})
		return nil, nil, false
	}
	defer opsCleanup(jobID)

	select {
	case res := <-resCh:
		return &server, &res, true
	case <-time.After(timeout):
		c.JSON(504, gin.H{"code": "50400", "msg": "执行超时，请确认该服务器 Agent 已升级至 v1.20"})
		return nil, nil, false
	case <-c.Request.Context().Done():
		return nil, nil, false
	}
}

// ProxyServerTune 一键 BBR + UDP 缓冲区内核调优。
func ProxyServerTune(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 ID 无效"})
		return
	}
	server, res, ok := runServerOpsRPC(c, req.ID, serverOpsJob{Action: "tune-network"}, 25*time.Second)
	if !ok {
		return
	}
	if !res.OK {
		c.JSON(500, gin.H{"code": "50000", "msg": res.Error})
		return
	}
	updates := map[string]any{}
	if res.BBR != "" {
		updates["bbr"] = res.BBR
	}
	if res.Qdisc != "" {
		updates["qdisc"] = res.Qdisc
	}
	if res.Rmem > 0 {
		updates["rmem_max"] = res.Rmem
	}
	if res.Wmem > 0 {
		updates["wmem_max"] = res.Wmem
	}
	if len(updates) > 0 {
		_ = models.DB.Model(&models.ProxyServer{}).Where("id = ?", server.ID).Updates(updates).Error
	}
	c.JSON(200, gin.H{"code": "00000", "data": res, "msg": "内核网络调优完成：" + res.Output})
}

// ProxyServerUpgradeCore 一键热升级 Xray 内核（含失败自动回滚）。
func ProxyServerUpgradeCore(c *gin.Context) {
	var req struct {
		ID      uint   `json:"id"`
		Version string `json:"version"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 ID 无效"})
		return
	}
	_, res, ok := runServerOpsRPC(c, req.ID, serverOpsJob{
		Action:  "upgrade-core",
		Version: strings.TrimSpace(req.Version),
	}, 95*time.Second)
	if !ok {
		return
	}
	if !res.OK {
		c.JSON(500, gin.H{"code": "50000", "msg": res.Error})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": res, "msg": res.Output})
}

// ProxyServerLogs 按需拉取 Xray / Agent 实时运行日志。
func ProxyServerLogs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	if id <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 ID 无效"})
		return
	}
	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "180"))
	source := strings.TrimSpace(c.DefaultQuery("source", "xray"))
	filter := strings.TrimSpace(c.Query("filter"))
	_, res, ok := runServerOpsRPC(c, uint(id), serverOpsJob{
		Action: "fetch-logs",
		Source: source,
		Lines:  lines,
		Filter: filter,
	}, 20*time.Second)
	if !ok {
		return
	}
	if !res.OK {
		c.JSON(500, gin.H{"code": "50000", "msg": res.Error})
		return
	}
	entries, summary := parseStructuredLogs(res.Output, source)
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"logs":    res.Output,
			"source":  source,
			"entries": entries,
			"summary": summary,
		},
		"msg": "ok",
	})
}

// ProxyServerCheckPort 保存入站前预检服务器端口是否已被占用。
func ProxyServerCheckPort(c *gin.Context) {
	var req struct {
		ServerID uint   `json:"serverId"`
		Port     int    `json:"port"`
		Proto    string `json:"proto"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ServerID == 0 || req.Port <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数无效"})
		return
	}
	_, res, ok := runServerOpsRPC(c, req.ServerID, serverOpsJob{
		Action: "check-port",
		Port:   req.Port,
		Proto:  req.Proto,
	}, 12*time.Second)
	if !ok {
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"inUse": res.InUse, "detail": res.Output}, "msg": res.Output})
}
