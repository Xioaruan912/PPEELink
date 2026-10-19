package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ppeelink/models"
	"ppeelink/node"
	"ppeelink/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 订阅的上游来源：订阅可以直接引用一个或多个上游订阅链接（主源 + 备用源），
// 面板定时拉取，把节点写入与订阅同名的分组并更新订阅的节点。

// sourceSyncInterval 上游订阅的自动刷新间隔。
const sourceSyncInterval = 6 * time.Hour

const sourceMaxBytes = 8 << 20

// fetchSubscriptionSource 按顺序尝试换行或逗号分隔的多个源，首个 2xx 非空响应生效。
func fetchSubscriptionSource(raw string) ([]byte, http.Header, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == '\n' || r == ',' })
	client := utils.SafeHTTPClient(30 * time.Second)
	var lastErr error
	for _, source := range parts {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		req, err := http.NewRequest("GET", source, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("User-Agent", "v2rayNG/1.8.5")
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, sourceMaxBytes))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("%s 返回 HTTP %d", source, resp.StatusCode)
			continue
		}
		if len(body) == 0 {
			lastErr = fmt.Errorf("%s 返回空内容", source)
			continue
		}
		return body, resp.Header, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("没有可用的订阅源")
	}
	return nil, nil, lastErr
}

// nodesFromSubscriptionBody 解析上游常见的三种格式：Clash/Mihomo YAML、
// base64 编码的链接列表、纯文本链接列表。返回节点与识别出的格式。
func nodesFromSubscriptionBody(body []byte) ([]models.Node, string, error) {
	if node.IsClashConfig(body) {
		clashNodes, err := node.ParseClashToNodes(body)
		if err != nil {
			return nil, "clash-yaml", err
		}
		nodesList := make([]models.Node, 0, len(clashNodes))
		for _, cn := range clashNodes {
			if isPlaceholderNodeName(cn.Name) {
				continue
			}
			nodesList = append(nodesList, models.Node{Name: cn.Name, Link: cn.Link})
		}
		if len(nodesList) == 0 {
			return nil, "clash-yaml", fmt.Errorf("Clash YAML 只包含占位/信息节点，没有可用节点")
		}
		return nodesList, "clash-yaml", nil
	}

	text := string(body)
	format := "link-list"
	if decoded := node.Base64Decode(text); decoded != "" && decoded != text {
		text = decoded
		format = "base64-link-list"
	}

	lines := strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' || r == ',' })
	valid := make([]models.Node, 0, len(lines))
	skipped := 0
	for _, link := range lines {
		link = strings.TrimSpace(link)
		if link == "" {
			continue
		}
		// http(s) 是订阅转换地址而非节点
		if strings.HasPrefix(link, "http://") || strings.HasPrefix(link, "https://") || !strings.Contains(link, "://") {
			skipped++
			continue
		}
		n := models.Node{Link: link}
		parsed, err := DocodeNodeName(&n)
		if err != nil || parsed.Name == "" || isPlaceholderNodeName(parsed.Name) {
			skipped++
			continue
		}
		valid = append(valid, parsed)
	}
	if len(valid) == 0 {
		return nil, format, fmt.Errorf("上游返回 %d 行内容，未解析出有效节点（跳过 %d 行），请检查订阅地址", len(lines), skipped)
	}
	return valid, format, nil
}

// isPlaceholderNodeName 判断节点名是否为信息/占位节点（官网、到期、剩余流量等）。
func isPlaceholderNodeName(name string) bool {
	lower := strings.ToLower(name)
	for _, term := range defaultPlaceholderTerms {
		if term != "" && strings.Contains(lower, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

// parseSubscriptionUserinfo 解析 "upload=1; download=2; total=3; expire=4"。
// ok 为 false 表示头缺失或没有任何可识别字段。
func parseSubscriptionUserinfo(header string) (upload, download, total int64, expire *time.Time, ok bool) {
	for _, part := range strings.Split(header, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(part), "=")
		if !found {
			continue
		}
		n, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil || n < 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "upload":
			upload, ok = int64(n), true
		case "download":
			download, ok = int64(n), true
		case "total":
			total, ok = int64(n), true
		case "expire":
			if n > 0 {
				t := time.Unix(int64(n), 0)
				expire = &t
			}
			ok = true
		}
	}
	return
}

// sourcePull 一次成功拉取的结果。
type sourcePull struct {
	Nodes    []models.Node
	Userinfo string
	Format   string
}

// pullSubscriptionSource 拉取并解析上游，不写库。
func pullSubscriptionSource(raw string) (sourcePull, error) {
	body, header, err := fetchSubscriptionSource(raw)
	if err != nil {
		return sourcePull{}, err
	}
	nodes, format, err := nodesFromSubscriptionBody(body)
	if err != nil {
		return sourcePull{}, fmt.Errorf("解析失败（%s）: %w", format, err)
	}
	return sourcePull{Nodes: nodes, Userinfo: header.Get("Subscription-Userinfo"), Format: format}, nil
}

// sourceMatchKey 识别“同一个节点”：协议 + 服务器 + 端口，换名不换 ID；解析不了时退回链接。
func sourceMatchKey(n models.Node) string {
	if p, err := node.ParseOutbound(n.Link); err == nil && p.Server != "" {
		return fmt.Sprintf("%s|%s:%d", p.Type, strings.ToLower(p.Server), p.Port)
	}
	return n.Link
}

// applySubscriptionSource 把拉取结果写入订阅：节点放进与订阅同名的分组并替换订阅节点，
// 沿用匹配到的旧节点记录（保留 ID 与其他引用），上游已移除且不再被引用的节点会被删除。
func applySubscriptionSource(subID int, pull sourcePull) error {
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var sub models.Subcription
		if err := tx.Preload("Nodes").First(&sub, subID).Error; err != nil {
			return err
		}
		// Association.Replace 会改写 sub.Nodes，先留一份旧节点用于清理
		oldNodes := append([]models.Node(nil), sub.Nodes...)
		previous := map[string]models.Node{}
		for _, n := range oldNodes {
			previous[sourceMatchKey(n)] = n
		}
		var gn models.GroupNode
		if err := tx.Where("name = ?", sub.Name).FirstOrCreate(&gn, models.GroupNode{Name: sub.Name}).Error; err != nil {
			return err
		}
		kept := map[int]bool{}
		list := make([]models.Node, 0, len(pull.Nodes))
		names := make([]string, 0, len(pull.Nodes))
		for _, n := range pull.Nodes {
			key := sourceMatchKey(n)
			if prev, ok := previous[key]; ok && !kept[prev.ID] {
				if prev.Name != n.Name || prev.Link != n.Link {
					if err := tx.Model(&prev).Updates(map[string]any{"name": n.Name, "link": n.Link}).Error; err != nil {
						return err
					}
					prev.Name, prev.Link = n.Name, n.Link
				}
				n = prev
			} else {
				n.ID = 0
				if err := tx.Create(&n).Error; err != nil {
					return err
				}
			}
			if kept[n.ID] {
				continue
			}
			kept[n.ID] = true
			list = append(list, n)
			names = append(names, n.Name)
		}
		if err := tx.Model(&gn).Association("Nodes").Replace(list); err != nil {
			return err
		}
		if err := tx.Model(&sub).Association("Nodes").Replace(list); err != nil {
			return err
		}
		now := time.Now()
		updates := map[string]any{
			"node_order":         strings.Join(names, ","),
			"last_good_snapshot": strings.Join(names, "\n"),
			"last_sync_at":       &now,
			"last_sync_error":    "",
		}
		if up, down, total, expire, ok := parseSubscriptionUserinfo(pull.Userinfo); ok {
			updates["source_upload"], updates["source_download"], updates["source_total"] = up, down, total
			updates["source_expire"], updates["source_traffic_at"] = expire, &now
		}
		if err := tx.Model(&models.Subcription{}).Where("id = ?", sub.ID).Updates(updates).Error; err != nil {
			return err
		}
		// 上游移除的节点：不在任何分组、也没有其他订阅引用时删除
		for _, old := range oldNodes {
			if kept[old.ID] {
				continue
			}
			var groups, subs int64
			tx.Table("group_node_nodes").Where("node_id = ?", old.ID).Count(&groups)
			tx.Table("subcription_nodes").Where("node_id = ?", old.ID).Count(&subs)
			if groups == 0 && subs == 0 {
				if err := tx.Delete(&models.Node{}, old.ID).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err == nil {
		InvalidateOverview()
	}
	return err
}

// SyncSubscriptionSource 立即刷新一个订阅的上游；失败时记录错误并继续使用上次的节点。
func SyncSubscriptionSource(subID int) (int, error) {
	var sub models.Subcription
	if err := models.DB.First(&sub, subID).Error; err != nil {
		return 0, err
	}
	if strings.TrimSpace(sub.SourceURLs) == "" {
		return 0, fmt.Errorf("订阅 %s 没有上游来源", sub.Name)
	}
	pull, err := pullSubscriptionSource(sub.SourceURLs)
	if err == nil {
		err = applySubscriptionSource(sub.ID, pull)
	}
	if err != nil {
		models.DB.Model(&models.Subcription{}).Where("id = ?", sub.ID).Update("last_sync_error", err.Error())
		return 0, err
	}
	return len(pull.Nodes), nil
}

// SyncDueSubscriptionSources 刷新超过间隔未同步的上游订阅（由每小时的定时任务调用）。
func SyncDueSubscriptionSources() {
	var subs []models.Subcription
	if err := models.DB.Select("id", "name", "last_sync_at").
		Where("source_urls IS NOT NULL AND TRIM(source_urls) <> ''").Find(&subs).Error; err != nil {
		return
	}
	for _, s := range subs {
		if s.LastSyncAt != nil && time.Since(*s.LastSyncAt) < sourceSyncInterval {
			continue
		}
		if _, err := SyncSubscriptionSource(s.ID); err != nil {
			log.Printf("[Cron] 订阅 %s 上游同步失败: %v", s.Name, err)
		}
	}
}

// removeSubscriptionSource 删除上游订阅时一并清理它的同名分组，以及不再被其他分组或订阅引用的节点。
func removeSubscriptionSource(name string) {
	var gn models.GroupNode
	if err := models.DB.Preload("Nodes").Where("name = ?", name).First(&gn).Error; err != nil {
		return
	}
	nodes := append([]models.Node(nil), gn.Nodes...)
	_ = models.DB.Model(&gn).Association("Nodes").Clear()
	models.DB.Delete(&gn)
	for _, n := range nodes {
		var groups, subs int64
		models.DB.Table("group_node_nodes").Where("node_id = ?", n.ID).Count(&groups)
		models.DB.Table("subcription_nodes").Where("node_id = ?", n.ID).Count(&subs)
		if groups == 0 && subs == 0 {
			models.DB.Delete(&models.Node{}, n.ID)
		}
	}
	InvalidateOverview()
}

// SubSyncSource 手动刷新上游订阅。POST /api/v1/subcription/sync-source?id=
func SubSyncSource(c *gin.Context) {
	id, _ := strconv.Atoi(c.Query("id"))
	n, err := SyncSubscriptionSource(id)
	if err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "同步失败，继续使用上次的节点: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"nodes": n}, "msg": fmt.Sprintf("已同步 %d 个节点", n)})
}
