package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"ppeelink/models"

	"github.com/gin-gonic/gin"
)

type clashStatusDTO struct {
	Connected  bool     `json:"connected"`
	Controller string   `json:"controller,omitempty"`
	Version    string   `json:"version,omitempty"`
	Group      string   `json:"group,omitempty"`
	Current    string   `json:"current,omitempty"`
	Proxies    []string `json:"proxies,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type clashNodeTestResultDTO struct {
	Name         string  `json:"name"`
	EgressIP     string  `json:"egressIp,omitempty"`
	Country      string  `json:"country,omitempty"`
	TTFBMs       int     `json:"ttfbMs"`
	DownloadMbps float64 `json:"downloadMbps"`
	ChatGPT      string  `json:"chatgpt"`
	Claude       string  `json:"claude"`
	Gemini       string  `json:"gemini"`
	Netflix      string  `json:"netflix"`
	Disney       string  `json:"disney"`
	YouTube      string  `json:"youtube"`
	Error        string  `json:"error,omitempty"`
}

type clashReportDTO struct {
	JobID     string                   `json:"jobId"`
	ProbeName string                   `json:"probeName,omitempty"`
	Action    string                   `json:"action"`
	Status    *clashStatusDTO          `json:"status,omitempty"`
	Group     string                   `json:"group,omitempty"`
	Original  string                   `json:"original,omitempty"`
	Switched  string                   `json:"switched,omitempty"`
	Items     []clashNodeTestResultDTO `json:"items,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

type liveProbeClash struct {
	Group string
	Now   string
	At    time.Time
}

var (
	clashResultsHub = struct {
		mu      sync.Mutex
		waiters map[string]chan clashReportDTO
	}{
		waiters: map[string]chan clashReportDTO{},
	}

	probeClashLive = struct {
		mu   sync.RWMutex
		data map[uint]liveProbeClash
	}{
		data: map[uint]liveProbeClash{},
	}
)

func recordProbeClashLive(probeID uint, group, now string) {
	probeClashLive.mu.Lock()
	if group == "" && now == "" {
		delete(probeClashLive.data, probeID)
	} else {
		probeClashLive.data[probeID] = liveProbeClash{Group: group, Now: now, At: time.Now()}
	}
	probeClashLive.mu.Unlock()
}

func getProbeClashLive(probeID uint) (string, string) {
	probeClashLive.mu.RLock()
	st, ok := probeClashLive.data[probeID]
	probeClashLive.mu.RUnlock()
	if !ok || time.Since(st.At) > 2*time.Minute {
		return "", ""
	}
	return st.Group, st.Now
}

func dispatchClashJob(c *gin.Context, probeID uint, job queuedProbeJob, timeout time.Duration) (*clashReportDTO, bool) {
	target, err := pickOnlineUserProbe(probeID)
	if err != nil {
		c.JSON(500, gin.H{"code": "50000", "msg": "读取用户 Agent 失败"})
		return nil, false
	}
	if target == nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "没有在线的用户 Agent"})
		return nil, false
	}
	if job.ClashAPI == "" && target.ClashAPI != "" {
		job.ClashAPI = target.ClashAPI
	}
	if job.ClashSecret == "" && target.ClashSecret != "" {
		job.ClashSecret = target.ClashSecret
	}
	jobID := newProbeJobID()
	job.ID = jobID

	// Register waiter BEFORE waking the probe so fast responses can never race past waiters[jobID].
	resCh := make(chan clashReportDTO, 1)
	clashResultsHub.mu.Lock()
	clashResultsHub.waiters[jobID] = resCh
	clashResultsHub.mu.Unlock()
	defer func() {
		clashResultsHub.mu.Lock()
		delete(clashResultsHub.waiters, jobID)
		clashResultsHub.mu.Unlock()
	}()

	_, _, ok := probeHub.dispatchJob(target.ID, job, true)
	if !ok {
		c.JSON(409, gin.H{"code": "40900", "msg": "用户 Agent 繁忙，请稍后再试"})
		return nil, false
	}
	defer probeHub.cleanupDone(jobID)

	select {
	case res := <-resCh:
		res.ProbeName = target.Name
		if res.Status != nil && res.Status.Connected {
			recordProbeClashLive(target.ID, res.Status.Group, res.Status.Current)
		}
		return &res, true
	case <-time.After(timeout):
		c.JSON(504, gin.H{"code": "50400", "msg": "Clash 指令超时，请确认设备在线并已升级至 v1.9"})
		return nil, false
	case <-c.Request.Context().Done():
		return nil, false
	}
}

// UserProbeClashReport 用户 Agent 回报 Clash 状态/切换/全节点批量测试结果。
func UserProbeClashReport(c *gin.Context) {
	if _, ok := currentProbe(c); !ok {
		return
	}
	var req clashReportDTO
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": "40000", "msg": "格式错误"})
		return
	}
	clashResultsHub.mu.Lock()
	ch, ok := clashResultsHub.waiters[req.JobID]
	if ok {
		delete(clashResultsHub.waiters, req.JobID)
	}
	clashResultsHub.mu.Unlock()
	if ok {
		select {
		case ch <- req:
		default:
		}
		close(ch)
	}
	probeHub.complete(req.JobID)
	c.JSON(200, gin.H{"code": "00000", "msg": "ok"})
}

// ProbeClashStatus 查询本机用户 Agent 探测到的 Clash/Mihomo 外部控制状态与节点列表。
func ProbeClashStatus(c *gin.Context) {
	var req struct {
		ProbeID uint `json:"probeId"`
	}
	_ = c.ShouldBindJSON(&req)
	res, ok := dispatchClashJob(c, req.ProbeID, queuedProbeJob{Action: "clash-status"}, 10*time.Second)
	if !ok {
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": res, "msg": "ok"})
}

// ProbeClashSwitch 远程指令本机用户 Agent 切换 Clash/Mihomo 策略组当前节点。
func ProbeClashSwitch(c *gin.Context) {
	var req struct {
		ProbeID uint   `json:"probeId"`
		Group   string `json:"group"`
		Node    string `json:"node"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Node) == "" {
		c.JSON(400, gin.H{"code": "40000", "msg": "目标节点名不能为空"})
		return
	}
	res, ok := dispatchClashJob(c, req.ProbeID, queuedProbeJob{
		Action:     "clash-switch",
		ClashGroup: strings.TrimSpace(req.Group),
		ClashNode:  strings.TrimSpace(req.Node),
	}, 10*time.Second)
	if !ok {
		return
	}
	if res.Error != "" {
		c.JSON(400, gin.H{"code": "40000", "msg": res.Error, "data": res})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": res, "msg": "已切换至 " + res.Switched})
}

// ProbeClashBatchTest 通过本机用户 Agent 自动轮切 Clash 节点，实测每个节点的真实下载带宽与 AI/流媒体解锁。
func ProbeClashBatchTest(c *gin.Context) {
	var req struct {
		ProbeID uint     `json:"probeId"`
		Group   string   `json:"group"`
		Nodes   []string `json:"nodes"`
	}
	_ = c.ShouldBindJSON(&req)
	res, ok := dispatchClashJob(c, req.ProbeID, queuedProbeJob{
		Action:     "clash-batch",
		ClashGroup: strings.TrimSpace(req.Group),
		ClashNodes: req.Nodes,
	}, 120*time.Second)
	if !ok {
		return
	}
	if res.Error != "" && len(res.Items) == 0 {
		c.JSON(400, gin.H{"code": "40000", "msg": res.Error, "data": res})
		return
	}
	c.JSON(200, gin.H{"code": "00000", "data": res, "msg": fmt.Sprintf("已完成 %d 个 Clash 节点实测", len(res.Items))})
}

// ---- #9 首页端到端拓扑与异常焦点数据 ------------------------------------------

type probeCoord struct {
	Lat  float64
	Lon  float64
	City string
}

var probeCoordCache = struct {
	mu   sync.RWMutex
	data map[string]probeCoord
}{
	data: map[string]probeCoord{},
}

var cnCityCoords = map[string][2]float64{
	"福州": {26.0745, 119.2965},
	"天津": {39.0842, 117.2009},
	"北京": {39.9042, 116.4074},
	"上海": {31.2304, 121.4737},
	"广州": {23.1291, 113.2644},
	"深圳": {22.5431, 114.0579},
	"杭州": {30.2741, 120.1551},
	"成都": {30.5728, 104.0668},
	"武汉": {30.5928, 114.3055},
	"南京": {32.0603, 118.7969},
	"厦门": {24.4798, 118.0894},
	"西安": {34.3416, 108.9398},
	"重庆": {29.5630, 106.5516},
	"苏州": {31.2989, 120.5853},
	"长沙": {28.2282, 112.9388},
	"香港": {22.3193, 114.1694},
}

func resolveProbeGeoCoord(name, region, ip string) (float64, float64) {
	ip = strings.TrimSpace(ip)
	if ip != "" && !isPrivateOrLAN(ip) {
		probeCoordCache.mu.RLock()
		c, ok := probeCoordCache.data[ip]
		probeCoordCache.mu.RUnlock()
		if ok && (c.Lat != 0 || c.Lon != 0) {
			return c.Lat, c.Lon
		}
		client := &http.Client{Timeout: 2 * time.Second}
		if resp, err := client.Get("http://ip-api.com/json/" + ip + "?fields=status,lat,lon,city&lang=zh-CN"); err == nil {
			defer resp.Body.Close()
			var res struct {
				Status string  `json:"status"`
				Lat    float64 `json:"lat"`
				Lon    float64 `json:"lon"`
				City   string  `json:"city"`
			}
			if json.NewDecoder(resp.Body).Decode(&res) == nil && res.Status == "success" && (res.Lat != 0 || res.Lon != 0) {
				probeCoordCache.mu.Lock()
				probeCoordCache.data[ip] = probeCoord{Lat: res.Lat, Lon: res.Lon, City: res.City}
				probeCoordCache.mu.Unlock()
				return res.Lat, res.Lon
			}
		}
	}
	combined := name + " " + region
	for city, ll := range cnCityCoords {
		if strings.Contains(combined, city) {
			return ll[0], ll[1]
		}
	}
	return 31.2304, 121.4737 // China mainland center fallback
}

// ProbeTopology 返回首页 WorldMap 所需的在线用户 Agent 坐标、端到端实时延迟飞线及异常焦点。
func ProbeTopology(c *gin.Context) {
	probes, _ := models.GetProbes(true)
	now := time.Now()

	type probeOrigin struct {
		ID       uint    `json:"id"`
		Name     string  `json:"name"`
		ISP      string  `json:"isp,omitempty"`
		PublicIP string  `json:"publicIp,omitempty"`
		ClashNow string  `json:"clashNow,omitempty"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
	}
	type topologyArc struct {
		ProbeID uint   `json:"probeId"`
		NodeID  int    `json:"nodeId"`
		Success bool   `json:"success"`
		RTT     int    `json:"rtt"`
		Kind    string `json:"kind"`
	}
	type anomalyItem struct {
		Tone   string `json:"tone"` // warn | bad
		Title  string `json:"title"`
		Target string `json:"target"` // route path
	}

	origins := make([]probeOrigin, 0)
	onlineProbeIDs := make([]uint, 0)
	for i := range probes {
		p := &probes[i]
		if !isProbeRecentlyOnline(p) {
			continue
		}
		lat, lon := resolveProbeGeoCoord(p.Name, p.Region, p.PublicIP)
		_, clashNow := getProbeClashLive(p.ID)
		origins = append(origins, probeOrigin{
			ID:       p.ID,
			Name:     p.Name,
			ISP:      p.ISP,
			PublicIP: p.PublicIP,
			ClashNow: clashNow,
			Lat:      lat,
			Lon:      lon,
		})
		onlineProbeIDs = append(onlineProbeIDs, p.ID)
	}

	arcs := make([]topologyArc, 0)
	failedNodes := map[int]bool{}
	if len(onlineProbeIDs) > 0 {
		var rows []models.ProbeResult
		_ = models.DB.Where("probe_id IN ? AND checked_at >= ?", onlineProbeIDs, now.Add(-15*time.Minute)).
			Order("checked_at desc, id desc").Find(&rows).Error
		seen := map[string]bool{}
		for _, r := range rows {
			key := fmt.Sprintf("%d:%d", r.ProbeID, r.NodeID)
			if seen[key] {
				continue
			}
			seen[key] = true
			arcs = append(arcs, topologyArc{
				ProbeID: r.ProbeID,
				NodeID:  r.NodeID,
				Success: r.Success,
				RTT:     r.RTT,
				Kind:    r.Kind,
			})
			if !r.Success {
				failedNodes[r.NodeID] = true
			}
		}
	}

	anomalies := make([]anomalyItem, 0, 4)
	if len(failedNodes) > 0 {
		anomalies = append(anomalies, anomalyItem{
			Tone:   "warn",
			Title:  fmt.Sprintf("%d 个节点本机直连不可达", len(failedNodes)),
			Target: "/monitor/user-agent",
		})
	}
	var servers []models.ProxyServer
	if models.DB.Where("enabled = ? AND removing = ?", true, false).Find(&servers).Error == nil {
		offlineCnt := 0
		for _, s := range servers {
			if s.LastSeenAt == nil || now.Sub(*s.LastSeenAt) > 2*time.Minute {
				offlineCnt++
			}
		}
		if offlineCnt > 0 {
			anomalies = append(anomalies, anomalyItem{
				Tone:   "bad",
				Title:  fmt.Sprintf("%d 台代理服务器 Agent 离线", offlineCnt),
				Target: "/subcription/servers",
			})
		}
	}

	c.JSON(200, gin.H{
		"code": "00000",
		"data": gin.H{
			"origins":   origins,
			"arcs":      arcs,
			"anomalies": anomalies,
		},
		"msg": "ok",
	})
}
