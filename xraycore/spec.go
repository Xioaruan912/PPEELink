// Package xraycore 负责在一台服务器上运行 xray-core：根据面板下发的 Spec 生成配置、
// 安装与校验官方二进制、管理进程，并回报流量。面板本机与远程 agent 共用这一套逻辑，
// 本包不依赖数据库。
package xraycore

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// DefaultVersion 默认安装的 xray-core 版本（官方 release，下载后校验 SHA2-256）。
const DefaultVersion = "v26.3.27"

// 入站模板
const (
	TemplateRealityVision = "vless-reality-vision"
	TemplateXHTTPReality  = "vless-xhttp-reality"
)

// Spec 面板下发给一台服务器的完整期望状态。
type Spec struct {
	Version  string        `json:"version"`
	Enabled  bool          `json:"enabled"` // false 时停止 xray（一键停用）
	APIPort  int           `json:"apiPort"`
	Inbounds []InboundSpec `json:"inbounds"`
	// 防滥用：阻断访问内网与 BT 流量
	BlockPrivate    bool `json:"blockPrivate"`
	BlockBitTorrent bool `json:"blockBitTorrent"`
}

type InboundSpec struct {
	Tag        string     `json:"tag"`
	Template   string     `json:"template"`
	Port       int        `json:"port"`
	Target     string     `json:"target"`
	SNI        string     `json:"sni"`
	PrivateKey string     `json:"privateKey"`
	ShortID    string     `json:"shortId"`
	Path       string     `json:"path"`
	Users      []UserSpec `json:"users"`
}

type UserSpec struct {
	Email string `json:"email"`
	UUID  string `json:"uuid"`
}

// Rev 期望状态摘要，用于判断是否需要重新应用。
func (s Spec) Rev() string {
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
	Rev         string             `json:"rev"`
	XrayVersion string             `json:"xrayVersion"`
	Arch        string             `json:"arch"`
	Running     bool               `json:"running"`
	Error       string             `json:"error"`
	Traffic     map[string]Counter `json:"traffic"`
	Online      map[string]int     `json:"online"` // email -> 在线 IP 数
}
