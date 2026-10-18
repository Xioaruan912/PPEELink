<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as monaco from "monaco-editor/esm/vs/editor/editor.api";
import "monaco-editor/esm/vs/basic-languages/yaml/yaml.contribution";
import "monaco-editor/esm/vs/basic-languages/ini/ini.contribution";
import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import "monaco-editor/min/vs/editor/editor.main.css";
import { useTemplateMonacoTheme } from "./templateMonacoTheme";
import { usePrefersReducedMotion } from "@/motion";
(self as any).MonacoEnvironment = { getWorker: () => new EditorWorker() };
const props = defineProps<{
  original: string;
  modified: string;
  language?: string;
}>();
const host = ref<HTMLElement>();
const theme = useTemplateMonacoTheme();
const reduced = usePrefersReducedMotion();
let editor: monaco.editor.IStandaloneDiffEditor | undefined;
let originalModel: monaco.editor.ITextModel | undefined;
let modifiedModel: monaco.editor.ITextModel | undefined;
let observer: ResizeObserver | undefined;
const wide = window.matchMedia("(min-width: 768px)");
const resize = () => {
  editor?.updateOptions({ renderSideBySide: wide.matches });
  editor?.layout();
};
function resetModels() {
  if (!editor) return;
  editor.setModel(null);
  originalModel?.dispose();
  modifiedModel?.dispose();
  originalModel = monaco.editor.createModel(
    props.original || "",
    props.language || "yaml"
  );
  modifiedModel = monaco.editor.createModel(
    props.modified || "",
    props.language || "yaml"
  );
  editor.setModel({ original: originalModel, modified: modifiedModel });
}
onMounted(() => {
  if (!host.value) return;
  editor = monaco.editor.createDiffEditor(host.value, {
    theme: theme.value,
    automaticLayout: false,
    readOnly: true,
    originalEditable: false,
    renderSideBySide: wide.matches,
    renderOverviewRuler: true,
    minimap: { enabled: false },
    scrollBeyondLastLine: false,
    wordWrap: "on",
    fontSize: 14,
    lineHeight: 22,
    fontFamily: '"JetBrains Mono", monospace',
    lineNumbers: "on",
    smoothScrolling: !reduced.value,
    cursorBlinking: reduced.value ? "solid" : "blink",
  });
  resetModels();
  observer = new ResizeObserver(resize);
  observer.observe(host.value);
  wide.addEventListener("change", resize);
});
watch(() => [props.original, props.modified, props.language], resetModels);
watch(reduced, (value) =>
  editor?.updateOptions({
    smoothScrolling: !value,
    cursorBlinking: value ? "solid" : "blink",
  })
);
onBeforeUnmount(() => {
  observer?.disconnect();
  wide.removeEventListener("change", resize);
  editor?.dispose();
  originalModel?.dispose();
  modifiedModel?.dispose();
});
</script>
<template><div ref="host" class="diff-host"></div></template>
<style scoped>
.diff-host {
  width: 100%;
  height: 68vh;
  min-height: 420px;
  min-width: 0;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  overflow: hidden;
}
</style>
