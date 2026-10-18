<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api";
import "monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution";
import "monaco-editor/esm/vs/basic-languages/ini/ini.contribution";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import "monaco-editor/min/vs/editor/editor.main.css";
import { useTemplateMonacoTheme } from "./templateMonacoTheme";
import { usePrefersReducedMotion } from "@/motion";

const props = withDefaults(
  defineProps<{
    modelValue: string;
    language?: string;
    errors?: string[];
    readOnly?: boolean;
    minimap?: boolean;
  }>(),
  { language: "yaml", errors: () => [], readOnly: false, minimap: true }
);
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const host = ref<HTMLElement>();
const theme = useTemplateMonacoTheme();
const reduced = usePrefersReducedMotion();
let editor: monaco.editor.IStandaloneCodeEditor | undefined;
let model: monaco.editor.ITextModel | undefined;
let resizeObserver: ResizeObserver | undefined;
let contentListener: monaco.IDisposable | undefined;
let updating = false;
(self as any).MonacoEnvironment = { getWorker: () => new EditorWorker() };

const revealLine = (line: number) => {
  const target = Math.max(1, Math.min(model?.getLineCount() || 1, line));
  editor?.revealLineInCenter(target);
  editor?.setPosition({ lineNumber: target, column: 1 });
  editor?.focus();
};
defineExpose({ revealLine });
function markErrors() {
  if (!model) return;
  const current = model;
  monaco.editor.setModelMarkers(
    current,
    "server-validation",
    props.errors.map((message) => {
      const match = message.match(/(?:line\s*|第\s*)(\d+)/i);
      const line = Math.max(
        1,
        Math.min(current.getLineCount(), match ? Number(match[1]) : 1)
      );
      return {
        severity: monaco.MarkerSeverity.Error,
        message,
        startLineNumber: line,
        startColumn: 1,
        endLineNumber: line,
        endColumn: current.getLineMaxColumn(line),
      };
    })
  );
}
onMounted(() => {
  if (!host.value) return;
  model = monaco.editor.createModel(props.modelValue, props.language);
  editor = monaco.editor.create(host.value, {
    model,
    theme: theme.value,
    readOnly: props.readOnly,
    automaticLayout: false,
    minimap: { enabled: props.minimap },
    fontFamily: '"JetBrains Mono", monospace',
    fontSize: 14,
    lineHeight: 22,
    smoothScrolling: !reduced.value,
    cursorBlinking: reduced.value ? "solid" : "blink",
    scrollBeyondLastLine: false,
    wordWrap: "on",
    padding: { top: 16, bottom: 16 },
    ariaLabel: props.readOnly ? "模板配置预览" : "模板内容编辑器",
    fixedOverflowWidgets: true,
  });
  contentListener = editor.onDidChangeModelContent(() => {
    if (!updating) emit("update:modelValue", editor?.getValue() || "");
  });
  markErrors();
  resizeObserver = new ResizeObserver(() => editor?.layout());
  resizeObserver.observe(host.value);
});
watch(
  () => props.modelValue,
  (value) => {
    if (!editor || editor.getValue() === value) return;
    updating = true;
    editor.setValue(value);
    updating = false;
    markErrors();
  }
);
watch(
  () => props.language,
  (value) => {
    if (model) monaco.editor.setModelLanguage(model, value);
  }
);
watch(() => props.errors, markErrors, { deep: true });
watch(
  () => [props.readOnly, props.minimap, reduced.value],
  () => {
    editor?.updateOptions({
      readOnly: props.readOnly,
      minimap: { enabled: props.minimap },
      smoothScrolling: !reduced.value,
      cursorBlinking: reduced.value ? "solid" : "blink",
    });
  }
);
onBeforeUnmount(() => {
  resizeObserver?.disconnect();
  contentListener?.dispose();
  editor?.dispose();
  model?.dispose();
});
</script>
<template><div ref="host" class="monaco-host"></div></template>
<style scoped>
.monaco-host {
  width: 100%;
  height: 100%;
  min-height: 520px;
  min-width: 0;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  overflow: hidden;
}
@media (max-width: 767px) {
  .monaco-host {
    min-height: 420px;
  }
}
</style>
