# PPEELink Server Agent (`agent` 分支)

独立运行的远端 VPS Agent（Go 1.25+，零前端与数据库依赖），负责承载 Xray-core 入站管理、主机监控、原生流媒体/AI 解锁检测、安全防护与远程运维 RPC，与主面板通过独立发行清单解耦发布。

## 核心能力（当前版本 `v1.26`）

- **代理与转发托管 & 秒级唤醒**：自动拉取面板入站与端口转发配置，托管 Xray-core 与无域名自动 TLS 证书；支持防滥用长轮询秒级配置唤醒（单机限 1 连接 + 500ms 变更防抖 + 异常指数退避回退至 60s 周期）。
- **单机 Cloudflare WARP 按域名分流出口**：纯 Go 零依赖自动向 Cloudflare 注册免费 WARP WireGuard 凭据（持久化至 `warp.json`），原生注入 Xray-core `wireguard` 出站（`warp-out`），按预设平台（ChatGPT / Claude / Gemini / Netflix / Disney+ / TikTok / Google）分流解锁。
- **原生解锁矩阵（`unlock-check`）**：由 VPS 本机出口直测 ChatGPT、Claude、Gemini、Netflix（全区/自制及标准国家码）、Disney+、YouTube、TikTok、Spotify，供面板展示并驱动 Clash 订阅智能分流。
- **安全与隐私硬策略（`set-policy`）**：将 `WebShell 终端`、`文件浏览与上传`、`Docker 容器管理`、`允许远程自更新` 持久化到节点本地 `policy.json`，在主机侧直接拒绝关闭的功能请求。
- **面板地址预检迁移（`set-panel-url`）**：收到新面板 URL 后先用当前令牌连通预检 `/api/v1/agent/spec`，验证通过后原子更新 systemd unit 中的 `-panel` 参数（兼容引号/无引号/等号写法）并平滑重启，防止填错域名导致失联。
- **运维与安全体检**：支持内核 BBR / `qdisc` 调优、Xray 核心在线升级、Reality SNI VPS 侧实测评分（`sni-check`）、入站异常扫描审计与 `iptables` 封禁/解封、NextTrace 路由追踪及 iperf3 吞吐互测。

## 构建

需要 Go 1.25+ 与 Python 3：

```sh
./build.sh
```

`dist/` 会生成 `linux/amd64`、`linux/arm64`、`linux/armv7` 三个独立二进制以及带 SHA-256 校验值的 `release.json`。版本号维护在 `agentversion/version.go`（当前 `1.26`）。

## 运行

```sh
export PPEELINK_AGENT_TOKEN='<面板签发的令牌>'
./dist/agent-linux-amd64 agent -panel "https://panel.example.com"
./dist/agent-linux-amd64 --version
```

## 发布到面板

将同一次构建的完整 `dist/` 内容部署至面板运行目录的 `downloads/agent/`：先上传三份二进制，再原子替换 `release.json`。面板会自动识别新版本并支持单机或一键全量静默升级。

## 验证

```sh
go test ./...
PPEELINK_AGENT_SYSTEMD_TESTS=1 go test . -run 'TestAgentTerminalPersistent|TestAgentTerminalTask'
```

## 许可

Agent 本体遵循 [LICENSE](LICENSE)；自动准备的静态 `iperf3` 工具来源、校验与许可规则见 `throughputcore/prepare.go` 与 `throughputcore/licenses/`。
