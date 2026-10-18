package xraycore

import (
	"crypto/ecdh"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
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

// TargetCheck 伪装目标的可用性：Reality 要求目标支持 TLS 1.3（最好也支持 h2）。
type TargetCheck struct {
	OK      bool   `json:"ok"`
	TLS13   bool   `json:"tls13"`
	H2      bool   `json:"h2"`
	Latency int    `json:"latency"`
	Message string `json:"message"`
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
		res.Message = "可用"
	}
	return res
}
