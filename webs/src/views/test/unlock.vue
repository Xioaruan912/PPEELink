<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import { ElMessage } from "element-plus";
import { PageHeader, UiButton, UiSkeleton, StatusDot, EmptyState } from "@/ui";
import { getNodeOverview, UnlockTest } from "@/api/subcription/node";
import UnlockResults from "@/components/node-tests/UnlockResults.vue";
import type { UnlockResult } from "@/components/node-tests/unlock";
defineOptions({ name: "Unlock" });
interface NodeItem {
  id: number;
  name: string;
  link: string;
  valid: boolean;
}
const nodes = ref<NodeItem[]>([]),
  selectedNodeId = ref<number>();
const loading = ref(false),
  nodesLoading = ref(true),
  nodesError = ref("");
const result = ref<UnlockResult | null>(null),
  testError = ref("");
let alive = true;
async function loadNodes() {
  nodesLoading.value = true;
  nodesError.value = "";
  try {
    const { data } = await getNodeOverview();
    if (alive)
      nodes.value = (data || [])
        .filter((node: NodeItem) => node.valid)
        .sort((a: NodeItem, b: NodeItem) =>
          a.name.localeCompare(b.name, "zh-CN")
        );
  } catch {
    if (alive) nodesError.value = "无法加载节点，请重试。";
  } finally {
    if (alive) nodesLoading.value = false;
  }
}
async function startTest() {
  if (loading.value) return;
  if (!selectedNodeId.value) {
    ElMessage.warning("请先选择节点");
    return;
  }
  loading.value = true;
  result.value = null;
  testError.value = "";
  const id = selectedNodeId.value;
  try {
    const { data } = await UnlockTest({ id });
    if (alive) result.value = data;
  } catch (cause: any) {
    if (alive) {
      testError.value = cause?.message || "解锁检测失败，请重试。";
      ElMessage.error(testError.value);
    }
  } finally {
    if (alive) loading.value = false;
  }
}
watch(selectedNodeId, () => {
  result.value = null;
  testError.value = "";
});
onMounted(loadNodes);
onBeforeUnmount(() => {
  alive = false;
});
</script>
<template>
  <div class="unlock-page">
    <PageHeader
      title="解锁检测"
      description="检查节点的 AI、影视与论坛可用性。"
    />
    <section class="test-controls" data-enter>
      <div class="node-picker">
        <label for="unlock-node">选择节点</label
        ><el-select
          id="unlock-node"
          v-model="selectedNodeId"
          filterable
          placeholder="搜索可访问节点"
          :loading="nodesLoading"
          :disabled="nodesLoading || loading"
          aria-label="选择要检测的节点"
          ><el-option
            v-for="node in nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
            ><span class="node-option"
              ><span>{{ node.name }}</span></span
            ></el-option
          ></el-select
        >
      </div>
      <UiButton
        variant="primary"
        :loading="loading"
        :disabled="nodesLoading || !!nodesError || !nodes.length"
        @click="startTest"
        >{{ result ? "重新检测" : "开始检测" }}</UiButton
      >
      <StatusDot
        :state="loading ? 'warn' : result ? (result.ok ? 'ok' : 'bad') : 'idle'"
        :running="loading"
        :label="
          loading
            ? '正在检测服务'
            : result
              ? result.ok
                ? '有可解锁服务'
                : '无可解锁服务'
              : '尚未检测'
        "
      />
    </section>
    <EmptyState
      v-if="nodesError"
      size="section"
      title="无法加载节点"
      :description="nodesError"
      ><template #action
        ><UiButton @click="loadNodes">重新加载</UiButton></template
      ></EmptyState
    >
    <section
      v-else
      class="results-region"
      :aria-busy="loading || nodesLoading"
      data-enter
    >
      <template v-if="loading"
        ><p class="waiting-hint" role="status">
          等待服务检测完成，结果将统一显示。
        </p>
        <UiSkeleton :lines="4" height="64px"
      /></template>
      <UiSkeleton v-else-if="nodesLoading" :lines="3" height="64px" />
      <EmptyState
        v-else-if="testError"
        size="section"
        title="检测未完成"
        :description="testError"
        ><template #action
          ><UiButton @click="startTest">重新检测</UiButton></template
        ></EmptyState
      >
      <template v-else-if="result"
        ><header class="result-heading">
          <h2>
            {{
              result.nodeName ||
              nodes.find((node) => node.id === selectedNodeId)?.name
            }}
          </h2>
          <span class="mono"
            >{{ result.results.filter((item) => item.ok).length }} /
            {{ result.results.length }} 项可用</span
          >
        </header>
        <p v-if="result.error" class="error" role="alert">{{ result.error }}</p>
        <UnlockResults :result="result"
      /></template>
      <EmptyState
        v-else-if="!nodes.length"
        size="section"
        title="暂无可访问节点"
        description="请先添加节点或检查连接。"
        ><template #action
          ><UiButton @click="loadNodes">刷新节点</UiButton></template
        ></EmptyState
      >
      <EmptyState
        v-else
        size="section"
        title="选择节点开始检测"
        description="检测结果会按服务分组显示。"
      />
    </section>
  </div>
</template>
<style scoped>
.unlock-page {
  min-width: 0;
}
.test-controls {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.node-picker {
  width: 360px;
  max-width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.node-picker label {
  font-size: 14px;
}
.node-picker :deep(.el-select) {
  width: 100%;
}
.test-controls :deep(.status-dot) {
  min-height: 36px;
}
.node-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.node-option > span:first-child {
  overflow: hidden;
  text-overflow: ellipsis;
}
.node-option .mono {
  color: var(--c-muted);
  flex-shrink: 0;
}
.mono {
  font: 400 14px/1.6 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.results-region {
  min-height: 320px;
  padding-top: 20px;
}
.waiting-hint {
  margin: 0 0 16px;
  font-size: 14px;
  color: var(--c-muted);
}
.result-heading {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
}
.result-heading h2 {
  margin: 0;
  font: 600 17px/1.4 var(--font-ui);
  overflow-wrap: anywhere;
}
.result-heading .mono {
  color: var(--c-muted);
}
.error {
  color: var(--c-bad);
  overflow-wrap: anywhere;
}
@media (max-width: 767px) {
  .test-controls {
    align-items: stretch;
    gap: 12px;
  }
  .node-picker {
    width: 100%;
  }
}
</style>
