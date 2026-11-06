# 面板与 Agent 独立开发

Agent 的源码、Go module、测试、构建与 CI 位于同一仓库的 `agent` 分支。
该分支不包含面板 API、数据库或前端，独立产物约14MiB。
面板源码仅保留管理接口、协议数据类型与本机代理/路由追踪运行库，不包含远端工作程序入口、tmux运行管理或测速工具安装执行代码。

## Agent 发布

1. 在 `agent` 分支修改代码，按 `agentversion/version.go` 递增发行编号；运行分支 README 中的测试。
2. 执行 Agent 分支的 `./build.sh`，得到三种架构程序和 `dist/release.json`。
3. 将同批产物上传到面板工作目录 `downloads/agent/`。先上传程序，最后原子切换发行清单；也可通过 `PPEELINK_AGENT_RELEASE_DIR` 指向独立发行目录。
4. 在面板请求更新。面板按服务器架构下发独立 Agent；下载和回报使用发行清单 SHA256，与面板本身的文件和构建版本无关。

修改协议字段时需保持两端兼容，并运行跨进程兼容测试。旧的 `agent`、`monitor`、`probe` 子命令和下载 URL 保持兼容。面板未配置发行产物时会明确返回503，不会把面板程序作为后备 Agent。

首次分离使用同一个功能发行编号1.17，接口和任务行为保持一致；文件 SHA256 因去除面板代码而变化。
升级过程中 tmux 独立会话服务不会被替换或结束。

## 面板构建

面板 `build.sh` 只构建前端和面板程序。Agent 独立构建，不需要同步修改或重编译面板。
Docker发布 CI 单独 checkout `agent` 分支，构建 Agent 后把产物复制至 `agent-artifacts/` 作为镜像输入；源码不进入面板构建上下文。手动构建包含 Agent 下载的镜像前，同样需要将 `dist/` 复制到该目录。
镜像默认使用所包含的 Agent 产物；需要独立热发布时，可以显式挂载发行目录。

## 跨进程兼容测试

在 Agent 分支编译测试辅助程序：

```sh
go test -c -o /tmp/ppeelink-agent-integration.test .
```

在面板分支运行：

```sh
PPEELINK_AGENT_TEST_BINARY=/tmp/ppeelink-agent-integration.test go test ./integration
```

此测试以独立子进程运行 Agent 的终端传输代码，验证所有者隔离、会话操作、一次性票据和重连。
需要 root、systemd、tmux，并使用独立测试服务和临时目录，不操作用户现有会话。
