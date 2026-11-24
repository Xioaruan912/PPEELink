package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const unlockUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type unlockEgressInfo struct {
	IP        string `json:"ip"`
	Country   string `json:"country"`
	Loc       string `json:"loc"`
	City      string `json:"city,omitempty"`
	Colo      string `json:"colo,omitempty"`
	ISP       string `json:"isp,omitempty"`
	ASN       string `json:"asn,omitempty"`
	Hosting   bool   `json:"hosting"`
	RTT       int    `json:"rtt"`
	DirectIP  string `json:"directIp,omitempty"`
}

type unlockItemResult struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	Group     string `json:"group"`
	OK        bool   `json:"ok"`
	Status    string `json:"status"` // available | originals | blocked | unknown
	Region    string `json:"region,omitempty"`
	Note      string `json:"note"`
	Evidence  string `json:"evidence"` // native-http
	LatencyMs int    `json:"latencyMs"`
	CheckedAt string `json:"checkedAt"`
}

type unlockReportPayload struct {
	JobID  string             `json:"jobId"`
	Egress unlockEgressInfo   `json:"egress"`
	Items  []unlockItemResult `json:"items"`
	Error  string             `json:"error,omitempty"`
}

func runNativeUnlock(ctx context.Context) unlockReportPayload {
	proxyClient := newActiveProxyHTTPClient(9 * time.Second)
	directClient := newDirectPhysicalHTTPClient(5 * time.Second)

	var egress unlockEgressInfo
	var directIP string
	var initWG sync.WaitGroup
	initWG.Add(2)
	go func() {
		defer initWG.Done()
		egress = probeEgressInfo(ctx, proxyClient)
	}()
	go func() {
		defer initWG.Done()
		directIP = probeSimplePublicIP(ctx, directClient)
	}()

	checks := []func(context.Context, *http.Client) unlockItemResult{
		checkChatGPT,
		checkClaude,
		checkGemini,
		checkCopilot,
		checkNetflix,
		checkDisneyPlus,
		checkYouTubePremium,
		checkTikTok,
		checkSpotify,
		checkSteam,
		checkDAZN,
	}

	items := make([]unlockItemResult, len(checks))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, fn := range checks {
		wg.Add(1)
		go func(idx int, checkFn func(context.Context, *http.Client) unlockItemResult) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			items[idx] = checkFn(ctx, proxyClient)
		}(i, fn)
	}

	initWG.Wait()
	wg.Wait()
	egress.DirectIP = directIP
	return unlockReportPayload{
		Egress: egress,
		Items:  items,
	}
}

func doUnlockReq(ctx context.Context, client *http.Client, method, u string, body []byte, headers map[string]string) (int, http.Header, string, string, int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	var rdr io.Reader
	if len(body) > 0 {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(reqCtx, method, u, rdr)
	if err != nil {
		return 0, nil, "", "", 0, err
	}
	req.Header.Set("User-Agent", unlockUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	start := time.Now()
	resp, err := client.Do(req)
	lat := max(int(time.Since(start).Milliseconds()), 1)
	if err != nil {
		return 0, nil, "", "", lat, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return resp.StatusCode, resp.Header, string(raw), finalURL, lat, nil
}

func parseCFTrace(body string) map[string]string {
	out := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		if kv := strings.SplitN(strings.TrimSpace(line), "=", 2); len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func probeSimplePublicIP(ctx context.Context, client *http.Client) string {
	code, _, body, _, _, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil, nil)
	if err == nil && code == 200 {
		if ip := parseCFTrace(body)["ip"]; ip != "" {
			return ip
		}
	}
	return ""
}

func probeEgressInfo(ctx context.Context, client *http.Client) unlockEgressInfo {
	var info unlockEgressInfo
	code, _, body, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.cloudflare.com/cdn-cgi/trace", nil, nil)
	if err == nil && code == 200 {
		m := parseCFTrace(body)
		info.IP = m["ip"]
		info.Loc = m["loc"]
		info.Colo = m["colo"]
		info.RTT = lat
	}
	code2, _, body2, _, _, err2 := doUnlockReq(ctx, client, http.MethodGet, "http://ip-api.com/json/?fields=status,query,country,countryCode,city,isp,as,hosting&lang=zh-CN", nil, nil)
	if err2 == nil && code2 == 200 {
		var res struct {
			Status      string `json:"status"`
			Query       string `json:"query"`
			Country     string `json:"country"`
			CountryCode string `json:"countryCode"`
			City        string `json:"city"`
			ISP         string `json:"isp"`
			AS          string `json:"as"`
			Hosting     bool   `json:"hosting"`
		}
		if json.Unmarshal([]byte(body2), &res) == nil && res.Status == "success" {
			if info.IP == "" {
				info.IP = res.Query
			}
			if info.Loc == "" {
				info.Loc = res.CountryCode
			}
			info.Country = res.Country
			info.City = res.City
			info.ISP = res.ISP
			info.Hosting = res.Hosting
			if f := strings.Fields(res.AS); len(f) > 0 {
				info.ASN = f[0]
			}
		}
	}
	return info
}

func newUnlockItem(key, name, group string) unlockItemResult {
	return unlockItemResult{
		Key:       key,
		Name:      name,
		Group:     group,
		Evidence:  "native-http",
		CheckedAt: time.Now().Format(time.RFC3339),
	}
}

var openaiUnsupportedCountries = map[string]bool{
	"CN": true, "HK": true, "MO": true, "RU": true, "BY": true, "IR": true, "KP": true, "SY": true, "CU": true,
}

func checkChatGPT(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("chatgpt", "ChatGPT", "ai")
	code, _, traceBody, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://chatgpt.com/cdn-cgi/trace", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	loc := parseCFTrace(traceBody)["loc"]
	item.Region = loc

	code2, _, body2, _, lat2, err2 := doUnlockReq(ctx, client, http.MethodGet, "https://ios.chat.openai.com/public-api/mobile/server_status/v1", nil, nil)
	if err2 == nil {
		item.LatencyMs = (lat + lat2) / 2
		low := strings.ToLower(body2)
		if strings.Contains(low, "unsupported_country") || strings.Contains(low, "country_not_supported") {
			item.Status = "blocked"
			item.Note = "接口明确拒绝当前出口地区"
			return item
		}
		if code2 == 200 || (code2 == 403 && !strings.Contains(low, "unsupported")) {
			if openaiUnsupportedCountries[loc] {
				item.Status = "blocked"
				item.Note = "出口地区（" + loc + "）不在 OpenAI 支持列表"
				return item
			}
			item.OK = true
			item.Status = "available"
			item.Note = "Web 与 API 均支持当前出口地区"
			return item
		}
	}
	if code == 200 && loc != "" {
		if openaiUnsupportedCountries[loc] {
			item.Status = "blocked"
			item.Note = "出口地区（" + loc + "）不受支持"
		} else {
			item.OK = true
			item.Status = "available"
			item.Note = "支持访问（地区 " + loc + "）"
		}
		return item
	}
	item.Status = "unknown"
	item.Note = "无法确认接口状态"
	return item
}

func checkClaude(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("claude", "Claude", "ai")
	code, _, traceBody, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://claude.ai/cdn-cgi/trace", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	loc := parseCFTrace(traceBody)["loc"]
	item.Region = loc
	code2, _, body2, finalURL, lat2, err2 := doUnlockReq(ctx, client, http.MethodGet, "https://claude.ai/login", nil, nil)
	if err2 == nil {
		item.LatencyMs = (lat + lat2) / 2
		low := strings.ToLower(body2 + " " + finalURL)
		if strings.Contains(low, "app-unavailable-in-region") || strings.Contains(low, "unavailable in your region") || openaiUnsupportedCountries[loc] {
			item.Status = "blocked"
			item.Note = "当前出口地区不支持 Claude"
			return item
		}
		if code2 == 200 || code2 == 403 {
			item.OK = true
			item.Status = "available"
			item.Note = "支持访问 Claude（地区 " + loc + "）"
			return item
		}
	}
	if code == 200 && loc != "" && !openaiUnsupportedCountries[loc] {
		item.OK = true
		item.Status = "available"
		item.Note = "地区 " + loc + " 可用"
	} else {
		item.Status = "blocked"
		item.Note = "当前地区受限"
	}
	return item
}

func checkGemini(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("gemini", "Google Gemini", "ai")
	code, _, body, finalURL, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://gemini.google.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	if strings.Contains(body, "45631641,null,true") || (code == 200 && strings.Contains(finalURL, "gemini.google.com") && !strings.Contains(body, "not available in your country")) {
		item.OK = true
		item.Status = "available"
		item.Note = "原生支持 Google Gemini"
		if m := regexp.MustCompile(`"([A-Z]{2})"`).FindStringSubmatch(body); len(m) == 2 {
			// Best-effort region extraction only if cleanly matched near gl
			if gm := regexp.MustCompile(`"gl":"([A-Z]{2})"`).FindStringSubmatch(body); len(gm) == 2 {
				item.Region = gm[1]
			}
		}
		return item
	}
	item.Status = "blocked"
	item.Note = "出口地区不支持 Gemini"
	return item
}

func checkCopilot(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("copilot", "Microsoft Copilot", "ai")
	code, _, body, finalURL, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://copilot.microsoft.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	low := strings.ToLower(finalURL + " " + body)
	if strings.Contains(low, "cn.bing.com") || strings.Contains(low, "not available in your region") {
		item.Status = "blocked"
		item.Note = "被重定向或地区受限"
		return item
	}
	if code == 200 {
		item.OK = true
		item.Status = "available"
		item.Note = "可正常访问 Copilot"
		return item
	}
	item.Status = "unknown"
	item.Note = "HTTP 状态异常"
	return item
}

var netflixRegionRe = regexp.MustCompile(`netflix\.com/([a-z]{2})(?:-[a-z]+)?/title/`)

func checkNetflix(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("netflix", "Netflix", "streaming")
	// 81280792 = LEGO Ninjago (non-original, requires full unlock)
	code1, hdr1, _, final1, lat1, err1 := doUnlockReq(ctx, client, http.MethodGet, "https://www.netflix.com/title/81280792", nil, nil)
	item.LatencyMs = lat1
	if err1 != nil {
		item.Status = "unknown"
		item.Note = shortErr(err1)
		return item
	}
	extractRegion := func(u string, h http.Header) string {
		if m := netflixRegionRe.FindStringSubmatch(strings.ToLower(u)); len(m) == 2 {
			return strings.ToUpper(m[1])
		}
		if loc := h.Get("X-Originating-URL"); loc != "" {
			if m := netflixRegionRe.FindStringSubmatch(strings.ToLower(loc)); len(m) == 2 {
				return strings.ToUpper(m[1])
			}
		}
		return ""
	}
	if code1 == 200 {
		item.OK = true
		item.Status = "available"
		item.Region = extractRegion(final1, hdr1)
		if item.Region == "" {
			item.Region = "US"
		}
		item.Note = "完整解锁（含非自制剧）"
		return item
	}
	// 80018499 = Netflix Original
	code2, hdr2, _, final2, lat2, err2 := doUnlockReq(ctx, client, http.MethodGet, "https://www.netflix.com/title/80018499", nil, nil)
	if err2 == nil {
		item.LatencyMs = (lat1 + lat2) / 2
	}
	if err2 == nil && code2 == 200 {
		item.OK = true
		item.Status = "originals"
		item.Region = extractRegion(final2, hdr2)
		item.Note = "仅解锁 Netflix 自制剧"
		return item
	}
	item.Status = "blocked"
	item.Note = "当前出口不支持 Netflix"
	return item
}

func checkDisneyPlus(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("disney", "Disney+", "streaming")
	code, _, body, finalURL, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.disneyplus.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	low := strings.ToLower(finalURL + " " + body)
	if code == 403 || strings.Contains(low, "unavailable") || strings.Contains(low, "not available in your region") {
		item.Status = "blocked"
		item.Note = "当前出口地区不可用"
		return item
	}
	if code == 200 {
		item.OK = true
		item.Status = "available"
		if m := regexp.MustCompile(`disneyplus\.com/[a-z]{2}-([a-z]{2})`).FindStringSubmatch(strings.ToLower(finalURL)); len(m) == 2 {
			item.Region = strings.ToUpper(m[1])
		}
		item.Note = "支持访问 Disney+"
		return item
	}
	item.Status = "unknown"
	item.Note = "响应异常"
	return item
}

var ytRegionRe = regexp.MustCompile(`"contentRegion":"([A-Z]{2})"`)

func checkYouTubePremium(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("youtube", "YouTube Premium", "streaming")
	code, _, body, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.youtube.com/premium", nil, map[string]string{
		"Cookie": "CONSENT=YES+cb",
	})
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	if m := ytRegionRe.FindStringSubmatch(body); len(m) == 2 {
		item.Region = m[1]
	}
	if strings.Contains(body, "www.google.cn") || item.Region == "CN" {
		item.Status = "blocked"
		item.Region = "CN"
		item.Note = "IP 已被 Google 判定为中国大陆（送中）"
		return item
	}
	if strings.Contains(body, "Premium is not available in your country") {
		item.Status = "blocked"
		item.Note = "该地区暂不支持 YouTube Premium"
		return item
	}
	if code == 200 {
		item.OK = true
		item.Status = "available"
		item.Note = "正常支持 YouTube Premium"
		return item
	}
	item.Status = "unknown"
	item.Note = "响应异常"
	return item
}

var tiktokRegionRe = regexp.MustCompile(`"region":"([A-Z]{2})"`)

func checkTikTok(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("tiktok", "TikTok", "streaming")
	code, _, body, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.tiktok.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	if m := tiktokRegionRe.FindStringSubmatch(body); len(m) == 2 {
		item.Region = m[1]
		if item.Region == "CN" || item.Region == "HK" {
			item.Status = "blocked"
			item.Note = "当前出口地区受限（" + item.Region + "）"
			return item
		}
		item.OK = true
		item.Status = "available"
		item.Note = "正常解锁 TikTok（地区 " + item.Region + "）"
		return item
	}
	if code == 200 && !strings.Contains(strings.ToLower(body), "not available") {
		item.OK = true
		item.Status = "available"
		item.Note = "可正常访问 TikTok"
		return item
	}
	item.Status = "blocked"
	item.Note = "当前地区受限或被拦截"
	return item
}

func checkSpotify(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("spotify", "Spotify", "streaming")
	code, _, body, finalURL, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://www.spotify.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	if m := regexp.MustCompile(`spotify\.com/([a-z]{2})(?:-[a-z]+)?/`).FindStringSubmatch(strings.ToLower(finalURL)); len(m) == 2 {
		item.Region = strings.ToUpper(m[1])
	}
	if code == 200 && !strings.Contains(strings.ToLower(body), "not available in your country") {
		item.OK = true
		item.Status = "available"
		item.Note = "正常支持 Spotify"
		return item
	}
	item.Status = "blocked"
	item.Note = "地区受限"
	return item
}

func checkSteam(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("steam", "Steam 商店", "gaming")
	code, _, body, _, lat, err := doUnlockReq(ctx, client, http.MethodGet, "https://store.steampowered.com/", nil, nil)
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	if m := regexp.MustCompile(`"country_code":"([A-Z]{2})"`).FindStringSubmatch(body); len(m) == 2 {
		item.Region = m[1]
	}
	if code == 200 {
		item.OK = true
		item.Status = "available"
		item.Note = "可正常访问 Steam 商店"
		return item
	}
	item.Status = "unknown"
	item.Note = "响应异常"
	return item
}

func checkDAZN(ctx context.Context, client *http.Client) unlockItemResult {
	item := newUnlockItem("dazn", "DAZN", "streaming")
	payload := []byte(`{"LandingPageKey":"generic","Platform":"web","PlatformAttributes":{},"Manufacturer":"","Version":"2"}`)
	code, _, body, _, lat, err := doUnlockReq(ctx, client, http.MethodPost, "https://startup.core.indazn.com/misl/v5/Startup", payload, map[string]string{
		"Content-Type": "application/json",
	})
	item.LatencyMs = lat
	if err != nil {
		item.Status = "unknown"
		item.Note = shortErr(err)
		return item
	}
	var res struct {
		Region struct {
			IsAllowed bool   `json:"isAllowed"`
			Country   string `json:"Country"`
		} `json:"Region"`
	}
	if json.Unmarshal([]byte(body), &res) == nil && res.Region.Country != "" {
		item.Region = strings.ToUpper(res.Region.Country)
		if res.Region.IsAllowed {
			item.OK = true
			item.Status = "available"
			item.Note = "支持 DAZN（地区 " + item.Region + "）"
		} else {
			item.Status = "blocked"
			item.Note = "DAZN 拒绝当前出口 IP"
		}
		return item
	}
	if code == 403 {
		item.Status = "blocked"
		item.Note = "DAZN 拒绝当前出口"
		return item
	}
	item.Status = "unknown"
	item.Note = "无法识别响应"
	return item
}
