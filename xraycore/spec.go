// Package xraycore 负责在一台服务器上运行 xray-core：根据面板下发的 Spec 生成配置、
// 安装与校验官方二进制、管理进程，并回报流量。面板本机与远程 agent 共用这一套逻辑，
// 本包不依赖数据库。
package xraycore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/Xioaruan912/PPEELink/monitorcore"
	"github.com/Xioaruan912/PPEELink/relaycore"
)

// DefaultVersion 默认安装的 xray-core 版本（官方 release，下载后校验 SHA2-256）。
const DefaultVersion = "v26.3.27"

// 入站模板（全量覆盖 3x-ui 主流协议）
const (
	TemplateRealityVision = "vless-reality-vision"
	TemplateXHTTPReality  = "vless-xhttp-reality"
	TemplateGRPCReality   = "vless-grpc-reality"
	TemplateVLESSWSTLS    = "vless-ws-tls"
	TemplateVMessWSTLS    = "vmess-ws-tls"
	TemplateVMessTCP      = "vmess-tcp"
	TemplateTrojanReality = "trojan-reality"
	TemplateTrojanTLS     = "trojan-tls"
	TemplateSS2022        = "shadowsocks-2022"
	TemplateHysteria2     = "hysteria2"
)

func IsValidTemplate(t string) bool {
	switch t {
	case TemplateRealityVision, TemplateXHTTPReality, TemplateGRPCReality,
		TemplateVLESSWSTLS, TemplateVMessWSTLS, TemplateVMessTCP,
		TemplateTrojanReality, TemplateTrojanTLS, TemplateSS2022, TemplateHysteria2:
		return true
	default:
		return false
	}
}

func TemplateUsesReality(t string) bool {
	switch t {
	case TemplateRealityVision, TemplateXHTTPReality, TemplateGRPCReality, TemplateTrojanReality:
		return true
	default:
		return false
	}
}

func TemplateUsesTLS(t string) bool {
	switch t {
	case TemplateHysteria2, TemplateVLESSWSTLS, TemplateVMessWSTLS, TemplateTrojanTLS:
		return true
	default:
		return false
	}
}

// SS2022KeyBytes returns the required raw key size for a Shadowsocks 2022 cipher.
func SS2022KeyBytes(method string) int {
	switch method {
	case "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return 32
	default:
		return 16
	}
}

// NormalizeSS2022Method validates or defaults the Shadowsocks 2022 cipher method.
func NormalizeSS2022Method(method string) string {
	switch method {
	case "2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return method
	default:
		return "2022-blake3-aes-128-gcm"
	}
}

// GenerateSS2022Key generates a random base64 PSK of the required length for method.
func GenerateSS2022Key(method string) string {
	b := make([]byte, SS2022KeyBytes(method))
	_, _ = rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// DeriveSS2022UserKey deterministically derives a valid SS-2022 per-user base64 key from UUID.
func DeriveSS2022UserKey(uuid, method string) string {
	sum := sha256.Sum256([]byte("ss2022:" + uuid))
	return base64.StdEncoding.EncodeToString(sum[:SS2022KeyBytes(method)])
}

// Spec 面板下发给一台服务器的完整期望状态。
type Spec struct {
	DeploymentProfile string           `json:"deploymentProfile,omitempty"`
	Relays            []relaycore.Spec `json:"relays,omitempty"`
	Version           string           `json:"version"`
	Enabled           bool             `json:"enabled"` // false 时停止 xray（一键停用）
	APIPort           int              `json:"apiPort"`
	Inbounds          []InboundSpec    `json:"inbounds"`
	// 防滥用：阻断访问内网与 BT 流量
	BlockPrivate    bool `json:"blockPrivate"`
	BlockBitTorrent bool `json:"blockBitTorrent"`
	// 服务器已在面板删除：停止并卸载 xray，agent 随后移除自身
	Uninstall bool `json:"uninstall,omitempty"`
	// Monitor 为服务器监控开关；开启后 agent 会在回报里附带指标。
	Monitor *MonitorSpec `json:"monitor,omitempty"`
	// WebsiteTraffic meters only Xray-forwarded payload; disabled by default.
	WebsiteTraffic *WebsiteTrafficSpec `json:"websiteTraffic,omitempty"`
	// AgentUpdate 为面板主动请求 agent 自更新；AgentHash 是已发布独立 Agent 产物的 SHA-256。
	AgentUpdate bool   `json:"agentUpdate,omitempty"`
	AgentHash   string `json:"agentHash,omitempty"`
}

// WebsiteTrafficSpec defaults to UTC hour buckets (3600 seconds). Custom bucket
// widths must divide one UTC day and be between 60 and 86400 seconds.
type WebsiteTrafficSpec struct {
	Enabled       bool `json:"enabled"`
	BucketSeconds int  `json:"bucketSeconds,omitempty"`
}

// WebsiteTrafficBucket contains successfully forwarded application payload bytes
// (no SOCKS/TCP/IP headers), attributed only to an explicit SOCKS domain target.
// BucketStart is a UTC Unix timestamp. Unknown/IP destinations use "unknown".
// InboundTag maps to the panel node, not a user-supplied hostname or server ID.
type WebsiteTrafficBucket struct {
	BucketStart int64  `json:"bucketStart"`
	InboundTag  string `json:"inboundTag"`
	Domain      string `json:"domain"`
	Up          int64  `json:"up"`
	Down        int64  `json:"down"`
}

type MonitorSpec struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"intervalSeconds"`
}

type InboundSpec struct {
	Listen       string            `json:"listen,omitempty"`
	DomainRoutes []DomainRouteSpec `json:"domainRoutes,omitempty"`
	TLSAutoID    string            `json:"tlsAutoId,omitempty"`
	Egress       *EgressSpec       `json:"egress,omitempty"`
	TLSCertFile  string            `json:"tlsCertFile,omitempty"`
	TLSKeyFile   string            `json:"tlsKeyFile,omitempty"`
	Tag          string            `json:"tag"`
	Template     string            `json:"template"`
	Port         int               `json:"port"`
	Target       string            `json:"target"`
	SNI          string            `json:"sni"`
	PrivateKey   string            `json:"privateKey"`
	ShortID      string            `json:"shortId"`
	Path         string            `json:"path"`
	Host         string            `json:"host,omitempty"`
	Method       string            `json:"method,omitempty"`
	Users        []UserSpec        `json:"users"`
}

type UserSpec struct {
	Email string `json:"email"`
	UUID  string `json:"uuid"`
}

// Rev 期望状态摘要，用于判断是否需要重新应用。
func (s Spec) Rev() string {
	// Only configuration consumed by Xray/its relay belongs in this revision.
	// Agent upgrades and metric sampling settings do not change proxy routing.
	s.Monitor, s.AgentUpdate, s.AgentHash, s.Relays = nil, false, "", nil
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:16]
}

// Counter 一段时间内的上下行字节。
type Counter struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

// Report 服务器回报给面板的运行状态。Traffic 为自上次回报以来的增量。
type Report struct {
	Relays         map[string]relaycore.Status `json:"relays,omitempty"`
	Certificates   map[string]string           `json:"certificates,omitempty"`
	ProxyFeatures  int                         `json:"proxyFeatures"`
	Rev            string                      `json:"rev"`
	XrayVersion    string                      `json:"xrayVersion"`
	AgentVersion   string                      `json:"agentVersion,omitempty"`
	AgentHash      string                      `json:"agentHash,omitempty"`
	Arch           string                      `json:"arch"`
	Running        bool                        `json:"running"`
	Error          string                      `json:"error"`
	Traffic        map[string]Counter          `json:"traffic"`
	Online         map[string]int              `json:"online"` // email -> 在线 IP 数
	Uninstalled    bool                        `json:"uninstalled,omitempty"`
	Monitor        *monitorcore.Metric         `json:"monitor,omitempty"`
	WebsiteTraffic []WebsiteTrafficBucket      `json:"websiteTraffic,omitempty"`
	WebsiteBatchID string                      `json:"websiteBatchId,omitempty"`
}
