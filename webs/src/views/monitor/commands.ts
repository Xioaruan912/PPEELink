export interface QuickCommand {
  name: string;
  category: string;
  description: string;
  command: string;
  dangerous?: boolean;
  advanced?: boolean;
  variants?: { label: string; command: string }[];
}
// Fixed reviewed entry scripts; software packages and nested downloads remain upstream-managed.
const scriptCommit = "abe49861805c2a7fe837a7fcaa90e73860fdefef";
const scriptHashes: Record<string, string> = {
  "linux/docker/docker.sh":
    "788461e15cda5a7af15484531dcc1e4f172b93d65fe4201dd092d4b8e7ca6d5c",
  "linux/docker/docker-cn.sh":
    "b18ebee9df3c51e7ead7577489c75647cdf86a879161e04e5b78de251ee45cee",
  "linux/cert/certbot.sh":
    "4a18d48b1f1f0237692a49214e00140c1d68a99947edc42d415568e59ee78f81",
  "linux/cert/let_encrypt.sh":
    "ad98f1010ec79223712766a8ebc84d4defda1fd83fc6ee592f3ab7fb463dab30",
  "linux/basic/timeset_Shanghai.sh":
    "b9652521234c8f4ea41291c1c3d86089371519978b3ce261c5d1ea7b9bc67d9c",
  "linux/basic/miniconda.sh":
    "0a6d68e3a80273d6ac8a439eaf040d9e09cebb9f95848887c0392277d2081ff5",
  "linux/app/vaultwarden.sh":
    "50926577476220bed6ef95869200abd4bcfa1faaa85e7ab6df324cd87f41dd9d",
  "linux/app/jellyfin.sh":
    "ce69f293a172449246593add7b78be584f2cfc0be4dfdfdfc14e19087bd661ce",
  "linux/network/realm.sh":
    "efabc5d86380e2e98b84863de287cbd5393046c25ff06a501d84e97b7cfa9911",
  "linux/network/clash.sh":
    "4ee7b88067de1f924f4893561d60cab43246b2fdf17f3c4da72fabcb4af1786b",
  "linux/backup/rclone.sh":
    "dfbbdee3c6d18bff54aae5d09e46f40f23ba5089c0cdba4dc330e39874981e18",
  "linux/backup/backup.sh":
    "f018a3f0ba367de2d58757e51b39c223422957c6de751adfc84a1e347623c377",
};
function script(path: string) {
  const url = `https://raw.githubusercontent.com/Xioaruan912/Xcript/${scriptCommit}/${path}`;
  return `(set -eu; task_script=$(mktemp); trap 'rm -f "$task_script"' EXIT; curl -fL --connect-timeout 15 --max-time 120 '${url}' -o "$task_script"; printf '${scriptHashes[path]}  %s\\n' "$task_script" | sha256sum -c -; bash "$task_script")`;
}
const docker = (command: string) =>
  `if command -v docker >/dev/null 2>&1; then ${command}; else echo '未安装 Docker，请使用扩展工具安装'; fi`;
export const quickCommands: QuickCommand[] = [
  {
    name: "CPU 与内存",
    category: "系统",
    description: "查看负载、内存和交换空间",
    command: "uptime; free -h",
  },
  {
    name: "磁盘空间",
    category: "系统",
    description: "查看各文件系统占用",
    command: "df -hT",
  },
  {
    name: "大目录",
    category: "系统",
    description: "只读扫描 /var、/root、/opt，最多 20 秒",
    command:
      "if command -v timeout >/dev/null 2>&1; then timeout 20s du -xhd1 /var /root /opt 2>/dev/null | sort -h | tail -n 30; else echo '缺少 timeout，请手动选择目录检查'; fi",
  },
  {
    name: "Agent 状态",
    category: "服务",
    description: "检查已安装的 Agent / Monitor 服务",
    command:
      "systemctl --no-pager --full status ppeelink-agent.service ppeelink-monitor.service",
  },
  {
    name: "Agent 最近日志",
    category: "服务",
    description: "读取 Agent 和 Monitor 最近 100 条日志",
    command:
      "journalctl -u ppeelink-agent.service -u ppeelink-monitor.service -n 100 --no-pager",
  },
  {
    name: "Xray 状态与日志",
    category: "服务",
    description: "检查代理服务及最近 100 条日志",
    command:
      "systemctl --no-pager --full status ppeelink-xray.service; journalctl -u ppeelink-xray.service -n 100 --no-pager",
  },
  {
    name: "失败服务",
    category: "服务",
    description: "列出启动或运行失败的服务",
    command: "systemctl --failed --no-pager",
  },
  {
    name: "TCP / UDP 监听端口",
    category: "网络",
    description: "查看端口、进程与监听地址",
    command: "ss -lntup",
  },
  {
    name: "IPv4 / IPv6 连通性",
    category: "网络",
    description: "分别检查外网出口，每项最多 5 秒",
    command:
      "printf 'IPv4: '; curl -4 -fsS --connect-timeout 3 --max-time 5 https://api.ipify.org; printf '\\nIPv6: '; curl -6 -fsS --connect-timeout 3 --max-time 5 https://api64.ipify.org; printf '\\n'",
  },
  {
    name: "DNS 查询",
    category: "网络",
    description: "将 example.com 改为需要排查的域名",
    command: "getent ahosts example.com",
  },
  {
    name: "Docker 容器状态",
    category: "Docker",
    description: "查看容器运行和退出情况",
    command: docker("docker ps -a"),
  },
  {
    name: "Docker 最近日志",
    category: "Docker",
    description: "把 CONTAINER 改为容器名，不持续追踪",
    command: docker("docker logs --tail 100 CONTAINER"),
  },
  {
    name: "Docker 磁盘占用",
    category: "Docker",
    description: "只读查看镜像、容器与数据卷占用",
    command: docker("docker system df"),
  },
  {
    name: "日志占用",
    category: "系统",
    description: "只查看 journald 的空间占用",
    command: "journalctl --disk-usage",
  },
  {
    name: "清理包缓存",
    category: "系统配置",
    description: "仅清理软件包下载缓存，不删除日志或内核",
    advanced: true,
    dangerous: true,
    command:
      "if command -v apt-get >/dev/null 2>&1; then apt-get clean; elif command -v dnf >/dev/null 2>&1; then dnf clean packages; else echo '请使用本机包管理器手动清理缓存'; fi",
  },
  {
    name: "清理旧系统日志",
    category: "系统配置",
    description: "删除 14 天前的 journald 历史日志",
    advanced: true,
    dangerous: true,
    command: "journalctl --vacuum-time=14d",
  },
  {
    name: "安装 Docker",
    category: "扩展工具",
    description: "安装或更新 Docker；可能改动镜像源和服务配置",
    advanced: true,
    dangerous: true,
    command: script("linux/docker/docker.sh"),
    variants: [
      { label: "标准来源", command: script("linux/docker/docker.sh") },
      { label: "国内镜像", command: script("linux/docker/docker-cn.sh") },
    ],
  },
  {
    name: "证书工具",
    category: "系统配置",
    description: "选择证书方式；可能改动 Nginx 和防火墙配置",
    advanced: true,
    dangerous: true,
    command: script("linux/cert/certbot.sh"),
    variants: [
      {
        label: "已有 Nginx / certbot",
        command: script("linux/cert/certbot.sh"),
      },
      {
        label: "Nginx + snap certbot",
        command: script("linux/cert/let_encrypt.sh"),
      },
    ],
  },
  {
    name: "时区设为上海",
    category: "系统配置",
    description: "修改系统时区为 Asia/Shanghai",
    advanced: true,
    command: script("linux/basic/timeset_Shanghai.sh"),
  },
  {
    name: "安装 Miniconda",
    category: "扩展工具",
    description: "安装 Python 环境到 /opt/miniconda",
    advanced: true,
    command: script("linux/basic/miniconda.sh"),
  },
  {
    name: "安装 Vaultwarden",
    category: "扩展工具",
    description: "部署密码管理器，并可能安装 Docker",
    advanced: true,
    command: script("linux/app/vaultwarden.sh"),
  },
  {
    name: "安装 Jellyfin",
    category: "扩展工具",
    description: "部署媒体服务器，并可能安装 Docker",
    advanced: true,
    command: script("linux/app/jellyfin.sh"),
  },
  {
    name: "安装 rclone",
    category: "扩展工具",
    description: "安装云盘工具，可设置挂载服务",
    advanced: true,
    command: script("linux/backup/rclone.sh"),
  },
  {
    name: "Realm 端口转发",
    category: "网络工具安装",
    description: "独立部署 realm，并修改 BBR 配置",
    advanced: true,
    dangerous: true,
    command: script("linux/network/realm.sh"),
  },
  {
    name: "安装 mihomo 内核",
    category: "网络工具安装",
    description: "独立安装内核及 Geo 数据",
    advanced: true,
    command: script("linux/network/clash.sh"),
  },
  {
    name: "检查备份配置位置",
    category: "扩展工具",
    description: "仅检查配置文件是否存在；自动清理远端备份的脚本入口已停用",
    advanced: true,
    command:
      "for task_conf in /etc/xcript/backup.conf /root/backup/backup.conf; do if test -f \"$task_conf\"; then printf '配置存在：%s\\n' \"$task_conf\"; fi; done; printf '备份上传与远端清理策略须人工审阅，不提供直接执行入口\\n'",
  },
];
