package api

import (
	"strings"

	"gorm.io/gorm"
	"ppeelink/models"
)

// MergeDuplicateMonitors 合并“同一台机器既有独立监控、又由自建节点 agent 自动创建监控”的重复记录。
// 以已有独立监控为主：保留它的 SSH 配置和历史，把 agent 监控的指标迁入后删除重复项。
func MergeDuplicateMonitors() error {
	var linked []models.Monitor
	if err := models.DB.Where("proxy_server_id IS NOT NULL").Find(&linked).Error; err != nil {
		return err
	}
	for _, duplicate := range linked {
		if duplicate.ProxyServerID == nil {
			continue
		}
		var server models.ProxyServer
		if err := models.DB.First(&server, *duplicate.ProxyServerID).Error; err != nil || strings.TrimSpace(server.PublicHost) == "" {
			continue
		}
		var canonical models.Monitor
		err := models.DB.Where("proxy_server_id IS NULL AND ssh_host <> '' AND LOWER(ssh_host) = ?", strings.ToLower(strings.TrimSpace(server.PublicHost))).
			Order("id asc").First(&canonical).Error
		if err == gorm.ErrRecordNotFound {
			continue
		}
		if err != nil || canonical.ID == duplicate.ID {
			continue
		}
		if err := models.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&models.MonitorMetric{}).Where("monitor_id = ?", duplicate.ID).
				Update("monitor_id", canonical.ID).Error; err != nil {
				return err
			}
			if err := tx.Delete(&models.Monitor{}, duplicate.ID).Error; err != nil {
				return err
			}
			updates := map[string]any{"proxy_server_id": server.ID, "enabled": true}
			if canonical.SSHHost == "" {
				updates["ssh_host"] = server.PublicHost
			}
			return tx.Model(&models.Monitor{}).Where("id = ?", canonical.ID).Updates(updates).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
