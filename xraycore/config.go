package xraycore

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

// BuildConfig 由 Spec 生成 xray 配置 JSON。
// 统计按 email 计量（user>>>email>>>traffic>>>uplink/downlink），API 只监听回环地址。
func BuildConfig(spec Spec) ([]byte, error) {
	return BuildConfigWithWebsiteCollector(spec, nil)
}

// BuildConfigWithWebsiteCollector binds a running local collector to this config.
// Website collection must never silently fall back to unmetered freedom output.
func BuildConfigWithWebsiteCollector(spec Spec, collector *WebsiteCollector) ([]byte, error) {
	websiteEnabled := spec.WebsiteTraffic != nil && spec.WebsiteTraffic.Enabled
	if websiteEnabled && collector == nil {
		return nil, fmt.Errorf("website collector is required")
	}
	if spec.APIPort <= 0 {
		return nil, fmt.Errorf("API 端口无效")
	}
	inbounds := []any{map[string]any{
		"tag":      "api",
		"listen":   "127.0.0.1",
		"port":     spec.APIPort,
		"protocol": "dokodemo-door",
		"settings": map[string]any{"address": "127.0.0.1"},
	}}
	for _, in := range spec.Inbounds {
		item, err := buildInbound(in)
		if err != nil {
			return nil, fmt.Errorf("入站 %s: %w", in.Tag, err)
		}
		inbounds = append(inbounds, item)
	}
	rules := []any{map[string]any{"inboundTag": []string{"api"}, "outboundTag": "api"}}
	if spec.BlockPrivate {
		rules = append(rules, map[string]any{"ip": []string{"geoip:private"}, "outboundTag": "block"})
	}
	if spec.BlockBitTorrent {
		rules = append(rules, map[string]any{"protocol": []string{"bittorrent"}, "outboundTag": "block"})
	}
	outbounds := []any{
		map[string]any{"tag": "direct", "protocol": "freedom"},
		map[string]any{"tag": "block", "protocol": "blackhole"},
	}
	if websiteEnabled {
		collected, collectedRules := collector.configOutbounds()
		// A default blackhole is intentional: an unmatched inbound cannot escape
		// through an unmetered freedom outbound while collection is enabled.
		outbounds = append([]any{map[string]any{"tag": "direct", "protocol": "blackhole"}, map[string]any{"tag": "block", "protocol": "blackhole"}}, collected...)
		rules = append(rules, collectedRules...)
	}
	config := map[string]any{
		"log": map[string]any{"loglevel": "warning"},
		"api": map[string]any{
			"tag":      "api",
			"services": []string{"StatsService", "HandlerService"},
		},
		"stats": map[string]any{},
		"policy": map[string]any{
			"levels": map[string]any{"0": map[string]any{
				"statsUserUplink": true, "statsUserDownlink": true, "statsUserOnline": true,
			}},
		},
		"inbounds":  inbounds,
		"outbounds": outbounds,
		"routing":   map[string]any{"domainStrategy": "AsIs", "rules": rules},
	}
	return json.MarshalIndent(config, "", "  ")
}

func buildInbound(in InboundSpec) (map[string]any, error) {
	if in.Port <= 0 || in.Port > 65535 {
		return nil, fmt.Errorf("端口无效")
	}
	host, port, err := net.SplitHostPort(in.Target)
	if err != nil || host == "" || port == "" {
		return nil, fmt.Errorf("伪装目标需为 host:port")
	}
	if in.PrivateKey == "" || in.SNI == "" {
		return nil, fmt.Errorf("缺少 Reality 私钥或 SNI")
	}
	flow := ""
	if in.Template == TemplateRealityVision {
		flow = "xtls-rprx-vision"
	}
	clients := make([]any, 0, len(in.Users))
	for _, u := range in.Users {
		client := map[string]any{"id": u.UUID, "email": u.Email, "level": 0}
		if flow != "" {
			client["flow"] = flow
		}
		clients = append(clients, client)
	}
	reality := map[string]any{
		"show":        false,
		"target":      in.Target,
		"xver":        0,
		"serverNames": strings.Split(in.SNI, ","),
		"privateKey":  in.PrivateKey,
		"shortIds":    []string{in.ShortID},
	}
	stream := map[string]any{"security": "reality", "realitySettings": reality}
	switch in.Template {
	case TemplateRealityVision:
		stream["network"] = "raw"
	case TemplateXHTTPReality:
		path := in.Path
		if path == "" {
			path = "/"
		}
		stream["network"] = "xhttp"
		stream["xhttpSettings"] = map[string]any{"path": path, "mode": "auto"}
	default:
		return nil, fmt.Errorf("未知模板 %s", in.Template)
	}
	return map[string]any{
		"tag":            in.Tag,
		"listen":         "0.0.0.0",
		"port":           in.Port,
		"protocol":       "vless",
		"settings":       map[string]any{"clients": clients, "decryption": "none"},
		"sniffing":       map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true},
		"streamSettings": stream,
	}, nil
}
