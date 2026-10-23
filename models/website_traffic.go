package models

import "time"

// WebsiteTraffic records successful proxy payload bytes, never interface counters.
// Buckets are UTC hours; server ID is the self-hosted proxy node, not the user.
type WebsiteTraffic struct {
	ID          uint      `gorm:"primaryKey" json:"-"`
	ServerID    uint      `gorm:"uniqueIndex:idx_website_bucket,priority:1;index" json:"serverId"`
	BucketStart time.Time `gorm:"uniqueIndex:idx_website_bucket,priority:2;index" json:"bucketStart"`
	Domain      string    `gorm:"size:253;uniqueIndex:idx_website_bucket,priority:3" json:"domain"`
	Up          int64     `json:"up"`
	Down        int64     `json:"down"`
}

// WebsiteTrafficReceipt makes a retry after a lost HTTP response idempotent.
// Receipts intentionally survive disabling collection (late queued reports remain valid).
type WebsiteTrafficReceipt struct {
	ID        uint   `gorm:"primaryKey"`
	ServerID  uint   `gorm:"uniqueIndex:idx_website_receipt,priority:1"`
	BatchID   string `gorm:"size:64;uniqueIndex:idx_website_receipt,priority:2"`
	CreatedAt time.Time
}
