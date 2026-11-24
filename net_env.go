package main

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
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
	isp := cachedISP
	pubIP := cachedPublicIP
	stale := time.Since(cachedISPAt) > 30*time.Minute
	busy := ispRefreshIn
	if stale && !busy {
		ispRefreshIn = true
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

	dialer := &net.Dialer{
		Timeout: 3 * time.Second,
		Control: func(network, _ string, c syscall.RawConn) error {
			return bindSocketToInterface(network, c, iface)
		},
	}
	if iface != nil && iface.IPv4 != nil {
		dialer.LocalAddr = &net.TCPAddr{IP: iface.IPv4}
	}
	client := &http.Client{
		Timeout: 4 * time.Second,
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

func queryISPAndIP(client *http.Client) (string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3500*time.Millisecond)
	defer cancel()

	// Primary: ip-api.com (direct physical NIC, returns localized Chinese ISP + public IP).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://ip-api.com/json/?fields=status,query,isp,org,as&lang=zh-CN", nil)
	if err == nil {
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				var res struct {
					Status string `json:"status"`
					Query  string `json:"query"`
					ISP    string `json:"isp"`
					Org    string `json:"org"`
					AS     string `json:"as"`
				}
				if json.Unmarshal(raw, &res) == nil && res.Status == "success" {
					isp := normalizeISPLabel(res.ISP + " " + res.Org + " " + res.AS)
					if isp == "" {
						isp = strings.TrimSpace(res.ISP)
						if len(isp) > 24 {
							isp = isp[:24]
						}
					}
					return isp, strings.TrimSpace(res.Query)
				}
			}
		}
	}

	// Fallback: myip.ipip.net
	req2, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://myip.ipip.net", nil)
	if err == nil {
		req2.Header.Set("User-Agent", "curl/8.0")
		if resp, err := client.Do(req2); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
				if label := normalizeISPLabel(string(raw)); label != "" {
					return label, ""
				}
			}
		}
	}
	return "", ""
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
	case strings.Contains(low, "cernet") || strings.Contains(raw, "教育网"):
		return "教育网"
	case strings.Contains(raw, "广电") || strings.Contains(low, "cbn"):
		return "中国广电"
	default:
		return ""
	}
}
