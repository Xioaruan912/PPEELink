<script setup lang="ts">
import { UiDrawer, UiButton, UiSegmented, EmptyState } from "@/ui";
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
} = useSubscriptions();
const sections = [
  { label: "基本信息", value: "basic" },
  { label: "节点来源", value: "sources" },
  { label: "模板参数", value: "templates" },
  { label: "处理流水线", value: "pipeline" },
  { label: "应用代理", value: "apps" },
];
</script>
<template>
  <UiDrawer
    v-model="dialogVisible"
    :title="SubTitle"
    size="min(960px, 100vw)"
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    :show-close="!saving"
  >
    <div v-if="metadataError" role="alert" class="field-hint">
      {{ metadataError
      }}<UiButton variant="ghost" @click="loadMetadata">重试</UiButton>
    </div>
    <UiSegmented
      v-model="editorSection"
      :options="sections"
      label="订阅编辑分区"
    />
    <el-form label-position="top" class="sub-form" @submit.prevent="addSubs">
      <section v-show="editorSection === 'basic'" class="form-section">
        <h3>基本信息</h3>
        <el-form-item label="订阅名称" required
          ><el-input v-model="Subname" placeholder="填写订阅名称" clearable
        /></el-form-item>
        <p class="field-hint">按需选择节点来源、模板和处理规则。</p>
      </section>
      <SourceFields v-if="editorSection === 'sources'" />
      <TemplateFields v-if="editorSection === 'templates'" />
      <PipelineFields v-if="editorSection === 'pipeline'" />
      <AppRuleFields v-if="editorSection === 'apps'" />
    </el-form>
    <template #footer
      ><UiButton :disabled="saving" @click="dialogVisible = false"
        >取消</UiButton
      ><UiButton variant="primary" :loading="saving" @click="addSubs">{{
        SubTitle === "添加订阅" ? "新建订阅" : "保存修改"
      }}</UiButton></template
    >
  </UiDrawer>
</template>
<style scoped src="./forms.scss"></style>
