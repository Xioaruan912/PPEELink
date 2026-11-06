package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"os"
	"path/filepath"
	"ppeelink/models"
	"strings"
	"testing"
)

func setupAgentRelease(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PPEELINK_AGENT_RELEASE_DIR", dir)
	payload := []byte("standalone-agent-fixture")
	sum := sha256.Sum256(payload)
	digest := hex.EncodeToString(sum[:])
	for _, name := range []string{"agent-linux-amd64", "agent-linux-arm64", "agent-linux-armv7"} {
		if e := os.WriteFile(filepath.Join(dir, name), payload, 0755); e != nil {
			t.Fatal(e)
		}
	}
	manifest := map[string]any{"version": "1.17", "artifacts": map[string]any{"linux/amd64": map[string]string{"file": "agent-linux-amd64", "sha256": digest}, "linux/arm64": map[string]string{"file": "agent-linux-arm64", "sha256": digest}, "linux/arm": map[string]string{"file": "agent-linux-armv7", "sha256": digest}}}
	body, _ := json.Marshal(manifest)
	if e := os.WriteFile(filepath.Join(dir, "release.json"), body, 0644); e != nil {
		t.Fatal(e)
	}
	return digest
}
func TestAgentReleaseUsesSeparateArtifact(t *testing.T) {
	sum := setupAgentRelease(t)
	for _, arch := range []string{"amd64", "arm64", "arm"} {
		a, e := loadAgentArtifact(arch)
		if e != nil || a.SHA256 != sum || a.Version != "1.17" || !strings.Contains(a.Path, "agent-linux-") {
			t.Fatalf("%s %+v %v", arch, a, e)
		}
	}
	if currentAgentReleaseVersion() != "1.17" {
		t.Fatal("target version not read from release")
	}
}
func TestAgentReleaseRejectsMissingCorruptOrUnsafeFiles(t *testing.T) {
	setupAgentRelease(t)
	if _, e := loadAgentArtifact("../../secret"); e == nil {
		t.Fatal("bad arch accepted")
	}
	dir := os.Getenv("PPEELINK_AGENT_RELEASE_DIR")
	if e := os.WriteFile(filepath.Join(dir, "agent-linux-amd64"), []byte("tampered"), 0755); e != nil {
		t.Fatal(e)
	}
	if _, e := loadAgentArtifact("amd64"); e == nil {
		t.Fatal("bad checksum accepted")
	}
	os.Remove(filepath.Join(dir, "release.json"))
	if _, e := loadAgentArtifact("amd64"); e == nil {
		t.Fatal("fell back to panel binary")
	}
	if currentAgentReleaseVersion() != "" {
		t.Fatal("reported a version without artifacts")
	}
}

func TestAgentBinaryUsesPublishedArtifactForRequestedArchitecture(t *testing.T) {
	setupUnifiedAgentDB(t)
	server := models.ProxyServer{Name: "fixture", Kind: models.ServerKindAgent, Enabled: true, AgentToken: models.HashAgentToken("artifact-token")}
	models.DB.Create(&server)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/agent/binary?arch=arm64", nil)
	c.Request.Header.Set("Authorization", "Agent artifact-token")
	AgentBinary(c)
	if w.Code != 200 || w.Body.String() != "standalone-agent-fixture" || w.Header().Get("X-Agent-Version") != "1.17" {
		t.Fatalf("wrong artifact: %d %s", w.Code, w.Body.String())
	}
}

func TestAgentReleaseRejectsPathTraversalAndSymlink(t *testing.T) {
	for _, kind := range []string{"traversal", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			setupAgentRelease(t)
			dir := os.Getenv("PPEELINK_AGENT_RELEASE_DIR")
			manifestPath := filepath.Join(dir, "release.json")
			raw, _ := os.ReadFile(manifestPath)
			var m agentReleaseManifest
			json.Unmarshal(raw, &m)
			if kind == "traversal" {
				e := m.Artifacts["linux/amd64"]
				e.File = "../agent-linux-amd64"
				m.Artifacts["linux/amd64"] = e
				raw, _ = json.Marshal(m)
				os.WriteFile(manifestPath, raw, 0644)
			} else {
				file := filepath.Join(dir, "agent-linux-amd64")
				os.Rename(file, file+"-real")
				if e := os.Symlink(file+"-real", file); e != nil {
					t.Fatal(e)
				}
			}
			if _, e := loadAgentArtifact("amd64"); e == nil {
				t.Fatal("unsafe artifact accepted", kind)
			}
		})
	}
}
