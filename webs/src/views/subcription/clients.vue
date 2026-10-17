<script setup lang="ts">
import { ref, computed } from "vue";
import { ElMessage } from "element-plus";
import WorkIcon from "@/components/legacy/WorkIcon.vue";

defineOptions({ name: "ClientsPage" });

interface DownloadLink {
  label: string;
  arch: string;
  url: string;
  icon?: string;
}

interface ClientCard {
  id: string;
  name: string;
  icon: string;
  badge?: string;
  badgeType?: "success" | "warning" | "danger" | "info" | "primary";
  category: "desktop" | "mobile" | "core";
  description: string;
  kernel: string;
  protocols: string[];
  platforms: string[];
  homepage: string;
  releaseUrl: string;
  downloads?: DownloadLink[];
  price?: string;
  region?: string;
}

const activeCategory = ref<"all" | "desktop" | "mobile" | "core">("all");
const searchQuery = ref("");

const clients: ClientCard[] = [
  {
    id: "clash-verge-rev",
    name: "Clash Verge Rev",
    icon: "network",
    badge: "推荐 · Mihomo 内核",
    badgeType: "primary",
    category: "desktop",
    description:
      "采用 Mihomo 内核的桌面客户端，支持规则集、TUN 模式与配置管理。",
    kernel: "Mihomo (Clash.Meta)",
    protocols: ["Shadowsocks", "VMess", "VLESS", "Trojan", "Hysteria2", "TUIC"],
    platforms: ["Windows", "macOS", "Linux"],
    homepage: "https://github.com/clash-verge-rev/clash-verge-rev",
    releaseUrl:
      "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest",
    downloads: [
      {
        label: "Windows x64 安装包",
        arch: "win-x64",
        url: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest",
      },
      {
        label: "macOS Apple Silicon (M系列)",
        arch: "mac-arm64",
        url: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest",
      },
      {
        label: "macOS Intel 芯片",
        arch: "mac-x64",
        url: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest",
      },
      {
        label: "Linux AppImage / Deb",
        arch: "linux",
        url: "https://github.com/clash-verge-rev/clash-verge-rev/releases/latest",
      },
    ],
  },
  {
    id: "mihomo-party",
    name: "Mihomo Party",
    icon: "network",
    badge: "高颜值 · 现代",
    badgeType: "success",
    category: "desktop",
    description:
      "采用 Mihomo 内核的桌面客户端，支持图形化策略、配置切换与内核更新。",
    kernel: "Mihomo",
    protocols: [
      "Shadowsocks",
      "VMess",
      "VLESS",
      "Trojan",
      "Hysteria2",
      "WireGuard",
    ],
    platforms: ["Windows", "macOS", "Linux"],
    homepage: "https://github.com/mihomo-party-org/mihomo-party",
    releaseUrl:
      "https://github.com/mihomo-party-org/mihomo-party/releases/latest",
    downloads: [
      {
        label: "Windows 安装程序",
        arch: "win-x64",
        url: "https://github.com/mihomo-party-org/mihomo-party/releases/latest",
      },
      {
        label: "macOS 通用安装包",
        arch: "mac",
        url: "https://github.com/mihomo-party-org/mihomo-party/releases/latest",
      },
      {
        label: "Linux 便携包",
        arch: "linux",
        url: "https://github.com/mihomo-party-org/mihomo-party/releases/latest",
      },
    ],
  },
  {
    id: "v2rayn",
    name: "v2rayN",
    icon: "network",
    badge: "经典全能",
    badgeType: "info",
    category: "desktop",
    description:
      "支持 Xray 与 Sing-box 内核的图形客户端，可管理节点、订阅及路由规则。",
    kernel: "Xray / Sing-box",
    protocols: ["VLESS (Reality)", "VMess", "Trojan", "Shadowsocks", "Socks5"],
    platforms: ["Windows"],
    homepage: "https://github.com/2dust/v2rayN",
    releaseUrl: "https://github.com/2dust/v2rayN/releases/latest",
    downloads: [
      {
        label: "Windows 64位 自包含核心版",
        arch: "win-x64",
        url: "https://github.com/2dust/v2rayN/releases/latest",
      },
      {
        label: "Windows arm64 架构版",
        arch: "win-arm64",
        url: "https://github.com/2dust/v2rayN/releases/latest",
      },
    ],
  },
  {
    id: "sing-box",
    name: "Sing-box",
    icon: "network",
    badge: "极致性能",
    badgeType: "primary",
    category: "desktop",
    description:
      "跨平台网络代理工具，支持多种出站协议与路由配置，可在桌面及移动设备使用。",
    kernel: "Sing-box Core",
    protocols: [
      "Shadowsocks",
      "VMess",
      "VLESS",
      "Trojan",
      "Hysteria",
      "Hysteria2",
      "TUIC",
      "WireGuard",
    ],
    platforms: ["Windows", "macOS", "Linux", "Android", "iOS"],
    homepage: "https://sing-box.sagernet.org",
    releaseUrl: "https://github.com/SagerNet/sing-box/releases/latest",
    downloads: [
      {
        label: "GitHub 官方所有平台发布页",
        arch: "all",
        url: "https://github.com/SagerNet/sing-box/releases/latest",
      },
      {
        label: "Android 客户端 (Google Play)",
        arch: "android",
        url: "https://play.google.com/store/apps/details?id=io.nekohasekai.sfa",
      },
      {
        label: "Apple App Store (iOS / macOS)",
        arch: "apple",
        url: "https://apps.apple.com/us/app/sing-box/id6451272673",
      },
    ],
  },
  {
    id: "flclash",
    name: "FlClash",
    icon: "network",
    badge: "跨平台 · Flutter",
    badgeType: "info",
    category: "desktop",
    description:
      "基于 Flutter 框架开发的跨平台 Clash 代理客户端，支持全系统一致的现代化 Material You 界面体验与极低资源开销。",
    kernel: "Mihomo",
    protocols: ["Shadowsocks", "VMess", "VLESS", "Trojan", "Hysteria2"],
    platforms: ["Windows", "macOS", "Linux", "Android"],
    homepage: "https://github.com/chen08209/FlClash",
    releaseUrl: "https://github.com/chen08209/FlClash/releases/latest",
    downloads: [
      {
        label: "Windows 安装包 (x64/arm64)",
        arch: "win",
        url: "https://github.com/chen08209/FlClash/releases/latest",
      },
      {
        label: "macOS 安装包 (.dmg)",
        arch: "mac",
        url: "https://github.com/chen08209/FlClash/releases/latest",
      },
      {
        label: "Android APK 安装包",
        arch: "android",
        url: "https://github.com/chen08209/FlClash/releases/latest",
      },
    ],
  },
  {
    id: "shadowrocket",
    name: "Shadowrocket (小火箭)",
    icon: "network",
    badge: "iOS 必备",
    badgeType: "warning",
    category: "mobile",
    description:
      "iOS 端经典规则分流代理客户端。配置直观，支持扫码一键导入所有主流节点与订阅格式，支持按域名/IP精细化分流。",
    kernel: "Shadowrocket Native",
    protocols: [
      "Shadowsocks",
      "VMess",
      "VLESS",
      "Trojan",
      "Hysteria2",
      "Snell",
    ],
    platforms: ["iOS", "iPadOS"],
    homepage: "https://apps.apple.com/us/app/shadowrocket/id932747118",
    releaseUrl: "https://apps.apple.com/us/app/shadowrocket/id932747118",
    price: "$2.99",
    region: "美区 / 非国区",
    downloads: [
      {
        label: "前往 App Store (美区)",
        arch: "ios",
        url: "https://apps.apple.com/us/app/shadowrocket/id932747118",
      },
    ],
  },
  {
    id: "loon",
    name: "Loon",
    icon: "network",
    badge: "iOS 旗舰",
    badgeType: "warning",
    category: "mobile",
    description:
      "面向 Apple 设备的分流与诊断工具，支持脚本插件、节点链和规则管理。",
    kernel: "Loon Native",
    protocols: ["Shadowsocks", "VMess", "Trojan", "Hysteria2", "WireGuard"],
    platforms: ["iOS", "macOS"],
    homepage: "https://apps.apple.com/us/app/loon/id1373567447",
    releaseUrl: "https://apps.apple.com/us/app/loon/id1373567447",
    price: "$7.99",
    region: "美区 / 非国区",
    downloads: [
      {
        label: "前往 App Store (美区)",
        arch: "ios",
        url: "https://apps.apple.com/us/app/loon/id1373567447",
      },
    ],
  },
  {
    id: "quantumult-x",
    name: "Quantumult X",
    icon: "network",
    badge: "iOS 性能强悍",
    badgeType: "warning",
    category: "mobile",
    description:
      "iOS 平台极受极客欢迎的网络工具，支持多级策略组嵌套、强大的分流重写、自定义过滤策略与 JavaScript 实时脚本处理。",
    kernel: "Quantumult Native",
    protocols: ["Shadowsocks", "VMess", "Trojan", "HTTP(S)"],
    platforms: ["iOS", "iPadOS"],
    homepage: "https://apps.apple.com/us/app/quantumult-x/id1252015438",
    releaseUrl: "https://apps.apple.com/us/app/quantumult-x/id1252015438",
    price: "$7.99",
    region: "美区 / 非国区",
    downloads: [
      {
        label: "前往 App Store (美区)",
        arch: "ios",
        url: "https://apps.apple.com/us/app/quantumult-x/id1252015438",
      },
    ],
  },
  {
    id: "clash-meta-android",
    name: "Clash Meta for Android",
    icon: "network",
    badge: "Android 推荐",
    badgeType: "success",
    category: "mobile",
    description:
      "专为 Android 打造的 Clash Meta 图形化客户端，基于 Mihomo 核心，完美支持现代协议、分应用代理与全局快捷瓷贴。",
    kernel: "Mihomo",
    protocols: ["Shadowsocks", "VMess", "VLESS", "Trojan", "Hysteria2"],
    platforms: ["Android"],
    homepage: "https://github.com/MetaCubeX/ClashMetaForAndroid",
    releaseUrl:
      "https://github.com/MetaCubeX/ClashMetaForAndroid/releases/latest",
    downloads: [
      {
        label: "GitHub Releases (下载 APK)",
        arch: "android",
        url: "https://github.com/MetaCubeX/ClashMetaForAndroid/releases/latest",
      },
    ],
  },
  {
    id: "v2rayng",
    name: "v2rayNG",
    icon: "network",
    badge: "Android 常用",
    badgeType: "info",
    category: "mobile",
    description:
      "Android 端经典的 Xray / V2Ray 客户端，更新维护频繁，对 VLESS Reality、gRPC、Websocket 具有完善兼容性。",
    kernel: "Xray Core",
    protocols: ["VLESS", "VMess", "Shadowsocks", "Trojan", "Socks5"],
    platforms: ["Android"],
    homepage: "https://github.com/2dust/v2rayNG",
    releaseUrl: "https://github.com/2dust/v2rayNG/releases/latest",
    downloads: [
      {
        label: "GitHub Releases (下载 APK)",
        arch: "android",
        url: "https://github.com/2dust/v2rayNG/releases/latest",
      },
    ],
  },
];

const filteredClients = computed(() => {
  return clients.filter((c) => {
    const matchCategory =
      activeCategory.value === "all" || c.category === activeCategory.value;
    const q = searchQuery.value.trim().toLowerCase();
    const matchSearch =
      !q ||
      c.name.toLowerCase().includes(q) ||
      c.description.toLowerCase().includes(q) ||
      c.kernel.toLowerCase().includes(q) ||
      c.protocols.some((p) => p.toLowerCase().includes(q)) ||
      c.platforms.some((p) => p.toLowerCase().includes(q));
    return matchCategory && matchSearch;
  });
});

const copyUrl = async (url: string) => {
  try {
    await navigator.clipboard.writeText(url);
    ElMessage.success("链接已复制至剪贴板");
  } catch {
    ElMessage.info("请长按复制链接: " + url);
  }
};

const categories: {
  key: "all" | "desktop" | "mobile";
  label: string;
  icon: string;
}[] = [
  { key: "all", label: "全部客户端", icon: "grid" },
  { key: "desktop", label: "桌面设备", icon: "network" },
  { key: "mobile", label: "移动设备", icon: "link" },
];
</script>
<template>
  <div class="client-library">
    <aside class="library-filter">
      <span class="library-filter-label">适用设备</span
      ><button
        v-for="category in categories"
        :key="category.key"
        :class="{ active: activeCategory === category.key }"
        @click="activeCategory = category.key"
      >
        <WorkIcon :name="category.icon" />{{ category.label
        }}<span class="data-number">{{
          category.key === "all"
            ? clients.length
            : clients.filter((c) => c.category === category.key).length
        }}</span>
      </button>
    </aside>
    <section class="library-content">
      <header class="library-toolbar">
        <el-input
          v-model="searchQuery"
          placeholder="搜索客户端、协议或平台…"
          clearable
          aria-label="搜索客户端"
          ><template #prefix><WorkIcon name="search" /></template></el-input
        ><span
          >找到
          <b class="data-number">{{ filteredClients.length }}</b> 个客户端</span
        >
      </header>
      <TransitionGroup name="catalog" tag="div" class="library-list"
        ><article
          v-for="item in filteredClients"
          :key="item.id"
          class="library-client"
        >
          <div class="client-identity">
            <span class="client-monogram">{{
              item.name
                .replace(/[^a-zA-Z]/g, "")
                .slice(0, 2)
                .toUpperCase()
            }}</span>
            <div>
              <h2>{{ item.name }}</h2>
              <div class="client-platforms">
                <span v-for="platform in item.platforms" :key="platform">{{
                  platform
                }}</span>
              </div>
            </div>
            <a
              :href="item.homepage"
              target="_blank"
              rel="noopener noreferrer"
              aria-label="打开客户端官方网站"
              ><WorkIcon name="diagonal"
            /></a>
          </div>
          <p class="client-description">{{ item.description }}</p>
          <details class="client-technical">
            <summary>技术详情</summary>
            <div class="client-specifications">
              <span><small>内核</small>{{ item.kernel }}</span
              ><span><small>协议</small>{{ item.protocols.join(" / ") }}</span>
            </div>
          </details>
          <details class="client-downloads">
            <summary>
              <span
                ><WorkIcon name="diagonal" />{{
                  item.price ? "在 App Store 查看" : "获取客户端"
                }}</span
              ><WorkIcon name="down" />
            </summary>
            <div class="client-download-links">
              <a
                v-for="download in item.downloads"
                :key="download.label"
                :href="download.url"
                target="_blank"
                rel="noopener noreferrer"
                >{{ download.label }}<WorkIcon name="diagonal" /></a
              ><a
                :href="item.releaseUrl"
                target="_blank"
                rel="noopener noreferrer"
                >查看官方发布页<WorkIcon name="diagonal"
              /></a>
            </div>
          </details>
          <div class="client-source">
            <span>{{
              item.price ? "付费应用 · 价格以商店为准" : "从官方来源获取"
            }}</span
            ><button @click="copyUrl(item.homepage)">复制项目链接</button>
          </div>
        </article></TransitionGroup
      >
      <div v-if="!filteredClients.length" class="library-empty">
        <WorkIcon name="search" />
        <h3>没有找到匹配的客户端</h3>
        <p>试试按设备名称或协议搜索。</p>
        <button
          @click="
            searchQuery = '';
            activeCategory = 'all';
          "
        >
          清除筛选
        </button>
      </div>
    </section>
  </div>
</template>
<style scoped>
.client-library {
  display: grid;
  grid-template-columns: 190px minmax(0, 1fr);
  gap: 35px;
  padding-top: 5px;
}
.library-filter {
  padding-top: 10px;
}
.library-filter-label {
  font-size: 14px;
  color: var(--ui-text-muted);
  display: block;
  margin-bottom: 15px;
}
.library-filter > button {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 13px;
  margin-bottom: 5px;
  width: 100%;
  text-align: left;
  background: transparent;
  border-radius: 6px;
  font-size: 15px;
  color: var(--ui-text-secondary);
}
.library-filter > button.active {
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
  font-weight: 600;
}
.library-filter > button > svg {
  height: 15px;
  width: 15px;
}
.library-filter > button > span {
  margin-left: auto;
  font-size: 14px;
  opacity: 0.7;
}
.library-note {
  padding: 25px 12px;
  margin-top: 26px;
  border-top: 1px solid var(--ui-border);
}
.library-note > svg {
  height: 15px;
  width: 15px;
  color: var(--ui-text-muted);
}
.library-note p {
  font-size: 14px;
  line-height: 1.9;
  color: var(--ui-text-muted);
}
.library-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 15px;
  padding-bottom: 18px;
}
.library-toolbar > .el-input {
  max-width: 390px;
}
.library-toolbar:deep(svg) {
  width: 15px;
  height: 15px;
}
.library-toolbar > span {
  font-size: 14px;
  color: var(--ui-text-muted);
  white-space: nowrap;
}
.library-toolbar b {
  font-weight: 400;
  color: var(--ui-text);
}
.library-list {
  display: grid;
  grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
  border-top: 1px solid var(--ui-border);
}
.library-client {
  padding: 25px 25px 20px 0;
  min-width: 0;
  border-bottom: 1px solid var(--ui-border);
  display: flex;
  flex-direction: column;
}
.library-client:nth-child(even) {
  padding-left: 27px;
  padding-right: 0;
  border-left: 1px solid var(--ui-border);
}
.client-identity {
  display: flex;
  align-items: center;
  gap: 12px;
}
.client-monogram {
  height: 39px;
  width: 39px;
  display: grid;
  place-items: center;
  font-weight: 600;
  font-size: 14px;
  letter-spacing: -1px;
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
  border-radius: 10px;
  flex-shrink: 0;
}
.library-client:nth-child(3n) .client-monogram {
  border-radius: 50%;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border-strong);
}
.client-identity h2 {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.5px;
  margin: 0 0 6px;
}
.client-platforms {
  display: flex;
  gap: 7px;
  flex-wrap: wrap;
  color: var(--ui-text-muted);
  font-size: 14px;
}
.client-platforms > span + span:before {
  content: "/";
  margin-right: 7px;
  color: var(--ui-border-strong);
}
.client-identity > a {
  margin-left: auto;
  color: var(--ui-text-muted);
}
.client-identity > a > svg {
  height: 15px;
  width: 15px;
}
.client-description {
  font-size: 15px;
  line-height: 1.9;
  color: var(--ui-text-secondary);
  margin: 20px 0 16px;
  min-height: 62px;
}
.client-specifications {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 14px;
  line-height: 1.8;
  margin-bottom: 20px;
  color: var(--ui-text-secondary);
}
.client-specifications > span {
  display: grid;
  grid-template-columns: 33px 1fr;
}
.client-specifications small {
  font-size: 14px;
  color: var(--ui-text-muted);
}
.client-downloads {
  margin-top: auto;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border);
  border-radius: 6px;
  overflow: hidden;
}
.client-downloads summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  list-style: none;
  cursor: pointer;
  padding: 11px 13px;
  font-size: 14px;
  font-weight: 550;
}
.client-downloads summary::-webkit-details-marker {
  display: none;
}
.client-downloads summary > span {
  display: flex;
  align-items: center;
  gap: 8px;
}
.client-downloads summary svg {
  height: 13px;
  width: 13px;
}
.client-downloads summary > svg {
  height: 11px;
  width: 11px;
  transition: transform 0.25s var(--ease-workspace);
}
.client-downloads[open] summary > svg {
  transform: rotate(180deg);
}
.client-downloads[open] summary {
  background: var(--ui-accent-soft);
}
.client-download-links {
  display: flex;
  flex-direction: column;
  padding: 5px 13px;
}
.client-download-links a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  font-size: 14px;
  gap: 10px;
  border-bottom: 1px solid var(--ui-border-subtle);
}
.client-download-links a:last-child {
  border: 0;
}
.client-download-links svg {
  width: 11px;
  height: 11px;
  flex-shrink: 0;
}
.client-source {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 11px;
  color: var(--ui-text-muted);
  font-size: 14px;
}
.client-source button {
  background: none;
  color: var(--ui-text-secondary);
  font-size: 14px;
}
.library-empty {
  text-align: center;
  padding: 80px 15px;
}
.library-empty > svg {
  height: 28px;
  width: 28px;
  color: var(--ui-text-muted);
}
.library-empty h3 {
  font-size: 15px;
  font-weight: 550;
}
.library-empty p {
  font-size: 15px;
  color: var(--ui-text-muted);
}
.library-empty button {
  font-size: 15px;
  color: var(--ui-accent);
  background: none;
  margin-top: 8px;
}
.catalog-enter-active,
.catalog-leave-active,
.catalog-move {
  transition:
    opacity 0.25s,
    transform 0.45s var(--ease-workspace);
}
.catalog-enter-from,
.catalog-leave-to {
  opacity: 0;
  transform: translateY(15px);
}
.catalog-leave-active {
  display: none;
  position: absolute;
  pointer-events: none;
}
@media (max-width: 1000px) {
  .client-library {
    grid-template-columns: 150px minmax(0, 1fr);
    gap: 25px;
  }
  .library-list {
    grid-template-columns: 1fr;
  }
  .library-client:nth-child(even) {
    padding-left: 0;
    border-left: 0;
  }
  .library-client {
    padding-right: 0;
  }
  .client-description {
    min-height: 0;
  }
}
@media (max-width: 650px) {
  .client-library {
    grid-template-columns: 1fr;
    gap: 0;
    padding-top: 0;
  }
  .library-filter {
    display: flex;
    gap: 5px;
    padding: 0 0 18px;
  }
  .library-filter-label,
  .library-note {
    display: none;
  }
  .library-filter > button {
    width: auto;
    font-size: 14px;
    padding: 9px 10px;
    gap: 7px;
    margin: 0;
  }
  .library-filter > button > span {
    margin-left: 4px;
  }
  .library-filter > button > svg {
    height: 12px;
    width: 12px;
  }
  .library-toolbar {
    gap: 9px;
    padding-bottom: 16px;
  }
  .library-toolbar > span {
    font-size: 14px;
  }
  .library-client {
    padding: 22px 0 18px;
  }
  .client-description {
    font-size: 14px;
  }
  .client-identity h2 {
    font-size: 15px;
  }
  .client-platforms {
    font-size: 14px;
  }
  .client-specifications {
    font-size: 14px;
  }
  .client-monogram {
    width: 36px;
    height: 36px;
  }
  .library-toolbar > .el-input {
    min-width: 0;
  }
}
</style>
