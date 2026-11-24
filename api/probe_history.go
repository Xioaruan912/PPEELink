package api

import (
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

const probeHistoryRetention = 7 * 24 * time.Hour

var startProbeSchedulerOnce sync.Once

// StartUserProbeScheduler runs a lightweight 1-minute scheduler for User Agents
// that have opted into continuous telemetry recording (AutoProbe=true), and
// prunes ProbeResult records older than 7 days.
func StartUserProbeScheduler() {
	startProbeSchedulerOnce.Do(func() {
		go func() {
			// Initial 7-day retention cleanup.
			if err := models.CleanupProbeResults(time.Now().Add(-probeHistoryRetention)); err != nil {
				log.Printf("[用户 Agent 调度] 清理历史记录失败: %v", err)
			}

			probeTicker := time.NewTicker(60 * time.Second)
			cleanupTicker := time.NewTicker(6 * time.Hour)
			defer probeTicker.Stop()
			defer cleanupTicker.Stop()

			for {
				select {
				case <-probeTicker.C:
					dispatchScheduledUserProbes()
				case <-cleanupTicker.C:
					_ = models.CleanupProbeResults(time.Now().Add(-probeHistoryRetention))
				}
			}
		}()
	})
}

func dispatchScheduledUserProbes() {
	probes, err := models.GetProbes(true)
	if err != nil || len(probes) == 0 {
		return
	}
	var active []models.Probe
	for _, p := range probes {
		if p.AutoProbe && isProbeRecentlyOnline(&p) {
			active = append(active, p)
		}
	}
	if len(active) == 0 {
		return
	}
	nodes, err := buildProbeNodes(nil)
	if err != nil || len(nodes) == 0 {
		return
	}
	for _, p := range active {
		jobID, doneCh, ok := probeHub.dispatch(p.ID, nodes, false)
		if !ok {
			continue
		}
		go func(id string, ch chan jobOutcome) {
			defer probeHub.cleanupDone(id)
			select {
			case <-ch:
			case <-time.After(35 * time.Second):
			}
		}(jobID, doneCh)
	}
}

// ProbeAutoToggle 管理端：开关单个用户 Agent 的 1 分钟定时记录（保留 7 天曲线）。
func ProbeAutoToggle(c *gin.Context) {
	var req struct {
		ID        uint `json:"id"`
		AutoProbe bool `json:"autoProbe"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ID == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "参数无效"})
		return
	}
	var probe models.Probe
	if err := models.DB.First(&probe, req.ID).Error; err != nil {
		c.JSON(404, gin.H{"code": "40400", "msg": "用户 Agent 不存在"})
		return
	}
	if err := models.DB.Model(&probe).Update("auto_probe", req.AutoProbe).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "保存失败"})
		return
	}
	// Turning on while online: fire one immediate sample asynchronously so charts populate right away.
	if req.AutoProbe && isProbeRecentlyOnline(&probe) {
		go func(probeID uint) {
			nodes, err := buildProbeNodes(nil)
			if err != nil || len(nodes) == 0 {
				return
			}
			if jobID, doneCh, ok := probeHub.dispatch(probeID, nodes, true); ok {
				defer probeHub.cleanupDone(jobID)
				select {
				case <-doneCh:
				case <-time.After(25 * time.Second):
				}
			}
		}(probe.ID)
	}
	msg := "已关闭 1 分钟定时记录（恢复纯按需模式）"
	if req.AutoProbe {
		msg = "已开启 1 分钟定时记录（自动保留 7 天曲线）"
	}
	c.JSON(200, gin.H{"code": "00000", "data": gin.H{"id": probe.ID, "autoProbe": req.AutoProbe}, "msg": msg})
}

type historyPoint struct {
	T    int64   `json:"t"`             // Unix ms
	RTT  int     `json:"rtt"`           // Avg RTT ms (-1 if all failed in bucket)
	Min  int     `json:"min,omitempty"` // Min RTT ms
	Max  int     `json:"max,omitempty"` // Max RTT ms
	OK   bool    `json:"ok"`
	Loss float64 `json:"loss"` // 0..100 %
}

type historyStats struct {
	Current    *int    `json:"current,omitempty"`
	Avg        int     `json:"avg"`
	Min        int     `json:"min"`
	Max        int     `json:"max"`
	Jitter     int     `json:"jitter"`
	PeakAvg    int     `json:"peakAvg,omitempty"`
	OffPeakAvg int     `json:"offPeakAvg,omitempty"`
	PeakUptime float64 `json:"peakUptime,omitempty"`
	Uptime     float64 `json:"uptime"`
	Samples    int     `json:"samples"`
}

type bestPeakSummary struct {
	NodeID   int     `json:"nodeId"`
	NodeName string  `json:"nodeName"`
	Avg      int     `json:"avg"`
	PeakAvg  int     `json:"peakAvg,omitempty"`
	Jitter   int     `json:"jitter"`
	Uptime   float64 `json:"uptime"`
}

type sparklineSeries struct {
	Points []historyPoint `json:"points"`
	Stats  historyStats   `json:"stats"`
}

type probeHistorySeries struct {
	ProbeID   uint           `json:"probeId"`
	ProbeName string         `json:"probeName"`
	Region    string         `json:"region"`
	Stats     historyStats   `json:"stats"`
	Points    []historyPoint `json:"points"`
}

type peakBand struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

// computePeakBands returns Asia/Shanghai evening peak windows (20:00–23:30)
// intersecting [from, to] so charts can subtly shade peak congestion hours.
func computePeakBands(from, to time.Time) []peakBand {
	loc := time.Local
	if sh, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		loc = sh
	}
	startLocal := from.In(loc)
	endLocal := to.In(loc)
	day := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, loc)
	var bands []peakBand
	for !day.After(endLocal) {
		pStart := day.Add(20 * time.Hour)
		pEnd := day.Add(23*time.Hour + 30*time.Minute)
		if pEnd.After(startLocal) && pStart.Before(endLocal) {
			clampedStart := pStart
			if clampedStart.Before(startLocal) {
				clampedStart = startLocal
			}
			clampedEnd := pEnd
			if clampedEnd.After(endLocal) {
				clampedEnd = endLocal
			}
			bands = append(bands, peakBand{
				Start: clampedStart.UnixMilli(),
				End:   clampedEnd.UnixMilli(),
			})
		}
		day = day.AddDate(0, 0, 1)
	}
	return bands
}

func isEveningPeak(t time.Time) bool {
	loc := time.Local
	if sh, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		loc = sh
	}
	lt := t.In(loc)
	mins := lt.Hour()*60 + lt.Minute()
	return mins >= 20*60 && mins <= 23*60+30
}

func aggregateProbeRows(rows []models.ProbeResult, bucketDur time.Duration) ([]historyPoint, historyStats) {
	if len(rows) == 0 {
		return []historyPoint{}, historyStats{}
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].CheckedAt.Before(rows[j].CheckedAt)
	})

	var totalOK, sumRTT, minRTT, maxRTT int
	var peakCnt, peakOK, peakSum int
	var offOK, offSum int
	var jitterSum, jitterCnt int
	prevRTT := -1
	minRTT = math.MaxInt32
	var lastOK *int

	for _, r := range rows {
		peak := isEveningPeak(r.CheckedAt)
		if peak {
			peakCnt++
		}
		if r.Success && r.RTT > 0 {
			totalOK++
			sumRTT += r.RTT
			if r.RTT < minRTT {
				minRTT = r.RTT
			}
			if r.RTT > maxRTT {
				maxRTT = r.RTT
			}
			if prevRTT > 0 {
				diff := r.RTT - prevRTT
				if diff < 0 {
					diff = -diff
				}
				jitterSum += diff
				jitterCnt++
			}
			prevRTT = r.RTT
			if peak {
				peakOK++
				peakSum += r.RTT
			} else {
				offOK++
				offSum += r.RTT
			}
			v := r.RTT
			lastOK = &v
		}
	}
	stats := historyStats{
		Current: lastOK,
		Samples: len(rows),
		Uptime:  math.Round(float64(totalOK)*1000/float64(len(rows))) / 10,
	}
	if totalOK > 0 {
		stats.Avg = int(math.Round(float64(sumRTT) / float64(totalOK)))
		stats.Min = minRTT
		stats.Max = maxRTT
	}
	if jitterCnt > 0 {
		stats.Jitter = int(math.Round(float64(jitterSum) / float64(jitterCnt)))
	}
	if peakCnt > 0 {
		stats.PeakUptime = math.Round(float64(peakOK)*1000/float64(peakCnt)) / 10
	}
	if peakOK > 0 {
		stats.PeakAvg = int(math.Round(float64(peakSum) / float64(peakOK)))
	}
	if offOK > 0 {
		stats.OffPeakAvg = int(math.Round(float64(offSum) / float64(offOK)))
	}

	if bucketDur <= 0 {
		pts := make([]historyPoint, 0, len(rows))
		for _, r := range rows {
			loss := 100.0
			rtt := -1
			if r.Success && r.RTT > 0 {
				loss = 0
				rtt = r.RTT
			}
			pts = append(pts, historyPoint{
				T:    r.CheckedAt.UnixMilli(),
				RTT:  rtt,
				Min:  rtt,
				Max:  rtt,
				OK:   r.Success && r.RTT > 0,
				Loss: loss,
			})
		}
		return pts, stats
	}

	type acc struct {
		ts     int64
		count  int
		okCnt  int
		sumRTT int
		minRTT int
		maxRTT int
	}
	bucketMap := make(map[int64]*acc)
	var keys []int64
	bucketMs := bucketDur.Milliseconds()
	for _, r := range rows {
		ms := r.CheckedAt.UnixMilli()
		key := (ms / bucketMs) * bucketMs
		a, exists := bucketMap[key]
		if !exists {
			a = &acc{ts: key + bucketMs/2, minRTT: math.MaxInt32}
			bucketMap[key] = a
			keys = append(keys, key)
		}
		a.count++
		if r.Success && r.RTT > 0 {
			a.okCnt++
			a.sumRTT += r.RTT
			if r.RTT < a.minRTT {
				a.minRTT = r.RTT
			}
			if r.RTT > a.maxRTT {
				a.maxRTT = r.RTT
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	pts := make([]historyPoint, 0, len(keys))
	for _, k := range keys {
		a := bucketMap[k]
		loss := math.Round(float64(a.count-a.okCnt)*1000/float64(a.count)) / 10
		if a.okCnt > 0 {
			avg := int(math.Round(float64(a.sumRTT) / float64(a.okCnt)))
			pts = append(pts, historyPoint{
				T:    a.ts,
				RTT:  avg,
				Min:  a.minRTT,
				Max:  a.maxRTT,
				OK:   true,
				Loss: loss,
			})
		} else {
			pts = append(pts, historyPoint{
				T:    a.ts,
				RTT:  -1,
				OK:   false,
				Loss: 100,
			})
		}
	}
	return pts, stats
}

// ProbeSparklines 批量返回所有节点的迷你延迟曲线（默认最近 24 小时，最多 ~48 个桶）。
func ProbeSparklines(c *gin.Context) {
	hours, _ := strconv.Atoi(c.DefaultQuery("hours", "24"))
	if hours < 1 {
		hours = 1
	}
	if hours > 168 {
		hours = 168
	}
	probeID, _ := strconv.Atoi(c.DefaultQuery("probeId", "0"))

	now := time.Now()
	since := now.Add(-time.Duration(hours) * time.Hour)

	q := models.DB.Where("probe_id > 0 AND checked_at >= ?", since)
	if probeID > 0 {
		q = q.Where("probe_id = ?", probeID)
	}
	var rows []models.ProbeResult
	if err := q.Order("checked_at asc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取历史曲线失败"})
		return
	}

	byNode := make(map[int][]models.ProbeResult)
	for _, r := range rows {
		byNode[r.NodeID] = append(byNode[r.NodeID], r)
	}

	// Target ~48 buckets across the selected window; for 1h use raw 1-min points.
	var bucketDur time.Duration
	if hours <= 1 {
		bucketDur = 0
	} else {
		bucketDur = time.Duration(hours) * time.Hour / 48
		if bucketDur < time.Minute {
			bucketDur = time.Minute
		}
	}

	out := make(map[int]sparklineSeries, len(byNode))
	nodeNames := map[int]string{}
	if allNodes, err := models.GetNodeList(); err == nil {
		for _, n := range allNodes {
			nodeNames[n.ID] = n.Name
		}
	}
	var bestPeak *bestPeakSummary
	bestScore := math.MaxFloat64
	for nodeID, list := range byNode {
		pts, stats := aggregateProbeRows(list, bucketDur)
		out[nodeID] = sparklineSeries{Points: pts, Stats: stats}
		if stats.Avg <= 0 || stats.Uptime < 80 {
			continue
		}
		refRTT := float64(stats.Avg)
		if stats.PeakAvg > 0 {
			refRTT = float64(stats.PeakAvg)
		}
		// Composite score: lower RTT + lower jitter + higher uptime wins.
		score := refRTT + float64(stats.Jitter)*1.5 + (100.0-stats.Uptime)*4.0
		if score < bestScore {
			bestScore = score
			bestPeak = &bestPeakSummary{
				NodeID:   nodeID,
				NodeName: nodeNames[nodeID],
				Avg:      stats.Avg,
				PeakAvg:  stats.PeakAvg,
				Jitter:   stats.Jitter,
				Uptime:   stats.Uptime,
			}
		}
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"from":      since.UnixMilli(),
			"to":        now.UnixMilli(),
			"peakBands": computePeakBands(since, now),
			"bestPeak":  bestPeak,
			"nodes":     out,
		},
		"msg": "ok",
	})
}

// ProbeHistory 返回单个节点的高精度多设备历史延迟曲线（支持 1h / 6h / 24h / 7d）。
func ProbeHistory(c *gin.Context) {
	nodeID, _ := strconv.Atoi(c.Query("nodeId"))
	if nodeID <= 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": "nodeId 无效"})
		return
	}
	rangeKey := strings.ToLower(strings.TrimSpace(c.DefaultQuery("range", "24h")))
	var dur time.Duration
	var bucketDur time.Duration
	switch rangeKey {
	case "1h":
		dur = time.Hour
		bucketDur = 0 // raw 1-minute samples
	case "6h":
		dur = 6 * time.Hour
		bucketDur = 2 * time.Minute
	case "7d":
		dur = 7 * 24 * time.Hour
		bucketDur = 45 * time.Minute
	default:
		rangeKey = "24h"
		dur = 24 * time.Hour
		bucketDur = 10 * time.Minute
	}

	probeID, _ := strconv.Atoi(c.DefaultQuery("probeId", "0"))
	now := time.Now()
	since := now.Add(-dur)

	probes, _ := models.GetProbes(false)
	probeByID := make(map[uint]models.Probe, len(probes))
	for _, p := range probes {
		probeByID[p.ID] = p
	}

	q := models.DB.Where("probe_id > 0 AND node_id = ? AND checked_at >= ?", nodeID, since)
	if probeID > 0 {
		q = q.Where("probe_id = ?", probeID)
	}
	var rows []models.ProbeResult
	if err := q.Order("checked_at asc").Find(&rows).Error; err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取节点历史失败"})
		return
	}

	byProbe := make(map[uint][]models.ProbeResult)
	for _, r := range rows {
		byProbe[r.ProbeID] = append(byProbe[r.ProbeID], r)
	}

	series := make([]probeHistorySeries, 0, len(byProbe))
	for _, p := range probes {
		list, ok := byProbe[p.ID]
		if !ok || len(list) == 0 {
			continue
		}
		pts, stats := aggregateProbeRows(list, bucketDur)
		series = append(series, probeHistorySeries{
			ProbeID:   p.ID,
			ProbeName: p.Name,
			Region:    p.Region,
			Stats:     stats,
			Points:    pts,
		})
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"nodeId":    nodeID,
			"range":     rangeKey,
			"from":      since.UnixMilli(),
			"to":        now.UnixMilli(),
			"peakBands": computePeakBands(since, now),
			"series":    series,
		},
		"msg": "ok",
	})
}
