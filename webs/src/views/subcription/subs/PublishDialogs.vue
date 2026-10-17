<script setup lang="ts">
import { UiDialog, UiButton, EmptyState } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
const {
  publishDialog,
  publishSub,
  publishForm,
  publishLoading,
  templist,
  runSafePublish,
  buildDialog,
  buildSub,
  buildClient,
  buildLoading,
  runBuild,
  confirmation,
  resolveConfirmation,
} = useSubscriptions();
const clients = [
  "clash",
  "surge",
  "loon",
  "v2ray",
  "singbox",
  "qx",
  "shadowrocket",
];
</script>
<template>
  <UiDialog
    v-model="publishDialog"
    :title="`${publishSub?.Name || ''} · 发布验证`"
    width="600px"
    :close-on-click-modal="!publishLoading"
    :close-on-press-escape="!publishLoading"
    :show-close="!publishLoading"
  >
    <p class="field-hint">
      模板预检 → 协议兼容 → 分流回归 → 候选产物 → 出口验证 → LKG
      对比。全部通过后发布。
    </p>
    <el-form label-position="top" @submit.prevent="runSafePublish"
      ><el-form-item label="目标客户端"
        ><el-select
          v-model="publishForm.client"
          @change="publishForm.template = ''"
          ><el-option label="Clash/Mihomo" value="clash" /><el-option
            label="Surge"
            value="surge" /><el-option
            label="Loon"
            value="loon" /></el-select></el-form-item
      ><el-form-item label="模板"
        ><el-select v-model="publishForm.template" filterable
          ><el-option
            v-for="t in templist.filter((t) =>
              publishForm.client === 'clash'
                ? /\.ya?ml$/i.test(t.file)
                : /\.conf$/i.test(t.file)
            )"
            :key="t.file"
            :label="t.file"
            :value="t.file" /></el-select></el-form-item
    ></el-form>
    <EmptyState
      v-if="!templist.length"
      size="section"
      title="暂无模板"
      description="添加模板后可运行发布验证。"
    />
    <template #footer
      ><UiButton :disabled="publishLoading" @click="publishDialog = false"
        >取消</UiButton
      ><UiButton
        variant="primary"
        :loading="publishLoading"
        :disabled="!publishForm.template"
        @click="runSafePublish"
        >开始发布验证</UiButton
      ></template
    >
  </UiDialog>
  <UiDialog
    v-model="buildDialog"
    :title="`${buildSub?.Name || ''} · 生成产物`"
    :close-on-click-modal="!buildLoading"
    :close-on-press-escape="!buildLoading"
    :show-close="!buildLoading"
    ><el-form label-position="top"
      ><el-form-item label="目标客户端"
        ><el-select v-model="buildClient"
          ><el-option
            v-for="client in clients"
            :key="client"
            :label="client"
            :value="client" /></el-select></el-form-item></el-form
    ><template #footer
      ><UiButton :disabled="buildLoading" @click="buildDialog = false"
        >取消</UiButton
      ><UiButton variant="primary" :loading="buildLoading" @click="runBuild"
        >生成产物</UiButton
      ></template
    ></UiDialog
  >
  <UiDialog
    v-model="confirmation.visible"
    :title="confirmation.title"
    width="480px"
    ><p>{{ confirmation.message }}</p>
    <template #footer
      ><UiButton @click="resolveConfirmation(false)">取消</UiButton
      ><UiButton variant="danger" @click="resolveConfirmation(true)">{{
        confirmation.action
      }}</UiButton></template
    ></UiDialog
  >
</template>
<style scoped src="./forms.scss"></style>
