package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
	"ppeelink/models"
	"ppeelink/node"
)

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// validateSubscriptionContent 对生成结果做结构校验，不通过时回退到上一次成功的版本。
func validateSubscriptionContent(client string, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("订阅产物为空")
	}
	switch strings.ToLower(client) {
	case "clash", "clash-proxies":
		var root map[string]any
		if err := yaml.Unmarshal(body, &root); err != nil {
			return fmt.Errorf("Clash YAML 无效: %w", err)
		}
		if len(root) == 0 {
			return fmt.Errorf("Clash YAML 为空")
		}
	case "surge", "loon":
		if !strings.Contains(string(body), "[") {
			return fmt.Errorf("配置缺少段落")
		}
	case "v2ray", "shadowrocket":
		decoded := node.Base64Decode(string(body))
		if decoded == "" || !strings.Contains(decoded, "://") {
			return fmt.Errorf("V2Ray 订阅未解析到节点")
		}
	case "singbox":
		var root map[string]any
		if err := json.Unmarshal(body, &root); err != nil {
			return fmt.Errorf("sing-box JSON 无效: %w", err)
		}
		if _, ok := root["outbounds"]; !ok {
			return fmt.Errorf("sing-box 配置缺少 outbounds")
		}
	case "qx":
		if !strings.Contains(string(body), "=") {
			return fmt.Errorf("Quantumult X 节点列表为空")
		}
	default:
		return fmt.Errorf("未知客户端 %s", client)
	}
	return nil
}

func lastKnownGoodArtifact(subscriptionID int, client string) (*models.SubscriptionArtifact, error) {
	var pointer models.SubscriptionArtifactPointer
	if err := models.DB.Where("subscription_id = ? AND client = ?", subscriptionID, strings.ToLower(client)).First(&pointer).Error; err != nil {
		return nil, err
	}
	var artifact models.SubscriptionArtifact
	if err := models.DB.First(&artifact, pointer.LastKnownGoodArtifactID).Error; err != nil {
		return nil, err
	}
	return &artifact, nil
}

// rememberGoodArtifact 保存本次成功生成的内容作为该客户端的回退版本；
// 内容未变时不写库，每个订阅每个客户端只保留最新一份。
func rememberGoodArtifact(subscriptionID int, client string, body []byte, deliveryDigest string) {
	client = strings.ToLower(client)
	checksum := shaHex(body)
	if current, err := lastKnownGoodArtifact(subscriptionID, client); err == nil && current.ContentChecksum == checksum && current.InputDigest == deliveryDigest {
		return
	}
	artifact := models.SubscriptionArtifact{
		SubscriptionID: subscriptionID, Client: client, ContentChecksum: checksum,
		InputDigest: deliveryDigest,
		ByteSize:    len(body), ValidationStatus: "valid", TestStatus: "passed",
		Content: append([]byte(nil), body...),
	}
	if err := models.DB.Create(&artifact).Error; err != nil {
		return
	}
	var pointer models.SubscriptionArtifactPointer
	if err := models.DB.Where("subscription_id = ? AND client = ?", subscriptionID, client).First(&pointer).Error; err == nil {
		_ = models.DB.Model(&pointer).Update("last_known_good_artifact_id", artifact.ID).Error
	} else {
		_ = models.DB.Create(&models.SubscriptionArtifactPointer{SubscriptionID: subscriptionID, Client: client, LastKnownGoodArtifactID: artifact.ID}).Error
	}
	_ = models.DB.Where("subscription_id = ? AND client = ? AND id <> ?", subscriptionID, client, artifact.ID).
		Delete(&models.SubscriptionArtifact{}).Error
}

// buildSubscriptionOutput 在内存中渲染订阅；providerBase 用于 proxy-provider 模式拼接节点地址。
func buildSubscriptionOutput(sub models.Subcription, client, providerBase string) ([]byte, error) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Set("subname", sub.Name)
	c.Set("subToken", sub.Token)
	c.Set("providerBase", providerBase)
	switch client {
	case "clash", "clash-proxies":
		c.Set("clashProxiesOnly", client == "clash-proxies")
		GetClash(c)
	case "surge":
		GetSurge(c)
	case "loon":
		GetLoon(c)
	case "v2ray", "shadowrocket":
		GetV2ray(c)
	case "singbox":
		GetSingbox(c)
	case "qx":
		GetQX(c)
	default:
		return nil, fmt.Errorf("不支持的客户端: %s", client)
	}
	body := recorder.Body.Bytes()
	if len(body) == 0 {
		return nil, fmt.Errorf("订阅生成结果为空")
	}
	if recorder.Code >= 400 {
		return nil, fmt.Errorf("订阅生成失败 HTTP %d", recorder.Code)
	}
	var appConfig node.SqlConfig
	if err := json.Unmarshal([]byte(sub.Config), &appConfig); err != nil {
		return nil, err
	}
	if client == "clash" || client == "singbox" {
		return node.ApplyApplicationRules(client, body, appConfig)
	}
	return append([]byte(nil), body...), nil
}

var clientFileExt = map[string]string{
	"clash": "yaml", "clash-proxies": "yaml", "surge": "conf", "loon": "conf",
	"v2ray": "txt", "shadowrocket": "txt", "singbox": "json", "qx": "conf",
}

func serveSubscriptionClient(c *gin.Context, sub models.Subcription, client string) {
	client = strings.ToLower(client)
	var disabled []int
	if err := models.DB.Model(&models.Node{}).Where("delivery_disabled = ?", true).Order("id").Pluck("id", &disabled).Error; err != nil {
		c.String(500, "读取节点下发设置失败")
		return
	}
	deliveryDigest := ""
	if len(disabled) > 0 {
		raw, _ := json.Marshal(disabled)
		deliveryDigest = shaHex(raw)
	}
	var appConfig node.SqlConfig
	_ = json.Unmarshal([]byte(sub.Config), &appConfig)
	if len(appConfig.AppRules) > 0 && client != "clash" && client != "singbox" && client != "clash-proxies" {
		c.String(400, "此订阅包含应用规则，请使用 Mihomo 或 sing-box 完整配置")
		return
	}

	body, err := buildSubscriptionOutput(sub, client, baseURL(c))
	if err == nil {
		err = validateSubscriptionContent(client, body)
	}
	liveValid := err == nil
	if !liveValid && len(appConfig.AppRules) > 0 {
		c.String(502, "应用规则生成失败，请检查目标节点是否仍在订阅中")
		return
	}
	if liveValid {
		rememberGoodArtifact(sub.ID, client, body, deliveryDigest)
	} else if artifact, lkgErr := lastKnownGoodArtifact(sub.ID, client); lkgErr == nil && len(artifact.Content) > 0 && artifact.InputDigest == deliveryDigest {
		body, err = artifact.Content, nil
		c.Header("X-SubLinkX-Artifact", "last-known-good")
	}
	if err != nil || len(body) == 0 {
		c.String(502, "订阅生成失败，且没有可用的上一版本")
		return
	}
	if info := subscriptionUserinfo(sub); info != "" {
		c.Header("Subscription-Userinfo", info)
	}
	c.Header("Profile-Update-Interval", "24")
	contentType := "text/plain; charset=utf-8"
	if client == "singbox" {
		contentType = "application/json; charset=utf-8"
	}
	filename := sub.Name + "." + clientFileExt[client]
	c.Header("Content-Disposition", "inline; filename*=utf-8''"+url.QueryEscape(filename))
	c.Data(200, contentType, body)
}
