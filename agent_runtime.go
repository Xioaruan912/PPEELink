package main

import (
	"context"
	"flag"
	"github.com/Xioaruan912/PPEELink/agentversion"
	"github.com/Xioaruan912/PPEELink/xraycore"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// runAgent 远程节点：只负责运行 xray，配置与流量都与面板同步。
func runAgent(args []string) {
	cmd := flag.NewFlagSet("agent", flag.ExitOnError)
	panel := cmd.String("panel", "", "面板地址，如 https://panel.example.com")
	token := cmd.String("token", os.Getenv("PPEELINK_AGENT_TOKEN"), "agent 令牌")
	dir := cmd.String("dir", "/var/lib/ppeelink-agent", "xray 安装与配置目录")
	interval := cmd.Duration("interval", 60*time.Second, "同步间隔")
	_ = cmd.Parse(args)
	if *panel == "" || *token == "" {
		log.Fatal("需要 -panel 与 -token")
	}
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		log.Fatal(err)
	}
	xraycore.AgentVersion = agentversion.Current
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runner := xraycore.NewRunner(*dir, "ppeelink-xray")
	log.Printf("[agent] 已启动，面板 %s，每 %s 同步一次", *panel, *interval)
	agentClient := &http.Client{Timeout: 20 * time.Second}
	go runAgentTraceLoop(ctx, agentClient, *panel, *token)
	go runAgentThroughputLoop(ctx, agentClient, *panel, *token)
	go runAgentCheckLoop(ctx, agentClient, *panel, *token)
	go runAgentTerminal(ctx, *panel, *token)
	if xraycore.RunLoop(ctx, &xraycore.HTTPSource{Panel: *panel, Token: *token, Client: agentClient}, runner, *interval) {
		log.Printf("[agent] 服务器已在面板删除，xray 已卸载，正在移除 agent")
		removeAgent()
		return
	}
	runner.Stop()
}

// removeAgent 删除安装脚本创建的 agent 文件与 systemd 服务。最后一步停止服务会结束本进程。
func removeAgent() {
	service := "ppeelink-agent"
	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		service = "ppeelink-monitor"
	}
	_ = os.RemoveAll("/etc/" + service)
	if exe, err := os.Executable(); err == nil && filepath.Base(exe) == service {
		_ = os.Remove(exe)
	}
	if os.Getenv("INVOCATION_ID") == "" {
		return // 不是由 systemd 启动的，没有服务需要清理
	}
	_ = exec.Command("systemctl", "disable", service).Run()
	_ = os.Remove("/etc/systemd/system/" + service + ".service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "stop", "--no-block", service).Run()
}
