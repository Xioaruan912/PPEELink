<template>
  <UiDialog
    :model-value="visible"
    title="测试状态"
    width="520px"
    append-to-body
    :close-on-click-modal="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <template v-if="status && !failed"
      ><div class="test-context">
        <StatusDot state="warn" running label="测试进行中" /><span>{{
          status.type === "unlock" ? "解锁检测" : "国内可达性检测"
        }}</span>
      </div>
      <div class="elapsed">
        <span>已进行</span><strong class="test-mono">{{ elapsed }}</strong>
      </div>
      <dl class="status-details">
        <div>
          <dt>检测节点</dt>
          <dd>{{ status.nodeName }}</dd>
        </div>
        <div>
          <dt>开始时间</dt>
          <dd>
            <time class="test-mono" :datetime="status.startedAt">{{
              formatDateTime(status.startedAt)
            }}</time>
          </dd>
        </div>
      </dl></template
    >
    <EmptyState
      v-else
      size="section"
      :title="
        failed ? '无法读取状态' : refreshing ? '正在读取状态' : '当前没有测试'
      "
      :description="failed ? '状态读取失败，请重试' : '可从节点列表发起检测'"
    />
    <template #footer
      ><div class="test-footer">
        <UiButton v-if="failed" :loading="refreshing" @click="refresh"
          >重试</UiButton
        ><UiButton @click="emit('update:visible', false)">关闭</UiButton
        ><UiButton
          v-if="status"
          variant="danger"
          :loading="canceling"
          @click="cancel"
          >停止测试</UiButton
        >
      </div></template
    >
  </UiDialog>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { formatDateTime } from "@/utils/date";
import { GetTestStatus, CancelTest } from "@/api/subcription/node";
import UiDialog from "@/ui/UiDialog.vue";
import UiButton from "@/ui/UiButton.vue";
import StatusDot from "@/ui/StatusDot.vue";
import EmptyState from "@/ui/EmptyState.vue";
const props = defineProps<{ visible: boolean }>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
interface TestStatus {
  nodeName: string;
  nodeId: number;
  type: string;
  startedAt: string;
}
const status = ref<TestStatus | null>(null);
const canceling = ref(false);
const refreshing = ref(false);
const failed = ref(false);
const now = ref(Date.now());
let version = 0;
let disposed = false;
let poll: ReturnType<typeof setTimeout> | undefined;
let clock: ReturnType<typeof setInterval> | undefined;
const elapsed = computed(() => {
  const start = new Date(status.value?.startedAt || "").getTime();
  const seconds = Number.isFinite(start)
    ? Math.max(0, Math.floor((now.value - start) / 1000))
    : 0;
  return seconds < 60
    ? `${seconds} 秒`
    : `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`;
});
async function refresh() {
  if (disposed || !props.visible || refreshing.value) return;
  clearTimeout(poll);
  const own = version;
  refreshing.value = true;
  try {
    const response = await GetTestStatus();
    if (!disposed && own === version) {
      status.value = response?.data || null;
      failed.value = false;
      now.value = Date.now();
    }
  } catch {
    if (!disposed && own === version) failed.value = true;
  } finally {
    if (!disposed && own === version) {
      refreshing.value = false;
      if (props.visible) poll = setTimeout(refresh, 3000);
    }
  }
}
async function cancel() {
  const own = ++version;
  clearTimeout(poll);
  refreshing.value = false;
  canceling.value = true;
  try {
    await CancelTest();
    if (!disposed && own === version) {
      status.value = null;
      ElMessage.success("已停止测试");
    }
  } catch (cause: any) {
    if (!disposed && own === version)
      ElMessage.error(cause?.message || "停止失败");
  } finally {
    if (!disposed && own === version) {
      canceling.value = false;
      void refresh();
    }
  }
}
function clearTimers() {
  clearTimeout(poll);
  clearInterval(clock);
}
watch(
  () => props.visible,
  (visible) => {
    version++;
    clearTimers();
    refreshing.value = false;
    canceling.value = false;
    if (visible) {
      void refresh();
      clock = setInterval(() => {
        now.value = Date.now();
      }, 1000);
    }
  },
  { immediate: true }
);
onBeforeUnmount(() => {
  disposed = true;
  version++;
  clearTimers();
});
</script>
<style scoped lang="scss">
@use "./node-tests/dialog.scss";
.elapsed {
  margin: 20px 0;
  padding: 16px 0;
  border-bottom: 1px solid var(--c-line);
}
.elapsed > span {
  display: block;
  margin-bottom: 12px;
  color: var(--c-muted);
  font-size: 14px;
}
.elapsed strong {
  font-size: clamp(28px, 6vw, 40px);
  font-weight: 400;
}
.status-details {
  margin: 0;
}
.status-details div {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr);
  gap: 12px;
  margin: 12px 0;
  font-size: 14px;
}
.status-details dt {
  color: var(--c-muted);
}
.status-details dd {
  margin: 0;
  overflow-wrap: anywhere;
}
</style>
