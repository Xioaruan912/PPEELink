<script setup lang="ts">
import { Plus, Back } from "@element-plus/icons-vue";
import { PageHeader, UiButton } from "@/ui";
import { provideTemplates } from "./list/useTemplates";
import TemplateList from "./list/TemplateList.vue";
import EditorWorkspace from "./list/EditorWorkspace.vue";
defineOptions({ name: "TemplateListPage" });
const {
  dialogVisible,
  TempTitle,
  dirty,
  saving,
  handleAddTemp,
  closeEditor,
  addtemp,
} = provideTemplates();
</script>
<template>
  <div class="templates-page">
    <PageHeader
      :title="dialogVisible ? '编辑工作区' : '模板'"
      :description="
        dialogVisible
          ? `${TempTitle}${dirty ? '，有未保存修改' : ''}`
          : '编辑客户端配置模板。'
      "
      ><template #actions
        ><template v-if="dialogVisible"
          ><UiButton :icon="Back" :disabled="saving" @click="closeEditor"
            >返回列表</UiButton
          ><UiButton variant="primary" :loading="saving" @click="addtemp">{{
            TempTitle === "添加模板" ? "添加模板" : "保存修改"
          }}</UiButton></template
        ><UiButton v-else variant="primary" :icon="Plus" @click="handleAddTemp"
          >添加模板</UiButton
        ></template
      ></PageHeader
    >
    <EditorWorkspace v-if="dialogVisible" /><TemplateList v-else />
  </div>
</template>
<style scoped>
.templates-page {
  min-width: 0;
}
</style>
