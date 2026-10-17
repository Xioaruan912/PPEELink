<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import {
  ArrowLeft,
  ArrowRight,
  DocumentChecked,
  DocumentCopy,
  Refresh,
} from "@element-plus/icons-vue";
import { PageHeader, UiButton, UiPanel, UiSegmented, UiSkeleton } from "@/ui";
import TemplateMonacoEditor from "@/components/TemplateMonacoEditor.vue";
import { useMotion } from "@/motion";
import ClashFields from "./builder/ClashFields.vue";
import LoonFields from "./builder/LoonFields.vue";
import { provideTemplateBuilder } from "./builder/useTemplateBuilder";

defineOptions({ name: "TemplateBuilder" });
const {
  loading,
  sourceLoading,
  sourceError,
  buildError,
  savedFilename,
  savedRevision,
  isSavedPreview,
  templateSource,
  templateFiles,
  target,
  form,
  loonForm,
  preview,
  loadDefault,
  onSourceChange,
  build,
  copyYaml,
} = provideTemplateBuilder();
const step = ref<string | number>(0);
const fields = ref<HTMLElement>();
const outline = ref<HTMLElement>();
const { animate, reduced, stop } = useMotion();
const steps = computed(() =>
  target.value === "loon"
    ? [
        { label: "基础配置", value: 0 },
        { label: "节点策略", value: 1 },
        { label: "规则插件", value: 2 },
      ]
    : [
        { label: "基础端口", value: 0 },
        { label: "高级配置", value: 1 },
        { label: "分组规则", value: 2 },
      ]
);
watch([step, target], async () => {
  await nextTick();
  if (fields.value)
    animate(
      fields.value,
      { opacity: 0, x: 12 },
      {
        opacity: 1,
        x: 0,
        duration: 0.32,
        clearProps: "opacity,transform",
        ease: "power3.out",
      }
    );
});
watch(savedRevision, async () => {
  await nextTick();
  if (!outline.value || reduced.value) return;
  stop();
  animate(
    outline.value,
    { opacity: 1, clipPath: "inset(0 100% 100% 0)" },
    {
      opacity: 1,
      clipPath: "inset(0 0 0 0)",
      duration: 0.56,
      ease: "power3.out",
      onComplete: () => {
        if (outline.value)
          animate(
            outline.value,
            { opacity: 1 },
            { opacity: 0, duration: 0.32 }
          );
      },
    }
  );
});
</script>

<template>
  <div class="template-builder-workbench">
    <PageHeader title="模板构建" description="逐步配置，实时核对生成内容。">
      <template #actions>
        <UiButton
          variant="primary"
          :icon="DocumentChecked"
          :loading="loading"
          :disabled="sourceLoading"
          @click="build"
          >生成并保存</UiButton
        >
      </template>
    </PageHeader>

    <section class="builder-source" aria-label="模板来源" data-enter>
      <div class="source-field">
        <label for="builder-target">目标类型</label>
        <el-select id="builder-target" v-model="target" aria-label="目标类型">
          <el-option label="Clash / Mihomo" value="clash" />
          <el-option label="Loon" value="loon" />
        </el-select>
      </div>
      <div class="source-field source-field--wide">
        <label for="builder-source">模板来源</label>
        <el-select
          id="builder-source"
          v-model="templateSource"
          aria-label="模板来源"
          @change="onSourceChange"
        >
          <el-option label="内置默认 mihomo 配置" value="default" />
          <el-option label="新建空白" value="blank" />
          <el-option
            v-for="template in templateFiles"
            :key="template.file"
            :label="'编辑已有：' + template.file"
            :value="'file:' + template.file"
          />
        </el-select>
      </div>
      <div class="source-field source-field--wide">
        <label for="builder-filename"
          >文件名 <span class="required-label">必填</span></label
        >
        <el-input
          v-if="target === 'loon'"
          id="builder-filename"
          v-model="loonForm.filename"
          aria-label="文件名（必填）"
          placeholder="例如 loon.conf"
        />
        <el-input
          v-else
          id="builder-filename"
          v-model="form.filename"
          aria-label="文件名（必填）"
          placeholder="例如 my_clash.yaml"
        />
      </div>
    </section>
    <div v-if="sourceError" class="builder-alert" role="alert">
      <span>{{ sourceError }}</span>
      <UiButton :icon="Refresh" variant="ghost" @click="loadDefault"
        >重试</UiButton
      >
    </div>
    <p v-if="buildError" class="build-error" role="alert">{{ buildError }}</p>

    <div class="builder-layout">
      <UiPanel class="builder-controls" data-enter>
        <div class="step-header">
          <span class="step-count"
            >{{ String(Number(step) + 1).padStart(2, "0") }} / 03</span
          >
          <UiSegmented v-model="step" :options="steps" label="配置步骤" />
        </div>
        <UiSkeleton v-if="sourceLoading" :lines="6" />
        <div v-show="!sourceLoading" ref="fields" class="builder-fields">
          <LoonFields v-if="target === 'loon'" :step="step" />
          <ClashFields v-else :step="step" />
        </div>
        <template #footer>
          <nav class="step-actions" aria-label="步骤导航">
            <UiButton
              :icon="ArrowLeft"
              :disabled="Number(step) === 0"
              @click="step = Number(step) - 1"
              >上一步</UiButton
            >
            <UiButton
              v-if="Number(step) < 2"
              :icon="ArrowRight"
              @click="step = Number(step) + 1"
              >下一步</UiButton
            >
            <span v-else class="step-finish">配置完成后，生成并保存。</span>
          </nav>
        </template>
      </UiPanel>

      <aside class="builder-preview" aria-label="配置预览" data-enter>
        <UiPanel title="配置预览">
          <template #actions
            ><UiButton
              :icon="DocumentCopy"
              variant="ghost"
              :disabled="!preview"
              @click="copyYaml"
              >复制</UiButton
            ></template
          >
          <p class="preview-note" role="status">
            {{
              isSavedPreview
                ? `已保存：${savedFilename}`
                : target === "loon"
                  ? "当前表单的实时配置，尚未保存。"
                  : "表单实时预览；保存时补全内置配置。"
            }}
          </p>
          <div class="preview-editor">
            <TemplateMonacoEditor
              :model-value="preview"
              :language="target === 'loon' ? 'ini' : 'yaml'"
              :read-only="true"
              :minimap="false"
            />
          </div>
          <div ref="outline" class="preview-outline" aria-hidden="true"></div>
        </UiPanel>
      </aside>
    </div>
  </div>
</template>

<style scoped lang="scss">
.template-builder-workbench {
  min-width: 0;
}
.builder-source {
  display: grid;
  grid-template-columns: minmax(160px, 0.7fr) minmax(0, 1.3fr) minmax(0, 1fr);
  gap: 24px;
  margin-bottom: 32px;
  padding-bottom: 32px;
  border-bottom: 1px solid var(--c-line);
}
.source-field {
  min-width: 0;
  label {
    display: block;
    margin-bottom: 8px;
    font-size: 14px;
    color: var(--c-muted);
  }
  :deep(.el-select) {
    width: 100%;
  }
}
.required-label {
  margin-left: 8px;
  font-size: 13px;
}
.builder-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(0, 1fr);
  align-items: start;
  gap: 24px;
}
.builder-controls,
.builder-fields {
  min-width: 0;
}
.step-header {
  display: flex;
  flex-direction: column;
  align-items: start;
  gap: 16px;
  margin-bottom: 32px;
}
.step-count {
  font: 400 13px/1.4 var(--font-mono);
  color: var(--c-muted);
}
.step-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
.step-finish {
  color: var(--c-muted);
  font-size: 14px;
}
.builder-preview {
  position: sticky;
  top: 96px;
  min-width: 0;
  > :deep(.ui-panel) {
    position: relative;
  }
  :deep(.ui-panel__header) {
    margin-bottom: 8px;
  }
}
.preview-note {
  margin: 0 0 24px;
  font-size: 14px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.preview-editor {
  height: min(74vh, 860px);
  min-height: 520px;
  min-width: 0;
}
.preview-outline {
  position: absolute;
  inset: -1px;
  border: 2px solid var(--c-signal);
  border-radius: var(--radius-panel);
  pointer-events: none;
  opacity: 0;
}
.builder-alert {
  display: flex;
  gap: 16px;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 24px;
  color: var(--c-bad);
  font-size: 14px;
}
.build-error {
  color: var(--c-bad);
  font-size: 14px;
  margin-bottom: 24px;
}
@media (max-width: 1100px) {
  .builder-layout {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }
  .builder-preview {
    position: relative;
    top: auto;
  }
}
@media (max-width: 767px) {
  .builder-source {
    grid-template-columns: minmax(0, 1fr);
    gap: 16px;
  }
  .step-actions {
    align-items: start;
  }
  .step-finish {
    max-width: 150px;
  }
  .preview-editor {
    height: 560px;
  }
  .builder-alert {
    align-items: start;
  }
}
</style>
