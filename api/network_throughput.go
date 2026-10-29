package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ppeelink/models"
	"ppeelink/throughputcore"
	"ppeelink/tracecore"
)

// This mutex serializes reservations and state transitions on the single panel.
// Agent commands carry absolute leases, so a stopped panel cannot leave listeners behind.
var throughputMu sync.Mutex
var throughputActive = []string{"queued", "starting", "running"}

func throughputError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": fmt.Sprintf("%d00", status), "msg": message})
}
func throughputOK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"code": "00000", "data": data, "msg": "ok"})
}

func advanceThroughput(runID uint) error {
	var run models.ThroughputRun
	if err := models.DB.First(&run, runID).Error; err != nil {
		return err
	}
	if run.Status != "running" {
		return nil
	}
	var count int64
	if err := models.DB.Model(&models.ThroughputJob{}).Where("run_id = ? AND status IN ?", runID, []string{"starting", "running"}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var next models.ThroughputJob
	err := models.DB.Where("run_id = ? AND status = ?", runID, "queued").Order("sequence").First(&next).Error
	if err == nil {
		now := time.Now()
		if next.Sequence > 0 {
			now = now.Add(4 * time.Second)
		}
		expires := now.Add(120 * time.Second)
		return models.DB.Model(&next).Where("status = ?", "queued").Updates(map[string]any{"status": "starting", "started_at": now, "expires_at": expires}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	var completed int64
	models.DB.Model(&models.ThroughputJob{}).Where("run_id = ? AND status = ?", runID, "completed").Count(&completed)
	status := "completed"
	if completed == 0 {
		status = "failed"
	}
	return models.DB.Model(&run).Updates(map[string]any{"status": status, "finished_at": time.Now()}).Error
}

func expireThroughput() {
	now := time.Now()
	var runs []models.ThroughputRun
	models.DB.Where("status = ?", "running").Find(&runs)
	for _, run := range runs {
		if now.Sub(run.CreatedAt) > 45*time.Minute {
			models.DB.Model(&models.ThroughputJob{}).Where("run_id = ? AND status IN ?", run.ID, throughputActive).Updates(map[string]any{"status": "timeout", "error": "整批测试超时，所有端点将停止", "finished_at": now, "password": "", "private_key": "", "public_key": ""})
			models.DB.Model(&run).Updates(map[string]any{"status": "failed", "finished_at": now})
			continue
		}
		models.DB.Model(&models.ThroughputJob{}).Where("run_id = ? AND status IN ? AND expires_at < ?", run.ID, []string{"starting", "running"}, now).Updates(map[string]any{"status": "timeout", "error": "端点未在租约内完成；请检查 Agent、测试端口和两端连接", "finished_at": now, "password": "", "private_key": "", "public_key": ""})
		_ = advanceThroughput(run.ID)
	}
	models.DB.Model(&models.ThroughputCapability{}).Where("prepare_requested = ? AND prepare_at < ?", true, now.Add(-3*time.Minute)).Updates(map[string]any{"prepare_requested": false, "error": "工具准备超时，请检查 Agent 后重试"})
	cutoff := now.Add(-7 * 24 * time.Hour)
	models.DB.Where("created_at < ?", cutoff).Delete(&models.ThroughputJob{})
	models.DB.Where("created_at < ?", cutoff).Delete(&models.ThroughputRun{})
}

func NetworkThroughputSources(c *gin.Context) {
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	var servers []models.ProxyServer
	if err := models.DB.Where("kind = ? AND enabled = ? AND removing = ?", models.ServerKindAgent, true, false).Order("id").Find(&servers).Error; err != nil {
		throughputError(c, 500, "读取服务器失败")
		return
	}
	var caps []models.ThroughputCapability
	models.DB.Find(&caps)
	byID := map[uint]models.ThroughputCapability{}
	for _, v := range caps {
		byID[v.ServerID] = v
	}
	items := make([]gin.H, 0, len(servers))
	now := time.Now()
	for _, s := range servers {
		cap := byID[s.ID]
		fresh := !cap.SeenAt.IsZero() && now.Sub(cap.SeenAt) < 2*time.Minute
		reason := cap.Error
		if !fresh {
			reason = "请更新 Agent 并等待测速能力上报"
		} else if !cap.Available || !cap.AuthSupported {
			if reason == "" {
				reason = "请准备支持安全认证的 iperf3 3.22 或更新版本"
			}
		}
		items = append(items, gin.H{"id": s.ID, "name": s.Name, "publicHost": s.PublicHost, "available": fresh && cap.Available && cap.AuthSupported, "version": cap.Version, "path": cap.Path, "error": reason, "seenAt": cap.SeenAt, "prepareRequested": cap.PrepareRequested, "bidirSupported": cap.BidirSupported})
	}
	throughputOK(c, items)
}

func NetworkThroughputPrepare(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		ServerIDs []uint `json:"serverIds"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.ServerIDs) == 0 || len(req.ServerIDs) > 16 {
		throughputError(c, 400, "请选择需要准备工具的服务器")
		return
	}
	throughputMu.Lock()
	defer throughputMu.Unlock()
	var servers []models.ProxyServer
	if err := models.DB.Where("id IN ? AND kind = ? AND enabled = ? AND removing = ?", req.ServerIDs, models.ServerKindAgent, true, false).Find(&servers).Error; err != nil || len(servers) != len(req.ServerIDs) {
		throughputError(c, 400, "服务器不存在、重复或已停用")
		return
	}
	var active []models.ThroughputRun
	models.DB.Where("status = ?", "running").Find(&active)
	for _, run := range active {
		var count int64
		models.DB.Model(&models.ThroughputJob{}).Where("run_id = ? AND (source_id IN ? OR ? IN ?)", run.ID, req.ServerIDs, run.LandingID, req.ServerIDs).Count(&count)
		if count > 0 {
			throughputError(c, 409, "服务器正在测速，请完成或取消后再准备工具")
			return
		}
	}
	now := time.Now()
	for _, s := range servers {
		var existing models.ThroughputCapability
		if models.DB.First(&existing, "server_id = ?", s.ID).Error == nil && existing.PrepareRequested {
			continue
		}
		nonce := make([]byte, 4)
		if _, err := rand.Read(nonce); err != nil {
			throughputError(c, 500, "生成工具准备任务失败")
			return
		}
		prepareID := uint(binary.BigEndian.Uint32(nonce)&0x7fffffff) + 1
		cap := models.ThroughputCapability{ServerID: s.ID, PrepareID: prepareID, PrepareRequested: true, PrepareAt: &now}
		if err := models.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}}, DoUpdates: clause.AssignmentColumns([]string{"prepare_requested", "prepare_at", "prepare_id"})}).Create(&cap).Error; err != nil {
			throughputError(c, 500, "保存工具准备请求失败")
			return
		}
	}
	throughputOK(c, gin.H{"requested": len(servers)})
}

func throughputIP(host string, family int) (string, error) {
	target, err := tracecore.ValidateTarget(host)
	if err != nil {
		return "", err
	}
	if ip := net.ParseIP(target); ip != nil {
		if (family == 4) != (ip.To4() != nil) {
			return "", fmt.Errorf("落地地址不支持选择的 IPv%d", family)
		}
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			return "", fmt.Errorf("落地不能使用回环或链路本地地址")
		}
		return ip.String(), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIP(ctx, fmt.Sprintf("ip%d", family), target)
	if err != nil {
		return "", fmt.Errorf("无法解析落地 IPv%d 地址", family)
	}
	for _, ip := range addrs {
		if ip.IsGlobalUnicast() && !ip.IsLoopback() {
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("落地没有可用的 IPv%d 地址", family)
}
func throughputRandom() (string, error) {
	b := make([]byte, 24)
	_, err := rand.Read(b)
	return hex.EncodeToString(b), err
}

func NetworkThroughputCreate(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		LandingID uint    `json:"landingId"`
		SourceIDs []uint  `json:"sourceIds"`
		Protocol  string  `json:"protocol"`
		Direction string  `json:"direction"`
		Family    int     `json:"family"`
		Port      int     `json:"port"`
		Duration  int     `json:"duration"`
		Streams   int     `json:"streams"`
		RateMbps  float64 `json:"rateMbps"`
	}
	if c.ShouldBindJSON(&req) != nil {
		throughputError(c, 400, "请求格式无效")
		return
	}
	if req.Port == 0 {
		req.Port = 5201
	}
	if req.LandingID == 0 || len(req.SourceIDs) < 1 || len(req.SourceIDs) > 8 || (req.Protocol != "tcp" && req.Protocol != "udp") || (req.Direction != "forward" && req.Direction != "reverse" && req.Direction != "both" && req.Direction != "bidir") || (req.Family != 4 && req.Family != 6) || req.Duration < 5 || req.Duration > 30 || req.Streams < 1 || req.Streams > 8 || math.IsNaN(req.RateMbps) || req.RateMbps < 1 || req.RateMbps > 10000 || req.Port < 1024 || req.Port > 65535 {
		throughputError(c, 400, "参数无效：选择协议、方向与 IP 版本，时长 5–30 秒，连接 1–8，速率 1–10000 Mbps，测试端口 1024–65535")
		return
	}
	seen := map[uint]bool{req.LandingID: true}
	for _, id := range req.SourceIDs {
		if id == 0 || seen[id] {
			throughputError(c, 400, "来源不能重复，也不能与落地相同")
			return
		}
		seen[id] = true
	}
	// Resolve once so every candidate tests the exact same endpoint and IP family.
	var landing models.ProxyServer
	if models.DB.Where("id = ? AND kind = ? AND enabled = ? AND removing = ?", req.LandingID, models.ServerKindAgent, true, false).First(&landing).Error != nil {
		throughputError(c, 400, "落地服务器不可用")
		return
	}
	ip, err := throughputIP(landing.PublicHost, req.Family)
	if err != nil {
		throughputError(c, 400, err.Error())
		return
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		throughputError(c, 500, "生成临时认证失败")
		return
	}
	private := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		throughputError(c, 500, "生成临时认证失败")
		return
	}
	public := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}))
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	ids := append([]uint{req.LandingID}, req.SourceIDs...)
	var servers []models.ProxyServer
	if err := models.DB.Where("id IN ? AND kind = ? AND enabled = ? AND removing = ?", ids, models.ServerKindAgent, true, false).Find(&servers).Error; err != nil || len(servers) != len(ids) {
		throughputError(c, 400, "部分服务器已停用或不存在")
		return
	}
	byID := map[uint]models.ProxyServer{}
	for _, s := range servers {
		byID[s.ID] = s
	}
	for _, id := range ids {
		var cap models.ThroughputCapability
		if models.DB.First(&cap, "server_id = ?", id).Error != nil || time.Since(cap.SeenAt) > 2*time.Minute || !cap.Available || !cap.AuthSupported || cap.PrepareRequested || (req.Direction == "bidir" && !cap.BidirSupported) {
			throughputError(c, 400, byID[id].Name+" 的测速工具未就绪，请先更新 Agent 或准备工具")
			return
		}
	}
	// An Agent polls desired state every two seconds. Allow in-flight HTTP
	// reports to unwind before reusing a cancelled listener port.
	var cooling int64
	if err := models.DB.Table("throughput_jobs AS j").Joins("JOIN throughput_runs AS r ON r.id = j.run_id").Where("j.status IN ? AND j.finished_at > ? AND (j.source_id IN ? OR r.landing_id IN ?)", []string{"cancelled", "failed", "timeout"}, time.Now().Add(-15*time.Second), ids, ids).Count(&cooling).Error; err != nil {
		throughputError(c, 500, "无法检查端点清理状态")
		return
	}
	if cooling > 0 {
		throughputError(c, 409, "上次测速正在清理临时监听，请在 15 秒后重试")
		return
	}
	var active []models.ThroughputRun
	models.DB.Where("status = ?", "running").Find(&active)
	for _, r := range active {
		var jobs []models.ThroughputJob
		models.DB.Where("run_id = ?", r.ID).Find(&jobs)
		if seen[r.LandingID] {
			throughputError(c, 409, "所选服务器正在参与其他测速，请先等待或取消")
			return
		}
		for _, j := range jobs {
			if seen[j.SourceID] {
				throughputError(c, 409, "所选服务器正在参与其他测速，请先等待或取消")
				return
			}
		}
	}
	now := time.Now()
	run := models.ThroughputRun{LandingID: landing.ID, LandingName: landing.Name, Target: ip, Protocol: req.Protocol, Direction: req.Direction, Family: req.Family, Port: req.Port, Duration: req.Duration, Streams: req.Streams, RateMbps: req.RateMbps, Status: "running", CreatedAt: now}
	directions := []string{req.Direction}
	if req.Direction == "both" {
		directions = []string{"forward", "reverse"}
	}
	var jobs []models.ThroughputJob
	for _, id := range req.SourceIDs {
		for _, direction := range directions {
			password, err := throughputRandom()
			if err != nil {
				throughputError(c, 500, "生成临时凭据失败")
				return
			}
			username, err := throughputRandom()
			if err != nil {
				throughputError(c, 500, "生成临时凭据失败")
				return
			}
			jobs = append(jobs, models.ThroughputJob{SourceID: id, SourceName: byID[id].Name, Sequence: len(jobs), Direction: direction, Status: "queued", Username: username, Password: password, PrivateKey: private, PublicKey: public, CreatedAt: now})
		}
	}
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&run).Error; err != nil {
			return err
		}
		for i := range jobs {
			jobs[i].RunID = run.ID
		}
		return tx.Create(&jobs).Error
	})
	if err != nil {
		throughputError(c, 500, "保存测速任务失败")
		return
	}
	if err = advanceThroughput(run.ID); err != nil {
		models.DB.Model(&run).Update("status", "failed")
		throughputError(c, 500, "启动测速任务失败")
		return
	}
	throughputOK(c, run)
}

func NetworkThroughputList(c *gin.Context) {
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	var runs []models.ThroughputRun
	if err := models.DB.Order("id desc").Limit(30).Find(&runs).Error; err != nil {
		throughputError(c, 500, "读取历史失败")
		return
	}
	throughputOK(c, runs)
}
func NetworkThroughputDetail(c *gin.Context) {
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || id == 0 {
		throughputError(c, 400, "测速 id 无效")
		return
	}
	var run models.ThroughputRun
	if models.DB.First(&run, id).Error != nil {
		throughputError(c, 404, "测速不存在或已过期")
		return
	}
	var jobs []models.ThroughputJob
	if err := models.DB.Where("run_id = ?", id).Order("sequence").Find(&jobs).Error; err != nil {
		throughputError(c, 500, "读取结果失败")
		return
	}
	throughputOK(c, gin.H{"run": run, "jobs": jobs})
}
func NetworkThroughputCancel(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ID == 0 {
		throughputError(c, 400, "测速 id 无效")
		return
	}
	throughputMu.Lock()
	defer throughputMu.Unlock()
	now := time.Now()
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ThroughputRun{}).Where("id = ? AND status = ?", req.ID, "running").Updates(map[string]any{"status": "cancelled", "finished_at": now}).Error; err != nil {
			return err
		}
		return tx.Model(&models.ThroughputJob{}).Where("run_id = ? AND status IN ?", req.ID, throughputActive).Updates(map[string]any{"status": "cancelled", "finished_at": now, "password": "", "private_key": "", "public_key": ""}).Error
	})
	if err != nil {
		throughputError(c, 500, "取消失败")
		return
	}
	throughputOK(c, gin.H{"cancelled": true})
}

func AgentThroughputCapabilities(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req throughputcore.Capability
	if c.ShouldBindJSON(&req) != nil || len(req.Version) > 80 || len(req.Error) > 512 || len(req.Path) > 512 {
		throughputError(c, 400, "能力格式无效")
		return
	}
	throughputMu.Lock()
	defer throughputMu.Unlock()
	cap := models.ThroughputCapability{ServerID: server.ID, Path: req.Path, Available: req.Available, AuthSupported: req.AuthSupported, BidirSupported: req.BidirSupported, JSONStreamSupported: req.JSONStreamSupported, Version: req.Version, Error: req.Error, SeenAt: time.Now()}
	// Preserve an outstanding preparation request; role=prepare completion clears it.
	if err := models.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}}, DoUpdates: clause.AssignmentColumns([]string{"path", "available", "auth_supported", "bidir_supported", "json_stream_supported", "version", "error", "seen_at"})}).Create(&cap).Error; err != nil {
		throughputError(c, 500, "保存能力失败")
		return
	}
	throughputOK(c, gin.H{})
}
func AgentThroughputJobs(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	models.DB.Model(&models.ThroughputCapability{}).Where("server_id = ?", server.ID).Update("seen_at", time.Now())
	jobs := make([]throughputcore.Job, 0, 1)
	if !server.Enabled || server.Removing {
		throughputOK(c, gin.H{"jobs": jobs})
		return
	}
	var cap models.ThroughputCapability
	if models.DB.First(&cap, "server_id = ?", server.ID).Error == nil && cap.PrepareRequested && cap.PrepareAt != nil {
		jobs = append(jobs, throughputcore.Job{ID: cap.PrepareID, Role: "prepare", ExpiresAt: cap.PrepareAt.Add(150 * time.Second)})
		throughputOK(c, gin.H{"jobs": jobs})
		return
	}
	var assigned []models.ThroughputJob
	models.DB.Where("status IN ? AND started_at <= ?", []string{"starting", "running"}, time.Now()).Order("id").Find(&assigned)
	for _, j := range assigned {
		var run models.ThroughputRun
		if models.DB.First(&run, j.RunID).Error != nil || run.Status != "running" || j.ExpiresAt == nil {
			continue
		}
		role := ""
		if server.ID == run.LandingID {
			role = "server"
		} else if server.ID == j.SourceID && j.Status == "running" {
			role = "client"
		}
		if role == "" {
			continue
		}
		work := throughputcore.Job{ID: j.ID, Role: role, Host: run.Target, Port: run.Port, Protocol: run.Protocol, Direction: j.Direction, Family: run.Family, Duration: run.Duration, Streams: run.Streams, RateMbps: run.RateMbps, Username: j.Username, Password: j.Password, ExpiresAt: *j.ExpiresAt}
		if role == "server" {
			work.PrivateKey = j.PrivateKey
		} else {
			work.PublicKey = j.PublicKey
		}
		jobs = append(jobs, work)
	}
	throughputOK(c, gin.H{"jobs": jobs})
}
func AgentThroughputReport(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	var req throughputcore.Report
	if c.ShouldBindJSON(&req) != nil || len(req.Error) > 512 {
		throughputError(c, 400, "结果格式无效或过大")
		return
	}
	throughputMu.Lock()
	defer throughputMu.Unlock()
	expireThroughput()
	if req.Role == "prepare" && (req.Status == "completed" || req.Status == "failed") {
		var cap models.ThroughputCapability
		if models.DB.First(&cap, "server_id = ?", server.ID).Error != nil || !cap.PrepareRequested || cap.PrepareID != req.ID {
			throughputError(c, 409, "工具准备任务已失效")
			return
		}
		updates := map[string]any{"prepare_requested": false}
		if req.Status == "failed" {
			updates["error"] = req.Error
		}
		models.DB.Model(&models.ThroughputCapability{}).Where("server_id = ?", server.ID).Updates(updates)
		throughputOK(c, gin.H{})
		return
	}
	var job models.ThroughputJob
	var run models.ThroughputRun
	if models.DB.First(&job, req.ID).Error != nil || models.DB.First(&run, job.RunID).Error != nil || run.Status != "running" || (job.Status != "starting" && job.Status != "running") || ((req.Role != "server" || server.ID != run.LandingID) && (req.Role != "client" || server.ID != job.SourceID)) || !server.Enabled || server.Removing {
		throughputError(c, 409, "任务不属于此 Agent、已取消或已结束")
		return
	}
	if req.Role == "server" && req.Status == "ready" {
		if job.Status == "starting" {
			if err := models.DB.Model(&job).Update("status", "running").Error; err != nil {
				throughputError(c, 500, "更新就绪状态失败")
				return
			}
		}
		throughputOK(c, gin.H{})
		return
	}
	if req.Role == "server" && req.Status == "completed" {
		throughputOK(c, gin.H{})
		return
	}
	if req.Role == "client" && req.Status == "running" {
		if job.Status == "running" && req.Result == nil {
			throughputOK(c, gin.H{})
			return
		}
		if job.Status != "running" || req.Result == nil || req.Result.Protocol != run.Protocol || req.Result.Direction != job.Direction || req.Result.Streams != run.Streams || req.Result.RateMbps != run.RateMbps || len(req.Result.ProgressIntervals) > 120 {
			throughputError(c, 400, "进度参数无效")
			return
		}
		encoded, err := json.Marshal(req.Result)
		if err != nil || len(encoded) > 100*1024 {
			throughputError(c, 400, "进度过大")
			return
		}
		if err := models.DB.Model(&job).Update("result", json.RawMessage(encoded)).Error; err != nil {
			throughputError(c, 500, "保存进度失败")
			return
		}
		throughputOK(c, gin.H{})
		return
	}
	if req.Status != "completed" && req.Status != "failed" {
		throughputError(c, 400, "结果状态无效")
		return
	}
	updates := map[string]any{"status": "failed", "error": strings.TrimSpace(req.Error), "finished_at": time.Now(), "password": "", "private_key": "", "public_key": ""}
	if req.Status == "completed" {
		if req.Role != "client" || job.Status != "running" || req.Result == nil {
			throughputError(c, 409, "测速未就绪或缺少结果")
			return
		}
		result := *req.Result
		if result.Direction != job.Direction || result.Protocol != run.Protocol || result.Duration <= 0 || result.Duration > 90 || result.Streams != run.Streams || result.RateMbps != run.RateMbps || (job.Direction != "reverse" && result.Forward == nil) || (job.Direction != "forward" && result.Reverse == nil) {
			throughputError(c, 400, "结果方向或参数不符合任务")
			return
		}
		encoded, err := json.Marshal(result)
		if err != nil || len(encoded) > 100*1024 {
			throughputError(c, 400, "结果格式无效或过大")
			return
		}
		updates["result"] = json.RawMessage(encoded)
		updates["status"] = "completed"
		updates["error"] = ""
	}
	if err := models.DB.Model(&job).Where("status IN ?", []string{"starting", "running"}).Updates(updates).Error; err != nil {
		throughputError(c, 500, "保存结果失败")
		return
	}
	if err := advanceThroughput(run.ID); err != nil {
		throughputError(c, 500, "推进下一项测试失败")
		return
	}
	throughputOK(c, gin.H{})
}
