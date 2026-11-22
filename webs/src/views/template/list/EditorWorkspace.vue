<script setup lang="ts">
import { Check } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
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
  <section class="editor-studio">
    <div class="studio-bar">
      <input
        v-model="Tempname"
        type="text"
        class="filename-input mono"
        :disabled="saving"
        aria-label="模板文件名"
        placeholder="输入文件名，如 my_clash.yaml / loon.conf"
      />

      <div class="studio-right">
        <span
          class="syntax-pill"
          :class="{
            'is-ok': validationCurrent && !validationErrors.length && !validating,
            'is-bad': validationCurrent && validationErrors.length > 0 && !validating,
            'is-warn': validating,
          }"
        >
          <i aria-hidden="true"></i>
          {{
            validating
              ? "语法校验中…"
              : !validationCurrent
                ? "未校验"
                : validationErrors.length
                  ? `${validationErrors.length} 处语法错误`
                  : "语法通过"
          }}
        </span>

        <UiButton
          variant="ghost"
          :icon="Check"
          :loading="validating"
          :disabled="saving"
          @click="runValidation()"
        >
          校验语法
        </UiButton>
      </div>
    </div>

    <KineticNotice :raw="validationError" />

    <ul
      v-if="validationCurrent && validationErrors.length"
      class="error-box mono"
      role="alert"
    >
      <li v-for="message in validationErrors" :key="message">{{ message }}</li>
    </ul>

    <div class="editor-vessel">
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
.editor-studio {
  display: grid;
  gap: 14px;
}

.studio-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  flex-wrap: wrap;
}

.filename-input {
  flex: 1 1 260px;
  min-width: 0;
  padding: 4px 0;
  border: 0;
  border-bottom: 1px solid transparent;
  background: transparent;
  color: var(--c-ink);
  font-size: 14px;
  font-weight: 500;
  outline: none !important;
}

.filename-input:focus {
  border-bottom-color: var(--c-ink);
}

.studio-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.syntax-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 12px;
  color: var(--c-muted);
}

.syntax-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.syntax-pill.is-ok {
  color: var(--c-ok);
  background: rgb(21 184 90 / 0.12);
}

.syntax-pill.is-bad {
  color: var(--c-bad);
  background: rgb(220 38 38 / 0.12);
}

.syntax-pill.is-warn {
  color: var(--c-warn);
}

.error-box {
  margin: 0;
  padding: 12px 18px 12px 30px;
  border: 1px solid color-mix(in srgb, var(--c-bad) 35%, var(--c-line));
  border-radius: 14px;
  background: var(--c-surface);
  color: var(--c-bad);
  font-size: 12px;
  line-height: 1.6;
}

.editor-vessel {
  height: max(460px, calc(100dvh - 260px));
  display: flex;
  flex-direction: column;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  overflow: hidden;
  background: var(--c-surface);
}

.editor-vessel :deep(.monaco-host) {
  flex: 1;
  min-height: 0;
}

@media (max-width: 767px) {
  .editor-vessel {
    height: 58dvh;
  }
}
</style>
