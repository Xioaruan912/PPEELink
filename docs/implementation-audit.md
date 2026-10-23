# 实现核查与未完成项

本记录区分源码闭环、明确差距与尚未实测；README 的功能陈述不自动构成实现证据。以下记录本次整合及验证结果，行号为审阅时位置，后续编辑可能移动。

## 已闭环：统一 Agent 与升级

- `monitor` 入口兼容调用同一个 `runAgent`，共用期望状态拉取、心跳回报、终端、文件/Docker、检测和追踪；不再运行第二套只上传指标的客户端循环。
- [身份绑定](../api/agent_identity.go)按已有监控凭据事务关联默认关闭代理的远程服务器，重复拉取不重复创建；授权限定于该持久化绑定。可在 Web 填写公网地址、配置入站并启用代理，无需重新安装。
- [安装生成器](../api/agent_install.go)是两种入口唯一模板，同一个可执行程序按兼容服务名安装；令牌以 0600 环境文件保存，不放在命令参数中。
- 代理停用/监控指标关闭不撤销管理身份。服务器保存同步关联监控开关，未附带指标的回报仍更新管理心跳、实际版本和摘要；[回归](../api/website_traffic_test.go)覆盖关闭代理与指标后保持心跳。
- 监控页与自建节点均可发起升级、查看待执行/失败/实际摘要确认；不再用 `2.2.revision` 虚构实际 Agent 版本。下载按摘要校验，替换或 exec 失败回滚旧文件；凭据两种入口的轮换互相吊销旧身份，自动绑定节点删除走卸载回执闭环。
- 验证覆盖：[身份绑定及凭据轮换](../api/agent_identity_test.go)、[共用安装脚本](../api/agent_install_test.go)、[升级接口](../api/monitor_update_test.go)、[升级失败回滚](../xraycore/agent_update_test.go)。
- **部署边界**：旧独立监控二进制没有升级指令处理代码，服务端无法远程补上；需先重跑一次新版安装命令。未在生产 systemd 节点实测替换/重启/网络断开恢复，不将临时文件测试冒充生产验证。

## 已闭环：仅代理转发的网站流量

- [Web 面板](../webs/src/views/subcription/servers/WebsiteTraffic.vue)与默认关闭的服务器开关、[期望状态](../api/proxy_runtime.go)、[Xray 本地认证转发计量器](../xraycore/website_collector.go)、[批次回报](../xraycore/agent.go)、[事务入库与查询](../api/website_traffic.go)已经接通。
- 节点粒度为 `ProxyServer`，域名为请求明确提供的精确主机名，不是完整网址或 eTLD+1。TCP/UDP 仅计成功转发的载荷字节；不采整机流量，不解密 HTTPS，不做 IP 反查；无法归属或基数溢出均归 `unknown`。
- 按 UTC 小时桶 `[start,end)` 查询（最长366天），支持节点/精确域名/上传下载总量排序、最多100条排名；汇总不受 top-N 截断影响。查询受既有管理员或 admin API Token 权限保护。
- [自动迁移](../models/sqlite.go)新增 `ProxyServer.website_traffic_enabled`、[小时统计与批次回执](../models/website_traffic.go)，默认关闭；迁移回归覆盖已有数据保留、重复迁移、事务回滚。没有独立需要手工执行的 SQL 迁移文件。
- [待确认批次](../xraycore/website_spool.go)原子持久化、跨重启恢复；相同 ID 与完全相同负载重试，面板在同一事务保存回执和增量，避免 HTTP 回应丢失后重计。升级前补报、卸载前停转发并确认最后批次，之后才删除目录。
- **边界**：尚未 drain/save 的实时计数不能保证进程崩溃后恢复；本统计不作为精确计费依据。资源有界，队列满时拒绝新转发；开启采集后 Agent 重启/升级需重建计量器和 Xray，可能中断既有连接。历史统计/回执目前不自动清理，需部署方规划存储。
- [后端回归](../api/website_traffic_test.go)、[主循环重试/恢复](../xraycore/agent_website_test.go)、[落盘安全性](../xraycore/website_spool_test.go)、[计量器与真实 Xray 测试](../xraycore/website_collector_test.go)、[Web 状态测试](../webs/src/api/proxy/websiteTraffic.test.mjs)覆盖对应链路。默认与 Asia/Kolkata 时区的 Web 测试均通过，避免本地整点误当 UTC 整点。


## 已闭环：网络拓扑移除与共享能力边界

- 删除拓扑专属页面、图算法、快照接口和路由：原 `webs/src/views/network/Topology.vue`、`topology.ts`、旧 `topology.test.mjs`、`api/topology.go`、旧 `api/topology_test.go`、`routers/topology.go`。
- `main.go` 只删除 `routers.NetworkTopology(r)` 注册一行，没有覆盖其他注册或后台任务。
- [菜单](../api/mentu.go#L66-L72)保留网络工具与多地路由追踪，默认跳转改为 `/network/traceroute`；[前端导航](../webs/src/layout/navigation.ts#L29-L38)只删除拓扑元数据，节点管理仍使用同一个图标。
- 原拓扑只是读取配置关系的 HTTP 快照，前端挂载时请求一次；未发现独立后台拓扑采集任务。因此没有以“删除拓扑采集”为名停掉任何 Agent、节点检测、追踪、监控或流量任务。
- 共享函数 [monitorOnline](../api/monitor.go#L71-L73)、[decodeNodeFlags](../api/sub_pipeline.go#L78-L85)保留；节点与服务器模型、`xraycore` 的期望状态/连接/计数能力不因拓扑移除而删除。
- [README](../README.md#L231-L233)已移除“现有拓扑页面”的陈述，明确共享能力保留。
- 新增[菜单回归测试](../api/network_navigation_test.go#L9)与[禁用服务器仍可获取 Agent 控制状态测试](../api/network_shared_state_test.go#L15)：停用代理不能阻断 Agent 拉取停止配置，也不应连带关闭独立监控。

## Web 开关：期望状态已保存，不等于 Agent 已执行

### 已核实的控制链

1. [toggleServer](../webs/src/views/subcription/servers.vue#L206-L222)调用服务器保存 API。
2. [ProxyServerSave](../api/proxy_api.go#L94-L125)更新数据库并调用 `NotifyProxyChanged`。
3. [NotifyProxyChanged](../api/proxy_runtime.go#L307-L314)不直接推送远端指令；远端依赖下一轮拉取。
4. [AgentSpec](../api/proxy_api.go#L462-L473)经令牌认证返回 [BuildServerSpec](../api/proxy_runtime.go#L49-L57)；禁用服务器仍必须通过认证，否则无法收到停止配置。
5. [RunLoop](../xraycore/agent.go#L65-L108)拉取并调用 [Runner.Apply](../xraycore/runner.go#L53-L65)，再回报实际运行状态。

### 审计发现及并发修正状态

- 原 UI 在保存数据库后即提示“已启用”或“一分钟内停止”，没有等待执行回执；原 Runner 禁用分支调用 `stopLocked` 后直接清空错误、提交新版本，而 `systemctl stop` 的错误被忽略，存在停止失败也伪成功的路径。
- 本次重新读取发现其他并发变更已将 [UI 提示](../webs/src/views/subcription/servers.vue#L217-L221)改为“等待 Agent 应用，以运行状态/最近回报为准”；[Runner 禁用分支](../xraycore/runner.go#L57-L64)也增加停止后仍在运行的检查。此为并发 Agent 的修正，本拓扑任务未修改其逻辑。
- 仍须区分“代码存在控制路径”与“部署中的 Agent 实际执行成功”：网络断开、旧二进制、systemd 权限、服务配置和 Agent 自报版本均需部署验收，不能根据保存成功断言执行成功。
- 已修复原监控关闭后卡片自然过期变离线的语义缺口：保存服务器配置同步关联 Monitor 的 `enabled`，回报不附带指标时仍更新管理心跳、版本与摘要。关闭指标不再意味着 Agent 失联。

## 明确未闭环：README 愿景与实际实现差距

以下均只报告，不扩展实现。

### 1. 不可变版本、出口验证与安全发布

- [README](../README.md#L49-L61)描述生成候选、规则/回归/真实出口验证、与 LKG 比较、全部通过后事务发布；[版本章节](../README.md#L152-L164)描述带测试报告的不可变产物快照。
- 实际 [validateSubscriptionContent](../api/sub_artifact.go#L23-L60)是基础结构校验，例如 YAML 能否解析、是否存在段落等；不等于真实代理出口验证。
- [rememberGoodArtifact](../api/sub_artifact.go#L76-L99)创建记录时直接写入 `ValidationStatus: "valid", TestStatus: "passed"`，随后删除同订阅/客户端旧产物，只保留一份。不能把该字段当作已执行真实出口测试的证据，也不能声称已有不可变多版本历史。
- 当前[订阅路由](../routers/subcription.go#L9-L21)有 `/pipeline/preview`，未见发布提交/版本比较/回滚路由；[模板路由](../routers/template.go#L9-L19)也没有完整安全发布操作。

### 2. Web 任务中心、质量矩阵、规则解释与安全发布入口

- [README 任务中心](../README.md#L137-L150)宣称统一持久化后台任务；[Web 控制台清单](../README.md#L272-L282)宣称质量矩阵、规则解释、版本及安全发布页面。
- [任务路由](../routers/tasks.go#L8-L13)确实提供 list/cancel/retry，但[菜单](../api/mentu.go)未提供任务中心入口。
- 审阅时搜索整个 `webs/src` 中 `TaskCenter|task-center|任务中心|安全发布|LKG|Last Known|quality.?matrix|质量矩阵|规则解释` 无匹配。这支持“README 所列 Web 交互没有对应入口”的结论，不意味着所有底层数据模型或算法均不存在。
- 这些项目不属于本次拓扑移除范围，不补 UI、不虚构实现。

### 3. 更新记录与节点检测说明失配

- 审阅时[前端更新记录](../webs/CHANGELOG.md)最新二开段仍为 `2.1-subL`，描述旧的首页延迟检测及旧样式。本次已新增未发布记录，明确统一 Agent、拓扑移除、代理网站统计、迁移及持久化边界；历史/上游记录仍不能证明 README 所有能力已交付。
- 原 README 同时写 Agent 参与 TCP 检测与“所有检测从面板发起”，本次已将[节点检测说明](../README.md#L254)澄清为面板和 Agent 多视角 TCP 检测，与经代理出口的解锁/国内可达性检测区分。

## 最终验证结果（2026-10-23，未部署生产）

- `go test ./...` 通过。
- `XRAY_WEBSITE_E2E_BIN=/tmp/subl-website-xray-e2e/xray go test -race ./... -count=1` 全部通过：API 22.268s、节点 14.183s、Xray 2.810s，其余包通过或无测试文件。该二进制由既有 EnsureXray 流程下载并校验官方 v26.3.27 SHA-256；真实 Xray SOCKS 出站经认证计量器的 TCP/UDP 回环 echo 验证精确字节，非伪造上报数据。
- 最后补充 Linux 正在运行的 executable inode 摘要回归，避免磁盘替换而尚未重启就虚报新版本；升级、凭据轮换、节点卸载等高风险监控管理 API 也要求 admin scope。再次执行 `XRAY_WEBSITE_E2E_BIN=/tmp/subl-website-xray-e2e/xray go test -race ./xraycore ./middlewares -count=1` 通过。
- `node --test src/api/proxy/websiteTraffic.test.mjs` 7/7 通过；Asia/Kolkata 时区同样通过。覆盖参数、UTC边界、366天限制、加载/成功/空/无结果/失败重试/离线旧数据、竞态回复及只读展示。
- 生产前端 Vite 构建通过（3281模块，43.01s），`vue-tsc --noEmit` 通过。环境默认 pnpm 11 尝试自动安装并因现有锁版本/无 TTY 失败；没有删除依赖目录或重装，而是直接用已安装的 Vite 与 vue-tsc 执行相同构建步骤。仍有既有 Sass legacy API、环境 NODE_ENV 和大 chunk 警告，无编译/类型错误。
- `go build -o /tmp/subl-verified-panel .` 通过，验证 Go 能嵌入重建后的前端资源；未覆盖仓库旧程序或重启生产服务。`git diff --check` 通过。

### 尚未实测与明确限制

- 尚未在生产 systemd 服务器验证 Web 启停、实际服务状态、跨网络升级/重启和 UI 收敛；需对比期望配置、运行中摘要、实际 xray 状态与最后回报，不能把保存成功视为执行成功。
- 真实 Xray 测试覆盖本次计量器 SOCKS TCP/UDP 链路，并非生产公网 VLESS/Reality/TLS 全组合、长时间负载或每种内核平台的测试。
- 网站批次的幂等去重不等于旧账户配额流量链路也已全量升级为幂等协议；旧 `Report.Traffic` 仍存在 HTTP 已提交但响应丢失时重复增量的历史风险，未在本次网站统计中扩展改写。
- 上述 README 安全发布、不可变多版本历史及部分 Web 入口仍未闭环；本次如实列明而不声称已实现。


## 本地后续修复：监控布局与 Agent 生命周期（2026-10-23，待部署）

- 浏览器本地复现确认卡片留白根因为未声明的 `UiButton size="sm"` 透传给 DOM，被 UnoCSS attributify 解释为 24rem 宽高（384px）。修复后小号按钮高度 28px；同一组样例卡片从约 644px 降至 305–325px。
- 网站统计页面移除全部筛选表单，使用每次刷新重新计算的最近 24 小时窗口、全部节点、合计排序、最多 100 条；默认仍收起为前 5 条。API 保留既有查询能力，汇总不被排名截断。
- `Spec.Rev` 排除 Monitor/AgentUpdate/AgentHash，防止仅更新监控或管理状态重启代理。WebsiteTraffic 与实际入站仍参与摘要。
- Agent 连接替换先发布新连接，再在注册表锁外关闭旧连接，避免 shutdown 重入同一把锁。WebSocket 增加 ping/pong 和写入超时；客户端取消会关闭空闲连接并取消关联操作。
- 终端输出结束后 Wait 回收 shell 并上报实际退出码；同 ID 替换时旧会话退出不会删除新会话。
- 程序摘要按运行进程缓存，Linux 仍读取 `/proc/self/exe`，exec 后重新计算；失败不缓存。节点检测使用固定 worker 池而不是每节点创建等待 goroutine，HTTP 和 TCP 请求使用 context；检测回报检查业务 code，最多尝试 3 次，除 429 外的 4xx 不重试。
- 本节为工作区修复，不表示既有生产面板和远程 Agent 已升级。此前未闭环的安全发布等功能边界继续适用。

### 本地验证结果

- `go test ./... -count=1 -timeout 120s`：通过。
- `go test -race . ./api ./xraycore -count=1 -timeout 90s`：通过。
- Web 三组 Node 测试共 16 个用例：通过；`vue-tsc --noEmit`：通过；Vite 生产构建：通过（保留既有 Monaco 大包提示）。
- 使用 `/tmp/subl-website-xray-e2e/xray` 运行 `TestWebsiteXraySOCKSE2E`：真实 TCP/UDP 转发及载荷计量通过，未访问生产数据库。
- `go build -tags=with_utls,with_quic`：通过；本地测试二进制为 `/tmp/subl-monitor-agent-20261023`，未部署。
- 程序摘要基准：修复前 5 次运行约 5.8 ms/op；缓存后 100000 次热路径约 4.8 ns/op。此为本机摘要读取基准，不代表整套 Agent 吞吐量提升倍数。
