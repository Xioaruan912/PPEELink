package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// expandNodeLink 将一个存储的节点链接展开为一到多个分享链接。
// 支持逗号分隔的多链接、远程订阅/转换地址（base64/明文链接列表或 Clash YAML）以及普通链接。
func expandNodeLink(link string) []string {
	link = strings.TrimSpace(link)
	switch {
	case link == "":
		return nil
	case strings.Contains(link, ","):
		parts := strings.Split(link, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	case strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://"):
		return fetchAndExpandRemote(link)
	default:
		return []string{link}
	}
}

func fetchAndExpandRemote(rawURL string) []string {
	client := utils.SafeHTTPClient(20 * time.Second)
	resp, err := client.Get(rawURL)
	if err != nil {
		log.Printf("[Subscription] 拉取转换源失败: %v", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if node.IsClashConfig(body) {
		clashNodes, err := node.ParseClashToNodes(body)
		if err != nil {
			log.Printf("[Subscription] 解析 Clash 转换源失败: %v", err)
			return nil
		}
		out := make([]string, 0, len(clashNodes))
		for _, cn := range clashNodes {
			out = append(out, cn.Link)
		}
		return out
	}
	decoded := node.Base64Decode(string(body))
	if strings.TrimSpace(decoded) == "" {
		decoded = string(body)
	}
	lines := strings.FieldsFunc(decoded, func(r rune) bool { return r == '\n' || r == '\r' })
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// collectNodeInputs 展开订阅节点的链接，并带上每个节点的构建覆盖属性。
func collectNodeInputs(nodes []models.Node) ([]string, map[string]node.NodeFlags) {
	urls := []string{}
	flags := map[string]node.NodeFlags{}
	byID := map[int]models.Node{}
	linksByID := map[int][]string{}
	names := map[int]string{}
	remember := func(n models.Node) {
		byID[n.ID] = n
		links := expandNodeLink(n.Link)
		linksByID[n.ID] = links
		names[n.ID] = n.Name
		if len(links) == 1 {
			if p, e := node.ParseOutbound(links[0]); e == nil && p.Name != "" {
				names[n.ID] = p.Name
			}
		}
	}
	for _, n := range nodes {
		remember(n)
	}
	emitted := map[int]bool{}
	var visit func(models.Node, int)
	visit = func(n models.Node, depth int) {
		if emitted[n.ID] {
			return
		}
		emitted[n.ID] = true
		f := decodeNodeFlags(n.Flags)
		if f.DialerProxyID != 0 {
			relay, ok := byID[f.DialerProxyID]
			if !ok && depth < 4 {
				if models.DB.First(&relay, f.DialerProxyID).Error == nil {
					remember(relay)
					ok = true
				}
			}
			if ok {
				f.DialerProxy = names[relay.ID]
				if depth < 4 {
					visit(relay, depth+1)
				}
			} else {
				f.DialerProxy = fmt.Sprintf("已删除的中转节点 #%d", f.DialerProxyID)
			}
		}
		links := linksByID[n.ID]
		urls = append(urls, links...)
		if !f.Empty() {
			for _, link := range links {
				flags[link] = f
			}
		}
	}
	for _, n := range nodes {
		visit(n, 0)
	}
	return urls, flags
}

// mergeGroupNodes 将订阅引用的分组节点合并进 sub.Nodes（去重，分组节点在后），
// 并去掉暂不下发的节点（见 node_delivery.go）。
// 分组引用按 ID 关联，分组节点变化后订阅拉取时自动跟进
func mergeGroupNodes(sub *models.Subcription) error {
	_, err := mergeGroupNodesDetailed(sub)
	return err
}

// mergeGroupNodesDetailed 同 mergeGroupNodes，另外返回被暂不下发的节点及原因。
func mergeGroupNodesDetailed(sub *models.Subcription) ([]excludedNode, error) {
	if sub.ID == 0 {
		return nil, nil
	}
	var withRefs models.Subcription
	if err := models.DB.Preload("Nodes").Preload("GroupRefs.Nodes").First(&withRefs, sub.ID).Error; err != nil {
		return nil, err
	}
	// 收集当前手动节点（按名称去重）
	seen := map[string]bool{}
	merged := []models.Node{}
	hiddenIDs, _ := models.GloballyHiddenNodeIDs()
	for _, n := range withRefs.Nodes {
		if n.Name == "" || n.Hidden || hiddenIDs[n.ID] || seen[n.Name] {
			continue
		}
		seen[n.Name] = true
		merged = append(merged, n)
	}

	// 追加各分组当前节点
	for _, g := range withRefs.GroupRefs {
		if g.Hidden {
			continue
		}
		for _, n := range g.Nodes {
			if n.Name == "" || n.Hidden || hiddenIDs[n.ID] {
				continue
			}
			if seen[n.Name] {
				continue
			}
			seen[n.Name] = true
			merged = append(merged, n)
		}
	}

	merged, excluded := filterDeliverable(merged, undeliverableNodes(time.Now()))
	sub.Nodes = merged
	if sub.Pipeline != "" {
		preview, err := ApplySubscriptionPipeline(sub.Nodes, sub.Pipeline)
		if err != nil {
			return excluded, err
		}
		sub.Nodes = preview.Nodes
	}
	return excluded, nil
}

// subName 从请求上下文取订阅名（由 GetClient 匹配后设置，避免全局变量并发串号）
func subName(c *gin.Context) string {
	v, _ := c.Get("subname")
	s, _ := v.(string)
	return s
}

func GetClient(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.String(400, "token为空")
		return
	}
	// 仅按随机 token 匹配，不再兼容可猜测的 md5(订阅名)
	sub, err := models.FindSubscriptionByToken(token)
	if err != nil {
		// 非 200 状态让客户端保留旧配置，而不是把错误文字当配置保存
		c.String(404, "无效的订阅令牌")
		return
	}
	if sub.ExpiresAt != nil && time.Now().After(*sub.ExpiresAt) {
		c.String(410, "订阅已过期")
		return
	}
	// 记录订阅名供访问日志中间件使用
	c.Set("subname", sub.Name)
	client := normalizeClient(c.Query("client"))
	if client == "" {
		client = clientFromUserAgent(c.Request.UserAgent())
	}
	serveSubscriptionClient(c, *sub, client)
}

// normalizeClient 将客户端标识归一化为内部名称，无法识别时返回空串。
func normalizeClient(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	// Stash / Egern / FlClash / Clash Verge 等读取 Clash(mihomo) 配置
	case "clash", "mihomo", "clashmeta", "clash.meta", "meta", "stash", "egern", "flclash", "clash-verge", "clashverge", "meowx":
		return "clash"
	// Surfboard 与 Surge Mac 使用 Surge 风格配置
	case "surge", "surgemac", "surge-mac", "surfboard":
		return "surge"
	case "loon":
		return "loon"
	case "v2ray", "v2rayng", "v2raytun":
		return "v2ray"
	case "clash-proxies":
		return "clash-proxies"
	case "singbox", "sing-box", "sing_box":
		return "singbox"
	case "qx", "quantumultx", "quantumult", "quantumult x":
		return "qx"
	case "shadowrocket", "srocket", "sr":
		return "shadowrocket"
	default:
		return ""
	}
}

// clientFromUserAgent 根据 User-Agent 猜测客户端，默认 V2Ray。
func clientFromUserAgent(ua string) string {
	lower := strings.ToLower(ua)
	switch {
	case strings.Contains(lower, "mihomo") || strings.Contains(lower, "clash") ||
		strings.Contains(lower, "stash") || strings.Contains(lower, "egern"):
		return "clash"
	case strings.Contains(lower, "surge") || strings.Contains(lower, "surfboard"):
		return "surge"
	case strings.Contains(lower, "loon"):
		return "loon"
	case strings.Contains(lower, "sing-box") || strings.Contains(lower, "singbox"):
		return "singbox"
	case strings.Contains(lower, "quantumult"):
		return "qx"
	case strings.Contains(lower, "shadowrocket"):
		return "shadowrocket"
	default:
		return "v2ray"
	}
}
func GetV2ray(c *gin.Context) {
	var sub models.Subcription
	if subName(c) == "" {
		c.Writer.WriteString("订阅名为空")
		return
	}
	// subname := c.Param("subname")
	// subname := SunName
	// subname = node.Base64Decode(subname)
	sub.Name = subName(c)
	err := sub.Find()
	if err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	// 合并引用分组节点（分组变化自动跟进）
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}
	baselist := ""
	for _, v := range sub.Nodes {
		for _, link := range expandNodeLink(v.Link) {
			baselist += link + "\n"
		}
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.txt", subName(c))
	encodedFilename := url.QueryEscape(filename)
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+encodedFilename)
	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteString(node.Base64Encode(baselist))
}
func GetClash(c *gin.Context) {
	var sub models.Subcription
	// subname := c.Param("subname")
	// subname := node.Base64Decode(SunName)
	sub.Name = subName(c)
	err := sub.Find()
	if err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	// 合并引用分组节点（分组变化自动跟进）
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}

	urls, flags := collectNodeInputs(sub.Nodes)
	log.Printf("[Subscription] 构建 Clash 订阅: %s，节点数: %d\n", sub.Name, len(sub.Nodes))
	log.Printf("[Subscription] Clash 转换输入节点数: %d\n", len(urls))
	var configs node.SqlConfig
	err = json.Unmarshal([]byte(sub.Config), &configs)
	if err != nil {
		c.Writer.WriteString("配置读取错误")
		return
	}
	var DecodeClash []byte
	proxiesOnly := c.GetBool("clashProxiesOnly")
	token := c.GetString("subToken")
	base := c.GetString("providerBase")
	switch {
	case proxiesOnly:
		DecodeClash, err = node.EncodeClashProxies(urls, flags, configs)
	case configs.ProxyProvider && token != "" && base != "":
		providerURL := base + "/c/?token=" + url.QueryEscape(token) + "&client=clash-proxies"
		DecodeClash, err = node.EncodeClashProvider(configs.Clash, "sub-"+token[:8], providerURL, "")
	default:
		DecodeClash, err = node.EncodeClashWithFlags(urls, flags, configs)
	}
	if err != nil {
		c.String(500, err.Error())
		return
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.yaml", subName(c))
	encodedFilename := url.QueryEscape(filename)
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+encodedFilename)
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteString(string(DecodeClash))
}
func GetSurge(c *gin.Context) {
	var sub models.Subcription
	// subname := c.Param("subname")
	// subname := node.Base64Decode(SunName)
	sub.Name = subName(c)
	err := sub.Find()
	if err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	// 合并引用分组节点（分组变化自动跟进）
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}
	urls, flags := collectNodeInputs(sub.Nodes)

	var configs node.SqlConfig
	err = json.Unmarshal([]byte(sub.Config), &configs)
	if err != nil {
		c.Writer.WriteString("配置读取错误")
		return
	}
	// log.Println("surge路径:", configs)
	DecodeClash, err := node.EncodeSurgeWithFlags(urls, flags, configs)
	if err != nil {
		c.Writer.WriteString(err.Error())
		return
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.conf", subName(c))
	encodedFilename := url.QueryEscape(filename)
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+encodedFilename)
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	host := c.Request.Host
	url := c.Request.URL.String()
	// 如果包含头部更新信息
	if strings.Contains(DecodeClash, "#!MANAGED-CONFIG") {
		c.Writer.WriteString(DecodeClash)
		return
	}
	// 否则就插入头部更新信息
	interval := fmt.Sprintf("#!MANAGED-CONFIG %s interval=86400 strict=false", host+url)
	c.Writer.WriteString(string(interval + "\n" + DecodeClash))
}

// GetLoon 输出 Loon 订阅（填充 [Proxy] 段，策略组靠 Remote Filter 筛选）
func GetLoon(c *gin.Context) {
	var sub models.Subcription
	sub.Name = subName(c)
	err := sub.Find()
	if err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	// 合并引用分组节点（分组变化自动跟进）
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}

	urls, flags := collectNodeInputs(sub.Nodes)

	var configs node.SqlConfig
	err = json.Unmarshal([]byte(sub.Config), &configs)
	if err != nil {
		c.Writer.WriteString("配置读取错误")
		return
	}
	loonText, err := node.EncodeLoonWithFlags(urls, flags, configs)
	if err != nil {
		c.Writer.WriteString(err.Error())
		return
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.conf", subName(c))
	encodedFilename := url.QueryEscape(filename)
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+encodedFilename)
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteString(loonText)
}

// GetSingbox 输出 sing-box 客户端配置（JSON）。
func GetSingbox(c *gin.Context) {
	var sub models.Subcription
	sub.Name = subName(c)
	if err := sub.Find(); err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}
	urls, _ := collectNodeInputs(sub.Nodes)
	var configs node.SqlConfig
	if err := json.Unmarshal([]byte(sub.Config), &configs); err != nil {
		c.Writer.WriteString("配置读取错误")
		return
	}
	text, err := node.EncodeSingbox(urls, configs)
	if err != nil {
		c.Writer.WriteString(err.Error())
		return
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.json", subName(c))
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+url.QueryEscape(filename))
	c.Writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.Writer.WriteString(text)
}

// GetQX 输出 Quantumult X 节点列表。
func GetQX(c *gin.Context) {
	var sub models.Subcription
	sub.Name = subName(c)
	if err := sub.Find(); err != nil {
		c.Writer.WriteString("找不到这个订阅:" + subName(c))
		return
	}
	sub.Pipeline = pipelineWithOverrides(sub.Pipeline, c)
	if err := mergeGroupNodes(&sub); err != nil {
		log.Println("合并分组节点失败:", err)
	}
	urls, _ := collectNodeInputs(sub.Nodes)
	var configs node.SqlConfig
	if err := json.Unmarshal([]byte(sub.Config), &configs); err != nil {
		c.Writer.WriteString("配置读取错误")
		return
	}
	text, err := node.EncodeQX(urls, configs)
	if err != nil {
		c.Writer.WriteString(err.Error())
		return
	}
	c.Set("subname", subName(c))
	filename := fmt.Sprintf("%s.conf", subName(c))
	c.Writer.Header().Set("Content-Disposition", "inline; filename*=utf-8''"+url.QueryEscape(filename))
	c.Writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.Writer.WriteString(text)
}
