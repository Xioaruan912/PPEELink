<script setup lang="ts">
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
    :title="`${importSub?.Name || ''} · 分享与导入`"
    size="min(760px, 100vw)"
  >
    <UiSkeleton v-if="importLoading" :lines="4" />
    <template v-else-if="importData">
      <h3>一键导入</h3>
      <div
        v-for="(link, client) in importData.imports"
        :key="client"
        class="import-entry"
      >
        <h4>{{ client }}</h4>
        <SecretLink :value="link" :label="`${client} 导入链接`" />
      </div>
      <h3>订阅直链</h3>
      <div
        v-for="(link, client) in importData.links"
        :key="client"
        class="import-entry"
      >
        <h4>{{ client }}</h4>
        <SecretLink :value="link" :label="`${client} 订阅链接`"
          ><UiButton @click="makeShortLink(link)"
            >生成短链</UiButton
          ></SecretLink
        >
      </div>
      <section v-if="shortLink">
        <h3>短链</h3>
        <SecretLink :value="shortLink" label="短链" />
      </section>
    </template>
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
