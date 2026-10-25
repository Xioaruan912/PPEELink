<div align="center">
  <img src="webs/src/assets/logo.svg" width="150" height="150" alt="PPEELink Logo" />
  <h1>PPEELink</h1>
  <p><strong>代理订阅管理、规则调试、节点质量分析与安全发布平台</strong></p>
  <p>不只是生成订阅，而是在发布前告诉你：配置会怎么走、节点能不能用、目标客户端是否兼容，以及这次修改是否值得上线。</p>
</div>

<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8.svg" alt="Go 1.25" />
  <img src="https://img.shields.io/badge/Gin-1.10-008ECF.svg" alt="Gin 1.10" />
  <img src="https://img.shields.io/badge/Vue-3.4-42B883.svg" alt="Vue 3.4" />
  <img src="https://img.shields.io/badge/Element%20Plus-2.6-409EFF.svg" alt="Element Plus 2.6" />
  <img src="https://img.shields.io/badge/SQLite-GORM-003B57.svg" alt="SQLite + GORM" />
  <img src="https://img.shields.io/badge/License-MIT-green.svg" alt="MIT License" />
  <br />
  中文 · <a href="README.en-US.md">English</a>
</div>

---

> **实现状态（2026-10-23）**：本文部分安全发布、不可变版本及任务中心描述属于尚未闭环的目标，不能视为已交付承诺。源码核查、未完成项与部署验证边界见[实现审计](docs/implementation-audit.md)，本次改动见[未发布更新记录](webs/CHANGELOG.md)。

## 项目定位

PPEELink 面向自建代理节点与复杂的 Clash/Mihomo 分流模板，目标是把传统的“订阅转换面板”升级成一套可验证、可解释、可回滚的代理配置发布系统。

典型工作流：

```text
订阅 / 节点
    ↓
模板与规则
    ↓
语法、引用、循环、协议检查
    ↓
关键网站规则命中模拟
    ↓
按目标场景选择节点
    ↓
真实出口验证
    ↓
与 Last Known Good 对比
    ↓
安全发布 / 失败保持旧版本
```

后端使用 **Go + Gin + GORM + SQLite**，节点真实检测基于 **sing-box**；前端使用 **Vue 3 + Element Plus + ECharts + Monaco Editor**。

## 核心能力

### 1. 一键安全发布

选择“订阅 + 模板 + 目标客户端”后自动执行发布前验证：

- 生成候选配置，不先修改线上绑定
- 校验 YAML / 配置段落、策略组、节点与规则引用
- 检测策略组循环引用
- 校验所有实际引用的 Rule Provider 是否可下载、解析或使用安全缓存
- 执行节点协议兼容性检查
- 运行已保存的分流回归用例
- 检查节点可用性并进行真实出口验证
- 与当前 Last Known Good 版本比较
- 全部通过后才事务发布；失败时保留原模板绑定和原 LKG

### 2. 通用规则解释器

输入域名、IP、端口和 TCP/UDP 上下文，可以看到完整的首条命中路径：

```text
gemini.google.com
  → RULE-SET Google
  → Gemini 策略组
  → JP 自动选择组
  → JP-02
  → 实际出口日本
```

同时展示：

- 命中前有哪些规则未命中，以及原因
- 规则来自本地规则还是远程 Rule Provider
- 策略组如何继续引用下一层策略组
- 哪些节点是候选、为什么被排除
- 最终选择哪个节点及其质量依据
- 模板编辑前 / 编辑后的命中 Diff

支持保存 Regression Case（分流回归用例），可定义预期策略、禁止策略、预期地区、端口和协议。

### 3. 节点 × 测试目标质量矩阵

节点不再只有一个笼统的总分。PPEELink 会记录真实的：

```text
节点 × 目标网站 × 场景 × 成功/失败 × RTT
```

场景包括网络、AI、社交、内容、金融、工具、开发、媒体等。

模板分流和规则解释器选择节点时按以下顺序使用历史质量：

```text
具体目标历史
  → 同场景历史
  → TCP 总质量兜底
```

系统每 6 小时对当前在线节点进行低负载场景采样，也可在任务中心手动执行完整目标采样。

### 4. 节点协议兼容性矩阵

发布前会针对以下客户端进行能力诊断：

- Clash / Mihomo
- sing-box
- Surge
- Loon
- Quantumult X

诊断会区分：

- 原生支持
- 需要转换
- 完全不支持
- 转换后可能丢失字段

覆盖 `ws`、`grpc`、`httpupgrade`、`reality`、UDP、TFO、MPTCP 等能力，尽量把运行时错误提前变成生成阶段的明确提示。

### 5. 规则中心与更新影响预览

- 支持本地和远程 Clash Rule Provider
- 本地 provider 限制在 `template/` 安全路径内，拒绝目录越权
- 远程 provider 支持缓存回退和请求超时
- 区分 `domain`、`ipcidr`、`classical` 等 behavior
- 更新前显示新增、删除、修改、重复和前序覆盖
- 扫描哪些模板正在引用该规则集
- 对已配置检测目标比较更新前后的分流结果
- 更新前自动保留规则缓存快照，可一键回滚

### 6. 统一后台任务中心

以下长任务统一进入持久化任务中心：

- 节点 / 出口检测
- 质量矩阵采样
- 模板分流验证
- 规则集同步
- 订阅构建
- 模板验证
- 一键安全发布
- 系统部署任务

支持进度、耗时、取消、重试、错误详情和历史记录。服务重启后未完成任务会被标记为中断，而不是在页面刷新后“消失”。

### 7. 订阅版本与 Last Known Good

订阅后台构建会保存不可变产物快照，包括：

- 输入摘要
- 模板校验值
- 规则校验值
- 产物 SHA
- 语法验证结果
- 分流 / 出口测试报告
- 生成与发布时间

验证成功的版本可以成为 **Last Known Good（最后已知可用版本）**。当实时生成或远程规则临时故障时，可回退到已经验证过的 LKG。

### 8. 可配置分流检测目标

管理员可直接配置测试目标，无需修改代码：

- 名称、Key、域名、分类和图标
- GET / HEAD 与请求路径
- 期望 HTTP 状态码
- 响应内容特征
- 是否要求出口 IP
- 超时与重试
- 启停和排序

检测目标本身不硬编码“应该走哪个国家”；期望地区只从当前模板的真实规则和策略组约束中推导。

### 9. 节点与分组管理

- 所有节点强制至少属于一个分组，历史无分组节点自动进入“默认”组
- 自建节点统一进入单一「自建」分组，不再按服务器生成多个分组；升级时会合并旧分组并保留订阅引用
- 自建节点默认使用“任一视角通过”检测策略：面板不可达但任一自建服务器 agent 检测通过时仍视为有效
- 支持一个节点属于多个分组
- 支持单节点和整组**全局隐藏**，隐藏不会删除数据
- 被隐藏节点不会进入正常列表、订阅下发、质量采样、推荐、分流选节点和安全发布
- 隐藏状态可在分组管理中恢复

### 10. 安全、审计与状态页

- 用户密码使用 bcrypt；历史明文密码在成功登录后自动迁移
- 登录失败限流
- API Token 仅保存 SHA-256 摘要，支持 `read` / `write` / `admin` 权限和有效期
- 安全发布、系统部署、备份恢复、审计等高风险操作要求 admin scope
- 节点链接、订阅源、密码、Token、Webhook 等敏感日志脱敏
- 默认不信任任意反向代理；只有管理员显式配置可信代理 IP / CIDR 后才接受代理转发的客户端地址
- 管理操作审计记录操作者、认证类型、IP、方法、路径、状态和时间，不保存请求体或敏感凭据
- 安全配置导出默认排除密码、JWT Secret、API Token、订阅 Token、节点链接和 Webhook
- `/status` 提供不泄露节点地址的公开只读状态页和事故时间线

### 11. 独立服务器监控

- 自建节点复用同一个 agent：在服务器编辑里打开「服务器监控」，不需要再装第二套服务
- 同一台机器已有独立监控时，agent 会复用该监控记录和历史，不再生成重复卡片
- 每台自建服务器 agent 同时作为节点检测视角，每 5 分钟检测所有节点并回报 TCP 结果，监控卡片直接显示成功数/总数
- 独立服务器也可以用一行安装命令接入；监控与代理安装共用模板和同一个可执行程序，旧 `monitor` 命令兼容运行统一 Agent 循环
- 新版本监控 Agent 首次连接后会关联一台默认关闭代理的自建服务器；可在 Web 填写公网地址、配置入站并开启代理，无需重新安装。关闭代理或指标采集不影响管理心跳、配置接收、终端和升级
- 监控页和自建服务器页均可请求升级；仅 Agent 回报实际二进制摘要匹配后才确认成功。此前不支持升级指令的旧独立监控需先使用更新后的安装命令升级一次
- CPU、内存、Swap、磁盘、负载、uptime、网络速度、累计流量、TCP/UDP 连接数、进程数
- 服务器列表与详情历史曲线（1 小时 / 6 小时 / 24 小时 / 7 天）
- 原始指标保留 7 天，按时间范围自动降采样
- 当前版本只做监控展示，不包含通知、DDNS 和计划任务

### 12. Agent 终端与快速指令

- 终端不再保存或使用 SSH 地址、用户名、密码；agent 主动连接面板并建立反向 WebSocket
- 浏览器只连接面板一次性票据；面板把终端字节转发给 agent，agent 在本机 PTY 中执行 shell
- 快速指令来自 [Xioaruan912/Xcript](https://github.com/Xioaruan912/Xcript)，执行前会二次确认
- 终端可以执行任意命令，仅限管理员使用；agent 断开时终端自动结束
- 面板只保存 agent 令牌，不保存任何子服务器登录凭据

### 13. Agent 文件与 Docker 工作台

在「监控 → 服务器详情」打开文件管理或 Docker 管理，全部通过该服务器的 agent 本地执行，不额外安装 SSH/SFTP 服务。

- 文件目录浏览、面包屑、上传、下载、UTF-8 文本编辑、新建目录、重命名和删除；上传/下载单文件最多 64 MiB，编辑最多 1 MiB
- 删除必须确认，仅删除文件、符号链接或空目录，不递归删除；根目录禁止修改，符号链接不用于编辑或下载
- 保存使用同目录临时文件和原子替换，保留原文件 POSIX 权限位；不保留所有者、ACL、扩展属性
- Docker 容器列表、按需资源快照、最近 500 行日志、启动/停止/重启；改变状态前需确认，不提供任意命令接口，不自动 sudo
- 所有文件、Docker、终端及网络工具接口要求管理员账户或 admin API Token；agent 操作有 45 秒级超时和输出上限

### 14. 多地路由追踪

配置关系网络拓扑页面及其专属快照接口已移除；节点状态、服务器监控、Agent 连接和 xray 通用连接/计数能力保留。

「网络工具 → 多地路由追踪」从面板及所选自建服务器 agent 追踪目标域名或 IPv4/IPv6；这是来源到目标的直接网络路径，不经过代理节点，不是 MTR 丢包统计。

- 来源机器需要安装 `traceroute` 或 `tracepath`（Debian/Ubuntu 可安装 `traceroute` 软件包）
- 自建服务器 agent 作为来源：在「自建节点」卡片上点击「更新 Agent」，agent 校验面板二进制哈希后覆盖自身并重启，随后自动上报追踪能力
- agent 独立轮询追踪任务，不等待常规 TCP 扫描，页面会显示来源是否就绪
- 最多 24 跳、单个命令 75 秒、一次最多 8 个远程来源；结果包含逐跳地址/耗时、状态、原始输出以及 7 天历史
- 无应答不等同业务丢包，追踪结束不代表一定到达目标；请结合原始输出判断。取消后结果停止写入，远程命令最多等待超时后退出

### 15. 节点网站流量（仅代理转发）

在「自建节点」服务器设置开启「网站流量统计」（默认关闭），升级 Agent 后开始计量；下方统计面板按服务器节点、域名和 UTC 小时范围查询上传、下载、总流量及排名。

- 上传＝客户端至目标网站，下载方向相反；只计算成功转发的 TCP/UDP 载荷，不计算协议头、Agent 心跳或整机其他网络流量，不回填历史。
- 仅使用代理请求明确提供的域名，不解密 HTTPS、不获取完整 URL、不按 IP 反查或猜测网站。IP 目标、无法识别及域名基数超限的流量归为「未知」；即使 IP 目标有 TLS 嗅探结果也不改变原目标或伪造归属。
- 时间范围按 UTC 小时桶 `[开始,结束)` 筛选，最长 366 天；浏览器按本地时区显示。汇总涵盖全部筛选结果，排名默认前 50 条、最多 100 条。
- 计量器只监听回环、使用随机认证，具备私网目标保护及会话、UDP socket、域名和队列上限。采集开启时 Agent 管理本地转发器，重启/升级会重建转发器及 Xray 配置，既有连接可能中断；请在维护窗口启用。
- 待确认批次先持久化，网络失败按同批次重试，面板事务去重。尚未排入持久化批次的实时计数在进程崩溃时可能丢失，统计不作为精确计费依据；面板长时间不可达导致采集队列满时会拒绝新的转发，恢复回报或关闭采集后可恢复。
- 数据库自动迁移新增开关、小时统计及批次回执表；默认不清理历史统计/回执，部署方需规划存储。升级前备份数据库。管理员权限才能查询网站统计。

## 订阅输出与客户端

当前订阅产物支持：

- Clash / Mihomo
- Surge
- Loon
- V2Ray / Base64 通用节点订阅

协议兼容检查的覆盖范围比订阅输出范围更广，用于提前判断同一批节点在 sing-box、Quantumult X 等目标客户端中的可表达程度。

## 节点检测

TCP 检测由面板及在线 Agent 多视角直接连接节点端口，不等同于经代理出口的业务检测；解锁和国内可达性由面板发起并经节点出口访问目标。结果不代表用户所在地的延迟体验：

- **有效 / 无效**：面板及 Agent 的 TCP 可达性，按节点配置的检测策略（例如“任一视角通过”）判定，不仅看面板单一视角。
- **解锁检测**：AI、影视、论坛服务是否可用。服务只看节点的出口 IP，所以结果和从哪台电脑使用一致。
- **国内可达性**：经节点连接国内各省三大运营商的服务器，连不上通常说明节点 IP 被墙或线路异常；只检测 IP 是否可达，不代表某个端口或协议没被干扰。

从面板服务器测出的延迟和速度不能代表用户自己电脑上的体验，所以不再提供测速。

### 暂不下发

以下节点不会出现在订阅里，条件解除后自动恢复：手动暂停到某个日期（例如节点本月流量用完）、自建服务器超出月流量上限、连续多次检测无效。

## 项目预览

仓库原有的 `webs/src/assets/1.png`、`2.png` 已经是旧版界面，README 不再把它们作为当前项目截图展示。

当前 Web 控制台主要包括：

- 首页节点地图、健康状态与场景推荐
- 独立服务器监控：CPU、内存、磁盘、网络与历史曲线
- WebSSH 终端与 Xcript 快速指令
- 节点分组、质量详情、TCP / 解锁检测与全局隐藏
- 分流检测、规则解释器与节点 × 场景质量矩阵
- Clash / Surge / Loon 模板编辑、发布前预检与分流回归
- 规则中心、Rule Provider 更新影响预览与回滚
- 订阅产物版本、Last Known Good 与一键安全发布
- 后台任务中心、运维中心、审计日志与公开状态页

> 后续只加入与当前版本一致的新截图，避免继续展示已经失效的旧 UI。

## Linux 安装

### 一键安装（源码构建）

PPEELink 使用 `with_utls` 与 `with_quic` 编译标签，以支持 Reality、Hysteria2、TUIC 等能力。

```bash
curl -s -H "Cache-Control: no-cache" -H "Pragma: no-cache" \
  https://raw.githubusercontent.com/Xioaruan912/PPEELink/main/install.sh | sudo bash
```

需要：

- `git`
- Go 1.25+
- Node.js 20+
- corepack / pnpm
- `curl`

安装脚本会完成前端构建、Go 编译、systemd 注册和管理命令安装。当前兼容管理命令 / 服务二进制名称仍为：

```bash
ppeelink
```

默认 Web 端口为 `8000`。

首次安装会创建管理员账号：

```text
用户名：admin
密码：123456
```

**首次登录后请立即修改默认密码。**

### Docker

仓库包含多阶段 `Dockerfile`。构建阶段会编译 Vue 前端与 Go 后端；运行阶段工作目录为 `/app`，程序默认监听 `8000`，运行数据位于 `/app/db`、`/app/template`、`/app/logs`。

推荐使用独立的数据目录运行：

```bash
docker build -t ppeelink:latest .

mkdir -p ./ppeelink-data/{db,template,logs}

docker run -d \
  --name ppeelink \
  --restart unless-stopped \
  -p 8000:8000 \
  -v "$(pwd)/ppeelink-data/db:/app/db" \
  -v "$(pwd)/ppeelink-data/template:/app/template" \
  -v "$(pwd)/ppeelink-data/logs:/app/logs" \
  ppeelink:latest
```

启动后访问：

```text
http://服务器IP:8000
```

查看运行状态：

```bash
docker ps --filter name=ppeelink
docker logs -f ppeelink
```

升级前至少备份 `ppeelink-data/db/` 与 `ppeelink-data/template/`。如果之后修改了程序内部监听端口，Docker 的 `-p` 映射也需要同步修改。

## 开发与构建

```bash
# 前端
cd webs
corepack enable
pnpm install --frozen-lockfile
pnpm exec vue-tsc --noEmit
pnpm build
cd ..

# Go 测试
go test ./...

# Linux 生产构建
GOOS=linux GOARCH=amd64 \
  go build -tags "with_utls with_quic" -ldflags="-w -s" -o ppeelink .
```

也可以使用仓库中的 `build.sh` 生成 Linux amd64 / arm64 产物。

## 目录结构

```text
api/          HTTP API、规则解释、安全发布、任务中心等业务逻辑
models/       SQLite / GORM 数据模型与质量、任务、版本数据
node/         节点解析、sing-box 检测、出口与协议能力
routers/      Gin 路由注册
rulecenter/   规则集解析、缓存与规则中心
template/     Clash / Surge / Loon 模板与本地规则
webs/         Vue 3 管理前端
db/           SQLite、运行配置和规则缓存（运行时数据）
```

## 数据与升级建议

- 不要把 `db/` 当作可随意删除的缓存目录，其中包含主要业务数据。
- 升级前备份 `db/`、`template/` 和当前运行二进制。
- 生产替换建议使用“备份 → 上传临时文件 → SHA256 校验 → 原子替换 → 重启 → HTTP / 日志验证”的流程。
- 不要把密码、API Token、订阅地址或节点链接写入公开日志和 Git 仓库。

## License

[MIT](LICENSE)

## 致谢

感谢 [gooaclok819/sublinkX](https://github.com/gooaclok819/sublinkX) 早期项目提供的基础思路与代码参考。PPEELink 在后续开发中围绕节点质量、规则解释、Rule Provider、任务中心、版本回滚、安全发布、安全审计与运维体系进行了持续扩展与重构。
