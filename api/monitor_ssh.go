package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type monitorSSHTicket struct {
	MonitorID uint
	ExpiresAt time.Time
}

var (
	monitorSSHTickets sync.Map
	sshUpgrader       = websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			u, err := url.Parse(origin)
			return err == nil && u.Host == r.Host
		},
	}
)

type sshClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

func monitorSSHConfigured(m models.Monitor) bool {
	return m.SSHHost != "" && m.SSHUser != "" && m.SSHPasswordEnc != ""
}

// MonitorSSHSave 管理端：保存服务器的 SSH 连接信息。密码只进不出。
func MonitorSSHSave(c *gin.Context) {
	var req struct {
		ID       uint   `json:"id"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数无效"})
		return
	}
	req.Host = strings.TrimSpace(req.Host)
	req.User = strings.TrimSpace(req.User)
	if req.Host == "" || req.User == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "SSH 地址和用户名不能为空"})
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		req.Port = 22
	}
	var monitor models.Monitor
	if err := models.DB.First(&monitor, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	updates := map[string]any{
		"ssh_host": req.Host,
		"ssh_port": req.Port,
		"ssh_user": req.User,
	}
	if req.Password != "" {
		encrypted, err := models.EncryptSecret(req.Password)
		if err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "保存 SSH 凭据失败"})
			return
		}
		updates["ssh_password_enc"] = encrypted
	} else if monitor.SSHPasswordEnc == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "首次配置需要填写 SSH 密码"})
		return
	}
	if err := models.DB.Model(&monitor).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "SSH 配置已保存"})
}

// MonitorSSHTicket 管理端：签发一次性 WebSSH 票据，避免 JWT 出现在 WebSocket URL。
func MonitorSSHTicket(c *gin.Context) {
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
	if !monitorSSHConfigured(monitor) {
		c.JSON(400, gin.H{"code": "40000", "msg": "还没有配置 SSH"})
		return
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "生成票据失败"})
		return
	}
	ticket := hex.EncodeToString(raw)
	monitorSSHTickets.Store(ticket, monitorSSHTicket{MonitorID: monitor.ID, ExpiresAt: time.Now().Add(60 * time.Second)})
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"ticket": ticket, "expiresIn": 60,
	}, "msg": "WebSSH 票据"})
}

func takeSSHTicket(raw string) (uint, bool) {
	value, ok := monitorSSHTickets.LoadAndDelete(raw)
	if !ok {
		return 0, false
	}
	ticket, ok := value.(monitorSSHTicket)
	if !ok || time.Now().After(ticket.ExpiresAt) {
		return 0, false
	}
	return ticket.MonitorID, true
}

// MonitorSSHWS 升级为 WebSocket，把浏览器终端接到服务器 SSH。
func MonitorSSHWS(c *gin.Context) {
	monitorID, ok := takeSSHTicket(c.Query("ticket"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40100, "msg": "票据无效或已过期"})
		return
	}
	var monitor models.Monitor
	if err := models.DB.First(&monitor, monitorID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 40400, "msg": "服务器不存在"})
		return
	}
	password, err := models.DecryptSecret(monitor.SSHPasswordEnc)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50000, "msg": "SSH 凭据无法解密，请重新配置"})
		return
	}

	conn, err := sshUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(1 << 20)

	host := monitor.SSHHost
	port := monitor.SSHPort
	if port <= 0 {
		port = 22
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	rawConn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("SSH 连接失败: "+err.Error()+"\r\n"))
		return
	}
	cfg := &ssh.ClientConfig{
		User:            monitor.SSHUser,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 自建节点场景先保证可用；后续可加指纹校验
		Timeout:         10 * time.Second,
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(rawConn, addr, cfg)
	if err != nil {
		_ = rawConn.Close()
		_ = conn.WriteMessage(websocket.TextMessage, []byte("SSH 认证失败: "+err.Error()+"\r\n"))
		return
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("创建 SSH 会话失败: "+err.Error()+"\r\n"))
		return
	}
	defer session.Close()
	if err := session.RequestPty("xterm-256color", 32, 100, ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("申请 PTY 失败: "+err.Error()+"\r\n"))
		return
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return
	}
	if err := session.Shell(); err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("启动 shell 失败: "+err.Error()+"\r\n"))
		return
	}

	var writeMu sync.Mutex
	write := func(data []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(websocket.TextMessage, data)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, 32*1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				if write(append([]byte(nil), buf[:n]...)) != nil {
					return
				}
			}
			if err != nil {
				close(done)
				return
			}
		}
	}()
	go func() {
		buf := make([]byte, 8*1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				if write(append([]byte(nil), buf[:n]...)) != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	go func() {
		<-done
		_ = conn.Close()
	}()

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg sshClientMessage
		if json.Unmarshal(payload, &msg) != nil {
			continue
		}
		switch msg.Type {
		case "input":
			if _, err := stdin.Write([]byte(msg.Data)); err != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 {
				_ = session.WindowChange(msg.Rows, msg.Cols)
			}
		}
	}
	_ = session.Close()
}
