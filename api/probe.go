package api

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
)

// probeOnlineWindow 超过这个时间没有回报视为离线。
const probeOnlineWindow = 3 * time.Minute

type probeNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type probeReport struct {
	Results []struct {
		NodeID    int    `json:"nodeId"`
		Kind      string `json:"kind"`
		Success   bool   `json:"success"`
		RTT       int    `json:"rtt"`
		Error     string `json:"error"`
		IPVersion int    `json:"ipVersion"`
	} `json:"results"`
}

func currentProbe(c *gin.Context) (*models.Probe, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Probe "))
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "缺少探针令牌"})
		return nil, false
	}
	var probe models.Probe
	if err := models.DB.Where("token_hash = ? AND enabled = ?", models.HashProbeToken(token), true).
		First(&probe).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "探针令牌无效"})
		return nil, false
	}
	return &probe, true
}

func probeInstallCommand(c *gin.Context, token string) string {
	panel := strings.TrimSuffix(baseURL(c), "/")
	return fmt.Sprintf(`curl -fsSL -H 'Authorization: Probe %s' '%s/api/v1/probe/binary' -o /usr/local/bin/ppeelink-probe
chmod +x /usr/local/bin/ppeelink-probe
cat >/etc/systemd/system/ppeelink-probe.service <<'UNIT'
[Unit]
Description=PPEELink probe
After=network-online.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/ppeelink-probe probe -panel '%s' -token '%s' -interval 5m
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now ppeelink-probe`, token, panel, panel, token)
}

// ProbeList 管理端：探针列表。
func ProbeList(c *gin.Context) {
	probes, err := models.GetProbes(false)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取探针失败"})
		return
	}
	now := time.Now()
	items := make([]gin.H, 0, len(probes))
	for _, p := range probes {
		online := p.LastSeenAt != nil && now.Sub(*p.LastSeenAt) < probeOnlineWindow
		items = append(items, gin.H{
			"id": p.ID, "name": p.Name, "region": p.Region, "enabled": p.Enabled,
			"online": online, "lastSeenAt": p.LastSeenAt, "version": p.Version,
		})
	}
	c.JSON(200, gin.H{"code": "00000", "data": items, "msg": "探针列表"})
}

// ProbeAdd 管理端：新建探针，令牌只返回一次。
func ProbeAdd(c *gin.Context) {
	var req struct {
		Name   string `json:"name"`
		Region string `json:"region"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "探针名称不能为空"})
		return
	}
	token := models.GenerateToken()
	probe := models.Probe{
		Name: strings.TrimSpace(req.Name), Region: strings.TrimSpace(req.Region),
		TokenHash: models.HashProbeToken(token), Enabled: true,
	}
	if err := models.DB.Create(&probe).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建探针失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"id": probe.ID, "name": probe.Name, "token": token,
		"install": probeInstallCommand(c, token),
	}, "msg": "探针已创建，令牌只显示这一次"})
}

// ProbeDelete 管理端：删除探针。
func ProbeDelete(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "探针 id 无效"})
		return
	}
	models.DB.Where("probe_id = ?", req.ID).Delete(&models.ProbeResult{})
	if err := models.DB.Delete(&models.Probe{}, req.ID).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除探针失败"})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "msg": "探针已删除"})
}

// ProbeResetToken 管理端：重置探针令牌。
func ProbeResetToken(c *gin.Context) {
	var req struct {
		ID uint `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "探针 id 无效"})
		return
	}
	var probe models.Probe
	if err := models.DB.First(&probe, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "探针不存在"})
		return
	}
	token := models.GenerateToken()
	if err := models.DB.Model(&probe).Update("token_hash", models.HashProbeToken(token)).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "重置失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"token": token, "install": probeInstallCommand(c, token),
	}, "msg": "令牌已重置"})
}

// ProbeBinary 下发与面板同架构的探针程序。
func ProbeBinary(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
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
	c.Header("Content-Disposition", "attachment; filename=ppeelink-probe")
	c.File(exe)
}

// ProbeJobs 探针拉取待检测节点。
func ProbeJobs(c *gin.Context) {
	probe, ok := currentProbe(c)
	if !ok {
		return
	}
	nodes, err := models.GetNodeList()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点失败"})
		return
	}
	items := make([]probeNode, 0, len(nodes))
	for _, n := range nodes {
		host, port := node.ExtractServerHost(n.Link)
		if host == "" || port <= 0 {
			continue
		}
		items = append(items, probeNode{ID: n.ID, Name: n.Name, Host: host, Port: port})
	}
	models.DB.Model(probe).Update("last_seen_at", time.Now())
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"probe": gin.H{"id": probe.ID, "name": probe.Name},
		"nodes": items, "intervalSeconds": 300,
	}, "msg": "探针任务"})
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
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// NodeProbePolicy 管理端：设置节点的检测策略。
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
