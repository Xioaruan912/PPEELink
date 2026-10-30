package xraycore

import (
	"fmt"
	"net"
	"path/filepath"
	"regexp"
	"strings"
)

var hysteriaDomain = regexp.MustCompile(`(?i)^[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?$`)

// ValidateHysteriaTLS validates target-host paths without trying to read remote files on the panel.
func ValidateHysteriaTLS(sni, cert, key string) error {
	if len(sni) > 253 || !hysteriaDomain.MatchString(sni) || strings.Contains(sni, "..") || strings.TrimSpace(sni) == "" || strings.ContainsAny(sni, " /\\,\r\n\t") || net.ParseIP(sni) != nil {
		return fmt.Errorf("Hysteria2 需填写证书对应的域名")
	}
	for _, p := range []string{cert, key} {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || len(p) > 1024 || strings.ContainsAny(p, "\x00\r\n") {
			return fmt.Errorf("证书和私钥需填写目标服务器上的规范绝对路径")
		}
	}
	if cert == key {
		return fmt.Errorf("证书与私钥不能使用相同文件")
	}
	return nil
}
func buildHysteriaInbound(in InboundSpec) (map[string]any, error) {
	if in.Port <= 0 || in.Port > 65535 {
		return nil, fmt.Errorf("端口无效")
	}
	if err := ValidateHysteriaTLS(in.SNI, in.TLSCertFile, in.TLSKeyFile); err != nil {
		return nil, err
	}
	listen, err := NormalizeListen(in.Listen)
	if err != nil {
		return nil, err
	}
	users := make([]any, 0, len(in.Users))
	for _, u := range in.Users {
		if u.UUID == "" {
			return nil, fmt.Errorf("Hysteria2 账户凭据为空")
		}
		users = append(users, map[string]any{"auth": u.UUID, "email": u.Email, "level": 0})
	}
	item := map[string]any{"tag": in.Tag, "port": in.Port, "protocol": "hysteria",
		"settings":       map[string]any{"version": 2, "clients": users},
		"streamSettings": map[string]any{"network": "hysteria", "security": "tls", "tlsSettings": map[string]any{"serverName": in.SNI, "minVersion": "1.3", "alpn": []string{"h3"}, "certificates": []any{map[string]any{"certificateFile": in.TLSCertFile, "keyFile": in.TLSKeyFile}}}, "hysteriaSettings": map[string]any{"version": 2, "udpIdleTimeout": 60}},
		"sniffing":       map[string]any{"enabled": true, "destOverride": []string{"http", "tls", "quic"}, "routeOnly": true}}
	if listen != "" {
		item["listen"] = listen
	}
	return item, nil
}
