<script setup lang="ts">
import { useMediaQuery } from "@vueuse/core";
import { EmptyState, UiButton, UiDialog, UiDrawer, UiSkeleton } from "@/ui";
import RuleAnalysis from "./RuleAnalysis.vue";
import { useRules } from "./useRules";
const mobile = useMediaQuery("(max-width: 767px)");
const {
  focusedRule,
  detailOpen,
  operationLoading,
  importOpen,
  importLoading,
  templateLoading,
  groupsLoading,
  importError,
  importItems,
  templates,
  policyGroups,
  proxyOptions,
  importForm,
  loadTemplateGroups,
  submitImport,
  confirmation,
  resolveConfirmation,
} = useRules();
</script>
<template>
  <UiDrawer
    v-model="detailOpen"
    :title="focusedRule?.name || '规则分析'"
    :direction="mobile ? 'btt' : 'rtl'"
    :size="mobile ? '88dvh' : 'min(840px, 100vw)'"
    :close-on-click-modal="!operationLoading"
    :close-on-press-escape="!operationLoading"
    :show-close="!operationLoading"
    ><RuleAnalysis /><template #footer
      ><UiButton :disabled="operationLoading" @click="detailOpen = false"
        >关闭</UiButton
      ></template
    ></UiDrawer
  >
  <UiDialog
    v-model="importOpen"
    title="导入规则"
    width="640px"
    :close-on-click-modal="!importLoading"
    :close-on-press-escape="!importLoading"
    :show-close="!importLoading"
  >
    <p class="import-summary">
      将导入 {{ importItems.length }} 项：{{
        importItems.map((row) => row.name).join("、")
      }}
    </p>
    <p v-if="importError" class="notice error" role="alert">
      {{ importError }}
    </p>
    <UiSkeleton v-if="templateLoading" :lines="4" height="48px" />
    <template v-else>
      <EmptyState
        v-if="!templates.length"
        size="section"
        title="暂无可用模板"
        description="添加 YAML 模板后即可导入 Clash 规则。"
      />
      <el-form v-else label-position="top" @submit.prevent="submitImport">
        <el-form-item label="目标模板"
          ><el-select
            v-model="importForm.template"
            filterable
            aria-label="目标模板"
            :disabled="importLoading"
            @change="loadTemplateGroups"
            ><el-option
              v-for="template in templates"
              :key="template.file"
              :label="template.file"
              :value="template.file" /></el-select
        ></el-form-item>
        <el-form-item label="策略组"
          ><el-select
            v-model="importForm.policy"
            aria-label="策略组"
            :loading="groupsLoading"
            :disabled="groupsLoading || importLoading"
            ><el-option
              v-for="group in policyGroups"
              :key="group"
              :label="group"
              :value="group" /></el-select
          ><span class="field-hint"
            >读取模板的 proxy-groups，并额外提供 DIRECT。</span
          ></el-form-item
        >
        <el-form-item label="规则下载代理（proxy）"
          ><el-select
            v-model="importForm.proxy"
            aria-label="规则下载代理"
            :disabled="groupsLoading || importLoading"
            ><el-option label="不指定（proxy: 空字符串）" value="" /><el-option
              v-for="group in proxyOptions"
              :key="group"
              :label="group"
              :value="group" /></el-select
          ><span class="field-hint"
            >通过所选策略下载远程规则文件。</span
          ></el-form-item
        >
        <el-form-item label="Provider 冲突方式"
          ><el-select
            v-model="importForm.conflictPolicy"
            aria-label="Provider 冲突方式"
            :disabled="importLoading"
            ><el-option label="保留现有" value="keep" /><el-option
              label="更新 URL"
              value="update-url" /><el-option
              label="替换"
              value="replace" /></el-select
        ></el-form-item>
      </el-form>
      <p class="notice">
        使用 Rule Provider，插入 MATCH 前；写入前会创建模板版本。
      </p>
    </template>
    <template #footer
      ><UiButton :disabled="importLoading" @click="importOpen = false"
        >取消</UiButton
      ><UiButton
        variant="primary"
        :loading="importLoading"
        :disabled="
          templateLoading ||
          groupsLoading ||
          !importForm.template ||
          !importForm.policy.trim()
        "
        @click="submitImport"
        >检查并导入</UiButton
      ></template
    >
  </UiDialog>
  <UiDialog
    :model-value="confirmation.visible"
    :title="confirmation.title"
    width="480px"
    @update:model-value="!$event && resolveConfirmation(false)"
    ><p class="confirmation-copy">{{ confirmation.message }}</p>
    <template #footer
      ><UiButton @click="resolveConfirmation(false)">取消</UiButton
      ><UiButton variant="danger" @click="resolveConfirmation(true)">{{
        confirmation.action
      }}</UiButton></template
    ></UiDialog
  >
</template>
<style scoped>
.import-summary,
.confirmation-copy {
  font-size: 16px;
  line-height: 1.7;
  overflow-wrap: anywhere;
  margin: 0 0 24px;
}
.notice {
  padding: 12px 16px;
  background: var(--c-surface-2);
  border-left: 2px solid var(--c-warn);
  font-size: 14px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.notice.error {
  border-color: var(--c-bad);
  color: var(--c-bad);
}
.field-hint {
  display: block;
  margin-top: 8px;
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.6;
}
:deep(.el-select) {
  width: 100%;
}
:deep(.el-form-item) {
  margin-bottom: 24px;
}
</style>
