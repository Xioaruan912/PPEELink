<script setup lang="ts">
import { UiDrawer, UiDialog, UiButton, UiSkeleton, EmptyState } from "@/ui";
import TemplateMonacoDiff from "@/components/TemplateMonacoDiff.vue";
import VersionTimeline from "./VersionTimeline.vue";
import { useTemplates } from "./useTemplates";
const {
  versionsVisible,
  diffVisible,
  diffLoading,
  diffError,
  oldVersionTitle,
  oldVersionText,
  TempText,
  editorLanguage,
} = useTemplates();
</script>
<template>
  <UiDrawer v-model="versionsVisible" title="模板版本历史" size="460px"
    ><VersionTimeline /><template #footer
      ><UiButton @click="versionsVisible = false">关闭</UiButton></template
    ></UiDrawer
  >
  <UiDialog
    v-model="diffVisible"
    :title="oldVersionTitle || '版本比较'"
    width="min(1480px, 96vw)"
    top="3vh"
    ><p class="diff-caption">历史版本 → 当前编辑内容；新增与删除会分别高亮。</p>
    <UiSkeleton v-if="diffLoading" :lines="6" height="48px" /><EmptyState
      v-else-if="diffError"
      size="section"
      title="版本读取失败"
      :description="diffError"
    /><TemplateMonacoDiff
      v-else-if="diffVisible"
      :original="oldVersionText"
      :modified="TempText"
      :language="editorLanguage"
    /><template #footer
      ><UiButton @click="diffVisible = false">关闭</UiButton></template
    ></UiDialog
  >
</template>
<style scoped>
.diff-caption {
  margin: 0 0 24px;
  color: var(--c-muted);
  font-size: 14px;
}
</style>
