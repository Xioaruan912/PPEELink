package xraycore

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestE2E 需要本地已下载的 xray（XRAY_E2E_DIR），默认跳过。
func TestE2E(t *testing.T) {
	dir := os.Getenv("XRAY_E2E_DIR")
	if dir == "" {
		t.Skip("set XRAY_E2E_DIR to run")
	}
	priv, pub, err := GenerateRealityKeys()
	if err != nil {
		t.Fatal(err)
	}
	if p2, _ := PublicKeyFor(priv); p2 != pub {
		t.Fatal("public key mismatch")
	}
	spec := Spec{Version: DefaultVersion, Enabled: true, APIPort: 18085, BlockPrivate: true, BlockBitTorrent: true,
		Inbounds: []InboundSpec{{Tag: "in-1", Template: TemplateRealityVision, Port: 18443, Target: "www.apple.com:443",
			SNI: "www.apple.com", PrivateKey: priv, ShortID: "a1b2c3d4", Users: []UserSpec{{Email: "u7", UUID: "11111111-2222-4333-8444-555555555555"}}}}}
	r := NewRunner(filepath.Join(dir, "server"), "ppeelink-xray-test")
	r.systemd = false
	if err := r.Apply(context.Background(), spec); err != nil {
		t.Fatal(err)
	}
	defer r.Stop()
	client := map[string]any{
		"inbounds": []any{map[string]any{"port": 18080, "listen": "127.0.0.1", "protocol": "socks", "settings": map[string]any{"udp": false}}},
		"outbounds": []any{map[string]any{"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{
			"address": "127.0.0.1", "port": 18443, "users": []any{map[string]any{"id": "11111111-2222-4333-8444-555555555555", "encryption": "none", "flow": "xtls-rprx-vision"}}}}},
			"streamSettings": map[string]any{"network": "raw", "security": "reality", "realitySettings": map[string]any{
				"serverName": "www.apple.com", "fingerprint": "chrome", "publicKey": pub, "shortId": "a1b2c3d4"}}}},
	}
	raw, _ := json.Marshal(client)
	cfg := filepath.Join(dir, "client", "c.json")
	_ = os.WriteFile(cfg, raw, 0o600)
	cc := exec.Command(filepath.Join(dir, "server", "xray"), "run", "-c", cfg)
	if err := cc.Start(); err != nil {
		t.Fatal(err)
	}
	defer cc.Process.Kill()
	time.Sleep(time.Second)
	out, err := exec.Command("curl", "-s", "-m", "15", "-o", "/dev/null", "-w", "%{http_code} %{size_download}", "-x", "socks5h://127.0.0.1:18080", "https://www.cloudflare.com/cdn-cgi/trace").CombinedOutput()
	t.Logf("curl via reality: %s %v", out, err)
	if !strings.HasPrefix(string(out), "200") {
		t.Fatalf("proxy request failed: %s", out)
	}
	slow := exec.Command("curl", "-s", "-m", "12", "-o", "/dev/null", "--limit-rate", "3k", "-x", "socks5h://127.0.0.1:18080", "https://www.apple.com/")
	_ = slow.Start()
	time.Sleep(3 * time.Second)
	traffic, online, err := r.Collect(context.Background())
	_ = slow.Process.Kill()
	t.Logf("traffic=%v online=%v err=%v", traffic, online, err)
	// xray 不统计回环地址的在线 IP，本地测试里 online 为空属正常，实机上再验证
	t.Logf("online (loopback is not tracked by xray): %v", online)
	if traffic["u7"].Down == 0 {
		t.Fatal("no downlink traffic recorded")
	}
	again, _, _ := r.Collect(context.Background())
	if again["u7"].Down >= traffic["u7"].Down+70000 {
		t.Fatalf("counters must reset between collections: %v then %v", traffic, again)
	}
	// 内网地址应被阻断
	out, _ = exec.Command("curl", "-s", "-m", "5", "-o", "/dev/null", "-w", "%{http_code}", "-x", "socks5h://127.0.0.1:18080", "http://127.0.0.1:8000/").CombinedOutput()
	t.Logf("private target via proxy: %s", out)
	if strings.HasPrefix(string(out), "200") {
		t.Fatal("private address must be blocked")
	}
	if running, msg := r.Status(); !running {
		t.Fatalf("xray not running: %s", msg)
	}
}
