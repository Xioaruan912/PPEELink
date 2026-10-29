package models

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestVisibilityMigrationKeepsOptOutButRestoresManagementVisibility(t *testing.T) {
	old := DB
	t.Cleanup(func() { DB = old })
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "migration.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	DB = db
	if err := DB.AutoMigrate(&Node{}, &GroupNode{}); err != nil {
		t.Fatal(err)
	}
	direct := Node{Name: "hidden", Hidden: true}
	member := Node{Name: "group member"}
	runtime := Node{Name: "pending relay", Hidden: true, Source: "agent-relay:3"}
	for _, n := range []*Node{&direct, &member, &runtime} {
		if err := DB.Create(n).Error; err != nil {
			t.Fatal(err)
		}
	}
	group := GroupNode{Name: "hidden group", Hidden: true, Nodes: []Node{member}}
	if err := DB.Create(&group).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := MigrateNodeVisibilityToDelivery(); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int{direct.ID, member.ID} {
		var n Node
		DB.First(&n, id)
		if n.Hidden || !n.DeliveryDisabled {
			t.Fatal("old hidden node not converted to visible opt-out", n.Name)
		}
	}
	DB.First(&runtime, runtime.ID)
	if !runtime.Hidden || runtime.DeliveryDisabled {
		t.Fatal("runtime readiness mistaken for a user opt-out")
	}
	DB.First(&group, group.ID)
	if group.Hidden {
		t.Fatal("group still hidden")
	}
	DB.Model(&direct).Update("delivery_disabled", false)
	if err := MigrateNodeVisibilityToDelivery(); err != nil {
		t.Fatal(err)
	}
	DB.First(&direct, direct.ID)
	if direct.DeliveryDisabled {
		t.Fatal("migration overwrote a later user re-enable")
	}
}
