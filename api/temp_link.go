package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ppeelink/middlewares"
	"ppeelink/models"
	"ppeelink/node"

	"github.com/gin-gonic/gin"
)

// 单节点临时订阅：链接自带节点 ID 与到期时间并经 HMAC 签名，无需落库，到期自动失效。

func signTempLink(nodeID int, expire int64) string {
	payload := fmt.Sprintf("%d.%d", nodeID, expire)
	mac := hmac.New(sha256.New, middlewares.Secret)
	mac.Write([]byte("temp-link:" + payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18])
	return payload + "." + sig
}

func parseTempLink(token string) (int, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return 0, time.Time{}, fmt.Errorf("链接格式错误")
	}
	nodeID, err1 := strconv.Atoi(parts[0])
	expire, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, time.Time{}, fmt.Errorf("链接格式错误")
	}
	if !hmac.Equal([]byte(signTempLink(nodeID, expire)), []byte(token)) {
		return 0, time.Time{}, fmt.Errorf("签名无效")
	}
	return nodeID, time.Unix(expire, 0), nil
}

// NodeTempLink 为单个节点生成限时订阅链接（1 小时到 30 天）。
func NodeTempLink(c *gin.Context) {
	var req struct {
		NodeID int `json:"nodeId"`
		Hours  int `json:"hours"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.NodeID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数错误"})
		return
	}
	if req.Hours < 1 || req.Hours > 720 {
		c.JSON(400, gin.H{"code": "40000", "msg": "有效期需在 1 到 720 小时之间"})
		return
	}
	var n models.Node
	if err := models.DB.First(&n, req.NodeID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "节点不存在"})
		return
	}
	expire := time.Now().Add(time.Duration(req.Hours) * time.Hour)
	token := signTempLink(n.ID, expire.Unix())
	base := baseURL(c) + "/t/?token=" + url.QueryEscape(token)
	links := map[string]string{}
	for _, cl := range []string{"clash", "v2ray", "shadowrocket", "singbox", "qx"} {
		links[cl] = base + "&client=" + cl
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"links": links, "expiresAt": expire, "name": n.Name}, "msg": "已生成临时订阅"})
}

// GetTempLink 公开下发单节点临时订阅。
func GetTempLink(c *gin.Context) {
	nodeID, expire, err := parseTempLink(c.Query("token"))
	if err != nil {
		c.String(404, "无效的临时订阅")
		return
	}
	if time.Now().After(expire) {
		c.String(410, "临时订阅已过期")
		return
	}
	var n models.Node
	if err := models.DB.First(&n, nodeID).Error; err != nil {
		c.String(404, "节点已删除")
		return
	}
	if n.DeliveryDisabled {
		c.String(403, "此节点已关闭订阅下发")
		return
	}
	client := normalizeClient(c.Query("client"))
	if client == "" {
		client = clientFromUserAgent(c.Request.UserAgent())
	}
	urls, flags := collectNodeInputs([]models.Node{n})
	var body []byte
	contentType := "text/plain; charset=utf-8"
	switch client {
	case "clash":
		body, err = node.EncodeClashMinimal(urls, flags)
	case "singbox":
		var s string
		s, err = node.EncodeSingbox(urls, node.SqlConfig{})
		body, contentType = []byte(s), "application/json; charset=utf-8"
	case "qx":
		var s string
		s, err = node.EncodeQX(urls, node.SqlConfig{})
		body = []byte(s)
	default:
		body = []byte(node.Base64Encode(strings.Join(urls, "\n")))
	}
	if err != nil || len(body) == 0 {
		c.String(502, "临时订阅生成失败")
		return
	}
	c.Header("Subscription-Userinfo", fmt.Sprintf("upload=0; download=0; total=0; expire=%d", expire.Unix()))
	c.Data(200, contentType, body)
}
