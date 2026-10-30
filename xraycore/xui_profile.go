package xraycore

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// NormalizeReality follows x-ui's target/serverNames/shortIds wire contract.
// Client-only fingerprint/spiderX never enter server realitySettings.
func NormalizeReality(target, sni, shortIDs string) (string, string, string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(target))
	if err != nil || host == "" {
		return "", "", "", fmt.Errorf("Reality 目标需为主机:端口")
	}
	n, e := strconv.Atoi(port)
	if e != nil || n < 1 || n > 65535 {
		return "", "", "", fmt.Errorf("Reality 目标端口无效")
	}
	if strings.ContainsAny(host, " /\\\r\n\t") {
		return "", "", "", fmt.Errorf("Reality 目标主机无效")
	}
	names := []string{}
	seen := map[string]bool{}
	if strings.TrimSpace(sni) == "" {
		sni = host
	}
	for _, part := range strings.Split(sni, ",") {
		name := strings.TrimSpace(part)
		if name == "" || len(name) > 253 || net.ParseIP(name) != nil || strings.ContainsAny(name, " /\\\r\n\t") {
			return "", "", "", fmt.Errorf("Reality SNI 需填写域名，多个域名用逗号分隔")
		}
		if !seen[name] {
			names = append(names, name)
			seen[name] = true
		}
	}
	ids := []string{}
	seen = map[string]bool{}
	for _, part := range strings.Split(shortIDs, ",") {
		id := strings.ToLower(strings.TrimSpace(part))
		raw, e := hex.DecodeString(id)
		if e != nil || len(raw) > 8 {
			return "", "", "", fmt.Errorf("Short ID 需为偶数位十六进制，最长16位")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if len(names) > 16 || len(ids) > 16 {
		return "", "", "", fmt.Errorf("最多填写16个 SNI / Short ID")
	}
	return net.JoinHostPort(host, port), strings.Join(names, ","), strings.Join(ids, ","), nil
}
func NormalizeListen(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	ip := net.ParseIP(strings.Trim(raw, "[]"))
	if ip == nil || ip.IsMulticast() {
		return "", fmt.Errorf("监听地址需为 IP，留空为双栈监听")
	}
	return ip.String(), nil
}
