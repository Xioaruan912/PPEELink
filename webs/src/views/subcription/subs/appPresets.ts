export type AppPresetCategory =
  | "ai"
  | "im"
  | "direct"
  | "download"
  | "browser"
  | "game";

export interface AppPreset {
  id: string;
  label: string;
  category: AppPresetCategory;
  processes: string[];
  packages?: string[];
  defaultTarget: "PROXY" | "DIRECT";
  /** 若当前 Clash 模板存在同名策略组，自动优先绑定该策略组 */
  preferredGroups?: string[];
}

export const APP_PRESET_CATEGORIES: {
  key: AppPresetCategory;
  label: string;
}[] = [
  { key: "ai", label: "AI 与编程" },
  { key: "im", label: "海外通讯" },
  { key: "direct", label: "国内直连" },
  { key: "download", label: "网盘下载" },
  { key: "browser", label: "浏览器" },
  { key: "game", label: "游戏平台" },
];

export const APP_PRESETS: AppPreset[] = [
  // AI 与编程
  {
    id: "chatgpt",
    label: "ChatGPT / Codex",
    category: "ai",
    processes: ["ChatGPT.exe", "ChatGPT", "codex.exe", "codex"],
    packages: ["com.openai.chatgpt"],
    defaultTarget: "PROXY",
    preferredGroups: ["ChatGPT", "OpenAI"],
  },
  {
    id: "claude",
    label: "Claude / Claude Code",
    category: "ai",
    processes: ["Claude.exe", "Claude", "claude.exe", "claude"],
    packages: ["com.anthropic.claude"],
    defaultTarget: "PROXY",
    preferredGroups: ["Claude", "Anthropic", "ChatGPT"],
  },
  {
    id: "antigravity",
    label: "Antigravity",
    category: "ai",
    processes: ["Antigravity.exe", "Antigravity"],
    defaultTarget: "PROXY",
    preferredGroups: ["Gemini", "Google"],
  },
  {
    id: "cursor",
    label: "Cursor",
    category: "ai",
    processes: ["Cursor.exe", "Cursor"],
    defaultTarget: "PROXY",
    preferredGroups: ["ChatGPT", "Claude"],
  },
  {
    id: "vscode",
    label: "VS Code",
    category: "ai",
    processes: ["Code.exe", "Code"],
    defaultTarget: "PROXY",
  },
  {
    id: "windsurf",
    label: "Windsurf",
    category: "ai",
    processes: ["Windsurf.exe", "Windsurf"],
    defaultTarget: "PROXY",
  },
  {
    id: "dev-cli",
    label: "开发 CLI 工具",
    category: "ai",
    processes: [
      "git.exe",
      "git",
      "gh.exe",
      "node.exe",
      "python.exe",
      "go.exe",
      "curl.exe",
    ],
    defaultTarget: "PROXY",
  },

  // 海外通讯
  {
    id: "telegram",
    label: "Telegram",
    category: "im",
    processes: ["Telegram.exe", "Telegram"],
    packages: ["org.telegram.messenger", "org.telegram.messenger.web"],
    defaultTarget: "PROXY",
    preferredGroups: ["日常使用"],
  },
  {
    id: "discord",
    label: "Discord",
    category: "im",
    processes: ["Discord.exe", "Discord"],
    packages: ["com.discord"],
    defaultTarget: "PROXY",
    preferredGroups: ["日常使用"],
  },
  {
    id: "whatsapp",
    label: "WhatsApp",
    category: "im",
    processes: ["WhatsApp.exe", "WhatsApp"],
    packages: ["com.whatsapp"],
    defaultTarget: "PROXY",
    preferredGroups: ["ChatGPT", "日常使用"],
  },
  {
    id: "x-twitter",
    label: "X / Twitter",
    category: "im",
    processes: [],
    packages: ["com.twitter.android"],
    defaultTarget: "PROXY",
    preferredGroups: ["日常使用"],
  },

  // 国内直连（TUN 模式下强烈建议直连，避免延迟/掉线/风控/消耗流量）
  {
    id: "wechat",
    label: "微信",
    category: "direct",
    processes: ["WeChat.exe", "Weixin.exe", "WeChatAppEx.exe", "WeChat"],
    packages: ["com.tencent.mm"],
    defaultTarget: "DIRECT",
  },
  {
    id: "qq",
    label: "QQ / TIM",
    category: "direct",
    processes: ["QQ.exe", "TIM.exe", "QQ"],
    packages: ["com.tencent.mobileqq", "com.tencent.tim"],
    defaultTarget: "DIRECT",
  },
  {
    id: "work-im",
    label: "企微 / 钉钉 / 飞书",
    category: "direct",
    processes: ["WXWork.exe", "DingTalk.exe", "Feishu.exe", "Lark.exe"],
    packages: [
      "com.tencent.wework",
      "com.alibaba.android.rimet",
      "com.ss.android.lark",
    ],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-meeting",
    label: "腾讯会议",
    category: "direct",
    processes: ["wemeetapp.exe", "WeMeet.exe", "TencentMeeting"],
    packages: ["com.tencent.wemeet.app"],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-remote",
    label: "远程桌面（ToDesk / 向日葵 / RustDesk）",
    category: "direct",
    processes: [
      "ToDesk.exe",
      "ToDesk_Service.exe",
      "SunloginClient.exe",
      "AweSun.exe",
      "rustdesk.exe",
    ],
    packages: ["com.todesk.android", "com.oray.sunlogin"],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-video",
    label: "国内影音（B站 / 抖音 / 爱优腾）",
    category: "direct",
    processes: [
      "bilibili.exe",
      "douyin.exe",
      "QQLive.exe",
      "QyClient.exe",
      "YoukuDesktop.exe",
    ],
    packages: [
      "tv.danmaku.bili",
      "com.ss.android.ugc.aweme",
      "com.tencent.qqlive",
      "com.qiyi.video",
    ],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-music",
    label: "国内音乐（网易云 / QQ / 酷狗）",
    category: "direct",
    processes: ["cloudmusic.exe", "QQMusic.exe", "KuGou.exe"],
    packages: [
      "com.netease.cloudmusic",
      "com.tencent.qqmusic",
      "com.kugou.android",
    ],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-games",
    label: "国内游戏与加速器（WeGame / 原神 / UU）",
    category: "direct",
    processes: [
      "wegame.exe",
      "tgp_daemon.exe",
      "CrossProxy.exe",
      "HYP.exe",
      "YuanShen.exe",
      "StarRail.exe",
      "PerfectWorldArena.exe",
      "uu.exe",
      "UU_Launcher.exe",
    ],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-office",
    label: "国内办公与剪辑（WPS / 剪映）",
    category: "direct",
    processes: [
      "wps.exe",
      "et.exe",
      "wpp.exe",
      "wpscloudsvr.exe",
      "JianyingPro.exe",
    ],
    packages: ["cn.wps.moffice_eng"],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-finance",
    label: "电商与支付（千牛 / 旺旺 / 支付宝）",
    category: "direct",
    processes: [
      "Alipay.exe",
      "AliPaySafe.exe",
      "AliWangWang.exe",
      "AliWorkbench.exe",
    ],
    packages: ["eg.android.AlipayGphone", "com.taobao.taobao"],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-nas",
    label: "局域网 / NAS（群晖 / 极空间 / 绿联 / LocalSend）",
    category: "direct",
    processes: [
      "SynologyDrive.exe",
      "cloud-drive-daemon.exe",
      "ZSpace.exe",
      "UGREEN NAS.exe",
      "LocalSend.exe",
      "localsend_app",
    ],
    packages: ["com.synology.DSfile", "org.localsend.localsend_app"],
    defaultTarget: "DIRECT",
  },

  // 网盘与下载
  {
    id: "baidu-pan",
    label: "百度网盘",
    category: "download",
    processes: ["baidunetdisk.exe", "baidunetdiskhost.exe", "BaiduNetdisk_mac"],
    packages: ["com.baidu.netdisk"],
    defaultTarget: "DIRECT",
  },
  {
    id: "ali-quark-pan",
    label: "阿里云盘 / 夸克网盘",
    category: "download",
    processes: ["aDrive.exe", "QuarkCloudDrive.exe"],
    packages: ["com.alicloud.databox", "com.quark.browser"],
    defaultTarget: "DIRECT",
  },
  {
    id: "pan-115",
    label: "115 网盘",
    category: "download",
    processes: ["115Chrome.exe", "115Browser.exe", "115PC.exe"],
    packages: ["com.ylmf.androidclient"],
    defaultTarget: "DIRECT",
  },
  {
    id: "thunder",
    label: "迅雷",
    category: "download",
    processes: ["Thunder.exe", "DownloadSDKServer.exe", "ThunderStart.exe"],
    defaultTarget: "DIRECT",
  },
  {
    id: "cn-downloader",
    label: "下载器（IDM / Motrix / NDM）",
    category: "download",
    processes: ["IDMan.exe", "Motrix.exe", "NeatDM.exe", "FDM.exe"],
    defaultTarget: "DIRECT",
  },
  {
    id: "bt-pt",
    label: "BT / PT（qBittorrent / Aria2 / BitComet）",
    category: "download",
    processes: [
      "qbittorrent.exe",
      "qbittorrent",
      "aria2c.exe",
      "aria2c",
      "BitComet_x64.exe",
      "Transmission.exe",
    ],
    defaultTarget: "DIRECT",
  },

  // 浏览器
  {
    id: "chrome",
    label: "Chrome",
    category: "browser",
    processes: ["chrome.exe", "Google Chrome"],
    packages: ["com.android.chrome"],
    defaultTarget: "PROXY",
  },
  {
    id: "edge",
    label: "Edge",
    category: "browser",
    processes: ["msedge.exe", "Microsoft Edge"],
    packages: ["com.microsoft.emmx"],
    defaultTarget: "PROXY",
  },
  {
    id: "firefox",
    label: "Firefox",
    category: "browser",
    processes: ["firefox.exe", "firefox"],
    packages: ["org.mozilla.firefox"],
    defaultTarget: "PROXY",
  },
  {
    id: "arc-brave",
    label: "Arc / Brave",
    category: "browser",
    processes: ["Arc.exe", "Arc", "brave.exe", "Brave Browser"],
    packages: ["com.brave.browser"],
    defaultTarget: "PROXY",
  },

  // 游戏平台
  {
    id: "steam",
    label: "Steam",
    category: "game",
    processes: ["steam.exe", "steamwebhelper.exe", "steam_osx"],
    packages: ["com.valvesoftware.android.steam.community"],
    defaultTarget: "PROXY",
  },
  {
    id: "xbox",
    label: "Xbox / 微软商店",
    category: "game",
    processes: ["XboxApp.exe", "XboxPcApp.exe", "GamingServices.exe"],
    defaultTarget: "PROXY",
  },
  {
    id: "epic",
    label: "Epic Games",
    category: "game",
    processes: ["EpicGamesLauncher.exe"],
    defaultTarget: "PROXY",
  },
  {
    id: "battlenet",
    label: "Battle.net 战网",
    category: "game",
    processes: ["Battle.net.exe", "Agent.exe"],
    defaultTarget: "PROXY",
  },
];

/** 从 Clash YAML 模板文本中轻量提取 proxy-groups 的名称列表。 */
export function extractClashProxyGroups(yamlText?: string): string[] {
  if (!yamlText) return [];
  const lines = yamlText.split(/\r?\n/);
  const out: string[] = [];
  let inGroups = false;
  let groupIndent = -1;
  for (const raw of lines) {
    const trimmed = raw.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const indent = raw.length - raw.trimStart().length;
    if (!inGroups) {
      if (indent === 0 && trimmed === "proxy-groups:") {
        inGroups = true;
      }
      continue;
    }
    if (indent === 0) break;
    if (groupIndent < 0) groupIndent = indent;
    const match = trimmed.match(/^(?:-\s*)?name:\s*["']?([^"'#]+?)["']?\s*$/);
    if (match && match[1]) {
      const name = match[1].trim();
      if (name && !out.includes(name)) out.push(name);
    }
  }
  return out;
}
