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
	JobID    string                `json:"jobId"`
	Action   string                `json:"action"` // clash-status | clash-switch | clash-batch
	Status   *clashStatus          `json:"status,omitempty"`
	Group    string                `json:"group,omitempty"`
	Original string                `json:"original,omitempty"`
	Switched string                `json:"switched,omitempty"`
	Items    []clashNodeTestResult `json:"items,omitempty"`
	Error    string                `json:"error,omitempty"`
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
	client := clashHTTPClient(700 * time.Millisecond)

	var baseURL, ver string
	var authErrURL string
	var authErrCode int
	for _, cand := range cands {
		pCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
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
	if err := json.NewDecoder(io.LimitReader(resp.Body, 512*1024)).Decode(&parsed); err != nil {
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
	updateTrayClashState(st.Group, st.Current, st.Proxies)
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
	updateTrayClashState(st.Group, st.Current, st.Proxies)
	return st, nil
}

// runClashBatchTest iterates over Clash nodes, measures quick real-world throughput
// + key AI/streaming unlock on each via Channel B, and ALWAYS restores the original node.
func runClashBatchTest(ctx context.Context, group string, requested []string) clashReportPayload {
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

	defer func() {
		if original != "" {
			restoreCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			_, _ = switchClashProxy(restoreCtx, group, original)
			cancel()
		}
	}()

	items := make([]clashNodeTestResult, 0, len(targets))
	for _, nodeName := range targets {
		if ctx.Err() != nil {
			break
		}
		swCtx, swCancel := context.WithTimeout(ctx, 2*time.Second)
		_, err := switchClashProxy(swCtx, group, nodeName)
		swCancel()
		if err != nil {
			items = append(items, clashNodeTestResult{Name: nodeName, Error: shortErr(err)})
			continue
		}
		select {
		case <-ctx.Done():
			break
		case <-time.After(280 * time.Millisecond):
		}

		// Fresh HTTP client per node so pooled TCP/TLS connections don't stay pinned to the previous node!
		client := newActiveProxyHTTPClient(6 * time.Second)
		items = append(items, testSingleClashNode(ctx, client, nodeName))
	}

	return clashReportPayload{
		Action:   "clash-batch",
		Status:   &st,
		Group:    group,
		Original: original,
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

	// 2. Fast 2.2s 2-stream download speedtest over Cloudflare edge.
	dlCtx, cancelDL := context.WithTimeout(ctx, 2200*time.Millisecond)
	var totalBytes atomic.Int64
	startDL := time.Now()
	var dlWG sync.WaitGroup
	for i := 0; i < 2; i++ {
		dlWG.Add(1)
		go func() {
			defer dlWG.Done()
			req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, "https://speed.cloudflare.com/__down?bytes=12000000", nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", unlockUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			buf := make([]byte, 32*1024)
			for {
				n, rerr := resp.Body.Read(buf)
				if n > 0 {
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
	elapsed := time.Since(startDL).Seconds()
	if b := totalBytes.Load(); b > 0 && elapsed > 0.05 {
		res.DownloadMbps = round2(float64(b) * 8.0 / (elapsed * 1_000_000.0))
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
