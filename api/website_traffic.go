package api

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/idna"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"ppeelink/models"
	"ppeelink/xraycore"
)

func websiteDomain(raw string) string {
	raw = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(raw)), ".")
	if raw == "" || raw == "unknown" || raw == "未知" || net.ParseIP(raw) != nil {
		return "unknown"
	}
	ascii, err := idna.Lookup.ToASCII(raw)
	if err != nil || len(ascii) > 253 || strings.ContainsAny(ascii, ":/\\@?# ") {
		return "unknown"
	}
	for _, label := range strings.Split(ascii, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "unknown"
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
				return "unknown"
			}
		}
	}
	return ascii
}

// The receipt and increments share the surrounding status-report transaction.
// Retransmitting the same batch after an ambiguous HTTP failure cannot double count.
func applyWebsiteTraffic(tx *gorm.DB, serverID uint, report xraycore.Report, now time.Time) error {
	if len(report.WebsiteTraffic) == 0 {
		return nil
	}
	if len(report.WebsiteTraffic) > 8192 || len(report.WebsiteBatchID) < 16 || len(report.WebsiteBatchID) > 64 {
		return fmt.Errorf("invalid website traffic batch")
	}
	for _, c := range report.WebsiteBatchID {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return fmt.Errorf("invalid website batch ID")
		}
	}
	for _, b := range report.WebsiteTraffic {
		if b.Up < 0 || b.Down < 0 || b.Up > 1<<50 || b.Down > 1<<50 || b.BucketStart <= 0 || b.BucketStart%3600 != 0 || b.BucketStart > now.UTC().Truncate(time.Hour).Unix() {
			return fmt.Errorf("invalid website traffic bucket")
		}
	}
	receipt := models.WebsiteTrafficReceipt{ServerID: serverID, BatchID: report.WebsiteBatchID, CreatedAt: now}
	res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return nil
	}
	for _, b := range report.WebsiteTraffic {
		if b.Up == 0 && b.Down == 0 {
			continue
		}
		row := models.WebsiteTraffic{ServerID: serverID, BucketStart: time.Unix(b.BucketStart, 0).UTC(), Domain: websiteDomain(b.Domain), Up: b.Up, Down: b.Down}
		err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "server_id"}, {Name: "bucket_start"}, {Name: "domain"}}, DoUpdates: clause.Assignments(map[string]any{"up": gorm.Expr("up + excluded.up"), "down": gorm.Expr("down + excluded.down")})}).Create(&row).Error
		if err != nil {
			return err
		}
	}
	return nil
}

type websiteTrafficRow struct {
	ServerID   uint   `json:"serverId"`
	ServerName string `json:"serverName"`
	Domain     string `json:"domain"`
	Up         int64  `json:"up"`
	Down       int64  `json:"down"`
	Total      int64  `json:"total"`
	Rank       int    `json:"rank"`
}

// ProxyWebsiteTraffic is an administrator-only report, under the proxy API group.
// Ranges are [start,end), aligned to UTC hour boundaries; never silently round.
func ProxyWebsiteTraffic(c *gin.Context) {
	bad := func(msg string) { c.JSON(400, gin.H{"code": "40000", "msg": msg}) }
	now := time.Now().UTC()
	end := now.Truncate(time.Hour).Add(time.Hour)
	start := end.Add(-24 * time.Hour)
	var err error
	if raw := c.Query("start"); raw != "" {
		start, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			bad("开始时间需为带时区的 RFC3339 时间")
			return
		}
	}
	if raw := c.Query("end"); raw != "" {
		end, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			bad("结束时间需为带时区的 RFC3339 时间")
			return
		}
	}
	start, end = start.UTC(), end.UTC()
	if !start.Before(end) || end.Sub(start) > 366*24*time.Hour || !start.Equal(start.Truncate(time.Hour)) || !end.Equal(end.Truncate(time.Hour)) {
		bad("请选择整点时间，范围须大于零且不超过366天（结束时间不包含在内）")
		return
	}
	query := models.DB.Table("website_traffics AS w").Where("w.bucket_start >= ? AND w.bucket_start < ?", start, end)
	if raw := c.Query("serverId"); raw != "" {
		id, e := strconv.ParseUint(raw, 10, 32)
		if e != nil || id == 0 {
			bad("节点 ID 无效")
			return
		}
		query = query.Where("w.server_id = ?", id)
	}
	if raw := strings.TrimSpace(c.Query("domain")); raw != "" {
		domain := websiteDomain(raw)
		if domain == "unknown" && raw != "unknown" && raw != "未知" {
			bad("请输入域名而非完整网址，未知流量请使用 unknown")
			return
		}
		query = query.Where("w.domain = ?", domain)
	}
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			bad("排名数量须为1至100")
			return
		}
	}
	order := "total"
	if raw := c.Query("order"); raw != "" {
		if raw != "total" && raw != "up" && raw != "down" {
			bad("排序仅支持 total、up、down")
			return
		}
		order = raw
	}
	summary := struct {
		Up    int64 `json:"up"`
		Down  int64 `json:"down"`
		Total int64 `json:"total"`
	}{}
	rows := []websiteTrafficRow{}
	// Read both aggregates in one snapshot, so a concurrent report cannot disagree
	// with the summary shown above its ranking.
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		scoped := tx.Table("website_traffics AS w").Where(query.Statement.Clauses["WHERE"].Expression)
		if e := scoped.Session(&gorm.Session{}).Select("COALESCE(SUM(w.up),0) AS up, COALESCE(SUM(w.down),0) AS down, COALESCE(SUM(w.up+w.down),0) AS total").Scan(&summary).Error; e != nil {
			return e
		}
		return scoped.Session(&gorm.Session{}).Joins("LEFT JOIN proxy_servers AS s ON s.id = w.server_id").Select("w.server_id, COALESCE(s.name, '已删除节点') AS server_name, w.domain, SUM(w.up) AS up, SUM(w.down) AS down, SUM(w.up+w.down) AS total").Group("w.server_id, s.name, w.domain").Order(order + " DESC, w.server_id ASC, w.domain ASC").Limit(limit).Scan(&rows).Error
	})
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取网站流量失败，请重试"})
		return
	}
	for i := range rows {
		rows[i].Rank = i + 1
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"items": rows, "summary": summary, "start": start, "end": end, "granularity": "hour", "scope": "proxy", "unknownDomain": "unknown"}})
}
