package models

import (
	"encoding/json"
	"time"
)

type ThroughputCapability struct {
	Path                string     `gorm:"size:512" json:"path"`
	PrepareID           uint       `json:"-"`
	ServerID            uint       `gorm:"primaryKey" json:"serverId"`
	Available           bool       `json:"available"`
	AuthSupported       bool       `json:"authSupported"`
	BidirSupported      bool       `json:"bidirSupported"`
	JSONStreamSupported bool       `json:"jsonStreamSupported"`
	Version             string     `gorm:"size:80" json:"version"`
	Error               string     `gorm:"size:512" json:"error"`
	SeenAt              time.Time  `json:"seenAt"`
	PrepareRequested    bool       `json:"prepareRequested"`
	PrepareAt           *time.Time `json:"prepareAt,omitempty"`
}

type ThroughputRun struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	LandingID   uint       `gorm:"index" json:"landingId"`
	LandingName string     `gorm:"size:120" json:"landingName"`
	Target      string     `gorm:"size:253" json:"target"`
	Protocol    string     `gorm:"size:8" json:"protocol"`
	Direction   string     `gorm:"size:12" json:"direction"`
	Family      int        `json:"family"`
	Port        int        `json:"port"`
	Duration    int        `json:"duration"`
	Streams     int        `json:"streams"`
	RateMbps    float64    `json:"rateMbps"`
	Status      string     `gorm:"size:16;index" json:"status"`
	CreatedAt   time.Time  `gorm:"index" json:"createdAt"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}

type ThroughputJob struct {
	ID         uint            `gorm:"primaryKey" json:"id"`
	RunID      uint            `gorm:"index" json:"runId"`
	SourceID   uint            `gorm:"index" json:"sourceId"`
	SourceName string          `gorm:"size:120" json:"sourceName"`
	Sequence   int             `json:"sequence"`
	Direction  string          `gorm:"size:12" json:"direction"`
	Status     string          `gorm:"size:16;index" json:"status"`
	Error      string          `gorm:"size:512" json:"error"`
	Result     json.RawMessage `gorm:"type:text" json:"result,omitempty"`
	Username   string          `gorm:"size:80" json:"-"`
	Password   string          `gorm:"size:80" json:"-"`
	PrivateKey string          `gorm:"type:text" json:"-"`
	PublicKey  string          `gorm:"type:text" json:"-"`
	CreatedAt  time.Time       `json:"createdAt"`
	StartedAt  *time.Time      `json:"startedAt,omitempty"`
	FinishedAt *time.Time      `json:"finishedAt,omitempty"`
	ExpiresAt  *time.Time      `json:"expiresAt,omitempty"`
}
