package api

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/xraycore"

	"github.com/gin-gonic/gin"
)

// ---- 服务器 ----------------------------------------------------------------

type serverView struct {
	models.ProxyServer
	Inbounds  []models.ProxyInbound `json:"inbounds"`
	Online    bool                  `json:"online"`
	TodayUp   int64                 `json:"todayUp"`
	TodayDown int64                 `json:"todayDown"`
	Users     int                   `json:"users"`
}

// ProxyServerList 服务器、入站与今日流量。agent 超过 3 分钟没回报视为离线。
func ProxyServerList(c *gin.Context) {
	var servers []models.ProxyServer
	if err := models.DB.Order("id asc").Find(&servers).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取服务器失败"})
		return
	}
	day := time.Now().Format("2006-01-02")
	views := make([]serverView, 0, len(servers))
	for _, s := range servers {
		v := serverView{ProxyServer: s, Inbounds: []models.ProxyInbound{}}
		models.DB.Where("server_id = ?", s.ID).Order("id asc").Find(&v.Inbounds)
		v.Online = s.LastSeenAt != nil && time.Since(*s.LastSeenAt) < 3*time.Minute
		var sum struct{ Up, Down int64 }
		models.DB.Model(&models.ProxyTraffic{}).Select("COALESCE(SUM(up),0) AS up, COALESCE(SUM(down),0) AS down").
			Where("day = ? AND server_id = ?", day, s.ID).Scan(&sum)
		v.TodayUp, v.TodayDown = sum.Up, sum.Down
		var users int64
		models.DB.Model(&models.ProxyTraffic{}).Where("day = ? AND server_id = ?", day, s.ID).Distinct("user_id").Count(&users)
		v.Users = int(users)
		views = append(views, v)
	}
	c.JSON(200, gin.H{"code": "00000", "data": views, "msg": "自建服务器"})
}

type serverSaveRequest struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	PublicHost string `json:"publicHost"`
	Enabled    bool   `json:"enabled"`
}

func validHost(host string) bool {
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 || !strings.Contains(host, ".") {
		return false
	}
	for _, r := range host {
		if !(r == '.' || r == '-' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

func installCommand(c *gin.Context, token string) string {
	return fmt.Sprintf("curl -fsSL -H 'Authorization: Agent %s' %s/agent/install.sh | sh", token, baseURL(c))
}

// ProxyServerSave 新建或修改服务器。新建 agent 服务器时返回一次性的令牌与安装命令。
func ProxyServerSave(c *gin.Context) {
	var req serverSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.PublicHost = strings.TrimSpace(req.PublicHost)
	if req.Name == "" || !validHost(req.PublicHost) {
		c.JSON(400, gin.H{"code": "40000", "msg": "请填写名称和客户端连接用的域名或 IP"})
		return
	}
	if req.ID != 0 {
		res := models.DB.Model(&models.ProxyServer{}).Where("id = ?", req.ID).
			Updates(map[string]any{"name": req.Name, "public_host": req.PublicHost, "enabled": req.Enabled})
		if res.Error != nil || res.RowsAffected == 0 {
			c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
			return
		}
		NotifyProxyChanged()
		c.JSON(200, gin.H{"code": "00000", "msg": "已保存"})
		return
	}
	if req.Kind != models.ServerKindLocal && req.Kind != models.ServerKindAgent {
		c.JSON(400, gin.H{"code": "40000", "msg": "未知的服务器类型"})
		return
	}
	if req.Kind == models.ServerKindLocal {
		var n int64
		models.DB.Model(&models.ProxyServer{}).Where("kind = ?", models.ServerKindLocal).Count(&n)
		if n > 0 {
			c.JSON(400, gin.H{"code": "40000", "msg": "本机服务器已存在"})
			return
		}
		if runtime.GOOS != "linux" {
			c.JSON(400, gin.H{"code": "40000", "msg": "本机运行 xray 仅支持 Linux"})
			return
		}
	}
	server := models.ProxyServer{Name: req.Name, Kind: req.Kind, PublicHost: req.PublicHost, Enabled: true}
	token := ""
	if req.Kind == models.ServerKindAgent {
		token = models.RandomHex(24)
		server.AgentToken = models.HashAgentToken(token)
	}
	if err := models.DB.Create(&server).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "创建失败"})
		return
	}
	NotifyProxyChanged()
	data := gin.H{"id": server.ID}
	if token != "" {
		data["token"], data["install"] = token, installCommand(c, token)
	}
	c.JSON(200, gin.H{"code": "00000", "data": data, "msg": "已创建"})
}

// ProxyServerResetToken 重置 agent 令牌（旧 agent 立即失效），返回新的安装命令。
func ProxyServerResetToken(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	token := models.RandomHex(24)
	res := models.DB.Model(&models.ProxyServer{}).Where("id = ? AND kind = ?", id, models.ServerKindAgent).
		Update("agent_token", models.HashAgentToken(token))
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(404, gin.H{"code": "40400", "msg": "agent 服务器不存在"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"token": token, "install": installCommand(c, token)}, "msg": "令牌已重置"})
}

// ProxyServerDelete 删除服务器及其入站；对应的自建节点分组随之清理。
func ProxyServerDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	var s models.ProxyServer
	if err := models.DB.First(&s, id).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	models.DB.Where("server_id = ?", s.ID).Delete(&models.ProxyInbound{})
	// 先停用让同步清空节点，再删除记录
	models.DB.Model(&s).Update("enabled", false)
	_ = SyncSelfHostedNodes()
	models.DB.Delete(&s)
	models.DB.Where("name = ?", selfHostedGroupPrefix+s.Name).Delete(&models.GroupNode{})
	NotifyProxyChanged()
	c.JSON(200, gin.H{"code": "00000", "msg": "已删除；远程服务器上的 agent 可执行 systemctl disable --now ppeelink-agent ppeelink-xray 停止"})
}

// ---- 入站 ------------------------------------------------------------------

type inboundSaveRequest struct {
	ID            uint   `json:"id"`
	ServerID      uint   `json:"serverId"`
	Remark        string `json:"remark"`
	Template      string `json:"template"`
	Port          int    `json:"port"`
	Target        string `json:"target"`
	SNI           string `json:"sni"`
	Path          string `json:"path"`
	Enabled       bool   `json:"enabled"`
	RegenerateKey bool   `json:"regenerateKey"`
}

// ProxyInboundSave 新建或修改入站；新建时自动生成 Reality 密钥与 shortId。
func ProxyInboundSave(c *gin.Context) {
	var req inboundSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	if req.Template != xraycore.TemplateRealityVision && req.Template != xraycore.TemplateXHTTPReality {
		c.JSON(400, gin.H{"code": "40000", "msg": "未知的入站模板"})
		return
	}
	if req.Port < 1024 || req.Port > 65535 || req.Port == xrayAPIPort {
		c.JSON(400, gin.H{"code": "40000", "msg": "端口需在 1024 到 65535 之间"})
		return
	}
	if _, _, err := net.SplitHostPort(req.Target); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "伪装目标需为 域名:端口，如 www.apple.com:443"})
		return
	}
	if strings.TrimSpace(req.SNI) == "" {
		req.SNI, _, _ = net.SplitHostPort(req.Target)
	}
	if req.Template == xraycore.TemplateXHTTPReality && !strings.HasPrefix(req.Path, "/") {
		req.Path = "/" + models.RandomHex(6)
	}
	var server models.ProxyServer
	if err := models.DB.First(&server, req.ServerID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "服务器不存在"})
		return
	}
	var clash int64
	models.DB.Model(&models.ProxyInbound{}).Where("server_id = ? AND port = ? AND id <> ?", req.ServerID, req.Port, req.ID).Count(&clash)
	if clash > 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "该端口已被本服务器的其他入站使用"})
		return
	}
	in := models.ProxyInbound{}
	if req.ID != 0 {
		if err := models.DB.First(&in, req.ID).Error; err != nil {
			c.JSON(404, gin.H{"code": "40400", "msg": "入站不存在"})
			return
		}
	}
	in.ServerID, in.Remark, in.Template, in.Port = req.ServerID, strings.TrimSpace(req.Remark), req.Template, req.Port
	in.Target, in.SNI, in.Path, in.Enabled = req.Target, strings.TrimSpace(req.SNI), req.Path, req.Enabled
	if in.PrivateKey == "" || req.RegenerateKey {
		priv, pub, err := xraycore.GenerateRealityKeys()
		if err != nil {
			c.JSON(500, gin.H{"code": "50000", "msg": "生成密钥失败"})
			return
		}
		in.PrivateKey, in.PublicKey, in.ShortID = priv, pub, models.RandomHex(4)
	}
	if err := models.DB.Save(&in).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	NotifyProxyChanged()
	c.JSON(200, gin.H{"code": "00000", "data": in, "msg": "已保存，服务器将在一分钟内应用"})
}

func ProxyInboundDelete(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	if err := models.DB.Delete(&models.ProxyInbound{}, id).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "删除失败"})
		return
	}
	NotifyProxyChanged()
	c.JSON(200, gin.H{"code": "00000", "msg": "已删除"})
}

// ProxyCheckTarget 从面板所在机器检测伪装目标是否支持 TLS 1.3 与 h2。
func ProxyCheckTarget(c *gin.Context) {
	var req struct {
		Target string `json:"target"`
		SNI    string `json:"sni"`
	}
	_ = c.ShouldBindJSON(&req)
	c.JSON(200, gin.H{"code": "00000", "data": xraycore.CheckRealityTarget(req.Target, req.SNI, 6*time.Second), "msg": "检测完成"})
}

// ---- 账户配额 ---------------------------------------------------------------

type accountView struct {
	models.ProxyAccount
	Username   string `json:"username"`
	Nickname   string `json:"nickname"`
	TodayBytes int64  `json:"todayBytes"`
	Active     bool   `json:"active"`
}

// ProxyAccountList 所有账户的自建节点权限、配额与用量（未开通的也列出，便于开通）。
func ProxyAccountList(c *gin.Context) {
	var users []models.User
	models.DB.Order("id asc").Find(&users)
	day := time.Now().Format("2006-01-02")
	now := time.Now()
	list := make([]accountView, 0, len(users))
	for _, u := range users {
		var acc models.ProxyAccount
		if models.DB.Where("user_id = ?", u.ID).First(&acc).Error != nil {
			acc = models.ProxyAccount{UserID: u.ID}
		}
		var today struct{ Total int64 }
		models.DB.Model(&models.ProxyTraffic{}).Select("COALESCE(SUM(up+down),0) AS total").
			Where("day = ? AND user_id = ?", day, u.ID).Scan(&today)
		list = append(list, accountView{ProxyAccount: acc, Username: u.Username, Nickname: u.Nickname,
			TodayBytes: today.Total, Active: !u.Disabled && acc.Active(now)})
	}
	c.JSON(200, gin.H{"code": "00000", "data": list, "msg": "自建节点账户"})
}

type accountSaveRequest struct {
	UserID       int        `json:"userId"`
	Enabled      bool       `json:"enabled"`
	TrafficLimit int64      `json:"trafficLimit"`
	ResetDay     int        `json:"resetDay"`
	ExpireAt     *time.Time `json:"expireAt"`
	DeviceLimit  int        `json:"deviceLimit"`
	ResetUsage   bool       `json:"resetUsage"`
	Regenerate   bool       `json:"regenerate"`
	Unblock      bool       `json:"unblock"`
}

// ProxyAccountSave 开通 / 停用、设置配额；可清零用量、解除临时封禁或更换凭据。
func ProxyAccountSave(c *gin.Context) {
	var req accountSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	if req.TrafficLimit < 0 || req.DeviceLimit < 0 || req.ResetDay < 0 || req.ResetDay > 28 {
		c.JSON(400, gin.H{"code": "40000", "msg": "配额参数无效（清零日为 1 到 28）"})
		return
	}
	var user models.User
	if err := models.DB.First(&user, req.UserID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户不存在"})
		return
	}
	acc, err := models.EnsureProxyAccount(models.DB, req.UserID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取账户失败"})
		return
	}
	updates := map[string]any{
		"enabled": req.Enabled, "traffic_limit": req.TrafficLimit, "reset_day": req.ResetDay,
		"expire_at": req.ExpireAt, "device_limit": req.DeviceLimit,
	}
	if req.ResetUsage {
		updates["traffic_used"] = 0
		updates["last_reset_at"] = time.Now()
	}
	if req.Unblock {
		updates["blocked_until"] = nil
		updates["blocked_reason"] = ""
	}
	if req.Regenerate {
		updates["uuid"] = models.NewUUIDv4()
		updates["sub_token"] = models.RandomHex(16)
	}
	if err := models.DB.Model(&models.ProxyAccount{}).Where("user_id = ?", acc.UserID).Updates(updates).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	NotifyProxyChanged()
	c.JSON(200, gin.H{"code": "00000", "msg": "已保存"})
}

// ProxyTrafficDaily 最近 N 天的每日总流量（按服务器汇总）。
func ProxyTrafficDaily(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days <= 0 || days > 180 {
		days = 30
	}
	since := time.Now().AddDate(0, 0, -days+1).Format("2006-01-02")
	var rows []struct {
		Day      string `json:"day"`
		ServerID uint   `json:"serverId"`
		Up       int64  `json:"up"`
		Down     int64  `json:"down"`
	}
	models.DB.Model(&models.ProxyTraffic{}).Select("day, server_id, SUM(up) AS up, SUM(down) AS down").
		Where("day >= ?", since).Group("day, server_id").Order("day asc").Scan(&rows)
	c.JSON(200, gin.H{"code": "00000", "data": rows, "msg": "每日流量"})
}

// ---- agent 接口（令牌认证，不走 JWT） -----------------------------------------

func agentServer(c *gin.Context) (*models.ProxyServer, bool) {
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Agent "))
	if token == "" || token == auth {
		c.JSON(401, gin.H{"code": "40100", "msg": "缺少 agent 令牌"})
		return nil, false
	}
	var s models.ProxyServer
	if err := models.DB.Where("agent_token = ? AND kind = ?", models.HashAgentToken(token), models.ServerKindAgent).First(&s).Error; err != nil {
		c.JSON(401, gin.H{"code": "40100", "msg": "agent 令牌无效"})
		return nil, false
	}
	return &s, true
}

func AgentSpec(c *gin.Context) {
	s, ok := agentServer(c)
	if !ok {
		return
	}
	spec, err := BuildServerSpec(*s)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "生成配置失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": spec, "msg": "ok"})
}

func AgentReport(c *gin.Context) {
	s, ok := agentServer(c)
	if !ok {
		return
	}
	var report xraycore.Report
	if err := c.ShouldBindJSON(&report); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "回报格式错误"})
		return
	}
	if err := ApplyServerReport(*s, report); err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "记录失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// AgentInstallScript 生成安装脚本：下载与面板同版本的程序，以 systemd 运行 agent。
// 令牌放在 EnvironmentFile（600 权限）里，不出现在进程参数中。
func AgentInstallScript(c *gin.Context) {
	if _, ok := agentServer(c); !ok {
		return
	}
	token := strings.TrimSpace(strings.TrimPrefix(c.GetHeader("Authorization"), "Agent "))
	panel := baseURL(c)
	script := fmt.Sprintf(`#!/bin/sh
set -eu
PANEL='%s'
TOKEN='%s'
[ "$(id -u)" = 0 ] || { echo "请用 root 运行"; exit 1; }
command -v systemctl >/dev/null || { echo "需要 systemd"; exit 1; }
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) echo "不支持的架构: $(uname -m)"; exit 1 ;;
esac
curl -fsSL -H "Authorization: Agent $TOKEN" "$PANEL/agent/binary?arch=$ARCH" -o /usr/local/bin/ppeelink-agent.new
chmod 755 /usr/local/bin/ppeelink-agent.new
mv /usr/local/bin/ppeelink-agent.new /usr/local/bin/ppeelink-agent
mkdir -p /etc/ppeelink-agent /var/lib/ppeelink-agent
umask 077
printf 'PPEELINK_AGENT_TOKEN=%%s\n' "$TOKEN" > /etc/ppeelink-agent/env
cat > /etc/systemd/system/ppeelink-agent.service <<UNIT
[Unit]
Description=PPEELink agent
After=network-online.target
Wants=network-online.target

[Service]
EnvironmentFile=/etc/ppeelink-agent/env
ExecStart=/usr/local/bin/ppeelink-agent agent -panel $PANEL -dir /var/lib/ppeelink-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable ppeelink-agent >/dev/null 2>&1
systemctl restart ppeelink-agent
echo "PPEELink agent 已安装并启动，一分钟内会出现在面板中。"
`, panel, token)
	c.Data(200, "text/x-shellscript; charset=utf-8", []byte(script))
}

// AgentBinary 下发面板自身的可执行文件（同一程序的 agent 子命令）。
func AgentBinary(c *gin.Context) {
	if _, ok := agentServer(c); !ok {
		return
	}
	if arch := c.Query("arch"); arch != runtime.GOARCH {
		c.JSON(400, gin.H{"code": "40000", "msg": fmt.Sprintf("面板程序为 %s 架构，无法下发 %s 版本", runtime.GOARCH, arch)})
		return
	}
	exe, err := os.Executable()
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "找不到面板程序"})
		return
	}
	c.Header("Content-Disposition", "attachment; filename=ppeelink-agent")
	c.File(exe)
}

// ---- 个人自建节点订阅（公开，按账户令牌） -----------------------------------------

func defaultTemplates() node.SqlConfig {
	cfg := node.SqlConfig{Udp: true}
	for path, set := range map[string]*string{"./template/clash.yaml": &cfg.Clash, "./template/surge.conf": &cfg.Surge, "./template/loon.conf": &cfg.Loon} {
		if _, err := os.Stat(path); err == nil {
			*set = path
		}
	}
	return cfg
}

// accountUserinfo 个人订阅的流量头：已用量、额度与到期时间。
func accountUserinfo(acc models.ProxyAccount) string {
	var expire int64
	if acc.ExpireAt != nil {
		expire = acc.ExpireAt.Unix()
	}
	return fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d", acc.TrafficUsed, acc.TrafficLimit, expire)
}

// GetPersonalSubscription /p/?token=：账户在全部自建服务器上的节点。
func GetPersonalSubscription(c *gin.Context) {
	var acc models.ProxyAccount
	token := strings.TrimSpace(c.Query("token"))
	if token == "" || models.DB.Where("sub_token = ?", token).First(&acc).Error != nil {
		c.String(http.StatusNotFound, "无效的订阅令牌")
		return
	}
	var user models.User
	if models.DB.First(&user, acc.UserID).Error != nil || user.Disabled || !acc.Enabled {
		c.String(http.StatusForbidden, "自建节点未开通或账户已停用")
		return
	}
	nodes := AccountNodes(acc)
	if len(nodes) == 0 {
		c.String(http.StatusServiceUnavailable, "暂无可用的自建节点")
		return
	}
	client := normalizeClient(c.Query("client"))
	if client == "" {
		client = clientFromUserAgent(c.Request.UserAgent())
	}
	body, ext, contentType := renderCollectionNodes(client, nodes, defaultTemplates())
	c.Header("Subscription-Userinfo", accountUserinfo(acc))
	c.Header("Profile-Update-Interval", "12")
	c.Header("Content-Disposition", "inline; filename*=utf-8''"+strings.ReplaceAll("自建节点."+ext, " ", "%20"))
	c.Data(200, contentType, body)
}

// personalLinks “我的订阅”里展示的自建节点订阅链接。
func personalLinks(c *gin.Context, acc models.ProxyAccount) (map[string]string, map[string]string) {
	base := baseURL(c)
	links := map[string]string{}
	for _, cl := range subscriptionClients {
		links[cl] = base + "/p/?token=" + acc.SubToken + "&client=" + cl
	}
	return links, importLinks(links)
}

// MySelfHosted 当前账户的自建节点订阅（未开通时 enabled 为 false）。
func MySelfHosted(c *gin.Context) {
	user, ok := currentUser(c)
	if !ok {
		c.JSON(401, gin.H{"code": 401, "msg": "账户不存在"})
		return
	}
	var acc models.ProxyAccount
	if models.DB.Where("user_id = ?", user.ID).First(&acc).Error != nil || !acc.Enabled {
		c.JSON(200, gin.H{"code": "00000", "data": gin.H{"enabled": false}, "msg": "未开通"})
		return
	}
	links, imports := personalLinks(c, acc)
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"enabled": true, "active": acc.Active(time.Now()), "links": links, "imports": imports,
		"used": acc.TrafficUsed, "limit": acc.TrafficLimit, "expireAt": acc.ExpireAt,
		"blockedReason": acc.BlockedReason, "blockedUntil": acc.BlockedUntil, "nodes": len(AccountNodes(acc)),
	}, "msg": "自建节点"})
}
