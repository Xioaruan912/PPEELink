<script setup lang="ts">
import QrcodeVue from "qrcode.vue";
import { UiDialog, UiDrawer, UiButton, UiSkeleton, EmptyState } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
import SecretLink from "./SecretLink.vue";
import ClientShare from "@/components/share/ClientShare.vue";
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
    :title="`${importSub?.Name || ''} · 分享到客户端`"
    size="min(820px, 100vw)"
  >
    <UiSkeleton v-if="importLoading" :lines="4" />
    <template v-else-if="importData">
      <ClientShare
        :links="importData.links"
        :imports="importData.imports"
        :reset-key="importSub?.ID"
        ><template #actions="{ link }"
          ><UiButton @click="makeShortLink(link)">生成短链</UiButton></template
        ></ClientShare
      >
      <SecretLink
        v-if="shortLink"
        class="short-link"
        :value="shortLink"
        label="短链"
      />
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
<style scoped>
.short-link {
  margin-top: 16px;
}
</style>
