package xraycore

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const unlockAgentUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

type ServerUnlockItem struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Status string `json:"status"` // available | originals | blocked | unknown
	Region string `json:"region,omitempty"`
	Note   string `json:"note,omitempty"`
}

type ServerUnlockReport struct {
	IP        string             `json:"ip,omitempty"`
	Country   string             `json:"country,omitempty"`
	ISP       string             `json:"isp,omitempty"`
	ASN       string             `json:"asn,omitempty"`
	Hosting   bool               `json:"hosting"`
	CheckedAt string             `json:"checkedAt"`
	Items     []ServerUnlockItem `json:"items"`
}

var (
	srvYTRegionRe = regexp.MustCompile(`"GL":"([A-Z]{2})"`)
	srvTTRegionRe = regexp.MustCompile(`"region":"([A-Z]{2})"`)
)

// RunServerNativeUnlock tests the VPS's own direct outbound IP for major AI & streaming unlocks.
func RunServerNativeUnlock(ctx context.Context) ServerUnlockReport {
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			Proxy:               nil,
			ForceAttemptHTTP2:   true,
			TLSHandshakeTimeout: 6 * time.Second,
		},
	}

	report := ServerUnlockReport{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// 1. Egress IP / ASN / Hosting
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(pCtx, http.MethodGet, "http://ip-api.com/json/?fields=status,query,countryCode,isp,as,hosting", nil)
		if err != nil {
			return
		}
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		var r struct {
			Status      string `json:"status"`
			Query       string `json:"query"`
			CountryCode string `json:"countryCode"`
			ISP         string `json:"isp"`
			AS          string `json:"as"`
			Hosting     bool   `json:"hosting"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&r) == nil && r.Status == "success" {
			report.IP = r.Query
			report.Country = r.CountryCode
			report.ISP = r.ISP
			if fields := strings.Fields(r.AS); len(fields) > 0 {
				report.ASN = fields[0]
			}
			report.Hosting = r.Hosting
		}
	}()

	checks := []func(context.Context, *http.Client) ServerUnlockItem{
		srvCheckChatGPT,
		srvCheckClaude,
		srvCheckGemini,
		srvCheckNetflix,
		srvCheckDisney,
		srvCheckYouTube,
		srvCheckTikTok,
		srvCheckSpotify,
	}
	items := make([]ServerUnlockItem, len(checks))
	for i, fn := range checks {
		wg.Add(1)
		go func(idx int, f func(context.Context, *http.Client) ServerUnlockItem) {
			defer wg.Done()
			items[idx] = f(ctx, client)
		}(i, fn)
	}
	wg.Wait()
	report.Items = items
	return report
}

func srvHTTPGet(ctx context.Context, client *http.Client, urlStr string, limit int64) (int, string, http.Header, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, urlStr, nil)
	if err != nil {
		return 0, "", nil, err
	}
	req.Header.Set("User-Agent", unlockAgentUA)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, limit))
	return resp.StatusCode, string(raw), resp.Header, nil
}

func srvCheckChatGPT(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "chatgpt", Name: "ChatGPT", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://chatgpt.com/cdn-cgi/trace", 2048)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	loc := ""
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "loc=") {
			loc = strings.TrimSpace(strings.TrimPrefix(l, "loc="))
		}
	}
	item.Region = loc
	blockedLoc := map[string]bool{"CN": true, "HK": true, "MO": true, "RU": true, "BY": true, "IR": true, "KP": true}
	if blockedLoc[loc] {
		item.Status = "blocked"
		item.Note = "地区受限 (" + loc + ")"
		return item
	}
	wCode, wBody, _, wErr := srvHTTPGet(ctx, client, "https://api.openai.com/v1/models", 2048)
	if wErr == nil && (wCode == 401 || wCode == 200) && !strings.Contains(strings.ToLower(wBody), "unsupported_country") {
		item.Status = "available"
		return item
	}
	if code == 200 && loc != "" {
		item.Status = "available"
		return item
	}
	item.Status = "blocked"
	return item
}

func srvCheckClaude(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "claude", Name: "Claude", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://claude.ai/cdn-cgi/trace", 2048)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	loc := ""
	for _, l := range strings.Split(body, "\n") {
		if strings.HasPrefix(l, "loc=") {
			loc = strings.TrimSpace(strings.TrimPrefix(l, "loc="))
		}
	}
	item.Region = loc
	blockedLoc := map[string]bool{"CN": true, "HK": true, "MO": true, "RU": true, "BY": true, "IR": true, "KP": true}
	if blockedLoc[loc] {
		item.Status = "blocked"
		return item
	}
	if code == 200 && loc != "" {
		item.Status = "available"
		return item
	}
	item.Status = "blocked"
	return item
}

func srvCheckGemini(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "gemini", Name: "Gemini", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://gemini.google.com/", 32*1024)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	low := strings.ToLower(body)
	if strings.Contains(low, "not available in your country") || strings.Contains(low, "isn't supported in your country") {
		item.Status = "blocked"
		return item
	}
	if code == 200 && strings.Contains(body, "45631641") {
		item.Status = "available"
		return item
	}
	if code == 200 {
		item.Status = "available"
		return item
	}
	item.Status = "blocked"
	return item
}

func srvCheckNetflix(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "netflix", Name: "Netflix", Status: "unknown"}
	// Non-original licensed title (Breaking Bad) -> 200 means Full unlock!
	c1, _, hdr1, err1 := srvHTTPGet(ctx, client, "https://www.netflix.com/title/70143836", 8192)
	if err1 != nil {
		item.Status = "blocked"
		return item
	}
	if c1 == 200 {
		item.Status = "available"
		if loc := hdr1.Get("X-Originating-URL"); loc != "" {
			if parts := strings.Split(loc, "/"); len(parts) > 3 && len(parts[3]) <= 5 {
				item.Region = strings.ToUpper(parts[3])
			}
		}
		return item
	}
	// Netflix Original (Stranger Things) -> 200 means Originals-only
	c2, _, _, err2 := srvHTTPGet(ctx, client, "https://www.netflix.com/title/80057281", 4096)
	if err2 == nil && c2 == 200 {
		item.Status = "originals"
		item.Note = "仅自制剧"
		return item
	}
	item.Status = "blocked"
	return item
}

func srvCheckDisney(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "disney", Name: "Disney+", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://www.disneyplus.com/", 32*1024)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	low := strings.ToLower(body)
	if code == 403 || strings.Contains(low, "unavailable in your region") || strings.Contains(low, "not available in your") {
		item.Status = "blocked"
		return item
	}
	if code == 200 {
		item.Status = "available"
		return item
	}
	item.Status = "blocked"
	return item
}

func srvCheckYouTube(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "youtube", Name: "YouTube", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://www.youtube.com/premium", 64*1024)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	if m := srvYTRegionRe.FindStringSubmatch(body); len(m) == 2 {
		item.Region = m[1]
	}
	if strings.Contains(body, "YouTube Premium is not available in your country") {
		item.Status = "blocked"
		return item
	}
	if code == 200 {
		item.Status = "available"
		return item
	}
	return item
}

func srvCheckTikTok(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "tiktok", Name: "TikTok", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://www.tiktok.com/api/v1/item_list/?count=1", 8192)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	if m := srvTTRegionRe.FindStringSubmatch(body); len(m) == 2 {
		item.Region = m[1]
	}
	if code == 200 && item.Region != "" && item.Region != "CN" && item.Region != "HK" {
		item.Status = "available"
		return item
	}
	if code == 403 {
		item.Status = "blocked"
		return item
	}
	item.Status = "unknown"
	return item
}

func srvCheckSpotify(ctx context.Context, client *http.Client) ServerUnlockItem {
	item := ServerUnlockItem{Key: "spotify", Name: "Spotify", Status: "unknown"}
	code, body, _, err := srvHTTPGet(ctx, client, "https://open.spotify.com/", 16*1024)
	if err != nil {
		item.Status = "blocked"
		return item
	}
	if code == 200 && !strings.Contains(strings.ToLower(body), "not available in your country") {
		item.Status = "available"
		return item
	}
	item.Status = "blocked"
	return item
}
