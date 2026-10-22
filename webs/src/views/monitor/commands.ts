export interface QuickCommand {
  name: string;
  category: string;
  description: string;
  command: string;
  dangerous?: boolean;
}

const base = "https://raw.githubusercontent.com/Xioaruan912/Xcript/main";

/** 常用 VPS 脚本，全部来自 Xioaruan912/Xcript。交互式脚本在 WebSSH 里直接运行。 */
export const quickCommands: QuickCommand[] = [
  {
    name: "清理缓存与日志",
    category: "基础",
    description: "清理 apt 缓存、日志和旧内核",
    command: `bash <(curl -sSL ${base}/linux/basic/clean.sh)`,
    dangerous: true,
  },
  {
    name: "时区设为上海",
    category: "基础",
    description: "把系统时区改为 Asia/Shanghai",
    command: `bash <(curl -sSL ${base}/linux/basic/timeset_Shanghai.sh)`,
  },
  {
    name: "安装 Miniconda",
    category: "基础",
    description: "默认安装到 /opt/miniconda",
    command: `bash <(curl -sSL ${base}/linux/basic/miniconda.sh)`,
  },
  {
    name: "安装 / 更新 Docker",
    category: "Docker",
    description: "Docker + Compose，自动换源",
    command: `bash <(curl -sSL ${base}/linux/docker/docker.sh)`,
  },
  {
    name: "Docker 国内镜像版",
    category: "Docker",
    description: "使用国内镜像安装 Docker + Compose",
    command: `bash <(curl -sSL ${base}/linux/docker/docker-cn.sh)`,
  },
  {
    name: "重建当前 Compose",
    category: "Docker",
    description: "强制重建当前目录的 compose 项目",
    command: `bash <(curl -sSL ${base}/linux/docker/docker_rebuild.sh)`,
    dangerous: true,
  },
  {
    name: "Let's Encrypt 证书",
    category: "证书",
    description: "Nginx + certbot 自动申请证书",
    command: `bash <(curl -sSL ${base}/linux/cert/certbot.sh)`,
  },
  {
    name: "证书 + 续期自检",
    category: "证书",
    description: "snap 版 Let's Encrypt 与续期检查",
    command: `bash <(curl -sSL ${base}/linux/cert/let_encrypt.sh)`,
  },
  {
    name: "安装 Vaultwarden",
    category: "应用",
    description: "密码管理器，含 Docker",
    command: `bash <(curl -sSL ${base}/linux/app/vaultwarden.sh)`,
  },
  {
    name: "安装 Jellyfin",
    category: "应用",
    description: "媒体服务器，含 Docker",
    command: `bash <(curl -sSL ${base}/linux/app/jellyfin.sh)`,
  },
  {
    name: "Realm 端口转发",
    category: "网络",
    description: "realm 转发并开启 BBR",
    command: `bash <(curl -sSL ${base}/linux/network/realm.sh)`,
  },
  {
    name: "安装 mihomo 内核",
    category: "网络",
    description: "Clash.Meta / mihomo 与 Geo 数据",
    command: `bash <(curl -sSL ${base}/linux/network/clash.sh)`,
  },
  {
    name: "安装 rclone",
    category: "备份",
    description: "rclone 安装与 OneDrive 等云盘挂载",
    command: `bash <(curl -sSL ${base}/linux/backup/rclone.sh)`,
  },
  {
    name: "通用自动备份",
    category: "备份",
    description: "打包并上传备份",
    command: `bash <(curl -sSL ${base}/linux/backup/backup.sh)`,
  },
];
