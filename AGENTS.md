# AGENTS.md — PPEELink Agent 开发与发布规范

> **【强制规则 · 只允许续写，严禁覆盖】**
>
> 1. 完整工程协作工作流与面板规范见 `main` 分支根目录的 `AGENTS.md`。
> 2. **任何后续接手的 AI Agent 仅允许在文末 `## 后续 Agent 续写日志（Append-Only）` 区域追加内容，严禁覆盖或删改前文。**
> 3. **严禁在 `git commit` 中添加 `Co-Authored-By:` 署名**，避免污染仓库 Contributors 列表。

---

## Agent 开发与发布要点（`agent` 分支）

- **版本号管理**：位于 `agentversion/version.go`（`const Current`），每次功能或修复发布均需递增小版本号（如 `1.19 → 1.20`）。
- **核心能力红线**：
  - Xray-core 3x-ui 10 种入站协议模板（`xraycore/`）与面板保持同步；Reality 校验拦截 Cloudflare CDN 目标。
  - 节点直测国内 TCP Ping（`china_ping` WebSocket RPC）不得破坏既有消息协议兼容。
  - 持久化终端（`terminalcore/sessions.go`）必须保持独立 systemd unit、全局 `history-limit 5000`、`prefix None`、`mouse on`、`set-clipboard on`（并在 `Attach` 时兜底生效）。
- **测试与构建门禁**：
  1. 先跑目标包测试（例如 `go test ./terminalcore/... ./relaycore/... ./tracecore/... ./throughputcore/...`），**切勿因为无关历史夹具用例失败而跳过交叉编译导致上传旧产物**。
  2. 交叉编译三架构并重新生成 `dist/release.json`：
     - `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o dist/agent-linux-amd64 .`
     - `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-w -s" -o dist/agent-linux-arm64 .`
     - `CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags="-w -s" -o dist/agent-linux-armv7 .`
  3. 上传至面板 `downloads/agent/` 后，必须核验 `release.json` 的 `version` 与三架构 `sha256` 均为本次新产物。

---

## 后续 Agent 续写日志（Append-Only）

### 2026-11-23 · v1.19 基线确立
- **变更摘要**：完成 3x-ui 全量 10 协议、节点直测 `china_ping` RPC，以及 `v1.19` `tmux` 鼠标滚轮历史回看与剪贴板支持。
- **新增约定 / 注意事项**：发布后务必在目标面板验证 `downloads/agent/release.json` 版本号已切换。
