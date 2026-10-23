package api

import (
	"fmt"
	"gorm.io/gorm"
	"ppeelink/models"
)

// resetServerAgentToken revokes both old credential roles atomically.
func resetServerAgentToken(id uint, hash string) error {
	return models.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.ProxyServer{}).Where("id = ? AND kind = ?", id, models.ServerKindAgent).Update("agent_token", hash)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&models.Monitor{}).Where("proxy_server_id = ?", id).Update("token_hash", "").Error
	})
}

func resetMonitorAgentToken(monitor models.Monitor, hash string) error {
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&monitor).Update("token_hash", hash).Error; err != nil {
			return err
		}
		if monitor.ProxyServerID != nil {
			return tx.Model(&models.ProxyServer{}).Where("id = ? AND kind = ?", *monitor.ProxyServerID, models.ServerKindAgent).Update("agent_token", "").Error
		}
		return nil
	})
}

// monitorAgent preserves the original monitor credential without giving it
// access to any server other than its persisted binding. Enabled controls
// metrics, never management authentication or proxy process lifetime.
func monitorAgent(token string) (*models.ProxyServer, error) {
	var server models.ProxyServer
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var monitor models.Monitor
		if err := tx.Where("token_hash = ?", models.HashMonitorToken(token)).First(&monitor).Error; err != nil {
			return err
		}
		if monitor.ProxyServerID != nil {
			return tx.Where("id = ? AND kind = ?", *monitor.ProxyServerID, models.ServerKindAgent).First(&server).Error
		}
		server = models.ProxyServer{Name: monitor.Name, Kind: models.ServerKindAgent, MonitorEnabled: monitor.Enabled, AgentUpdateRequested: monitor.AgentUpdateRequested}
		// The credential remains on Monitor, never duplicate it into AgentToken.
		if err := tx.Create(&server).Error; err != nil {
			return err
		}
		if err := tx.Model(&server).Update("enabled", false).Error; err != nil {
			return err
		}
		server.Enabled = false
		result := tx.Model(&models.Monitor{}).Where("id = ? AND proxy_server_id IS NULL", monitor.ID).Update("proxy_server_id", server.ID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("monitor binding changed; retry")
		}
		return nil
	})
	return &server, err
}
