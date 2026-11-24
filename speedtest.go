package main

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type speedSample struct {
	ElapsedMs int     `json:"elapsedMs"`
	Mbps      float64 `json:"mbps"`
}

type speedChannelResult struct {
	Mode         string        `json:"mode"`             // proxy | direct
	Target       string        `json:"target,omitempty"` // human-readable speedtest server label
	DownloadMbps float64       `json:"downloadMbps"`
	PeakMbps     float64       `json:"peakMbps"`
	UploadMbps   float64       `json:"uploadMbps"`
	TTFBMs       int           `json:"ttfbMs"`
	JitterMs     int           `json:"jitterMs"`
	BytesDown    int64         `json:"bytesDown"`
	BytesUp      int64         `json:"bytesUp"`
	DurationMs   int           `json:"durationMs"`
	Samples      []speedSample `json:"samples"`
	Error        string        `json:"error,omitempty"`
}

type speedtestReportPayload struct {
	JobID  string              `json:"jobId"`
	Mode   string              `json:"mode"` // proxy | direct | both
	Proxy  *speedChannelResult `json:"proxy,omitempty"`
	Direct *speedChannelResult `json:"direct,omitempty"`
	Error  string              `json:"error,omitempty"`
}

type speedTarget struct {
	Label    string
	ProbeURL string
	DownURL  string
	UpURL    string
}

var (
	proxySpeedTarget = speedTarget{
		Label:    "Cloudflare 边缘 (代理出口)",
		ProbeURL: "https://speed.cloudflare.com/__down?bytes=0",
		DownURL:  "https://speed.cloudflare.com/__down?bytes=25000000",
		UpURL:    "https://speed.cloudflare.com/__up",
	}

	directSpeedCandidates = []speedTarget{
		{
			Label:    "腾讯国内 CDN (物理直连)",
			ProbeURL: "https://dldir1v6.qq.com/weixin/Windows/WeChatSetup.exe",
			DownURL:  "https://dldir1v6.qq.com/weixin/Windows/WeChatSetup.exe",
			UpURL:    "https://speed.cloudflare.com/__up",
		},
		{
			Label:    "阿里云镜像 (物理直连)",
			ProbeURL: "https://mirrors.aliyun.com/ubuntu/ls-lR.gz",
			DownURL:  "https://mirrors.aliyun.com/ubuntu/ls-lR.gz",
			UpURL:    "https://speed.cloudflare.com/__up",
		},
		{
			Label:    "清华 TUNA 镜像 (教育网直连)",
			ProbeURL: "https://mirrors.tuna.tsinghua.edu.cn/ubuntu/ls-lR.gz",
			DownURL:  "https://mirrors.tuna.tsinghua.edu.cn/ubuntu/ls-lR.gz",
			UpURL:    "https://speed.cloudflare.com/__up",
		},
		{
			Label:    "Cloudflare 直连",
			ProbeURL: "https://speed.cloudflare.com/__down?bytes=0",
			DownURL:  "https://speed.cloudflare.com/__down?bytes=25000000",
			UpURL:    "https://speed.cloudflare.com/__up",
		},
	}
)

// pickDirectSpeedTarget races the China domestic CDN + Cloudflare candidates
// over the physical NIC (Fake-IP-free DNS) and returns the first reachable one.
func pickDirectSpeedTarget(ctx context.Context, client *http.Client) speedTarget {
	probeCtx, cancel := context.WithTimeout(ctx, 2800*time.Millisecond)
	defer cancel()

	ch := make(chan speedTarget, len(directSpeedCandidates))
	for _, cand := range directSpeedCandidates {
		go func(t speedTarget) {
			req, err := http.NewRequestWithContext(probeCtx, http.MethodHead, t.ProbeURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", unlockUA)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				select {
				case ch <- t:
				default:
				}
			}
		}(cand)
	}

	select {
	case winner := <-ch:
		return winner
	case <-probeCtx.Done():
		return directSpeedCandidates[0]
	}
}

func runUserSpeedtest(ctx context.Context, mode string) speedtestReportPayload {
	if mode == "" {
		mode = "proxy"
	}
	out := speedtestReportPayload{Mode: mode}
	if mode == "proxy" || mode == "both" {
		client := newActiveProxyHTTPClient(12 * time.Second)
		r := runSingleChannelSpeedtest(ctx, client, "proxy", proxySpeedTarget, 5*time.Second)
		out.Proxy = &r
	}
	if mode == "direct" || mode == "both" {
		client := newDirectPhysicalHTTPClient(12 * time.Second)
		dur := 5 * time.Second
		if mode == "both" {
			dur = 4 * time.Second
		}
		target := pickDirectSpeedTarget(ctx, client)
		r := runSingleChannelSpeedtest(ctx, client, "direct", target, dur)
		out.Direct = &r
	}
	return out
}

func runSingleChannelSpeedtest(ctx context.Context, client *http.Client, mode string, target speedTarget, downDur time.Duration) speedChannelResult {
	res := speedChannelResult{
		Mode:    mode,
		Target:  target.Label,
		Samples: []speedSample{},
	}

	// 1. Measure TTFB & Jitter with 3 lightweight HEAD / zero-byte probes.
	probeMethod := http.MethodHead
	if target.ProbeURL == proxySpeedTarget.ProbeURL {
		probeMethod = http.MethodGet
	}
	var rtts []int
	for i := 0; i < 3; i++ {
		pCtx, cancel := context.WithTimeout(ctx, 3500*time.Millisecond)
		req, _ := http.NewRequestWithContext(pCtx, probeMethod, target.ProbeURL, nil)
		req.Header.Set("User-Agent", unlockUA)
		t0 := time.Now()
		resp, err := client.Do(req)
		elapsed := max(int(time.Since(t0).Milliseconds()), 1)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				rtts = append(rtts, elapsed)
			}
		}
		cancel()
	}
	if len(rtts) > 0 {
		sum := 0
		for _, v := range rtts {
			sum += v
		}
		res.TTFBMs = int(math.Round(float64(sum) / float64(len(rtts))))
		if len(rtts) > 1 {
			jSum := 0
			for i := 1; i < len(rtts); i++ {
				d := rtts[i] - rtts[i-1]
				if d < 0 {
					d = -d
				}
				jSum += d
			}
			res.JitterMs = int(math.Round(float64(jSum) / float64(len(rtts)-1)))
		}
	}

	// 2. Concurrent streaming download test (capped strictly by downDur).
	dlCtx, cancelDL := context.WithTimeout(ctx, downDur)
	defer cancelDL()

	var totalDown atomic.Int64
	var dlErrMu sync.Mutex
	var firstErr string
	startDL := time.Now()

	var wg sync.WaitGroup
	const streams = 3
	for i := 0; i < streams; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, target.DownURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", unlockUA)
			resp, err := client.Do(req)
			if err != nil {
				dlErrMu.Lock()
				if firstErr == "" && dlCtx.Err() == nil {
					firstErr = shortErr(err)
				}
				dlErrMu.Unlock()
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				dlErrMu.Lock()
				if firstErr == "" {
					firstErr = resp.Status
				}
				dlErrMu.Unlock()
				return
			}
			buf := make([]byte, 32*1024)
			for {
				n, rerr := resp.Body.Read(buf)
				if n > 0 {
					totalDown.Add(int64(n))
				}
				if rerr != nil || dlCtx.Err() != nil {
					break
				}
			}
		}()
	}

	// Sample throughput every 350ms for smooth Apple monotone curve.
	sampleTicker := time.NewTicker(350 * time.Millisecond)
	var prevBytes int64
	prevTime := startDL
	peakMbps := 0.0

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

loop:
	for {
		select {
		case <-doneCh:
			break loop
		case now := <-sampleTicker.C:
			cur := totalDown.Load()
			dt := now.Sub(prevTime).Seconds()
			if dt > 0.05 {
				mbps := round2(float64(cur-prevBytes) * 8.0 / (dt * 1_000_000.0))
				if mbps > peakMbps {
					peakMbps = mbps
				}
				res.Samples = append(res.Samples, speedSample{
					ElapsedMs: int(now.Sub(startDL).Milliseconds()),
					Mbps:      mbps,
				})
				prevBytes = cur
				prevTime = now
			}
		}
	}
	sampleTicker.Stop()

	elapsedSec := time.Since(startDL).Seconds()
	bytesDown := totalDown.Load()
	res.BytesDown = bytesDown
	res.DurationMs = int(time.Since(startDL).Milliseconds())
	if bytesDown > 0 && elapsedSec > 0.05 {
		res.DownloadMbps = round2(float64(bytesDown) * 8.0 / (elapsedSec * 1_000_000.0))
		if peakMbps < res.DownloadMbps {
			peakMbps = res.DownloadMbps
		}
		res.PeakMbps = round2(peakMbps)
	} else if firstErr != "" {
		res.Error = firstErr
	}

	// 3. Lightweight 2.8-second upload probe (4MB payload max).
	if target.UpURL != "" {
		upCtx, cancelUP := context.WithTimeout(ctx, 2800*time.Millisecond)
		defer cancelUP()
		upPayload := make([]byte, 4*1024*1024)
		upStart := time.Now()
		upReq, err := http.NewRequestWithContext(upCtx, http.MethodPost, target.UpURL, bytes.NewReader(upPayload))
		if err == nil {
			upReq.Header.Set("User-Agent", unlockUA)
			upReq.Header.Set("Content-Type", "application/octet-stream")
			if upResp, err := client.Do(upReq); err == nil {
				_, _ = io.Copy(io.Discard, upResp.Body)
				upResp.Body.Close()
				upSec := time.Since(upStart).Seconds()
				if upSec > 0.05 {
					res.BytesUp = int64(len(upPayload))
					res.UploadMbps = round2(float64(len(upPayload)) * 8.0 / (upSec * 1_000_000.0))
				}
			}
		}
	}

	return res
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
