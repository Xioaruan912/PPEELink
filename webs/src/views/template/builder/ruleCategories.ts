/**
 * Rule categories for the builder's quick picker. Each category owns a set of
 * blackmatrix7 rule-providers and routes them to a target: the main proxy group,
 * DIRECT, REJECT, or its own dedicated select group.
 */
export type CategoryTarget = "main" | "DIRECT" | "REJECT" | "group";
export interface RuleCategory {
  key: string;
  label: string;
  emoji: string;
  providers: string[];
  target: CategoryTarget;
  /** Dedicated group name when target is "group". */
  group?: string;
}
export const RULE_CATEGORIES: RuleCategory[] = [
  {
    key: "ads",
    label: "广告拦截",
    emoji: "🛑",
    providers: ["Advertising"],
    target: "REJECT",
  },
  {
    key: "ai",
    label: "AI 服务",
    emoji: "🤖",
    providers: ["OpenAI", "Claude", "Gemini"],
    target: "group",
    group: "🤖 AI",
  },
  {
    key: "google",
    label: "谷歌服务",
    emoji: "🔍",
    providers: ["Google"],
    target: "main",
  },
  {
    key: "telegram",
    label: "电报消息",
    emoji: "✈️",
    providers: ["Telegram"],
    target: "group",
    group: "✈️ Telegram",
  },
  {
    key: "youtube",
    label: "油管视频",
    emoji: "📺",
    providers: ["YouTube"],
    target: "group",
    group: "📺 YouTube",
  },
  {
    key: "streaming",
    label: "国际流媒体",
    emoji: "🎬",
    providers: ["Netflix", "Disney"],
    target: "group",
    group: "🎬 流媒体",
  },
  {
    key: "microsoft",
    label: "微软服务",
    emoji: "🪟",
    providers: ["Microsoft"],
    target: "main",
  },
  {
    key: "github",
    label: "GitHub",
    emoji: "🐙",
    providers: ["GitHub"],
    target: "main",
  },
  {
    key: "apple",
    label: "苹果服务",
    emoji: "🍎",
    providers: ["Apple"],
    target: "DIRECT",
  },
  {
    key: "social",
    label: "社交媒体",
    emoji: "💬",
    providers: ["Twitter", "Instagram"],
    target: "main",
  },
  {
    key: "games",
    label: "游戏平台",
    emoji: "🎮",
    providers: ["Steam"],
    target: "main",
  },
  {
    key: "spotify",
    label: "Spotify",
    emoji: "🎵",
    providers: ["Spotify"],
    target: "main",
  },
  {
    key: "bilibili",
    label: "哔哩哔哩",
    emoji: "📼",
    providers: ["BiliBili"],
    target: "DIRECT",
  },
  {
    key: "pt",
    label: "PT 站点",
    emoji: "🧲",
    providers: ["PrivateTracker"],
    target: "DIRECT",
  },
  {
    key: "china",
    label: "国内服务",
    emoji: "🏠",
    providers: ["China"],
    target: "DIRECT",
  },
  {
    key: "lan",
    label: "私有网络",
    emoji: "🔒",
    providers: ["Lan"],
    target: "DIRECT",
  },
];
export const RULE_PRESETS: { label: string; value: string; keys: string[] }[] =
  [
    {
      label: "精简",
      value: "lite",
      keys: ["ads", "ai", "google", "telegram", "china", "lan"],
    },
    {
      label: "均衡（推荐）",
      value: "balanced",
      keys: [
        "ads",
        "ai",
        "google",
        "telegram",
        "youtube",
        "streaming",
        "microsoft",
        "github",
        "apple",
        "china",
        "lan",
      ],
    },
    { label: "全量", value: "full", keys: RULE_CATEGORIES.map((c) => c.key) },
  ];
export const providerUrl = (name: string) =>
  `https://raw.githubusercontent.com/blackmatrix7/ios_rule_script/master/rule/Clash/${name}/${name}.yaml`;
