package models

import "gorm.io/gorm"

// MigrateNodeVisibilityToDelivery preserves the old opt-out intent while
// making manually hidden nodes/groups manageable in the normal node list.
// Agent relay Hidden is an internal applied-state gate, not a user preference.
func MigrateNodeVisibilityToDelivery() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Node{}).Where("hidden = ? AND (source IS NULL OR source NOT LIKE ?)", true, "agent-relay:%").
			Updates(map[string]any{"hidden": false, "delivery_disabled": true}).Error; err != nil {
			return err
		}
		grouped := tx.Table("group_node_nodes AS x").Select("x.node_id").
			Joins("JOIN group_nodes AS g ON g.id = x.group_node_id AND g.deleted_at IS NULL").Where("g.hidden = ?", true)
		if err := tx.Model(&Node{}).Where("id IN (?)", grouped).Update("delivery_disabled", true).Error; err != nil {
			return err
		}
		return tx.Model(&GroupNode{}).Where("hidden = ?", true).Update("hidden", false).Error
	})
}
