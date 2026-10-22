package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"ppeelink/node"
)

// AgentCheckJobs 由自建服务器 agent 拉取节点列表；agent 从自己所在网络做 TCP 检测。
func AgentCheckJobs(c *gin.Context) {
	server, ok := agentServer(c)
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
	models.DB.Model(server).Update("last_seen_at", time.Now())
	c.JSON(http.StatusOK, gin.H{"code": "00000", "data": gin.H{
		"server": gin.H{"id": server.ID, "name": server.Name},
		"nodes":  items, "intervalSeconds": 300,
	}, "msg": "节点检测任务"})
}

// AgentCheckReport 保存 agent 的 TCP 检测结果。一个 agent 是一个独立网络视角。
func AgentCheckReport(c *gin.Context) {
	server, ok := agentServer(c)
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
			AgentServerID: server.ID, NodeID: item.NodeID, Kind: kind,
			Success: item.Success, RTT: item.RTT, Error: item.Error,
			IPVersion: item.IPVersion, CheckedAt: now,
		})
	}
	if err := models.RecordAgentCheckResults(server.ID, results, now); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录失败"})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}
