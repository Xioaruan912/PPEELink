<script setup lang="ts">
import { computed, ref, watch } from "vue";
import QrcodeVue from "qrcode.vue";
import { UiDialog, UiDrawer, UiButton, UiSkeleton, EmptyState } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
import SecretLink from "./SecretLink.vue";
const {
  qrDialog,
  qrTitle,
  qrcode,
  openUrl,
  importDialog,
  importSub,
  importData,
  importLoading,
  openImport,
  makeShortLink,
  shortLink,
} = useSubscriptions();
// Display metadata for the clients the backend issues links for.
const catalog = [
  {
    key: "clash",
    name: "Clash / Mihomo",
    platform: "Windows · macOS · Android",
    mark: "C",
    color: "#2f5bea",
  },
  {
    key: "shadowrocket",
    name: "Shadowrocket",
    platform: "iOS",
    mark: "S",
    color: "#e4472b",
  },
  {
    key: "surge",
    name: "Surge",
    platform: "iOS · macOS",
    mark: "Su",
    color: "#d99a1e",
  },
  { key: "loon", name: "Loon", platform: "iOS", mark: "L", color: "#1f9d6b" },
  {
    key: "qx",
    name: "Quantumult X",
    platform: "iOS",
    mark: "QX",
    color: "#7a5af5",
  },
  {
    key: "singbox",
    name: "sing-box",
    platform: "全平台",
    mark: "sb",
    color: "#17181a",
  },
  {
    key: "v2ray",
    name: "V2Ray / 通用",
    platform: "Base64 节点列表",
    mark: "V",
    color: "#625f5a",
  },
];
const selected = ref("clash");
const clients = computed(() => {
  const links = importData.value?.links || {};
  const known = catalog.filter((c) => links[c.key]);
  const extra = Object.keys(links)
    .filter((key) => !catalog.some((c) => c.key === key))
    .map((key) => ({
      key,
      name: key,
      platform: "订阅直链",
      mark: key.slice(0, 2),
      color: "#625f5a",
    }));
  return [...known, ...extra];
});
const current = computed(() => {
  const client =
    clients.value.find((c) => c.key === selected.value) || clients.value[0];
  if (!client) return null;
  return {
    ...client,
    link: importData.value.links[client.key] as string,
    importUrl: importData.value.imports?.[client.key] as string | undefined,
  };
});
watch(importSub, () => (selected.value = "clash"));
</script>
<template>
  <UiDialog v-model="qrDialog" :title="qrTitle" width="360px"
    ><div class="qr-body">
      <QrcodeVue :value="qrcode" :size="200" level="H" /><SecretLink
        :value="qrcode"
        ><UiButton @click="openUrl(qrcode)">打开</UiButton></SecretLink
      >
    </div></UiDialog
  >
  <UiDrawer
    v-model="importDialog"
    :title="`${importSub?.Name || ''} · 分享到客户端`"
    size="min(820px, 100vw)"
  >
    <UiSkeleton v-if="importLoading" :lines="4" />
    <div v-else-if="importData" class="share-layout">
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
          <span v-if="importData.imports?.[client.key]" class="client-badge"
            >一键导入</span
          >
        </button>
      </div>
      <section v-if="current" class="client-detail" aria-live="polite">
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
            ><UiButton @click="makeShortLink(current.link)"
              >生成短链</UiButton
            ></SecretLink
          >
          <SecretLink v-if="shortLink" :value="shortLink" label="短链" />
        </div>
      </section>
    </div>
    <EmptyState
      v-else
      size="section"
      title="导入链接未加载"
      description="请检查连接后重试。"
      ><template #action
        ><UiButton v-if="importSub" @click="openImport(importSub)"
          >重新加载</UiButton
        ></template
      ></EmptyState
    >
    <template #footer
      ><UiButton @click="importDialog = false">关闭</UiButton></template
    >
  </UiDrawer>
</template>
<style scoped src="./forms.scss"></style>
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
    transform var(--duration-medium) var(--ease-spring);
}
.client-tile:hover {
  border-color: var(--c-muted);
  transform: translateY(-1px);
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
