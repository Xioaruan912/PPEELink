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
| `--c-muted` | `#6B6A66` | `#9A9891` | 次要文字（对比度 ≥ 4.5:1） |
| `--c-signal` | `#E4472B` | `#FF6A4D` | 主强调：主按钮、选中、焦点环、关键数字 |
| `--c-data` | `#2F5BEA` | `#6E8CFF` | 数据/图表/链接 |
| `--c-ok` | `#1F9D6B` | `#3CC48D` | 正常/在线 |
| `--c-warn` | `#D99A1E` | `#F0B43C` | 警告/偏慢 |
| `--c-bad` | `#C8321E` | `#FF5A45` | 失败/离线 |

派生：`color-mix(in srgb, var(--c-signal) 12%, transparent)` 用作选中底色；阴影只用一层极淡 `0 1px 0 var(--c-line)` + 弹层用 `0 24px 48px -24px rgb(0 0 0 / .25)`。
圆角：控件 10px，面板 16px，弹层 20px，状态点/标签 999px。

## 3. 字体与字号
- 自托管 woff2（`public/fonts/`，在 `src/styles/fonts.css` 用 `@font-face` + `font-display: swap`，base 为 `/static/fonts/...` 时需走 Vite 资源路径，确保生产和开发都能加载）：
  - 展示：**Instrument Serif**（400 normal/italic）→ `--font-display`。只用于页面大标题、关键大数字旁的标签、空状态标题。中文回退 `"Songti SC","Noto Serif SC",serif`。
  - 界面：**Geist**（variable）→ `--font-ui`，回退 `"PingFang SC","HarmonyOS Sans SC","Microsoft YaHei","Noto Sans SC",system-ui,sans-serif`。
  - 数字/代码/链接/IP：**JetBrains Mono**（variable）→ `--font-mono`，统一 `font-variant-numeric: tabular-nums`。
- 字号：正文 16 / 控件 15 / 次要 14 / 小标签 13（仅 mono 大写 eyebrow 用，letter-spacing .08em）；页面标题 clamp(40px, 5vw, 72px) display 字体，行高 1.0；区块标题 22px UI 字体 600。最小字号 13，禁止 12 及以下。
- 行高：正文 1.6，控件 1.4。

## 4. 布局（VARIANCE 10，DENSITY 4）
- 间距栅格 8px（4 仅用于图标与文字间）。页面左右留白：桌面 48px，平板 32px，手机 16px。内容最大宽 1440px，但**不居中对称**：标题区左对齐并允许出血到左侧 dock 边。
- **外壳**：桌面 = 左侧 76px 竖向 dock（图标 + hover/focus 展开 tooltip 标签，分组之间发丝分隔），顶部无传统 navbar，仅页面内的 PageHeader；右上角固定一个小工具簇（⌘K 搜索、任务状态指示、主题切换、用户菜单）。手机（<768）= 底部 dock（最多 5 个一级入口 + "更多" sheet）+ 单列。
- **页面骨架**：`PageHeader`（mono eyebrow 分组名 / display 大标题 / 一句话说明可省 / 右侧主操作）→ 内容。区块之间用 48–64px 空白和发丝线分隔，而不是层层卡片嵌套。
- 非对称：列表页采用 "主列表 8 : 侧信息 4" 或 "数字条带横跨 + 列表"；仪表盘用不规则网格（一个大块 + 若干不等宽块），禁止三/四等分卡片排一行。
- 表格：行高 ≥ 48px，表头 13px mono 大写 muted，无竖线，仅横向发丝线；hover 行底 `--c-surface-2`；操作列最多 2 个可见按钮 + "更多" 下拉；手机下表格转为卡片列表（不出现横向滚动整页）。
- 空状态：display 字体标题 + 一句引导 + 主操作按钮；绝不塞假数据。

## 5. 组件层（`src/ui/`，全部自建，EP 仅作复杂控件底座）
`UiButton`（primary=signal 实心 / secondary=描边 / ghost / danger；支持 loading、icon、磁吸 `v-magnetic` 仅用于 primary）、`UiPanel`、`PageHeader`、`UiStat`（mono 大数字 + 计数动画 + 标签 + 可选趋势）、`StatusDot`（ok/warn/bad/idle，ok 带呼吸脉冲）、`UiTable`（包 el-table，统一样式 + 移动卡片插槽）、`EmptyState`、`UiSkeleton`、`UiDialog` / `UiDrawer`（包 el-dialog/el-drawer，统一头尾）、`UiSegmented`（分段切换，滑块弹簧动画）、`UiTag`、`CommandPalette`（⌘K / Ctrl+K，模糊搜索所有路由 + 常用动作，方向键 + Enter）。
Element Plus 的 input/select/date/switch/message/notification/dropdown/tooltip 必须通过 `src/styles/element.scss` 全局重皮肤到上述 tokens，视觉上与自建组件一致。

## 6. 动效（MOTION 10，统一在 `src/motion/`）
- 基础：GSAP（已安装）+ CSS transitions。缓动：`--ease-out: cubic-bezier(.16,1,.3,1)`，弹簧用 GSAP `elastic.out(1,.6)` 或 `back.out(1.6)`；时长 180 / 320 / 560ms 三档。
- 必做：
  1. 路由过渡：内容区 clip/translate+opacity 交错入场（标题先、内容后，stagger 40ms），离场 140ms。
  2. 列表/表格首屏行 stagger 入场；筛选结果变化用 `<TransitionGroup>` FLIP。
  3. `UiStat` 数字从 0 滚动到值（mono）；延迟数值变化时颜色随阈值过渡。
  4. dock 图标弹簧放大 + 选中指示条滑动（FLIP）。
  5. 主按钮 `v-magnetic` 磁吸（指针 24px 内吸附，离开弹回）。
  6. 状态脉冲：在线点呼吸、测速/检测进行中扫描线或进度波纹。
  7. 仪表盘地图/地球：节点位置点 + 连线描线动画（基于现有 `getNodeMap` 真实数据，无数据则空态）。
  8. 骨架屏 shimmer；对话框/抽屉弹簧入场。
- 守则：所有动画经 `useMotion()`/`prefersReducedMotion` 守卫，reduced-motion 时直接到终态；组件卸载时 kill tween / 移除监听 / cancel rAF；只动 transform/opacity/clip-path；不要永不停的装饰性无限循环（脉冲仅限真实状态）。

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
