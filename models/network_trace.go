package models

import "time"

// NetworkTraceRun groups bounded route traces from independent network sources.
type NetworkTraceRun struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Protocol  string    `gorm:"size:8" json:"protocol"`
	Port      int       `json:"port"`
	Family    string    `gorm:"size:8" json:"family"`
	Target    string    `gorm:"size:253;not null" json:"target"`
	CreatedAt time.Time `gorm:"index" json:"createdAt"`
}

type NetworkTraceView struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	RunID         uint       `gorm:"index;not null" json:"runId"`
	ProbeID       uint       `gorm:"index" json:"probeId"`
	AgentServerID uint       `gorm:"index" json:"agentServerId"`
	SourceName    string     `gorm:"size:120" json:"sourceName"`
	Status        string     `gorm:"size:24;index" json:"status"`
	Tool          string     `gorm:"size:24" json:"tool"`
	Output        string     `gorm:"type:text" json:"output"`
	Error         string     `gorm:"size:512" json:"error"`
	CreatedAt     time.Time  `gorm:"index" json:"createdAt"`
	StartedAt     *time.Time `json:"startedAt"`
	FinishedAt    *time.Time `json:"finishedAt"`
}

type TraceProbeCapability struct {
	ProbeID uint      `gorm:"primaryKey" json:"probeId"`
	SeenAt  time.Time `json:"seenAt"`
	Tool    string    `gorm:"size:24" json:"tool"`
	Version int       `json:"version"`
}

type TraceAgentCapability struct {
	PrepareID          string     `gorm:"size:32" json:"-"`
	PrepareRequestedAt *time.Time `json:"prepareRequestedAt"`
	PrepareStartedAt   *time.Time `json:"prepareStartedAt"`
	PrepareFinishedAt  *time.Time `json:"prepareFinishedAt"`
	PrepareError       string     `gorm:"size:512" json:"prepareError"`
	ServerID           uint       `gorm:"primaryKey" json:"serverId"`
	SeenAt             time.Time  `json:"seenAt"`
	Tool               string     `gorm:"size:24" json:"tool"`
	Version            int        `json:"version"`
}
