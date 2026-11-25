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

  // 国内直连
  {
    id: "wechat",
    label: "微信",
    category: "direct",
    processes: ["WeChat.exe", "Weixin.exe", "WeChat"],
    packages: ["com.tencent.mm"],
    defaultTarget: "DIRECT",
  },
  {
    id: "qq",
    label: "QQ",
    category: "direct",
    processes: ["QQ.exe", "QQ"],
    packages: ["com.tencent.mobileqq"],
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
    id: "cn-music",
    label: "网易云 / QQ 音乐",
    category: "direct",
    processes: ["cloudmusic.exe", "QQMusic.exe"],
    packages: ["com.netease.cloudmusic", "com.tencent.qqmusic"],
    defaultTarget: "DIRECT",
  },

  // 网盘与下载
  {
    id: "baidu-pan",
    label: "百度网盘",
    category: "download",
    processes: ["baidunetdisk.exe", "BaiduNetdisk_mac"],
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
    processes: ["115Chrome.exe", "115Browser.exe"],
    packages: ["com.ylmf.androidclient"],
    defaultTarget: "DIRECT",
  },
  {
    id: "thunder",
    label: "迅雷",
    category: "download",
    processes: ["Thunder.exe", "DownloadSDKServer.exe"],
    defaultTarget: "DIRECT",
  },
  {
    id: "bt-pt",
    label: "qBittorrent / Aria2",
    category: "download",
    processes: ["qbittorrent.exe", "qbittorrent", "aria2c.exe", "aria2c"],
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
