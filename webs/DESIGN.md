# PPEELink 设计规范 - "Ion"（v3，2026-10-18 起，取代 Signal Atlas）

> 前端唯一视觉/交互标准。taste-skill 档位：**DESIGN_VARIANCE 10 · MOTION_INTENSITY 10 · VISUAL_DENSITY 4**（用户要求设计与动效开到最大）。

## 1. Design Read
自托管代理订阅控制台的整体视觉重做，用户是懂行的单人管理员。语言：**dark-tech 精密仪器感**，冷色 zinc 中性色 + 唯一的青柠强调色，深浅双主题。
不要：衬线大标题（Instrument Serif 已移除）、暖米色/骨白底、紫色渐变、三等分卡片、emoji 当图标、装饰性网格线、em-dash。

## 2. 设计 tokens（`src/styles/tokens.scss`，`:root` 浅色，`html.dark` 深色）

| token | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `--c-canvas` | `#F3F4F6` | `#0B0C0E` | 页面底 |
| `--c-surface` | `#FCFCFD` | `#131418` | 卡片 / 面板 |
| `--c-surface-2` | `#ECEEF1` | `#1B1D22` | 输入底、悬停、凹陷区 |
| `--c-line` | `#DCDFE4` | `#262930` | 1px 边线 |
| `--c-ink` | `#101216` | `#ECEEF2` | 主文字 |
| `--c-muted` | `#596070` | `#8D93A0` | 次要文字 |
| `--c-signal` | `#C2EC2C` | `#C8F03A` | 强调**填充**（主按钮、选中图标块、进度、地图节点光晕）。其上文字一律 `--c-on-signal` |
| `--c-signal-text` | `#4B6600` | `#D2F55C` | 强调**文字** |
| `--c-focus` | `#4B6600` | `#C8F03A` | 焦点环、选中描边、开关、地图节点（满足 3:1） |
| `--c-data` | `#2F6BFF` | `#6F9BFF` | 第二数据色（图表、渐变） |
| `--c-ok / warn / bad` | `#14915C / #C27A06 / #D63A3F` | `#3FD18F / #F4B13D / #FF6467` | 状态 |
| `--c-glow` | lime 18% | lime 14% | 光晕、聚光灯 |

形状规则：按钮 / 分段 / 标签 / 胶囊 = `--radius-pill`；输入 10px；卡片与面板 16px；弹层 20px。卡片阴影 = `--shadow-panel` + `--edge-highlight`（顶部 1px 内高光）。

## 3. 字体与字号
- 只用两套自托管字体：**Geist**（标题与界面，`--font-ui` / `--font-display` 同指 Geist）+ **JetBrains Mono**（数字、主机、链接）。
- 页面标题 clamp(24px, 2.4vw, 32px) 600，字距 -0.02em（中文不能再紧）；面板标题 15px 600；正文 14；次要 13；元信息 12（最小）。
- `UiStat`：`hero` 为首页超大数字（Geist 600，最高 112px，-0.06em）；`large` 26px 600；`medium/small` mono。

## 4. 布局
- **外壳**：顶部 64px 粘性栏，毛玻璃底。左：Logo + 字标；中：胶囊形主导航，"首页 / 订阅管理 / 测试 / 模板"，分组悬停或点击展开下拉菜单（图标块 + 名称 + 一句说明），胶囊高亮弹簧跟随指针；右：搜索胶囊（⌘K）、测试任务按钮（仅在真实运行时出现状态点）、主题切换、头像菜单。<1100px 主导航收进右侧菜单按钮，展开为全屏页面清单。页面顶部有一团极淡的青柠光晕。
- **内容**：最大宽 1400px 居中；页面 = PageHeader → 可选 `.stat-tiles` → 若干 `UiPanel`。首页为 12 列 bento：节点总数大块（2 行，青柠/蓝色渐变底）、订阅、连接评分环、订阅到期、节点地图（点阵大陆）、所在地区，单元格数量与内容一致，手机单列。
- **登录页**：左侧圆角深色舞台（缓慢漂移的三色光场 + 大标题），右侧表单。

## 5. 组件层（`src/ui/`，全部自建，EP 仅作复杂控件底座）
`UiButton`（primary=signal 实心 / secondary=描边 / ghost / danger；支持 loading、icon、磁吸 `v-magnetic` 仅用于 primary）、`UiPanel`、`PageHeader`、`UiStat`（按 size 区分 display / mono 数字 + 计数动画 + 标签 + 可选趋势）、`StatusDot`（ok/warn/bad/idle，ok 带呼吸脉冲）、`UiTable`（包 el-table，统一样式 + 移动卡片插槽）、`EmptyState`、`UiSkeleton`、`UiDialog` / `UiDrawer`（包 el-dialog/el-drawer，统一头尾）、`UiSegmented`（分段切换，滑块弹簧动画）、`UiTag`、`CommandPalette`（⌘K / Ctrl+K，模糊搜索所有路由 + 常用动作，方向键 + Enter）。
Element Plus 的 input/select/date/switch/message/notification/dropdown/tooltip 必须通过 `src/styles/element.scss` 全局重皮肤到上述 tokens，视觉上与自建组件一致。

## 6. 动效（MOTION 10，统一在 `src/motion/`）
- 基础：GSAP（已安装）+ CSS transitions。缓动：`--ease-out: cubic-bezier(.16,1,.3,1)`，弹簧用 GSAP `elastic.out(1,.6)` 或 `back.out(1.6)`；时长 180 / 320 / 560ms 三档。
- 必做：
  1. 路由过渡：内容区 clip/translate+opacity 交错入场（标题先、内容后，stagger 40ms），离场 140ms。
  2. 列表/表格首屏行 stagger 入场；筛选结果变化用 `<TransitionGroup>` FLIP。
  3. `UiStat` 数字从 0 滚动到值（字体按第 3 节 size 规则）。
  4. 顶栏导航胶囊弹簧跟随悬停项；下拉菜单缩放淡入；主题图标旋转切换；卡片/面板指针聚光灯（`installSpotlight`）；路由内容模糊 + 位移交错入场（expo.out）；数字计数 1.1s；首页评分环描边动画；地图节点脉冲；登录页光场漂移。
  5. 主按钮 `v-magnetic` 磁吸（指针 24px 内吸附，离开弹回）。
  6. 状态脉冲：在线点呼吸、测速/检测进行中扫描线或进度波纹。
  7. 仪表盘地图/地球：底图始终渲染，缓慢扫描 / 经纬线描线可暂停，reduced-motion 静止；节点位置点与标注线仅来自 `getNodeMap` 真实数据，无节点时左下角叠加小号说明与管理节点按钮。
  8. 骨架屏 shimmer；对话框/抽屉弹簧入场。
- 守则：所有动画经 `useMotion()`/`prefersReducedMotion` 守卫，reduced-motion 时直接到终态；组件卸载时 kill tween / 移除监听 / cancel rAF；只动 transform/opacity/clip-path；除可暂停的地图底图扫描外，不要永不停的装饰性无限循环（脉冲仅限真实状态）。

## 7. 文案
中文、短、具体。标题 2–6 字；说明 ≤ 25 字；按钮用动词（"添加节点"、"开始检测"）。删除重复说明、英文装饰词（如 "TELEMETRY"、"LIVE"、"CORE"），mono eyebrow 只用于分组名（如 `订阅 / 01`）。

## 8. 无障碍与响应式
- 所有可交互元素可 Tab 到达，`:focus-visible` 为 2px `--c-signal` 外环 + 2px offset；图标按钮必须有 `aria-label`。
- 对比度 AA；不只用颜色表达状态（配文字/形状）。
- 断点：390 / 768 / 1024 / 1440 均无整页横向滚动；触控目标 ≥ 44px。

## 9. 验收清单（每阶段）
1. `pnpm build`（含 vue-tsc）通过；改动文件 eslint 无 error。
2. `src/api/**`、`src/utils/request.ts`、`src/store/modules/user.ts`、`src/store/modules/permission.ts` 未修改；页面文件路径不变（后端菜单按 `views/<path>.vue` 映射）。
3. 每个页面原有的 API 调用、按钮、对话框、流程全部保留（对照任务书清单）。
4. 无硬编码颜色（除 tokens.scss / echarts 主题读取 token）；无 12px 以下字号；无 TagsView/面包屑/旧 Settings 抽屉残留。
5. 浏览器：控制台 0 error；390/768/1440 无横向溢出；深浅主题；键盘可用；reduced-motion 生效。


## 10. 组件清单（实际实现）

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

### 动效清单（MOTION 10）
- 路由内容模糊上浮交错入场；页面标题逐字上浮（`PageHeader`，每字 28ms）。
- 卡片 / 面板指针聚光灯；`.cell`、客户端卡片、订阅卡片、指标卡随指针 3D 倾斜（±5°，`installSpotlight`）。悬停抬起用独立的 `translate` 属性，与倾斜的 `transform` 叠加。
- `v-reveal`：首屏以下的面板与卡片滚入视口时模糊上浮出现（IntersectionObserver，只播一次）。
- 链式代理示意的虚线流动、批量改名预览逐行滑入、设置页分区切换模糊过渡。
- 全部受 `prefers-reduced-motion` 与 `html[data-motion="reduced"]` 控制。

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
