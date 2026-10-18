package main

import (
	"context"
	"log"
	"ppeelink/api"
	"ppeelink/models"
	"ppeelink/rulecenter"
	"ppeelink/utils"
	"time"

	"github.com/robfig/cron/v3"
)

func StartCronTasks() {
	c := cron.New(cron.WithSeconds()) // 支持秒级

	// 每天凌晨 3:00 跑一次机场节点拉取与测活清理
	_, err := c.AddFunc("0 0 3 * * *", func() {
		defer utils.RecoverPanic("cron-airport-sync")
		api.SyncAllAirports()
		if err := models.CleanupNodeQuality(time.Now().Add(-7 * 24 * time.Hour)); err != nil {
			log.Println("[Cron] 清理节点质量历史失败:", err)
		}
		models.CleanupMaintenance(time.Now())
	})

	if err != nil {
		log.Println("[Cron] 添加定时任务失败:", err)
		return
	}
	_, err = c.AddFunc("0 30 3 * * *", func() {
		defer utils.RecoverPanic("cron-rule-sync")
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := rulecenter.SyncAll(ctx); err != nil {
			log.Println("[Cron] 规则中心同步失败:", err)
		}
	})
	if err != nil {
		log.Println("[Cron] 添加规则中心同步任务失败:", err)
		return
	}

	_, err = c.AddFunc("0 */10 * * * *", func() {
		defer utils.RecoverPanic("cron-node-quality")
		if _, err := api.CollectNodeQuality(); err != nil {
			log.Println("[Cron] 节点质量检测失败:", err)
		}
	})
	if err != nil {
		log.Println("[Cron] 添加节点质量任务失败:", err)
		return
	}

	_, err = c.AddFunc("0 15 * * * *", func() {
		defer utils.RecoverPanic("cron-airport-interval")
		api.SyncDueAirports()
	})
	if err != nil {
		log.Println("[Cron] 添加机场间隔同步任务失败:", err)
		return
	}

	_, err = c.AddFunc("0 0 9 * * *", func() {
		defer utils.RecoverPanic("cron-sub-alerts")
		api.CheckSubscriptionAlerts()
	})
	if err != nil {
		log.Println("[Cron] 添加订阅告警任务失败:", err)
		return
	}

	c.Start()
	go func() {
		defer utils.RecoverPanic("startup-rule-sync")
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := rulecenter.SyncAll(ctx); err != nil {
			log.Println("[RuleCenter] 启动同步失败，保留现有缓存:", err)
		}
	}()
	log.Println("[Cron] 机场/规则同步、机场间隔同步与节点每10分钟 TCP 质量检测已启动")
}
