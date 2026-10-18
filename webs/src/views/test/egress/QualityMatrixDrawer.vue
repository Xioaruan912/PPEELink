<template>
  <UiDrawer
    :model-value="modelValue"
    title="场景质量矩阵"
    size="1080px"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <p class="matrix-note">
      近 24 小时 · 优先使用具体目标历史，再使用场景历史，最后回退 TCP 质量。
    </p>
    <div class="matrix-actions">
      <UiButton :loading="sampling" @click="sample('scene')"
        >立即场景采样</UiButton
      ><UiButton :loading="sampling" @click="sample('full')"
        >完整采样 {{ data.targets.length }} 个目标</UiButton
      ><UiButton :icon="Refresh" :loading="loading" @click="load"
        >刷新矩阵</UiButton
      >
    </div>
    <p class="matrix-note">自动采样每 6 小时轮换在线节点与场景代表目标。</p>
    <p v-if="error" class="matrix-error" role="alert">{{ error }}</p>
    <UiTable
      :rows="data.nodes"
      row-key="nodeId"
      :loading="loading"
      label="节点场景质量矩阵"
    >
      <el-table-column
        prop="name"
        label="节点"
        min-width="190"
      /><el-table-column
        v-for="scene in scenes"
        :key="scene"
        :label="groupLabels[scene] || scene"
        min-width="170"
        ><template #default="{ row }"
          ><div v-if="row.scenes?.[scene]?.sampleCount" class="matrix-cell">
            <b>{{ row.scenes[scene].score }} 分</b
            ><span
              >{{ row.scenes[scene].availability }}% ·
              {{
                row.scenes[scene].averageRtt >= 0
                  ? `${row.scenes[scene].averageRtt} ms`
                  : "无延迟"
              }}</span
            ><small
              >{{ row.scenes[scene].sampleCount }} 样本 · 置信
              {{ row.scenes[scene].confidence }}%</small
            >
          </div>
          <span v-else class="matrix-note">暂无样本</span></template
        ></el-table-column
      >
      <template #mobile-card="{ row }"
        ><h3 class="node-title">{{ row.name }}</h3>
        <dl class="matrix-mobile">
          <template v-for="scene in scenes" :key="scene"
            ><dt>{{ groupLabels[scene] || scene }}</dt>
            <dd v-if="row.scenes?.[scene]?.sampleCount" class="matrix-cell">
              <b>{{ row.scenes[scene].score }} 分</b
              ><span
                >{{ row.scenes[scene].availability }}% ·
                {{
                  row.scenes[scene].averageRtt >= 0
                    ? `${row.scenes[scene].averageRtt} ms`
                    : "无延迟"
                }}</span
              ><small
                >{{ row.scenes[scene].sampleCount }} 样本 · 置信
                {{ row.scenes[scene].confidence }}%</small
              >
            </dd>
            <dd v-else>暂无样本</dd></template
          >
        </dl></template
      >
      <template #empty
        ><EmptyState
          size="section"
          title="暂无质量样本"
          description="发起场景采样后，可在任务中心查看进度。"
      /></template>
    </UiTable>
  </UiDrawer>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import { UiDrawer, UiButton, UiTable, EmptyState } from "@/ui";
import { getNodeQualityMatrix } from "@/api/subcription/node";
import { startQualityMatrixSample } from "@/api/task";
import { groupLabels } from "./useEgressTest";
const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const loading = ref(false),
  sampling = ref(false),
  error = ref("");
const data = ref<{ targets: any[]; nodes: any[] }>({ targets: [], nodes: [] });
const scenes = computed(() => [
  ...new Set<string>(
    data.value.targets.map((item) => String(item.group || "")).filter(Boolean)
  ),
]);
let disposed = false;
async function load() {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  try {
    const response = await getNodeQualityMatrix(24);
    if (!disposed) data.value = response.data || { targets: [], nodes: [] };
  } catch (cause: any) {
    if (!disposed) error.value = cause.message || "矩阵读取失败";
  } finally {
    if (!disposed) loading.value = false;
  }
}
async function sample(mode: "scene" | "full") {
  if (sampling.value) return;
  try {
    if (mode === "full")
      await ElMessageBox.confirm(
        "完整采样会对每个当前在线节点检测全部启用目标，耗时和网络请求明显高于场景采样。继续？",
        "完整质量采样",
        {
          type: "warning",
          confirmButtonText: "开始采样",
          cancelButtonText: "取消",
        }
      );
    sampling.value = true;
    const response = await startQualityMatrixSample(mode);
    if (!disposed)
      ElMessage.success(
        `采样任务已创建 #${response.data?.taskId || ""}，可在任务中心查看进度`
      );
  } catch (cause: any) {
    if (!disposed && cause !== "cancel" && cause !== "close")
      error.value = cause.message || "采样失败";
  } finally {
    if (!disposed) sampling.value = false;
  }
}
watch(
  () => props.modelValue,
  (value) => {
    if (value) void load();
  },
  { immediate: true }
);
onBeforeUnmount(() => {
  disposed = true;
});
</script>
<style scoped>
.matrix-note {
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.6;
}
.matrix-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin: 16px 0;
}
.matrix-error {
  color: var(--c-bad);
  font-size: 14px;
}
.matrix-cell {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 8px 0;
}
.matrix-cell b {
  font: 400 20px var(--font-mono);
}
.matrix-cell span,
.matrix-cell small {
  color: var(--c-muted);
  font-size: 14px;
}
.node-title {
  font-size: 14px;
  margin: 0 0 12px;
}
.matrix-mobile {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  gap: 12px;
  margin: 0;
}
.matrix-mobile dt {
  font-size: 14px;
  color: var(--c-muted);
}
.matrix-mobile dd {
  margin: 0;
  padding: 0;
  font-size: 14px;
}
</style>
