package api

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"ppeelink/models"
	"strconv"
	"time"
)

func terminalMonitor(c *gin.Context) (models.Monitor, int, error) {
	owner, err := terminalOwner(c)
	if err != nil {
		return models.Monitor{}, 0, err
	}
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		return models.Monitor{}, 0, errors.New("服务器 id 无效")
	}
	var m models.Monitor
	if models.DB.First(&m, id).Error != nil || m.ProxyServerID == nil {
		return m, 0, errors.New("服务器未关联 Agent")
	}
	return m, owner, nil
}
func terminalOperation(server uint, owner int, operation, id, name string) (json.RawMessage, error) {
	ac := agentConnForServer(server)
	if ac == nil {
		return nil, errors.New("Agent 未连接，稍后重试")
	}
	if !ac.supportsPersistence() {
		return nil, errors.New("请先升级 Agent 至 1.15 或更新版本以启用任务保护")
	}
	timeout := 10 * time.Second
	if operation == "prepare" {
		timeout = 245 * time.Second
	}
	return agentRequest(server, "terminal", gin.H{"operation": operation, "owner": owner, "sessionId": id, "name": name}, timeout)
}
func MonitorTerminalSessions(c *gin.Context) {
	m, owner, err := terminalMonitor(c)
	if err != nil {
		opsError(c, 403, err.Error())
		return
	}
	operation := "list"
	name := ""
	id := c.Query("sessionId")
	switch c.Request.Method {
	case "POST":
		var req struct {
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			opsError(c, 400, "名称无效")
			return
		}
		name = req.Name
		operation = "create"
	case "DELETE":
		operation = "terminate"
	}
	raw, err := terminalOperation(*m.ProxyServerID, owner, operation, id, name)
	if operation == "list" {
		ac := agentConnForServer(*m.ProxyServerID)
		supported := ac != nil && ac.supportsPersistence()
		if err != nil {
			opsOK(c, gin.H{"supported": supported, "ready": false, "error": err.Error(), "sessions": []any{}})
			return
		}
		opsOK(c, gin.H{"supported": true, "ready": true, "sessions": json.RawMessage(raw)})
		return
	}
	if err != nil {
		opsError(c, 409, err.Error())
		return
	}
	opsOK(c, json.RawMessage(raw))
}
func MonitorTerminalPrepare(c *gin.Context) {
	m, owner, err := terminalMonitor(c)
	if err != nil {
		opsError(c, 403, err.Error())
		return
	}
	raw, err := terminalOperation(*m.ProxyServerID, owner, "prepare", "", "")
	if err != nil {
		opsError(c, 409, err.Error())
		return
	}
	opsOK(c, json.RawMessage(raw))
}
