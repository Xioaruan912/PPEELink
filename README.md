# PPEELink User Agent (`userAgent` 分支)

极简、零外部依赖、按需触发的用户侧探针客户端。运行在你自己的电脑（Windows / macOS）或家里软路由 / NAS（Linux）上，**只在你在面板点击「本机实测」时才发起一次有界测速，空闲时不向任何代理节点发包**。

## 特性
- **零后台网络滥用**：默认仅与面板保持轻量长轮询等待指令，不定时扫描节点。
- **真实用户视角**：直接从用户主机测试 TCP（VLESS / VMess / Trojan / SS / Mieru）与 UDP QUIC（Hysteria2 / TUIC）的连通性与毫秒延迟。
- **单文件免安装**：纯 Go 标准库编译，无需 root，无需修改任何代理客户端配置。

## 使用方法

```bash
./ppeelink-user-agent -panel https://your-panel.example.com -token <YOUR_TOKEN>
```

仅立即测速一次后退出：

```bash
./ppeelink-user-agent -panel https://your-panel.example.com -token <YOUR_TOKEN> -once
```

## 构建全平台产物

```bash
chmod +x build.sh && ./build.sh
```
