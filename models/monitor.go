package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// Monitor 是一台被独立监控的服务器，和节点探针是两套不同的东西：
// 节点探针检查节点端口能不能连上，Monitor 检查服务器本身是否健康。
type Monitor struct {
	ID                   uint       `gorm:"primaryKey" json:"id"`
	Name                 string     `gorm:"size:80;not null" json:"name"`
	Group                string     `gorm:"size:40" json:"group"`
	Note                 string     `gorm:"size:255" json:"note"`
	ProxyServerID        *uint      `gorm:"uniqueIndex" json:"proxyServerId,omitempty"`
	SSHHost              string     `gorm:"size:255" json:"sshHost"`
	SSHPort              int        `gorm:"not null;default:22" json:"sshPort"`
	SSHUser              string     `gorm:"size:80" json:"sshUser"`
	SSHPasswordEnc       string     `gorm:"type:text" json:"-"`
	TokenHash            string     `gorm:"size:64;index;not null" json:"-"`
	Enabled              bool       `gorm:"not null;default:true" json:"enabled"`
	Hidden               bool       `gorm:"not null;default:false" json:"hidden"`
	OS                   string     `gorm:"size:40" json:"os"`
	Arch                 string     `gorm:"size:20" json:"arch"`
	Version              string     `gorm:"size:32" json:"version"`
	AgentHash            string     `gorm:"size:64" json:"agentHash"`
	AgentUpdateRequested bool       `gorm:"not null;default:false" json:"agentUpdateRequested"`
	AgentUpdateError     string     `gorm:"type:text" json:"agentUpdateError"`
	LastSeenAt           *time.Time `json:"lastSeenAt"`
	CreatedAt            time.Time  `json:"createdAt"`
}

// MonitorMetric 是一次服务器指标上报。原始数据保留 7 天，页面查询时降采样。
type MonitorMetric struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	MonitorID    uint      `gorm:"index:idx_monitor_metric_time,priority:1;not null" json:"monitorId"`
	CollectedAt  time.Time `gorm:"index:idx_monitor_metric_time,priority:2;not null" json:"collectedAt"`
	CPUPercent   float64   `json:"cpuPercent"`
	MemUsed      uint64    `json:"memUsed"`
	MemTotal     uint64    `json:"memTotal"`
	SwapUsed     uint64    `json:"swapUsed"`
	SwapTotal    uint64    `json:"swapTotal"`
	DiskUsed     uint64    `json:"diskUsed"`
	DiskTotal    uint64    `json:"diskTotal"`
	Load1        float64   `json:"load1"`
	Load5        float64   `json:"load5"`
	Load15       float64   `json:"load15"`
	Uptime       uint64    `json:"uptime"`
	NetInSpeed   uint64    `json:"netInSpeed"`
	NetOutSpeed  uint64    `json:"netOutSpeed"`
	TransferIn   uint64    `json:"transferIn"`
	TransferOut  uint64    `json:"transferOut"`
	TCPConns     int       `json:"tcpConns"`
	UDPConns     int       `json:"udpConns"`
	ProcessCount int       `json:"processCount"`
}

// HashMonitorToken 监控令牌使用独立前缀，避免和 agent / 节点探针令牌混用。
func HashMonitorToken(plain string) string {
	sum := sha256.Sum256([]byte("monitor:" + plain))
	return hex.EncodeToString(sum[:])
}

func CleanupMonitorMetrics(before time.Time) error {
	return DB.Where("collected_at < ?", before).Delete(&MonitorMetric{}).Error
}
