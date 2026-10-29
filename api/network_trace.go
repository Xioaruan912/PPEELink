package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"
	"ppeelink/models"
	"ppeelink/tracecore"
)

var traceSlots = make(chan struct{}, 2)
var traceCancels sync.Map
var traceCreateMu sync.Mutex

func expireNetworkTraces() {
	now := time.Now()
	models.DB.Model(&models.NetworkTraceView{}).Where("status IN ? AND created_at < ?", []string{"queued", "running"}, now.Add(-3*time.Minute)).Updates(map[string]any{"status": "timeout", "error": "来源未在 3 分钟内完成追踪；请检查探针版本、在线状态与追踪工具", "finished_at": now})
	cutoff := now.Add(-7 * 24 * time.Hour)
	models.DB.Where("created_at < ?", cutoff).Delete(&models.NetworkTraceView{})
	models.DB.Where("created_at < ?", cutoff).Delete(&models.NetworkTraceRun{})
}

func NetworkTraceSources(c *gin.Context) {
	expireNetworkTraces()
	probes, err := models.GetProbes(true)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取探针失败"})
		return
	}
	var caps []models.TraceProbeCapability
	models.DB.Find(&caps)
	capMap := map[uint]models.TraceProbeCapability{}
	for _, cap := range caps {
		capMap[cap.ProbeID] = cap
	}
	items := make([]gin.H, 0, len(probes))
	now := time.Now()
	for _, p := range probes {
		cap, exists := capMap[p.ID]
		fresh := exists && now.Sub(cap.SeenAt) < 2*time.Minute
		reason := ""
		if !fresh {
			reason = "需要升级探针并等待路由追踪轮询"
		} else if cap.Tool == "" {
			reason = "探针未安装可用的 NextTrace / traceroute / tracepath"
		}
		items = append(items, gin.H{"id": p.ID, "name": p.Name, "region": p.Region, "supported": fresh, "tool": cap.Tool, "available": fresh && cap.Tool != "", "reason": reason, "lastSeenAt": p.LastSeenAt, "optionsSupported": cap.Version >= tracecore.ClientVersion, "enriched": cap.Tool == "nexttrace"})
	}
	var agents []models.ProxyServer
	models.DB.Where("kind = ? AND enabled = ?", models.ServerKindAgent, true).Order("id").Find(&agents)
	var agentCaps []models.TraceAgentCapability
	models.DB.Find(&agentCaps)
	agentCapMap := map[uint]models.TraceAgentCapability{}
	for _, cap := range agentCaps {
		agentCapMap[cap.ServerID] = cap
	}
	agentItems := make([]gin.H, 0, len(agents))
	for _, server := range agents {
		cap, exists := agentCapMap[server.ID]
		fresh := exists && now.Sub(cap.SeenAt) < 2*time.Minute
		reason := ""
		if !fresh {
			reason = "agent 尚未上报追踪能力；请升级 agent 后重试"
		} else if cap.Tool == "" {
			reason = "服务器未安装可用的 NextTrace / traceroute / tracepath"
		}
		agentItems = append(agentItems, gin.H{"id": server.ID, "name": server.Name, "supported": fresh, "tool": cap.Tool, "available": fresh && cap.Tool != "", "reason": reason, "lastSeenAt": server.LastSeenAt, "publicHost": server.PublicHost, "preparing": tracePreparing(cap, now), "prepareError": tracePrepareError(cap, now), "optionsSupported": cap.Version >= tracecore.ClientVersion, "enriched": cap.Tool == "nexttrace"})
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"panelTool": tracecore.AvailableTool(), "nextTraceVersion": tracecore.NextTraceVersion, "panelEnriched": tracecore.NextTraceReady(), "probes": items, "agents": agentItems}, "msg": "追踪来源"})
}

func NetworkTraceCreate(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		Target string `json:"target"`
		tracecore.Options
		IncludePanel bool   `json:"includePanel"`
		ProbeIDs     []uint `json:"probeIds"`
		AgentIDs     []uint `json:"agentIds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "请求格式无效"})
		return
	}
	target, err := tracecore.ValidateTarget(req.Target)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	opts, err := tracecore.NormalizeOptions(target, req.Options)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	ids := make([]uint, 0)
	seen := map[uint]bool{}
	for _, id := range req.ProbeIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	agentIDs := make([]uint, 0)
	seenAgents := map[uint]bool{}
	for _, id := range req.AgentIDs {
		if id > 0 && !seenAgents[id] {
			seenAgents[id] = true
			agentIDs = append(agentIDs, id)
		}
	}
	if len(ids)+len(agentIDs) > 8 || (!req.IncludePanel && len(ids)+len(agentIDs) == 0) {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择来源，最多 8 个远程来源"})
		return
	}
	traceCreateMu.Lock()
	defer traceCreateMu.Unlock()
	expireNetworkTraces()
	var active int64
	models.DB.Model(&models.NetworkTraceView{}).Where("status IN ?", []string{"queued", "running"}).Count(&active)
	if active >= 18 {
		c.JSON(429, gin.H{"code": "42900", "msg": "已有追踪正在运行，请稍后重试"})
		return
	}
	var probes []models.Probe
	if len(ids) > 0 {
		if err := models.DB.Where("id IN ? AND enabled = ?", ids, true).Find(&probes).Error; err != nil || len(probes) != len(ids) {
			c.JSON(400, gin.H{"code": "40000", "msg": "部分探针不存在或已停用"})
			return
		}
	}
	var agents []models.ProxyServer
	if len(agentIDs) > 0 {
		if err := models.DB.Where("id IN ? AND kind = ? AND enabled = ?", agentIDs, models.ServerKindAgent, true).Find(&agents).Error; err != nil || len(agents) != len(agentIDs) {
			c.JSON(400, gin.H{"code": "40000", "msg": "部分自建服务器不存在或未启用"})
			return
		}
	}
	now := time.Now()
	run := models.NetworkTraceRun{Target: target, Protocol: opts.Protocol, Port: opts.Port, Family: opts.Family, CreatedAt: now}
	views := make([]models.NetworkTraceView, 0, len(ids)+len(agents)+1)
	if req.IncludePanel {
		v := models.NetworkTraceView{SourceName: "面板服务器", Status: "queued", CreatedAt: now}
		if tracecore.AvailableTool() == "" {
			v.Status = "unsupported"
			v.Error = "面板未安装可用的 NextTrace / traceroute / tracepath"
			v.FinishedAt = &now
		}
		views = append(views, v)
	}
	for _, p := range probes {
		v := models.NetworkTraceView{ProbeID: p.ID, SourceName: p.Name, Status: "queued", CreatedAt: now}
		var cap models.TraceProbeCapability
		if models.DB.First(&cap, "probe_id = ?", p.ID).Error != nil || now.Sub(cap.SeenAt) > 2*time.Minute {
			v.Status = "unsupported"
			v.Error = "探针未提供追踪能力；请升级探针后重试"
			v.FinishedAt = &now
		} else if cap.Version < tracecore.ClientVersion && !tracecore.LegacyCompatible(opts) {
			v.Status = "unsupported"
			v.Error = "请升级 agent / 探针以使用指定协议、端口或 IP 版本"
			v.FinishedAt = &now
		} else if cap.Tool == "" {
			v.Status = "unsupported"
			v.Error = "探针未安装可用的 NextTrace / traceroute / tracepath"
			v.FinishedAt = &now
		}
		views = append(views, v)
	}
	for _, agent := range agents {
		v := models.NetworkTraceView{AgentServerID: agent.ID, SourceName: agent.Name, Status: "queued", CreatedAt: now}
		var cap models.TraceAgentCapability
		if models.DB.First(&cap, "server_id = ?", agent.ID).Error != nil || now.Sub(cap.SeenAt) > 2*time.Minute {
			v.Status = "unsupported"
			v.Error = "agent 未提供追踪能力；请先使用面板更新 agent"
			v.FinishedAt = &now
		} else if cap.Version < tracecore.ClientVersion && !tracecore.LegacyCompatible(opts) {
			v.Status = "unsupported"
			v.Error = "请升级 agent / 探针以使用指定协议、端口或 IP 版本"
			v.FinishedAt = &now
		} else if cap.Tool == "" {
			v.Status = "unsupported"
			v.Error = "服务器未安装可用的 NextTrace / traceroute / tracepath"
			v.FinishedAt = &now
		}
		views = append(views, v)
	}
	tx := models.DB.Begin()
	if tx.Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "无法创建追踪"})
		return
	}
	if err := tx.Create(&run).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"code": "50000", "msg": "保存追踪失败"})
		return
	}
	for i := range views {
		views[i].RunID = run.ID
	}
	if err := tx.Create(&views).Error; err != nil {
		tx.Rollback()
		c.JSON(500, gin.H{"code": "50000", "msg": "保存来源失败"})
		return
	}
	if err := tx.Commit().Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存追踪失败"})
		return
	}
	for _, v := range views {
		if v.ProbeID == 0 && v.AgentServerID == 0 && v.Status == "queued" {
			go runPanelTrace(v.ID, target, opts)
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": run, "msg": "追踪已创建"})
}

func runPanelTrace(id uint, target string, opts tracecore.Options) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	traceCancels.Store(id, cancel)
	defer traceCancels.Delete(id)
	select {
	case traceSlots <- struct{}{}:
		defer func() { <-traceSlots }()
	case <-ctx.Done():
		return
	}
	now := time.Now()
	tx := models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND status = ?", id, "queued").Updates(map[string]any{"status": "running", "started_at": now})
	if tx.Error != nil || tx.RowsAffected != 1 {
		return
	}
	result := tracecore.RunWithOptions(ctx, target, opts)
	finish := time.Now()
	status := "completed"
	if result.Error != "" {
		status = "failed"
	}
	models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND status = ?", id, "running").Updates(map[string]any{"status": status, "tool": result.Tool, "output": result.Output, "error": result.Error, "finished_at": finish})
}

func NetworkTraceList(c *gin.Context) {
	expireNetworkTraces()
	var runs []models.NetworkTraceRun
	if err := models.DB.Order("id desc").Limit(30).Find(&runs).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取历史失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": runs, "msg": "最近 7 天追踪"})
}
func NetworkTraceDetail(c *gin.Context) {
	expireNetworkTraces()
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "追踪 id 无效"})
		return
	}
	var run models.NetworkTraceRun
	if models.DB.First(&run, id).Error != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "追踪不存在或已过期"})
		return
	}
	var views []models.NetworkTraceView
	if err := models.DB.Where("run_id = ?", id).Order("id").Find(&views).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取来源失败"})
		return
	}
	items := make([]gin.H, 0, len(views))
	for _, v := range views {
		items = append(items, gin.H{"id": v.ID, "probeId": v.ProbeID, "agentServerId": v.AgentServerID, "sourceName": v.SourceName, "status": v.Status, "tool": v.Tool, "output": v.Output, "error": v.Error, "startedAt": v.StartedAt, "finishedAt": v.FinishedAt, "hops": tracecore.Parse(v.Output)})
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"run": run, "views": items}, "msg": "追踪详情"})
}
func NetworkTraceCancel(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "追踪 id 无效"})
		return
	}
	var views []models.NetworkTraceView
	models.DB.Where("run_id = ? AND status IN ?", req.ID, []string{"queued", "running"}).Find(&views)
	now := time.Now()
	models.DB.Model(&models.NetworkTraceView{}).Where("run_id = ? AND status IN ?", req.ID, []string{"queued", "running"}).Updates(map[string]any{"status": "cancelled", "finished_at": now})
	for _, v := range views {
		if cancel, ok := traceCancels.Load(v.ID); ok {
			cancel.(context.CancelFunc)()
		}
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已取消；远程探针会在当前命令超时后停止，结果不再写入"})
}

// ProbeTraceJobs is intentionally separate from TCP jobs so route diagnostics never
// wait behind a full TCP scan. Only token-bound assigned jobs are dispatched.
func ProbeTraceJobs(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	tool := c.Query("tool")
	if tool != "traceroute" && tool != "tracepath" && tool != "nexttrace" {
		tool = ""
	}
	version, _ := strconv.Atoi(c.Query("version"))
	if version != tracecore.ClientVersion {
		version = 0
	}
	cap := models.TraceProbeCapability{ProbeID: probe.ID, SeenAt: time.Now(), Tool: tool, Version: version}
	if err := models.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "probe_id"}}, DoUpdates: clause.AssignmentColumns([]string{"seen_at", "tool", "version"})}).Create(&cap).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录能力失败"})
		return
	}
	models.DB.Model(probe).Update("last_seen_at", time.Now())
	expireNetworkTraces()
	var views []models.NetworkTraceView
	if tool != "" {
		models.DB.Where("probe_id = ? AND status = ?", probe.ID, "queued").Order("id").Limit(1).Find(&views)
	}
	jobs := make([]gin.H, 0, 1)
	for _, v := range views {
		var run models.NetworkTraceRun
		if models.DB.First(&run, v.RunID).Error != nil {
			continue
		}
		if version < tracecore.ClientVersion && !tracecore.LegacyCompatible(traceRunOptions(run)) {
			markTraceUnsupported(v.ID, "请升级探针以执行指定追踪选项")
			continue
		}
		now := time.Now()
		tx := models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND probe_id = ? AND status = ?", v.ID, probe.ID, "queued").Updates(map[string]any{"status": "running", "started_at": now})
		if tx.Error == nil && tx.RowsAffected == 1 {
			jobs = append(jobs, gin.H{"id": v.ID, "target": run.Target, "protocol": run.Protocol, "port": run.Port, "family": run.Family})
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": jobs, "msg": "路由追踪任务"})
}
func ProbeTraceReport(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var req struct {
		ID     uint   `json:"id"`
		Tool   string `json:"tool"`
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	if c.ShouldBindJSON(&req) != nil || req.ID == 0 || len(req.Output) > tracecore.MaxOutput || len(req.Error) > 512 {
		c.JSON(400, gin.H{"code": "40000", "msg": "回报内容过大或格式无效"})
		return
	}
	if req.Tool != "traceroute" && req.Tool != "tracepath" && req.Tool != "nexttrace" {
		req.Tool = ""
	}
	status := "completed"
	if strings.TrimSpace(req.Error) != "" {
		status = "failed"
	}
	now := time.Now()
	tx := models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND probe_id = ? AND status = ?", req.ID, probe.ID, "running").Updates(map[string]any{"status": status, "tool": req.Tool, "output": strings.ToValidUTF8(req.Output, "�"), "error": req.Error, "finished_at": now})
	if tx.Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存回报失败"})
		return
	}
	if tx.RowsAffected != 1 {
		c.JSON(409, gin.H{"code": "40900", "msg": "任务不属于此探针、已取消或已结束"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已保存"})
}

// AgentTraceJobs 由共享 agent 拉取绑定的追踪任务；与 TCP 检测任务完全分离。
func AgentTraceJobs(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	tool := c.Query("tool")
	if tool != "traceroute" && tool != "tracepath" && tool != "nexttrace" {
		tool = ""
	}
	version, _ := strconv.Atoi(c.Query("version"))
	if version != tracecore.ClientVersion {
		version = 0
	}
	cap := models.TraceAgentCapability{ServerID: server.ID, SeenAt: time.Now(), Tool: tool, Version: version}
	if err := models.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}}, DoUpdates: clause.AssignmentColumns([]string{"seen_at", "tool", "version"})}).Create(&cap).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录能力失败"})
		return
	}
	models.DB.Model(server).Update("last_seen_at", time.Now())
	models.DB.First(&cap, "server_id = ?", server.ID)
	if version >= tracecore.ClientVersion && tracePreparing(cap, time.Now()) && cap.PrepareStartedAt == nil {
		now := time.Now()
		tx := models.DB.Model(&models.TraceAgentCapability{}).Where("server_id = ? AND prepare_id = ? AND prepare_started_at IS NULL AND prepare_finished_at IS NULL", server.ID, cap.PrepareID).Update("prepare_started_at", now)
		if tx.Error == nil && tx.RowsAffected == 1 {
			c.JSON(200, gin.H{"code": "00000", "data": []gin.H{{"action": "prepare", "prepareId": cap.PrepareID}}, "msg": "准备 NextTrace"})
			return
		}
	}

	expireNetworkTraces()
	var views []models.NetworkTraceView
	if tool != "" {
		models.DB.Where("agent_server_id = ? AND status = ?", server.ID, "queued").Order("id").Limit(1).Find(&views)
	}
	jobs := make([]gin.H, 0, 1)
	for _, v := range views {
		var run models.NetworkTraceRun
		if models.DB.First(&run, v.RunID).Error != nil {
			continue
		}
		if version < tracecore.ClientVersion && !tracecore.LegacyCompatible(traceRunOptions(run)) {
			markTraceUnsupported(v.ID, "请升级 agent 以执行指定追踪选项")
			continue
		}
		now := time.Now()
		tx := models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND agent_server_id = ? AND status = ?", v.ID, server.ID, "queued").Updates(map[string]any{"status": "running", "started_at": now})
		if tx.Error == nil && tx.RowsAffected == 1 {
			jobs = append(jobs, gin.H{"id": v.ID, "target": run.Target, "protocol": run.Protocol, "port": run.Port, "family": run.Family})
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": jobs, "msg": "路由追踪任务"})
}

func AgentTraceReport(c *gin.Context) {
	server, ok := agentServer(c)
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64*1024)
	var req struct {
		Action    string `json:"action"`
		PrepareID string `json:"prepareId"`
		ID        uint   `json:"id"`
		Tool      string `json:"tool"`
		Output    string `json:"output"`
		Error     string `json:"error"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.Output) > tracecore.MaxOutput || len(req.Error) > 512 {
		c.JSON(400, gin.H{"code": "40000", "msg": "回报内容过大或格式无效"})
		return
	}

	if req.Action == "prepare" {
		if len(req.PrepareID) != 32 {
			c.JSON(400, gin.H{"code": "40000", "msg": "准备标识无效"})
			return
		}
		message := req.Error
		if req.Tool != "nexttrace" && message == "" {
			message = "准备结束但未检测到匹配版本的 NextTrace"
		}
		now := time.Now()
		updates := map[string]any{"prepare_finished_at": now, "prepare_error": message, "seen_at": now}
		if message == "" {
			updates["tool"] = "nexttrace"
		}
		tx := models.DB.Model(&models.TraceAgentCapability{}).Where("server_id = ? AND prepare_id = ? AND prepare_started_at IS NOT NULL AND prepare_finished_at IS NULL AND prepare_requested_at > ?", server.ID, req.PrepareID, now.Add(-3*time.Minute)).Updates(updates)
		if tx.Error != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "保存准备结果失败"})
			return
		}
		if tx.RowsAffected != 1 {
			c.JSON(409, gin.H{"code": "40900", "msg": "准备任务不属于此 agent 或已经结束"})
			return
		}
		c.JSON(200, gin.H{"code": "00000", "msg": "准备结果已保存"})
		return
	}
	if req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "追踪标识无效"})
		return
	}
	if req.Tool != "traceroute" && req.Tool != "tracepath" && req.Tool != "nexttrace" {
		req.Tool = ""
	}
	status := "completed"
	if strings.TrimSpace(req.Error) != "" {
		status = "failed"
	}
	now := time.Now()
	tx := models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND agent_server_id = ? AND status = ?", req.ID, server.ID, "running").Updates(map[string]any{"status": status, "tool": req.Tool, "output": strings.ToValidUTF8(req.Output, "�"), "error": req.Error, "finished_at": now})
	if tx.Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存回报失败"})
		return
	}
	if tx.RowsAffected != 1 {
		c.JSON(409, gin.H{"code": "40900", "msg": "任务不属于此服务器、已取消或已结束"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已保存"})
}

func traceRunOptions(run models.NetworkTraceRun) tracecore.Options {
	return tracecore.Options{Protocol: run.Protocol, Port: run.Port, Family: run.Family}
}

func markTraceUnsupported(id uint, message string) {
	models.DB.Model(&models.NetworkTraceView{}).Where("id = ? AND status = ?", id, "queued").Updates(map[string]any{"status": "unsupported", "error": message, "finished_at": time.Now()})
}

func tracePreparing(cap models.TraceAgentCapability, now time.Time) bool {
	return cap.PrepareID != "" && cap.PrepareRequestedAt != nil && cap.PrepareFinishedAt == nil && now.Sub(*cap.PrepareRequestedAt) < 3*time.Minute
}
func tracePrepareError(cap models.TraceAgentCapability, now time.Time) string {
	if cap.PrepareID != "" && cap.PrepareRequestedAt != nil && cap.PrepareFinishedAt == nil && !tracePreparing(cap, now) {
		return "准备任务未在 3 分钟内完成，请重试"
	}
	return cap.PrepareError
}

// NetworkTracePrepare queues installation only after an authenticated admin action.
// Agent token and a one-use nonce bind the assignment and its report.
func NetworkTracePrepare(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	var req struct {
		AgentIDs []uint `json:"agentIds"`
	}
	if c.ShouldBindJSON(&req) != nil || len(req.AgentIDs) == 0 || len(req.AgentIDs) > 8 {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择 1–8 个 agent"})
		return
	}
	traceCreateMu.Lock()
	defer traceCreateMu.Unlock()
	now := time.Now()
	seen := map[uint]bool{}
	caps := make([]models.TraceAgentCapability, 0, len(req.AgentIDs))
	for _, id := range req.AgentIDs {
		if id == 0 || seen[id] {
			c.JSON(400, gin.H{"code": "40000", "msg": "agent 标识无效或重复"})
			return
		}
		seen[id] = true
		var server models.ProxyServer
		var cap models.TraceAgentCapability
		if models.DB.First(&server, "id = ? AND kind = ? AND enabled = ?", id, models.ServerKindAgent, true).Error != nil || models.DB.First(&cap, "server_id = ?", id).Error != nil || cap.Version < tracecore.ClientVersion || now.Sub(cap.SeenAt) > 2*time.Minute {
			c.JSON(400, gin.H{"code": "40000", "msg": "请先更新 agent 并等待其上线"})
			return
		}
		if tracePreparing(cap, now) {
			c.JSON(409, gin.H{"code": "40900", "msg": "所选 agent 正在准备 NextTrace"})
			return
		}
		caps = append(caps, cap)
	}
	tx := models.DB.Begin()
	if tx.Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建准备任务失败"})
		return
	}
	for _, cap := range caps {
		nonce := make([]byte, 16)
		if _, err := rand.Read(nonce); err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"code": "50000", "msg": "生成准备标识失败"})
			return
		}
		if err := tx.Model(&models.TraceAgentCapability{}).Where("server_id = ?", cap.ServerID).Updates(map[string]any{"prepare_id": hex.EncodeToString(nonce), "prepare_requested_at": now, "prepare_started_at": nil, "prepare_finished_at": nil, "prepare_error": ""}).Error; err != nil {
			tx.Rollback()
			c.JSON(500, gin.H{"code": "50000", "msg": "保存准备任务失败"})
			return
		}
	}
	if tx.Commit().Error != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存准备任务失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "已安排准备 NextTrace v" + tracecore.NextTraceVersion + "；agent 将校验官方 SHA256 后安装"})
}
