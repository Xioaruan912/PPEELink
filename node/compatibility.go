package node

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"ppeelink/certpin"
)

const CompatibilityRevision = "clients-2026-10-30-v1"

var ErrNoCompatibleNodes = errors.New("没有可供此客户端导出的节点，请在客户端兼容中查看原因")

type ClientProfile struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	Note string `json:"note"`
}

var ClientProfiles = []ClientProfile{
	{"clash", "Clash / Mihomo", "Reality、HY2 等现代协议需要 Mihomo 内核"},
	{"loon", "Loon", "按当前 Loon 导出器检查；请使用支持对应协议的新版客户端"},
	{"surge", "Surge", "HY2 需要 iOS 5.8.0 / Mac 5.4.0 或更新版本"},
	{"singbox", "sing-box", "按当前 sing-box JSON 导出器检查"},
	{"qx", "Quantumult X", "仅导出当前转换器可完整表达的参数"},
	{"stash", "Stash", "使用 Clash 配置格式，协议能力单独检查"},
	{"egern", "Egern", "沿用 Clash 配置导出，未建立独立能力基准"},
	{"surfboard", "Surfboard", "使用 Surge 风格配置，不等于具有 Surge 的全部协议能力"},
	{"shadowrocket", "Shadowrocket", "原始分享链接直出，实际支持取决于客户端版本"},
	{"v2ray", "通用链接 / V2Ray", "原始分享链接直出，HY2 等需要客户端配备对应内核"},
}

type CompatibilityInput struct {
	ID   int
	Name string
	Link string
}
type CompatibilityItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Protocol string `json:"protocol"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}
type ClientCompatibility struct {
	ClientProfile
	Total    int                 `json:"total"`
	Included int                 `json:"included"`
	Warnings int                 `json:"warnings"`
	Excluded int                 `json:"excluded"`
	Items    []CompatibilityItem `json:"items"`
}

func parseCompatibility(link string) (p Outbound, err error) {
	defer func() {
		if recover() != nil {
			err = errors.New("invalid node")
		}
	}()
	p, err = ParseOutbound(link)
	if err == nil && (p.Server == "" || p.Port < 1 || p.Port > 65535) {
		err = errors.New("invalid endpoint")
	}
	return
}
func transportSupported(t string, allowed ...string) bool {
	t = strings.ToLower(t)
	if t == "" || t == "raw" {
		t = "tcp"
	}
	for _, v := range allowed {
		if t == v {
			return true
		}
	}
	return false
}
func checkClient(p Outbound, client string) (string, string) {
	if client == "clash-proxies" {
		client = "clash"
	}
	raw := client == "v2ray" || client == "shadowrocket"
	if raw {
		return "warning", "原始链接直出；需由客户端版本及所用内核确认协议支持"
	}
	if client == "egern" {
		return "unsupported", "Egern 当前没有独立导出适配，不能把 Clash 配置兼容视为协议兼容"
	}
	if client == "surfboard" {
		client = "surge"
		if p.Type != "ss" && p.Type != "vmess" && p.Type != "trojan" {
			return "unsupported", "当前 Surfboard 导出仅支持 SS、VMess、Trojan"
		}
	}
	if client == "stash" {
		if p.Type == "vless" || p.Type == "anytls" || p.Type == "ssh" || p.Type == "snell" {
			return "unsupported", "当前 Stash 导出未适配该协议；请使用其他客户端"
		}
		client = "clash"
	}
	if client != "clash" && client != "singbox" && client != "surge" && client != "loon" && client != "qx" {
		return "unsupported", "未识别的客户端"
	}
	if (client == "surge" || client == "loon" || client == "qx") && strings.ContainsAny(p.Name, "\r\n,=") {
		return "unsupported", "节点名称含配置分隔符，请先修改名称"
	}
	switch v := p.Spec.(type) {
	case VLESS:
		if v.Uuid == "" {
			return "unsupported", "VLESS 缺少账户凭据"
		}
		if v.Query.Encryption != "" && v.Query.Encryption != "none" {
			return "unsupported", "当前导出器未适配此 VLESS 加密参数"
		}
		if v.Query.HeaderType != "" && v.Query.HeaderType != "none" {
			return "unsupported", "当前导出器未适配此 VLESS 头部伪装"
		}
		if client == "surge" {
			return "unsupported", "当前 Surge 原生导出不支持 VLESS / Reality"
		}
		if !transportSupported(v.Query.Type, "tcp", "ws", "grpc", "http") {
			return "unsupported", "当前导出器未适配 " + v.Query.Type + " 传输，不能自动转为 TCP"
		}
		if (client == "loon" || client == "qx") && !transportSupported(v.Query.Type, "tcp", "ws") {
			return "unsupported", "当前导出器仅支持 TCP、WebSocket 传输"
		}
		if client == "clash" && !transportSupported(v.Query.Type, "tcp", "ws", "grpc") {
			return "unsupported", "当前 Clash 导出器未适配该传输参数"
		}
		if v.Query.Security != "" && v.Query.Security != "none" && v.Query.Security != "tls" && v.Query.Security != "reality" {
			return "unsupported", "未适配此 VLESS 安全参数"
		}
		if v.Query.Security == "reality" {
			key, e := base64.RawURLEncoding.DecodeString(v.Query.Pbk)
			if e != nil || len(key) != 32 {
				return "unsupported", "Reality 公钥无效或缺失"
			}
			sid, e := hex.DecodeString(v.Query.Sid)
			if e != nil || len(sid) > 8 {
				return "unsupported", "Reality Short ID 格式无效"
			}
			if v.Query.Sni == "" {
				return "unsupported", "Reality 缺少 SNI"
			}
			if client == "qx" {
				return "unsupported", "当前 Quantumult X 导出器不能完整保留 Reality 公钥和 Short ID"
			}
		}
		if v.Query.Flow != "" && v.Query.Flow != "xtls-rprx-vision" {
			return "unsupported", "当前导出器未适配此 VLESS Flow"
		}
		if v.Query.Flow != "" && !transportSupported(v.Query.Type, "tcp") {
			return "unsupported", "Vision 不能与此传输组合导出"
		}
		if client == "qx" && v.Query.Flow != "" {
			return "unsupported", "当前 Quantumult X 导出器未适配 Vision"
		}
		if client == "clash" && v.Query.Security == "reality" {
			return "warning", "需要 Mihomo / Clash.Meta 内核；旧版 Clash 不支持 Reality"
		}
	case HY2:
		if v.Password == "" {
			return "unsupported", "HY2 缺少认证密码"
		}
		if client == "qx" {
			return "unsupported", "当前 Quantumult X 导出器不支持 HY2"
		}
		if v.Obfs != "" && v.Obfs != "salamander" {
			return "unsupported", "当前导出器仅适配 Salamander 混淆"
		}
		if v.Obfs != "" && v.ObfsPassword == "" {
			return "unsupported", "HY2 混淆密码缺失"
		}
		if v.PinSHA256 != "" {
			if _, e := certpin.Normalize(v.PinSHA256); e != nil {
				return "unsupported", "HY2 证书指纹格式无效"
			}
			if client == "singbox" {
				if _, e := hy2TLS(v); e != nil {
					return "unsupported", "sing-box 导出需要与指纹对应的公有证书"
				}
			}
		}
		if client == "surge" && v.Obfs != "" {
			return "warning", "Salamander 需要 Surge Mac 6.4.3 或更新版本；不适用于 iOS"
		}
		if v.Insecure != 0 && v.PinSHA256 == "" {
			return "warning", "此节点使用自签证书模式，客户端将按节点设置跳过证书校验"
		}
		if client == "clash" {
			return "warning", "需要支持 HY2 的 Mihomo 内核"
		}
	case Vmess:
		aid, e := convertToInt(v.Aid)
		if v.Aid != nil && v.Aid != "" && e != nil {
			return "unsupported", "VMess Alter ID 格式无效"
		}
		if aid != 0 && client != "clash" && client != "singbox" {
			return "unsupported", "当前导出器仅支持 Alter ID 为 0 的 VMess 认证"
		}
		allowed := transportSupported(v.Net, "tcp", "ws")
		if client == "singbox" {
			allowed = transportSupported(v.Net, "tcp", "ws", "grpc")
		}
		if !allowed {
			return "unsupported", "当前 VMess 导出器不能完整表达该传输参数"
		}
	case Trojan:
		if !transportSupported(v.Query.Type, "tcp", "ws") {
			return "unsupported", "当前 Trojan 导出器不能完整表达该传输参数"
		}
		if client == "surge" && !transportSupported(v.Query.Type, "tcp") {
			return "unsupported", "当前 Surge Trojan 导出器未适配该传输"
		}
	case Tuic:
		if client == "loon" || client == "qx" || client == "surge" {
			return "unsupported", "当前导出器未完整适配此 TUIC 节点的版本和认证参数"
		}
	case Ss:
		if u, e := url.Parse(p.Link); e == nil && u.Query().Get("plugin") != "" {
			return "unsupported", "此 SS 节点需要插件，当前导出器未适配该插件参数"
		}
	default:
		allowed := client == "clash" || (client == "singbox" && (p.Type == "anytls" || p.Type == "socks5" || p.Type == "socks5h" || p.Type == "http" || p.Type == "https" || p.Type == "wireguard" || p.Type == "wg")) || (client == "qx" && (p.Type == "socks5" || p.Type == "socks5h" || p.Type == "http"))
		if !allowed {
			return "unsupported", "当前客户端导出器未适配此协议"
		}
	}
	return "supported", "当前导出器可保留连接所需参数"
}

// FilterClientLinks is shared by real exporters and compatibility previews.
func FilterClientLinks(links []string, client string) ([]string, int) {
	out := make([]string, 0, len(links))
	excluded := 0
	for _, link := range links {
		p, err := parseCompatibility(link)
		if err != nil && client != "v2ray" && client != "shadowrocket" {
			excluded++
			continue
		}
		status, _ := checkClient(p, client)
		if status == "unsupported" {
			excluded++
			continue
		}
		out = append(out, link)
	}
	return out, excluded
}
func BuildCompatibility(inputs []CompatibilityInput) []ClientCompatibility {
	out := make([]ClientCompatibility, len(ClientProfiles))
	for i, profile := range ClientProfiles {
		out[i] = ClientCompatibility{ClientProfile: profile, Total: len(inputs), Items: []CompatibilityItem{}}
	}
	for _, input := range inputs {
		p, err := parseCompatibility(input.Link)
		for i := range out {
			status, reason := "unsupported", "节点链接无法解析或地址、端口无效"
			if err == nil {
				status, reason = checkClient(p, out[i].Key)
			} else if out[i].Key == "v2ray" || out[i].Key == "shadowrocket" {
				status, reason = "warning", "未识别的分享链接将原样导出，需由客户端确认支持"
			}
			name := input.Name
			if name == "" {
				name = p.Name
			}
			if name == "" {
				name = "未命名节点"
			}
			out[i].Items = append(out[i].Items, CompatibilityItem{ID: input.ID, Name: name, Protocol: p.Type, Status: status, Reason: reason})
			if status == "unsupported" {
				out[i].Excluded++
			} else {
				out[i].Included++
				if status == "warning" {
					out[i].Warnings++
				}
			}
		}
	}
	return out
}
