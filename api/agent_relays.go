package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/relaycore"
	"strconv"
	"strings"
	"sync"
	"time"
)

var relayMutationMu sync.Mutex
var relaySyncMu sync.Mutex

func relayTables() bool { return models.DB.Migrator().HasTable(&models.AgentRelay{}) }

type relaySaveRequest struct {
	ID           uint   `json:"id"`
	ServerID     uint   `json:"serverId"`
	Name         string `json:"name"`
	ListenPort   int    `json:"listenPort"`
	Network      string `json:"network"`
	TargetNodeID int    `json:"targetNodeId"`
	Enabled      bool   `json:"enabled"`
}
type relayEndpoint struct {
	host              string
	port              int
	network, protocol string
}

func relayPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001::/23", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}
func relayEndpointFor(n models.Node) (relayEndpoint, error) {
	var out relayEndpoint
	if strings.HasPrefix(n.Source, "agent-relay:") {
		return out, fmt.Errorf("请选择落地原始节点，不能把另一个端口转发节点作为目标")
	}
	u, e := url.Parse(n.Link)
	if e != nil || u.User == nil {
		return out, fmt.Errorf("节点链接格式无效")
	}
	out.host = u.Hostname()
	out.port, _ = strconv.Atoi(u.Port())
	out.network = "tcp"
	out.protocol = "Reality"
	switch strings.ToLower(u.Scheme) {
	case "vless":
		if u.Query().Get("security") != "reality" {
			return out, fmt.Errorf("端口转发支持 Reality 和 HY2 节点")
		}
	case "hy2", "hysteria2":
		out.network = "udp"
		out.protocol = "HY2"
	default:
		return out, fmt.Errorf("端口转发支持 Reality 和 HY2 节点")
	}
	if !validHost(out.host) || out.port < 1 || out.port > 65535 {
		return out, fmt.Errorf("落地地址或端口无效")
	}
	if ip, e := netip.ParseAddr(out.host); e == nil && !relayPublicIP(ip) {
		return out, fmt.Errorf("落地必须为公网地址")
	}
	return out, nil
}
func validateRelayDNS(ctx context.Context, host string) error {
	if ip, e := netip.ParseAddr(host); e == nil {
		if !relayPublicIP(ip) {
			return fmt.Errorf("落地必须为公网地址")
		}
		return nil
	}
	c, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	ips, e := net.DefaultResolver.LookupNetIP(c, "ip", host)
	if e != nil || len(ips) == 0 {
		return fmt.Errorf("落地服务器域名无法解析")
	}
	for _, ip := range ips {
		if !relayPublicIP(ip) {
			return fmt.Errorf("落地域名包含内网或保留地址")
		}
	}
	return nil
}
func relayRuleSpec(r models.AgentRelay) (relaycore.Spec, error) {
	var n models.Node
	if models.DB.First(&n, r.TargetNodeID).Error != nil {
		return relaycore.Spec{}, fmt.Errorf("落地节点已删除")
	}
	if n.PausedUntil != nil && n.PausedUntil.After(time.Now()) {
		return relaycore.Spec{}, fmt.Errorf("落地节点已暂停下发")
	}
	if n.Flags != "" {
		var f node.NodeFlags
		if json.Unmarshal([]byte(n.Flags), &f) != nil {
			return relaycore.Spec{}, fmt.Errorf("落地节点连接设置无效")
		}
	}
	e, err := relayEndpointFor(n)
	if err != nil {
		return relaycore.Spec{}, err
	}
	if r.Network != e.network && r.Network != "both" {
		return relaycore.Spec{}, fmt.Errorf("所选节点需要 %s 转发", strings.ToUpper(e.network))
	}
	return relaycore.Spec{ID: strconv.Itoa(int(r.ID)), ListenPort: r.ListenPort, Network: r.Network, TargetHost: e.host, TargetPort: e.port}, nil
}
func AgentRelayTargets(c *gin.Context) {
	var ns []models.Node
	if e := models.DB.Order("name asc").Find(&ns).Error; e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取落地节点失败"})
		return
	}
	out := []gin.H{}
	for _, n := range ns {
		if p, e := relayEndpointFor(n); e == nil {
			out = append(out, gin.H{"id": n.ID, "name": n.Name, "protocol": p.protocol, "network": p.network})
		}
	}
	c.JSON(200, gin.H{"code": "00000", "data": out})
}
func AgentRelayList(c *gin.Context) {
	if e := SyncAgentRelayNodes(); e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "同步中转节点失败"})
		return
	}
	rows := []models.AgentRelay{}
	if e := models.DB.Order("id asc").Find(&rows).Error; e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取中转规则失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": rows})
}
func AgentRelaySave(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8192)
	var req relaySaveRequest
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "中转参数无效"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len([]rune(req.Name)) > 80 || strings.ContainsAny(req.Name, "\r\n") {
		c.JSON(400, gin.H{"code": "40000", "msg": "填写 1–80 字的线路名称"})
		return
	}
	if req.ListenPort < 1024 || req.ListenPort > 65535 || req.ListenPort == xrayAPIPort || (req.Network != "tcp" && req.Network != "udp" && req.Network != "both") {
		c.JSON(400, gin.H{"code": "40000", "msg": "监听端口或转发类型无效"})
		return
	}
	relayMutationMu.Lock()
	defer relayMutationMu.Unlock()
	var server models.ProxyServer
	if models.DB.First(&server, req.ServerID).Error != nil || server.Removing {
		c.JSON(400, gin.H{"code": "40000", "msg": "入口服务器不存在"})
		return
	}
	if server.Kind != models.ServerKindAgent || server.ProxyFeatures < 5 {
		c.JSON(400, gin.H{"code": "40000", "msg": "请先升级入口 Agent 以支持端口转发"})
		return
	}
	rr := models.AgentRelay{}
	if req.ID != 0 {
		if models.DB.First(&rr, req.ID).Error != nil {
			c.JSON(404, gin.H{"code": "40400", "msg": "线路不存在"})
			return
		}
		if rr.ServerID != req.ServerID {
			c.JSON(400, gin.H{"code": "40000", "msg": "已有线路不能更换入口服务器，请新建线路"})
			return
		}
	}
	// Stopping an existing listener must not depend on upstream DNS or a
	// node that has since been deleted. Only its stored identity is needed.
	if req.ID != 0 && !req.Enabled {
		err := models.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&rr).Update("enabled", false).Error; e != nil {
				return e
			}
			if rr.NodeID > 0 {
				return tx.Model(&models.Node{}).Where("id = ?", rr.NodeID).Update("hidden", true).Error
			}
			return nil
		})
		if err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "停用线路失败"})
			return
		}
		rr.Enabled = false
		InvalidateOverview()
		c.JSON(200, gin.H{"code": "00000", "data": rr, "msg": "已停用，等待 Agent 停止监听"})
		return
	}
	var count int64
	models.DB.Model(&models.AgentRelay{}).Where("server_id = ? AND id <> ?", req.ServerID, req.ID).Count(&count)
	if count >= 64 {
		c.JSON(400, gin.H{"code": "40000", "msg": "每个 Agent 最多 64 条转发线路"})
		return
	}
	if err := relayPortConflict(req.ServerID, req.ListenPort, req.ID, 0); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": err.Error()})
		return
	}
	var names int64
	models.DB.Model(&models.AgentRelay{}).Where("server_id = ? AND name = ? AND id <> ?", req.ServerID, req.Name, req.ID).Count(&names)
	if names > 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "此入口已有同名线路，请使用不同名称"})
		return
	}
	if !validHost(server.PublicHost) {
		c.JSON(400, gin.H{"code": "40000", "msg": "请先填写入口服务器的公网连接地址"})
		return
	}
	rr.ServerID, rr.Name, rr.ListenPort, rr.Network, rr.TargetNodeID, rr.Enabled = req.ServerID, req.Name, req.ListenPort, req.Network, req.TargetNodeID, req.Enabled
	spec, e := relayRuleSpec(rr)
	if e == nil && strings.EqualFold(strings.Trim(server.PublicHost, "[]"), strings.Trim(spec.TargetHost, "[]")) {
		e = fmt.Errorf("请选择其他服务器的落地节点")
	}
	if e == nil {
		e = validateRelayDNS(c.Request.Context(), spec.TargetHost)
	}
	if e == nil {
		e = validateManagedForwardGraph(&rr, nil)
	}
	if e != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": e.Error()})
		return
	}
	if e = models.DB.Transaction(func(tx *gorm.DB) error { return tx.Save(&rr).Error }); e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存中转失败"})
		return
	}
	if e = SyncAgentRelayNodes(); e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "规则已保存，生成节点失败，请刷新后重试"})
		return
	}
	models.DB.First(&rr, rr.ID)
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "data": rr, "msg": "已保存，等待 Agent 应用"})
}
func AgentRelayDelete(c *gin.Context) {
	id, e := strconv.Atoi(c.Query("id"))
	if e != nil || id < 1 {
		c.JSON(400, gin.H{"code": "40000", "msg": "线路编号无效"})
		return
	}
	relayMutationMu.Lock()
	defer relayMutationMu.Unlock()
	relaySyncMu.Lock()
	defer relaySyncMu.Unlock()
	var rr models.AgentRelay
	if models.DB.First(&rr, id).Error != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "线路不存在"})
		return
	}
	e = models.DB.Transaction(func(tx *gorm.DB) error {
		return deleteAgentRelay(tx, rr)
	})
	if e != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除线路失败"})
		return
	}
	InvalidateOverview()
	c.JSON(200, gin.H{"code": "00000", "msg": "已删除，Agent 将停止对应监听"})
}

// The caller serializes mutations and node synchronization. Keep historical
// line traffic, but remove generated nodes and their subscription memberships.
func deleteAgentRelay(tx *gorm.DB, rr models.AgentRelay) error {
	if rr.NodeID != 0 {
		if err := tx.Exec("DELETE FROM group_node_nodes WHERE node_id = ?", rr.NodeID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM subcription_nodes WHERE node_id = ?", rr.NodeID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.Node{}, rr.NodeID).Error; err != nil {
			return err
		}
	}
	return tx.Delete(&rr).Error
}

func deleteServerAgentRelays(serverID uint) error {
	relayMutationMu.Lock()
	defer relayMutationMu.Unlock()
	relaySyncMu.Lock()
	defer relaySyncMu.Unlock()
	if !relayTables() {
		return nil
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var rows []models.AgentRelay
		if err := tx.Where("server_id = ?", serverID).Find(&rows).Error; err != nil {
			return err
		}
		for _, rr := range rows {
			if err := deleteAgentRelay(tx, rr); err != nil {
				return err
			}
		}
		return nil
	})
}
func relayPortConflict(serverID uint, port int, ruleID, inboundID uint) error {
	var cnt int64
	if e := models.DB.Model(&models.ProxyInbound{}).Where("server_id = ? AND port = ? AND id <> ?", serverID, port, inboundID).Count(&cnt).Error; e != nil {
		return e
	}
	if cnt > 0 {
		return fmt.Errorf("该端口已被本服务器入站占用")
	}
	if relayTables() {
		if e := models.DB.Model(&models.AgentRelay{}).Where("server_id = ? AND listen_port = ? AND id <> ?", serverID, port, ruleID).Count(&cnt).Error; e != nil {
			return e
		}
		if cnt > 0 {
			return fmt.Errorf("该端口已被中转线路占用")
		}
	}
	return nil
}
func buildAgentRelaySpecs(server models.ProxyServer) ([]relaycore.Spec, error) {
	out := []relaycore.Spec{}
	if !relayTables() {
		return out, nil
	}
	var rows []models.AgentRelay
	if e := models.DB.Where("server_id = ? AND enabled = ?", server.ID, true).Find(&rows).Error; e != nil {
		return nil, e
	}
	if len(rows) > 0 && server.ProxyFeatures < 5 {
		return nil, fmt.Errorf("请先升级入口 Agent 以支持端口转发")
	}
	for _, r := range rows {
		spec, e := relayRuleSpec(r)
		if e == nil {
			e = validateManagedForwardGraph(&r, nil)
		}
		if e != nil {
			models.DB.Model(&r).Updates(map[string]any{"last_error": e.Error()})
			continue
		}
		out = append(out, spec)
	}
	return out, nil
}

// SyncAgentRelayNodes keeps stable node IDs for subscriptions; no upstream secrets are changed.
func SyncAgentRelayNodes() error {
	relaySyncMu.Lock()
	defer relaySyncMu.Unlock()
	changed := false
	defer func() {
		if changed {
			InvalidateOverview()
		}
	}()
	if !relayTables() {
		return nil
	}
	var rows []models.AgentRelay
	if e := models.DB.Find(&rows).Error; e != nil {
		return e
	}
	for _, r := range rows {
		var s models.ProxyServer
		var n models.Node
		serverErr := models.DB.First(&s, r.ServerID).Error
		targetErr := models.DB.First(&n, r.TargetNodeID).Error
		source := fmt.Sprintf("agent-relay:%d", r.ID)
		var generated models.Node
		exists := r.NodeID > 0 && models.DB.First(&generated, r.NodeID).Error == nil
		if !exists {
			models.DB.Where("source = ?", source).First(&generated)
			exists = generated.ID != 0
		}
		if serverErr != nil || targetErr != nil {
			if exists && !generated.Hidden {
				if err := models.DB.Model(&generated).Update("hidden", true).Error; err != nil {
					return err
				}
				changed = true
			}
			continue
		}
		spec, e := relayRuleSpec(r)
		if e == nil {
			e = validateManagedForwardGraph(&r, nil)
		}
		if e != nil {
			models.DB.Model(&r).Update("last_error", e.Error())
			if exists && !generated.Hidden {
				if err := models.DB.Model(&generated).Update("hidden", true).Error; err != nil {
					return err
				}
				changed = true
			}
			continue
		}
		rev := spec.Rev()
		link, e := relayEntryLink(n, s.PublicHost, r.ListenPort)
		if e != nil {
			return e
		}
		name := r.Name
		parsedLink, _ := url.Parse(link)
		parsedLink.Fragment = name
		link = parsedLink.String()
		var f node.NodeFlags
		if n.Flags != "" {
			if json.Unmarshal([]byte(n.Flags), &f) != nil {
				return fmt.Errorf("落地节点连接参数无效")
			}
		}
		flags := encodeNodeFlags(f)
		hidden := !r.Enabled || !s.Enabled || s.Removing || !r.Running || r.AppliedRevision != rev || (s.TrafficLimit > 0 && serverQuota(s, time.Now()).Over)
		e = models.DB.Transaction(func(tx *gorm.DB) error {
			var group models.GroupNode
			if err := tx.Where("name = ?", "Agent 中转").FirstOrCreate(&group, models.GroupNode{Name: "Agent 中转"}).Error; err != nil {
				return err
			}
			if exists {
				old := generated.Name
				changes := map[string]any{}
				if generated.Name != name {
					changes["name"] = name
				}
				if generated.Link != link {
					changes["link"] = link
				}
				if generated.Flags != flags {
					changes["flags"] = flags
				}
				if generated.Source != source {
					changes["source"] = source
				}
				if generated.Hidden != hidden {
					changes["hidden"] = hidden
				}
				if len(changes) > 0 {
					if err := tx.Model(&generated).Updates(changes).Error; err != nil {
						return err
					}
					changed = true
				}
				if old != name {
					if err := renameInNodeOrders(tx, old, name); err != nil {
						return err
					}
				}
			} else {
				generated = models.Node{Name: name, Link: link, Flags: flags, Source: source, ProbePolicy: "any", Hidden: hidden}
				if err := tx.Create(&generated).Error; err != nil {
					return err
				}
				changed = true
			}
			var membership int64
			if err := tx.Table("group_node_nodes").Where("group_node_id = ? AND node_id = ?", group.ID, generated.ID).Count(&membership).Error; err != nil {
				return err
			}
			if membership == 0 {
				if err := tx.Model(&group).Association("Nodes").Append(&generated); err != nil {
					return err
				}
				changed = true
			}
			if r.NodeID != generated.ID || r.Revision != rev {
				return tx.Model(&r).Updates(map[string]any{"node_id": generated.ID, "revision": rev}).Error
			}
			return nil
		})
		if e != nil {
			return e
		}
	}
	return nil
}
func relayEntryLink(n models.Node, host string, port int) (string, error) {
	u, e := url.Parse(n.Link)
	if e != nil {
		return "", fmt.Errorf("节点链接无效")
	}
	oldHost := u.Hostname()
	if u.Scheme == "hy2" || u.Scheme == "hysteria2" {
		q := u.Query()
		if q.Get("sni") == "" {
			q.Set("sni", oldHost)
			u.RawQuery = q.Encode()
		}
	}
	u.Host = net.JoinHostPort(strings.Trim(host, "[]"), strconv.Itoa(port))
	return u.String(), nil
}
func applyAgentRelayReport(tx *gorm.DB, serverID uint, status map[string]relaycore.Status, now time.Time) error {
	if len(status) == 0 || !tx.Migrator().HasTable(&models.AgentRelay{}) {
		return nil
	}
	for key, st := range status {
		ident, e := strconv.Atoi(key)
		if e != nil || ident <= 0 || len(st.Session) > 128 || st.Up < 0 || st.Down < 0 {
			continue
		}
		var r models.AgentRelay
		if tx.Where("id = ? AND server_id = ?", ident, serverID).First(&r).Error != nil {
			continue
		}
		up, down := st.Up, st.Down
		if r.AgentSession == st.Session {
			up = max(0, up-r.LastUp)
			down = max(0, down-r.LastDown)
		}
		lastUp, lastDown := st.Up, st.Down
		if r.AgentSession == st.Session {
			lastUp = max(lastUp, r.LastUp)
			lastDown = max(lastDown, r.LastDown)
		}
		updates := map[string]any{"agent_session": st.Session, "last_up": lastUp, "last_down": lastDown, "up": r.Up + up, "down": r.Down + down, "running": st.Running, "applied_revision": st.Revision, "last_seen_at": now, "tcp_connections": st.TCPConnections, "udp_sessions": st.UDPSessions}
		if st.Error != "" {
			updates["last_error"] = st.Error
		} else if (st.Running && st.Revision == r.Revision) || !r.Enabled {
			updates["last_error"] = ""
		}
		if e := tx.Model(&r).Updates(updates).Error; e != nil {
			return e
		}
		if (up > 0 || down > 0) && tx.Migrator().HasTable(&models.AgentRelayDay{}) {
			d := models.AgentRelayDay{RuleID: r.ID, ServerID: serverID, Day: now.Format("2006-01-02"), Up: up, Down: down}
			if e := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "rule_id"}, {Name: "day"}}, DoUpdates: clause.Assignments(map[string]any{"up": gorm.Expr("up + ?", up), "down": gorm.Expr("down + ?", down)})}).Create(&d).Error; e != nil {
				return e
			}
		}
	}
	return nil
}
