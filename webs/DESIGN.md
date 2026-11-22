# PPEELink 前端设计

## 工作区

基于 `taste-skill`（`The quiet instrument` × `Three stones`）重构：桌面使用固定深炭黑侧栏（`#181817`），手机使用可关闭侧栏抽屉。顶栏与浅石色画布（`#eeecea`）融为一体，提供当前位置、页面搜索（`Ctrl/⌘ K`）、测试任务状态和账户菜单。管理员默认进入「订阅链接」，首屏即可选择客户端、复制链接或扫码导入；普通用户仅访问「我的订阅」。

功能分类按使用频率重写为 5 个意图域（保留全部后端授权路由与路径兼容）：

- **分发中心**：订阅链接、节点资产（普通用户为我的订阅）。
- **算力与网络**：自建实例、中转网关、主机与 Agent、容器与文件。
- **诊断实验室**：故障诊断、解锁与可达、路由追踪、吞吐互测。
- **态势总览**：运行态势、流量排行。
- **策略与系统**：分流编排、配置模板、规则仓库、成员与权限、系统偏好。

## 视觉与动效

`src/styles/tokens.scss` 采用 **Three stones** 三阶石色调（`#eeecea` 画布、`#f7f6f4` 面板、`#e4e1dd` 控件填充、`#1e1d1c` 炭墨、`#4a5d48` 苔石主操作色，及配套深色主题）。面板之间以明度阶梯与留白分隔，取消静止态阴影与 1px 线框堆砌；仅浮层保留柔和阴影。

动效统一由 `src/motion/index.ts` 与 `src/styles/base.scss` 驱动：全面采用指数缓出（`cubic-bezier(0.22, 1, 0.36, 1)`）与平滑位移（`cubic-bezier(0.76, 0, 0.24, 1)`），移除弹性回弹与无意义无限脉冲。包含页头与登录页的遮罩抬升标题（Masked headline）、侧栏与分段控件的物理滑动指示器、路由与列表错峰进场，以及全量 `prefers-reduced-motion` 降级支持。和技术数值使用 JetBrains Mono。页面用 `PageHeader` 放标题和主要动作，表格及详情采用分隔线组织信息，弹窗沿用统一的标题和操作区。

节点名称打开详情。连接区显示主机、协议、地区和分组。检测区显示当前策略、最近检测时间和探针结果，提供国内可达性与解锁检测入口。订阅下发区集中显示暂停原因、恢复时间及下发控制。复制和编辑保留在抽屉底部。

页面切换、选择和反馈使用短动效。组件按系统的减少动态效果偏好调整。键盘用户可通过跳转链接、页面搜索、Tab、方向键和 Escape 使用工作区。

## 组件参考

以下组件均位于 `src/ui/`。`?` 表示可选；未列出默认值的属性沿用 Vue 的类型默认行为（Boolean 为 false，其余为 undefined）。原生属性、事件及 ARIA 属性可传给组件；`UiTable`、`UiDialog`、`UiDrawer` 将额外属性透传至 Element Plus 底座。

| 组件 | props（类型与默认值） | 用途 / 插槽 |
|---|---|---|
| `UiButton` | `variant?: primary / secondary / ghost / danger = secondary`；`type?: button / submit / reset = button`；`disabled?: boolean`；`loading?: boolean`；`icon?: Component` | 统一按钮。默认插槽为文字，`icon` 插槽可替代图标；loading 时禁用，仅 primary 启用磁吸。纯图标使用者必须提供 aria-label 与 hover/focus tooltip。 |
| `RowIconButton` | `label: string`；`icon: Component` | 44px 行操作按钮，自带 aria-label 与 hover/focus tooltip；透传 disabled、loading、click 等至 ghost UiButton。直接从组件文件导入。 |
| `UiPanel` | `title?: string`；`description?: string`；`count?: number / string`；`flush?: boolean`；`collapsible?: boolean`；`defaultOpen?: boolean` | 区块卡片：头部标题 + 计数徽标 + 说明，`actions` 插槽放筛选/操作；`flush` 让列表贴边；`collapsible` 可折叠。默认内容及 `header`、`footer` 插槽。 |
| `PageHeader` | `title?: string`；`description?: string` | 页面唯一 h1、一句说明与右侧 `actions`；默认插槽可替代标题。`eyebrow` 仅为兼容保留，不再渲染。 |
| `UiStat` | `label: string`；`value: number / null`；`size?: hero / large / medium / small = large`；`decimals?: number = 0`；`unit?: string`；`trend?: string`；`state?: default / ok / warn / bad = default`；`thresholds?: { warn: number; bad: number }` | 真实指标与计数动画；null / 非有限数显示空值。阈值优先于 state，数值越高越差；`label`、`trend` 插槽。hero/large 用 display，medium/small 用 mono。 |
| `StatusDot` | `state?: ok / warn / bad / idle = idle`；`label?: string`；`running?: boolean` | 文字与形状共同表达状态；ok 呼吸，running 扫描；默认插槽可替代标签。 |
| `UiTable` | `rows: Record<string, any>[]`；`rowKey?: string = id`；`loading?: boolean`；`label?: string = 数据列表` | 桌面 EP 横线表，手机卡片；默认插槽定义列，`empty` 定义空态，`mobile-card({ row, index })` 自定义移动内容。默认空态为 section。 |
| `EmptyState` | `title?: string = 暂无内容`；`description?: string`；`size?: page / section / inline = page` | 页面 / 区块 / 行内空态，`icon`、`action` 插槽。仪表盘及弹层使用 section/inline。 |
| `UiSkeleton` | `lines?: number = 3`；`height?: string = 24px` | 加载占位；默认插槽可替代行，末行默认 65% 宽；带 busy/status 语义。 |
| `UiDialog` | `modelValue: boolean`；`title: string`；`width?: string = 560px`；`destroyOnClose?: boolean = true` | 统一对话框；`update:modelValue`，默认内容及 `header`、`footer` 插槽。EP 负责焦点进入、Tab 陷阱、关闭归还与 Esc；透传 open/closed 及 autofocus 事件。 |
| `UiDrawer` | `modelValue: boolean`；`title: string`；`size?: string = 480px`；`direction?: rtl / ltr / ttb / btt = rtl`；`destroyOnClose?: boolean = true` | 统一抽屉；`update:modelValue`，默认内容及 `header`、`footer` 插槽；焦点与 Esc 使用 EP 底座。 |
| `UiSegmented` | `modelValue: string / number`；`options: { label: string; value: string / number; disabled?: boolean }[]`；`label: string` | 带 aria-label 的 radio 分段切换；方向键/Home/End 跳过禁用项，`update:modelValue`；默认插槽接收 option。 |
| `UiTag` | `tone?: neutral / signal / ok / warn / bad = neutral` | 轻量语义标签；默认插槽为内容。 |
| `CommandPalette` | `modelValue: boolean`；`items: CommandItem[]` | Ctrl/⌘+K 开关、模糊匹配、方向键选择、Enter 执行、Esc 关闭。items 由外壳的可见路由加主题/退出动作提供；关闭并释放焦点后才执行动作。`update:modelValue`。 |
| `CountryFlag` | `code?: string`；`label?: string` | ISO 双字母国旗；不支持彩色 emoji 或代码无效时回退可读代码。label 同步用于 title 与 aria-label。直接从组件文件导入。 |

`CommandItem`：`{ id: string; label: string; group?: string; icon?: string; keywords?: string; run: () => void | Promise<unknown> }`。`src/ui/index.ts` 导出上述主体组件及 CommandItem 类型；`RowIconButton`、`CountryFlag` 使用直接导入。

动效统一使用 `src/motion`。CSS 动效由 `src/styles/base.scss` 的系统 reduced-motion 与 `html[data-motion="reduced"]` 双重终态守卫覆盖；地图、图表和 Monaco 读取同一 motion 状态。业务提交中的弹层保留现有关闭保护，空闲时支持 Esc。

### 借鉴功能（参考妙妙屋）
- 订阅「分享到客户端」：客户端卡片网格选择 → 二维码 + 订阅链接 + 一键导入 + 短链。
- 订阅到期：订阅详情与仪表盘展示到期时间与剩余天数。
- 模板构建「规则分类」：勾选分类自动增删规则集、规则行与专属分组，含精简 / 均衡 / 全量预设。
- 模板构建「策略分组」：分组卡片看板，拖拽策略进分组、拖拽卡片排序，支持「添加到 / 移出所有分组」投放区；触屏用下拉与菜单完成同样操作。
- 节点批量：正则改名（预览后应用，订阅中的节点顺序同步改名）、加 / 去国旗、UDP / TFO / 跳过证书开关、按“协议+服务器+端口”去重（引用转移给保留的节点）。
- 链式代理：节点设置中转节点，Clash 输出 `dialer-proxy`、Surge 输出 `underlying-proxy`，中转不在订阅里时自动补上。
- 节点不展示延迟：面板服务器 TCP 连接节点端口，连得上即“有效”，否则“无效”；节点页可“检测有效性”立即重测。首页显示有效节点数与有效率，订阅编辑、测试页的节点选择只按有效与否排序/过滤。“节点测试”里也只显示可用/可达，不显示毫秒。
- 暂不下发（`api/node_delivery.go`）：手动暂停到恢复日（节点菜单“暂停下发…”、批量栏）、自建服务器超出月流量上限（同时停止 xray，到每月重置日恢复）、连续多次检测无效（健康事件为 down；若订阅里全部节点都是这种情况则照常下发，避免面板自身网络问题让订阅变空）。条件解除自动恢复；节点列表显示“暂不下发 · 原因”，订阅详情列出被拦下的节点。
- 国旗：国家按名称识别优先（国旗 emoji、中文/英文名、全大写的 JP / USA 等代码片段），识别不到才用 GeoIP；添加/编辑节点时可选“自动 / 指定国家 / 不加国旗”，保存时把国旗写到名称前。国家表在 `node/country.go`，前端经 `/api/v1/nodes/countries` 共用。
- 节点测试：`/test/index` 两个标签——解锁检测（结果由节点出口 IP 决定，与从哪台电脑发起无关）和国内可达性（经节点连国内各省三网服务器，判断节点 IP 是否被墙）。测速已删除：从面板服务器测出的速度不代表用户电脑上的体验。旧地址自动跳转。
- Clash 节点集合：订阅可改用 proxy-provider 引用 `client=clash-proxies` 节点列表，客户端每小时自动刷新。
- 多用户：管理员 / 普通用户，普通用户只看到“我的订阅”；新账户与默认密码账户首次登录必须改密码。
- 单节点临时订阅：HMAC 签名、自带到期时间的 `/t/` 链接。
- 系统设置：完整备份（数据库快照 + 模板 tar.gz）、配置导入导出、API 令牌、操作审计、短链管理。

### 自建节点（xray-core）
- **服务器**：面板服务器（xray 与面板装在同一台机器，面板直接管理）或远程服务器（同一程序的 `agent` 子命令，凭令牌主动连回面板拉配置、回报流量，无需 SSH 密码与额外端口）。安装命令由面板生成，令牌只显示一次。
- **xray-core**：按固定版本（`xraycore.DefaultVersion`）从 GitHub 官方 release 下载，用同一 release 的 `.dgst` 校验 SHA2-256 后才落盘；有 systemd 时托管为 `ppeelink-xray` 服务，面板 / agent 重启不断连。新配置先 `xray run -test` 自检，失败继续跑旧配置。
- **入站**：VLESS Reality Vision、VLESS XHTTP Reality；Reality 密钥与 shortId 自动生成，伪装目标可一键检测 TLS 1.3 / h2。
- **账户**：每个面板账户一个 UUID，按 `u<用户ID>` 统计流量；额度、每月清零日、到期、同时在线 IP 上限（超限临时停用 10 分钟）。账户在“我的订阅”里多一份“自建节点”订阅；管理员的节点同步进“自建·服务器名”分组，可被已有订阅引用。
- **防滥用**：所有入站默认阻断内网地址与 BT；服务器可一键停用（xray 随即停止）。
- **删除即卸载**：删除面板服务器时立即停止并删除本机 xray（systemd 服务、程序与配置）；删除远程服务器时先标记“正在卸载”，agent 下一轮拉取时卸载 xray 与自身并回报，面板再删除记录。agent 从未连上或无回应时可强制删除，并给出手动卸载命令。
