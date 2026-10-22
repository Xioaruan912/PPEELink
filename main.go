package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"ppeelink/api"
	"ppeelink/middlewares"
	"ppeelink/models"
	"ppeelink/routers"
	"ppeelink/settings"
	"ppeelink/utils"
	"ppeelink/xraycore"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

// Include Vite-generated assets whose names may begin with "_" or ".".
// The default embed matcher excludes those files when walking directories,
// which can otherwise produce runtime 404s for chunks such as
// _plugin-vue_export-helper.*.js.
//
//go:embed all:webs/dist/*
var embeddedFiles embed.FS

//go:embed template
var Template embed.FS

// 版本号
var version string

func Templateinit() {
	// 设置template路径
	// 检查目录是否创建
	subFS, err := fs.Sub(Template, "template")
	if err != nil {
		log.Println(err)
		return // 如果出错，直接返回
	}
	entries, err := fs.ReadDir(subFS, ".")
	if err != nil {
		log.Println(err)
		return // 如果出错，直接返回
	}
	// 创建template目录
	_, err = os.Stat("./template")
	if os.IsNotExist(err) {
		err = os.Mkdir("./template", 0666)
		if err != nil {
			log.Println(err)
			return
		}
	}
	// 写入默认模板
	for _, entry := range entries {
		_, err := os.Stat("./template/" + entry.Name())
		//如果文件不存在则写入默认模板
		if os.IsNotExist(err) {
			data, err := fs.ReadFile(subFS, entry.Name())
			if err != nil {
				log.Println(err)
				continue
			}
			err = os.WriteFile("./template/"+entry.Name(), data, 0666)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func main() {
	// 面板按北京时间计算每日流量、月度重置和日志时间，避免服务器使用 UTC 时跨日错位。
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		time.Local = loc
	}
	// agent 模式运行在远程服务器上，不需要面板的配置与数据库
	if len(os.Args) > 1 && os.Args[1] == "agent" {
		runAgent(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "probe" {
		runProbeClient(os.Args[2:])
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "monitor" {
		runMonitorClient(os.Args[2:])
		return
	}
	// 初始化配置
	models.ConfigInit()
	config := models.ReadConfig() // 读取配置文件
	var port = config.Port        // 读取端口号
	// 获取版本号
	var Isversion bool
	version = "2.1"
	flag.BoolVar(&Isversion, "version", false, "显示版本号")
	flag.Parse()
	if Isversion {
		fmt.Println(version)
		return
	}
	// 初始化数据库
	models.InitSqlite()
	// 获取命令行参数
	args := os.Args
	// 如果长度小于2则没有接收到任何参数
	if len(args) < 2 {
		Run(port)
		return
	}
	// 命令行参数选择
	settingCmd := flag.NewFlagSet("setting", flag.ExitOnError)
	var username, password string
	settingCmd.StringVar(&username, "username", "", "设置账号")
	settingCmd.StringVar(&password, "password", "", "设置密码")
	settingCmd.IntVar(&port, "port", 8000, "修改端口")
	switch args[1] {
	// 解析setting命令标志
	case "setting":
		settingCmd.Parse(args[2:])
		settings.ResetUser(username, password)
		return
	case "probe-add":
		runProbeAdd(args[2:])
		return
	case "run":
		settingCmd.Parse(args[2:])
		models.SetConfig(models.Config{
			Port: port,
		}) // 设置端口
		Run(port)
	default:
		return

	}
}

func Run(port int) {
	// 加载 JWT 密钥（必须在配置初始化之后，避免读到空密钥）
	middlewares.InitSecret()
	// 初始化gin框架
	r := gin.Default()
	// Do not trust arbitrary X-Forwarded-For by default. Operators behind a
	// reverse proxy can explicitly set SUBLINKX_TRUSTED_PROXIES to a
	// comma-separated list of proxy IPs/CIDRs.
	trusted := []string{}
	for _, item := range strings.Split(os.Getenv("SUBLINKX_TRUSTED_PROXIES"), ",") {
		if value := strings.TrimSpace(item); value != "" {
			trusted = append(trusted, value)
		}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		log.Printf("可信代理配置无效: %v", err)
	}
	// 初始化日志配置
	utils.Loginit()
	// 初始化模板
	Templateinit()
	// 安装中间件
	r.Use(middlewares.AuthorToken) // jwt验证token
	r.Use(middlewares.AuditTrail)  // 管理写操作审计（不记录请求体）
	// 设置静态资源路径（自定义 handler，支持 gzip + 正确 Content-Length）
	staticFiles, err := fs.Sub(embeddedFiles, "webs/dist")
	if err != nil {
		log.Println(err)
	}
	r.Any("/static/*filepath", middlewares.StaticFS(staticFiles))
	// 站点图标与根路径 HEAD 探测（避免浏览器/监控产生 400/404 噪声）
	r.GET("/favicon.ico", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFiles, "favicon.ico")
		if err != nil {
			c.Status(http.StatusNoContent)
			return
		}
		c.Data(http.StatusOK, "image/x-icon", data)
	})
	r.HEAD("/", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	// 设置模板路径
	r.GET("/", func(c *gin.Context) {
		data, err := fs.ReadFile(staticFiles, "index.html")
		if err != nil {
			c.Error(err)
			return

		}
		c.Data(200, "text/html", data)
	})
	// 注册路由
	routers.User(r)
	routers.Mentus(r)
	routers.Subcription(r)
	routers.Nodes(r)
	routers.Clients(r)
	routers.Total(r)
	routers.Templates(r)
	routers.Version(r, version)
	routers.Rules(r)             // 规则中心
	routers.Tasks(r)             // 后台任务中心
	routers.Tokens(r)            // API Token 管理
	routers.Ops(r)               // 安全运维/备份
	routers.Status(r)            // 公共只读状态页
	routers.Audit(r)             // 管理操作审计
	routers.ShortLinks(r)        // 内部短链与一键导入
	routers.Proxy(r)             // 自建节点与到期/用量告警
	routers.Probes(r)            // 多视角检测探针
	routers.Monitors(r)          // 服务器监控
	routers.MonitorOperations(r) // SSH 文件与 Docker 管理
	routers.NetworkTopology(r)   // 真实关联网络拓扑
	routers.NetworkTraces(r)     // 多地路由追踪
	// 启动时合并旧的自建分组并同步单个「自建」分组。
	if err := api.SyncSelfHostedNodes(); err != nil {
		log.Printf("[主程序] 同步自建节点失败: %v", err)
	}
	// 同一台机器同时存在独立监控和 agent 监控时，保留独立监控并合并指标。
	if err := api.MergeDuplicateMonitors(); err != nil {
		log.Printf("[主程序] 合并重复监控失败: %v", err)
	}
	// 启动后台定时测活任务 (Cron)
	StartCronTasks()
	api.StartLocalProxy() // 有本机自建服务器时启动 xray 同步

	// 启动服务
	r.Run(fmt.Sprintf("0.0.0.0:%d", port))
}

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	runner := xraycore.NewRunner(*dir, "ppeelink-xray")
	log.Printf("[agent] 已启动，面板 %s，每 %s 同步一次", *panel, *interval)
	go runAgentTraceLoop(ctx, &http.Client{Timeout: 20 * time.Second}, *panel, *token)
	if xraycore.RunLoop(ctx, &xraycore.HTTPSource{Panel: *panel, Token: *token}, runner, *interval) {
		log.Printf("[agent] 服务器已在面板删除，xray 已卸载，正在移除 agent")
		removeAgent()
		return
	}
	runner.Stop()
}

// removeAgent 删除安装脚本创建的 agent 文件与 systemd 服务。最后一步停止服务会结束本进程。
func removeAgent() {
	_ = os.RemoveAll("/etc/ppeelink-agent")
	if exe, err := os.Executable(); err == nil && filepath.Base(exe) == "ppeelink-agent" {
		_ = os.Remove(exe)
	}
	if os.Getenv("INVOCATION_ID") == "" {
		return // 不是由 systemd 启动的，没有服务需要清理
	}
	_ = exec.Command("systemctl", "disable", "ppeelink-agent").Run()
	_ = os.Remove("/etc/systemd/system/ppeelink-agent.service")
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "stop", "--no-block", "ppeelink-agent").Run()
}
