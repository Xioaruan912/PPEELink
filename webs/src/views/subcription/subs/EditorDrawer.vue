<script setup lang="ts">
import { computed } from "vue";
import { UiDrawer, UiButton } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
import TemplateFields from "./TemplateFields.vue";
import SourceFields from "./SourceFields.vue";
import AppRuleFields from "./AppRuleFields.vue";
import PipelineFields from "./PipelineFields.vue";

const {
  dialogVisible,
  SubTitle,
  Subname,
  editorSection,
  saving,
  addSubs,
  metadataError,
  loadMetadata,
  selectedGroups,
  value1,
  appRules,
} = useSubscriptions();

const sourceBadge = computed(() => {
  const g = selectedGroups.value.length;
  const n = value1.value.length;
  if (!g && !n) return "";
  if (g && n) return `${g}组·${n}点`;
  return g ? `${g}组` : `${n}点`;
});

const sections = computed(() => [
  { label: "基本信息", value: "basic", badge: "" },
  { label: "节点来源", value: "sources", badge: sourceBadge.value },
  { label: "模板参数", value: "templates", badge: "" },
  { label: "处理流水线", value: "pipeline", badge: "" },
  {
    label: "应用代理",
    value: "apps",
    badge: appRules.value.length ? String(appRules.value.length) : "",
  },
]);
</script>

<template>
  <UiDrawer
    v-model="dialogVisible"
    :title="SubTitle"
    size="min(920px, 100vw)"
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    :show-close="!saving"
  >
    <div class="sub-editor">
      <div v-if="metadataError" role="alert" class="meta-alert">
        <span>{{ metadataError }}</span>
        <UiButton variant="ghost" @click="loadMetadata">重试</UiButton>
      </div>

      <!-- Editorial Underline Tabs -->
      <nav class="editor-tabs" role="tablist" aria-label="订阅编辑分区">
        <button
          v-for="tab in sections"
          :key="tab.value"
          type="button"
          role="tab"
          class="editor-tab"
          :class="{ 'is-active': editorSection === tab.value }"
          :aria-selected="editorSection === tab.value"
          @click="editorSection = tab.value"
        >
          <span>{{ tab.label }}</span>
          <span v-if="tab.badge" class="tab-badge mono">{{ tab.badge }}</span>
        </button>
      </nav>

      <el-form label-position="top" class="editor-body" @submit.prevent="addSubs">
        <section v-show="editorSection === 'basic'" class="basic-panel">
          <el-form-item label="订阅名称" required>
            <el-input
              v-model="Subname"
              placeholder="例如：日常主力线路、海外流媒体"
              size="large"
              clearable
            />
          </el-form-item>
          <p class="panel-note">
            保存后将自动生成专属订阅链接，可按需在上方页签配置节点来源、模板与处理规则。
          </p>
        </section>

        <SourceFields v-if="editorSection === 'sources'" />
        <TemplateFields v-if="editorSection === 'templates'" />
        <PipelineFields v-if="editorSection === 'pipeline'" />
        <AppRuleFields v-if="editorSection === 'apps'" />
      </el-form>
    </div>

    <template #footer>
      <div class="editor-footer">
        <UiButton :disabled="saving" @click="dialogVisible = false">
          取消
        </UiButton>
        <UiButton variant="primary" :loading="saving" @click="addSubs">
          {{ SubTitle === "添加订阅" ? "新建订阅" : "保存修改" }}
        </UiButton>
      </div>
    </template>
  </UiDrawer>
</template>

<style scoped>
.sub-editor {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding-bottom: 20px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

.meta-alert {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, #ef4444 28%, transparent);
  background: color-mix(in srgb, #ef4444 6%, var(--c-surface));
  font-size: 13px;
  color: var(--c-ink);
}

.editor-tabs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 22px;
  border-bottom: 1px solid var(--c-line);
}

.editor-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 2px 0 12px;
  border: none;
  background: transparent;
  font-size: 14px;
  font-weight: 500;
  color: var(--c-muted);
  cursor: pointer;
  transition: color 0.18s ease;
}

.editor-tab:hover {
  color: var(--c-ink);
}

.editor-tab.is-active {
  color: var(--c-ink);
  font-weight: 600;
}

.editor-tab.is-active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  background: var(--c-ink);
  border-radius: 2px;
}

.tab-badge {
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--c-surface-2, rgba(0, 0, 0, 0.05));
  font-size: 11.5px;
  color: var(--c-muted);
}

.editor-tab.is-active .tab-badge {
  background: var(--c-ink);
  color: var(--c-surface);
}

.editor-body {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.basic-panel {
  max-width: 520px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.panel-note {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--c-muted);
}

.editor-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}
</style>
