package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
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

func opsSFTP(c *gin.Context) (*sftp.Client, func(), bool) {
	sshClient, closeSSH, err := opsSSH(c)
	if err != nil {
		opsError(c, 400, err.Error())
		return nil, nil, false
	}
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		closeSSH()
		opsError(c, 502, "SFTP 子系统不可用，请检查服务器 sshd 的 SFTP 配置")
		return nil, nil, false
	}
	return client, func() { _ = client.Close(); closeSSH() }, true
}

func fileError(c *gin.Context, err error) {
	msg := "SFTP 操作失败：请检查路径、权限或连接后重试"
	status := 502
	if os.IsNotExist(err) {
		msg = "文件或目录不存在，请刷新目录"
		status = 404
	} else if os.IsPermission(err) {
		msg = "SFTP 权限不足，请检查 SSH 用户的目录权限"
		status = 403
	}
	opsError(c, status, msg)
}

func filePath(c *gin.Context, mutation bool) (string, bool) {
	p, err := validateRemotePath(c.Query("path"), mutation)
	if err != nil {
		opsError(c, 400, err.Error())
		return "", false
	}
	return p, true
}

func MonitorFilesList(c *gin.Context) {
	p, ok := filePath(c, false)
	if !ok {
		return
	}
	s, close, ok := opsSFTP(c)
	if !ok {
		return
	}
	defer close()
	entries, err := s.ReadDirContext(c.Request.Context(), p)
	if err != nil {
		fileError(c, err)
		return
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	truncated := len(entries) > 2000
	if truncated {
		entries = entries[:2000]
	}
	items := make([]opsFile, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}
		kind := "file"
		target := ""
		if entry.IsDir() {
			kind = "directory"
		} else if entry.Mode()&os.ModeSymlink != 0 {
			kind = "symlink"
			target, _ = s.ReadLink(path.Join(p, name))
		} else if !entry.Mode().IsRegular() {
			kind = "special"
		}
		items = append(items, opsFile{Name: name, Path: path.Join(p, name), Size: entry.Size(), Mode: entry.Mode().String(), Modified: entry.ModTime().Unix(), Kind: kind, Target: target})
	}
	opsOK(c, gin.H{"path": p, "parent": path.Dir(p), "items": items, "truncated": truncated})
}

func regularFile(s *sftp.Client, p string, limit int64) (os.FileInfo, error) {
	info, err := s.Lstat(p)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("仅支持普通文件，不跟随符号链接")
	}
	if info.Size() > limit {
		return nil, errors.New("文件超过大小限制")
	}
	return info, nil
}

func MonitorFilesRead(c *gin.Context) {
	p, ok := filePath(c, false)
	if !ok {
		return
	}
	s, close, ok := opsSFTP(c)
	if !ok {
		return
	}
	defer close()
	limit := monitorEditorLimit
	if strings.HasSuffix(c.Request.URL.Path, "/download") {
		limit = monitorFileLimit
	}
	if _, err := regularFile(s, p, limit); err != nil {
		opsError(c, 400, "请选择不超过大小限制的普通文件（编辑 1 MiB，下载 64 MiB；不跟随符号链接）")
		return
	}
	f, err := s.Open(p)
	if err != nil {
		fileError(c, err)
		return
	}
	defer f.Close()
	data, err := readBounded(f, limit)
	if err != nil {
		opsError(c, 400, "读取失败或文件超过大小限制，请刷新后重试")
		return
	}
	if limit == monitorEditorLimit {
		if !utf8.Valid(data) || bytes.IndexByte(data, 0) >= 0 {
			opsError(c, 400, "编辑器仅支持不含 NUL 字节的 UTF-8 文本")
			return
		}
		opsOK(c, gin.H{"path": p, "content": string(data)})
		return
	}
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(p)}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(200, "application/octet-stream", data)
}

// atomicRemoteSave never truncates the destination. POSIX rename is required for replacement.
func atomicRemoteSave(s *sftp.Client, p string, data []byte) error {
	mode := os.FileMode(0644)
	if info, err := s.Lstat(p); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("目标不是普通文件，不允许覆盖")
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		return err
	}
	nonce := make([]byte, 12)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	tmp := path.Join(path.Dir(p), ".ppeelink-"+hex.EncodeToString(nonce)+".tmp")
	f, err := s.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if err != nil {
		return err
	}
	defer s.Remove(tmp)
	_, err = io.Copy(f, bytes.NewReader(data))
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = s.Chmod(tmp, mode); err != nil {
		return err
	}
	if _, ok := s.HasExtension("posix-rename@openssh.com"); ok {
		return s.PosixRename(tmp, p)
	}
	// Standard SFTP rename is atomic for new names but cannot replace existing files.
	if _, err = s.Lstat(p); err == nil {
		return errors.New("SFTP 不支持原子替换，请启用 OpenSSH posix-rename 扩展")
	}
	return s.Rename(tmp, p)
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
	// Bound slow uploads as well as the SSH operation. Unsupported transports keep
	// their server-level read policy; the body size remains bounded independently.
	controller := http.NewResponseController(c.Writer)
	_ = controller.SetReadDeadline(time.Now().Add(monitorOpsTimeout))
	defer controller.SetReadDeadline(time.Time{})
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
	s, close, ok := opsSFTP(c)
	if !ok {
		return
	}
	defer close()
	if err = atomicRemoteSave(s, p, data); err != nil {
		opsError(c, 502, "原子保存失败：请检查目标为普通文件、目录写权限及 SFTP posix-rename 扩展；原文件未主动删除")
		return
	}
	opsOK(c, gin.H{"path": p, "size": len(data)})
}

func MonitorFilesMutate(c *gin.Context) {
	p, ok := filePath(c, true)
	if !ok {
		return
	}
	var req struct {
		Destination string `json:"destination"`
	}
	destination := ""
	if strings.HasSuffix(c.Request.URL.Path, "/rename") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
		if c.ShouldBindJSON(&req) != nil {
			opsError(c, 400, "请提供目标绝对路径 destination")
			return
		}
		var err error
		destination, err = validateRemotePath(req.Destination, true)
		if err != nil {
			opsError(c, 400, err.Error())
			return
		}
	}
	s, close, ok := opsSFTP(c)
	if !ok {
		return
	}
	defer close()
	var err error
	switch path.Base(c.Request.URL.Path) {
	case "mkdir":
		err = s.Mkdir(p)
	case "rename":
		if _, e := s.Lstat(destination); e == nil {
			opsError(c, 409, "目标已存在，不允许覆盖；请换一个名称")
			return
		} else if !os.IsNotExist(e) {
			fileError(c, e)
			return
		}
		err = s.Rename(p, destination)
	case "delete":
		var info os.FileInfo
		info, err = s.Lstat(p)
		if err == nil {
			if info.IsDir() {
				err = s.RemoveDirectory(p)
			} else if info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				err = s.Remove(p)
			} else {
				opsError(c, 400, "不允许删除特殊设备文件")
				return
			}
		}
	}
	if err != nil {
		fileError(c, err)
		return
	}
	opsOK(c, gin.H{"path": p})
}
