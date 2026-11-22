export type CommandSource = "builtin" | "nodescripts";

export interface QuickCommand {
  name: string;
  category: string;
  description: string;
  command: string;
  source?: CommandSource;
  dangerous?: boolean;
  advanced?: boolean;
  variants?: { label: string; command: string }[];
}

const docker = (command: string) =>
  `if command -v docker >/dev/null 2>&1; then ${command}; else echo '未安装 Docker，请在脚本库中安装'; fi`;

export const quickCommands: QuickCommand[] = [
  // ────────────────────────── 内置诊断（只读安全） ──────────────────────────
  {
    name: "CPU 与内存",
    category: "系统",
    source: "builtin",
    description: "查看负载、内存和交换空间",
    command: "uptime; free -h",
  },
  {
    name: "磁盘空间",
    category: "系统",
    source: "builtin",
    description: "查看各文件系统占用",
    command: "df -hT",
  },
  {
    name: "大目录",
    category: "系统",
    source: "builtin",
    description: "只读扫描 /var、/root、/opt，最多 20 秒",
    command:
      "if command -v timeout >/dev/null 2>&1; then timeout 20s du -xhd1 /var /root /opt 2>/dev/null | sort -h | tail -n 30; else echo '缺少 timeout，请手动选择目录检查'; fi",
  },
  {
    name: "日志占用",
    category: "系统",
    source: "builtin",
    description: "只查看 journald 的空间占用",
    command: "journalctl --disk-usage",
  },
  {
    name: "Agent 状态",
    category: "服务",
    source: "builtin",
    description: "检查已安装的 Agent / Monitor 服务",
    command:
      "systemctl --no-pager --full status ppeelink-agent.service ppeelink-monitor.service",
  },
  {
    name: "Agent 最近日志",
    category: "服务",
    source: "builtin",
    description: "读取 Agent 和 Monitor 最近 100 条日志",
    command:
      "journalctl -u ppeelink-agent.service -u ppeelink-monitor.service -n 100 --no-pager",
  },
  {
    name: "Xray 状态与日志",
    category: "服务",
    source: "builtin",
    description: "检查代理服务及最近 100 条日志",
    command:
      "systemctl --no-pager --full status ppeelink-xray.service; journalctl -u ppeelink-xray.service -n 100 --no-pager",
  },
  {
    name: "失败服务",
    category: "服务",
    source: "builtin",
    description: "列出启动或运行失败的服务",
    command: "systemctl --failed --no-pager",
  },
  {
    name: "TCP / UDP 监听端口",
    category: "网络",
    source: "builtin",
    description: "查看端口、进程与监听地址",
    command: "ss -lntup",
  },
  {
    name: "IPv4 / IPv6 连通性",
    category: "网络",
    source: "builtin",
    description: "分别检查外网出口，每项最多 5 秒",
    command:
      "printf 'IPv4: '; curl -4 -fsS --connect-timeout 3 --max-time 5 https://api.ipify.org; printf '\\nIPv6: '; curl -6 -fsS --connect-timeout 3 --max-time 5 https://api64.ipify.org; printf '\\n'",
  },
  {
    name: "DNS 查询",
    category: "网络",
    source: "builtin",
    description: "将 example.com 改为需要排查的域名",
    command: "getent ahosts example.com",
  },
  {
    name: "Docker 容器状态",
    category: "Docker",
    source: "builtin",
    description: "查看容器运行和退出情况",
    command: docker("docker ps -a"),
  },
  {
    name: "Docker 最近日志",
    category: "Docker",
    source: "builtin",
    description: "把 CONTAINER 改为容器名，不持续追踪",
    command: docker("docker logs --tail 100 CONTAINER"),
  },
  {
    name: "Docker 磁盘占用",
    category: "Docker",
    source: "builtin",
    description: "只读查看镜像、容器与数据卷占用",
    command: docker("docker system df"),
  },
  {
    name: "清理包缓存",
    category: "系统维护",
    source: "builtin",
    description: "仅清理软件包下载缓存，不删除日志或内核",
    advanced: true,
    dangerous: true,
    command:
      "if command -v apt-get >/dev/null 2>&1; then apt-get clean; elif command -v dnf >/dev/null 2>&1; then dnf clean packages; else echo '请使用本机包管理器手动清理缓存'; fi",
  },
  {
    name: "清理旧系统日志",
    category: "系统维护",
    source: "builtin",
    description: "删除 14 天前的 journald 历史日志",
    advanced: true,
    dangerous: true,
    command: "journalctl --vacuum-time=14d",
  },

  // ─────────────────── NodeScripts 精选脚本库（nodescripts.net） ───────────────────
  // 1. 测速与检测
  {
    name: "NodeQuality",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "融合 Yabs + IP 质量 + 网络质量 + 融合怪综合检测",
    command: "bash <(curl -sL https://run.NodeQuality.com)",
  },
  {
    name: "IPQuality / IP 质量检测",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "检测 IP 风险、欺诈评分、原生属性与流媒体解锁",
    command: "bash <(curl -sL IP.Check.Place)",
    variants: [
      { label: "IP 质量检测 (IP.Check.Place)", command: "bash <(curl -sL IP.Check.Place)" },
      { label: "流媒体检测 (Media.Check.Place)", command: "bash <(curl -sL Media.Check.Place)" },
      {
        label: "Docker 隔离运行",
        command:
          "docker run --rm --net=host -it xykt/check -I && docker rmi xykt/check > /dev/null 2>&1",
      },
    ],
  },
  {
    name: "TcpQuality",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "TCP 质量检测脚本，默认检测全国三网运营商节点",
    command:
      "bash <(curl -fsSL https://raw.githubusercontent.com/ibsgss/TcpQuality/main/runTcpQuality.sh)",
    variants: [
      {
        label: "GitHub 源",
        command:
          "bash <(curl -fsSL https://raw.githubusercontent.com/ibsgss/TcpQuality/main/runTcpQuality.sh)",
      },
      {
        label: "镜像源 (tcpquality.ibsgss.uk)",
        command: "bash <(curl -fsSL https://tcpquality.ibsgss.uk/run)",
      },
    ],
  },
  {
    name: "流媒体解锁检测",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "检测 Netflix、Disney+、YouTube 等主流平台解锁",
    command: "bash <(curl -L -s check.unlock.media)",
  },
  {
    name: "Speedtest",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "多地区节点上下行带宽测速",
    command: "bash <(curl -sL bash.icu/speedtest)",
  },
  {
    name: "NAT 信息检测",
    category: "测速与检测",
    source: "nodescripts",
    advanced: true,
    description: "检测机器 NAT 类型、出口与基础网络信息",
    command: "curl -fsSL cesl1493939.bohrium.tech:50004/sysinfo.sh|sh",
  },

  // 2. 性能基准
  {
    name: "VPS 融合怪",
    category: "性能基准",
    source: "nodescripts",
    advanced: true,
    description: "全功能 VPS 综合基准、三网回程与解锁测试",
    command:
      "bash <(wget -qO- --no-check-certificate https://gitlab.com/spiritysdx/za/-/raw/main/ecs.sh)",
  },
  {
    name: "Yabs",
    category: "性能基准",
    source: "nodescripts",
    advanced: true,
    description: "经典 Geekbench、磁盘 IO 与 iperf3 基准测试",
    command: "curl -sL yabs.sh | bash",
  },
  {
    name: "NodeBench",
    category: "性能基准",
    source: "nodescripts",
    advanced: true,
    description: "轻量快速的服务器性能与线路基准测试",
    command:
      "bash <(curl -sL https://raw.githubusercontent.com/LloydAsp/NodeBench/main/NodeBench.sh)",
  },
  {
    name: "bench.sh",
    category: "性能基准",
    source: "nodescripts",
    advanced: true,
    description: "Teddysun 经典硬件信息、磁盘 IO 与全球测速",
    command: "wget -qO- bench.sh | bash",
  },
  {
    name: "LemonBench",
    category: "性能基准",
    source: "nodescripts",
    advanced: true,
    description: "系统硬件、网速与路由快速综合评测",
    command:
      "wget -qO- https://raw.githubusercontent.com/LemonBench/LemonBench/main/LemonBench.sh | bash -s -- --fast",
  },

  // 3. 网络与转发
  {
    name: "锐速 / BBR / BBR3",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Linux 常用网络拥塞控制与内核加速合集",
    command:
      'wget -O tcpx.sh "https://github.com/ylx2016/Linux-NetSpeed/raw/master/tcpx.sh" && chmod +x tcpx.sh && ./tcpx.sh',
  },
  {
    name: "BBR + FQ + Swap",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "一键开启 BBR+FQ 拥塞控制与 Swap 内存优化",
    command:
      "bash <(curl -sL https://raw.githubusercontent.com/Samzzj/vps-3xui-Network-optimization/main/install.sh)",
  },
  {
    name: "WARP",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Cloudflare WARP 双栈 / 解锁配置管理菜单",
    command:
      "wget -N https://gitlab.com/fscarmen/warp/-/raw/main/menu.sh && bash menu.sh",
  },
  {
    name: "Realm-xwPF",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Realm 终端可视化一键中转与端口转发脚本",
    command:
      "wget -qO- https://raw.githubusercontent.com/zywe03/realm-xwPF/main/xwPF.sh | sudo bash -s install",
    variants: [
      {
        label: "GitHub 源",
        command:
          "wget -qO- https://raw.githubusercontent.com/zywe03/realm-xwPF/main/xwPF.sh | sudo bash -s install",
      },
      {
        label: "IPv6 镜像源",
        command:
          "wget -qO- https://v6.gh-proxy.org/https://raw.githubusercontent.com/zywe03/realm-xwPF/main/xwPF.sh | sudo bash -s install",
      },
    ],
  },
  {
    name: "NetShape Manager",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "面向中转机、Emby 反代和高 RTT 链路的 TCP 调优",
    command:
      "curl -fsSL --retry 3 https://raw.githubusercontent.com/bear4f/netshape-manager/main/netshape-manager.sh -o /tmp/netshape-manager.sh && sudo bash /tmp/netshape-manager.sh install",
    variants: [
      {
        label: "安装",
        command:
          "curl -fsSL --retry 3 https://raw.githubusercontent.com/bear4f/netshape-manager/main/netshape-manager.sh -o /tmp/netshape-manager.sh && sudo bash /tmp/netshape-manager.sh install",
      },
      { label: "打开交互菜单", command: "sudo netshape" },
      { label: "卸载", command: "sudo netshape uninstall" },
    ],
  },
  {
    name: "iptables-control",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "交互式管理 iptables 端口转发规则",
    command:
      "curl -L https://raw.githubusercontent.com/exing567/iptables-control/main/ipt.sh -o ipt.sh && chmod +x ipt.sh && sudo bash ipt.sh",
  },
  {
    name: "frps-onekey",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "FRP 服务端一键安装与配置脚本",
    command:
      "wget https://raw.githubusercontent.com/mvscode/frps-onekey/master/install-frps.sh -O ./install-frps.sh && chmod 700 ./install-frps.sh && ./install-frps.sh install",
    variants: [
      {
        label: "GitHub 安装",
        command:
          "wget https://raw.githubusercontent.com/mvscode/frps-onekey/master/install-frps.sh -O ./install-frps.sh && chmod 700 ./install-frps.sh && ./install-frps.sh install",
      },
      {
        label: "Gitee 安装",
        command:
          "wget https://gitee.com/mvscode/frps-onekey/raw/master/install-frps.sh -O ./install-frps.sh && chmod 700 ./install-frps.sh && ./install-frps.sh install",
      },
      { label: "更新", command: "./install-frps.sh update" },
      { label: "卸载", command: "./install-frps.sh uninstall" },
    ],
  },
  {
    name: "ddns-go",
    category: "网络与转发",
    source: "nodescripts",
    advanced: true,
    description: "Docker 一键部署多厂商动态域名解析服务",
    command:
      "docker run -d --name ddns-go --restart=always --net=host -v /opt/ddns-go:/root jeessy/ddns-go",
    variants: [
      {
        label: "Docker 启动",
        command:
          "docker run -d --name ddns-go --restart=always --net=host -v /opt/ddns-go:/root jeessy/ddns-go",
      },
      {
        label: "重置密码 (请修改密码)",
        command:
          "docker exec ddns-go ./ddns-go -resetPassword 123456 && docker restart ddns-go",
      },
    ],
  },

  // 4. 运维与面板
  {
    name: "KEJILION.SH",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    description: "科技 Lion 全功能 Linux 综合运维管理脚本",
    command: "bash <(curl -sL kejilion.sh)",
  },
  {
    name: "安装 Docker",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "官方一键安装 Docker 运行环境",
    command: "bash <(curl -sL 'https://get.docker.com')",
  },
  {
    name: "1Panel",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "现代化 Linux 服务器运维管理面板 v2",
    command:
      'bash -c "$(curl -sSL https://resource.fit2cloud.com/1panel/package/v2/quick_start.sh)"',
  },
  {
    name: "宝塔面板",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "宝塔 Linux 面板官方一键安装脚本",
    command:
      "if [ -f /usr/bin/curl ];then curl -sSO https://download.bt.cn/install/install_panel.sh;else wget -O install_panel.sh https://download.bt.cn/install/install_panel.sh;fi;bash install_panel.sh ed8484bec",
  },
  {
    name: "Caddy",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "自动签发 HTTPS 的轻量 Web 服务器与反代",
    command:
      "bash <(wget -qO- https://raw.githubusercontent.com/fscarmen/tools/main/caddy.sh)",
  },
  {
    name: "Fail2ban",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "SSH 防爆破与自动封禁一键安装",
    command:
      "wget --no-check-certificate https://raw.githubusercontent.com/FunctionClub/Fail2ban/master/fail2ban.sh && bash fail2ban.sh 2>&1 | tee fail2ban.log",
  },
  {
    name: "Taier",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    description: "轻量服务器常用管理脚本",
    command: "bash <(curl -sL res.yserver.ink/taier.sh)",
  },
  {
    name: "JDK 安装",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Linux 一键安装配置 Java JDK 环境",
    command:
      "bash <(curl -fsSL http://cesl1493939.bohrium.tech:50004/JDK/install-jdk.sh)",
  },
  {
    name: "Hermes TG Bot",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Hermes 一键安装并配置自定义模型 Telegram Bot",
    command:
      'temp_file="$(mktemp "${TMPDIR:-/tmp}/nodescripts.XXXXXX")" && (trap \'rm -f "$temp_file"\' EXIT HUP INT TERM; curl -fsSL \'https://nodescripts.net/api/files/c097775c-0818-4c3e-bb1d-1984083e1363/04eeff6031a79e078bd9ec2eb40357ca88cbdd474cd91e86b7719576399f327c\' -o "$temp_file" && bash "$temp_file")',
  },
  {
    name: "一键删除服务器监控",
    category: "运维与面板",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "卸载常见云厂商自带监控组件",
    command:
      "curl -L https://raw.githubusercontent.com/spiritLHLS/one-click-installation-script/main/install_scripts/dlm.sh -o dlm.sh && chmod +x dlm.sh && bash dlm.sh",
  },

  // 5. 系统重装（高风险）
  {
    name: "一键重装 Debian 12 (bin)",
    category: "系统重装",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "使用 bin456789 脚本重装 Debian 12（执行前请修改密码）",
    command:
      "curl -O https://raw.githubusercontent.com/bin456789/reinstall/main/reinstall.sh || wget -O reinstall.sh $_ && bash reinstall.sh debian 12 --username root --password admin@123 && reboot",
  },
  {
    name: "inst 一键重装",
    category: "系统重装",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "Linux 极简一键重装 Debian 系统",
    command: "curl -sSL inst.sh|bash -s - -t debian",
  },
  {
    name: "DD Windows",
    category: "系统重装",
    source: "nodescripts",
    advanced: true,
    dangerous: true,
    description: "一键 DD 重装 Windows 10 中文系统（会清空磁盘）",
    command:
      'bash <(curl -sSL https://raw.githubusercontent.com/leitbogioro/Tools/master/Linux_reinstall/InstallNET.sh) -windows 10 -lang "cn"',
  },
];
