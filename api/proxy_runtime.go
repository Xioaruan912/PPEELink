package api

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"

	"gorm.io/gorm"
)

// xray API 只监听回环地址，固定端口即可。
const xrayAPIPort = 62789

// deviceBlockDuration 同时在线 IP 超过上限时的临时停用时长。
const deviceBlockDuration = 10 * time.Minute

func inboundTag(in models.ProxyInbound) string { return "in-" + strconv.Itoa(int(in.ID)) }

// activeAccounts 当前应写入 xray 配置的账户（账户本身未停用、配额与有效期内）。
func activeAccounts(now time.Time) ([]models.ProxyAccount, error) {
	var accounts []models.ProxyAccount
	if err := models.DB.Where("enabled = ?", true).Find(&accounts).Error; err != nil {
		return nil, err
	}
	var disabled []int
	models.DB.Model(&models.User{}).Where("disabled = ?", true).Pluck("id", &disabled)
	off := map[int]bool{}
	for _, id := range disabled {
		off[id] = true
	}
	out := accounts[:0]
	for _, a := range accounts {
		if !off[a.UserID] && a.Active(now) {
			out = append(out, a)
		}
	}
	return out, nil
}

// BuildServerSpec 一台服务器的期望状态：启用的入站 × 有效账户。
func BuildServerSpec(server models.ProxyServer) (xraycore.Spec, error) {
	spec := xraycore.Spec{
		Version: xraycore.DefaultVersion, Enabled: server.Enabled, APIPort: xrayAPIPort,
		BlockPrivate: true, BlockBitTorrent: true,
	}
	var inbounds []models.ProxyInbound
	if err := models.DB.Where("server_id = ? AND enabled = ?", server.ID, true).Order("id asc").Find(&inbounds).Error; err != nil {
		return spec, err
	}
	accounts, err := activeAccounts(time.Now())
	if err != nil {
		return spec, err
	}
	users := make([]xraycore.UserSpec, 0, len(accounts))
	for _, a := range accounts {
		users = append(users, xraycore.UserSpec{Email: a.Email(), UUID: a.UUID})
	}
	for _, in := range inbounds {
		spec.Inbounds = append(spec.Inbounds, xraycore.InboundSpec{
			Tag: inboundTag(in), Template: in.Template, Port: in.Port, Target: in.Target, SNI: in.SNI,
			PrivateKey: in.PrivateKey, ShortID: in.ShortID, Path: in.Path, Users: users,
		})
	}
	return spec, nil
}

func userIDFromEmail(email string) (int, bool) {
	if !strings.HasPrefix(email, "u") {
		return 0, false
	}
	id, err := strconv.Atoi(email[1:])
	return id, err == nil && id > 0
}

// resetDueAccounts 到了每月清零日的账户把已用流量归零。
func resetDueAccounts(tx *gorm.DB, now time.Time) {
	var accounts []models.ProxyAccount
	tx.Where("reset_day > 0").Find(&accounts)
	for _, a := range accounts {
		day := a.ResetDay
		if day > 28 {
			day = 28
		}
		due := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, now.Location())
		if now.Before(due) {
			due = due.AddDate(0, -1, 0)
		}
		if a.LastResetAt == nil || a.LastResetAt.Before(due) {
			tx.Model(&models.ProxyAccount{}).Where("user_id = ?", a.UserID).
				Updates(map[string]any{"traffic_used": 0, "last_reset_at": now})
		}
	}
}

// ApplyServerReport 记录服务器回报：运行状态、流量增量、在线 IP，并执行设备数限制。
func ApplyServerReport(server models.ProxyServer, report xraycore.Report) error {
	now := time.Now()
	day := now.Format("2006-01-02")
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.ProxyServer{}).Where("id = ?", server.ID).Updates(map[string]any{
			"running": report.Running, "last_error": report.Error, "last_seen_at": now,
			"xray_ver": report.XrayVersion, "arch": report.Arch, "applied_rev": report.Rev,
		}).Error; err != nil {
			return err
		}
		for email, c := range report.Traffic {
			if id, ok := userIDFromEmail(email); ok {
				if err := models.AddProxyTraffic(tx, day, id, server.ID, c.Up, c.Down); err != nil {
					return err
				}
			}
		}
		var accounts []models.ProxyAccount
		tx.Where("enabled = ?", true).Find(&accounts)
		for _, a := range accounts {
			ips := report.Online[a.Email()]
			updates := map[string]any{"online_ips": ips}
			if a.DeviceLimit > 0 && ips > a.DeviceLimit {
				until := now.Add(deviceBlockDuration)
				updates["blocked_until"] = until
				updates["blocked_reason"] = fmt.Sprintf("同时在线 %d 个 IP，超过上限 %d", ips, a.DeviceLimit)
			}
			tx.Model(&models.ProxyAccount{}).Where("user_id = ?", a.UserID).Updates(updates)
		}
		resetDueAccounts(tx, now)
		return nil
	})
}

// ---- 本机服务器：面板进程内直接驱动 ------------------------------------------

type localSource struct{ serverID uint }

func (l localSource) server() (models.ProxyServer, error) {
	var s models.ProxyServer
	err := models.DB.First(&s, l.serverID).Error
	return s, err
}

func (l localSource) Spec(context.Context) (xraycore.Spec, error) {
	s, err := l.server()
	if err != nil {
		return xraycore.Spec{}, err
	}
	return BuildServerSpec(s)
}

func (l localSource) Report(_ context.Context, report xraycore.Report) error {
	s, err := l.server()
	if err != nil {
		return err
	}
	return ApplyServerReport(s, report)
}

var localProxy struct {
	mu     sync.Mutex
	cancel context.CancelFunc
	id     uint
	wake   chan struct{}
}

// StartLocalProxy 存在本机服务器时在后台运行同步循环；服务器被删除时停止。
func StartLocalProxy() {
	localProxy.mu.Lock()
	defer localProxy.mu.Unlock()
	var s models.ProxyServer
	err := models.DB.Where("kind = ?", models.ServerKindLocal).First(&s).Error
	if err != nil {
		if localProxy.cancel != nil {
			localProxy.cancel()
			localProxy.cancel = nil
		}
		return
	}
	if localProxy.cancel != nil && localProxy.id == s.ID {
		return
	}
	if localProxy.cancel != nil {
		localProxy.cancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	localProxy.cancel, localProxy.id = cancel, s.ID
	runner := xraycore.NewRunner("./xray", "ppeelink-xray")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[xray] 本机同步异常: %v", r)
			}
		}()
		xraycore.RunLoop(ctx, localSource{serverID: s.ID}, runner, 30*time.Second)
	}()
}

// NotifyProxyChanged 配置变化后的收尾：同步管理员的自建节点并确保本机循环在跑。
// 远程 agent 在下一轮（最长 60 秒）拉取时生效。
func NotifyProxyChanged() {
	StartLocalProxy()
	if err := SyncSelfHostedNodes(); err != nil {
		log.Printf("[xray] 同步自建节点失败: %v", err)
	}
}

// ---- 链接 -------------------------------------------------------------------

// InboundLink 账户在某入站上的 VLESS 分享链接。
func InboundLink(server models.ProxyServer, in models.ProxyInbound, acc models.ProxyAccount, name string) string {
	q := node.VLESSQuery{
		Security: "reality", Sni: strings.Split(in.SNI, ",")[0], Fp: "chrome",
		Sid: in.ShortID, Pbk: in.PublicKey, Encryption: "none",
	}
	switch in.Template {
	case xraycore.TemplateXHTTPReality:
		q.Type, q.Path, q.Mode = "xhttp", in.Path, "auto"
	default:
		q.Type, q.Flow = "tcp", "xtls-rprx-vision"
	}
	return node.EncodeVLESSURL(node.VLESS{Name: name, Uuid: acc.UUID, Server: server.PublicHost, Port: in.Port, Query: q})
}

func inboundName(server models.ProxyServer, in models.ProxyInbound) string {
	if in.Remark != "" {
		return server.Name + " " + in.Remark
	}
	return server.Name + " " + strconv.Itoa(in.Port)
}

// serverNodes 账户在一台服务器上的节点（启用的入站）。
func serverNodes(s models.ProxyServer, acc models.ProxyAccount) []models.Node {
	var inbounds []models.ProxyInbound
	models.DB.Where("server_id = ? AND enabled = ?", s.ID, true).Order("id asc").Find(&inbounds)
	nodes := make([]models.Node, 0, len(inbounds))
	for _, in := range inbounds {
		name := inboundName(s, in)
		nodes = append(nodes, models.Node{Name: name, Link: InboundLink(s, in, acc, name)})
	}
	return nodes
}

// AccountNodes 账户可用的全部自建节点（启用的服务器与入站）。
func AccountNodes(acc models.ProxyAccount) []models.Node {
	var servers []models.ProxyServer
	models.DB.Where("enabled = ? AND public_host <> ''", true).Order("id asc").Find(&servers)
	nodes := []models.Node{}
	for _, s := range servers {
		nodes = append(nodes, serverNodes(s, acc)...)
	}
	return nodes
}

const selfHostedGroupPrefix = "自建·"

// SyncSelfHostedNodes 用第一个开通了自建节点的管理员账户生成节点，放进“自建·服务器名”分组，
// 这样已有订阅和合集可以像引用机场一样引用自建节点。
func SyncSelfHostedNodes() error {
	var admins []models.User
	models.DB.Where("(role = ? OR role = '' OR role IS NULL) AND disabled = ?", models.RoleAdmin, false).Order("id asc").Find(&admins)
	var owner *models.ProxyAccount
	for _, u := range admins {
		var acc models.ProxyAccount
		if models.DB.Where("user_id = ? AND enabled = ?", u.ID, true).First(&acc).Error == nil {
			owner = &acc
			break
		}
	}
	var servers []models.ProxyServer
	models.DB.Find(&servers)
	return models.DB.Transaction(func(tx *gorm.DB) error {
		for _, s := range servers {
			groupName := selfHostedGroupPrefix + s.Name
			var gn models.GroupNode
			tx.Preload("Nodes").Where("name = ?", groupName).First(&gn)
			want := []models.Node{}
			if owner != nil && s.Enabled && s.PublicHost != "" {
				want = serverNodes(s, *owner)
			}
			if gn.ID == 0 {
				if len(want) == 0 {
					continue
				}
				gn = models.GroupNode{Name: groupName}
				if err := tx.Create(&gn).Error; err != nil {
					return err
				}
			}
			existing := map[string]models.Node{}
			for _, n := range gn.Nodes {
				existing[n.Name] = n
			}
			keep := map[uint]bool{}
			for _, n := range want {
				if old, ok := existing[n.Name]; ok {
					keep[uint(old.ID)] = true
					if old.Link != n.Link {
						tx.Model(&models.Node{}).Where("id = ?", old.ID).Update("link", n.Link)
					}
					continue
				}
				fresh := n
				if err := tx.Create(&fresh).Error; err != nil {
					return err
				}
				if err := tx.Model(&fresh).Association("GroupNodes").Append(&gn); err != nil {
					return err
				}
				keep[uint(fresh.ID)] = true
			}
			for _, n := range gn.Nodes {
				if !keep[uint(n.ID)] {
					tx.Exec("DELETE FROM group_node_nodes WHERE node_id = ?", n.ID)
					tx.Exec("DELETE FROM subcription_nodes WHERE node_id = ?", n.ID)
					tx.Delete(&models.Node{}, n.ID)
				}
			}
		}
		return nil
	})
}
