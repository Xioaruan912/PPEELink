package xraycore

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
	"strings"
	"time"

	"ppeelink/monitorcore"
)

// Source 期望状态的来源与回报去向：面板本机直接读写数据库，远程 agent 走 HTTPS。
// AgentVersion 由 agent 主程序注入，随每次回报发送给面板。
var AgentVersion = "dev"

type Source interface {
	Spec(ctx context.Context) (Spec, error)
	Report(ctx context.Context, report Report) error
}

// Updater 是可自更新的远端来源。HTTPSource 实现；面板本机来源不需要。
type Updater interface {
	Update(ctx context.Context, hash string) error
}

// RunLoop 周期性地拉取期望状态、应用到 xray，并回报流量与运行状态。
// 回报失败时把流量累计到下一轮，避免面板短暂不可达造成漏计。
// 面板要求卸载时，卸载 xray 并回报成功后返回 true。
func RunLoop(ctx context.Context, src Source, runner *Runner, interval time.Duration) bool {
	if interval <= 0 {
		interval = time.Minute
	}
	pending := map[string]Counter{}
	websiteBatchID, pendingWebsites, spoolLoadErr := loadWebsiteBatch(runner.Dir)
	ackWebsites := func() {
		if len(pendingWebsites) > 0 {
			if err := clearWebsiteBatch(runner.Dir); err != nil {
				// Replaying an acknowledged on-disk batch is safe: the panel
				// retains its receipt. Never change its ID on restore.
				log.Printf("[agent] 清理已确认网站批次失败: %v", err)
			}
		}
		pendingWebsites, websiteBatchID = nil, ""
	}
	collectWebsites := func() error {
		if spoolLoadErr != nil {
			return fmt.Errorf("恢复网站流量批次失败: %w", spoolLoadErr)
		}
		// Never add newly drained counters to a batch which may already have
		// committed remotely. Retries must retain both ID and exact payload.
		if len(pendingWebsites) == 0 {
			var id [16]byte
			if _, err := rand.Read(id[:]); err != nil {
				return fmt.Errorf("创建网站流量批次失败: %w", err)
			}
			pendingWebsites = runner.CollectWebsiteTraffic()
			if len(pendingWebsites) > 0 {
				websiteBatchID = hex.EncodeToString(id[:])
			}
		}
		if len(pendingWebsites) > 0 {
			return saveWebsiteBatch(runner.Dir, websiteBatchID, pendingWebsites)
		}
		return nil
	}
	collector := monitorcore.NewCollector()
	var lastMonitor time.Time
	tick := func() bool {
		if ctx.Err() != nil {
			return false
		}
		report := Report{ProxyFeatures: 2, Arch: runtime.GOARCH, AgentVersion: AgentVersion, AgentHash: ExecutableHash(), Traffic: map[string]Counter{}, Online: map[string]int{}}
		spec, err := src.Spec(ctx)
		if err == nil && spec.Uninstall {
			// 先收最后一轮流量，再卸载
			traffic, _, _ := runner.Collect(ctx)
			for email, c := range traffic {
				p := pending[email]
				p.Up += c.Up
				p.Down += c.Down
				pending[email] = p
			}
			// Stop forwarding before draining final counters. Do not delete the
			// spool directory until every batch has a confirmed panel receipt.
			if err := runner.Apply(ctx, Spec{Enabled: false}); err != nil {
				report.Error = "卸载前停止代理失败: " + err.Error()
				_ = src.Report(ctx, report)
				return false
			}
			for {
				if err := collectWebsites(); err != nil {
					report.Error = err.Error()
					_ = src.Report(ctx, report)
					return false
				}
				if len(pendingWebsites) == 0 {
					break
				}
				report.WebsiteTraffic, report.WebsiteBatchID = pendingWebsites, websiteBatchID
				if err := src.Report(ctx, report); err != nil {
					return false
				}
				ackWebsites()
			}
			report.WebsiteTraffic, report.WebsiteBatchID = nil, ""
			if err := runner.Uninstall(); err != nil {
				log.Printf("[xray] 卸载失败: %v", err)
				report.Error = "卸载失败: " + err.Error()
			} else {
				report.Uninstalled = true
			}
			report.Traffic = pending
			if err := src.Report(ctx, report); err != nil {
				log.Printf("[xray] 回报卸载结果失败: %v", err)
				return false
			}
			return report.Uninstalled
		}
		if err != nil {
			report.Error = "获取配置失败: " + err.Error()
		} else {
			if spec.AgentUpdate && spec.AgentHash != "" && report.AgentHash != spec.AgentHash {
				if updater, ok := src.(Updater); ok {
					// Persist/flush queued website data before replacing this process.
					flushErr := collectWebsites()
					if flushErr == nil && len(pendingWebsites) > 0 {
						report.WebsiteTraffic, report.WebsiteBatchID = pendingWebsites, websiteBatchID
						report.Rev = runner.Rev()
						report.Running, _ = runner.Status()
						flushErr = src.Report(ctx, report)
						if flushErr == nil {
							ackWebsites()
						}
						report.WebsiteTraffic, report.WebsiteBatchID = nil, ""
					}
					if flushErr != nil {
						report.Error = "agent 更新前网站流量补报失败: " + flushErr.Error()
					} else if updateErr := updater.Update(ctx, spec.AgentHash); updateErr != nil {
						report.Error = "agent 自更新失败: " + updateErr.Error()
					}
				}
			}
			if applyErr := runner.Apply(ctx, spec); applyErr != nil {
				if report.Error != "" {
					report.Error = errors.Join(errors.New(report.Error), applyErr).Error()
				} else {
					report.Error = applyErr.Error()
				}
			}
		}
		if spec.Monitor != nil && spec.Monitor.Enabled {
			interval := time.Duration(spec.Monitor.IntervalSeconds) * time.Second
			if interval <= 0 {
				interval = time.Minute
			}
			if lastMonitor.IsZero() || time.Since(lastMonitor) >= interval {
				metric := collector.Collect()
				report.Monitor = &metric
				lastMonitor = time.Now()
			}
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
		if err := collectWebsites(); err != nil {
			if report.Error != "" {
				report.Error += "; "
			}
			report.Error += "网站流量持久化失败: " + err.Error()
		}
		report.WebsiteTraffic, report.WebsiteBatchID = pendingWebsites, websiteBatchID
		report.Traffic = pending
		report.Online = online
		report.Rev = runner.Rev()
		var runtimeError string
		report.Running, runtimeError = runner.Status()
		if report.Error == "" {
			report.Error = runtimeError
		}
		report.XrayVersion = InstalledVersion(runner.Dir)
		report.Certificates = runner.CertificateReports()
		if err := src.Report(ctx, report); err != nil {
			log.Printf("[xray] 回报失败（流量将在下一轮补报）: %v", err)
			return false
		}
		pending = map[string]Counter{}
		ackWebsites()
		return false
	}
	if tick() {
		return true
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if tick() {
				return true
			}
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
