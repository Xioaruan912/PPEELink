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
	if server.Removing {
		return xraycore.Spec{Uninstall: true}, nil
	}
	spec := xraycore.Spec{
		Version: xraycore.DefaultVersion, Enabled: server.Enabled, APIPort: xrayAPIPort,
		BlockPrivate: true, BlockBitTorrent: true,
		Monitor: &xraycore.MonitorSpec{Enabled: server.MonitorEnabled, IntervalSeconds: 60},
	}
	// 本月流量超出上限：停止 xray，到重置日后下一轮拉取时自动恢复
	if server.TrafficLimit > 0 && serverQuota(server, time.Now()).Over {
		spec.Enabled = false
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
		if err := applyAgentMonitor(tx, server, report, now); err != nil {
			return err
		}
		return nil
	})
}

// applyAgentMonitor 把同一个 agent 上报的服务器指标写入监控表。
// 自建节点和服务器监控共用一条 agent 连接，不再额外安装监控服务。
func applyAgentMonitor(tx *gorm.DB, server models.ProxyServer, report xraycore.Report, now time.Time) error {
	if report.Monitor == nil || !server.MonitorEnabled {
		return nil
	}
	var monitor models.Monitor
	err := tx.Where("proxy_server_id = ?", server.ID).First(&monitor).Error
	if err == gorm.ErrRecordNotFound {
		serverID := server.ID
		// 同一台机器可能已有用户手动接入的独立监控（SSH 地址与服务器公网地址一致）。
		// 直接复用它，避免 agent 监控和独立监控各出现一张卡片。
		if strings.TrimSpace(server.PublicHost) != "" {
			err = tx.Where("proxy_server_id IS NULL AND ssh_host <> '' AND LOWER(ssh_host) = ?", strings.ToLower(strings.TrimSpace(server.PublicHost))).
				Order("id asc").First(&monitor).Error
		}
		if err == gorm.ErrRecordNotFound || err != nil || monitor.ID == 0 {
			monitor = models.Monitor{
				Name: server.Name, Group: "自建节点", ProxyServerID: &serverID, Enabled: true,
			}
			if err := tx.Create(&monitor).Error; err != nil {
				return err
			}
		} else {
			if err := tx.Model(&monitor).Updates(map[string]any{"proxy_server_id": serverID, "enabled": true}).Error; err != nil {
				return err
			}
		}
	} else if err != nil {
		return err
	}
	metric := models.MonitorMetric{
		MonitorID: monitor.ID, CollectedAt: now,
		CPUPercent: report.Monitor.CPUPercent,
		MemUsed:    report.Monitor.MemUsed, MemTotal: report.Monitor.MemTotal,
		SwapUsed: report.Monitor.SwapUsed, SwapTotal: report.Monitor.SwapTotal,
		DiskUsed: report.Monitor.DiskUsed, DiskTotal: report.Monitor.DiskTotal,
		Load1: report.Monitor.Load1, Load5: report.Monitor.Load5, Load15: report.Monitor.Load15,
		Uptime:     report.Monitor.Uptime,
		NetInSpeed: report.Monitor.NetInSpeed, NetOutSpeed: report.Monitor.NetOutSpeed,
		TransferIn: report.Monitor.TransferIn, TransferOut: report.Monitor.TransferOut,
		TCPConns: report.Monitor.TCPConns, UDPConns: report.Monitor.UDPConns,
		ProcessCount: report.Monitor.ProcessCount,
	}
	if err := tx.Create(&metric).Error; err != nil {
		return err
	}
	return tx.Model(&monitor).Updates(map[string]any{
		"name": server.Name, "last_seen_at": now, "os": "linux", "arch": report.Arch,
	}).Error
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
	done   chan struct{}
	runner *xraycore.Runner
	id     uint
	wake   chan struct{}
}

const localXrayDir = "./xray"

// UninstallLocalProxy 停止面板服务器的同步循环，并卸载 xray（systemd 服务、二进制与配置）。
func UninstallLocalProxy() error {
	localProxy.mu.Lock()
	cancel, done, runner := localProxy.cancel, localProxy.done, localProxy.runner
	localProxy.cancel, localProxy.done, localProxy.runner = nil, nil, nil
	localProxy.mu.Unlock()
	// 等循环退出后再卸载，避免它在卸载过程中重新拉起 xray；循环内可能回调 NotifyProxyChanged，所以不能持锁等待
	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-time.After(15 * time.Second):
		}
	}
	if runner == nil {
		runner = xraycore.NewRunner(localXrayDir, "ppeelink-xray")
	}
	return runner.Uninstall()
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
	done := make(chan struct{})
	runner := xraycore.NewRunner(localXrayDir, "ppeelink-xray")
	localProxy.cancel, localProxy.id, localProxy.done, localProxy.runner = cancel, s.ID, done, runner
	go func() {
		defer close(done)
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
		nodes = append(nodes, models.Node{
			Name: name, Link: InboundLink(s, in, acc, name),
			ProbePolicy: "any", Source: selfHostedSource,
		})
	}
	return nodes
}

// AccountNodes 账户可用的全部自建节点（启用的服务器与入站）。
func AccountNodes(acc models.ProxyAccount) []models.Node {
	var servers []models.ProxyServer
	models.DB.Where("enabled = ? AND public_host <> ''", true).Order("id asc").Find(&servers)
	nodes := []models.Node{}
	now := time.Now()
	for _, s := range servers {
		if s.TrafficLimit > 0 && serverQuota(s, now).Over {
			continue // 本月流量用完，到重置日前不下发
		}
		nodes = append(nodes, serverNodes(s, acc)...)
	}
	return nodes
}

// SyncSelfHostedNodes 用第一个开通了自建节点的管理员账户生成节点，统一放进「自建」分组。
// 旧版本的「自建·服务器名」会先迁移并合并，已有订阅引用不会丢失。
func SyncSelfHostedNodes() error {
	if err := migrateSelfHostedGroups(); err != nil {
		return err
	}
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
	want := make([]models.Node, 0)
	if owner != nil {
		for _, s := range servers {
			if s.Enabled && s.PublicHost != "" {
				want = append(want, serverNodes(s, *owner)...)
			}
		}
	}
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		var group models.GroupNode
		err := tx.Preload("Nodes").Where("name = ?", selfHostedGroupName).First(&group).Error
		if err == gorm.ErrRecordNotFound {
			if len(want) == 0 {
				return nil
			}
			group = models.GroupNode{Name: selfHostedGroupName}
			if err := tx.Create(&group).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err := backfillManagedSources(tx); err != nil {
			return err
		}
		if err := tx.Preload("Nodes").First(&group, group.ID).Error; err != nil {
			return err
		}
		byName := map[string]models.Node{}
		for _, n := range group.Nodes {
			byName[n.Name] = n
		}
		wanted := map[string]bool{}
		for _, n := range want {
			if wanted[n.Name] {
				continue
			}
			wanted[n.Name] = true
			if old, ok := byName[n.Name]; ok {
				updates := map[string]any{"source": selfHostedSource}
				if old.Link != n.Link {
					updates["link"] = n.Link
				}
				if old.ProbePolicy == "" || old.ProbePolicy == "panel" {
					updates["probe_policy"] = "any"
				}
				if err := tx.Model(&models.Node{}).Where("id = ?", old.ID).Updates(updates).Error; err != nil {
					return err
				}
				continue
			}
			var existing models.Node
			if err := tx.Where("name = ?", n.Name).First(&existing).Error; err == nil {
				updates := map[string]any{"source": selfHostedSource}
				if existing.Link != n.Link {
					updates["link"] = n.Link
				}
				if existing.ProbePolicy == "" || existing.ProbePolicy == "panel" {
					updates["probe_policy"] = "any"
				}
				if err := tx.Model(&models.Node{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
					return err
				}
				if err := tx.Model(&group).Association("Nodes").Append(&existing); err != nil {
					return err
				}
				continue
			}
			fresh := n
			fresh.Source = selfHostedSource
			if fresh.ProbePolicy == "" || fresh.ProbePolicy == "panel" {
				fresh.ProbePolicy = "any"
			}
			if err := tx.Create(&fresh).Error; err != nil {
				return err
			}
			if err := tx.Model(&fresh).Association("GroupNodes").Append(&group); err != nil {
				return err
			}
		}
		for _, n := range group.Nodes {
			if n.Source != selfHostedSource || wanted[n.Name] {
				continue
			}
			if err := tx.Exec("DELETE FROM group_node_nodes WHERE node_id = ?", n.ID).Error; err != nil {
				return err
			}
			if err := tx.Exec("DELETE FROM subcription_nodes WHERE node_id = ?", n.ID).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&models.Node{}, n.ID).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	InvalidateOverview()
	return nil
}

// serverQuotaState 自建服务器本计费周期的流量使用情况。
type serverQuotaState struct {
	Used    int64     `json:"used"`
	Limit   int64     `json:"limit"`
	Over    bool      `json:"over"`
	ResetAt time.Time `json:"resetAt"` // 下一次重置（恢复）时间
}

// quotaPeriod 返回 now 所在计费周期的起点和下一次重置时间。resetDay 取 1-28，其它值按 1 号。
func quotaPeriod(now time.Time, resetDay int) (time.Time, time.Time) {
	if resetDay < 1 || resetDay > 28 {
		resetDay = 1
	}
	start := time.Date(now.Year(), now.Month(), resetDay, 0, 0, 0, 0, now.Location())
	if now.Before(start) {
		start = start.AddDate(0, -1, 0)
	}
	return start, start.AddDate(0, 1, 0)
}

// serverQuota 统计服务器本周期已用流量（按每日记录累加）。
func serverQuota(s models.ProxyServer, now time.Time) serverQuotaState {
	start, next := quotaPeriod(now, s.ResetDay)
	var used int64
	models.DB.Model(&models.ProxyTraffic{}).Select("COALESCE(SUM(up + down), 0)").
		Where("server_id = ? AND day >= ?", s.ID, start.Format("2006-01-02")).Scan(&used)
	return serverQuotaState{Used: used, Limit: s.TrafficLimit, ResetAt: next,
		Over: s.TrafficLimit > 0 && used >= s.TrafficLimit}
}
