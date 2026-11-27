# PPEELink User Agent (`userAgent` 分支)

轻量、按需触发的本机用户侧探针客户端（当前版本 **`v1.14`**），支持 Windows、macOS（Intel / Apple Silicon）与 Linux（amd64 / arm64）。纯静默后台常驻，空闲时仅保持轻量长轮询等待指令，不主动扫描节点。

## 核心能力

- **双通道真实网络分离**：
  - **通道 A（物理网卡直连）**：绑定本机真实物理网卡并直连公共 DNS，自动绕过 Clash TUN / Fake-IP，准确获取真实公网 IP、运营商、直连带宽基准与物理去程路由。
  - **通道 B（当前代理出口）**：自动跟随 Clash TUN 或 Windows 系统代理，实测当前代理节点的真实 5 秒下载带宽、衰减比与无 CORS 原生 AI / 流媒体解锁。
- **Clash / Mihomo 联动控制 & 流式实时实测**：支持连接本机外部控制器（默认扫描 `9090 / [::1]:9090 / 9097 / 9091 / 29090`，支持自定义地址与 `Bearer` API 访问密钥并明确回报 `401` 鉴权状态），实现远程切换节点与全节点轮切真测速/解锁（每测完 1 个节点即流式实时回报面板，自动以全局模式隔离并兼容 `proxy-provider` 下 `GLOBAL -> 策略组` 级联切换、直连本机 `mixed-port`、支持 Cloudflare / CacheFly / OVH 三重 CDN 下载测速回退，测完自动恢复原模式与原节点，并回报面板融合至双源解锁矩阵）。
- **代理接管 & DNS & TUN 3 秒一键体检（`net-diag`）**：并行检测本机 Clash 控制器与端口、系统 DNS / Fake-IP 接管状态、国内直连延迟、海外代理出口 IP/地区、以及 UDP/QUIC 连通性，输出明确诊断结论与修复建议。
- **面板远程配置与 URL 热迁移（`set-config`）**：支持从面板一键下发新面板地址（先用令牌预检连通通过后再热切换并写入 `config.json`，无需重启），以及实时调整「上报公网 IP 与运营商」、「允许 Clash 联动」、「探测并发线程数」与 Clash API 密钥。
- **零侵入后台生命周期**：无系统托盘、无弹窗的纯静默后台守护进程，支持开机自启、面板端一键静默升级与无痕卸载（清除二进制、配置与开机自启项）。

## 快速运行

推荐直接在面板「用户探针」页面复制一键安装命令；也可手动启动：

```bash
# 后台安装并设置开机自启
./ppeelink-user-agent -install -panel https://your-panel.example.com -token <YOUR_TOKEN>

# 前台运行
./ppeelink-user-agent -panel https://your-panel.example.com -token <YOUR_TOKEN>

# 仅执行一次测速后退出
./ppeelink-user-agent -panel https://your-panel.example.com -token <YOUR_TOKEN> -once

# 无痕卸载
./ppeelink-user-agent -uninstall
```

## 构建全平台产物

```bash
go test -v ./...
chmod +x build.sh && ./build.sh
```

将生成的 `dist/` 内 5 个平台二进制部署至面板运行目录的 `downloads/user-agent/` 即可支持一键下载与在线静默升级。
