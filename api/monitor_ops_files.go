package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
)

type opsFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified int64  `json:"modified"`
	Kind     string `json:"kind"`
	Target   string `json:"target,omitempty"`
}

type agentFileReply struct {
	Path      string    `json:"path"`
	Parent    string    `json:"parent"`
	Items     []opsFile `json:"items"`
	Truncated bool      `json:"truncated"`
	Data      string    `json:"data"`
	Size      int64     `json:"size"`
}

func filePath(c *gin.Context, mutation bool) (string, bool) {
	p, err := validateRemotePath(c.Query("path"), mutation)
	if err != nil {
		opsError(c, 400, err.Error())
		return "", false
	}
	return p, true
}

func requestAgentFiles(c *gin.Context, server *models.ProxyServer, request gin.H, timeout time.Duration) (*agentFileReply, bool) {
	raw, err := agentRequest(server.ID, "files", request, timeout)
	if err != nil {
		opsError(c, 502, err.Error())
		return nil, false
	}
	var reply agentFileReply
	if err := json.Unmarshal(raw, &reply); err != nil {
		opsError(c, 502, "agent 返回的文件结果无法解析")
		return nil, false
	}
	return &reply, true
}

func MonitorFilesList(c *gin.Context) {
	p, ok := filePath(c, false)
	if !ok {
		return
	}
	server, ok := monitorProxyServer(c)
	if !ok {
		return
	}
	reply, ok := requestAgentFiles(c, server, gin.H{"operation": "list", "path": p}, 35*time.Second)
	if !ok {
		return
	}
	opsOK(c, gin.H{"path": reply.Path, "parent": reply.Parent, "items": reply.Items, "truncated": reply.Truncated})
}

func MonitorFilesRead(c *gin.Context) {
	p, ok := filePath(c, false)
	if !ok {
		return
	}
	server, ok := monitorProxyServer(c)
	if !ok {
		return
	}
	reply, ok := requestAgentFiles(c, server, gin.H{"operation": "read", "path": p}, 70*time.Second)
	if !ok {
		return
	}
	data, err := base64.StdEncoding.DecodeString(reply.Data)
	if err != nil || int64(len(data)) > monitorFileLimit {
		opsError(c, 400, "agent 返回的文件超过大小限制")
		return
	}
	download := strings.HasSuffix(c.Request.URL.Path, "/download")
	if !download {
		if int64(len(data)) > monitorEditorLimit || !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			opsError(c, 400, "编辑器仅支持不超过 1 MiB 且不含 NUL 字节的 UTF-8 文本")
			return
		}
		opsOK(c, gin.H{"path": reply.Path, "content": string(data)})
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(p)}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "application/octet-stream", data)
}

func MonitorFilesWrite(c *gin.Context) {
	p, ok := filePath(c, true)
	if !ok {
		return
	}
	limit := monitorEditorLimit
	if strings.HasSuffix(c.Request.URL.Path, "/upload") {
		limit = monitorFileLimit
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	data, err := readBounded(c.Request.Body, limit)
	if err != nil {
		opsError(c, 413, "请求过大或读取失败：编辑最多 1 MiB，上传最多 64 MiB")
		return
	}
	if limit == monitorEditorLimit && (!utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0) {
		opsError(c, 400, "仅支持 UTF-8 文本，不允许 NUL 字节")
		return
	}
	server, ok := monitorProxyServer(c)
	if !ok {
		return
	}
	_, ok = requestAgentFiles(c, server, gin.H{"operation": "write", "path": p, "data": base64.StdEncoding.EncodeToString(data)}, 90*time.Second)
	if !ok {
		return
	}
	opsOK(c, gin.H{"path": p, "size": len(data)})
}

func MonitorFilesMutate(c *gin.Context) {
	p, ok := filePath(c, true)
	if !ok {
		return
	}
	operation := path.Base(c.Request.URL.Path)
	payload := gin.H{"operation": operation, "path": p}
	if operation == "rename" {
		var req struct {
			Destination string `json:"destination"`
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		if c.ShouldBindJSON(&req) != nil {
			opsError(c, 400, "请提供目标绝对路径 destination")
			return
		}
		destination, err := validateRemotePath(req.Destination, true)
		if err != nil {
			opsError(c, 400, err.Error())
			return
		}
		payload["destination"] = destination
	}
	server, ok := monitorProxyServer(c)
	if !ok {
		return
	}
	_, ok = requestAgentFiles(c, server, payload, 35*time.Second)
	if !ok {
		return
	}
	opsOK(c, gin.H{"path": p})
}
