<script setup lang="ts">
import { ref, computed, onMounted } from "vue";
import { ElMessage } from "element-plus";
import {
  Download,
  ArrowDown,
  TopRight,
  Search,
  CopyDocument,
  MoreFilled,
} from "@element-plus/icons-vue";
import { PageHeader, UiSegmented, UiButton, EmptyState, UiDrawer } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useMotion, useStagger } from "@/motion";

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

const categories = [
  { value: "all", label: "全部客户端" },
  { value: "desktop", label: "桌面设备" },
  { value: "mobile", label: "移动设备" },
];
const platformGroups = computed(() =>
  [
    {
      key: "desktop",
      label: "桌面与跨平台 / 01",
      clients: filteredClients.value.filter(
        (client) => client.category === "desktop"
      ),
    },
    {
      key: "mobile",
      label: "移动平台 / 02",
      clients: filteredClients.value.filter(
        (client) => client.category === "mobile"
      ),
    },
  ].filter((group) => group.clients.length)
);
const catalog = ref<HTMLElement>();
const detailsClient = ref<ClientCard>();
const detailsOpen = ref(false);
function showDetails(client: ClientCard) {
  detailsClient.value = client;
  detailsOpen.value = true;
}
const { reduced } = useMotion();
const stagger = useStagger();
onMounted(() => {
  if (catalog.value) stagger(catalog.value.querySelectorAll(".client-row"));
});
function clearFilters() {
  searchQuery.value = "";
  activeCategory.value = "all";
}
</script>
<template>
  <div class="client-library" :class="{ 'client-library--reduced': reduced }">
    <PageHeader title="客户端" description="按设备选择，前往官方来源获取。" />
    <div class="library-toolbar">
      <UiSegmented
        v-model="activeCategory"
        :options="categories"
        label="客户端分类"
      />
      <el-input
        v-model="searchQuery"
        placeholder="搜索客户端、协议或平台"
        clearable
        aria-label="搜索客户端、协议或平台"
      >
        <template #prefix><Search aria-hidden="true" /></template>
      </el-input>
    </div>
    <div class="library-summary" role="status" aria-live="polite">
      <span
        >找到 <b>{{ filteredClients.length }}</b> 个客户端</span
      >
      <span>下载与价格以官方发布为准</span>
    </div>
    <div ref="catalog" class="library-catalog">
      <section
        v-for="group in platformGroups"
        :key="group.key"
        class="platform-group"
        :aria-label="group.label.split(' / ')[0]"
      >
        <p class="platform-eyebrow">
          {{ group.label }}<span>{{ group.clients.length }} 个客户端</span>
        </p>
        <TransitionGroup
          name="catalog"
          tag="div"
          class="library-list"
          :css="!reduced"
        >
          <article
            v-for="item in group.clients"
            :key="item.id"
            class="client-row"
          >
            <div class="client-identity">
              <h2>{{ item.name }}</h2>
              <div class="client-platforms">
                <span v-for="platform in item.platforms" :key="platform">{{
                  platform
                }}</span>
              </div>
            </div>
            <p class="client-description">{{ item.description }}</p>
            <div class="client-actions">
              <details class="client-downloads">
                <summary>
                  <Download aria-hidden="true" /><span>{{
                    item.price ? "查看商店" : "下载客户端"
                  }}</span
                  ><ArrowDown aria-hidden="true" />
                </summary>
                <div class="client-download-links">
                  <a
                    v-for="download in item.downloads"
                    :key="download.label"
                    :href="download.url"
                    target="_blank"
                    rel="noopener noreferrer"
                    ><span
                      >{{ download.label
                      }}<small>{{ download.arch }}</small></span
                    ><TopRight aria-hidden="true"
                  /></a>
                  <a
                    :href="item.releaseUrl"
                    target="_blank"
                    rel="noopener noreferrer"
                    >查看官方发布页<TopRight aria-hidden="true"
                  /></a>
                </div>
              </details>
              <RowIconButton
                class="client-more"
                label="技术详情与项目链接"
                :icon="MoreFilled"
                @click="showDetails(item)"
              />
            </div>
          </article>
        </TransitionGroup>
      </section>
    </div>
    <UiDrawer
      v-model="detailsOpen"
      :title="detailsClient?.name || '技术详情'"
      size="min(560px, 100vw)"
    >
      <template v-if="detailsClient">
        <p class="detail-description">{{ detailsClient.description }}</p>
        <div class="client-specifications">
          <dl>
            <div>
              <dt>内核</dt>
              <dd>{{ detailsClient.kernel }}</dd>
            </div>
            <div>
              <dt>协议</dt>
              <dd>{{ detailsClient.protocols.join(" / ") }}</dd>
            </div>
            <div v-if="detailsClient.badge">
              <dt>特点</dt>
              <dd>{{ detailsClient.badge }}</dd>
            </div>
            <div v-if="detailsClient.price">
              <dt>付费说明</dt>
              <dd>
                {{ detailsClient.price }} · {{ detailsClient.region }} ·
                价格以商店为准
              </dd>
            </div>
          </dl>
          <div class="client-source">
            <a
              :href="detailsClient.homepage"
              target="_blank"
              rel="noopener noreferrer"
              >访问官方网站<TopRight aria-hidden="true" /></a
            ><UiButton
              variant="ghost"
              :icon="CopyDocument"
              @click="copyUrl(detailsClient.homepage)"
              >复制项目链接</UiButton
            >
          </div>
        </div>
      </template>
    </UiDrawer>
    <EmptyState
      v-if="!filteredClients.length"
      title="没有匹配客户端"
      description="试试设备名称、内核或协议。"
      ><template #action
        ><UiButton @click="clearFilters">清除筛选</UiButton></template
      ></EmptyState
    >
  </div>
</template>
<style scoped>
.client-library {
  min-width: 0;
}
.library-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
}
.library-toolbar > .el-input {
  width: min(360px, 100%);
}
.library-toolbar :deep(svg) {
  width: 18px;
  height: 18px;
}
.library-summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin: 24px 0 48px;
  color: var(--c-muted);
  font-size: 14px;
}
.library-summary b {
  color: var(--c-ink);
  font-family: var(--font-mono);
  font-weight: 400;
}
.platform-group + .platform-group {
  margin-top: 56px;
}
.platform-eyebrow {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin: 0 0 16px;
  color: var(--c-muted);
  font: 400 13px/1.4 var(--font-mono);
  letter-spacing: 0.08em;
}
.platform-eyebrow > span {
  letter-spacing: 0;
  font: 400 14px/1.4 var(--font-ui);
}
.library-list {
  position: relative;
  border-top: 1px solid var(--c-line);
}
.client-row {
  display: grid;
  grid-template-columns: minmax(220px, 3fr) minmax(0, 5fr) auto;
  align-items: center;
  column-gap: 32px;
  padding: 24px 0;
  border-bottom: 1px solid var(--c-line);
  transition: background var(--duration-fast);
}
.client-row:hover {
  background: var(--c-surface-2);
}
.client-identity {
  min-width: 0;
}
.client-identity h2 {
  margin: 0 0 8px;
  font: 600 18px/1.4 var(--font-ui);
  overflow-wrap: anywhere;
}
.client-platforms {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.client-platforms > span {
  padding: 4px 8px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 14px;
  line-height: 1.4;
  color: var(--c-muted);
}
.client-description {
  margin: 0;
  font-size: 16px;
  line-height: 1.6;
  color: var(--c-muted);
}
.client-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}
.client-downloads {
  position: relative;
}
.detail-description {
  color: var(--c-muted);
  font-size: 16px;
  line-height: 1.6;
  margin: 0 0 24px;
}
summary {
  min-height: 44px;
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  list-style: none;
  font-size: 15px;
}
summary::-webkit-details-marker {
  display: none;
}
summary svg,
.client-source svg,
.client-download-links svg {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}
summary > svg:last-child {
  transition: transform var(--duration-medium) var(--ease-out);
}
details[open] > summary > svg:last-child {
  transform: rotate(180deg);
}
.client-downloads > summary {
  padding: 8px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface);
}
.client-download-links {
  position: absolute;
  z-index: 2;
  top: calc(100% + 8px);
  right: 0;
  width: 320px;
  max-width: calc(100vw - 32px);
  padding: 8px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-overlay);
}
.client-download-links a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 44px;
  padding: 8px 0;
  color: var(--c-data);
  font-size: 15px;
  border-bottom: 1px solid var(--c-line);
  overflow-wrap: anywhere;
}
.client-download-links a:last-child {
  border: 0;
}
.client-download-links small {
  display: block;
  color: var(--c-muted);
  font: 400 14px/1.4 var(--font-mono);
}
.client-specifications {
  padding: 16px 24px;
  background: var(--c-surface);
  border-radius: var(--radius-panel);
}
dl {
  margin: 0;
}
dl > div {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr);
  gap: 16px;
  padding: 8px 0;
  font-size: 14px;
}
dt {
  color: var(--c-muted);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.client-source {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 16px;
  margin-top: 16px;
  border-top: 1px solid var(--c-line);
  padding-top: 8px;
}
.client-source > a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 44px;
  color: var(--c-data);
  font-size: 15px;
}
summary:focus-visible,
a:focus-visible {
  outline: 2px solid var(--c-signal);
  outline-offset: 2px;
}
.catalog-enter-active,
.catalog-leave-active,
.catalog-move {
  transition:
    opacity 320ms var(--ease-out),
    transform 560ms var(--ease-out);
}
.catalog-enter-from,
.catalog-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
.catalog-leave-active {
  position: absolute;
  width: 100%;
  pointer-events: none;
}
.client-library--reduced *,
.client-library--reduced *::before,
.client-library--reduced *::after {
  transition: none !important;
  animation: none !important;
}
@media (max-width: 1023px) {
  .client-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 16px 24px;
  }
  .client-description {
    grid-column: 1;
  }
  .client-actions {
    grid-column: 2;
    grid-row: 1 / 3;
  }
}
@media (max-width: 767px) {
  .library-toolbar {
    align-items: stretch;
    flex-direction: column;
    gap: 16px;
  }
  .library-toolbar > .el-input {
    width: 100%;
  }
  .library-summary {
    margin-bottom: 32px;
    flex-wrap: wrap;
    gap: 8px;
  }
  .client-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .client-description,
  .client-actions {
    grid-column: 1;
    grid-row: auto;
  }
  .client-specifications {
    padding: 16px;
  }
  dl > div {
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 8px;
  }
  .client-download-links {
    right: auto;
    left: 0;
  }
  .platform-group + .platform-group {
    margin-top: 40px;
  }
}
@media (prefers-reduced-motion: reduce) {
  *,
  *::before,
  *::after {
    transition: none !important;
    animation: none !important;
  }
}
</style>
