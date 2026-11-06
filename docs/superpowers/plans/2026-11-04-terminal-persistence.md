# PPEELink 终端持久会话与快速指令优化 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**状态：已完成并部署 Agent 1.16；当前 6 台服务器的持久终端全部就绪。**

**确认日期：** 2026-11-04，Asia/Shanghai。

**Goal:** 终端断线、刷新或关闭弹窗后长任务继续运行，重新打开能接回原会话；快速指令以会话、日志和运维诊断为主。

**Architecture:** 保留 Vue + xterm.js + Go Agent 反向 WebSocket；在服务器端引入 tmux 持久会话层。浏览器和 Agent 通道只负责附着，运行会话具有独立生命周期。为覆盖 Agent 服务重启，tmux 必须独立于 Agent 服务的清理范围。

**Tech Stack:** Vue 3、TypeScript、xterm.js、Go、gorilla/websocket、creack/pty、tmux、systemd。

**Spec:** 本文件“已确认范围”及“快速指令调整”是用户已同意的方案；接口细节属于实施建议，开始实施前结合最新代码核对。

## 恢复工作须知

- 正确源码工作树：`/root/Documents/Codex/2026-11-03/ppeelink-bug/deploy-source`。
- 起草时 HEAD 为 `0d737a4`，分支 `deploy/mierus-recognition`。工作树包含前面已部署的修复，不能重置或覆盖。
- 线上面板版本：`2026.11.04-subscription-home.0d737a4`；Agent 显示版本 `1.14`。恢复时重新核对，不能把这里的记录当成实时状态。
- 最近备份：`/root/ppeelink-backups/20261104T095704Z-subscription-home`。正式部署前仍须制作新的备份。
- Mieru 的 Agent 协议适配已取消，不纳入本规划。
- 不在规划阶段安装依赖、改变服务、运行快捷脚本或部署。没有设置自动检测额度、定时执行或后台开发任务。

## 已确认范围

1. 默认附着上一次会话；原会话还在时不重新创建 shell。
2. 浏览器断网、刷新、关闭弹窗或页面卸载，均只离开会话，不结束运行任务。
3. Agent 与面板的连接重建后，允许恢复同一会话；面板重启不结束远端任务。
4. Agent 服务重启后仍保留 tmux 服务和其中运行的任务。
5. 明确区分“离开并保留”和“结束会话”；后者才显式终止任务。
6. 提供会话列表、新建、恢复、运行状态和结束指定会话的入口。
7. 自动重连重新获取授权，只恢复连接，不自动重发输入或重新执行此前命令。
8. 缺少 tmux、会话已结束、服务器重启等情况给出真实状态，不假装恢复成功。
9. 服务器重启、断电后不能恢复原进程执行现场。本期不实现任务检查点或自动重跑。

## 现有证据与定位

| 文件 | 当前行为与后续职责 |
| --- | --- |
| `agent_terminal.go` | `serveAgentTerminal` 的会话表属于单条连接；断线、输出写失败或收到 `term.close` 会 Kill shell、关闭 PTY。`startAgentPTY` 直接启动 bash/sh。改为附着持久会话，区分离开与终止。 |
| `api/agent_terminal.go` | 每次使用随机新 ID；浏览器退出时 `defer ac.closeTerminal`。增加可恢复会话的授权、连接与生命周期语义。 |
| `api/agent_hub.go` | 会话映射附着在 Agent 连接上；发送队列满会关闭整个连接。保留通道清理，但不能据此结束持久任务。 |
| `api/monitor_ssh.go` | 当前入口使用一次性票据；保留该授权机制。旧 SSH 实现不应误当作当前实际终端路径。 |
| `routers/monitor.go` | `/api/v1/monitors/ssh/ws` 实际映射 `MonitorAgentTerminal`。增加会话管理接口时保留兼容路径。 |
| `api/agent_install.go`、`install.sh` | 核对安装与 systemd 生命周期，在这里接入依赖及独立会话服务所需的安装/升级行为。 |
| `webs/src/views/monitor/MonitorTerminal.vue` | 目前断线提示“重连将建立新会话”，关闭即销毁显示缓存。改为恢复状态、会话操作与重连。 |
| `webs/src/views/monitor/commands.ts` | 当前 14 个外部脚本入口；按下表调整。 |
| `webs/src/api/monitor/` | 复用现有 API 定义方式；实施时先定位入口文件，再扩展会话请求。 |
| `agentversion/version.go`、`docs/agent-versions.md` | Agent 运行时发布时按现有整数序列递增；当前 1.14，若中间没有其他发布则下一版为 1.15。 |

任务丢失机制已由代码确认；造成最初断连的网络、代理或拥堵原因尚未用事故时间日志确认，不能把某个推测写成已验证根因。

## Global Constraints

- 复用现有 xterm.js 和 Agent 通道，不替换为另一整套 Web SSH 服务，不为此增加 Node 后端。
- 运行任务不得依赖浏览器、面板连接或 Agent 服务的存活；不得仅删除 Kill 调用而留下无法读取输出的孤立 PTY。
- tmux 服务使用独立 systemd unit/cgroup；不通过关闭整个 Agent 的进程清理来绕过生命周期问题。
- 会话按服务器和有权管理该服务器的用户隔离；不可让所有用户共用一个固定的 root 会话。
- 每次附着、列举和终止均重新校验权限；恢复 ID 不代替身份认证，不泄露票据或终端输出。
- 缺少依赖时明确提示保护未启用，不静默降级为会丢任务的普通 shell。
- 新旧 Agent 的能力通过明确能力信息区分；旧 Agent 不显示“任务已保护”。
- 不自动重发输入、重跑脚本或复活已结束的任务。资源上限、输出缓存和日志保存有界；不自动清理正在运行的长任务。
- 所有外部脚本只在实施时审阅；下载失败禁止继续执行，固定经审阅版本并核对校验值。
- 保留目前订阅首页、节点适配、流量采集和 Agent 版本显示修复。

## 快速指令调整

### 从常用区收起或合并

| 当前入口 | 已确认处理 |
| --- | --- |
| 安装 Miniconda、Vaultwarden、Jellyfin、rclone | 移到“扩展工具”，不占用常用区。 |
| 两个 Docker 安装入口 | 合并为一个入口，在其中选择安装来源。 |
| 两个证书安装入口 | 合并，移到偶尔使用的配置操作；与只读证书检查区分。 |
| 时区设为上海 | 移到系统配置，不放常用区。 |
| Realm 端口转发、安装 mihomo | 移到“网络工具安装”，与面板管理的代理服务区分。 |
| 重建当前 Compose | 删除现有快捷入口。已查脚本包含 `docker compose down --volumes`，不能继续当作普通重建操作。 |
| 清理缓存与日志 | 拆开；先只读查看占用，再选择清理对象；不可默认同时清理日志、Docker 资源和旧内核。 |
| 通用自动备份 | 改为高级工具，执行前明确备份对象、目的地和凭据来源，不直接作为通用常用按钮。 |

### 常用区增加/优先展示

| 分组 | 入口及边界 |
| --- | --- |
| 会话 | 恢复会话、会话列表、新建会话、离开并保留；“结束会话”单独确认。会话操作调用管理接口，不向未知前台程序盲发 shell 命令。 |
| 服务 | Agent 状态与日志、Xray 状态与日志、失败服务列表；动态识别实际 unit 名称。 |
| 系统 | CPU/内存概况、磁盘空间与大目录；默认只读，目录扫描范围有界。 |
| 网络 | TCP/UDP 监听端口、IPv4/IPv6 连通性、DNS 查询；缺少工具时提示，不自动安装。 |
| Docker | 容器状态、最近日志、磁盘占用；不默认执行 prune、删卷、重建或重启。 |

普通诊断指令默认先填入输入框，用户回车执行；不向正在运行的安装器、编辑器或交互程序自动追加命令。修改系统的操作继续明确展示影响并确认。

## Review Focus

1. Agent 服务重启时，tmux server 是否仍在 Agent cgroup 内而一起被杀；任务 1 用真实 systemd 环境验证。
2. 重连风暴、重复点击或两个浏览器同时进入，是否产生重复会话或重放输入；任务 2、3 覆盖。
3. 高输出任务与慢浏览器是否拖垮整个 Agent 通道，断线期间输出是否阻塞任务；任务 2 覆盖。
4. 用户换服务器、失去权限或旧票据被复用，是否附着到不属于自己的会话；任务 2 覆盖。
5. 会话实际结束、机器重启、tmux 缺失或安装失败时，是否错误宣称已恢复；任务 1、3 覆盖。

## 实施任务

### 任务 1：独立持久会话与安装生命周期

**文件：** 修改 `agent_terminal.go`、`api/agent_install.go`、`install.sh`；扩展 `agent_terminal_test.go`、`api/agent_install_test.go`；新增 `agent_terminal_sessions.go` 和对应测试承载会话管理，避免继续扩大现有终端文件。

**接口：** 建议定义 `terminalSession` 元数据（ID、名称、状态、创建时间）；会话管理器提供 List/Create/Attach/Terminate。Attach 返回临时 PTY 客户端；关闭该客户端不得等价于 Terminate。具体 Go 类型签名以现有 Agent 运维接口为基础落定。

- [x] 先写失败回归：断开附着客户端后计数任务继续递增，重接可看到同一进程；只有显式 Terminate 才结束。
- [x] 运行新用例，确认旧实现因会话被终止而失败。
- [x] 实现独立 tmux 会话服务及依赖检测；安装失败返回真实状态，既有 Agent 升级有清晰迁移步骤。
- [x] 覆盖 shell 正常退出、依赖缺失、会话不存在、并发创建与重复结束，不复用已失效的身份。
- [x] 在隔离 systemd 环境测试 Agent service restart，确认任务 PID 和执行进度保持，独立 unit 仍存活。
- [x] 运行 `PATH=/usr/local/go/bin:$PATH go test . -run 'TestAgentTerminal' -count=1` 及安装脚本相关测试；新用例全部通过后保存本任务变更。

### 任务 2：可恢复会话协议、权限与通道可靠性

**文件：** `api/agent_terminal.go`、`api/agent_hub.go`、`api/monitor_ssh.go`、`routers/monitor.go`、`agent_terminal.go`；按当前 API 组织方式新增会话管理文件与测试。

**接口：** 在现有一次性票据机制上扩展列举/新建/附着/结束操作；`term.close` 表示离开，显式终止使用独立操作。持久 ID 由服务端生成并绑定所属服务器/用户；连接随机 ID 只标识一次附着。接口能力需兼容旧 Agent。

- [x] 先写失败测试：浏览器断开、面板重启、Agent 通道重建后，附着同一个持久 ID，不发送终止操作或历史输入。
- [x] 增加越权 ID、跨服务器 ID、过期/重复票据、权限撤销、并发附着的拒绝或一致性断言。
- [x] 实现会话操作和授权绑定；断线清理仅清理附着关系。
- [x] 增加输出突发、慢读、队列满、网络半开测试；输出传输使用有界缓存/背压，浏览器不可阻塞整个 Agent 通道或终止远端任务。
- [x] 为断连记录阶段、关闭码、原因和会话 ID；日志不写令牌、终端正文或密码。
- [x] 运行相关 API/Agent 测试，确认旧能力有明确提示、新能力可恢复会话，再保存本任务变更。

### 任务 3：终端恢复体验和会话操作

**文件：** `webs/src/views/monitor/MonitorTerminal.vue`、`MonitorTerminal.test.mjs`、现有 monitor API 定义；按需要新增相邻的会话请求/重连模块及测试，不重做终端外观。

**接口：** 消费任务 2 的会话列表、创建、附着和结束接口；保存按服务器区分的最近会话 ID。自动重连每次换新票据，并受组件生命周期和当前服务器选择约束。

- [x] 先写失败测试：网络断开后恢复同一 ID；重复点击只建立一个连接；关闭组件或换服务器后停止旧连接重试。
- [x] 实现会话列表、新建、恢复、“离开并保留”及单独确认的“结束会话”。
- [x] 实现有上限的退避重连及真实状态：“连接恢复中 / 会话仍运行 / 会话已结束 / 未启用任务保护”；鉴权失败停止重试，不重放输入。
- [x] 覆盖服务器重启、tmux 缺失、会话退出、历史显示与实际进程状态不一致；区分运行继续和输出历史保存。
- [x] 浏览器检查 320、768、1024、1440px；验证手机键盘、复制/粘贴及会话操作。
- [x] 运行 `node --test src/views/monitor/MonitorTerminal.test.mjs` 和 `pnpm exec vue-tsc --noEmit`，通过后保存本任务变更。

### 任务 4：快速指令精简与诊断入口

**文件：** `webs/src/views/monitor/commands.ts`、`MonitorTerminal.vue`、相关测试；复用现有服务、网络和 Docker 运维能力。

**接口：** 快捷项区分会话操作、只读诊断、修改系统操作、外部安装脚本；保留名称、说明及执行内容，补充执行方式和依赖信息。

- [x] 按“快速指令调整”表写清单断言，禁止普通入口包含删卷命令或笼统清理多个对象。
- [x] 调整分组、合并重复项；增加会话/服务/系统/网络/Docker 常用操作。
- [x] 普通指令填入而不立即执行；会话管理走独立接口；脚本明确影响后确认，下载校验失败即停止。
- [x] 覆盖缺少 Docker、systemd、诊断工具，以及终端当前正在执行交互程序的情况。
- [x] 验证不会重复提供已有文件、容器管理页面的整套功能；通过相关测试后保存变更。

### 任务 5：发布验收与回滚

- [x] 在隔离环境启动带时间戳和进度的真实长任务，记录 PID；依次模拟断网、刷新、关闭弹窗、面板重启、Agent 通道重连、Agent service restart，每次核对 PID/进度未重置。
- [x] 显式结束会话，确认任务结束；模拟服务器重启，确认不自动重跑且页面不会显示虚假存活。
- [x] 用两个不同授权主体及服务器验证会话隔离；用高输出任务验证界面与 Agent 其他功能正常。
- [x] 运行前端全部现有测试、类型检查与生产构建；Go 测试按现有基线比较，不把已有失败说成通过，也不忽略新失败。
- [x] 按最新实际版本递增 Agent 发布号一次；当前基线无其他发布时为 `1.15`。保留编号展示及二进制校验。
- [x] 部署前新备份数据库、配置、模板、二进制和源码差异；确认有正在运行的旧终端时先提示其尚未受到新机制保护，禁止为验证而终止用户任务。
- [x] 先试点一台，再逐步更新其余 Agent；核对编号、会话存活、流量采集、订阅首页和既有节点行为。
- [x] 记录独立会话服务的升级/卸载与回滚策略；回滚不能顺手结束正在运行的持久会话。

## 调研来源与选择依据

- [tmux 官方使用说明](https://github.com/tmux/tmux/wiki/Getting-Started)：支持 detach/attach，用于保护远端程序免于连接中断。选为持久会话层。
- [xterm.js](https://github.com/xtermjs/xterm.js)：已有前端终端模拟器，继续复用；显示缓存不能代替后台进程存活。
- [ttyd 官方示例](https://github.com/tsl0922/ttyd/wiki/Example-Usage)、[WeTTY](https://github.com/butlerx/wetty)、[GoTTY](https://github.com/yudai/gotty#sharing-with-multiple-clients)：整体替换 Web 终端仍不能免除持久会话问题，暂不引入。
- [Zellij](https://github.com/zellij-org/zellij)、[会话复活文档](https://zellij.dev/documentation/session-resurrection.html)：完整工作台备选；重跑命令不等于恢复原任务，本期不采用。
- [systemd.kill](https://www.freedesktop.org/software/systemd/man/latest/systemd.kill.html)：默认按 control-group 清理，说明为何 tmux 不能只在 Agent 子进程里后台启动。
- [现有 Compose 脚本](https://raw.githubusercontent.com/Xioaruan912/Xcript/main/linux/docker/docker_rebuild.sh)：包含删除数据卷。
- [现有清理脚本](https://raw.githubusercontent.com/Xioaruan912/Xcript/main/linux/basic/clean.sh)：实际覆盖缓存、日志、Docker、旧内核等多个对象。

## 本次交付记录

- [x] 用户同意调研建议，要求建立规划文件，额度恢复后再实施。
- [x] 保存已确认范围、代码证据、实施顺序、风险边界和验收步骤。
- [x] 实现、故障验证、面板部署与当前全部 Agent 升级完成，最终版本为 1.16。
- [x] 线上试点已验证浏览器断线、Agent 服务重启、面板重启后 PID/进度连续。


## 完成记录（2026-11-04，Asia/Shanghai）

- 最终面板版本：`2026.11.04-terminal-1.16.0d737a4`；当前 6 台 Agent 均为 `1.16`，5 台启用代理正常运行，WEB 保持停用；更新请求已全部确认，无错误回报。
- 6 台服务器持久终端均已就绪。Ubuntu 20.04 / tmux 3.0a 的参数与首次输入兼容问题已在 1.16 修复；已有健康服务保持运行。
- WEB 与 AsiaCLoud_HK 线上试点分别验证浏览器断开、恢复原会话、Agent 服务重启、面板服务重启，任务 PID 不变且进度增长；测试会话、临时文件与临时验证凭据已清理。
- 44 项前端测试、类型检查、实际浏览器交互及 320/768/1024/1440px 检查通过；真实旧版 tmux、会话丢失不重跑、所属服务校验和 HTTP 权限测试通过。完整 Go 测试相对原基线无新增失败，原有 15 个失败项保留在部署记录；当前环境无 CGO/C 编译器，未运行竞态检测。
- 相对原计划的取舍：通过公共 terminalcore 与运行时准备统一新安装/已部署 Agent 的迁移；tmux 原生前缀键关闭，统一用页面选择会话；通用备份脚本因会删除远端备份而停用直接执行，只保留配置检查；扩展脚本入口固定并校验，其内部软件包仍由上游管理。
- 使用说明与回滚边界见 `docs/terminal-sessions.md`。服务器重启、断电或 tmux 自身退出不会恢复原进程，也不会自动重跑命令。
