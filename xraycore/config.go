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
		// An empty Hysteria user list must never expose an unauthenticated listener.
		if in.Template == TemplateHysteria2 && len(in.Users) == 0 {
			continue
		}
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
	var routeErr error
	inbounds, outbounds, rules, routeErr = appendEgressConfig(spec, collector, inbounds, outbounds, rules)
	if routeErr != nil {
		return nil, routeErr
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
	if in.Template == TemplateHysteria2 {
		return buildHysteriaInbound(in)
	}
	if in.Port <= 0 || in.Port > 65535 {
		return nil, fmt.Errorf("端口无效")
	}
	listen, normalizeErr := NormalizeListen(in.Listen)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	attachListen := func(item map[string]any) map[string]any {
		// Omit the default listener, as x-ui does. Xray panics on an explicit empty string.
		if listen != "" {
			item["listen"] = listen
		}
		return item
	}
	sniffing := map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}

	// Shadowsocks 2022 (TCP + UDP, per-user derived keys)
	if in.Template == TemplateSS2022 {
		method := NormalizeSS2022Method(in.Method)
		serverPass := strings.TrimSpace(in.PrivateKey)
		if serverPass == "" {
			return nil, fmt.Errorf("缺少 Shadowsocks 2022 服务端密钥")
		}
		clients := make([]any, 0, len(in.Users))
		for _, u := range in.Users {
			clients = append(clients, map[string]any{
				"password": DeriveSS2022UserKey(u.UUID, method),
				"email":    u.Email,
				"level":    0,
			})
		}
		return attachListen(map[string]any{
			"tag":      in.Tag,
			"port":     in.Port,
			"protocol": "shadowsocks",
			"settings": map[string]any{
				"method":   method,
				"password": serverPass,
				"network":  "tcp,udp",
				"clients":  clients,
			},
			"sniffing": sniffing,
		}), nil
	}

	// VMess TCP (no TLS)
	if in.Template == TemplateVMessTCP {
		clients := make([]any, 0, len(in.Users))
		for _, u := range in.Users {
			clients = append(clients, map[string]any{"id": u.UUID, "email": u.Email, "level": 0})
		}
		return attachListen(map[string]any{
			"tag":      in.Tag,
			"port":     in.Port,
			"protocol": "vmess",
			"settings": map[string]any{"clients": clients},
			"sniffing": sniffing,
			"streamSettings": map[string]any{
				"network":     "raw",
				"security":    "none",
				"rawSettings": map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}},
			},
		}), nil
	}

	// Non-Hysteria2 TLS protocols: vless-ws-tls, vmess-ws-tls, trojan-tls
	if TemplateUsesTLS(in.Template) {
		if err := ValidateHysteriaTLS(in.SNI, in.TLSCertFile, in.TLSKeyFile); err != nil {
			return nil, err
		}
		tlsSettings := map[string]any{
			"serverName":   in.SNI,
			"minVersion":   "1.2",
			"certificates": []any{map[string]any{"certificateFile": in.TLSCertFile, "keyFile": in.TLSKeyFile}},
		}
		stream := map[string]any{"security": "tls", "tlsSettings": tlsSettings}
		switch in.Template {
		case TemplateVLESSWSTLS:
			wsPath := in.Path
			if wsPath == "" {
				wsPath = "/ws"
			}
			wsSettings := map[string]any{"path": wsPath}
			if h := strings.TrimSpace(in.Host); h != "" {
				wsSettings["host"] = h
			}
			stream["network"] = "ws"
			stream["wsSettings"] = wsSettings
			clients := make([]any, 0, len(in.Users))
			for _, u := range in.Users {
				clients = append(clients, map[string]any{"id": u.UUID, "email": u.Email, "level": 0})
			}
			return attachListen(map[string]any{
				"tag": in.Tag, "port": in.Port, "protocol": "vless",
				"settings": map[string]any{"clients": clients, "decryption": "none"},
				"sniffing": sniffing, "streamSettings": stream,
			}), nil
		case TemplateVMessWSTLS:
			wsPath := in.Path
			if wsPath == "" {
				wsPath = "/vmess"
			}
			wsSettings := map[string]any{"path": wsPath}
			if h := strings.TrimSpace(in.Host); h != "" {
				wsSettings["host"] = h
			}
			stream["network"] = "ws"
			stream["wsSettings"] = wsSettings
			clients := make([]any, 0, len(in.Users))
			for _, u := range in.Users {
				clients = append(clients, map[string]any{"id": u.UUID, "email": u.Email, "level": 0})
			}
			return attachListen(map[string]any{
				"tag": in.Tag, "port": in.Port, "protocol": "vmess",
				"settings": map[string]any{"clients": clients},
				"sniffing": sniffing, "streamSettings": stream,
			}), nil
		case TemplateTrojanTLS:
			stream["network"] = "raw"
			stream["rawSettings"] = map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}}
			clients := make([]any, 0, len(in.Users))
			for _, u := range in.Users {
				clients = append(clients, map[string]any{"password": u.UUID, "email": u.Email, "level": 0})
			}
			return attachListen(map[string]any{
				"tag": in.Tag, "port": in.Port, "protocol": "trojan",
				"settings": map[string]any{"clients": clients},
				"sniffing": sniffing, "streamSettings": stream,
			}), nil
		}
	}

	// Reality protocols: vless-reality-vision, vless-xhttp-reality, vless-grpc-reality, trojan-reality
	host, port, err := net.SplitHostPort(in.Target)
	if err != nil || host == "" || port == "" {
		return nil, fmt.Errorf("伪装目标需为 host:port")
	}
	if in.PrivateKey == "" || in.SNI == "" {
		return nil, fmt.Errorf("缺少 Reality 私钥或 SNI")
	}
	target, sni, shortIDs, normalizeErr := NormalizeReality(in.Target, in.SNI, in.ShortID)
	if normalizeErr != nil {
		return nil, normalizeErr
	}
	if _, err := PublicKeyFor(in.PrivateKey); err != nil {
		return nil, fmt.Errorf("Reality 私钥无效")
	}
	reality := map[string]any{
		"show":        false,
		"target":      target,
		"xver":        0,
		"serverNames": strings.Split(sni, ","),
		"privateKey":  in.PrivateKey,
		"shortIds":    strings.Split(shortIDs, ","),
	}
	stream := map[string]any{"security": "reality", "realitySettings": reality}

	if in.Template == TemplateTrojanReality {
		stream["network"] = "raw"
		stream["rawSettings"] = map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}}
		clients := make([]any, 0, len(in.Users))
		for _, u := range in.Users {
			clients = append(clients, map[string]any{"password": u.UUID, "email": u.Email, "level": 0})
		}
		return attachListen(map[string]any{
			"tag": in.Tag, "port": in.Port, "protocol": "trojan",
			"settings": map[string]any{"clients": clients},
			"sniffing": sniffing, "streamSettings": stream,
		}), nil
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
	switch in.Template {
	case TemplateRealityVision:
		stream["network"] = "raw"
		stream["rawSettings"] = map[string]any{"acceptProxyProtocol": false, "header": map[string]any{"type": "none"}}
	case TemplateXHTTPReality:
		path := in.Path
		if path == "" {
			path = "/"
		}
		stream["network"] = "xhttp"
		stream["xhttpSettings"] = map[string]any{"path": path, "mode": "auto"}
	case TemplateGRPCReality:
		svc := strings.TrimPrefix(strings.TrimSpace(in.Path), "/")
		if svc == "" {
			svc = "grpc"
		}
		stream["network"] = "grpc"
		stream["grpcSettings"] = map[string]any{"serviceName": svc}
	default:
		return nil, fmt.Errorf("未知模板 %s", in.Template)
	}
	return attachListen(map[string]any{
		"tag":            in.Tag,
		"port":           in.Port,
		"protocol":       "vless",
		"settings":       map[string]any{"clients": clients, "decryption": "none"},
		"sniffing":       sniffing,
		"streamSettings": stream,
	}), nil
}
