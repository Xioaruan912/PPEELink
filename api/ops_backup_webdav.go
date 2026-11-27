package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"ppeelink/models"
)

var fullRestoreMu sync.Mutex

// buildFullBackupArchiveBytes 生成完整备份 tar.gz 字节流（SQLite VACUUM INTO 一致性快照 + template/）。
func buildFullBackupArchiveBytes() ([]byte, error) {
	dir, err := os.MkdirTemp("", "ppeelink-full-backup-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	snapshot := filepath.Join(dir, "ppeelink.db")
	if err := models.DB.Exec("VACUUM INTO ?", snapshot).Error; err != nil {
		return nil, fmt.Errorf("数据库快照失败: %w", err)
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	addFile := func(diskPath, archiveName string) error {
		info, err := os.Stat(diskPath)
		if err != nil || info.IsDir() {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = archiveName
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		f, err := os.Open(diskPath)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(tw, f)
		return err
	}

	if err := addFile(snapshot, "db/ppeelink.db"); err != nil {
		return nil, err
	}
	if entries, err := os.ReadDir(baseTemplateDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				_ = addFile(filepath.Join(baseTemplateDir, e.Name()), "template/"+e.Name())
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// restoreFullBackupFromArchive 校验并原地恢复完整 tar.gz 备份（自动保留恢复前安全副本）。
func restoreFullBackupFromArchive(archiveBytes []byte) (int, error) {
	fullRestoreMu.Lock()
	defer fullRestoreMu.Unlock()

	tmpDir, err := os.MkdirTemp("", "ppeelink-restore-")
	if err != nil {
		return 0, err
	}
	defer os.RemoveAll(tmpDir)

	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return 0, fmt.Errorf("无法解压备份文件（需为 .tar.gz 格式）: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var extractedDB string
	extractedTemplates := map[string][]byte{}

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("读取压缩包失败: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		clean := path.Clean(strings.TrimPrefix(hdr.Name, "./"))
		if strings.Contains(clean, "..") {
			continue
		}
		switch {
		case clean == "db/ppeelink.db":
			dst := filepath.Join(tmpDir, "ppeelink.db")
			f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return 0, err
			}
			if _, err := io.Copy(f, io.LimitReader(tr, 256<<20)); err != nil {
				_ = f.Close()
				return 0, err
			}
			_ = f.Close()
			extractedDB = dst
		case strings.HasPrefix(clean, "template/"):
			name := filepath.Base(strings.TrimPrefix(clean, "template/"))
			if name == "" || name == "." || name == ".." {
				continue
			}
			data, err := io.ReadAll(io.LimitReader(tr, 16<<20))
			if err == nil {
				extractedTemplates[name] = data
			}
		}
	}

	if extractedDB == "" {
		return 0, errors.New("备份包中未找到 db/ppeelink.db")
	}

	// 校验 SQLite 完整性
	checkDB, err := gorm.Open(sqlite.Open(extractedDB), &gorm.Config{})
	if err != nil {
		return 0, fmt.Errorf("SQLite 校验失败: %w", err)
	}
	var integrity string
	_ = checkDB.Raw("PRAGMA integrity_check").Scan(&integrity).Error
	if sqlDB, cerr := checkDB.DB(); cerr == nil {
		_ = sqlDB.Close()
	}
	if strings.ToLower(strings.TrimSpace(integrity)) != "ok" {
		return 0, fmt.Errorf("备份数据库完整性校验失败: %s", integrity)
	}

	// 恢复前留存当前数据库安全快照
	if err := os.MkdirAll("./db", 0755); err != nil {
		return 0, err
	}
	safetyBak := filepath.Join("./db", "ppeelink.db.pre-restore-"+time.Now().Format("20060102-150405")+".bak")
	if models.DB != nil {
		_ = models.DB.Exec("VACUUM INTO ?", safetyBak).Error
	}

	// 恢复模板文件
	restoredTemplates := 0
	for name, content := range extractedTemplates {
		if dstPath, err := safeFilePath(name); err == nil {
			if err := atomicWrite(dstPath, string(content)); err == nil {
				restoredTemplates++
			}
		}
	}

	// 替换 SQLite 主文件并重新打开连接池
	models.CloseSqlite()
	_ = os.Remove("./db/ppeelink.db-wal")
	_ = os.Remove("./db/ppeelink.db-shm")
	in, err := os.ReadFile(extractedDB)
	if err != nil {
		_ = models.ReopenSqlite()
		return 0, err
	}
	tmpTarget := "./db/ppeelink.db.restoring"
	if err := os.WriteFile(tmpTarget, in, 0600); err != nil {
		_ = models.ReopenSqlite()
		return 0, err
	}
	if err := os.Rename(tmpTarget, "./db/ppeelink.db"); err != nil {
		_ = models.ReopenSqlite()
		return 0, err
	}
	if err := models.ReopenSqlite(); err != nil {
		return 0, fmt.Errorf("恢复后重连数据库失败: %w", err)
	}
	return restoredTemplates, nil
}

// ConfigBackupFullRestore 从本机上传的 .tar.gz 完整快照恢复整机数据库与模板。
// POST /api/v1/ops/backup/full/restore
func ConfigBackupFullRestore(c *gin.Context) {
	var raw []byte
	var err error
	if fh, ferr := c.FormFile("file"); ferr == nil && fh != nil {
		f, oerr := fh.Open()
		if oerr != nil {
			c.JSON(400, gin.H{"code": "40000", "msg": "读取上传文件失败"})
			return
		}
		defer f.Close()
		raw, err = io.ReadAll(io.LimitReader(f, 256<<20))
	} else {
		raw, err = io.ReadAll(io.LimitReader(c.Request.Body, 256<<20))
	}
	if err != nil || len(raw) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "请上传有效的 .tar.gz 完整备份文件"})
		return
	}

	tplCount, err := restoreFullBackupFromArchive(raw)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  fmt.Sprintf("整机备份已恢复（同时恢复 %d 个模板，恢复前自动备份已留存）", tplCount),
		"data": gin.H{"templates": tplCount},
	})
}

// ---- Pure-Go WebDAV Client (坚果云 / Alist / Nextcloud / Synology) ----

type webdavClient struct {
	baseURL  string
	username string
	password string
	dir      string
	http     *http.Client
}

type WebDAVRemoteFile struct {
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	LastModified string `json:"lastModified"`
}

func newWebDAVClient(cfg *models.WebDAVBackupSetting) (*webdavClient, error) {
	rawURL := strings.TrimSpace(cfg.URL)
	if rawURL == "" {
		return nil, errors.New("请填写 WebDAV 服务器地址")
	}
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("WebDAV 地址格式无效（需以 https:// 或 http:// 开头）")
	}
	dir := strings.Trim(strings.TrimSpace(cfg.RemoteDir), "/")
	if dir == "" {
		dir = "ppeelink-backups"
	}
	return &webdavClient{
		baseURL:  strings.TrimRight(rawURL, "/"),
		username: strings.TrimSpace(cfg.Username),
		password: cfg.Password,
		dir:      dir,
		http:     &http.Client{Timeout: 45 * time.Second},
	}, nil
}

func (w *webdavClient) do(method, fullURL string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	if w.username != "" || w.password != "" {
		req.SetBasicAuth(w.username, w.password)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return w.http.Do(req)
}

func (w *webdavClient) dirURL() string {
	parts := strings.Split(w.dir, "/")
	escaped := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			escaped = append(escaped, url.PathEscape(p))
		}
	}
	if len(escaped) == 0 {
		return w.baseURL + "/"
	}
	return w.baseURL + "/" + strings.Join(escaped, "/") + "/"
}

func (w *webdavClient) ensureDir() error {
	parts := strings.Split(w.dir, "/")
	cur := w.baseURL
	for _, p := range parts {
		if p == "" {
			continue
		}
		cur += "/" + url.PathEscape(p)
		resp, err := w.do("MKCOL", cur+"/", nil, nil)
		if err != nil {
			return err
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		// 201 Created, 405 Method Not Allowed (already exists), 301/302/200 OK
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return fmt.Errorf("WebDAV 鉴权失败 (HTTP %d)，坚果云请使用「应用密码」", resp.StatusCode)
		}
		if resp.StatusCode >= 400 && resp.StatusCode != 405 && resp.StatusCode != 409 {
			return fmt.Errorf("创建远端目录失败 (HTTP %d)", resp.StatusCode)
		}
	}
	return nil
}

func (w *webdavClient) testConnection() error {
	resp, err := w.do("PROPFIND", w.baseURL+"/", nil, map[string]string{"Depth": "0"})
	if err != nil {
		return fmt.Errorf("连接 WebDAV 失败: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return fmt.Errorf("WebDAV 认证失败 (HTTP %d)，请检查账号与应用密码", resp.StatusCode)
	}
	if resp.StatusCode != 200 && resp.StatusCode != 207 {
		return fmt.Errorf("WebDAV 返回异常状态 (HTTP %d)", resp.StatusCode)
	}
	return w.ensureDir()
}

func (w *webdavClient) upload(filename string, data []byte) error {
	if err := w.ensureDir(); err != nil {
		return err
	}
	target := w.dirURL() + url.PathEscape(filename)
	resp, err := w.do(http.MethodPut, target, bytes.NewReader(data), map[string]string{
		"Content-Type": "application/gzip",
	})
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("上传 WebDAV 失败 (HTTP %d)", resp.StatusCode)
	}
	return nil
}

func (w *webdavClient) download(filename string) ([]byte, error) {
	target := w.dirURL() + url.PathEscape(filename)
	resp, err := w.do(http.MethodGet, target, nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("下载远端备份失败 (HTTP %d)", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 256<<20))
}

func (w *webdavClient) deleteFile(filename string) error {
	target := w.dirURL() + url.PathEscape(filename)
	resp, err := w.do(http.MethodDelete, target, nil, nil)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return nil
}

type davMultistatus struct {
	Responses []struct {
		Href     string `xml:"href"`
		Propstat []struct {
			Prop struct {
				ContentLength int64  `xml:"getcontentlength"`
				LastModified  string `xml:"getlastmodified"`
			} `xml:"prop"`
		} `xml:"propstat"`
	} `xml:"response"`
}

func (w *webdavClient) listBackups() ([]WebDAVRemoteFile, error) {
	resp, err := w.do("PROPFIND", w.dirURL(), nil, map[string]string{"Depth": "1"})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return []WebDAVRemoteFile{}, nil
	}
	if resp.StatusCode != 200 && resp.StatusCode != 207 {
		return nil, fmt.Errorf("读取远端目录失败 (HTTP %d)", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var ms davMultistatus
	if err := xml.Unmarshal(raw, &ms); err != nil {
		return nil, fmt.Errorf("解析 WebDAV 目录失败: %w", err)
	}
	files := make([]WebDAVRemoteFile, 0, len(ms.Responses))
	for _, r := range ms.Responses {
		decoded, err := url.PathUnescape(r.Href)
		if err != nil {
			decoded = r.Href
		}
		name := path.Base(strings.TrimRight(decoded, "/"))
		if !strings.HasSuffix(strings.ToLower(name), ".tar.gz") {
			continue
		}
		item := WebDAVRemoteFile{Name: name}
		for _, ps := range r.Propstat {
			if ps.Prop.ContentLength > 0 {
				item.Size = ps.Prop.ContentLength
			}
			if ps.Prop.LastModified != "" {
				item.LastModified = ps.Prop.LastModified
			}
		}
		files = append(files, item)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name > files[j].Name
	})
	return files, nil
}

func runWebDAVBackupWithConfig(cfg *models.WebDAVBackupSetting) (string, int, error) {
	client, err := newWebDAVClient(cfg)
	if err != nil {
		return "", 0, err
	}
	archive, err := buildFullBackupArchiveBytes()
	if err != nil {
		return "", 0, err
	}
	filename := "ppeelink-backup-" + time.Now().Format("20060102-150405") + ".tar.gz"
	if err := client.upload(filename, archive); err != nil {
		now := time.Now()
		_ = models.DB.Model(cfg).Updates(map[string]any{
			"last_status":    "error",
			"last_error":     err.Error(),
			"last_backup_at": &now,
		}).Error
		return "", 0, err
	}

	// 自动按保留份数清理旧云端备份
	if cfg.KeepCount > 0 {
		if list, lerr := client.listBackups(); lerr == nil && len(list) > cfg.KeepCount {
			for _, old := range list[cfg.KeepCount:] {
				_ = client.deleteFile(old.Name)
			}
		}
	}

	now := time.Now()
	_ = models.DB.Model(cfg).Updates(map[string]any{
		"last_status":    "ok",
		"last_error":     "",
		"last_file":      filename,
		"last_backup_at": &now,
	}).Error
	return filename, len(archive), nil
}

// RunScheduledWebDAVBackup 供 cron.go 每日定时调用，按配置周期自动备份到 WebDAV。
func RunScheduledWebDAVBackup() {
	if models.DB == nil {
		return
	}
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil || !cfg.Enabled || cfg.Schedule == "off" || strings.TrimSpace(cfg.URL) == "" {
		return
	}
	if cfg.LastBackupAt != nil && cfg.LastStatus == "ok" {
		elapsed := time.Since(*cfg.LastBackupAt)
		switch cfg.Schedule {
		case "daily":
			if elapsed < 22*time.Hour {
				return
			}
		case "every3d":
			if elapsed < 70*time.Hour {
				return
			}
		case "weekly":
			if elapsed < 166*time.Hour {
				return
			}
		default:
			return
		}
	}
	name, size, err := runWebDAVBackupWithConfig(cfg)
	if err != nil {
		log.Printf("[WebDAV] 自动云备份失败: %v", err)
		return
	}
	log.Printf("[WebDAV] 自动云备份完成: %s (%d bytes)", name, size)
}

// ---- HTTP Handlers ----

func WebDAVBackupGetConfig(c *gin.Context) {
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取 WebDAV 配置失败"})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"enabled":      cfg.Enabled,
			"preset":       cfg.Preset,
			"url":          cfg.URL,
			"username":     cfg.Username,
			"hasPassword":  cfg.Password != "",
			"remoteDir":    cfg.RemoteDir,
			"schedule":     cfg.Schedule,
			"keepCount":    cfg.KeepCount,
			"lastStatus":   cfg.LastStatus,
			"lastError":    cfg.LastError,
			"lastFile":     cfg.LastFile,
			"lastBackupAt": cfg.LastBackupAt,
		},
	})
}

func WebDAVBackupSaveConfig(c *gin.Context) {
	var req struct {
		Enabled   bool   `json:"enabled"`
		Preset    string `json:"preset"`
		URL       string `json:"url"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		RemoteDir string `json:"remoteDir"`
		Schedule  string `json:"schedule"`
		KeepCount int    `json:"keepCount"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数无效"})
		return
	}
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取配置失败"})
		return
	}
	if req.KeepCount <= 0 {
		req.KeepCount = 7
	}
	if req.KeepCount > 60 {
		req.KeepCount = 60
	}
	switch req.Schedule {
	case "off", "daily", "every3d", "weekly":
	default:
		req.Schedule = "daily"
	}
	if strings.TrimSpace(req.RemoteDir) == "" {
		req.RemoteDir = "ppeelink-backups"
	}
	updates := map[string]any{
		"enabled":    req.Enabled,
		"preset":     strings.TrimSpace(req.Preset),
		"url":        strings.TrimSpace(req.URL),
		"username":   strings.TrimSpace(req.Username),
		"remote_dir": strings.Trim(strings.TrimSpace(req.RemoteDir), "/"),
		"schedule":   req.Schedule,
		"keep_count": req.KeepCount,
	}
	if req.Password != "" {
		updates["password"] = req.Password
	}
	if err := models.DB.Model(cfg).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "WebDAV 备份配置已保存"})
}

func WebDAVBackupTest(c *gin.Context) {
	var req struct {
		URL       string `json:"url"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		RemoteDir string `json:"remoteDir"`
	}
	_ = c.ShouldBindJSON(&req)
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取配置失败"})
		return
	}
	testCfg := *cfg
	if strings.TrimSpace(req.URL) != "" {
		testCfg.URL = strings.TrimSpace(req.URL)
	}
	if strings.TrimSpace(req.Username) != "" {
		testCfg.Username = strings.TrimSpace(req.Username)
	}
	if req.Password != "" {
		testCfg.Password = req.Password
	}
	if strings.TrimSpace(req.RemoteDir) != "" {
		testCfg.RemoteDir = strings.TrimSpace(req.RemoteDir)
	}
	client, err := newWebDAVClient(&testCfg)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	if err := client.testConnection(); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "WebDAV 连接正常，目录可写入"})
}

func WebDAVBackupRunNow(c *gin.Context) {
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取配置失败"})
		return
	}
	name, size, err := runWebDAVBackupWithConfig(cfg)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "云备份失败: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  fmt.Sprintf("已上传云端快照 %s", name),
		"data": gin.H{"file": name, "bytes": size},
	})
}

func WebDAVBackupList(c *gin.Context) {
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取配置失败"})
		return
	}
	if strings.TrimSpace(cfg.URL) == "" {
		c.JSON(200, gin.H{"code": "00000", "data": []WebDAVRemoteFile{}})
		return
	}
	client, err := newWebDAVClient(cfg)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	files, err := client.listBackups()
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": files})
}

func WebDAVBackupRestore(c *gin.Context) {
	var req struct {
		File string `json:"file"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.File) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "请选择要恢复的云端备份文件"})
		return
	}
	filename := path.Base(strings.TrimSpace(req.File))
	if !strings.HasSuffix(strings.ToLower(filename), ".tar.gz") {
		c.JSON(400, gin.H{"code": "40000", "msg": "文件名无效"})
		return
	}
	cfg, err := models.GetOrCreateWebDAVBackupSetting()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取配置失败"})
		return
	}
	client, err := newWebDAVClient(cfg)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	archive, err := client.download(filename)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	tplCount, err := restoreFullBackupFromArchive(archive)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"code": "00000",
		"msg":  fmt.Sprintf("已从云端恢复 %s（恢复 %d 个模板）", filename, tplCount),
		"data": gin.H{"file": filename, "templates": tplCount},
	})
}
