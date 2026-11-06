package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"ppeelink/models"
	"sync"
	"time"
)

// Browser connections attach to an owned tmux session; closing one only detaches.
func MonitorAgentTerminal(c *gin.Context) {
	ticket, ok := takeTerminalTicket(c.Query("ticket"))
	if !ok {
		opsError(c, http.StatusUnauthorized, "票据无效或已过期")
		return
	}
	var monitor models.Monitor
	if models.DB.First(&monitor, ticket.MonitorID).Error != nil || monitor.ProxyServerID == nil {
		opsError(c, 404, "服务器未关联 Agent")
		return
	}
	serverID := *monitor.ProxyServerID
	ac := agentConnForServer(serverID)
	if ac == nil || !ac.supportsPersistence() {
		opsError(c, 409, "Agent 尚未就绪，请稍后重试")
		return
	}
	conn, err := sshUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(16 << 10)
	_ = conn.SetReadDeadline(time.Now().Add(75 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(75 * time.Second)) })
	var writeMu sync.Mutex
	write := func(value any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
		return conn.WriteJSON(value)
	}
	attachmentID := randomWireID()
	closed := func() { _ = conn.Close() }
	if !ac.openPersistentTerminal(attachmentID, ticket.SessionID, ticket.Owner, 100, 30, func(data []byte) {
		if write(gin.H{"type": "output", "data": encodeBase64(data)}) != nil {
			ac.closeTerminal(attachmentID)
		}
	}, func(msg agentWireMessage) {
		kind := "ready"
		if msg.Type == "term.exit" {
			kind = "exit"
		} else if msg.Type == "term.detached" {
			kind = "detached"
		}
		_ = write(gin.H{"type": kind, "sessionId": ticket.SessionID, "error": msg.Error, "code": msg.Code, "persistent": true})
	}, closed) {
		_ = write(gin.H{"type": "error", "error": "终端通道繁忙，请稍后重试"})
		return
	}
	defer ac.closeTerminal(attachmentID)
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				var current models.Monitor
				if !terminalOwnerValid(ticket.Owner) || models.DB.First(&current, ticket.MonitorID).Error != nil || current.ProxyServerID == nil || *current.ProxyServerID != serverID {
					_ = write(gin.H{"type": "unauthorized", "error": "终端权限已失效"})
					closed()
					return
				}
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(2*time.Second)); err != nil {
					closed()
					return
				}
			}
		}
	}()
	log.Printf("[terminal] attached server=%d owner=%d session=%s", serverID, ticket.Owner, ticket.SessionID)
	for {
		var msg sshClientMessage
		if err := conn.ReadJSON(&msg); err != nil {
			code := 0
			var closeErr *websocket.CloseError
			if errors.As(err, &closeErr) {
				code = closeErr.Code
			}
			log.Printf("[terminal] detached server=%d session=%s code=%d", serverID, ticket.SessionID, code)
			return
		}
		switch msg.Type {
		case "input":
			if len(msg.Data) > 8192 {
				closed()
				return
			}
			if !ac.sendMessage(agentWireMessage{Type: "term.input", ID: attachmentID, Data: encodeBase64([]byte(msg.Data))}) {
				closed()
				return
			}
		case "resize":
			if msg.Cols >= 10 && msg.Cols <= 500 && msg.Rows >= 3 && msg.Rows <= 300 {
				if !ac.sendMessage(agentWireMessage{Type: "term.resize", ID: attachmentID, Cols: msg.Cols, Rows: msg.Rows}) {
					closed()
					return
				}
			}
		}
	}
}
