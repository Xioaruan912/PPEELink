package api

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

const (
	monitorOnlineWindow = 3 * time.Minute
	monitorInterval     = 60
	monitorMaxPoints    = 360
)

type monitorReport struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Version  string `json:"version"`
	Metrics  struct {
		CPUPercent   float64 `json:"cpuPercent"`
		MemUsed      uint64  `json:"memUsed"`
		MemTotal     uint64  `json:"memTotal"`
		SwapUsed     uint64  `json:"swapUsed"`
		SwapTotal    uint64  `json:"swapTotal"`
		DiskUsed     uint64  `json:"diskUsed"`
		DiskTotal    uint64  `json:"diskTotal"`
		Load1        float64 `json:"load1"`
		Load5        float64 `json:"load5"`
		Load15       float64 `json:"load15"`
		Uptime       uint64  `json:"uptime"`
		NetInSpeed   uint64  `json:"netInSpeed"`
		NetOutSpeed  uint64  `json:"netOutSpeed"`
		TransferIn   uint64  `json:"transferIn"`
		TransferOut  uint64  `json:"transferOut"`
		TCPConns     int     `json:"tcpConns"`
		UDPConns     int     `json:"udpConns"`
		ProcessCount int     `json:"processCount"`
	} `json:"metrics"`
}

func currentMonitor(c *gin.Context) (*models.Monitor, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Monitor "))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "缺少监控令牌"})
		return nil, false
	}
	var monitor models.Monitor
	if err := models.DB.Where("token_hash = ? AND enabled = ?", models.HashMonitorToken(token), true).
		First(&monitor).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "监控令牌无效"})
		return nil, false
	}
	return &monitor, true
}

func monitorInstallCommand(c *gin.Context, token string) string {
	panel := strings.TrimSuffix(baseURL(c), "/")
	return fmt.Sprintf(`curl -fsSL -H 'Authorization: Monitor %s' '%s/api/v1/monitor/binary' -o /usr/local/bin/ppeelink-monitor
chmod +x /usr/local/bin/ppeelink-monitor
cat >/etc/systemd/system/ppeelink-monitor.service <<'UNIT'
[Unit]
Description=PPEELink monitor
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/ppeelink-monitor monitor -panel '%s' -token '%s' -interval 60s
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now ppeelink-monitor`, token, panel, panel, token)
}

func monitorOnline(monitor models.Monitor, now time.Time) bool {
	return monitor.LastSeenAt != nil && now.Sub(*monitor.LastSeenAt) < monitorOnlineWindow
}

// MonitorList 管理端：服务器列表 + 最新指标。
func MonitorList(c *gin.Context) {
	var monitors []models.Monitor
	if err := models.DB.Order("id asc").Find(&monitors).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取服务器失败"})
		return
	}
	now := time.Now()
	items := make([]gin.H, 0, len(monitors))
	online := 0
	for _, m := range monitors {
		var latest models.MonitorMetric
		hasLatest := models.DB.Where("monitor_id = ?", m.ID).Order("collected_at desc").First(&latest).Error == nil
		item := gin.H{
			"id": m.ID, "name": m.Name, "group": m.Group, "note": m.Note,
			"enabled": m.Enabled, "hidden": m.Hidden, "os": m.OS, "arch": m.Arch,
			"version": m.Version, "lastSeenAt": m.LastSeenAt, "createdAt": m.CreatedAt,
			"online": monitorOnline(m, now), "sshConfigured": monitorSSHConfigured(m),
		}
		if hasLatest {
			item["latest"] = latest
		}
		if monitorOnline(m, now) {
			online++
		}
		items = append(items, item)
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"items":   items,
		"summary": gin.H{"total": len(monitors), "online": online, "offline": len(monitors) - online},
	}, "msg": "服务器列表"})
}

// MonitorAdd 管理端：新增服务器，令牌只返回一次。
func MonitorAdd(c *gin.Context) {
	var req struct {
		Name  string `json:"name"`
		Group string `json:"group"`
		Note  string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器名称不能为空"})
		return
	}
	token := models.GenerateToken()
	monitor := models.Monitor{
		Name: strings.TrimSpace(req.Name), Group: strings.TrimSpace(req.Group),
		Note: strings.TrimSpace(req.Note), TokenHash: models.HashMonitorToken(token), Enabled: true,
	}
	if err := models.DB.Create(&monitor).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建服务器失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"id": monitor.ID, "name": monitor.Name, "token": token,
		"install": monitorInstallCommand(c, token),
	}, "msg": "服务器已创建，令牌只显示这一次"})
}

// MonitorDelete 管理端：删除服务器及其指标。
func MonitorDelete(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 id 无效"})
		return
	}
	models.DB.Where("monitor_id = ?", req.ID).Delete(&models.MonitorMetric{})
	if err := models.DB.Delete(&models.Monitor{}, req.ID).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除服务器失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "服务器已删除"})
}

// MonitorResetToken 管理端：重置监控令牌。
func MonitorResetToken(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 id 无效"})
		return
	}
	var monitor models.Monitor
	if err := models.DB.First(&monitor, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	token := models.GenerateToken()
	if err := models.DB.Model(&monitor).Update("token_hash", models.HashMonitorToken(token)).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "重置失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"token": token, "install": monitorInstallCommand(c, token),
	}, "msg": "令牌已重置"})
}

// MonitorDetail 管理端：服务器详情与历史指标。
func MonitorDetail(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	if id <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "服务器 id 无效"})
		return
	}
	var monitor models.Monitor
	if err := models.DB.First(&monitor, id).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	var latest models.MonitorMetric
	hasLatest := models.DB.Where("monitor_id = ?", monitor.ID).Order("collected_at desc").First(&latest).Error == nil

	rangeKey := c.DefaultQuery("range", "24h")
	duration := 24 * time.Hour
	switch rangeKey {
	case "1h":
		duration = time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	}
	var rows []models.MonitorMetric
	models.DB.Where("monitor_id = ? AND collected_at >= ?", monitor.ID, time.Now().Add(-duration)).
		Order("collected_at asc").Find(&rows)
	rows = downsampleMonitorMetrics(rows, monitorMaxPoints)

	data := gin.H{
		"monitor": monitor, "online": monitorOnline(monitor, time.Now()),
		"sshConfigured": monitorSSHConfigured(monitor),
		"metrics":       rows, "range": rangeKey,
	}
	if hasLatest {
		data["latest"] = latest
	}
	c.JSON(200, gin.H{"code": "00000", "data": data, "msg": "服务器详情"})
}

// downsampleMonitorMetrics 把长时间范围压到最多 max 个点，避免一次返回几万行。
func downsampleMonitorMetrics(rows []models.MonitorMetric, max int) []models.MonitorMetric {
	if len(rows) <= max || max <= 0 {
		return rows
	}
	bucket := len(rows)/max + 1
	out := make([]models.MonitorMetric, 0, max)
	for start := 0; start < len(rows); start += bucket {
		end := start + bucket
		if end > len(rows) {
			end = len(rows)
		}
		part := rows[start:end]
		var m models.MonitorMetric
		var n float64 = float64(len(part))
		m.MonitorID = part[0].MonitorID
		m.CollectedAt = part[len(part)-1].CollectedAt
		for _, p := range part {
			m.CPUPercent += p.CPUPercent
			m.MemUsed += p.MemUsed
			m.MemTotal += p.MemTotal
			m.SwapUsed += p.SwapUsed
			m.SwapTotal += p.SwapTotal
			m.DiskUsed += p.DiskUsed
			m.DiskTotal += p.DiskTotal
			m.Load1 += p.Load1
			m.Load5 += p.Load5
			m.Load15 += p.Load15
			m.NetInSpeed += p.NetInSpeed
			m.NetOutSpeed += p.NetOutSpeed
			m.TCPConns += p.TCPConns
			m.UDPConns += p.UDPConns
			m.ProcessCount += p.ProcessCount
			m.Uptime = p.Uptime
			m.TransferIn = p.TransferIn
			m.TransferOut = p.TransferOut
		}
		m.CPUPercent /= n
		m.MemUsed = uint64(float64(m.MemUsed) / n)
		m.MemTotal = uint64(float64(m.MemTotal) / n)
		m.SwapUsed = uint64(float64(m.SwapUsed) / n)
		m.SwapTotal = uint64(float64(m.SwapTotal) / n)
		m.DiskUsed = uint64(float64(m.DiskUsed) / n)
		m.DiskTotal = uint64(float64(m.DiskTotal) / n)
		m.Load1 /= n
		m.Load5 /= n
		m.Load15 /= n
		m.NetInSpeed = uint64(float64(m.NetInSpeed) / n)
		m.NetOutSpeed = uint64(float64(m.NetOutSpeed) / n)
		m.TCPConns = int(float64(m.TCPConns) / n)
		m.UDPConns = int(float64(m.UDPConns) / n)
		m.ProcessCount = int(float64(m.ProcessCount) / n)
		out = append(out, m)
	}
	return out
}

// MonitorConfig 监控端：拉取上报间隔。
func MonitorConfig(c *gin.Context) {
	monitor, ok := currentMonitor(c)
	if !ok {
		return
	}
	now := time.Now()
	models.DB.Model(monitor).Update("last_seen_at", now)
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"monitor":         gin.H{"id": monitor.ID, "name": monitor.Name},
		"intervalSeconds": monitorInterval,
	}, "msg": "监控配置"})
}

// MonitorReport 监控端：上报服务器指标。
func MonitorReport(c *gin.Context) {
	monitor, ok := currentMonitor(c)
	if !ok {
		return
	}
	var report monitorReport
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "上报格式错误"})
		return
	}
	now := time.Now()
	metric := models.MonitorMetric{
		MonitorID: monitor.ID, CollectedAt: now,
		CPUPercent: report.Metrics.CPUPercent,
		MemUsed:    report.Metrics.MemUsed, MemTotal: report.Metrics.MemTotal,
		SwapUsed: report.Metrics.SwapUsed, SwapTotal: report.Metrics.SwapTotal,
		DiskUsed: report.Metrics.DiskUsed, DiskTotal: report.Metrics.DiskTotal,
		Load1: report.Metrics.Load1, Load5: report.Metrics.Load5, Load15: report.Metrics.Load15,
		Uptime:     report.Metrics.Uptime,
		NetInSpeed: report.Metrics.NetInSpeed, NetOutSpeed: report.Metrics.NetOutSpeed,
		TransferIn: report.Metrics.TransferIn, TransferOut: report.Metrics.TransferOut,
		TCPConns: report.Metrics.TCPConns, UDPConns: report.Metrics.UDPConns,
		ProcessCount: report.Metrics.ProcessCount,
	}
	if err := models.DB.Create(&metric).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录指标失败"})
		return
	}
	updates := map[string]any{
		"last_seen_at": now,
		"os":           report.OS,
		"arch":         report.Arch,
		"version":      report.Version,
	}
	models.DB.Model(monitor).Updates(updates)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// MonitorBinary 下发与面板同架构的监控程序。
func MonitorBinary(c *gin.Context) {
	if _, ok := currentMonitor(c); !ok {
		return
	}
	if arch := c.Query("arch"); arch != "" && arch != runtime.GOARCH {
		c.JSON(400, gin.H{"code": "40000", "msg": fmt.Sprintf("面板程序为 %s 架构，无法下发 %s 版本", runtime.GOARCH, arch)})
		return
	}
	exe, err := os.Executable()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "找不到面板程序"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=ppeelink-monitor")
	c.File(exe)
}
