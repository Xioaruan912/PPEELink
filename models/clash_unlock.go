package models

import "time"

// ClashUnlockSnapshot 保存本机用户 Agent 通过真实 Clash 链路批量实测得到的节点解锁快照。
// 既可关联到自建 ProxyServer，也可独立记录第三方/中转节点，与服务端原生检测共同构成双源解锁矩阵。
type ClashUnlockSnapshot struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NodeName     string    `gorm:"size:160;uniqueIndex;not null" json:"nodeName"`
	CleanName    string    `gorm:"size:160;index;not null" json:"cleanName"`
	ServerID     *uint     `gorm:"index" json:"serverId,omitempty"`
	ProbeName    string    `gorm:"size:80" json:"probeName,omitempty"`
	EgressIP     string    `gorm:"size:64" json:"egressIp,omitempty"`
	Country      string    `gorm:"size:16" json:"country,omitempty"`
	TTFBMs       int       `json:"ttfbMs"`
	DownloadMbps float64   `json:"downloadMbps"`
	ItemsJSON    string    `gorm:"type:text" json:"itemsJson"`
	CheckedAt    time.Time `gorm:"index" json:"checkedAt"`
}
