package models

import "time"

// WebDAVBackupSetting 保存 WebDAV 云端自动备份配置（支持坚果云、Alist、Nextcloud、群晖等）。
type WebDAVBackupSetting struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Enabled      bool       `json:"enabled"`
	Preset       string     `gorm:"size:32;default:'jianguoyun'" json:"preset"`
	URL          string     `gorm:"size:512" json:"url"`
	Username     string     `gorm:"size:256" json:"username"`
	Password     string     `gorm:"size:512" json:"-"`
	RemoteDir    string     `gorm:"size:256;default:'ppeelink-backups'" json:"remoteDir"`
	Schedule     string     `gorm:"size:32;default:'daily'" json:"schedule"`
	KeepCount    int        `gorm:"default:7" json:"keepCount"`
	LastStatus   string     `gorm:"size:32" json:"lastStatus"`
	LastError    string     `gorm:"size:512" json:"lastError"`
	LastFile     string     `gorm:"size:256" json:"lastFile"`
	LastBackupAt *time.Time `json:"lastBackupAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

func (WebDAVBackupSetting) TableName() string {
	return "webdav_backup_settings"
}

// GetOrCreateWebDAVBackupSetting 返回唯一的 WebDAV 备份配置记录。
func GetOrCreateWebDAVBackupSetting() (*WebDAVBackupSetting, error) {
	var cfg WebDAVBackupSetting
	if err := DB.First(&cfg).Error; err != nil {
		cfg = WebDAVBackupSetting{
			Enabled:   false,
			Preset:    "jianguoyun",
			URL:       "https://dav.jianguoyun.com/dav/",
			RemoteDir: "ppeelink-backups",
			Schedule:  "daily",
			KeepCount: 7,
		}
		if createErr := DB.Create(&cfg).Error; createErr != nil {
			return nil, createErr
		}
	}
	return &cfg, nil
}
