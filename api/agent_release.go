package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"ppeelink/agentversion"
)

type agentReleaseEntry struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}
type agentReleaseManifest struct {
	Version   string                       `json:"version"`
	Artifacts map[string]agentReleaseEntry `json:"artifacts"`
}
type agentArtifact struct{ Path, Version, SHA256 string }

func agentReleaseDirectory() string {
	if dir := os.Getenv("PPEELINK_AGENT_RELEASE_DIR"); dir != "" {
		return dir
	}
	return filepath.Join("downloads", "agent")
}
func readAgentRelease() (agentReleaseManifest, error) {
	var m agentReleaseManifest
	f, e := os.Open(filepath.Join(agentReleaseDirectory(), "release.json"))
	if e != nil {
		return m, fmt.Errorf("尚未发布独立 Agent 程序，请部署 agent 分支的构建产物")
	}
	defer f.Close()
	raw, e := io.ReadAll(io.LimitReader(f, 64*1024+1))
	if e != nil {
		return m, e
	}
	if len(raw) > 64*1024 {
		return m, fmt.Errorf("Agent 发行清单超过大小限制")
	}
	if e = json.Unmarshal(raw, &m); e != nil {
		return m, fmt.Errorf("Agent 发行清单无效")
	}
	if _, _, ok := agentversion.Parse(m.Version); !ok {
		return m, fmt.Errorf("Agent 发行版本格式无效")
	}
	return m, nil
}
func currentAgentReleaseVersion() string {
	m, e := readAgentRelease()
	if e != nil {
		return ""
	}
	return m.Version
}
func loadAgentArtifact(arch string) (agentArtifact, error) {
	if arch == "" {
		arch = runtime.GOARCH
	}
	switch arch {
	case "amd64", "arm64", "arm":
	default:
		return agentArtifact{}, fmt.Errorf("未发布此架构的 Agent：%s", arch)
	}
	m, e := readAgentRelease()
	if e != nil {
		return agentArtifact{}, e
	}
	entry, ok := m.Artifacts["linux/"+arch]
	if !ok {
		return agentArtifact{}, fmt.Errorf("尚未发布 Linux %s Agent", arch)
	}
	if entry.File == "" || filepath.Base(entry.File) != entry.File || strings.ContainsAny(entry.File, "\\\x00") {
		return agentArtifact{}, fmt.Errorf("Agent 发行文件名无效")
	}
	expected, e := hex.DecodeString(entry.SHA256)
	if e != nil || len(expected) != sha256.Size {
		return agentArtifact{}, fmt.Errorf("Agent 发行校验值无效")
	}
	path := filepath.Join(agentReleaseDirectory(), entry.File)
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() || info.Size() < 1 || info.Size() > 128<<20 {
		return agentArtifact{}, fmt.Errorf("Agent 发行文件缺失或无效")
	}
	f, e := os.Open(path)
	if e != nil {
		return agentArtifact{}, e
	}
	defer f.Close()
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(f, (128<<20)+1))
	if e != nil || n != info.Size() {
		return agentArtifact{}, fmt.Errorf("Agent 发行文件读取失败")
	}
	digest := hex.EncodeToString(h.Sum(nil))
	if digest != strings.ToLower(entry.SHA256) {
		return agentArtifact{}, fmt.Errorf("Agent 发行文件 SHA256 不匹配")
	}
	return agentArtifact{Path: path, Version: m.Version, SHA256: digest}, nil
}
func agentArtifactHash(arch string) string {
	a, e := loadAgentArtifact(arch)
	if e != nil {
		return ""
	}
	return a.SHA256
}
func serveAgentArtifact(c *gin.Context, filename string) {
	a, e := loadAgentArtifact(c.Query("arch"))
	if e != nil {
		c.JSON(503, gin.H{"code": "50300", "msg": e.Error()})
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+filename)
	c.Header("X-Agent-Version", a.Version)
	c.Header("X-Agent-SHA256", a.SHA256)
	c.File(a.Path)
}
