package api

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"ppeelink/models"
)

type topologyNode struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	Detail    string     `json:"detail,omitempty"`
	Route     string     `json:"route"`
	CheckedAt *time.Time `json:"checkedAt,omitempty"`
}
type topologyEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

// NetworkTopology is a read-only inventory snapshot. It never resolves links,
// generates subscription tokens, runs probes, or guesses identity by host/name.
func NetworkTopology(c *gin.Context) {
	nodes := make([]topologyNode, 0)
	edges := make([]topologyEdge, 0)
	warnings := make([]string, 0)
	failed := 0
	query := func(err error, section string) bool {
		if err != nil {
			warnings = append(warnings, section+"读取失败，相关关系可能不完整")
			failed++
			return false
		}
		return true
	}
	key := func(kind string, id any) string { return fmt.Sprintf("%s:%v", kind, id) }
	addEdge := func(source, target, relation string) { edges = append(edges, topologyEdge{source, target, relation}) }
	now := time.Now()
	var subs []models.Subcription
	if query(models.DB.Select("id", "name", "expires_at").Order("id").Find(&subs).Error, "订阅") {
		for _, s := range subs {
			status := "configured"
			if s.ExpiresAt != nil && s.ExpiresAt.Before(now) {
				status = "expired"
			}
			nodes = append(nodes, topologyNode{ID: key("subscription", s.ID), Kind: "subscription", Name: s.Name, Status: status, Route: "/subcription/subs"})
		}
	}
	var groups []models.GroupNode
	if query(models.DB.Select("id", "name", "hidden").Order("id").Find(&groups).Error, "分组") {
		for _, g := range groups {
			status := "configured"
			if g.Hidden {
				status = "hidden"
			}
			nodes = append(nodes, topologyNode{ID: key("group", g.ID), Kind: "group", Name: g.Name, Status: status, Route: "/subcription/nodes"})
		}
	}
	var ns []models.Node
	if query(models.DB.Select("id", "name", "flags", "hidden", "paused_until").Order("id").Find(&ns).Error, "节点") {
		for _, n := range ns {
			status := "unknown"
			if n.Hidden {
				status = "hidden"
			} else if n.PausedUntil != nil && n.PausedUntil.After(now) {
				status = "paused"
			}
			nodes = append(nodes, topologyNode{ID: key("node", n.ID), Kind: "node", Name: n.Name, Status: status, Detail: "配置节点；未推断服务器归属", Route: "/subcription/nodes"})
			if relay := decodeNodeFlags(n.Flags).DialerProxyID; relay != 0 {
				addEdge(key("node", n.ID), key("node", relay), "relay")
			}
		}
	}
	// Join rows preserve explicit dangling IDs for the client to disclose, while
	// deleted source records are excluded by the live-source ID set below.
	live := map[string]bool{}
	for _, n := range nodes {
		live[n.ID] = true
	}
	var fixed []struct {
		SubcriptionID int
		NodeID        int
	}
	if query(models.DB.Table("subcription_nodes").Scan(&fixed).Error, "固定节点关系") {
		for _, r := range fixed {
			if live[key("subscription", r.SubcriptionID)] {
				addEdge(key("subscription", r.SubcriptionID), key("node", r.NodeID), "fixed")
			}
		}
	}
	var refs []struct {
		SubcriptionID int
		GroupNodeID   int
	}
	if query(models.DB.Table("subcription_groups").Scan(&refs).Error, "订阅分组关系") {
		for _, r := range refs {
			if live[key("subscription", r.SubcriptionID)] {
				addEdge(key("subscription", r.SubcriptionID), key("group", r.GroupNodeID), "reference")
			}
		}
	}
	var members []struct {
		GroupNodeID int
		NodeID      int
	}
	if query(models.DB.Table("group_node_nodes").Scan(&members).Error, "分组成员关系") {
		for _, r := range members {
			if live[key("group", r.GroupNodeID)] {
				addEdge(key("group", r.GroupNodeID), key("node", r.NodeID), "member")
			}
		}
	}
	var servers []models.ProxyServer
	if query(models.DB.Select("id", "name", "enabled", "running", "last_seen_at").Order("id").Find(&servers).Error, "代理服务器") {
		for _, s := range servers {
			status := "stopped"
			if !s.Enabled {
				status = "disabled"
			} else if s.Running {
				status = "running"
			}
			nodes = append(nodes, topologyNode{ID: key("server", s.ID), Kind: "server", Name: s.Name, Status: status, Detail: "运行状态为最近上报值", Route: "/subcription/servers", CheckedAt: s.LastSeenAt})
		}
	}
	var inbounds []models.ProxyInbound
	if query(models.DB.Select("id", "server_id", "remark", "template", "port", "enabled").Order("id").Find(&inbounds).Error, "自建入站") {
		for _, in := range inbounds {
			status := "configured"
			if !in.Enabled {
				status = "disabled"
			}
			name := in.Remark
			if name == "" {
				name = fmt.Sprintf("入站 #%d · :%d", in.ID, in.Port)
			}
			nodes = append(nodes, topologyNode{ID: key("inbound", in.ID), Kind: "inbound", Name: name, Status: status, Detail: in.Template, Route: "/subcription/servers"})
			addEdge(key("inbound", in.ID), key("server", in.ServerID), "server")
		}
	}
	var monitors []models.Monitor
	if query(models.DB.Select("id", "name", "enabled", "hidden", "last_seen_at", "proxy_server_id").Order("id").Find(&monitors).Error, "服务器监控") {
		for _, m := range monitors {
			status := "offline"
			if !m.Enabled {
				status = "disabled"
			} else if monitorOnline(m, now) {
				status = "online"
			}
			nodes = append(nodes, topologyNode{ID: key("monitor", m.ID), Kind: "monitor", Name: m.Name, Status: status, Detail: "监控心跳状态", Route: "/monitor/index", CheckedAt: m.LastSeenAt})
			if m.ProxyServerID != nil {
				addEdge(key("server", *m.ProxyServerID), key("monitor", m.ID), "monitor")
			}
		}
	}
	if failed == 9 {
		c.JSON(500, gin.H{"code": "50000", "msg": "无法读取拓扑，请重试"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"nodes": nodes, "edges": edges, "warnings": warnings, "generatedAt": now}, "msg": "只读网络拓扑"})
}
