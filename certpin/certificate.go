package certpin

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"strings"
	"time"
)

func Normalize(pin string) (string, error) {
	pin = strings.ToLower(strings.ReplaceAll(pin, ":", ""))
	b, e := hex.DecodeString(pin)
	if e != nil || len(b) != 32 {
		return "", fmt.Errorf("证书指纹无效")
	}
	return pin, nil
}
func Parse(public string) (*x509.Certificate, string, error) {
	if len(public) > 8192 {
		return nil, "", fmt.Errorf("公有证书过大")
	}
	block, rest := pem.Decode([]byte(public))
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, "", fmt.Errorf("公有证书格式无效")
	}
	c, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		return nil, "", fmt.Errorf("公有证书解析失败")
	}
	now := time.Now()
	if now.Before(c.NotBefore) || !now.Before(c.NotAfter) {
		return nil, "", fmt.Errorf("证书尚未生效或已过期")
	}
	sum := sha256.Sum256(c.Raw)
	return c, hex.EncodeToString(sum[:]), nil
}
func Validate(public, pin, sni string) error {
	c, actual, e := Parse(public)
	if e != nil {
		return e
	}
	expected, e := Normalize(pin)
	if e != nil || actual != expected {
		return fmt.Errorf("证书与指纹不匹配")
	}
	return c.VerifyHostname(sni)
}
