package api

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// 测速护栏：真实测速会经节点下载数据，消耗机场流量与服务器带宽。
// 这里限制单次规模、每日总量和发起频率，并拒绝把节点当跳板访问内网地址。
const (
	speedMaxNodesPerRun  = 30
	speedMaxBytesPerRun  = int64(1 << 30) // 单次最多 1 GiB（节点数 × 单节点下载量）
	speedDailyBudget     = int64(5 << 30) // 每日最多 5 GiB，按实际下载字节计
	speedCooldown        = 30 * time.Second
	speedBudgetResetHour = 0 // 本地时间 0 点重置
)

type speedBudget struct {
	mu       sync.Mutex
	day      string
	used     int64
	lastStop time.Time
}

var speedGuard = &speedBudget{}

func (b *speedBudget) rollover(now time.Time) {
	day := now.Add(-time.Duration(speedBudgetResetHour) * time.Hour).Format("2006-01-02")
	if b.day != day {
		b.day, b.used = day, 0
	}
}

// snapshot 返回今日已用与剩余字节，以及距离可再次发起的等待时间。
func (b *speedBudget) snapshot(now time.Time) (used, remaining int64, wait time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rollover(now)
	if until := b.lastStop.Add(speedCooldown); now.Before(until) {
		wait = until.Sub(now)
	}
	remaining = speedDailyBudget - b.used
	if remaining < 0 {
		remaining = 0
	}
	return b.used, remaining, wait
}

// add 记录实际下载量（失败的测速也可能已下载了部分数据）。
func (b *speedBudget) add(n int64) {
	if n <= 0 {
		return
	}
	b.mu.Lock()
	b.rollover(time.Now())
	b.used += n
	b.mu.Unlock()
}

func (b *speedBudget) finish() {
	b.mu.Lock()
	b.lastStop = time.Now()
	b.mu.Unlock()
}

// checkSpeedPlan 校验一次测速计划，返回给用户看的拒绝原因（为空表示放行）。
func checkSpeedPlan(nodes int, perNode int64, now time.Time) string {
	if nodes > speedMaxNodesPerRun {
		return fmt.Sprintf("单次最多测 %d 个节点，请分批测速", speedMaxNodesPerRun)
	}
	planned := int64(nodes) * perNode
	if planned > speedMaxBytesPerRun {
		return fmt.Sprintf("本次预计下载 %s，超过单次上限 %s，请减少节点或下载量",
			formatSize(planned), formatSize(speedMaxBytesPerRun))
	}
	_, remaining, wait := speedGuard.snapshot(now)
	if wait > 0 {
		return fmt.Sprintf("测速过于频繁，请 %d 秒后再试", int(wait.Seconds())+1)
	}
	if planned > remaining {
		return fmt.Sprintf("今日测速额度剩余 %s，不足本次预计的 %s", formatSize(remaining), formatSize(planned))
	}
	return ""
}

// validateSpeedTarget 只允许 http(s)，并拒绝指向回环、内网与链路本地地址的目标。
func validateSpeedTarget(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("测速地址必须是 http(s) 链接")
	}
	host := strings.ToLower(u.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal") {
		return fmt.Errorf("测速地址不能指向本机或内网")
	}
	if ip := net.ParseIP(host); ip != nil &&
		(ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return fmt.Errorf("测速地址不能指向本机或内网")
	}
	return nil
}

func formatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.0f MB", float64(n)/(1<<20))
	default:
		return fmt.Sprintf("%d KB", n>>10)
	}
}

// SpeedTestBudget 返回测速限额，供前端在发起前提示预计消耗。
// GET /api/v1/nodes/speedtest/budget
func SpeedTestBudget(c *gin.Context) {
	used, remaining, wait := speedGuard.snapshot(time.Now())
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{
		"used":            used,
		"remaining":       remaining,
		"daily":           speedDailyBudget,
		"maxNodes":        speedMaxNodesPerRun,
		"maxBytesPerRun":  speedMaxBytesPerRun,
		"cooldownSeconds": int(wait.Seconds() + 0.999),
	}, "msg": "测速额度"})
}
