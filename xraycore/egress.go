package xraycore

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"ppeelink/certpin"
	"strconv"
	"strings"
)

// EgressSpec describes a physical upstream; client-side proxy chains are not inherited.
type EgressSpec struct {
	Protocol     string   `json:"protocol,omitempty"`
	Address      string   `json:"address"`
	Port         int      `json:"port"`
	UUID         string   `json:"uuid,omitempty"`
	SNI          string   `json:"sni"`
	PublicKey    string   `json:"publicKey,omitempty"`
	ShortID      string   `json:"shortId,omitempty"`
	Transport    string   `json:"transport,omitempty"`
	Flow         string   `json:"flow,omitempty"`
	Path         string   `json:"path,omitempty"`
	Fingerprint  string   `json:"fingerprint,omitempty"`
	Password     string   `json:"password,omitempty"`
	PinSHA256    string   `json:"pinSHA256,omitempty"`
	Obfs         string   `json:"obfs,omitempty"`
	ObfsPassword string   `json:"obfsPassword,omitempty"`
	ALPN         []string `json:"alpn,omitempty"`
}

func (e EgressSpec) IsHY2() bool { return e.Protocol == "hysteria2" }

func validEgressHost(host string) bool {
	host = strings.TrimSuffix(host, ".")
	if ip := net.ParseIP(host); ip != nil {
		return !websitePrivateIP(ip)
	}
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if c != '-' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
				return false
			}
		}
	}
	lower := strings.ToLower(host)
	return !strings.HasSuffix(lower, ".localhost") && !strings.HasSuffix(lower, ".local") && !strings.HasSuffix(lower, ".internal")
}

func (e EgressSpec) Validate() error {
	if !validEgressHost(e.Address) || e.Port < 1 || e.Port > 65535 {
		return fmt.Errorf("出口需为有效公网地址和端口")
	}
	if e.Protocol != "" && e.Protocol != "vless" && !e.IsHY2() {
		return fmt.Errorf("出口协议不受支持")
	}
	if e.SNI == "" || strings.ContainsAny(e.SNI, " ,/\\\r\n\t") {
		return fmt.Errorf("出口 SNI 无效")
	}
	if e.IsHY2() {
		if e.Password == "" || len(e.Password) > 4096 {
			return fmt.Errorf("出口 HY2 密码无效")
		}
		if e.PinSHA256 != "" {
			if _, err := certpin.Normalize(e.PinSHA256); err != nil {
				return fmt.Errorf("出口 HY2 证书指纹无效")
			}
		}
		if e.Obfs != "" && e.Obfs != "salamander" || (e.Obfs == "") != (e.ObfsPassword == "") {
			return fmt.Errorf("出口 HY2 混淆参数无效")
		}
		for _, alpn := range e.ALPN {
			if len(alpn) == 0 || len(alpn) > 255 || strings.ContainsAny(alpn, ", \t\r\n") {
				return fmt.Errorf("出口 HY2 ALPN 无效")
			}
		}
		if e.UUID != "" || e.PublicKey != "" || e.ShortID != "" || e.Transport != "" || e.Flow != "" || e.Path != "" || e.Fingerprint != "" {
			return fmt.Errorf("出口 HY2 包含不支持的传输参数")
		}
		return nil
	}
	if e.Password != "" || e.PinSHA256 != "" || e.Obfs != "" || e.ObfsPassword != "" || len(e.ALPN) > 0 {
		return fmt.Errorf("出口 Reality 包含不支持的 HY2 参数")
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
	if e.Transport != "raw" && e.Transport != "xhttp" {
		return fmt.Errorf("出口仅支持 VLESS Reality TCP/XHTTP")
	}
	if e.Transport == "raw" && e.Path != "" {
		return fmt.Errorf("出口 TCP 不支持 path 参数")
	}
	if (e.Flow != "" && e.Flow != "xtls-rprx-vision") || (e.Transport == "xhttp" && e.Flow != "") {
		return fmt.Errorf("出口 flow 不受支持")
	}
	return nil
}
func (e EgressSpec) outbound(tag string) map[string]any {
	if e.IsHY2() {
		alpn := e.ALPN
		if len(alpn) == 0 {
			alpn = []string{"h3"}
		}
		tls := map[string]any{"serverName": e.SNI, "alpn": alpn}
		if e.PinSHA256 != "" {
			pin, _ := certpin.Normalize(e.PinSHA256)
			// Xray treats a matching leaf pin as the trust anchor, replacing
			// normal PKI/name verification. Never emit allowInsecure.
			tls["pinnedPeerCertSha256"] = pin
		}
		stream := map[string]any{"network": "hysteria", "security": "tls", "tlsSettings": tls, "hysteriaSettings": map[string]any{"version": 2, "auth": e.Password}}
		if e.Obfs == "salamander" {
			stream["finalmask"] = map[string]any{"udp": []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": e.ObfsPassword}}}}
		}
		return map[string]any{"tag": tag, "protocol": "hysteria", "settings": map[string]any{"version": 2, "address": e.Address, "port": e.Port}, "streamSettings": stream}
	}
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
