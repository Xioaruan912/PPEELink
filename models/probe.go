package models

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"gorm.io/gorm"
)

// Probe 是运行在另一个网络里的检测探针，用来补充面板侧单视角检测。
type Probe struct {
	gorm.Model
	ID         uint
	Name       string     `gorm:"size:80;not null" json:"name"`
	Region     string     `gorm:"size:32" json:"region"`
	TokenHash  string     `gorm:"size:64;index;not null" json:"-"`
	Enabled    bool       `gorm:"not null;default:true" json:"enabled"`
	LastSeenAt *time.Time `json:"lastSeenAt"`
	Version    string     `gorm:"size:32" json:"version"`
}

// ProbeResult 探针一次 TCP 检测结果。
type ProbeResult struct {
	gorm.Model
	ProbeID   uint      `gorm:"index:idx_probe_node_time,priority:1;not null" json:"probeId"`
	NodeID    int       `gorm:"index:idx_probe_node_time,priority:2;not null" json:"nodeId"`
	Kind      string    `gorm:"size:16;not null" json:"kind"`
	Success   bool      `gorm:"not null" json:"success"`
	RTT       int       `json:"rtt"`
	Error     string    `gorm:"size:255" json:"error"`
	IPVersion int       `json:"ipVersion"`
	CheckedAt time.Time `gorm:"index;not null" json:"checkedAt"`
}

// HashProbeToken 用独立前缀哈希探针令牌，避免和 agent / API Token 混用。
func HashProbeToken(plain string) string {
	sum := sha256.Sum256([]byte("probe:" + plain))
	return hex.EncodeToString(sum[:])
}

func GetProbes(onlyEnabled bool) ([]Probe, error) {
	var probes []Probe
	q := DB.Order("id asc")
	if onlyEnabled {
		q = q.Where("enabled = ?", true)
	}
	return probes, q.Find(&probes).Error
}

// RecordProbeResults 保存一批检测结果，并刷新探针在线时间。
func RecordProbeResults(probeID uint, results []ProbeResult, now time.Time) error {
	if len(results) > 0 {
		if err := DB.CreateInBatches(results, 100).Error; err != nil {
			return err
		}
	}
	return DB.Model(&Probe{}).Where("id = ?", probeID).
		Updates(map[string]any{"last_seen_at": now}).Error
}

func CleanupProbeResults(before time.Time) error {
	return DB.Where("checked_at < ?", before).Delete(&ProbeResult{}).Error
}
