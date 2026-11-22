package xraycore

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"time"
)

// GenerateRealityKeys 生成 Reality 用的 X25519 密钥对（base64 raw URL，与 `xray x25519` 一致）。
func GenerateRealityKeys() (privateKey, publicKey string, err error) {
	key, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return "", "", err
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(key.Bytes()), enc.EncodeToString(key.PublicKey().Bytes()), nil
}

// PublicKeyFor 由私钥推出公钥，用于校验与回填。
func PublicKeyFor(privateKey string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(privateKey)
	if err != nil {
		return "", err
	}
	key, err := ecdh.X25519().NewPrivateKey(raw)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), nil
}

// TargetCheck 伪装目标的可用性：Reality 要求目标支持 TLS 1.3（最好也支持 h2），且排除 Cloudflare CDN。
type TargetCheck struct {
	OK         bool   `json:"ok"`
	TLS13      bool   `json:"tls13"`
	H2         bool   `json:"h2"`
	Cloudflare bool   `json:"cloudflare,omitempty"`
	Latency    int    `json:"latency"`
	Message    string `json:"message"`
}

var cloudflareNets = func() []*net.IPNet {
	cidrs := []string{
		// Official Cloudflare IPv4 ranges
		"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
		"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
		"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
		"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
		// Official Cloudflare IPv6 ranges
		"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
		"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
	}
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		if _, n, err := net.ParseCIDR(c); err == nil {
			out = append(out, n)
		}
	}
	return out
}()

// IsCloudflareIP reports whether ip falls inside Cloudflare's published edge ranges.
func IsCloudflareIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, n := range cloudflareNets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

func isCloudflareDomainName(host string) bool {
	h := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
	for _, suffix := range []string{"cloudflare.com", "cloudflare.net", "cloudflaressl.com", "workers.dev", "pages.dev"} {
		if h == suffix || strings.HasSuffix(h, "."+suffix) {
			return true
		}
	}
	return false
}

func isCloudflareCert(state tls.ConnectionState) bool {
	if len(state.PeerCertificates) == 0 {
		return false
	}
	leaf := state.PeerCertificates[0]
	for _, san := range leaf.DNSNames {
		if strings.Contains(strings.ToLower(san), "cloudflaressl.com") {
			return true
		}
	}
	for _, org := range leaf.Subject.Organization {
		if strings.Contains(strings.ToLower(org), "cloudflare") {
			return true
		}
	}
	return false
}

// IsCloudflareTarget checks whether the Reality target/SNI resolves to or belongs to Cloudflare CDN.
func IsCloudflareTarget(target, sni string, timeout time.Duration) bool {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		host = strings.TrimSpace(target)
	}
	if isCloudflareDomainName(host) || isCloudflareDomainName(sni) {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return IsCloudflareIP(ip)
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return false
	}
	for _, ip := range ips {
		if IsCloudflareIP(ip.IP) {
			return true
		}
	}
	return false
}

// CheckRealityTarget 从当前机器握手目标站点；面板与 agent 都可调用。
func CheckRealityTarget(target, sni string, timeout time.Duration) TargetCheck {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return TargetCheck{Message: "目标需为 host:port"}
	}
	if sni == "" {
		sni = host
	}
	if isCloudflareDomainName(host) || isCloudflareDomainName(sni) {
		return TargetCheck{
			Cloudflare: true,
			Message:    "已排除：该目标使用 Cloudflare CDN，存在被用于攻击的风险，请更换非 CF 站点（如 www.amd.com）",
		}
	}
	start := time.Now()
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", target, &tls.Config{
		ServerName: sni,
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1"},
	})
	if err != nil {
		return TargetCheck{Message: fmt.Sprintf("握手失败: %v", err)}
	}
	defer conn.Close()
	state := conn.ConnectionState()
	remoteCF := false
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok && IsCloudflareIP(addr.IP) {
		remoteCF = true
	}
	if remoteCF || isCloudflareCert(state) {
		return TargetCheck{
			TLS13:      state.Version == tls.VersionTLS13,
			H2:         state.NegotiatedProtocol == "h2",
			Cloudflare: true,
			Latency:    int(time.Since(start).Milliseconds()),
			Message:    "已排除：该目标使用 Cloudflare CDN，存在被用于攻击的风险，请更换非 CF 站点（如 www.amd.com）",
		}
	}
	res := TargetCheck{
		TLS13:   state.Version == tls.VersionTLS13,
		H2:      state.NegotiatedProtocol == "h2",
		Latency: int(time.Since(start).Milliseconds()),
	}
	res.OK = res.TLS13
	switch {
	case !res.TLS13:
		res.Message = "目标不支持 TLS 1.3，不能用作 Reality 伪装"
	case !res.H2:
		res.Message = "可用，但目标不支持 h2，建议换一个"
	default:
		res.Message = fmt.Sprintf("可用 · %dms", res.Latency)
	}
	return res
}
