# PPEELink 设计规范 —— "Signal Atlas"

> 这是本次前端完全重构的唯一视觉/交互标准。所有实现必须对照本文件；验收方按第 9 节逐项检查。
> taste-skill 档位：**DESIGN_VARIANCE 10 · MOTION_INTENSITY 10 · VISUAL_DENSITY 4**。

## 1. Design Read
自托管网络/订阅运维台，用户是懂行的单人管理员（中文界面）。语言：**编辑感排版 + 精密仪器感数据**。
像一份会动的航线图/信号报告，而不是 Element Admin 模板。不要：紫色渐变、满屏玻璃拟态、霓虹赛博网格、
居中大 Hero、三等分特性卡片、Inter + slate 默认风、emoji 当图标、到处 badge。

## 2. 设计 tokens（唯一色源，组件里禁止硬编码散色）
定义在 `src/styles/tokens.scss`，`:root` 为浅色，`html.dark`（沿用现有 dark class 机制）为深色，并同步映射到 Element Plus 的 `--el-*` 变量。

| token | 浅色 | 深色 | 用途 |
|---|---|---|---|
| `--c-canvas` | `#F3F1EC` | `#121314` | 页面底 |
| `--c-surface` | `#FBFAF7` | `#1A1B1D` | 面板 |
| `--c-surface-2` | `#ECE9E2` | `#222326` | 凹陷区/表头/输入底 |
| `--c-line` | `#DAD6CD` | `#2E3033` | 发丝线 1px |
| `--c-ink` | `#17181A` | `#ECEAE4` | 主文字 |
| `--c-muted` | `#625F5A` | `#9A9891` | 次要文字（对比度 ≥ 4.5:1） |
| `--c-signal` | `#E4472B` | `#FF6A4D` | 图形强调：填充背景、边框、焦点环、指示条与图形 |
| `--c-signal-text` | `#AF301A` | `#FF8A70` | 文字强调：选中项、计数、链接式按钮、eyebrow 与强调数字（对比度 ≥ 4.5:1） |
| `--c-data` | `#2F5BEA` | `#6E8CFF` | 数据/图表/链接 |
| `--c-ok` | `#1F9D6B` | `#3CC48D` | 正常/在线 |
| `--c-warn` | `#D99A1E` | `#F0B43C` | 警告/偏慢 |
| `--c-bad` | `#C8321E` | `#FF5A45` | 失败/离线 |

对比度核算（WCAG sRGB，依次为 canvas / surface / surface-2）：浅色 muted **5.63 / 6.09 / 5.24 : 1**；深色 muted **6.44 / 5.97 / 5.44 : 1**。

文字强调色对比度（依次为 canvas / surface / surface-2）：浅色 **5.71 / 6.17 / 5.31 : 1**，深色 **8.08 / 7.49 / 6.83 : 1**。在三种底色分别叠加 signal 12% 后，浅色为 **4.91 / 5.28 / 4.59 : 1**，深色为 **6.94 / 6.35 / 5.76 : 1**。朱红文字统一用 `--c-signal-text`，不从填充色派生；Element Plus 的 primary 文字及旧 ui-accent 别名映射到文字色，实心按钮等图形填充单独保留 `--c-signal`。

派生：`color-mix(in srgb, var(--c-signal) 12%, transparent)` 用作选中底色；阴影只用一层极淡 `0 1px 0 var(--c-line)` + 弹层用 `0 24px 48px -24px rgb(0 0 0 / .25)`。
圆角：控件 10px，面板 16px，弹层 20px，状态点/标签 999px。

## 3. 字体与字号
- 自托管 woff2（`public/fonts/`，在 `src/styles/fonts.css` 用 `@font-face` + `font-display: swap`，base 为 `/static/fonts/...` 时需走 Vite 资源路径，确保生产和开发都能加载）：
  - 展示：**Instrument Serif**（400 normal/italic）→ `--font-display`。用于页面大标题、英雄数字、关键大数字旁的标签、页面级空状态标题。中文回退 `"Songti SC","Noto Serif SC",serif`。
  - 界面：**Geist**（variable）→ `--font-ui`，回退 `"PingFang SC","HarmonyOS Sans SC","Microsoft YaHei","Noto Sans SC",system-ui,sans-serif`。
  - 数字/代码/链接/IP：**JetBrains Mono**（variable）→ `--font-mono`，统一 `font-variant-numeric: tabular-nums`。
- `UiStat` 通过 `size` 控制数字层级：`hero` / `large`（≥40px）使用 `--font-display` 与 `font-variant-numeric: lining-nums tabular-nums`；`medium`（32px）/ `small`（≤24px）继续使用 mono。
- 字号：正文 16 / 控件 15 / 次要 14 / 小标签 13（仅 mono 大写 eyebrow 用，letter-spacing .08em）；页面标题 clamp(40px, 5vw, 72px) display 字体，行高 1.0；区块标题 22px UI 字体 600。最小字号 13，禁止 12 及以下。
- 行高：正文 1.6，控件 1.4。

## 4. 布局（VARIANCE 10，DENSITY 4）
- 间距栅格 8px（4 仅用于图标与文字间）。页面左右留白：桌面 48px，平板 32px，手机 16px。内容最大宽 1440px，但**不居中对称**：标题区左对齐并允许出血到左侧 dock 边。
- **外壳**：桌面 = 左侧 76px 竖向 dock（图标 + hover/focus 展开 tooltip 标签，分组之间发丝分隔），顶部无传统 navbar，仅页面内的 PageHeader；右上角固定一个小工具簇（⌘K 搜索、任务状态指示、主题切换、用户菜单）。手机（<768）= 底部 dock（最多 5 个一级入口 + "更多" sheet）+ 单列。
- **页面骨架**：`PageHeader`（mono eyebrow 分组名 / display 大标题 / 一句话说明可省 / 右侧主操作）→ 内容。区块之间用 48–64px 空白和发丝线分隔，而不是层层卡片嵌套。
- 非对称：列表页采用 "主列表 8 : 侧信息 4" 或 "数字条带横跨 + 列表"；仪表盘用不规则网格（一个大块 + 若干不等宽块），禁止三/四等分卡片排一行。
- 表格：行高 ≥ 48px，表头 13px mono 大写 muted，无竖线，仅横向发丝线；hover 行底 `--c-surface-2`；操作列最多 2 个可见按钮 + "更多" 下拉；手机下表格转为卡片列表（不出现横向滚动整页）。
- 空状态：`EmptyState size="page"` 使用 display 大标题；`section` 使用 20px UI 标题、14px 说明与次级按钮；`inline` 使用一行 muted 文本。仪表盘内只使用 section / inline；绝不塞假数据。

## 5. 组件层（`src/ui/`，全部自建，EP 仅作复杂控件底座）
`UiButton`（primary=signal 实心 / secondary=描边 / ghost / danger；支持 loading、icon、磁吸 `v-magnetic` 仅用于 primary）、`UiPanel`、`PageHeader`、`UiStat`（按 size 区分 display / mono 数字 + 计数动画 + 标签 + 可选趋势）、`StatusDot`（ok/warn/bad/idle，ok 带呼吸脉冲）、`UiTable`（包 el-table，统一样式 + 移动卡片插槽）、`EmptyState`、`UiSkeleton`、`UiDialog` / `UiDrawer`（包 el-dialog/el-drawer，统一头尾）、`UiSegmented`（分段切换，滑块弹簧动画）、`UiTag`、`CommandPalette`（⌘K / Ctrl+K，模糊搜索所有路由 + 常用动作，方向键 + Enter）。
Element Plus 的 input/select/date/switch/message/notification/dropdown/tooltip 必须通过 `src/styles/element.scss` 全局重皮肤到上述 tokens，视觉上与自建组件一致。

## 6. 动效（MOTION 10，统一在 `src/motion/`）
- 基础：GSAP（已安装）+ CSS transitions。缓动：`--ease-out: cubic-bezier(.16,1,.3,1)`，弹簧用 GSAP `elastic.out(1,.6)` 或 `back.out(1.6)`；时长 180 / 320 / 560ms 三档。
- 必做：
  1. 路由过渡：内容区 clip/translate+opacity 交错入场（标题先、内容后，stagger 40ms），离场 140ms。
  2. 列表/表格首屏行 stagger 入场；筛选结果变化用 `<TransitionGroup>` FLIP。
  3. `UiStat` 数字从 0 滚动到值（字体按第 3 节 size 规则）；延迟数值变化时颜色随阈值过渡。
  4. dock 图标弹簧放大 + 选中指示条滑动（FLIP）。
  5. 主按钮 `v-magnetic` 磁吸（指针 24px 内吸附，离开弹回）。
  6. 状态脉冲：在线点呼吸、测速/检测进行中扫描线或进度波纹。
  7. 仪表盘地图/地球：底图始终渲染，缓慢扫描 / 经纬线描线可暂停，reduced-motion 静止；节点位置点与标注线仅来自 `getNodeMap` 真实数据，无节点时左下角叠加小号说明与管理节点按钮。
  8. 骨架屏 shimmer；对话框/抽屉弹簧入场。
- 守则：所有动画经 `useMotion()`/`prefersReducedMotion` 守卫，reduced-motion 时直接到终态；组件卸载时 kill tween / 移除监听 / cancel rAF；只动 transform/opacity/clip-path；除可暂停的地图底图扫描外，不要永不停的装饰性无限循环（脉冲仅限真实状态）。

## 7. 文案
中文、短、具体。标题 2–6 字；说明 ≤ 25 字；按钮用动词（"添加节点"、"开始测速"）。删除重复说明、英文装饰词（如 "TELEMETRY"、"LIVE"、"CORE"），mono eyebrow 只用于分组名（如 `订阅 / 01`）。

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
| `UiPanel` | `title?: string` | 面板容器；默认内容及 `header`、`actions`、`footer` 插槽。 |
| `PageHeader` | `title?: string`；`eyebrow?: string`；`description?: string` | 页面唯一 h1 与右侧 `actions`；默认插槽可替代标题。eyebrow 未传时按路由分组与序号推导，传空字符串可隐藏。登录页沿用专用双栏标题。 |
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
