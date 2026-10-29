package models

import (
	"encoding/json"

	"gorm.io/gorm"
)

// RemoveClientProxyChains retires the old per-node client policy without
// changing links, identities, membership or the remaining connection flags.
func RemoveClientProxyChains() error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var nodes []Node
		if err := tx.Select("id", "flags").Where("flags LIKE ?", "%dialerProxyId%").Find(&nodes).Error; err != nil {
			return err
		}
		for _, n := range nodes {
			var flags map[string]json.RawMessage
			if json.Unmarshal([]byte(n.Flags), &flags) != nil {
				continue
			}
			if _, ok := flags["dialerProxyId"]; !ok {
				continue
			}
			delete(flags, "dialerProxyId")
			raw := ""
			if len(flags) != 0 {
				encoded, err := json.Marshal(flags)
				if err != nil {
					return err
				}
				raw = string(encoded)
			}
			if err := tx.Model(&Node{}).Where("id = ?", n.ID).Update("flags", raw).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
