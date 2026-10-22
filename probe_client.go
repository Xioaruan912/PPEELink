package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type probeJobNode struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type probeJobsResponse struct {
	Data struct {
		Nodes           []probeJobNode `json:"nodes"`
		IntervalSeconds int            `json:"intervalSeconds"`
	} `json:"data"`
}

type probeReportResult struct {
	NodeID    int    `json:"nodeId"`
	Kind      string `json:"kind"`
	Success   bool   `json:"success"`
	RTT       int    `json:"rtt"`
	Error     string `json:"error"`
	IPVersion int    `json:"ipVersion"`
}

type probeReportPayload struct {
	Results []probeReportResult `json:"results"`
}

// runProbeClient 在外部网络里运行检测探针：拉取节点、做 TCP 检测、回报面板。
func runProbeClient(args []string) {
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	panel := fs.String("panel", "", "面板地址")
	token := fs.String("token", "", "探针令牌")
	interval := fs.Duration("interval", 5*time.Minute, "检测间隔")
	timeout := fs.Duration("timeout", 3*time.Second, "单个节点超时")
	_ = fs.Parse(args)
	if *panel == "" || *token == "" {
		log.Fatal("需要 -panel 与 -token")
	}
	panelURL := strings.TrimSuffix(*panel, "/")
	client := &http.Client{Timeout: 45 * time.Second}
	log.Printf("[Probe] 启动，面板 %s，间隔 %s", panelURL, *interval)

	for {
		jobs, err := fetchProbeJobs(client, panelURL, *token)
		if err != nil {
			log.Printf("[Probe] 拉取任务失败: %v", err)
			time.Sleep(30 * time.Second)
			continue
		}
		results := make([]probeReportResult, 0, len(jobs.Data.Nodes))
		for _, n := range jobs.Data.Nodes {
			results = append(results, checkProbeNode(n, *timeout))
		}
		if err := reportProbeResults(client, panelURL, *token, results); err != nil {
			log.Printf("[Probe] 回报失败: %v", err)
		} else {
			log.Printf("[Probe] 已检测 %d 个节点", len(results))
		}
		wait := *interval
		if jobs.Data.IntervalSeconds > 0 {
			wait = time.Duration(jobs.Data.IntervalSeconds) * time.Second
		}
		time.Sleep(wait)
	}
}

func fetchProbeJobs(client *http.Client, panel, token string) (*probeJobsResponse, error) {
	req, err := http.NewRequest("GET", panel+"/api/v1/probe/jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Probe "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var out probeJobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

func reportProbeResults(client *http.Client, panel, token string, results []probeReportResult) error {
	payload, err := json.Marshal(probeReportPayload{Results: results})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", panel+"/api/v1/probe/report", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Probe "+token)
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

func checkProbeNode(n probeJobNode, timeout time.Duration) probeReportResult {
	result := probeReportResult{
		NodeID: n.ID, Kind: "tcp", Success: false, RTT: -1,
		IPVersion: probeIPVersion(n.Host),
	}
	addr := net.JoinHostPort(n.Host, strconv.Itoa(n.Port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	_ = conn.Close()
	ms := int(time.Since(start).Milliseconds())
	if ms < 1 {
		ms = 1
	}
	result.Success = true
	result.RTT = ms
	return result
}

func probeIPVersion(host string) int {
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return 4
		}
		return 6
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return 0
	}
	if ips[0].To4() != nil {
		return 4
	}
	return 6
}
