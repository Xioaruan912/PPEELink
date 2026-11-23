# AGENTS.md — PPEELink 工程协作与开发工作流规范

> **【强制规则 · 只允许续写，严禁覆盖】**
>
> 1. 本文件是 **PPEELink** 仓库的权威 AI 工程协作手册，由主维护流程沉淀而成。
> 2. **任何后续接手的 AI Agent 仅允许在文末 `## 后续 Agent 续写日志（Append-Only）` 区域按时间顺序追加记录，严禁覆盖、重写、删减或篡改前文已确立的工作流、架构红线与审美标准。**
> 3. **严禁在 `git commit` 中添加 `Co-Authored-By:` 署名**，避免污染仓库 Contributors 列表。
> 4. 仓库仅维护 `main`（面板）与 `agent`（独立 Agent）两个长期分支，不遗留临时分支。

---

## 1. 仓库与双分支架构

| 分支 | 定位 | 技术栈 | 构建与产物 |
| :--- | :--- | :--- | :--- |
| **`main`** | 主面板（后端 + 嵌入式前端） | Go 1.25 + Gin + GORM + SQLite (WAL)<br>Vue 3 + TypeScript + Vite + Element Plus + SCSS + ECharts + GSAP | 前端编译至 `webs/dist/`，经 `//go:embed all:webs/dist/*` 打包进单一二进制 |
| **`agent`** | 节点端独立守护进程 | 独立 Go 模块（Xray-core、tmux、NextTrace、iperf3、WS RPC） | 交叉编译 `linux/amd64`、`linux/arm64`、`linux/armv7` + `release.json`，放至面板运行目录 `downloads/agent/` |

### 核心目录速览（`main`）
- `api/`：HTTP/WebSocket 接口、节点概览（`node_overview.go`）、Agent 代理与终端桥接、订阅输出与安全认证。
- `models/`：GORM 模型与 SQLite 性能调优（WAL、mmap、缓存、复合索引）。
- `middlewares/`：安全响应头、令牌桶限流、池化 Gzip、JWT 登出吊销与 API Token 节流。
- `node/`：协议解析、国家/地区识别（`country.go`）、国内 TCP Ping（`china_ping.go`）。
- `rulecenter/`：上游分流规则解析与按 `(RuleCount, NormalizedContentHash)` 自动去重。
- `xraycore/`：入站模板生成、Reality 目标校验（自动拦截 Cloudflare CDN IP/证书）。
- `webs/src/`：前端源码，视觉与组件规范见 [webs/DESIGN.md](webs/DESIGN.md)。

---

## 2. 标准开发工作流

### 阶段一：调研与定位（改代码前必做）
1. 先阅读目标模块的源码与现有单测，确认既有约定和边界条件，不凭猜测修改。
2. 涉及前端 UI 或新增页面时，先读 [webs/DESIGN.md](webs/DESIGN.md)，延续 **Three stones** 石色调体系与 `src/ui/` 基础组件，不另起炉灶。
3. 涉及 Agent 能力变更时，先确认 `agentversion/version.go` 当前版本号并按小版本递增（例如 `1.19 → 1.20`）。

### 阶段二：前端设计与实现规范
- **审美原则**：克制、精准、信息层级清晰；拒绝廉价 AI 模板感、拒绝嵌套卡片、拒绝无意义渐变与装饰性动效。
- **布局与视口约束**：
  - 弹窗/抽屉/终端等容器必须显式锁定高度与内部滚动区域（`grid-template-rows: minmax(0, 1fr)` + `min-height: 0`），绝不允许列表内容将弹窗或页面无限拉长。
  - 同时兼顾桌面（`1440px`）与移动端（`390px`）无横向溢出、无文字重叠。
- **交互完整态**：补齐 `hover`、`focus-visible`、`disabled`、`loading`、`empty`、`error` 状态，并遵守 `prefers-reduced-motion`。
- **视觉验证**：重大 UI 调整须通过真实浏览器截图检查桌面端与移动端渲染效果，不凭纯代码推断“看起来没问题”。

### 阶段三：领域与安全红线（不可回退）
1. **编译标签**：面板 Go 构建必须启用 `-tags "with_utls with_quic"`，否则完整协议探测能力缺失。
2. **Reality 伪装校验**：自动拦截 Cloudflare CDN 的 IPv4/IPv6 与证书目标，默认伪装域名使用 `www.amd.com`。
3. **3x-ui 全量入站协议**：保持 10 种协议模板（VLESS Reality Vision/XHTTP/gRPC/WS+TLS、VMess WS+TLS/TCP、Trojan Reality/TLS、Shadowsocks 2022、Hysteria2）前后端一致。
4. **节点可达性与国家归类**：
   - 节点可达性判定默认策略为 `any`（任意探针视角可达即有效，兼容国内专线入口），不展示混乱的 `1/5` 比例。
   - 已移除手动分组，统一按国家/地区自动归类与筛选。
5. **WebShell 安全与体验**：
   - 快捷指令与 NodeScripts 脚本库点选一律先填入命令草稿箱，高风险脚本（内核、防火墙、面板、系统重装）标明风险并二次确认后才发送到终端。
   - Agent 端 `tmux` 保持 `mouse on`、`set-clipboard on`、`history-limit 5000`；前端 `xterm.js` 保持 `scrollback: 5000` 并拦截未开启鼠标追踪时的备用屏滚轮转方向键行为。
6. **后端安全加固**：保留 JWT 登出即刻吊销、单 IP 登录防爆破、公开订阅限流、`index.html` 禁缓存、SQLite WAL 调优及 `SIGINT/SIGTERM` 优雅停机。

### 阶段四：测试与发布门禁（每次提交必过）

#### 前端验证（`webs/` 目录）
```bash
./node_modules/.bin/vue-tsc --noEmit
git ls-files -z '*test.mjs' | xargs -0 node --test
./node_modules/.bin/vite build --mode production
```
- 要求 TypeScript 0 错误、所有前端单测全绿、生产构建成功。

#### 面板后端验证与构建（根目录）
```bash
go test ./rulecenter/... ./middlewares/... ./node/... ./api/...
# 注意：go test ./api 可能生成临时目录 api/template，提交前务必清理
rm -rf api/template

GOOS=linux GOARCH=amd64 go build \
  -tags "with_utls with_quic" \
  -ldflags="-w -s -X main.version=redesign-$(git rev-parse --short HEAD)" \
  -o ppeelink .
```

#### Agent 构建与发布（`agent` 分支）
- 修改后递增 `agentversion/version.go` 中的 `Current`。
- 运行相关包测试（如 `go test ./terminalcore/...`），**切勿让历史夹具用例失败静默跳过编译导致上传旧产物**。
- 交叉编译 `linux/amd64`、`linux/arm64`、`linux/armv7` 并重新生成 `dist/release.json`（核对 `version` 与三个文件的 `sha256` 一致）后，再发布到面板的 `downloads/agent/` 目录。

---

## 后续 Agent 续写日志（Append-Only）

> 请严格按以下格式在文末追加，勿修改上方任何条目：
> ```markdown
> ### YYYY-MM-DD · <简短主题>
> - **变更摘要**：...
> - **新增约定 / 注意事项**：...
> ```

### 2026-11-23 · 基线工作流确立
- **变更摘要**：完成前后端全面重构与安全加固、3x-ui 10 协议全量支持、节点直测国内 TCP Ping、国家/地区自动归类、写实地形地图与跳点动画、WebShell 整合 NodeScripts 分类脚本库与定高布局、Agent `v1.19` 开启 `tmux` 滚轮历史回看，并清理历史冗余分支与提交署名。
- **新增约定 / 注意事项**：
  - 面板单二进制内嵌 `webs/dist/`，每次改动前端后必须重新 `vite build --mode production` 再编译 Go 二进制。
  - `go test ./api` 会在本地产生未追踪的 `api/template`，提交前记得删除。
  - 发布新版本 Agent 后，务必核验目标服务器 `downloads/agent/release.json` 中的 `version` 与二进制 `sha256` 已更新。
