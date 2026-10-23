## 本地开发修复（2026-10-23，待部署）

- 修复 UiButton 未声明 size 参数，导致 UnoCSS 将 size="sm" 渲染为 384px 方形按钮并撑高监控卡片；增加真实小号按钮和可换行的 Agent 操作区。
- 监控网站移除节点、域名、时间、排序和条数筛选，固定最近 24 小时、全部节点、合计排行；每次刷新重新计算窗口，不再额外加载服务器列表。
- 监控列表每 15 秒静默刷新实际回报，页面隐藏时跳过请求，离开或缓存停用时停止刷新。
- Agent 修复重连时连接表锁重入死锁、空闲 WebSocket 无法随取消退出、终端进程未回收及退出码错误；增加连接心跳与写入超时。
- 监控设置和 Agent 升级状态不再触发无关的 Xray 配置重启；缓存运行中程序的摘要，检测采用固定 worker 池，并贯穿 HTTP/TCP 取消。回报校验业务状态并有限重试，永久请求错误不会阻塞下一轮任务。

# PPEELink 二开发布记录

## 未发布（2026/10/23）

- 统一监控与代理 Agent：共用安装模板、运行循环、升级及控制连接；保留旧监控命令和令牌兼容，关闭代理或指标不阻断管理心跳和自身升级。
- 移除网络拓扑页面、导航和专用接口；保留节点状态、Agent 连接、多地路由追踪及共享采集能力。
- 新增仅代理转发的网站流量统计：按服务器节点、请求域名、UTC 小时范围查询上传/下载/合计与排名。需升级 Agent 并在 Web 开启采集，默认关闭，不回填历史。
- HTTPS 不解密，不采集完整网址；IP 目标、不能归属或域名基数超限的流量归为“未知”。统计为成功转发载荷字节，不包含整机网络流量或协议头。
- 网站批次持久化及服务端事务去重，网络失败重试不重复累加；尚未排入批次的实时计数不能保证进程崩溃后恢复，统计不作为精确计费依据。
- Web 启停文案明确区分“配置保存”和“Agent 已执行”；修复停机失败仍提交成功状态及展示虚构 Agent 版本的问题。
- 数据库使用自动迁移新增采集开关、小时统计及批次回执表；升级前请备份数据库。其他未闭环能力及验证边界见仓库 docs/implementation-audit.md。

## 2.1-subL (2026/08/24)
### ✨ feat
- 首页新增节点世界地图（内置 GeoIP，自动识别国家）
- 首页新增节点延迟检测（常见目标 + 每节点 TCP 延迟）
- 新增「解锁测试」页面（sing-box 真实走节点检测 AI/影视/论坛）
- 模板支持 Xboard 风格 filter 正则分组
- 前端整体改为 OpenList 风格（Ant Design 蓝、浅色侧边栏、大圆角）
- 安装脚本改为源码编译（with_utls/with_quic）

# 2.9.2 (2024/03/05)
### ✨ feat
- vscode开发扩展推荐 （author by [cshaptx4869](https://github.com/cshaptx4869)）
- 完善基础增删改查Mock接口 （author by [haoxianrui](https://github.com/haoxianrui)）

### ♻️ refactor
- 修改login密码框功能实现（author by [cshaptx4869](https://github.com/cshaptx4869)）
- 弱化页面进入动画效果（author by [cshaptx4869](https://github.com/cshaptx4869)）
- 取消推荐TypeScript Vue Plugin （author by [cshaptx4869](https://github.com/cshaptx4869)）
- 网站加载动画替换 （author by [haoxianrui](https://github.com/haoxianrui)）
- 优化主题和主题色监听，避免多个页面重复初始化 （author by [haoxianrui](https://github.com/haoxianrui)）

### 🐛 fix
- AppMain 高度在非固定头部不正确导致出现滚动条问题修复 （author by [haoxianrui](https://github.com/haoxianrui)）
- 修复混合模式开启固定Head时的样式问题 （author by [cshaptx4869](https://github.com/cshaptx4869)）
- 设置面板统一字体大小 （author by [cshaptx4869](https://github.com/cshaptx4869)）

### 📦️build
- 通过env配置控制mock服务 （author by [cshaptx4869](https://github.com/cshaptx4869)）
- 升级依赖包至最新版本 （author by [haoxianrui](https://github.com/haoxianrui)）
- 定义vite全局常量替换项目标题和版本 （author by [cshaptx4869](https://github.com/cshaptx4869)）

# 2.9.1 (2024/02/28)
### ♻️ refactor
- 项目配置按钮移入navbar（author by [cshaptx4869](https://github.com/cshaptx4869)）
- 优化user数据定义（author by [cshaptx4869](https://github.com/cshaptx4869)）
- 统一设置栏的 SVG 图标风格 

### 🐛 fix
- 规整一些开发依赖 （author by [cshaptx4869](https://github.com/cshaptx4869)）
- 修复登录页主题切换问题 （author by [cshaptx4869](https://github.com/cshaptx4869)）

### 🚀 pref

- 压缩图片资源  （author by [cshaptx4869](https://github.com/cshaptx4869)）


# 2.9.0 (2024/02/25)

### ✨ feat
- 引入 animate.css 动画库
- 新增水印和配置
- 动态路由菜单支持 element plus 的图标

### ♻️ refactor
- Layout 布局重构和相关问题修复
- sass 使用 @use 替代 @import 引入外部文件指令

### 🐛 fix
- 修复管理页面部分弹窗无法打开问题
- 主题颜色设置按钮 hover 等未变化问题修复


# 2.8.1 (2024/01/10)

### ✨ feat
- 替换 Mock 解决方案 vite-plugin-mock 为 vite-plugin-mock-dev-server 适配 Vite5 

# 2.8.0 (2023/12/27)

### ⬆️ chore
- 升级 Vite4 至 Vite5

# 2.7.1 (2023/12/12)

### ♻️ refactor
- 将打包后的文件进行分类 （author by [ityangzhiwen](https://gitee.com/ityangzhiwen)）

# 2.7.0 (2023/11/19)

### ♻️ refactor
- 代码重构优化
- 修改自动导入组件类型声明文件路径
- 完善 typescript 类型

### 🐛 fix
- 修复管理页面部分弹窗无法打开问题


# 2.7.0 (2023/11/19)

### ♻️ refactor
- 代码重构
- 修改自动导入组件类型声明文件路径
- 完善 typescript 类型

### 🐛 fix
- 修复管理页面部分弹窗无法打开问题


# 2.6.3 (2023/10/22)

### ✨ feat
- 菜单管理新增目录只有一级子路由是否始终显示(alwaysShow)和路由页面是否缓存(keepAlive)的配置
- 接口文档新增 swagger、knife4j
- 引入和支持 tsx 

### ♻️ refactor
- 代码瘦身，整理并删除未使用的 svg
- 控制台样式优化

### 🐛 fix
- 菜单栏折叠和展开的图标暗黑模式显示问题修复


# 2.6.2 (2023/10/11)

### 🐛 fix
- 主题设置未持久化问题
- UnoCSS 插件无智能提示

### ♻️ refactor
- WebSocket 演示样式和代码优化
- 用户管理代码重构

# 2.6.1 (2023/9/4)

### 🐛 fix
- 导航顶部模式、混合模式样式在固定 Header 出现的样式问题修复
- 固定 Header 没有持久化问题修复
- 字典回显兼容 String 和 Number 类型

# 2.6.0 (2023/8/24)💥💥💥

### ✨ feat
- 导航顶部模式、混合模式支持（author by [april-tong](https://april-tong.com/)）
- 平台文档(内嵌)（author by [april-tong](https://april-tong.com/)）

# 2.5.0 (2023/8/8)

### ✨ feat
- 新增 Mock（author by [ygcaicn](https://github.com/ygcaicn)）
- 图标 DEMO（author by [ygcaicn](https://github.com/ygcaicn)）

### 🐛 fix
- 字典支持 Number 类型

# 2.4.1 (2023/7/20)

### ✨ feat
- 整合 vite-plugin-compression 插件打包优化(3.66MB → 1.58MB) （author by [april-tong](https://april-tong.com/)）
- 字典组件封装（author by [haoxr](https://juejin.cn/user/4187394044331261/posts)）

### 🐛 fix
- 分页组件hidden无效 
- 签名无法保存至后端
- Git 提交 stylelint 校验部分机器报错

# 2.4.0 (2023/6/17)

### ✨ feat
- 新增组件标签输入框（author by [april-tong](https://april-tong.com/)）
- 新增组件签名（author by [april-tong](https://april-tong.com/)）
- 新增组件表格（author by [april-tong](https://april-tong.com/)）
- Echarts 图表添加下载功能 author by [april-tong](https://april-tong.com/)）

### ♻️ refactor
- 限制包管理器为 pnpm 和 node 版本16+
- 自定义组件自动导入配置
- 搜索框样式写法优化

### 🐛 fix
- 用户导入的部门回显成数字问题修复

### ⬆️ chore
- element-plus 版本升级 2.3.5 → 2.3.6

# 2.3.1 (2023/5/21)

### 🔄 refactor
- 组件示例文件名称优化

# 2.2.2 (2023/5/11)

### ✨ feat
- 组件封装示例添加源码地址
- 角色、菜单、部门、字段按钮添加权限控制


# 2.3.0 (2023/5/12)

### ⬆️ chore
- vue 版本升级 3.2.45 → 3.3.1 ([CHANGELOG](https://github.com/vuejs/core/blob/main/CHANGELOG.md))
- vite 版本升级 4.3.1 → 4.3.5

### ♻️ refactor
- 使用 vue 3.3 版本新特性 `defineOptions` 在 `setup` 定义组件名称，移除重复的 `script` 标签

# 2.2.2 (2023/5/11)

### ✨ feat
-  用户新增提交添加 `vueUse` 的 `useDebounceFn` 函数实现按钮防抖节流


# 2.2.1 (2023/4/25)

### 🐛 fix
- 图标选择器组件使用 `onClickOutside` 未排除下拉弹出框元素导致无法输入搜索。

