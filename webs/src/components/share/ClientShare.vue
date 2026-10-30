<script setup lang="ts">
import { computed, ref, watch } from "vue";
import QrcodeVue from "qrcode.vue";
import CompatibilityReport from "./CompatibilityReport.vue";
import type { CompatibilityData } from "./compatibility";
import { UiButton } from "@/ui";
import SecretLink from "@/views/subcription/subs/SecretLink.vue";

/** 客户端选择 + 二维码 + 链接：订阅分享抽屉与“我的订阅”共用。 */
const props = defineProps<{
  links: Record<string, string>;
  imports?: Record<string, string>;
  /** 切换订阅时重置为默认客户端 */
  resetKey?: string | number;
  subscriptionId?: number;
  compatibility?: CompatibilityData;
}>();
defineSlots<{
  actions?: (props: { link: string; clientName: string }) => unknown;
}>();

const catalog = [
  {
    key: "clash",
    name: "Clash / Mihomo",
    platform: "Windows · macOS · Android",
    mark: "C",
    color: "#2f6bff",
  },
  {
    key: "shadowrocket",
    name: "Shadowrocket",
    platform: "iOS",
    mark: "S",
    color: "#ef4a5a",
  },
  {
    key: "stash",
    name: "Stash",
    platform: "iOS · macOS",
    mark: "St",
    color: "#7b61ff",
  },
  {
    key: "surge",
    name: "Surge",
    platform: "iOS · macOS",
    mark: "Su",
    color: "#f2a33a",
  },
  { key: "loon", name: "Loon", platform: "iOS", mark: "L", color: "#16a07a" },
  {
    key: "qx",
    name: "Quantumult X",
    platform: "iOS",
    mark: "QX",
    color: "#4b7bec",
  },
  { key: "egern", name: "Egern", platform: "iOS", mark: "E", color: "#0ea5a5" },
  {
    key: "singbox",
    name: "sing-box",
    platform: "全平台",
    mark: "sb",
    color: "#3a3f4b",
  },
  {
    key: "surfboard",
    name: "Surfboard",
    platform: "Android",
    mark: "Sf",
    color: "#2a9df4",
  },
  {
    key: "v2ray",
    name: "V2Ray / 通用",
    platform: "Base64 节点列表",
    mark: "V",
    color: "#596070",
  },
];
const selected = ref("clash");
const clients = computed(() => {
  const known = catalog.filter((c) => props.links[c.key]);
  const extra = Object.keys(props.links)
    .filter((key) => !catalog.some((c) => c.key === key))
    .map((key) => ({
      key,
      name: key,
      platform: "订阅直链",
      mark: key.slice(0, 2),
      color: "#596070",
    }));
  return [...known, ...extra];
});
const current = computed(() => {
  const client =
    clients.value.find((c) => c.key === selected.value) || clients.value[0];
  if (!client) return null;
  return {
    ...client,
    link: props.links[client.key],
    importUrl: props.imports?.[client.key],
  };
});
watch(
  () => props.resetKey,
  () => (selected.value = "clash")
);
function openUrl(url: string) {
  window.location.href = url;
}
</script>
<template>
  <div class="share-layout">
    <div class="client-grid" role="radiogroup" aria-label="选择客户端">
      <button
        v-for="client in clients"
        :key="client.key"
        class="client-tile"
        role="radio"
        :aria-checked="selected === client.key"
        @click="selected = client.key"
      >
        <span class="client-mark" :style="{ '--mark': client.color }">{{
          client.mark
        }}</span>
        <span class="client-copy"
          ><strong>{{ client.name }}</strong
          ><small>{{ client.platform }}</small></span
        >
        <span v-if="imports?.[client.key]" class="client-badge">一键导入</span>
      </button>
    </div>
    <Transition name="detail" mode="out-in">
      <section
        v-if="current"
        :key="current.key"
        class="client-detail"
        aria-live="polite"
      >
        <div class="client-qr">
          <QrcodeVue :value="current.link" :size="168" level="M" />
        </div>
        <div class="client-actions">
          <h3>{{ current.name }}</h3>
          <p class="client-hint">
            {{
              current.importUrl
                ? "在装有客户端的设备上点“一键导入”，或用它扫码。"
                : "复制订阅链接，粘贴到客户端的订阅设置中。"
            }}
          </p>
          <UiButton
            v-if="current.importUrl"
            variant="primary"
            @click="openUrl(current.importUrl)"
            >一键导入 {{ current.name }}</UiButton
          >
          <SecretLink :value="current.link" :label="`${current.name} 订阅链接`"
            ><slot
              name="actions"
              :link="current.link"
              :client-name="current.name"
            ></slot
          ></SecretLink>
        </div>
      </section>
    </Transition>
    <CompatibilityReport
      v-if="(subscriptionId || compatibility) && current"
      :subscription-id="subscriptionId"
      :client-key="current.key"
      :preloaded="compatibility"
    />
  </div>
</template>
<style scoped>
.share-layout {
  display: grid;
  gap: 20px;
}
.client-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 8px;
}
.client-tile {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 60px;
  padding: 10px 12px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  text-align: left;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    translate var(--duration-medium) var(--ease-spring),
    transform var(--duration-slow) var(--ease-out);
}
.client-tile:hover {
  border-color: var(--c-muted);
  translate: 0 -1px;
}
.client-tile[aria-checked="true"] {
  border-color: var(--c-focus);
  box-shadow: inset 0 0 0 1px var(--c-focus);
  background: var(--c-selected);
}
.client-mark {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 36px;
  height: 36px;
  border-radius: 9px;
  background: var(--mark);
  color: #fff;
  font: 600 13px var(--font-mono);
}
html.dark .client-mark {
  box-shadow: inset 0 0 0 1px rgb(255 255 255 / 0.14);
}
.client-copy {
  display: grid;
  min-width: 0;
  line-height: 1.3;
}
.client-copy strong {
  font-size: 14px;
  font-weight: 600;
}
.client-copy small {
  color: var(--c-muted);
  font-size: 12px;
}
.client-badge {
  position: absolute;
  top: 6px;
  right: 8px;
  color: var(--c-signal-text);
  font-size: 12px;
}
.client-detail {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 20px;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-canvas);
}
.client-qr {
  padding: 10px;
  border-radius: var(--radius-control);
  background: #fff;
  align-self: start;
}
.client-qr :deep(canvas) {
  display: block;
}
.client-actions {
  display: grid;
  align-content: start;
  justify-items: start;
  gap: 12px;
  min-width: 0;
}
.client-actions > .secret-link {
  width: 100%;
}
.client-hint {
  color: var(--c-muted);
  font-size: 13px;
}
.detail-enter-active,
.detail-leave-active {
  transition:
    opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-medium) var(--ease-out);
}
.detail-enter-from {
  opacity: 0;
  transform: translateY(8px);
}
.detail-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}
.client-actions h3 {
  font-size: 16px;
  font-weight: 600;
}
@media (max-width: 599px) {
  .client-detail {
    grid-template-columns: minmax(0, 1fr);
    justify-items: center;
  }
  .client-actions {
    justify-items: stretch;
  }
}
</style>
