package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type clashStatus struct {
	Connected  bool     `json:"connected"`
	Controller string   `json:"controller,omitempty"`
	Version    string   `json:"version,omitempty"`
	Group      string   `json:"group,omitempty"`
	Current    string   `json:"current,omitempty"`
	Proxies    []string `json:"proxies,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type clashNodeTestResult struct {
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

type clashReportPayload struct {
	JobID       string                `json:"jobId"`
	Action      string                `json:"action"` // clash-status | clash-switch | clash-batch | clash-batch-progress | net-diag
	Status      *clashStatus          `json:"status,omitempty"`
	Group       string                `json:"group,omitempty"`
	Original    string                `json:"original,omitempty"`
	Switched    string                `json:"switched,omitempty"`
	CurrentNode string                `json:"currentNode,omitempty"`
	Total       int                   `json:"total,omitempty"`
	Done        bool                  `json:"done,omitempty"`
	Items       []clashNodeTestResult `json:"items,omitempty"`
	NetDiag     *netDiagReport        `json:"netDiag,omitempty"`
	Error       string                `json:"error,omitempty"`
}

var (
	clashCfgMu  sync.RWMutex
	clashAPIURL string
	clashSecret string
)

func setClashConfig(apiURL, secret string) {
	clashCfgMu.Lock()
	clashAPIURL = strings.TrimRight(strings.TrimSpace(apiURL), "/")
	clashSecret = strings.TrimSpace(secret)
	clashCfgMu.Unlock()
}

func clashHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy: nil, // loopback External Controller must never hit system proxy
		},
	}
}

func clashCandidates() ([]string, string) {
	clashCfgMu.RLock()
	cfgURL := clashAPIURL
	sec := clashSecret
	clashCfgMu.RUnlock()

	if envURL := strings.TrimRight(strings.TrimSpace(os.Getenv("CLASH_API_URL")), "/"); envURL != "" && cfgURL == "" {
		cfgURL = envURL
	}
	if envSec := strings.TrimSpace(os.Getenv("CLASH_API_SECRET")); envSec != "" && sec == "" {
		sec = envSec
	}

	cands := make([]string, 0, 6)
	if cfgURL != "" {
		if !strings.HasPrefix(cfgURL, "http://") && !strings.HasPrefix(cfgURL, "https://") {
			cfgURL = "http://" + cfgURL
		}
		cands = append(cands, cfgURL)
	}
	for _, u := range []string{
		"http://127.0.0.1:9090",
		"http://[::1]:9090",
		"http://127.0.0.1:9097",
		"http://127.0.0.1:9091",
		"http://127.0.0.1:29090",
	} {
		dup := false
		for _, e := range cands {
			if e == u {
				dup = true
				break
			}
		}
		if !dup {
			cands = append(cands, u)
		}
	}
	return cands, sec
}

func clashDoRequest(ctx context.Context, client *http.Client, method, endpoint, secret string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return nil, err
	}
	if secret != "" {
		req.Header.Set("Authorization", "Bearer "+secret)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return client.Do(req)
}

func isBuiltinClashProxy(name string) bool {
	u := strings.ToUpper(strings.TrimSpace(name))
	return u == "" || u == "DIRECT" || u == "REJECT" || u == "REJECT-DROP" || u == "PASS" || u == "COMPATIBLE"
}

func detectClashStatus(ctx context.Context) clashStatus {
	cands, sec := clashCandidates()
	client := clashHTTPClient(2500 * time.Millisecond)

	var baseURL, ver string
	var authErrURL string
	var authErrCode int
	for _, cand := range cands {
		pCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		resp, err := clashDoRequest(pCtx, client, http.MethodGet, cand+"/version", sec, nil)
		if err == nil {
			raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var v struct {
					Version string `json:"version"`
				}
				if json.Unmarshal(raw, &v) == nil && v.Version != "" {
					baseURL = cand
					ver = v.Version
					cancel()
					break
				}
			} else if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && authErrURL == "" {
				authErrURL = cand
				authErrCode = resp.StatusCode
			}
		}
		cancel()
	}
	if baseURL == "" {
		if authErrURL != "" {
			return clashStatus{
				Connected:  false,
				Controller: authErrURL,
				Error:      fmt.Sprintf("检测到本机 Clash (%s)，但 API 访问密钥未填写或不正确 (HTTP %d)", authErrURL, authErrCode),
			}
		}
		return clashStatus{Connected: false}
	}

	pCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	resp, err := clashDoRequest(pCtx, client, http.MethodGet, baseURL+"/proxies", sec, nil)
	if err != nil {
		return clashStatus{Connected: true, Controller: baseURL, Version: ver, Error: shortErr(err)}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return clashStatus{Connected: true, Controller: baseURL, Version: ver, Error: fmt.Sprintf("HTTP %d", resp.StatusCode)}
	}

	var parsed struct {
		Proxies map[string]struct {
			Name string   `json:"name"`
			Type string   `json:"type"`
			Now  string   `json:"now"`
			All  []string `json:"all"`
		} `json:"proxies"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4*1024*1024)).Decode(&parsed); err != nil {
		return clashStatus{Connected: true, Controller: baseURL, Version: ver, Error: shortErr(err)}
	}

	// Pick best Selector group: prefer known main groups, else largest selector.
	preferred := []string{"日常使用", "节点选择", "PROXY", "Proxy", "Select", "GLOBAL"}
	var bestGroup, bestNow string
	var bestProxies []string

	filterReal := func(all []string) []string {
		out := make([]string, 0, len(all))
		for _, n := range all {
			if isBuiltinClashProxy(n) {
				continue
			}
			// Skip sub-groups when listing concrete nodes unless everything is a group.
			if p, ok := parsed.Proxies[n]; ok && (p.Type == "Selector" || p.Type == "URLTest" || p.Type == "Fallback" || p.Type == "LoadBalance") {
				continue
			}
			out = append(out, n)
		}
		if len(out) == 0 {
			for _, n := range all {
				if !isBuiltinClashProxy(n) {
					out = append(out, n)
				}
			}
		}
		return out
	}

	for _, pref := range preferred {
		if g, ok := parsed.Proxies[pref]; ok && strings.EqualFold(g.Type, "Selector") {
			real := filterReal(g.All)
			if len(real) > 0 {
				bestGroup = pref
				bestNow = g.Now
				bestProxies = real
				break
			}
		}
	}
	if bestGroup == "" {
		for name, g := range parsed.Proxies {
			if !strings.EqualFold(g.Type, "Selector") {
				continue
			}
			real := filterReal(g.All)
			if len(real) > len(bestProxies) {
				bestGroup = name
				bestNow = g.Now
				bestProxies = real
			}
		}
	}

	st := clashStatus{
		Connected:  true,
		Controller: baseURL,
		Version:    ver,
		Group:      bestGroup,
		Current:    bestNow,
		Proxies:    bestProxies,
	}
	return st
}

func switchClashProxy(ctx context.Context, group, targetNode string) (clashStatus, error) {
	st := detectClashStatus(ctx)
	if !st.Connected || st.Controller == "" {
		return st, fmt.Errorf("未检测到本机 Clash / Mihomo 外部控制接口")
	}
	if strings.TrimSpace(group) == "" {
		group = st.Group
	}
	if group == "" {
		return st, fmt.Errorf("未找到可用的 Clash 策略组")
	}
	// Allow fuzzy match by node name substring (handles leading flag emojis).
	resolved := targetNode
	foundExact := false
	for _, p := range st.Proxies {
		if p == targetNode {
			foundExact = true
			break
		}
	}
	if !foundExact {
		lowTarget := strings.ToLower(strings.TrimSpace(targetNode))
		for _, p := range st.Proxies {
			if strings.Contains(strings.ToLower(p), lowTarget) || strings.Contains(lowTarget, strings.ToLower(p)) {
				resolved = p
				break
			}
		}
	}

	_, sec := clashCandidates()
	client := clashHTTPClient(2 * time.Second)
	body, _ := json.Marshal(map[string]string{"name": resolved})
	endpoint := fmt.Sprintf("%s/proxies/%s", st.Controller, url.PathEscape(group))
	resp, err := clashDoRequest(ctx, client, http.MethodPut, endpoint, sec, bytes.NewReader(body))
	if err != nil {
		return st, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return st, fmt.Errorf("切换失败 (HTTP %d): %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	st.Group = group
	st.Current = resolved
	return st, nil
}

type clashRuntimeConfig struct {
	Mode      string `json:"mode"`
	Port      int    `json:"port"`
	MixedPort int    `json:"mixed-port"`
}

func getClashRuntimeConfig(ctx context.Context, controller, sec string) clashRuntimeConfig {
	client := clashHTTPClient(2 * time.Second)
	resp, err := clashDoRequest(ctx, client, http.MethodGet, controller+"/configs", sec, nil)
	if err != nil {
		return clashRuntimeConfig{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return clashRuntimeConfig{}
	}
	var cfg clashRuntimeConfig
	if json.NewDecoder(io.LimitReader(resp.Body, 32*1024)).Decode(&cfg) == nil {
		cfg.Mode = strings.TrimSpace(cfg.Mode)
		return cfg
	}
	return clashRuntimeConfig{}
}

func setClashMode(ctx context.Context, controller, sec, mode string) error {
	if strings.TrimSpace(mode) == "" {
		return nil
	}
	client := clashHTTPClient(2 * time.Second)
	body, _ := json.Marshal(map[string]string{"mode": mode})
	resp, err := clashDoRequest(ctx, client, http.MethodPatch, controller+"/configs", sec, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func getClashGroupInfo(ctx context.Context, controller, sec, groupName string) (string, []string) {
	client := clashHTTPClient(2 * time.Second)
	endpoint := fmt.Sprintf("%s/proxies/%s", controller, url.PathEscape(groupName))
	resp, err := clashDoRequest(ctx, client, http.MethodGet, endpoint, sec, nil)
	if err != nil {
		return "", nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil
	}
	var g struct {
		Now string   `json:"now"`
		All []string `json:"all"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&g) != nil {
		return "", nil
	}
	return g.Now, g.All
}

func containsExact(list []string, target string) bool {
	for _, s := range list {
		if s == target {
			return true
		}
	}
	return false
}

func newClashBatchHTTPClient(controller string, cfg clashRuntimeConfig, timeout time.Duration) *http.Client {
	proxyPort := cfg.MixedPort
	if proxyPort <= 0 {
		proxyPort = cfg.Port
	}
	if proxyPort > 0 {
		host := "127.0.0.1"
		if u, err := url.Parse(controller); err == nil && u.Hostname() != "" {
			host = u.Hostname()
		}
		if proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%d", host, proxyPort)); err == nil {
			return &http.Client{
				Timeout: timeout,
				Transport: &http.Transport{
					Proxy:                 http.ProxyURL(proxyURL),
					ForceAttemptHTTP2:     true,
					TLSHandshakeTimeout:   8 * time.Second,
					ResponseHeaderTimeout: timeout,
					MaxIdleConnsPerHost:   4,
				},
			}
		}
	}
	return newActiveProxyHTTPClient(timeout)
}

func putClashGroupProxy(ctx context.Context, controller, sec, groupName, nodeName string) error {
	client := clashHTTPClient(2 * time.Second)
	body, _ := json.Marshal(map[string]string{"name": nodeName})
	endpoint := fmt.Sprintf("%s/proxies/%s", controller, url.PathEscape(groupName))
	resp, err := clashDoRequest(ctx, client, http.MethodPut, endpoint, sec, bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}

// runClashBatchTest iterates over Clash nodes, temporarily switches Clash to Global mode
// so rule-specific groups (ChatGPT/Google/etc.) cannot leak traffic onto another node,
// routes directly via Clash's local mixed/http port, measures real throughput + 6 AI/streaming unlocks,
// and ALWAYS restores mode, GLOBAL selection, and original group node.
func runClashBatchTest(ctx context.Context, group string, requested []string, onProgress func(group, currentNode string, items []clashNodeTestResult, total int)) clashReportPayload {
	st := detectClashStatus(ctx)
	if !st.Connected || st.Controller == "" {
		return clashReportPayload{Action: "clash-batch", Error: "未检测到本机 Clash / Mihomo 外部控制接口（9090/9097）"}
	}
	if strings.TrimSpace(group) == "" {
		group = st.Group
	}
	if group == "" {
		return clashReportPayload{Action: "clash-batch", Error: "未找到可切换的 Clash Selector 策略组"}
	}
	original := st.Current
	targets := requested
	if len(targets) == 0 {
		targets = st.Proxies
	}
	if len(targets) > 16 {
		targets = targets[:16]
	}
	if len(targets) == 0 {
		return clashReportPayload{Action: "clash-batch", Group: group, Original: original, Error: "策略组内无可用节点"}
	}

	_, sec := clashCandidates()
	rtCfg := getClashRuntimeConfig(ctx, st.Controller, sec)
	origMode := rtCfg.Mode
	origGlobalNow, globalAll := getClashGroupInfo(ctx, st.Controller, sec, "GLOBAL")
	globalHasGroup := containsExact(globalAll, group)
	globalHasAnyTarget := false
	for _, t := range targets {
		if containsExact(globalAll, t) {
			globalHasAnyTarget = true
			break
		}
	}

	inGlobalMode := strings.EqualFold(origMode, "global")
	switchedGlobalMode := false
	// Only switch to Global mode when GLOBAL can actually select either the target nodes
	// or the parent selector group (e.g. "日常使用"); otherwise GLOBAL could remain pinned to DIRECT!
	if (globalHasGroup || globalHasAnyTarget) && !inGlobalMode {
		if globalHasGroup {
			// Point GLOBAL -> group before enabling global mode so traffic never hits DIRECT.
			if err := putClashGroupProxy(ctx, st.Controller, sec, "GLOBAL", group); err == nil {
				if err := setClashMode(ctx, st.Controller, sec, "global"); err == nil {
					inGlobalMode = true
					switchedGlobalMode = true
				}
			}
		} else if err := setClashMode(ctx, st.Controller, sec, "global"); err == nil {
			inGlobalMode = true
			switchedGlobalMode = true
		}
	}

	defer func() {
		restoreCtx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		if len(globalAll) > 0 && origGlobalNow != "" {
			_ = putClashGroupProxy(restoreCtx, st.Controller, sec, "GLOBAL", origGlobalNow)
		}
		if switchedGlobalMode && origMode != "" {
			_ = setClashMode(restoreCtx, st.Controller, sec, origMode)
		}
		if original != "" {
			_ = putClashGroupProxy(restoreCtx, st.Controller, sec, group, original)
		}
	}()

	items := make([]clashNodeTestResult, 0, len(targets))
	total := len(targets)
	for _, nodeName := range targets {
		if ctx.Err() != nil {
			break
		}
		if onProgress != nil {
			snapshot := make([]clashNodeTestResult, len(items))
			copy(snapshot, items)
			onProgress(group, nodeName, snapshot, total)
		}
		swCtx, swCancel := context.WithTimeout(ctx, 2500*time.Millisecond)
		var swErr error
		if inGlobalMode {
			switch {
			case containsExact(globalAll, nodeName):
				swErr = putClashGroupProxy(swCtx, st.Controller, sec, "GLOBAL", nodeName)
				if swErr == nil {
					_ = putClashGroupProxy(swCtx, st.Controller, sec, group, nodeName)
				}
			case globalHasGroup:
				// In proxy-provider configs, GLOBAL lists selector groups ("日常使用") rather than raw provider nodes.
				// Chain GLOBAL -> group -> nodeName so 100% of traffic exits via nodeName (never DIRECT).
				swErr = putClashGroupProxy(swCtx, st.Controller, sec, group, nodeName)
				if swErr == nil {
					swErr = putClashGroupProxy(swCtx, st.Controller, sec, "GLOBAL", group)
				}
			default:
				// Safety guard: never run tests in global mode if GLOBAL cannot route to this node.
				if switchedGlobalMode && origMode != "" {
					_ = setClashMode(swCtx, st.Controller, sec, origMode)
					inGlobalMode = false
					switchedGlobalMode = false
				}
				swErr = putClashGroupProxy(swCtx, st.Controller, sec, group, nodeName)
			}
		} else {
			swErr = putClashGroupProxy(swCtx, st.Controller, sec, group, nodeName)
		}
		swCancel()
		if swErr != nil {
			items = append(items, clashNodeTestResult{Name: nodeName, Error: shortErr(swErr)})
			continue
		}
		select {
		case <-ctx.Done():
			break
		case <-time.After(350 * time.Millisecond):
		}

		// Fresh HTTP client per node routed explicitly through Clash's local mixed/http port
		// so pooled TCP/TLS connections never stay pinned to the previous node or leak to direct NIC.
		client := newClashBatchHTTPClient(st.Controller, rtCfg, 9*time.Second)
		items = append(items, testSingleClashNode(ctx, client, nodeName))
		if onProgress != nil {
			snapshot := make([]clashNodeTestResult, len(items))
			copy(snapshot, items)
			onProgress(group, "", snapshot, total)
		}
	}

	return clashReportPayload{
		Action:   "clash-batch",
		Status:   &st,
		Group:    group,
		Original: original,
		Total:    total,
		Done:     true,
		Items:    items,
	}
}

func testSingleClashNode(ctx context.Context, client *http.Client, nodeName string) clashNodeTestResult {
	res := clashNodeTestResult{Name: nodeName}

	// 1. Parallel: quick egress + 6 core AI/streaming unlock checks (max 4.5s).
	uCtx, cancelU := context.WithTimeout(ctx, 4500*time.Millisecond)
	var wg sync.WaitGroup
	var mu sync.Mutex

	wg.Add(1)
	go func() {
		defer wg.Done()
		t0 := time.Now()
		req, err := http.NewRequestWithContext(uCtx, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil)
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		rtt := max(int(time.Since(t0).Milliseconds()), 1)
		kv := parseTraceKV(string(raw))
		mu.Lock()
		res.TTFBMs = rtt
		res.EgressIP = kv["ip"]
		res.Country = kv["loc"]
		mu.Unlock()
	}()

	runUnlock := func(fn func(context.Context, *http.Client) unlockItemResult, dst *string) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := fn(uCtx, client)
			val := r.Status
			if r.Region != "" && (r.Status == "available" || r.Status == "originals") {
				val = r.Status + ":" + r.Region
			}
			mu.Lock()
			*dst = val
			mu.Unlock()
		}()
	}
	runUnlock(checkChatGPT, &res.ChatGPT)
	runUnlock(checkClaude, &res.Claude)
	runUnlock(checkGemini, &res.Gemini)
	runUnlock(checkNetflix, &res.Netflix)
	runUnlock(checkDisneyPlus, &res.Disney)
	runUnlock(checkYouTubePremium, &res.YouTube)
	wg.Wait()
	cancelU()

	// 2. Fast 4.2s download speedtest with multi-CDN fallback (Cloudflare -> CacheFly -> OVH).
	dlURLs := []string{
		"https://speed.cloudflare.com/__down?bytes=15000000",
		"https://cachefly.cachefly.net/10mb.test",
		"https://proof.ovh.net/files/10Mb.dat",
	}
	for _, dlURL := range dlURLs {
		if ctx.Err() != nil {
			break
		}
		dlCtx, cancelDL := context.WithTimeout(ctx, 4200*time.Millisecond)
		var totalBytes atomic.Int64
		var firstByteNs atomic.Int64
		var dlWG sync.WaitGroup
		for i := 0; i < 2; i++ {
			dlWG.Add(1)
			go func() {
				defer dlWG.Done()
				req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, dlURL, nil)
				if err != nil {
					return
				}
				req.Header.Set("User-Agent", unlockUA)
				req.Header.Set("Accept-Encoding", "identity")
				resp, err := client.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return
				}
				buf := make([]byte, 32*1024)
				for {
					n, rerr := resp.Body.Read(buf)
					if n > 0 {
						firstByteNs.CompareAndSwap(0, time.Now().UnixNano())
						totalBytes.Add(int64(n))
					}
					if rerr != nil || dlCtx.Err() != nil {
						break
					}
				}
			}()
		}
		dlWG.Wait()
		cancelDL()
		if b := totalBytes.Load(); b > 16*1024 {
			if fb := firstByteNs.Load(); fb > 0 {
				elapsed := time.Since(time.Unix(0, fb)).Seconds()
				if elapsed < 0.25 {
					elapsed = 0.25
				}
				res.DownloadMbps = round2(float64(b) * 8.0 / (elapsed * 1_000_000.0))
				break
			}
		}
	}
	if res.EgressIP == "" && res.DownloadMbps == 0 {
		res.Error = "节点不可达"
	}
	return res
}

func parseTraceKV(raw string) map[string]string {
	kv := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		if idx := strings.IndexByte(line, '='); idx > 0 {
			kv[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
		}
	}
	return kv
}
