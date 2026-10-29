package diagnostic

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"ppeelink/certpin"
	"strconv"
	"strings"
)

type profile struct {
	host, protocol, network string
	port                    int
	out                     map[string]any
}

func invalid(s string) error { return fmt.Errorf("%s", s) }
func parse(link string) (profile, error) {
	p := profile{}
	if len(link) > 16384 {
		return p, invalid("链接过长")
	}
	u, e := url.Parse(strings.TrimSpace(link))
	if e != nil || u.User == nil {
		return p, invalid("节点链接格式错误")
	}
	p.host = u.Hostname()
	p.port, _ = strconv.Atoi(u.Port())
	if p.host == "" || p.port < 1 || p.port > 65535 {
		return p, invalid("服务器地址或端口无效")
	}
	q := u.Query()
	stream := map[string]any{}
	p.out = map[string]any{"tag": "proxy", "streamSettings": stream}
	if q.Get("extra") != "" || q.Get("downloadSettings") != "" || q.Get("dialerProxy") != "" {
		return p, invalid("链接含扩展或链式连接配置，当前诊断无法完整还原，请使用标准节点链接")
	}
	switch strings.ToLower(u.Scheme) {
	case "vless":
		p.protocol = "Reality"
		if q.Get("encryption") != "" && q.Get("encryption") != "none" {
			return p, invalid("此链接使用扩展 VLESS 加密，诊断内核无法完整还原")
		}
		if q.Get("security") != "reality" {
			return p, invalid("此诊断支持 VLESS Reality 和 HY2")
		}
		id, e := hex.DecodeString(strings.ReplaceAll(u.User.Username(), "-", ""))
		if e != nil || len(id) != 16 {
			return p, invalid("Reality UUID 格式错误")
		}
		pk, e := base64.RawURLEncoding.DecodeString(q.Get("pbk"))
		if e != nil || len(pk) != 32 {
			return p, invalid("Reality 公钥格式错误")
		}
		sid, e := hex.DecodeString(q.Get("sid"))
		if e != nil || len(sid) > 8 {
			return p, invalid("Reality Short ID 格式错误")
		}
		if q.Get("sni") == "" {
			return p, invalid("Reality 缺少 SNI")
		}
		p.network = q.Get("type")
		if p.network == "" || p.network == "tcp" {
			p.network = "raw"
		}
		fp := q.Get("fp")
		if fp == "" {
			fp = "chrome"
		}
		stream["network"] = p.network
		stream["security"] = "reality"
		stream["realitySettings"] = map[string]any{"serverName": q.Get("sni"), "publicKey": q.Get("pbk"), "shortId": q.Get("sid"), "fingerprint": fp}
		switch p.network {
		case "raw":
			if q.Get("headerType") != "" && q.Get("headerType") != "none" {
				stream["rawSettings"] = map[string]any{"header": map[string]any{"type": q.Get("headerType")}}
			}
		case "xhttp", "splithttp":
			stream["network"] = "xhttp"
			mode := q.Get("mode")
			if mode == "" {
				mode = "auto"
			}
			stream["xhttpSettings"] = map[string]any{"path": q.Get("path"), "host": q.Get("host"), "mode": mode}
		case "grpc":
			stream["grpcSettings"] = map[string]any{"serviceName": q.Get("serviceName")}
		case "ws":
			stream["wsSettings"] = map[string]any{"path": q.Get("path"), "host": q.Get("host")}
		case "httpupgrade":
			stream["httpupgradeSettings"] = map[string]any{"path": q.Get("path"), "host": q.Get("host")}
		default:
			return p, invalid("当前 Xray 诊断不支持该 Reality 传输")
		}
		p.out["protocol"] = "vless"
		p.out["settings"] = map[string]any{"vnext": []any{map[string]any{"address": p.host, "port": p.port, "users": []any{map[string]any{"id": u.User.Username(), "encryption": "none", "flow": q.Get("flow")}}}}}
	case "hy2", "hysteria2":
		p.protocol = "HY2"
		p.network = "udp"
		password := u.User.String()
		password, _ = url.PathUnescape(password)
		if password == "" {
			return p, invalid("HY2 密码为空")
		}
		sni := q.Get("sni")
		if sni == "" {
			sni = p.host
		}
		tls := map[string]any{"serverName": sni, "alpn": []string{"h3"}}
		if pin := q.Get("pinSHA256"); pin != "" {
			pin, e = certpin.Normalize(pin)
			if e != nil {
				return p, invalid("HY2 证书指纹格式错误")
			}
			tls["pinnedPeerCertSha256"] = pin
		}
		if q.Get("insecure") == "1" && q.Get("pinSHA256") == "" {
			return p, invalid("HY2 关闭了证书校验且没有指纹，无法安全验证服务器；请补充可信证书或 pinSHA256")
		}
		if cert := q.Get("subl-cert"); cert != "" {
			data, e := base64.RawURLEncoding.DecodeString(cert)
			if e != nil || certpin.Validate(string(data), q.Get("pinSHA256"), sni) != nil {
				return p, invalid("HY2 公有证书与指纹或 SNI 不匹配")
			}
		}
		stream["network"] = "hysteria"
		stream["security"] = "tls"
		stream["tlsSettings"] = tls
		stream["hysteriaSettings"] = map[string]any{"version": 2, "auth": password}
		if obfs := q.Get("obfs"); obfs != "" {
			if obfs != "salamander" || q.Get("obfs-password") == "" {
				return p, invalid("HY2 混淆参数无效或不受当前内核支持")
			}
			stream["finalmask"] = map[string]any{"udp": []any{map[string]any{"type": "salamander", "settings": map[string]any{"password": q.Get("obfs-password")}}}}
		}
		if alpn := q.Get("alpn"); alpn != "" {
			tls["alpn"] = strings.Split(alpn, ",")
		}
		p.out["protocol"] = "hysteria"
		p.out["settings"] = map[string]any{"version": 2, "address": p.host, "port": p.port}
	default:
		return p, invalid("此诊断支持 Reality / HY2 链接")
	}
	return p, nil
}
func (p profile) pinAddress(ip string) {
	s := p.out["settings"].(map[string]any)
	if p.protocol == "HY2" {
		s["address"] = ip
	} else {
		s["vnext"].([]any)[0].(map[string]any)["address"] = ip
	}
}
func publicIP(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, s := range []string{"100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16"} {
		_, n, _ := net.ParseCIDR(s)
		if n.Contains(ip) {
			return false
		}
	}
	return true
}
func config(p profile, port int, user, pass string) []byte {
	b, _ := json.Marshal(map[string]any{"log": map[string]any{"loglevel": "warning"}, "inbounds": []any{map[string]any{"listen": "127.0.0.1", "port": port, "protocol": "socks", "settings": map[string]any{"auth": "password", "accounts": []any{map[string]any{"user": user, "pass": pass}}}}}, "outbounds": []any{p.out}})
	return b
}
