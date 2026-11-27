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

func StartCronTasks() *cron.Cron {
	c := cron.New(cron.WithSeconds()) // 支持秒级

	// 每天凌晨 3:00 清理历史数据并优化 SQLite 统计信息
	_, err := c.AddFunc("0 0 3 * * *", func() {
		defer utils.RecoverPanic("cron-cleanup")
		if err := models.CleanupNodeQuality(time.Now().Add(-7 * 24 * time.Hour)); err != nil {
			log.Println("[Cron] 清理节点质量历史失败:", err)
		}
		if err := models.CleanupProbeResults(time.Now().Add(-24 * time.Hour)); err != nil {
			log.Println("[Cron] 清理探针结果失败:", err)
		}
		if err := models.CleanupMonitorMetrics(time.Now().Add(-7 * 24 * time.Hour)); err != nil {
			log.Println("[Cron] 清理监控指标失败:", err)
		}
		models.CleanupMaintenance(time.Now())
		models.OptimizeSqlite()
	})

	if err != nil {
		log.Println("[Cron] 添加定时任务失败:", err)
		return c
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
		return c
	}

	_, err = c.AddFunc("0 */10 * * * *", func() {
		defer utils.RecoverPanic("cron-node-quality")
		if _, err := api.CollectNodeQuality(); err != nil {
			log.Println("[Cron] 节点质量检测失败:", err)
		}
	})
	if err != nil {
		log.Println("[Cron] 添加节点质量任务失败:", err)
		return c
	}

	_, err = c.AddFunc("0 0 9 * * *", func() {
		defer utils.RecoverPanic("cron-sub-alerts")
		api.CheckSubscriptionAlerts()
	})
	if err != nil {
		log.Println("[Cron] 添加订阅告警任务失败:", err)
		return c
	}

	// 每天凌晨 4:00 检查并执行 WebDAV 自动云备份
	_, err = c.AddFunc("0 0 4 * * *", func() {
		defer utils.RecoverPanic("cron-webdav-backup")
		api.RunScheduledWebDAVBackup()
	})
	if err != nil {
		log.Println("[Cron] 添加 WebDAV 云备份任务失败:", err)
		return c
	}

	// 每 12 小时（04:15 / 16:15）错峰巡检在线服务器原生解锁状态，退化节点自动摘除智能分流组
	_, err = c.AddFunc("0 15 4,16 * * *", func() {
		defer utils.RecoverPanic("cron-unlock-sweep")
		api.RunScheduledUnlockSweep()
	})
	if err != nil {
		log.Println("[Cron] 添加定时解锁巡检任务失败:", err)
		return c
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
	log.Println("[Cron] 规则同步与节点每10分钟 TCP 质量检测已启动")
	return c
}
