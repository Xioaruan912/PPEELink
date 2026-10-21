package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"ppeelink/models"
	"ppeelink/utils"

	"github.com/gin-gonic/gin"
)

func currentAlertSetting() models.AlertSetting {
	var setting models.AlertSetting
	if err := models.DB.First(&setting).Error; err != nil {
		setting = models.AlertSetting{FailureThreshold: 3, AutoBlockInvalid: true}
		models.DB.Create(&setting)
	}
	if setting.FailureThreshold < 1 {
		setting.FailureThreshold = 3
	}
	return setting
}

func inMaintenance(setting models.AlertSetting, now time.Time) bool {
	if setting.MaintenanceStart == "" || setting.MaintenanceEnd == "" {
		return false
	}
	current := now.Format("15:04")
	if setting.MaintenanceStart <= setting.MaintenanceEnd {
		return current >= setting.MaintenanceStart && current <= setting.MaintenanceEnd
	}
	return current >= setting.MaintenanceStart || current <= setting.MaintenanceEnd
}

func notifyHealthEvent(setting models.AlertSetting, event models.NodeHealthEvent) {
	if !setting.Enabled || setting.WebhookURL == "" || inMaintenance(setting, time.Now()) {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{"event": event.Type, "node": event.NodeName, "message": event.Message, "time": event.CreatedAt})
	go func() {
		defer utils.RecoverPanic("health-webhook")
		req, err := http.NewRequest("POST", setting.WebhookURL, bytes.NewReader(payload))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: 8 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
}

func processNodeHealthEvents(items []NodeOverviewItem) {
	setting := currentAlertSetting()
	if !setting.AutoBlockInvalid {
		return
	}
	for _, item := range items {
		var recent []models.NodeQualitySample
		models.DB.Where("node_id = ?", item.ID).Order("checked_at desc").Limit(setting.FailureThreshold).Find(&recent)
		var last models.NodeHealthEvent
		models.DB.Where("node_id = ?", item.ID).Order("id desc").Limit(1).Find(&last)
		allFailed := len(recent) >= setting.FailureThreshold
		for _, sample := range recent {
			if sample.Success {
				allFailed = false
				break
			}
		}
		var event *models.NodeHealthEvent
		if allFailed && last.Type != "down" {
			e := models.NodeHealthEvent{NodeID: item.ID, NodeName: item.Name, Type: "down", Message: fmt.Sprintf("连续 %d 次连接失败", setting.FailureThreshold), CreatedAt: time.Now()}
			event = &e
		} else if item.Valid && last.Type == "down" {
			e := models.NodeHealthEvent{NodeID: item.ID, NodeName: item.Name, Type: "recovery", Message: "节点恢复可用", CreatedAt: time.Now()}
			event = &e
		}
		if event != nil {
			models.DB.Create(event)
			notifyHealthEvent(setting, *event)
		}
	}
}

// releaseStaleHealthBlocks 给最近一次事件为 down 的节点补一条 recovery，
// 用于重新开启自动判定时让旧失败状态失效。
func releaseStaleHealthBlocks() {
	var downs []models.NodeHealthEvent
	models.DB.Where("id IN (?)", models.DB.Model(&models.NodeHealthEvent{}).Select("MAX(id)").Group("node_id")).
		Where("type = ?", "down").Find(&downs)
	if len(downs) == 0 {
		return
	}
	events := make([]models.NodeHealthEvent, 0, len(downs))
	now := time.Now()
	for _, e := range downs {
		events = append(events, models.NodeHealthEvent{
			NodeID: e.NodeID, NodeName: e.NodeName, Type: "recovery",
			Message: "已重新开启自动判定，等待下次检测", CreatedAt: now,
		})
	}
	models.DB.CreateInBatches(events, 100)
}

func NodeHealthEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	var events []models.NodeHealthEvent
	q := models.DB.Order("created_at desc").Limit(limit)
	if id := c.Query("id"); id != "" {
		q = q.Where("node_id = ?", id)
	}
	if err := q.Find(&events).Error; err != nil {
		c.JSON(500, gin.H{"msg": "读取健康事件失败"})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": events, "msg": "健康事件"})
}

func GetAlertSetting(c *gin.Context) {
	c.JSON(200, gin.H{"code": "00000", "data": currentAlertSetting(), "msg": "告警设置"})
}

func UpdateAlertSetting(c *gin.Context) {
	setting := currentAlertSetting()
	previousAutoBlock := setting.AutoBlockInvalid
	if v, ok := c.GetPostForm("enabled"); ok {
		setting.Enabled = v == "true"
	}
	if v, ok := c.GetPostForm("webhookUrl"); ok {
		setting.WebhookURL = v
	}
	if v, ok := c.GetPostForm("maintenanceStart"); ok {
		setting.MaintenanceStart = v
	}
	if v, ok := c.GetPostForm("maintenanceEnd"); ok {
		setting.MaintenanceEnd = v
	}
	if n, err := strconv.Atoi(c.PostForm("failureThreshold")); err == nil && n >= 1 && n <= 20 {
		setting.FailureThreshold = n
	}
	if n, err := strconv.Atoi(c.PostForm("expiryReminderDays")); err == nil && n >= 0 && n <= 90 {
		setting.ExpiryReminderDays = n
	}
	if v, ok := c.GetPostForm("autoDisableExpired"); ok {
		setting.AutoDisableExpired = v == "true"
	}
	if n, err := strconv.Atoi(c.PostForm("accessLimit")); err == nil && n >= 0 {
		setting.AccessLimit = n
	}
	if v, ok := c.GetPostForm("autoDisableOverLimit"); ok {
		setting.AutoDisableOverLimit = v == "true"
	}
	if v, ok := c.GetPostForm("autoBlockInvalid"); ok {
		setting.AutoBlockInvalid = v == "true"
	}
	if err := models.DB.Save(&setting).Error; err != nil {
		c.JSON(500, gin.H{"msg": "保存告警设置失败"})
		return
	}
	// 重新开启自动判定时清掉旧的 down 状态，让每个节点先通过一次新检测，
	// 避免关闭期间积累的事件在开启瞬间把节点全部排除。
	if !previousAutoBlock && setting.AutoBlockInvalid {
		releaseStaleHealthBlocks()
	}
	c.JSON(200, gin.H{"code": "00000", "msg": "告警设置已保存"})
}
