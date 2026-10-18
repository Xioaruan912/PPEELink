<script setup lang="ts">
import { UiButton, StatusDot } from "@/ui";
import TemplateMonacoEditor from "@/components/TemplateMonacoEditor.vue";
import { useTemplates } from "./useTemplates";
const {
  Tempname,
  TempText,
  editorLanguage,
  validationErrors,
  validating,
  validationError,
  validationCurrent,
  saving,
  runValidation,
} = useTemplates();
</script>
<template>
  <section data-enter>
    <div class="workspace-toolbar">
      <el-input
        v-model="Tempname"
        class="filename"
        :disabled="saving"
        aria-label="模板文件名"
        placeholder="文件名，如 my_clash.yaml / loon.conf"
        clearable
      />
      <StatusDot
        :state="
          validating
            ? 'warn'
            : !validationCurrent
              ? 'idle'
              : validationErrors.length
                ? 'bad'
                : 'ok'
        "
        :running="validating"
        :label="
          validating
            ? '正在检查'
            : !validationCurrent
              ? '未校验'
              : validationErrors.length
                ? `${validationErrors.length} 处语法错误`
                : '语法通过'
        "
      />
      <UiButton
        :loading="validating"
        :disabled="saving"
        @click="runValidation()"
        >校验语法</UiButton
      >
    </div>
    <p v-if="validationError" class="message danger" role="alert">
      {{ validationError }}
    </p>
    <ul
      v-if="validationCurrent && validationErrors.length"
      class="message danger error-list"
    >
      <li v-for="message in validationErrors" :key="message">{{ message }}</li>
    </ul>
    <div class="editor-pane">
      <TemplateMonacoEditor
        v-model="TempText"
        :language="editorLanguage"
        :errors="validationCurrent ? validationErrors : []"
        :read-only="saving"
        :minimap="false"
      />
    </div>
  </section>
</template>
<style scoped>
.workspace-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}
.filename {
  flex: 1;
  max-width: 420px;
}
.workspace-toolbar :deep(.ui-button) {
  margin-left: auto;
}
.message {
  margin: 0 0 12px;
  font-size: 13px;
}
.danger {
  color: var(--c-bad);
}
.error-list {
  padding-left: 16px;
  overflow-wrap: anywhere;
}
.editor-pane {
  height: max(480px, calc(100dvh - 240px));
  display: flex;
  flex-direction: column;
}
.editor-pane :deep(.monaco-host) {
  flex: 1;
  min-height: 0;
}
@media (max-width: 767px) {
  .workspace-toolbar {
    flex-wrap: wrap;
  }
  .filename {
    max-width: none;
    flex-basis: 100%;
  }
  .editor-pane {
    height: 60dvh;
  }
}
</style>
