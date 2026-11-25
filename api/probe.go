package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
)

// probeOnlineWindow 用户 Agent 每 25 秒长轮询一次，超过 60 秒无心跳视为离线。
const probeOnlineWindow = 60 * time.Second

// latestUserAgentVersion 与 userAgent 分支保持同步。
const latestUserAgentVersion = "1.12"

type probeNode struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Scheme string `json:"scheme,omitempty"`
	SNI    string `json:"sni,omitempty"`
}

type probeReport struct {
	JobID   string `json:"jobId,omitempty"`
	Results []struct {
		NodeID    int    `json:"nodeId"`
		Kind      string `json:"kind"`
		Success   bool   `json:"success"`
		RTT       int    `json:"rtt"`
		Error     string `json:"error"`
		IPVersion int    `json:"ipVersion"`
	} `json:"results"`
}

type userAgentConfigPatchDTO struct {
	PanelURL       string `json:"panelUrl,omitempty"`
	Concurrency    int    `json:"concurrency,omitempty"`
	ReportPublicIP *bool  `json:"reportPublicIp,omitempty"`
	AllowClash     *bool  `json:"allowClash,omitempty"`
	ClashAPI       string `json:"clashApi,omitempty"`
	ClashSecret    string `json:"clashSecret,omitempty"`
}

type queuedProbeJob struct {
	ID          string
	Action      string
	TargetHost  string
	SpeedMode   string
	ClashAPI    string
	ClashSecret string
	ClashGroup  string
	ClashNode   string
	ClashNodes  []string
	Config      *userAgentConfigPatchDTO
	Nodes       []probeNode
}

type jobOutcome struct {
	OK    bool
	Error string
}

// userProbeHub coordinates on-demand test/update/uninstall dispatches to connected User Agents.
// When idle, User Agents receive no jobs and send zero traffic to proxy nodes.
type userProbeHub struct {
	mu      sync.Mutex
	waiters map[uint]chan queuedProbeJob
	pending map[uint][]queuedProbeJob
	done    map[string]chan jobOutcome
}

var probeHub = &userProbeHub{
	waiters: map[uint]chan queuedProbeJob{},
	pending: map[uint][]queuedProbeJob{},
	done:    map[string]chan jobOutcome{},
}

func (h *userProbeHub) register(probeID uint) chan queuedProbeJob {
	h.mu.Lock()
	defer h.mu.Unlock()
	ch := make(chan queuedProbeJob, 1)
	if q := h.pending[probeID]; len(q) > 0 {
		ch <- q[0]
		if len(q) == 1 {
			delete(h.pending, probeID)
		} else {
			h.pending[probeID] = q[1:]
		}
		return ch
	}
	h.waiters[probeID] = ch
	return ch
}

func (h *userProbeHub) unregister(probeID uint, ch chan queuedProbeJob) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if cur, ok := h.waiters[probeID]; ok && cur == ch {
		delete(h.waiters, probeID)
	}
}

func (h *userProbeHub) dispatchAction(probeID uint, action string, nodes []probeNode, allowQueue bool) (string, chan jobOutcome, bool) {
	return h.dispatchJob(probeID, queuedProbeJob{Action: action, Nodes: nodes}, allowQueue)
}

func newProbeJobID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *userProbeHub) dispatchJob(probeID uint, job queuedProbeJob, allowQueue bool) (string, chan jobOutcome, bool) {
	jobID := job.ID
	if jobID == "" {
		jobID = newProbeJobID()
	}
	doneCh := make(chan jobOutcome, 1)
	job.ID = jobID

	h.mu.Lock()
	ch, waiting := h.waiters[probeID]
	if waiting {
		delete(h.waiters, probeID)
		h.done[jobID] = doneCh
		h.mu.Unlock()
		select {
		case ch <- job:
			return jobID, doneCh, true
		default:
			h.mu.Lock()
			if allowQueue && len(h.pending[probeID]) < 4 {
				h.pending[probeID] = append(h.pending[probeID], job)
				h.mu.Unlock()
				return jobID, doneCh, true
			}
			delete(h.done, jobID)
			h.mu.Unlock()
			return "", nil, false
		}
	}
	if allowQueue && len(h.pending[probeID]) < 4 {
		h.done[jobID] = doneCh
		h.pending[probeID] = append(h.pending[probeID], job)
		h.mu.Unlock()
		return jobID, doneCh, true
	}
	h.mu.Unlock()
	return "", nil, false
}

func (h *userProbeHub) dispatch(probeID uint, nodes []probeNode, allowQueue bool) (string, chan jobOutcome, bool) {
	return h.dispatchAction(probeID, "probe", nodes, allowQueue)
}

func (h *userProbeHub) complete(jobID string) {
	h.completeWith(jobID, jobOutcome{OK: true})
}

func (h *userProbeHub) completeWith(jobID string, outcome jobOutcome) {
	if jobID == "" {
		return
	}
	h.mu.Lock()
	ch, ok := h.done[jobID]
	if ok {
		delete(h.done, jobID)
	}
	h.mu.Unlock()
	if ok {
		select {
		case ch <- outcome:
		default:
		}
		close(ch)
	}
}

func (h *userProbeHub) cleanupDone(jobID string) {
	h.mu.Lock()
	delete(h.done, jobID)
	h.mu.Unlock()
}

func (h *userProbeHub) isWaiting(probeID uint) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.waiters[probeID]
	return ok
}

func isProbeRecentlyOnline(p *models.Probe) bool {
	if p == nil {
		return false
	}
	return probeHub.isWaiting(p.ID) || (p.LastSeenAt != nil && time.Since(*p.LastSeenAt) < probeOnlineWindow)
}

func extractNodeScheme(link string) string {
	link = strings.TrimSpace(link)
	if idx := strings.Index(link, "://"); idx > 0 {
		return strings.ToLower(strings.TrimSpace(link[:idx]))
	}
	return ""
}

func extractNodeSNI(link string) string {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return ""
	}
	q := u.Query()
	for _, k := range []string{"sni", "servername", "peer"} {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func buildProbeNodes(filterIDs []int) ([]probeNode, error) {
	nodes, err := models.GetNodeList()
	if err != nil {
		return nil, err
	}
	filter := make(map[int]bool, len(filterIDs))
	for _, id := range filterIDs {
		if id > 0 {
			filter[id] = true
		}
	}
	items := make([]probeNode, 0, len(nodes))
	for _, n := range nodes {
		if len(filter) > 0 && !filter[n.ID] {
			continue
		}
		host, port := node.ExtractServerHost(n.Link)
		if host == "" || port <= 0 {
			continue
		}
		items = append(items, probeNode{
			ID:     n.ID,
			Name:   n.Name,
			Host:   host,
			Port:   port,
			Scheme: extractNodeScheme(n.Link),
			SNI:    extractNodeSNI(n.Link),
		})
	}
	return items, nil
}

func currentProbe(c *gin.Context) (*models.Probe, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Probe "))
	if token == "" {
		token = strings.TrimSpace(c.Query("token"))
	}
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "缺少用户 Agent 令牌"})
		return nil, false
	}
	var probe models.Probe
	if err := models.DB.Where("token_hash = ? AND enabled = ?", models.HashProbeToken(token), true).
		First(&probe).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "用户 Agent 令牌无效"})
		return nil, false
	}
	return &probe, true
}

func probeInstallCommands(c *gin.Context, token string) gin.H {
	panel := strings.TrimSuffix(baseURL(c), "/")
	return gin.H{
		"windows": fmt.Sprintf(
			`$f="$env:TEMP\ppeelink-ua-setup.exe"; $d="$env:LOCALAPPDATA\PPEELinkUserAgent"; Write-Host "正在下载并安装 PPEELink 用户 Agent..." -ForegroundColor Cyan; Invoke-WebRequest -UseBasicParsing -Headers @{Authorization="Probe %s"} -Uri "%s/api/v1/probe/download?os=windows&arch=amd64" -OutFile $f; $p=Start-Process -PassThru -FilePath $f -ArgumentList "-install","-panel","%s","-token","%s"; $null=$p.WaitForExit(15000); Remove-Item -Force -ErrorAction SilentlyContinue $f; if (Test-Path "$d\ppeelink-user-agent.exe") { Write-Host ""; Write-Host "==========================================" -ForegroundColor Green; Write-Host "  PPEELink 用户 Agent 安装成功" -ForegroundColor Green; Write-Host "==========================================" -ForegroundColor Green; Write-Host "  运行状态 : 已后台静默启动（无窗口，开机自启）"; Write-Host "  安装目录 : $d"; Write-Host "  主程序   : $d\ppeelink-user-agent.exe"; Write-Host "  配置文件 : $d\config.json"; Write-Host "  自启项   : HKCU\Software\Microsoft\Windows\CurrentVersion\Run"; Write-Host "  面板管理 : 可在网页端随时发起实测、远程更新或无痕卸载"; Write-Host "==========================================" -ForegroundColor Green } else { Write-Host "安装未完成，请检查网络或杀毒软件拦截后重试" -ForegroundColor Red }; Write-Host ""; Write-Host "按任意键关闭此窗口..." -ForegroundColor DarkGray; try { $null=$Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown") } catch { Read-Host | Out-Null }; exit`,
			token, panel, panel, token,
		),
		"macos": fmt.Sprintf(
			`curl -fsSL -H 'Authorization: Probe %s' '%s/api/v1/probe/download?os=darwin&arch=arm64' -o /tmp/ppeelink-ua-setup && chmod +x /tmp/ppeelink-ua-setup && /tmp/ppeelink-ua-setup -install -panel '%s' -token '%s' && printf "\n按回车键关闭..." && read _ && exit`,
			token, panel, panel, token,
		),
		"linux": fmt.Sprintf(
			`curl -fsSL -H 'Authorization: Probe %s' '%s/api/v1/probe/download?os=linux&arch=amd64' -o /tmp/ppeelink-ua-setup && chmod +x /tmp/ppeelink-ua-setup && /tmp/ppeelink-ua-setup -install -panel '%s' -token '%s' && printf "\n按回车键关闭..." && read _ && exit`,
			token, panel, panel, token,
		),
	}
}

func probeInstallCommand(c *gin.Context, token string) string {
	cmds := probeInstallCommands(c, token)
	return cmds["linux"].(string)
}

// ProbeList 管理端：用户 Agent 列表。
func ProbeList(c *gin.Context) {
	probes, err := models.GetProbes(false)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	now := time.Now()
	items := make([]gin.H, 0, len(probes))
	for _, p := range probes {
		online := probeHub.isWaiting(p.ID) || (p.LastSeenAt != nil && now.Sub(*p.LastSeenAt) < probeOnlineWindow)
		cGroup, cNow := getProbeClashLive(p.ID)
		items = append(items, gin.H{
			"id": p.ID, "name": p.Name, "region": p.Region, "enabled": p.Enabled,
			"autoProbe": p.AutoProbe, "hasIpv6": p.HasIPv6, "isp": p.ISP, "iface": p.IfaceName, "publicIp": p.PublicIP,
			"reportPublicIp": p.ReportPublicIP == nil || *p.ReportPublicIP,
			"allowClash":     p.AllowClash == nil || *p.AllowClash,
			"concurrency":    p.Concurrency,
			"clashApi":       p.ClashAPI,
			"hasClashSecret": p.ClashSecret != "",
			"clashGroup":     cGroup, "clashNow": cNow,
			"online": online, "lastSeenAt": p.LastSeenAt, "version": p.Version,
			"latestVersion":   latestUserAgentVersion,
			"updateAvailable": p.Version != "" && p.Version != latestUserAgentVersion,
		})
	}
	c.JSON(200, gin.H{"code": "00000", "data": items, "msg": "用户 Agent 列表"})
}

// ProbeAdd 管理端：新建用户 Agent，令牌只返回一次。
func ProbeAdd(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Region string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "名称不能为空"})
		return
	}
	token := models.GenerateToken()
	probe := models.Probe{
		Name: strings.TrimSpace(req.Name), Region: strings.TrimSpace(req.Region),
		TokenHash: models.HashProbeToken(token), Enabled: true,
	}
	if err := models.DB.Create(&probe).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"id": probe.ID, "name": probe.Name, "token": token,
		"install":  probeInstallCommand(c, token),
		"commands": probeInstallCommands(c, token),
	}, "msg": "用户 Agent 已创建，令牌只显示这一次"})
}

// ProbeDelete 管理端：删除用户 Agent；若在线则先下发无痕卸载指令清除本机所有文件与自启项。
func ProbeDelete(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 无效"})
		return
	}
	var probe models.Probe
	_ = models.DB.First(&probe, req.ID).Error
	uninstalled := false
	if jobID, doneCh, ok := probeHub.dispatchAction(req.ID, "uninstall", nil, isProbeRecentlyOnline(&probe)); ok {
		defer probeHub.cleanupDone(jobID)
		select {
		case <-doneCh:
			uninstalled = true
		case <-time.After(5 * time.Second):
			uninstalled = true
		}
	}
	models.DB.Where("probe_id = ?", req.ID).Delete(&models.ProbeResult{})
	if err := models.DB.Delete(&models.Probe{}, req.ID).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除失败"})
		return
	}
	InvalidateOverview()
	msg := "已从面板移除"
	if uninstalled {
		msg = "已通知本机无痕卸载所有文件与自启项，并从面板移除"
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"uninstalled": uninstalled}, "msg": msg})
}

// ProbeUpdate 管理端：向在线用户 Agent 下发远程静默更新指令。
func ProbeUpdate(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 无效"})
		return
	}
	var probe models.Probe
	if err := models.DB.First(&probe, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户 Agent 不存在"})
		return
	}
	jobID, doneCh, ok := probeHub.dispatchAction(req.ID, "update", nil, isProbeRecentlyOnline(&probe))
	if !ok {
		c.JSON(400, gin.H{"code": "40000", "msg": "该用户 Agent 当前离线，无法远程更新"})
		return
	}
	defer probeHub.cleanupDone(jobID)
	select {
	case res := <-doneCh:
		if !res.OK {
			c.JSON(500, gin.H{"code": "50000", "msg": "更新失败: " + res.Error})
			return
		}
		c.JSON(200, gin.H{"code": "00000", "msg": "已下发更新，用户 Agent 正在静默重启升级至 v" + latestUserAgentVersion})
	case <-time.After(15 * time.Second):
		c.JSON(200, gin.H{"code": "00000", "msg": "已下发更新指令，请稍候刷新查看新版本"})
	case <-c.Request.Context().Done():
		return
	}
}

// ProbeAck 用户 Agent 回报 update / uninstall 指令确认。
func ProbeAck(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	var req struct {
		JobID  string `json:"jobId"`
		Action string `json:"action"`
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	probeHub.completeWith(req.JobID, jobOutcome{OK: req.OK, Error: req.Error})
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// ProbeResetToken 管理端：重置用户 Agent 令牌。
func ProbeResetToken(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "id 无效"})
		return
	}
	var probe models.Probe
	if err := models.DB.First(&probe, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户 Agent 不存在"})
		return
	}
	token := models.GenerateToken()
	if err := models.DB.Model(&probe).Update("token_hash", models.HashProbeToken(token)).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "重置失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"token":    token,
		"install":  probeInstallCommand(c, token),
		"commands": probeInstallCommands(c, token),
	}, "msg": "令牌已重置"})
}

// ProbeDownload serves the cross-platform standalone User Agent binary.
func ProbeDownload(c *gin.Context) {
	osName := strings.ToLower(strings.TrimSpace(c.DefaultQuery("os", "linux")))
	arch := strings.ToLower(strings.TrimSpace(c.DefaultQuery("arch", "amd64")))
	allowed := map[string]string{
		"windows/amd64": "ppeelink-user-agent-windows-amd64.exe",
		"darwin/arm64":  "ppeelink-user-agent-darwin-arm64",
		"darwin/amd64":  "ppeelink-user-agent-darwin-amd64",
		"linux/amd64":   "ppeelink-user-agent-linux-amd64",
		"linux/arm64":   "ppeelink-user-agent-linux-arm64",
	}
	filename, ok := allowed[osName+"/"+arch]
	if !ok {
		c.JSON(400, gin.H{"code": "40000", "msg": "不支持的系统架构"})
		return
	}
	fullPath := filepath.Join("downloads", "user-agent", filename)
	if _, err := os.Stat(fullPath); err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器上尚未放置用户 Agent 二进制文件: " + filename})
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.File(fullPath)
}

// ProbeDownloadAuth wraps ProbeDownload with Probe <token> authentication.
func ProbeDownloadAuth(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	ProbeDownload(c)
}

// ProbeBinary serves the standalone User Agent binary for legacy callers.
func ProbeBinary(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	ProbeDownload(c)
}

// ProbeWait 长轮询等待管理端触发按需实测任务（空闲时不返回节点、不发包）。
func ProbeWait(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	updates := map[string]any{"last_seen_at": time.Now()}
	if v := strings.TrimSpace(c.Query("version")); v != "" && len(v) <= 32 {
		updates["version"] = v
	}
	if v6 := strings.TrimSpace(c.Query("hasIpv6")); v6 == "1" || v6 == "0" {
		b := v6 == "1"
		updates["has_ipv6"] = b
	}
	if isp := strings.TrimSpace(c.Query("isp")); isp != "" && len(isp) <= 64 {
		updates["isp"] = isp
	}
	if iface := strings.TrimSpace(c.Query("iface")); iface != "" && len(iface) <= 64 {
		updates["iface_name"] = iface
	}
	pubIP := strings.TrimSpace(c.Query("publicIp"))
	if pubIP == "" || isPrivateOrLAN(pubIP) {
		if realIP := extractRealPublicIP(c); realIP != "" && !isKnownProxyServerIP(realIP) {
			pubIP = realIP
		}
	}
	if pubIP != "" && len(pubIP) <= 64 {
		updates["public_ip"] = pubIP
	}
	models.DB.Model(probe).Updates(updates)
	if _, hasISP := updates["isp"]; !hasISP && strings.TrimSpace(probe.ISP) == "" && pubIP != "" {
		go enrichProbeISPAsync(probe.ID, pubIP)
	}
	if cNow := strings.TrimSpace(c.Query("clashNow")); cNow != "" || c.Query("clashGroup") != "" {
		recordProbeClashLive(probe.ID, strings.TrimSpace(c.Query("clashGroup")), cNow)
	}

	waitSec, _ := strconv.Atoi(c.DefaultQuery("timeout", "25"))
	if waitSec < 5 {
		waitSec = 5
	}
	if waitSec > 30 {
		waitSec = 30
	}

	ch := probeHub.register(probe.ID)
	defer probeHub.unregister(probe.ID, ch)

	timer := time.NewTimer(time.Duration(waitSec) * time.Second)
	defer timer.Stop()

	select {
	case job := <-ch:
		act := job.Action
		if act == "" {
			act = "probe"
		}
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{
			"jobId":       job.ID,
			"action":      act,
			"targetHost":  job.TargetHost,
			"speedMode":   job.SpeedMode,
			"clashApi":    job.ClashAPI,
			"clashSecret": job.ClashSecret,
			"clashGroup":  job.ClashGroup,
			"clashNode":   job.ClashNode,
			"clashNodes":  job.ClashNodes,
			"config":      job.Config,
			"nodes":       job.Nodes,
		}, "msg": "dispatched"})
	case <-timer.C:
		models.DB.Model(probe).Update("last_seen_at", time.Now())
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{
			"jobId":  "",
			"action": "",
			"nodes":  []probeNode{},
		}, "msg": "idle"})
	case <-c.Request.Context().Done():
		return
	}
}

// ProbeJobs 单次模式（-once）直接拉取待测节点列表。
func ProbeJobs(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	items, err := buildProbeNodes(nil)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	models.DB.Model(probe).Update("last_seen_at", time.Now())
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"probe": gin.H{"id": probe.ID, "name": probe.Name},
		"nodes": items,
	}, "msg": "探针任务"})
}

// ProbeTrigger 管理端：按需触发一次用户主机实测（仅在用户点击时执行）。
func ProbeTrigger(c *gin.Context) {
	var req struct {
		ProbeID uint  `json:"probeId"`
		NodeIDs []int `json:"nodeIds"`
	}
	_ = c.ShouldBindJSON(&req)

	var target *models.Probe
	probes, err := models.GetProbes(true)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return
	}
	var fallback *models.Probe
	for i := range probes {
		p := &probes[i]
		if req.ProbeID > 0 && p.ID != req.ProbeID {
			continue
		}
		if probeHub.isWaiting(p.ID) {
			target = p
			break
		}
		if fallback == nil && isProbeRecentlyOnline(p) {
			fallback = p
		}
	}
	if target == nil {
		target = fallback
	}
	if target == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent，请先在本机启动用户 Agent"})
		return
	}
	nodes, err := buildProbeNodes(req.NodeIDs)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点列表失败"})
		return
	}
	if len(nodes) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有可测试的节点"})
		return
	}
	jobID, doneCh, ok := probeHub.dispatch(target.ID, nodes, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 待处理任务过多，请稍后再试"})
		return
	}
	defer probeHub.cleanupDone(jobID)

	select {
	case <-doneCh:
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{
			"jobId": jobID, "completed": true, "probeName": target.Name, "count": len(nodes),
		}, "msg": fmt.Sprintf("「%s」已完成 %d 个节点实测", target.Name, len(nodes))})
	case <-time.After(18 * time.Second):
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{
			"jobId": jobID, "completed": false, "probeName": target.Name, "count": len(nodes),
		}, "msg": "已下发给用户 Agent，结果稍后自动更新"})
	case <-c.Request.Context().Done():
		return
	}
}

// ProbeReport 探针回报检测结果。
func ProbeReport(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	var report probeReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "回报格式错误"})
		return
	}
	if len(report.Results) > 1000 {
		c.JSON(400, gin.H{"code": "40000", "msg": "单次回报节点过多"})
		return
	}
	now := time.Now()
	results := make([]models.ProbeResult, 0, len(report.Results))
	for _, item := range report.Results {
		if item.NodeID <= 0 {
			continue
		}
		kind := item.Kind
		if kind == "" {
			kind = "tcp"
		}
		results = append(results, models.ProbeResult{
			ProbeID: probe.ID, NodeID: item.NodeID, Kind: kind,
			Success: item.Success, RTT: item.RTT, Error: item.Error,
			IPVersion: item.IPVersion, CheckedAt: now,
		})
	}
	if err := models.RecordProbeResults(probe.ID, results, now); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录失败"})
		return
	}
	InvalidateOverview()
	probeHub.complete(report.JobID)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// NodeProbePolicy 兼容保留接口（前端已移除旧多视角策略选择，统一以用户 Agent 实测为准）。
func NodeProbePolicy(c *gin.Context) {
	var req struct {
		ID     int    `json:"id"`
		Policy string `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "节点 id 无效"})
		return
	}
	switch req.Policy {
	case "panel", "any", "all", "manual":
	default:
		c.JSON(400, gin.H{"code": "40000", "msg": "检测策略无效"})
		return
	}
	var n models.Node
	if err := models.DB.First(&n, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "节点不存在"})
		return
	}
	if err := models.DB.Model(&n).Update("probe_policy", req.Policy).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "msg": "检测策略已保存"})
}
