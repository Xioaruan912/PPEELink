# PPEELink Agent

此分支只维护远端 Agent。面板、前端、数据库和管理 API 不在此分支。
Agent 与面板通过既有 HTTP/WebSocket JSON 协议连接，发布与面板分开。

## 构建

需要 Go1.25+；发行清单生成使用 Python3。不需要 Node、pnpm、前端资源或数据库。

```sh
./build.sh
```

`dist/` 输出 Linux amd64、arm64、ARMv7 独立程序及 `release.json`（版本、架构和 SHA256）。
版本在 `agentversion/version.go` 维护，目前1.17。修改 Agent 后按1.17 →1.18递增。
CI 只验证并生成构建产物，不自动创建公开发行版或更新服务器。

## 运行与兼容

```sh
export PPEELINK_AGENT_TOKEN='<面板签发的令牌>'
./dist/agent-linux-amd64 agent -panel https://panel.example.com
./dist/agent-linux-amd64 --version
```

已有 `agent`、`monitor` 和 `probe` 子命令、参数、systemd 服务名及令牌方式保持兼容。
`monitor` 继续走统一 Agent 生命周期。Agent 只下载官方/固定校验的运行工具；
iperf3 首次缺失时自动准备固定3.22静态工具，目标服务器无需编译环境。

## 发布到面板

将同一次构建的完整 `dist/` 内容部署至面板工作目录的 `downloads/agent/`。
先上传三份程序，再原子替换 `release.json`；不要用面板可执行文件冒充 Agent。
面板通过现有下载接口发送对应架构程序，并使用清单 SHA256 校验升级回报。
面板只更换前端/API时无需重新发布 Agent。部署程序的版本和校验值应与清单一致。

## 验证

```sh
go test . ./agentversion ./certpin ./monitorcore ./relaycore ./terminalcore ./throughputcore ./tracecore
go test ./xraycore -run 'TestExecutableHash|TestUpdate|TestStats'
PPEELINK_AGENT_SYSTEMD_TESTS=1 go test . -run 'TestAgentTerminalPersistent|TestAgentTerminalTask'
```

systemd/tmux 与真实网络测量需要隔离测试环境并显式启用。xraycore 中历史路由测试的
既有失败不属于此次结构迁移；不声称完整旧测试集全部通过。
面板跨进程兼容测试使用本分支编译的测试辅助程序，不在 Agent 中引入面板包。

## 许可

Agent 本身见 `LICENSE`；下载的测速工具许可见 `throughputcore/licenses/`，安装时写入
工具旁的 `iperf3.LICENSE`。固定静态发行资产来源、SHA256 和校验规则见
`throughputcore/prepare.go`；该二进制由 userdocs 构建，不是 ESnet 官方二进制发行。
