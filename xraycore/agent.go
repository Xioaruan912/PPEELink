package xraycore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"
)

// Source 期望状态的来源与回报去向：面板本机直接读写数据库，远程 agent 走 HTTPS。
type Source interface {
	Spec(ctx context.Context) (Spec, error)
	Report(ctx context.Context, report Report) error
}

// RunLoop 周期性地拉取期望状态、应用到 xray，并回报流量与运行状态。
// 回报失败时把流量累计到下一轮，避免面板短暂不可达造成漏计。
func RunLoop(ctx context.Context, src Source, runner *Runner, interval time.Duration) {
	pending := map[string]Counter{}
	tick := func() {
		report := Report{Arch: runtime.GOARCH, Traffic: map[string]Counter{}, Online: map[string]int{}}
		spec, err := src.Spec(ctx)
		if err != nil {
			report.Error = "获取配置失败: " + err.Error()
		} else if err := runner.Apply(ctx, spec); err != nil {
			report.Error = err.Error()
		}
		traffic, online, err := runner.Collect(ctx)
		if err != nil && report.Error == "" {
			report.Error = err.Error()
		}
		for email, c := range traffic {
			p := pending[email]
			p.Up += c.Up
			p.Down += c.Down
			pending[email] = p
		}
		report.Traffic = pending
		report.Online = online
		report.Rev = runner.Rev()
		report.Running, _ = runner.Status()
		if _, msg := runner.Status(); report.Error == "" {
			report.Error = msg
		}
		report.XrayVersion = InstalledVersion(runner.Dir)
		if err := src.Report(ctx, report); err != nil {
			log.Printf("[xray] 回报失败（流量将在下一轮补报）: %v", err)
			return
		}
		pending = map[string]Counter{}
	}
	tick()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick()
		}
	}
}

// HTTPSource 远程 agent 使用：凭令牌向面板拉取配置、回报状态。
type HTTPSource struct {
	Panel  string // 面板地址，如 https://panel.example.com
	Token  string
	Client *http.Client
}

func (h *HTTPSource) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(h.Panel, "/")+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+h.Token)
	req.Header.Set("Content-Type", "application/json")
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var envelope struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || resp.StatusCode != http.StatusOK || envelope.Code != "00000" {
		if envelope.Msg != "" {
			return fmt.Errorf("面板返回 HTTP %d: %s", resp.StatusCode, envelope.Msg)
		}
		return fmt.Errorf("面板返回 HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(envelope.Data, out)
	}
	return nil
}

func (h *HTTPSource) Spec(ctx context.Context) (Spec, error) {
	var spec Spec
	err := h.do(ctx, http.MethodGet, "/api/v1/agent/spec", nil, &spec)
	return spec, err
}

func (h *HTTPSource) Report(ctx context.Context, report Report) error {
	return h.do(ctx, http.MethodPost, "/api/v1/agent/report", report, nil)
}
