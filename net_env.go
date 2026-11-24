package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

type localNetEnv struct {
	HasIPv4   bool
	HasIPv6   bool
	IfaceName string
	ISP       string
	PublicIP  string
}

var (
	ispMu          sync.Mutex
	cachedISP      string
	cachedPublicIP string
	cachedISPAt    time.Time
	ispRefreshIn   bool
	ipv4ExtractRe  = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
)

func detectLocalNetEnv() localNetEnv {
	v4 := findPhysicalInterface(false)
	v6 := findPhysicalInterface(true)
	env := localNetEnv{
		HasIPv4: v4 != nil && v4.IPv4 != nil,
		HasIPv6: v6 != nil && v6.IPv6 != nil,
	}
	if v4 != nil {
		env.IfaceName = v4.Name
	} else if v6 != nil {
		env.IfaceName = v6.Name
	}
	env.ISP, env.PublicIP = currentISPAndIP(v4)
	return env
}

func currentISPAndIP(v4 *physicalInterface) (string, string) {
	ispMu.Lock()
	firstRun := cachedISPAt.IsZero()
	isp := cachedISP
	pubIP := cachedPublicIP
	// Retry after 60s when either value is missing; otherwise refresh every 30m.
	ttl := 30 * time.Minute
	if isp == "" || pubIP == "" {
		ttl = 60 * time.Second
	}
	stale := firstRun || time.Since(cachedISPAt) > ttl
	busy := ispRefreshIn
	if stale && !busy {
		ispRefreshIn = true
		if firstRun {
			// On startup, wait briefly (up to ~4s) so the very first heartbeat already carries real physical IP + ISP.
			ispMu.Unlock()
			refreshISPBg(v4)
			ispMu.Lock()
			isp = cachedISP
			pubIP = cachedPublicIP
			ispMu.Unlock()
			return isp, pubIP
		}
		go refreshISPBg(v4)
	}
	ispMu.Unlock()
	return isp, pubIP
}

func refreshISPBg(iface *physicalInterface) {
	defer func() {
		ispMu.Lock()
		ispRefreshIn = false
		ispMu.Unlock()
	}()

	dialer := newPhysicalDialer(iface, 4*time.Second)
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy:       nil,
			DialContext: dialer.DialContext,
		},
	}

	isp, pubIP := queryISPAndIP(client)
	ispMu.Lock()
	if isp != "" {
		cachedISP = isp
	}
	if pubIP != "" {
		cachedPublicIP = pubIP
	}
	cachedISPAt = time.Now()
	ispMu.Unlock()
}

func extractPublicIPv4(raw string) string {
	for _, m := range ipv4ExtractRe.FindAllString(raw, 4) {
		ip := net.ParseIP(m)
		if ip != nil && ip.To4() != nil && !ip.IsPrivate() && !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}

func queryISPAndIP(client *http.Client) (string, string) {
	var isp, pubIP string

	// 1. China-friendly myip.ipip.net (fast even on CERNET/campus networks, returns IPv4 + ISP).
	{
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://myip.ipip.net", nil)
		if err == nil {
			req.Header.Set("User-Agent", "curl/8.0")
			if resp, err := client.Do(req); err == nil {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					text := string(raw)
					if ip := extractPublicIPv4(text); ip != "" {
						pubIP = ip
					}
					if label := normalizeISPLabel(text); label != "" {
						isp = label
					}
				}
			}
		}
		cancel()
	}

	// 2. Fallback pure IPv4 endpoint (4.ipw.cn) if pubIP still empty.
	if pubIP == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 3500*time.Millisecond)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://4.ipw.cn", nil)
		if err == nil {
			req.Header.Set("User-Agent", "curl/8.0")
			if resp, err := client.Do(req); err == nil {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 128))
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					if ip := extractPublicIPv4(string(raw)); ip != "" {
						pubIP = ip
					}
				}
			}
		}
		cancel()
	}

	// 3. ip-api.com (fills ISP and/or public IP when not yet detected).
	if isp == "" || pubIP == "" {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/?fields=status,query,isp,org,as&lang=zh-CN", nil)
		if err == nil {
			if resp, err := client.Do(req); err == nil {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var res struct {
						Status string `json:"status"`
						Query  string `json:"query"`
						ISP    string `json:"isp"`
						Org    string `json:"org"`
						AS     string `json:"as"`
					}
					if json.Unmarshal(raw, &res) == nil && res.Status == "success" {
						if pubIP == "" {
							pubIP = strings.TrimSpace(res.Query)
						}
						if isp == "" {
							isp = normalizeISPLabel(res.ISP + " " + res.Org + " " + res.AS)
							if isp == "" {
								isp = strings.TrimSpace(res.ISP)
								if len(isp) > 24 {
									isp = isp[:24]
								}
							}
						}
					}
				}
			}
		}
		cancel()
	}

	return isp, pubIP
}

func normalizeISPLabel(raw string) string {
	low := strings.ToLower(raw)
	switch {
	case strings.Contains(low, "chinanet") || strings.Contains(low, "chinatelecom") || strings.Contains(low, "china telecom") || strings.Contains(raw, "电信") || strings.Contains(low, "as4134") || strings.Contains(low, "as4809"):
		return "中国电信"
	case strings.Contains(low, "unicom") || strings.Contains(raw, "联通") || strings.Contains(low, "as4837") || strings.Contains(low, "as9929"):
		return "中国联通"
	case strings.Contains(low, "china mobile") || strings.Contains(low, "cmnet") || strings.Contains(raw, "移动") || strings.Contains(low, "as9808") || strings.Contains(low, "as56040"):
		return "中国移动"
	case strings.Contains(low, "cernet") || strings.Contains(low, "as4538") || strings.Contains(raw, "教育网") || strings.Contains(low, "education"):
		return "教育网"
	case strings.Contains(raw, "广电") || strings.Contains(low, "cbn"):
		return "中国广电"
	default:
		return ""
	}
}
