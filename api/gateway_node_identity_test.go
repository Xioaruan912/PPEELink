package api

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"ppeelink/models"

	"gorm.io/gorm"
)

func TestGatewayNodeIdentityPreservesCollidingDirectNodeAndReferences(t *testing.T) {
	server, direct, user := selfHostedFixture(t)
	if err := models.DB.AutoMigrate(&models.ProxyTraffic{}, &models.NodeHealthEvent{}, &models.AlertSetting{}); err != nil {
		t.Fatal(err)
	}
	// Sync calls generatedNodeNames inside a transaction. A naming helper that
	// queries models.DB instead of using its arguments deadlocks this pool.
	sqlDB, err := models.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	models.DB = models.DB.WithContext(ctx)
	var account models.ProxyAccount
	if err := models.DB.First(&account, "user_id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	var original models.Node
	if err := models.DB.Where("source = ?", selfHostedSource).First(&original).Error; err != nil {
		t.Fatal(err)
	}
	if original.Name != "JP Reality" || original.Link != InboundLink(server, direct, account, "JP Reality") {
		t.Fatal("fixture did not preserve the legacy direct-node identity")
	}
	var group models.GroupNode
	if err := models.DB.Where("name = ?", selfHostedGroupName).First(&group).Error; err != nil {
		t.Fatal(err)
	}
	legacy := models.Subcription{Name: "original-saved-node", Nodes: []models.Node{original}}
	follow := models.Subcription{Name: "follow-self-hosted-group", GroupRefs: []models.GroupNode{group}}
	for _, sub := range []*models.Subcription{&legacy, &follow} {
		if err := sub.Add(); err != nil {
			t.Fatal(err)
		}
	}
	// Reproduce an old unmarked direct node too: source backfill must recognize
	// its unchanged name without mistaking the gateway for the same node.
	if err := models.DB.Model(&original).Update("source", "").Error; err != nil {
		t.Fatal(err)
	}
	exit := domainTestExit(t, "1.1.1.1", 443)
	gateway := direct
	gateway.ID, gateway.Port, gateway.EgressNodeID = 12, 24444, exit.ID
	if err := models.DB.Create(&gateway).Error; err != nil {
		t.Fatal(err)
	}

	accountNodes := AccountNodes(account)
	if len(accountNodes) != 2 || accountNodes[0].Name == accountNodes[1].Name {
		t.Fatalf("same server/remark direct and gateway must both have distinct identities: %+v", accountNodes)
	}
	if accountNodes[0].Name != original.Name || accountNodes[0].Link != original.Link {
		t.Fatal("adding a gateway renamed or rewrote the existing direct node")
	}
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	var generated []models.Node
	if err := models.DB.Where("source = ?", selfHostedSource).Order("id").Find(&generated).Error; err != nil {
		t.Fatal(err)
	}
	if len(generated) != 2 || generated[0].ID != original.ID || generated[0].Name != original.Name || generated[0].Link != original.Link {
		t.Fatalf("sync dropped the gateway or replaced the original direct node: %+v", generated)
	}
	if generated[1].Name != accountNodes[1].Name || generated[1].Link != accountNodes[1].Link {
		t.Fatal("saved gateway and account subscription use different identities")
	}
	selected := models.Subcription{Name: "select-both-entries", Nodes: generated}
	if err := selected.Add(); err != nil {
		t.Fatal(err)
	}
	assertIdentityAndRefs := func() {
		t.Helper()
		var current []models.Node
		if err := models.DB.Where("source = ?", selfHostedSource).Order("id").Find(&current).Error; err != nil {
			t.Fatal(err)
		}
		if len(current) != len(generated) {
			t.Fatalf("generated node count changed: %+v", current)
		}
		for i, n := range current {
			if n.ID != generated[i].ID || n.Name != generated[i].Name || n.Link != generated[i].Link {
				t.Fatalf("entry identity changed after sync/exit switch: before=%+v after=%+v", generated[i], n)
			}
		}
		if nodes := AccountNodes(account); len(nodes) != 2 || nodes[0].Name != current[0].Name || nodes[0].Link != current[0].Link || nodes[1].Name != current[1].Name || nodes[1].Link != current[1].Link {
			t.Fatalf("account nodes disagree with saved nodes: %+v", nodes)
		}
		wantIDs := []int{generated[0].ID, generated[1].ID}
		ids := serverNodeIDs(server)
		sort.Ints(ids)
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Fatalf("server quota/delivery mapping lost an entry: %v, want %v", ids, wantIDs)
		}
		if err := models.DB.Transaction(func(tx *gorm.DB) error {
			names, err := generatedNodeNames(tx)
			if err != nil {
				return err
			}
			if len(names) != 2 || !names[current[0].Name] || !names[current[1].Name] {
				return fmt.Errorf("transactional generated names disagree with delivery: %v", names)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		for _, check := range []struct {
			sub  models.Subcription
			want []int
		}{{legacy, []int{original.ID}}, {selected, wantIDs}, {follow, wantIDs}} {
			sub := check.sub
			if _, err := mergeGroupNodesDetailed(&sub); err != nil {
				t.Fatal(err)
			}
			got := []int{}
			for _, n := range sub.Nodes {
				got = append(got, n.ID)
			}
			sort.Ints(got)
			if !reflect.DeepEqual(got, check.want) {
				t.Fatalf("subscription %q lost saved/group node references: got %v want %v", sub.Name, got, check.want)
			}
		}
		var storedLegacy models.Subcription
		if err := models.DB.Preload("Nodes").First(&storedLegacy, legacy.ID).Error; err != nil {
			t.Fatal(err)
		}
		if len(storedLegacy.Nodes) != 1 || storedLegacy.Nodes[0].ID != original.ID || storedLegacy.Nodes[0].Link != original.Link || storedLegacy.NodeOrder != legacy.NodeOrder || storedLegacy.Token != legacy.Token {
			t.Fatal("the original saved subscription or direct link was modified")
		}
	}
	assertIdentityAndRefs()
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	assertIdentityAndRefs()
	otherExit := models.Node{Name: "new-physical-exit", Link: "hy2://secret@8.8.8.8:443?sni=exit.example.com"}
	if err := models.DB.Create(&otherExit).Error; err != nil {
		t.Fatal(err)
	}
	if err := models.DB.Model(&gateway).Update("egress_node_id", otherExit.ID).Error; err != nil {
		t.Fatal(err)
	}
	if err := SyncSelfHostedNodes(); err != nil {
		t.Fatal(err)
	}
	assertIdentityAndRefs()
}

func TestGatewayNodeNamesUseUserInputAndKeepDirectNames(t *testing.T) {
	server := models.ProxyServer{Name: "JP"}
	direct := models.ProxyInbound{Remark: "Reality", Port: 443}
	if name := inboundName(server, direct); name != "JP Reality" {
		t.Fatalf("legacy direct name changed: %s", name)
	}
	direct.Remark = ""
	if name := inboundName(server, direct); name != "JP 443" {
		t.Fatalf("legacy unnamed direct node changed: %s", name)
	}
	for _, remark := range []string{"Reality", "我的网关"} {
		gateway := models.ProxyInbound{ID: 12, Remark: remark, Port: 24444, EgressNodeID: 10}
		name := inboundName(server, gateway)
		if name != remark {
			t.Fatal("gateway name must exactly match user input")
		}
		gateway.EgressNodeID = 20
		if after := inboundName(server, gateway); after != name {
			t.Fatal("exit switching changed the stable gateway name")
		}
		gateway.Port = 24445
		if other := inboundName(server, gateway); other != name {
			t.Fatal("port editing added an automatic node name")
		}
		direct.Remark, direct.Port = remark, 24444
		if name == inboundName(server, direct) {
			t.Fatal("gateway name still collides with the legacy direct name")
		}
	}
}
