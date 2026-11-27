package xraycore

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WarpProfile 保存本机自动注册的免费 Cloudflare WARP WireGuard 凭据（持久化至 <dir>/warp.json）。
type WarpProfile struct {
	PrivateKey string `json:"privateKey"`
	PublicKey  string `json:"publicKey"`
	PeerPubKey string `json:"peerPubKey"`
	Endpoint   string `json:"endpoint"`
	AddressV4  string `json:"addressV4"`
	AddressV6  string `json:"addressV6"`
	Reserved   []int  `json:"reserved"`
}

var warpPresetDomains = map[string][]string{
	"openai": {
		"domain:openai.com",
		"domain:chatgpt.com",
		"domain:oaistatic.com",
		"domain:oaiusercontent.com",
		"domain:auth0.com",
		"domain:ai.com",
		"domain:sora.com",
	},
	"claude": {
		"domain:anthropic.com",
		"domain:claude.ai",
		"domain:claudeusercontent.com",
	},
	"gemini": {
		"domain:gemini.google.com",
		"domain:generativelanguage.googleapis.com",
		"domain:aistudio.google.com",
		"domain:alkalimakersuite-pa.clients6.google.com",
		"domain:proactivebackend-pa.googleapis.com",
		"domain:labs.google",
		"domain:notebooklm.google.com",
	},
	"netflix": {
		"domain:netflix.com",
		"domain:netflix.net",
		"domain:nflxvideo.net",
		"domain:nflximg.net",
		"domain:nflxext.com",
		"domain:nflxso.net",
	},
	"disney": {
		"domain:disneyplus.com",
		"domain:disney-plus.net",
		"domain:bamgrid.com",
		"domain:dssott.com",
	},
	"tiktok": {
		"domain:tiktok.com",
		"domain:tiktokv.com",
		"domain:tiktokcdn.com",
		"domain:byteoversea.com",
	},
	"google": {
		"domain:google.com",
		"domain:googleapis.com",
		"domain:gstatic.com",
	},
}

// ResolveWarpDomains 将预设键（openai/claude/gemini/...）与自定义域名展开为 Xray domain 规则列表。
func ResolveWarpDomains(keys []string) []string {
	if len(keys) == 0 {
		keys = []string{"openai", "claude", "gemini"}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(keys)*4)
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" {
			return
		}
		if _, ok := seen[d]; ok {
			return
		}
		seen[d] = struct{}{}
		out = append(out, d)
	}
	for _, k := range keys {
		norm := strings.ToLower(strings.TrimSpace(k))
		if norm == "" {
			continue
		}
		if preset, ok := warpPresetDomains[norm]; ok {
			for _, d := range preset {
				add(d)
			}
			continue
		}
		if strings.Contains(norm, ":") {
			add(norm)
		} else {
			add("domain:" + norm)
		}
	}
	return out
}

// EnsureWarpProfile 加载或自动向 Cloudflare 注册免费 WARP WireGuard 凭据。
func EnsureWarpProfile(ctx context.Context, dir string) (*WarpProfile, error) {
	path := filepath.Join(dir, "warp.json")
	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		var p WarpProfile
		if json.Unmarshal(raw, &p) == nil && p.PrivateKey != "" && p.PeerPubKey != "" && p.AddressV4 != "" {
			if p.Endpoint == "" {
				p.Endpoint = "engage.cloudflareclient.com:2408"
			}
			return &p, nil
		}
	}

	p, err := registerWarpFree(ctx)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if raw, merr := json.MarshalIndent(p, "", "  "); merr == nil {
			_ = os.WriteFile(path, raw, 0o600)
		}
	}
	return p, nil
}

func registerWarpFree(ctx context.Context) (*WarpProfile, error) {
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成 WARP X25519 密钥失败: %w", err)
	}
	privB64 := base64.StdEncoding.EncodeToString(privKey.Bytes())
	pubB64 := base64.StdEncoding.EncodeToString(privKey.PublicKey().Bytes())

	payload := map[string]any{
		"key":        pubB64,
		"install_id": "",
		"fcm_token":  "",
		"tos":        time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"model":      "PC",
		"serial_number": "",
		"locale":     "en_US",
	}
	body, _ := json.Marshal(payload)

	endpoints := []string{
		"https://api.cloudflareclient.com/v0a2158/reg",
		"https://api.cloudflareclient.com/v0a2483/reg",
	}
	client := &http.Client{Timeout: 15 * time.Second}
	var lastErr error
	for _, ep := range endpoints {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("CF-Client-Version", "a-6.11-2223")
		req.Header.Set("User-Agent", "okhttp/3.12.1")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("WARP 注册返回 HTTP %d", resp.StatusCode)
			continue
		}

		var parsed struct {
			Config struct {
				ClientID  string `json:"client_id"`
				Interface struct {
					Addresses struct {
						V4 string `json:"v4"`
						V6 string `json:"v6"`
					} `json:"addresses"`
				} `json:"interface"`
				Peers []struct {
					PublicKey string `json:"public_key"`
					Endpoint  struct {
						Host string `json:"host"`
					} `json:"endpoint"`
				} `json:"peers"`
			} `json:"config"`
		}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			lastErr = fmt.Errorf("解析 WARP 注册响应失败: %w", err)
			continue
		}
		if len(parsed.Config.Peers) == 0 || parsed.Config.Interface.Addresses.V4 == "" {
			lastErr = fmt.Errorf("WARP 注册响应缺少接口地址或节点信息")
			continue
		}
		reserved := []int{0, 0, 0}
		if dec, derr := base64.StdEncoding.DecodeString(parsed.Config.ClientID); derr == nil && len(dec) >= 3 {
			reserved = []int{int(dec[0]), int(dec[1]), int(dec[2])}
		}
		host := strings.TrimSpace(parsed.Config.Peers[0].Endpoint.Host)
		if host == "" || strings.HasSuffix(host, ":0") {
			host = "engage.cloudflareclient.com:2408"
		}
		return &WarpProfile{
			PrivateKey: privB64,
			PublicKey:  pubB64,
			PeerPubKey: parsed.Config.Peers[0].PublicKey,
			Endpoint:   host,
			AddressV4:  parsed.Config.Interface.Addresses.V4,
			AddressV6:  parsed.Config.Interface.Addresses.V6,
			Reserved:   reserved,
		}, nil
	}
	return nil, fmt.Errorf("Cloudflare WARP 注册失败: %w", lastErr)
}

func appendWarpConfig(spec Spec, outbounds, rules []any) ([]any, []any) {
	if spec.Warp == nil || !spec.Warp.Enabled || spec.Warp.Profile == nil {
		return outbounds, rules
	}
	p := spec.Warp.Profile
	if p.PrivateKey == "" || p.PeerPubKey == "" || p.AddressV4 == "" {
		return outbounds, rules
	}
	domains := ResolveWarpDomains(spec.Warp.Domains)
	if len(domains) == 0 {
		return outbounds, rules
	}
	addrs := []string{p.AddressV4 + "/32"}
	if p.AddressV6 != "" {
		addrs = append(addrs, p.AddressV6+"/128")
	}
	endpoint := p.Endpoint
	if endpoint == "" {
		endpoint = "engage.cloudflareclient.com:2408"
	}
	settings := map[string]any{
		"secretKey": p.PrivateKey,
		"address":   addrs,
		"peers": []any{
			map[string]any{
				"publicKey": p.PeerPubKey,
				"endpoint":  endpoint,
			},
		},
		"mtu": 1280,
	}
	if len(p.Reserved) == 3 {
		settings["reserved"] = p.Reserved
	}
	outbounds = append(outbounds, map[string]any{
		"tag":      "warp-out",
		"protocol": "wireguard",
		"settings": settings,
	})
	warpRule := map[string]any{
		"type":        "field",
		"domain":      domains,
		"outboundTag": "warp-out",
	}
	// 插入在 api / block 规则之后、默认出口规则之前，确保按域名优先命中 WARP
	insertAt := len(rules)
	for i, r := range rules {
		if m, ok := r.(map[string]any); ok {
			tag, _ := m["outboundTag"].(string)
			if tag != "api" && tag != "block" {
				insertAt = i
				break
			}
		}
	}
	newRules := make([]any, 0, len(rules)+1)
	newRules = append(newRules, rules[:insertAt]...)
	newRules = append(newRules, warpRule)
	newRules = append(newRules, rules[insertAt:]...)
	return outbounds, newRules
}
