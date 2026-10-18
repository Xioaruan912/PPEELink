import type { Component } from "vue";
import IconAppleLogo from "~icons/ph/apple-logo";
import IconChatCircleDots from "~icons/ph/chat-circle-dots";
import IconFilmStrip from "~icons/ph/film-strip";
import IconGameController from "~icons/ph/game-controller";
import IconGithubLogo from "~icons/ph/github-logo";
import IconGoogleLogo from "~icons/ph/google-logo";
import IconHouseLine from "~icons/ph/house-line";
import IconMagnet from "~icons/ph/magnet";
import IconNetwork from "~icons/ph/network";
import IconProhibit from "~icons/ph/prohibit";
import IconSparkle from "~icons/ph/sparkle";
import IconSpotifyLogo from "~icons/ph/spotify-logo";
import IconTelegramLogo from "~icons/ph/telegram-logo";
import IconTelevisionSimple from "~icons/ph/television-simple";
import IconWindowsLogo from "~icons/ph/windows-logo";
import IconYoutubeLogo from "~icons/ph/youtube-logo";

/**
 * Rule categories for the builder's quick picker. Each category owns a set of
 * blackmatrix7 rule-providers and routes them to a target: the main proxy group,
 * DIRECT, REJECT, or its own dedicated select group.
 */
export type CategoryTarget = "main" | "DIRECT" | "REJECT" | "group";
export interface RuleCategory {
  key: string;
  label: string;
  icon: Component;
  providers: string[];
  target: CategoryTarget;
  /** Dedicated group name when target is "group". */
  group?: string;
}
export const RULE_CATEGORIES: RuleCategory[] = [
  {
    key: "ads",
    label: "广告拦截",
    icon: IconProhibit,
    providers: ["Advertising"],
    target: "REJECT",
  },
  {
    key: "ai",
    label: "AI 服务",
    icon: IconSparkle,
    providers: ["OpenAI", "Claude", "Gemini"],
    target: "group",
    group: "🤖 AI",
  },
  {
    key: "google",
    label: "谷歌服务",
    icon: IconGoogleLogo,
    providers: ["Google"],
    target: "main",
  },
  {
    key: "telegram",
    label: "电报消息",
    icon: IconTelegramLogo,
    providers: ["Telegram"],
    target: "group",
    group: "✈️ Telegram",
  },
  {
    key: "youtube",
    label: "油管视频",
    icon: IconYoutubeLogo,
    providers: ["YouTube"],
    target: "group",
    group: "📺 YouTube",
  },
  {
    key: "streaming",
    label: "国际流媒体",
    icon: IconFilmStrip,
    providers: ["Netflix", "Disney"],
    target: "group",
    group: "🎬 流媒体",
  },
  {
    key: "microsoft",
    label: "微软服务",
    icon: IconWindowsLogo,
    providers: ["Microsoft"],
    target: "main",
  },
  {
    key: "github",
    label: "GitHub",
    icon: IconGithubLogo,
    providers: ["GitHub"],
    target: "main",
  },
  {
    key: "apple",
    label: "苹果服务",
    icon: IconAppleLogo,
    providers: ["Apple"],
    target: "DIRECT",
  },
  {
    key: "social",
    label: "社交媒体",
    icon: IconChatCircleDots,
    providers: ["Twitter", "Instagram"],
    target: "main",
  },
  {
    key: "games",
    label: "游戏平台",
    icon: IconGameController,
    providers: ["Steam"],
    target: "main",
  },
  {
    key: "spotify",
    label: "Spotify",
    icon: IconSpotifyLogo,
    providers: ["Spotify"],
    target: "main",
  },
  {
    key: "bilibili",
    label: "哔哩哔哩",
    icon: IconTelevisionSimple,
    providers: ["BiliBili"],
    target: "DIRECT",
  },
  {
    key: "pt",
    label: "PT 站点",
    icon: IconMagnet,
    providers: ["PrivateTracker"],
    target: "DIRECT",
  },
  {
    key: "china",
    label: "国内服务",
    icon: IconHouseLine,
    providers: ["China"],
    target: "DIRECT",
  },
  {
    key: "lan",
    label: "私有网络",
    icon: IconNetwork,
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
