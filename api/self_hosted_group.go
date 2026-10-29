package api

import (
	"gorm.io/gorm"
	"ppeelink/models"
)

const (
	selfHostedGroupName   = "自建"
	selfHostedGroupPrefix = "自建·"
	selfHostedSource      = "self-hosted"
)

func ensureSelfHostedGroup(tx *gorm.DB) (*models.GroupNode, error) {
	var group models.GroupNode
	err := tx.Where("name = ?", selfHostedGroupName).First(&group).Error
	if err == gorm.ErrRecordNotFound {
		group = models.GroupNode{Name: selfHostedGroupName}
		if err := tx.Create(&group).Error; err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}
	if err := tx.Preload("Nodes").First(&group, group.ID).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

// migrateSelfHostedGroups 把旧的「自建·服务器名」分组并入单一「自建」分组。
// 它同时迁移订阅分组引用和直连节点引用，避免升级后已有订阅丢失节点。
func migrateSelfHostedGroups() error {
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var legacy []models.GroupNode
		if err := tx.Preload("Nodes").Where("name LIKE ?", selfHostedGroupPrefix+"%").Find(&legacy).Error; err != nil {
			return err
		}
		if len(legacy) == 0 {
			return nil
		}
		group, err := ensureSelfHostedGroup(tx)
		if err != nil {
			return err
		}
		byName := map[string]models.Node{}
		for _, n := range group.Nodes {
			byName[n.Name] = n
		}
		for i := range legacy {
			old := &legacy[i]
			for _, n := range old.Nodes {
				if n.Source == "" {
					_ = tx.Model(&models.Node{}).Where("id = ?", n.ID).Update("source", selfHostedSource).Error
				}
				if keep, ok := byName[n.Name]; ok && keep.ID != n.ID {
					if err := mergeManagedNode(tx, keep.ID, n.ID); err != nil {
						return err
					}
					continue
				}
				if err := tx.Model(group).Association("Nodes").Append(&n); err != nil {
					return err
				}
				byName[n.Name] = n
			}
			if err := tx.Exec(`INSERT OR IGNORE INTO subcription_groups (subcription_id, group_node_id)
				SELECT subcription_id, ? FROM subcription_groups WHERE group_node_id = ?`, group.ID, old.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM subcription_groups WHERE group_node_id = ?", old.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM group_node_nodes WHERE group_node_id = ?", old.ID).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&models.GroupNode{}, old.ID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// mergeManagedNode 把重复的自动节点合并到保留节点：迁移订阅直连引用，
// 保留分组归属。
func mergeManagedNode(tx *gorm.DB, keepID, duplicateID int) error {
	if keepID <= 0 || duplicateID <= 0 || keepID == duplicateID {
		return nil
	}
	var groups []models.GroupNode
	if err := tx.Table("group_nodes").
		Select("group_nodes.*").
		Joins("JOIN group_node_nodes ON group_node_nodes.group_node_id = group_nodes.id").
		Where("group_node_nodes.node_id = ? AND group_nodes.deleted_at IS NULL", duplicateID).
		Find(&groups).Error; err != nil {
		return err
	}
	for _, g := range groups {
		if err := tx.Exec("INSERT OR IGNORE INTO group_node_nodes (group_node_id, node_id) VALUES (?, ?)", g.ID, keepID).Error; err != nil {
			return err
		}
	}
	if err := tx.Exec(`INSERT OR IGNORE INTO subcription_nodes (subcription_id, node_id)
		SELECT subcription_id, ? FROM subcription_nodes WHERE node_id = ?`, keepID, duplicateID).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM subcription_nodes WHERE node_id = ?", duplicateID).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM group_node_nodes WHERE node_id = ?", duplicateID).Error; err != nil {
		return err
	}
	return tx.Unscoped().Delete(&models.Node{}, duplicateID).Error
}

// generatedNodeNames 返回所有服务器入站会生成的节点名，用于识别和清理旧自动节点。
func generatedNodeNames(tx *gorm.DB) (map[string]bool, error) {
	var servers []models.ProxyServer
	var inbounds []models.ProxyInbound
	if err := tx.Find(&servers).Error; err != nil {
		return nil, err
	}
	if err := tx.Find(&inbounds).Error; err != nil {
		return nil, err
	}
	byServer := map[uint]models.ProxyServer{}
	for _, s := range servers {
		byServer[s.ID] = s
	}
	names := map[string]bool{}
	for _, in := range inbounds {
		if s, ok := byServer[in.ServerID]; ok {
			names[inboundName(s, in)] = true
		}
	}
	return names, nil
}

func backfillManagedSources(tx *gorm.DB) error {
	names, err := generatedNodeNames(tx)
	if err != nil {
		return err
	}
	for name := range names {
		if err := tx.Model(&models.Node{}).Where("name = ? AND (source IS NULL OR source = '')", name).
			Update("source", selfHostedSource).Error; err != nil {
			return err
		}
	}
	return nil
}
