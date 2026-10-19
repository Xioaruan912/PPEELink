package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"gorm.io/gorm"
)

// 自建节点：面板在本机或远程服务器上运行 xray-core，按账户发放凭据并统计流量。

const (
	ServerKindLocal = "local" // 面板服务器：xray 与面板装在同一台机器，由面板进程直接管理
	ServerKindAgent = "agent" // 远程服务器上的 agent 主动连回面板
)

// ProxyServer 一台运行 xray-core 的服务器。
type ProxyServer struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	Name       string     `gorm:"size:80;not null" json:"name"`
	Kind       string     `gorm:"size:16;not null" json:"kind"`
	PublicHost string     `gorm:"size:255" json:"publicHost"` // 客户端连接用的域名或 IP
	Enabled    bool       `gorm:"not null;default:true" json:"enabled"`
	AgentToken string     `gorm:"size:64;index" json:"-"` // sha256(明文)，明文只在创建时返回一次
	XrayVer    string     `gorm:"size:40" json:"xrayVersion"`
	Arch       string     `gorm:"size:20" json:"arch"`
	Running    bool       `json:"running"`
	LastError  string     `gorm:"type:text" json:"lastError"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	AppliedRev string     `gorm:"size:64" json:"appliedRev"`              // agent 已应用的配置摘要
	Removing   bool       `gorm:"not null;default:false" json:"removing"` // 已在面板删除，等待 agent 卸载后回报
	// 月流量上限（字节，0 不限）与每月重置日：超出后停止 xray、节点暂不下发，到重置日自动恢复
	TrafficLimit int64     `json:"trafficLimit"`
	ResetDay     int       `json:"resetDay"`
	CreatedAt    time.Time `json:"createdAt"`
}

// ProxyInbound 服务器上的一个入站。
type ProxyInbound struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	ServerID   uint   `gorm:"index;not null" json:"serverId"`
	Remark     string `gorm:"size:80" json:"remark"`
	Template   string `gorm:"size:40;not null" json:"template"` // vless-reality-vision / vless-xhttp-reality
	Port       int    `gorm:"not null" json:"port"`
	Target     string `gorm:"size:255" json:"target"` // Reality 伪装目标 host:port
	SNI        string `gorm:"size:255" json:"sni"`
	PrivateKey string `gorm:"size:64" json:"-"`
	PublicKey  string `gorm:"size:64" json:"publicKey"`
	ShortID    string `gorm:"size:32" json:"shortId"`
	Path       string `gorm:"size:120" json:"path"` // XHTTP 路径
	Enabled    bool   `gorm:"not null;default:true" json:"enabled"`
}

// ProxyTraffic 每日每账户每服务器的流量记录（字节）。
type ProxyTraffic struct {
	ID       uint   `gorm:"primaryKey"`
	Day      string `gorm:"size:10;uniqueIndex:idx_proxy_traffic,priority:1" json:"day"`
	UserID   int    `gorm:"uniqueIndex:idx_proxy_traffic,priority:2" json:"userId"`
	ServerID uint   `gorm:"uniqueIndex:idx_proxy_traffic,priority:3" json:"serverId"`
	Up       int64  `json:"up"`
	Down     int64  `json:"down"`
}

// ProxyAccount 账户使用自建节点的权限、凭据与配额，与 User 一对一。
type ProxyAccount struct {
	UserID        int        `gorm:"primaryKey" json:"userId"`
	Enabled       bool       `gorm:"not null;default:false" json:"enabled"`
	UUID          string     `gorm:"size:36;uniqueIndex" json:"uuid"`
	SubToken      string     `gorm:"size:64;uniqueIndex" json:"-"`
	TrafficLimit  int64      `json:"trafficLimit"` // 字节，0 不限
	TrafficUsed   int64      `json:"trafficUsed"`
	ResetDay      int        `json:"resetDay"` // 每月几号清零，0 不清零
	LastResetAt   *time.Time `json:"lastResetAt"`
	ExpireAt      *time.Time `json:"expireAt"`
	DeviceLimit   int        `json:"deviceLimit"` // 同时在线 IP 数上限，0 不限
	BlockedUntil  *time.Time `json:"blockedUntil"`
	BlockedReason string     `gorm:"size:120" json:"blockedReason"`
	OnlineIPs     int        `json:"onlineIps"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

// Email xray 里用于统计和识别账户的标识。
func (a ProxyAccount) Email() string { return "u" + strconv.Itoa(a.UserID) }

// Active 账户此刻是否应当出现在 xray 配置里。
func (a ProxyAccount) Active(now time.Time) bool {
	if !a.Enabled || a.UUID == "" {
		return false
	}
	if a.TrafficLimit > 0 && a.TrafficUsed >= a.TrafficLimit {
		return false
	}
	if a.ExpireAt != nil && now.After(*a.ExpireAt) {
		return false
	}
	if a.BlockedUntil != nil && now.Before(*a.BlockedUntil) {
		return false
	}
	return true
}

// NewUUIDv4 生成随机 UUID（xray 用户 ID）。
func NewUUIDv4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b)
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// RandomHex 生成 n 字节随机数的十六进制串。
func RandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// HashAgentToken agent 令牌只存摘要。
func HashAgentToken(plain string) string {
	sum := sha256.Sum256([]byte("agent:" + plain))
	return hex.EncodeToString(sum[:])
}

// EnsureProxyAccount 读取或创建账户的自建节点记录（默认未启用）。
func EnsureProxyAccount(tx *gorm.DB, userID int) (ProxyAccount, error) {
	var acc ProxyAccount
	err := tx.Where("user_id = ?", userID).First(&acc).Error
	if err == nil {
		return acc, nil
	}
	if err != gorm.ErrRecordNotFound {
		return acc, err
	}
	acc = ProxyAccount{UserID: userID, UUID: NewUUIDv4(), SubToken: RandomHex(16)}
	return acc, tx.Create(&acc).Error
}

// AddProxyTraffic 累加一段流量到当日记录与账户已用量。
func AddProxyTraffic(tx *gorm.DB, day string, userID int, serverID uint, up, down int64) error {
	if up == 0 && down == 0 {
		return nil
	}
	var rec ProxyTraffic
	err := tx.Where("day = ? AND user_id = ? AND server_id = ?", day, userID, serverID).First(&rec).Error
	if err == gorm.ErrRecordNotFound {
		rec = ProxyTraffic{Day: day, UserID: userID, ServerID: serverID, Up: up, Down: down}
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if err := tx.Model(&rec).Updates(map[string]any{"up": rec.Up + up, "down": rec.Down + down}).Error; err != nil {
		return err
	}
	return tx.Model(&ProxyAccount{}).Where("user_id = ?", userID).
		Update("traffic_used", gorm.Expr("traffic_used + ?", up+down)).Error
}
