package models

import "time"

// AgentRelay is an opaque transport line. Its bytes are not per-user/site billing.
type AgentRelay struct {
	ID              uint       `gorm:"primaryKey" json:"id"`
	ServerID        uint       `gorm:"index;not null" json:"serverId"`
	Name            string     `gorm:"size:80;not null" json:"name"`
	ListenPort      int        `json:"listenPort"`
	Network         string     `gorm:"size:8" json:"network"`
	TargetNodeID    int        `gorm:"index" json:"targetNodeId"`
	Enabled         bool       `json:"enabled"`
	NodeID          int        `json:"nodeId"`
	Revision        string     `gorm:"size:64" json:"revision"`
	AppliedRevision string     `gorm:"size:64" json:"appliedRevision"`
	Running         bool       `json:"running"`
	LastError       string     `gorm:"size:500" json:"lastError"`
	LastSeenAt      *time.Time `json:"lastSeenAt"`
	Up              int64      `json:"up"`
	Down            int64      `json:"down"`
	TCPConnections  int        `json:"tcpConnections"`
	UDPSessions     int        `json:"udpSessions"`
	AgentSession    string     `gorm:"size:128" json:"-"`
	LastUp          int64      `json:"-"`
	LastDown        int64      `json:"-"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}
type AgentRelayDay struct {
	ID       uint   `gorm:"primaryKey"`
	RuleID   uint   `gorm:"uniqueIndex:idx_relay_day,priority:1"`
	Day      string `gorm:"size:10;uniqueIndex:idx_relay_day,priority:2;index"`
	ServerID uint   `gorm:"index"`
	Up       int64
	Down     int64
}
