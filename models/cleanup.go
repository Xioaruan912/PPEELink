package models

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CleanupMaintenance 按 7 天上限清理历史表和文件日志，供后台排查使用同时避免数据无限膨胀。
func CleanupMaintenance(now time.Time) {
	sevenDays := 7 * 24 * time.Hour
	before := now.Add(-sevenDays)

	type job struct {
		name string
		run  func() (int64, error)
	}
	jobs := []job{
		{"node_health_events", func() (int64, error) {
			tx := DB.Where("created_at < ?", before).Delete(&NodeHealthEvent{})
			return tx.RowsAffected, tx.Error
		}},
		{"unlock_observations", func() (int64, error) {
			tx := DB.Where("checked_at < ?", before).Delete(&UnlockObservation{})
			return tx.RowsAffected, tx.Error
		}},
		{"audit_logs", func() (int64, error) {
			tx := DB.Where("created_at < ?", before).Delete(&AuditLog{})
			return tx.RowsAffected, tx.Error
		}},
		{"task_runs", func() (int64, error) {
			tx := DB.Where("created_at < ? AND status IN ?", before, []string{"success", "failed", "cancelled"}).Delete(&TaskRun{})
			return tx.RowsAffected, tx.Error
		}},
		{"sub_logs", func() (int64, error) {
			tx := DB.Where("date < ?", before.Format("2006-01-02")).Delete(&SubLogs{})
			return tx.RowsAffected, tx.Error
		}},
	}
	for _, j := range jobs {
		deleted, err := j.run()
		if err != nil {
			log.Printf("[Cleanup] %s 清理失败: %v", j.name, err)
			continue
		}
		if deleted > 0 {
			log.Printf("[Cleanup] %s 清理 %d 行 (7天上限)", j.name, deleted)
		}
	}

	cleanupDiskLogFiles(before)
}

func cleanupDiskLogFiles(before time.Time) {
	logDir := "./logs"
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		path := filepath.Join(logDir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(before) {
			if err := os.Remove(path); err == nil {
				log.Printf("[Cleanup] 删除超 7 天文件日志: %s", e.Name())
			}
		}
	}
}
