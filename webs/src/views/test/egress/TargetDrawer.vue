<template>
  <UiDrawer
    :model-value="modelValue"
    title="检测目标"
    size="720px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <p class="drawer-note">
      目标定义检测方式；期望地区由模板规则与策略组推导。
    </p>
    <div v-if="error" class="drawer-error" role="alert">{{ error }}</div>
    <form class="target-form" @submit.prevent="persist">
      <label
        >标识 key<el-input
          v-model="form.key"
          :disabled="!!form.id || saving"
          placeholder="例如 youtube"
      /></label>
      <label
        >名称<el-input
          v-model="form.name"
          :disabled="saving"
          placeholder="目标名称"
      /></label>
      <label
        >域名<el-input
          v-model="form.domain"
          :disabled="saving"
          placeholder="例如 www.youtube.com"
      /></label>
      <label
        >分类<el-input
          v-model="form.group"
          :disabled="saving"
          placeholder="例如 media"
      /></label>
      <label
        >图标 / Emoji<el-input
          v-model="form.icon"
          :disabled="saving"
          placeholder="图标或文字"
      /></label>
      <label
        >检测路径<el-input
          v-model="form.path"
          :disabled="saving"
          placeholder="/cdn-cgi/trace"
      /></label>
      <label
        >请求方法<el-select
          v-model="form.method"
          :disabled="saving"
          aria-label="请求方法"
          ><el-option label="GET" value="GET" /><el-option
            label="HEAD"
            value="HEAD" /></el-select
      ></label>
      <label
        >期望状态<el-input
          v-model="form.expectedStatus"
          :disabled="saving"
          placeholder="200-399 或 200,204"
      /></label>
      <label class="full-field"
        >响应必须包含<el-input
          v-model="form.responseContains"
          :disabled="saving"
          placeholder="可选，忽略大小写"
      /></label>
      <label
        >超时（秒）<el-input-number
          v-model="form.timeoutSeconds"
          :disabled="saving"
          :min="1"
          :max="60"
          controls-position="right"
      /></label>
      <label
        >重试次数<el-input-number
          v-model="form.retries"
          :disabled="saving"
          :min="0"
          :max="5"
          controls-position="right"
      /></label>
      <label
        >排列顺序<el-input-number
          v-model="form.sortOrder"
          :disabled="saving"
          :min="0"
          :max="9999"
          controls-position="right"
      /></label>
      <div class="target-switches">
        <el-switch
          v-model="form.requireEgressIp"
          :disabled="saving"
          active-text="要求出口 IP"
        /><el-switch
          v-model="form.enabled"
          :disabled="saving"
          active-text="启用目标"
        />
      </div>
      <div class="form-actions full-field">
        <UiButton :disabled="saving" :icon="Plus" @click="newTarget"
          >新建目标</UiButton
        ><UiButton variant="primary" type="submit" :loading="saving"
          >保存目标</UiButton
        >
      </div>
    </form>
    <h3 class="list-title">
      已配置目标 <span>{{ targets.length }}</span>
    </h3>
    <UiTable :rows="targets" row-key="key" label="检测目标管理">
      <el-table-column
        prop="name"
        label="名称"
        min-width="110"
      /><el-table-column
        prop="domain"
        label="域名"
        min-width="170"
      /><el-table-column prop="group" label="分类" width="80" />
      <el-table-column label="状态" width="80"
        ><template #default="{ row }"
          ><span class="target-state">{{
            row.enabled ? "启用" : "停用"
          }}</span></template
        ></el-table-column
      >
      <el-table-column label="操作" width="104"
        ><template #default="{ row }"
          ><div class="row-actions">
            <RowIconButton
              :icon="Edit"
              label="编辑目标"
              :disabled="saving || !!deleting"
              @click="edit(row)"
            /><RowIconButton
              :icon="Delete"
              label="删除目标"
              :disabled="saving || !!deleting"
              @click="remove(row)"
            /></div></template
      ></el-table-column>
      <template #mobile-card="{ row }"
        ><div class="mobile-target">
          <strong>{{ row.name }}</strong
          ><span>{{ row.enabled ? "启用" : "停用" }} · {{ row.group }}</span>
          <p>{{ row.domain }}</p>
          <div class="row-actions">
            <RowIconButton
              :icon="Edit"
              label="编辑目标"
              :disabled="saving || !!deleting"
              @click="edit(row)"
            /><RowIconButton
              :icon="Delete"
              label="删除目标"
              :disabled="saving || !!deleting"
              @click="remove(row)"
            />
          </div></div
      ></template>
      <template #empty
        ><EmptyState
          size="section"
          title="暂无检测目标"
          description="填写上方信息，添加第一个目标。"
      /></template>
    </UiTable>
  </UiDrawer>
</template>
<script setup lang="ts">
import RowIconButton from "@/ui/RowIconButton.vue";
import { ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Plus, Edit, Delete } from "@element-plus/icons-vue";
import { UiDrawer, UiButton, UiTable, EmptyState } from "@/ui";
import { saveEgressTarget, deleteEgressTarget } from "@/api/subcription/node";
import type { Target } from "./useEgressTest";
const props = defineProps<{ modelValue: boolean; targets: Target[] }>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  changed: [];
}>();
const defaults = (): Target => ({
  key: "",
  name: "",
  domain: "",
  group: "ai",
  icon: "",
  path: "/cdn-cgi/trace",
  method: "GET",
  expectedStatus: "200-399",
  responseContains: "",
  requireEgressIp: true,
  timeoutSeconds: 7,
  retries: 0,
  enabled: true,
  sortOrder: (props.targets.length + 1) * 10,
});
const form = ref(defaults()),
  saving = ref(false),
  deleting = ref(0),
  error = ref("");
function newTarget() {
  form.value = defaults();
  error.value = "";
}
function edit(item: Record<string, any>) {
  form.value = { ...item } as Target;
  error.value = "";
}
async function persist() {
  if (saving.value) return;
  if (
    !form.value.key ||
    !form.value.name ||
    !form.value.domain ||
    !form.value.group
  ) {
    ElMessage.warning("请填写 key、名称、域名和分类");
    return;
  }
  saving.value = true;
  error.value = "";
  try {
    await saveEgressTarget({ ...form.value });
    ElMessage.success("检测目标已保存");
    emit("changed");
    newTarget();
  } catch (cause: any) {
    error.value = cause.message || "保存失败，请重试";
  } finally {
    saving.value = false;
  }
}
async function remove(item: Record<string, any>) {
  if (!item.id || deleting.value) return;
  try {
    await ElMessageBox.confirm(`删除检测目标「${item.name}」？`, "确认删除", {
      type: "warning",
      confirmButtonText: "删除目标",
      cancelButtonText: "取消",
    });
    deleting.value = item.id;
    await deleteEgressTarget(item.id);
    if (form.value.id === item.id) newTarget();
    ElMessage.success("已删除");
    emit("changed");
  } catch (cause: any) {
    if (cause !== "cancel" && cause !== "close")
      error.value = cause.message || "删除失败，请重试";
  } finally {
    deleting.value = 0;
  }
}
</script>
<style scoped>
.drawer-note {
  color: var(--c-muted);
  font-size: 14px;
  margin: 0 0 24px;
  line-height: 1.6;
}
.drawer-error {
  color: var(--c-bad);
  font-size: 14px;
  margin-bottom: 16px;
}
.target-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px 16px;
}
.target-form label {
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 14px;
  color: var(--c-muted);
  min-width: 0;
}
.target-form .el-input-number {
  width: 100%;
}
.full-field {
  grid-column: 1 / -1;
}
.target-switches {
  display: flex;
  flex-direction: column;
  justify-content: end;
  gap: 12px;
}
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 8px;
}
.list-title {
  font-size: 18px;
  margin: 40px 0 16px;
}
.list-title span {
  font: 400 14px var(--font-mono);
  color: var(--c-muted);
  margin-left: 12px;
}
.row-actions {
  display: flex;
}
.target-state {
  font-size: 14px;
}
.mobile-target {
  display: grid;
  grid-template-columns: 1fr auto;
  align-items: center;
  gap: 12px;
}
.mobile-target > span {
  font-size: 13px;
  color: var(--c-muted);
}
.mobile-target p {
  grid-column: 1 / -1;
  font: 400 14px var(--font-mono);
  margin: 0;
  overflow-wrap: anywhere;
}
.mobile-target .row-actions {
  grid-column: 1 / -1;
  justify-content: flex-end;
}
@media (max-width: 767px) {
  .target-form {
    grid-template-columns: 1fr;
  }
  .form-actions {
    flex-wrap: wrap;
  }
}
</style>
