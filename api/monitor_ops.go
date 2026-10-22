package api

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

const monitorOpsTimeout = 45 * time.Second
const monitorFileLimit int64 = 64 << 20
const monitorEditorLimit int64 = 1 << 20

// MonitorOperationsAdmin is defense in depth: remote files and Docker always require admin.
func MonitorOperationsAdmin(c *gin.Context) {
	allowed := false
	switch c.GetString("authType") {
	case "jwt":
		allowed = c.GetBool("isAdmin")
	case "api-token":
		token := models.APIToken{Scopes: c.GetString("apiScopes")}
		allowed = token.HasScope("admin")
	}
	if !allowed {
		c.AbortWithStatusJSON(403, gin.H{"code": "40300", "msg": "服务器管理需要管理员权限"})
		return
	}
	c.Next()
}

func opsError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"code": strconv.Itoa(status) + "00", "msg": message})
}
func opsOK(c *gin.Context, data any) { c.JSON(200, gin.H{"code": "00000", "data": data}) }

func validateRemotePath(value string, mutation bool) (string, error) {
	if value == "" && !mutation {
		value = "/"
	}
	if len(value) > 4096 || !utf8.ValidString(value) || !strings.HasPrefix(value, "/") {
		return "", errors.New("路径必须是绝对路径，最长 4096 字节")
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return "", errors.New("路径不能包含控制字符")
		}
	}
	for _, part := range strings.Split(value, "/") {
		if part == ".." {
			return "", errors.New("路径不能包含 ..")
		}
	}
	clean := path.Clean(value)
	if mutation && clean == "/" {
		return "", errors.New("不允许修改或删除根目录")
	}
	return clean, nil
}

// opsSSH reuses the Monitor encrypted password. The hard transport deadline also bounds
// the SSH handshake, SFTP subsystem and Docker sessions; request cancellation closes it.
func opsSSH(c *gin.Context) (*ssh.Client, func(), error) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 32)
	if err != nil || id == 0 {
		return nil, nil, errors.New("服务器 id 无效")
	}
	var m models.Monitor
	if models.DB.First(&m, id).Error != nil {
		return nil, nil, errors.New("服务器不存在，请刷新服务器列表")
	}
	if !monitorSSHConfigured(m) {
		return nil, nil, errors.New("尚未配置 SSH，请到服务器监控页面设置 SSH")
	}
	password, err := models.DecryptSecret(m.SSHPasswordEnc)
	if err != nil {
		return nil, nil, errors.New("SSH 凭据无法解密，请重新保存 SSH 配置")
	}
	port := m.SSHPort
	if port <= 0 {
		port = 22
	}
	addr := net.JoinHostPort(m.SSHHost, strconv.Itoa(port))
	ctx, cancel := context.WithTimeout(c.Request.Context(), monitorOpsTimeout)
	raw, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		cancel()
		return nil, nil, errors.New("SSH 连接失败，请检查主机、端口和网络")
	}
	_ = raw.SetDeadline(time.Now().Add(monitorOpsTimeout))
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = raw.Close()
		case <-done:
		}
	}()
	cleanup := func() { close(done); cancel(); _ = raw.Close() }
	cfg := &ssh.ClientConfig{
		User: m.SSHUser, Auth: []ssh.AuthMethod{ssh.Password(password)},
		// Preserve the existing Monitor SSH trust policy; no new credential storage.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 10 * time.Second,
	}
	conn, chans, reqs, err := ssh.NewClientConn(raw, addr, cfg)
	if err != nil {
		cleanup()
		return nil, nil, errors.New("SSH 握手或认证失败，请检查保存的 SSH 凭据")
	}
	client := ssh.NewClient(conn, chans, reqs)
	return client, func() { _ = client.Close(); cleanup() }, nil
}

// cappedWriter consumes excess output without retaining it, avoiding SSH pipe deadlocks.
type cappedWriter struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	n := len(p)
	remaining := w.limit - w.buffer.Len()
	if remaining < len(p) {
		w.truncated = true
	}
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
		}
		_, _ = w.buffer.Write(p)
	}
	return n, nil
}

func opsCommand(client *ssh.Client, command string) (string, bool, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", false, errors.New("无法创建 SSH 会话")
	}
	defer session.Close()
	out := &cappedWriter{limit: 2 << 20}
	stderr := &cappedWriter{limit: 8192}
	session.Stdout = out
	session.Stderr = stderr
	if err = session.Run(command); err != nil {
		return "", false, errors.New("Docker 操作失败：请确认 Docker 已安装、服务运行且 SSH 用户有 Docker 权限（操作限时 45 秒）")
	}
	return out.buffer.String(), out.truncated, nil
}

func readBounded(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("文件超过大小限制")
	}
	return data, nil
}
