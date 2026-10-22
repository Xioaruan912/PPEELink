package api

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"ppeelink/models"
)

// MonitorAgentTerminal 把浏览器终端直接桥接到自建服务器 agent 的本地 PTY。
// 面板不保存 SSH 主机、用户或密码；只保存 agent 令牌。
func MonitorAgentTerminal(c *gin.Context) {
	monitorID, ok := takeSSHTicket(c.Query("ticket"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "40100", "msg": "票据无效或已过期"})
		return
	}
	var monitor models.Monitor
	if err := models.DB.First(&monitor, monitorID).Error; err != nil || monitor.ProxyServerID == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "40400", "msg": "服务器未关联 agent"})
		return
	}
	ac := agentConnForServer(*monitor.ProxyServerID)
	if ac == nil {
		c.JSON(http.StatusConflict, gin.H{"code": "40900", "msg": "agent 未连接，请等待心跳恢复后重试"})
		return
	}
	conn, err := sshUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)

	var writeMu sync.Mutex
	write := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	sessionID := randomWireID()
	closed := func() {
		writeMu.Lock()
		_ = conn.Close()
		writeMu.Unlock()
	}
	if !ac.openTerminal(sessionID, 100, 30, func(data []byte) {
		if write(data) != nil {
			ac.closeTerminal(sessionID)
		}
	}, closed) {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("agent 终端通道繁忙，请稍后重试\r\n"))
		return
	}
	defer ac.closeTerminal(sessionID)

	for {
		var msg sshClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return
		}
		switch msg.Type {
		case "input":
			ac.sendMessage(agentWireMessage{Type: "term.input", ID: sessionID, Data: encodeBase64([]byte(msg.Data))})
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				ac.sendMessage(agentWireMessage{Type: "term.resize", ID: sessionID, Cols: msg.Cols, Rows: msg.Rows})
			}
		}
	}
}
