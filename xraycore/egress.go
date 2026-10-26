package xraycore

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// EgressSpec deliberately accepts only VLESS Reality raw/XHTTP, without chaining or insecure TLS overrides.
type EgressSpec struct {
	Address     string `json:"address"`
	Port        int    `json:"port"`
	UUID        string `json:"uuid"`
	SNI         string `json:"sni"`
	PublicKey   string `json:"publicKey"`
	ShortID     string `json:"shortId"`
	Transport   string `json:"transport"`
	Flow        string `json:"flow"`
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
}

func (e EgressSpec) Validate() error {
	if strings.TrimSpace(e.Address) == "" || strings.ContainsAny(e.Address, " /\\\r\n") || e.Port < 1 || e.Port > 65535 {
		return fmt.Errorf("出口地址或端口无效")
	}
	if ip := net.ParseIP(e.Address); ip != nil && websitePrivateIP(ip) {
		return fmt.Errorf("出口需为公网地址")
	}
	id, err := hex.DecodeString(strings.ReplaceAll(e.UUID, "-", ""))
	if err != nil || len(id) != 16 {
		return fmt.Errorf("出口 UUID 无效")
	}
	pk, err := base64.RawURLEncoding.DecodeString(e.PublicKey)
	if err != nil || len(pk) != 32 {
		return fmt.Errorf("出口 Reality 公钥无效")
	}
	sid, err := hex.DecodeString(e.ShortID)
	if err != nil || len(sid) > 8 {
		return fmt.Errorf("出口 Short ID 无效")
	}
	if e.SNI == "" || strings.ContainsAny(e.SNI, " ,/\r\n") {
		return fmt.Errorf("出口 SNI 无效")
	}
	if e.Transport != "raw" && e.Transport != "xhttp" {
		return fmt.Errorf("出口仅支持 VLESS Reality TCP/XHTTP")
	}
	if (e.Flow != "" && e.Flow != "xtls-rprx-vision") || (e.Transport == "xhttp" && e.Flow != "") {
		return fmt.Errorf("出口 flow 不受支持")
	}
	return nil
}
func (e EgressSpec) outbound(tag string) map[string]any {
	fp := e.Fingerprint
	if fp == "" {
		fp = "chrome"
	}
	user := map[string]any{"id": e.UUID, "encryption": "none"}
	if e.Flow != "" {
		user["flow"] = e.Flow
	}
	stream := map[string]any{"network": e.Transport, "security": "reality", "realitySettings": map[string]any{"serverName": e.SNI, "fingerprint": fp, "publicKey": e.PublicKey, "shortId": e.ShortID}}
	if e.Transport == "xhttp" {
		path := e.Path
		if path == "" {
			path = "/"
		}
		stream["xhttpSettings"] = map[string]any{"path": path, "mode": "auto"}
	}
	return map[string]any{"tag": tag, "protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": e.Address, "port": e.Port, "users": []any{user}}}}, "streamSettings": stream}
}
func appendEgressConfig(spec Spec, collector *WebsiteCollector, inbounds, outbounds, rules []any) ([]any, []any, []any, error) {
	metered := spec.WebsiteTraffic != nil && spec.WebsiteTraffic.Enabled
	if metered {
		expanded, _, err := meteredRouteInbounds(spec.Inbounds)
		if err != nil {
			return nil, nil, nil, err
		}
		spec.Inbounds = expanded
	}
	for _, in := range spec.Inbounds {
		if err := validateDomainRoutes(in.DomainRoutes); err != nil {
			return nil, nil, nil, err
		}
		for i, r := range in.DomainRoutes {
			tag := "direct"
			if r.Egress != nil {
				tag = "route-exit-" + in.Tag + "-" + strconv.Itoa(i)
				outbounds = append(outbounds, r.Egress.outbound(tag))
			}
			pattern, _ := r.domainPattern()
			rules = append(rules, map[string]any{"inboundTag": []string{in.Tag}, "domain": []string{pattern}, "outboundTag": tag})
		}
		if in.Egress == nil {
			continue
		}
		if err := in.Egress.Validate(); err != nil {
			return nil, nil, nil, err
		}
		tag := "exit-" + in.Tag
		outbounds = append(outbounds, in.Egress.outbound(tag))
		source := in.Tag
		if metered {
			bridge := collector.egress[in.Tag]
			if bridge == nil {
				return nil, nil, nil, fmt.Errorf("出口统计转发缺失")
			}
			source = "bridge-" + in.Tag
			inbounds = append(inbounds, map[string]any{"tag": source, "listen": "127.0.0.1", "port": bridge.port, "protocol": "socks", "settings": map[string]any{"auth": "password", "accounts": []any{map[string]any{"user": bridge.user, "pass": bridge.password}}, "udp": true, "ip": "127.0.0.1"}})
		}
		rules = append(rules, map[string]any{"inboundTag": []string{source}, "outboundTag": tag})
	}
	return inbounds, outbounds, rules, nil
}
