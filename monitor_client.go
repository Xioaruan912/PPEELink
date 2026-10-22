package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"ppeelink/monitorcore"
)

type monitorReportPayload struct {
	Hostname string             `json:"hostname"`
	OS       string             `json:"os"`
	Arch     string             `json:"arch"`
	Version  string             `json:"version"`
	Metrics  monitorcore.Metric `json:"metrics"`
}

type monitorConfigResponse struct {
	Data struct {
		IntervalSeconds int `json:"intervalSeconds"`
	} `json:"data"`
}

func runMonitorClient(args []string) {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	panel := fs.String("panel", "", "面板地址")
	token := fs.String("token", "", "监控令牌")
	interval := fs.Duration("interval", 60*time.Second, "上报间隔")
	_ = fs.Parse(args)
	if *panel == "" || *token == "" {
		log.Fatal("需要 -panel 与 -token")
	}
	panelURL := strings.TrimSuffix(*panel, "/")
	client := &http.Client{Timeout: 30 * time.Second}
	if secs := fetchMonitorInterval(client, panelURL, *token); secs > 0 {
		*interval = time.Duration(secs) * time.Second
	}
	hostname, _ := os.Hostname()
	log.Printf("[Monitor] 启动，面板 %s，间隔 %s", panelURL, *interval)

	collector := monitorcore.NewCollector()
	// 预热一次并等待 2 秒，让第一份上报就有可用的 CPU / 网络速度。
	collector.Collect()
	time.Sleep(2 * time.Second)

	for {
		metrics := collector.Collect()
		report := monitorReportPayload{
			Hostname: hostname, OS: runtime.GOOS, Arch: runtime.GOARCH,
			Version: monitorVersion(), Metrics: metrics,
		}
		if err := reportMonitor(client, panelURL, *token, report); err != nil {
			log.Printf("[Monitor] 上报失败: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}
		time.Sleep(*interval)
	}
}

func monitorVersion() string {
	if version != "" {
		return version
	}
	return "2.1"
}

func fetchMonitorInterval(client *http.Client, panel, token string) int {
	req, err := http.NewRequest("GET", panel+"/api/v1/monitor/config", nil)
	if err != nil {
		return 0
	}
	req.Header.Set("Authorization", "Monitor "+token)
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Monitor] 拉取配置失败: %v", err)
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("[Monitor] 拉取配置失败: HTTP %d", resp.StatusCode)
		return 0
	}
	var out monitorConfigResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0
	}
	return out.Data.IntervalSeconds
}

func reportMonitor(client *http.Client, panel, token string, report monitorReportPayload) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", panel+"/api/v1/monitor/report", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Monitor "+token)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
