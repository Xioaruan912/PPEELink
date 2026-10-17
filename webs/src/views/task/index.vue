<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref } from "vue";
import { Close, RefreshRight, View } from "@element-plus/icons-vue";
import { cancelTask, getTasks, retryTask } from "@/api/task";
import {
  EmptyState,
  PageHeader,
  StatusDot,
  UiButton,
  UiDrawer,
  UiSegmented,
  UiSkeleton,
  UiStat,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useMotion, useStagger } from "@/motion";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "TaskCenter" });
const tasks = ref<any[]>([]);
const loading = ref(false);
const loadError = ref("");
const filter = ref<string | number>("all");
const detailVisible = ref(false);
const detailId = ref<number>();
const pending = ref<number[]>([]);
const list = ref<HTMLElement>();
const { reduced } = useMotion();
const stagger = useStagger();
let timer: number | undefined;
let disposed = false;
let fetching = false;
let revealed = false;
const active = computed(() => tasks.value.filter(isActive).length);
const failed = computed(
  () => tasks.value.filter((item) => item.status === "failed").length
);
const filtered = computed(() =>
  tasks.value.filter(
    (item) =>
      filter.value === "all" ||
      (filter.value === "active"
        ? isActive(item)
        : item.status === filter.value)
  )
);
const detailTask = computed(() =>
  tasks.value.find((item) => item.id === detailId.value)
);
const filters = [
  { label: "全部", value: "all" },
  { label: "进行中", value: "active" },
  { label: "失败", value: "failed" },
  { label: "成功", value: "success" },
  { label: "已取消", value: "cancelled" },
];
const statusNames: Record<string, string> = {
  queued: "排队中",
  running: "运行中",
  success: "成功",
  failed: "失败",
  cancelled: "已取消",
};
const fieldNames: Record<string, string> = {
  id: "任务编号",
  name: "名称",
  type: "类型",
  status: "状态",
  progress: "进度",
  message: "消息",
  retryOf: "重试来源",
  createdAt: "创建时间",
  startedAt: "开始时间",
  finishedAt: "完成时间",
};
const detailFields = computed(() =>
  Object.entries(detailTask.value || {}).filter(
    ([key]) => !["error", "resultJson"].includes(key)
  )
);
function isActive(item: any) {
  return item.status === "running" || item.status === "queued";
}
const state = (status: string): "ok" | "warn" | "bad" | "idle" =>
  status === "running"
    ? "ok"
    : status === "failed"
      ? "bad"
      : status === "queued"
        ? "warn"
        : "idle";
const progress = (item: any) =>
  Math.max(0, Math.min(100, Number(item.progress) || 0));
const elapsed = (item: any) => {
  if (!item.startedAt) return "待开始";
  const seconds = Math.max(
    0,
    Math.round(
      ((item.finishedAt ? new Date(item.finishedAt).getTime() : Date.now()) -
        new Date(item.startedAt).getTime()) /
        1000
    )
  );
  return seconds < 60
    ? `${seconds}s`
    : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
};
const prettyResult = (raw: unknown) => {
  if (!raw) return "暂无结果";
  try {
    return JSON.stringify(
      typeof raw === "string" ? JSON.parse(raw) : raw,
      null,
      2
    );
  } catch {
    return String(raw);
  }
};
const formatField = (key: string, value: any) =>
  key === "status"
    ? statusNames[value] || value
    : key === "progress"
      ? `${value}%`
      : key.endsWith("At") && value
        ? formatDateTime(value) || "未记录"
        : (value ?? "暂无");
async function load(quiet = false) {
  if (fetching || disposed) return;
  fetching = true;
  if (!quiet) loading.value = true;
  try {
    const { data } = await getTasks(150);
    if (disposed) return;
    tasks.value = data || [];
    loadError.value = "";
  } catch {
    if (!disposed) loadError.value = "任务读取失败，请重新加载。";
  } finally {
    fetching = false;
    if (!disposed) {
      loading.value = false;
      await nextTick();
      if (!disposed && !revealed && list.value) {
        stagger(list.value.querySelectorAll(".task-row"));
        revealed = true;
      }
    }
  }
}
async function action(row: any, kind: "cancel" | "retry") {
  if (pending.value.includes(row.id)) return;
  if (kind === "cancel") {
    try {
      await ElMessageBox.confirm(`取消任务「${row.name}」？`, "取消任务", {
        type: "warning",
        confirmButtonText: "取消任务",
        cancelButtonText: "继续运行",
      });
    } catch {
      return;
    }
  }
  pending.value.push(row.id);
  try {
    await (kind === "cancel" ? cancelTask(row.id) : retryTask(row.id));
    ElMessage.success(kind === "cancel" ? "已请求取消" : "已创建重试任务");
    await load(true);
  } catch {
    /* API 层显示失败信息，保留当前任务以便重试。 */
  } finally {
    pending.value = pending.value.filter((id) => id !== row.id);
  }
}
function showDetail(row: any) {
  detailId.value = row.id;
  detailVisible.value = true;
}
onMounted(async () => {
  await load();
  if (!disposed)
    timer = window.setInterval(() => {
      void load(true);
    }, 3000);
});
onUnmounted(() => {
  disposed = true;
  if (timer !== undefined) window.clearInterval(timer);
});
</script>

<template>
  <div class="task-page">
    <PageHeader
      title="后台任务"
      description="查看任务进度、执行结果与重试记录。"
      ><template #actions
        ><UiButton :icon="RefreshRight" :loading="loading" @click="load()"
          >刷新任务</UiButton
        ></template
      ></PageHeader
    >
    <section class="task-summary" data-enter>
      <UiStat
        size="hero"
        label="进行中的任务"
        :value="loading ? null : active"
      />
      <div class="summary-side">
        <UiStat
          size="small"
          label="失败任务"
          :value="loading ? null : failed"
          state="bad"
        /><UiStat
          size="small"
          label="历史记录"
          :value="loading ? null : tasks.length"
        />
        <p>每 3 秒自动更新 · 最多显示 150 条</p>
      </div>
    </section>
    <div class="task-toolbar" data-enter>
      <h2>执行记录</h2>
      <UiSegmented v-model="filter" :options="filters" label="任务状态筛选" />
    </div>
    <EmptyState
      v-if="loadError"
      size="section"
      title="任务暂不可用"
      :description="loadError"
      ><template #action
        ><UiButton @click="load()">重新加载</UiButton></template
      ></EmptyState
    >
    <UiSkeleton v-if="loading" :lines="5" height="80px" />
    <div v-else ref="list">
      <EmptyState
        v-if="!tasks.length && !loadError"
        size="page"
        title="暂无后台任务"
        description="启动测活、同步或发布后，可在此跟踪。"
      />
      <EmptyState
        v-else-if="tasks.length && !filtered.length"
        size="section"
        title="没有匹配任务"
        description="切换状态查看其他执行记录。"
      />
      <div v-if="filtered.length" class="task-heading" aria-hidden="true">
        <span>任务 / 状态</span><span>进度 / 消息</span><span>耗时 / 错误</span
        ><span>操作</span>
      </div>
      <TransitionGroup
        tag="div"
        :name="reduced ? '' : 'task'"
        class="task-list"
      >
        <article v-for="task in filtered" :key="task.id" class="task-row">
          <div class="task-identity">
            <button class="task-name" @click="showDetail(task)">
              {{ task.name }}</button
            ><span class="task-meta">#{{ task.id }} · {{ task.type }}</span
            ><StatusDot
              :state="state(task.status)"
              :running="task.status === 'running'"
              :label="statusNames[task.status] || task.status"
            />
          </div>
          <div class="task-progress">
            <div class="progress-caption">
              <span>进度</span><span>{{ progress(task) }}%</span>
            </div>
            <div
              class="progress-track"
              role="progressbar"
              :aria-label="`${task.name} 进度`"
              :aria-valuenow="progress(task)"
              :aria-valuemin="0"
              :aria-valuemax="100"
            >
              <span
                :class="{ failed: task.status === 'failed' }"
                :style="{ transform: `scaleX(${progress(task) / 100})` }"
              ></span>
            </div>
            <p>{{ task.message || "暂无执行消息" }}</p>
          </div>
          <div class="task-outcome">
            <span class="mono">{{ elapsed(task) }}</span>
            <p :class="{ 'error-text': task.error }">
              {{ task.error || "暂无错误" }}
            </p>
          </div>
          <div class="task-actions">
            <RowIconButton
              :icon="View"
              :label="`查看 ${task.name} 详情`"
              @click="showDetail(task)"
            /><RowIconButton
              v-if="isActive(task)"
              :icon="Close"
              :label="`取消 ${task.name}`"
              :loading="pending.includes(task.id)"
              @click="action(task, 'cancel')"
            /><RowIconButton
              v-else
              :icon="RefreshRight"
              :label="`重试 ${task.name}`"
              :loading="pending.includes(task.id)"
              @click="action(task, 'retry')"
            />
          </div>
        </article>
      </TransitionGroup>
    </div>
    <UiDrawer
      v-model="detailVisible"
      :title="`任务 #${detailId || ''}`"
      size="760px"
      ><template v-if="detailTask"
        ><h3 class="detail-name">{{ detailTask.name }}</h3>
        <dl class="task-fields">
          <template v-for="[key, value] in detailFields" :key="key"
            ><dt>{{ fieldNames[key] || key }}</dt>
            <dd>{{ formatField(key, value) }}</dd></template
          >
          <dt>耗时</dt>
          <dd>{{ elapsed(detailTask) }}</dd>
        </dl>
        <h3>错误</h3>
        <p class="error-text detail-error">
          {{ detailTask.error || "暂无错误" }}
        </p>
        <h3>执行结果</h3>
        <pre class="task-result">{{
          prettyResult(detailTask.resultJson)
        }}</pre></template
      ><EmptyState
        v-else
        size="section"
        title="记录暂不可用"
        description="任务已不在最近 150 条记录中。"
      /><template #footer
        ><UiButton @click="detailVisible = false">关闭详情</UiButton></template
      ></UiDrawer
    >
  </div>
</template>

<style scoped>
.task-page {
  min-width: 0;
}
.task-summary {
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) minmax(0, 1fr);
  gap: 32px;
  padding-bottom: 48px;
}
.summary-side {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  align-content: end;
}
.summary-side p {
  grid-column: 1 / -1;
  margin: 8px 0 0;
  color: var(--c-muted);
  font-size: 14px;
}
.task-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  padding: 24px 0;
  border-top: 1px solid var(--c-line);
}
h2 {
  margin: 0;
  font-size: 22px;
}
.task-heading,
.task-row {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 1.4fr) minmax(0, 1fr) 88px;
  gap: 24px;
}
.task-heading {
  padding: 16px 0;
  color: var(--c-muted);
  font: 400 13px/1.4 var(--font-mono);
}
.task-row {
  padding: 24px 0;
  min-height: 120px;
  align-items: center;
  border-bottom: 1px solid var(--c-line);
  transition: background var(--duration-fast);
}
.task-row:hover {
  background: var(--c-surface-2);
}
.task-identity {
  min-width: 0;
  display: flex;
  align-items: flex-start;
  flex-direction: column;
  gap: 8px;
}
.task-name {
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--c-ink);
  text-align: left;
  font: 600 16px/1.4 var(--font-ui);
  overflow-wrap: anywhere;
  cursor: pointer;
}
.task-meta,
.mono {
  font: 400 13px/1.5 var(--font-mono);
}
.task-meta {
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.progress-caption {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  color: var(--c-muted);
  font: 400 13px/1.4 var(--font-mono);
}
.progress-track {
  margin: 8px 0;
  height: 4px;
  background: var(--c-line);
  overflow: hidden;
  border-radius: 999px;
}
.progress-track span {
  display: block;
  height: 100%;
  background: var(--c-data);
  transform-origin: left;
  transition: transform var(--duration-medium) var(--ease-out);
}
.progress-track .failed {
  background: var(--c-bad);
}
.task-progress p,
.task-outcome p {
  font-size: 14px;
  margin: 8px 0 0;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.task-outcome .error-text,
.error-text {
  color: var(--c-bad);
}
.task-actions {
  display: flex;
  align-items: center;
}
.task-fields {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 16px;
  padding: 24px 0;
  border-block: 1px solid var(--c-line);
}
.task-fields dt {
  color: var(--c-muted);
  font-size: 14px;
}
.task-fields dd {
  margin: 0;
  overflow-wrap: anywhere;
  font-family: var(--font-mono);
  font-size: 14px;
}
.detail-name {
  font-size: 22px;
  overflow-wrap: anywhere;
}
.detail-error {
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.task-result {
  max-height: 50vh;
  overflow: auto;
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface-2);
  color: var(--c-data);
  font: 400 14px/1.6 var(--font-mono);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.task-move,
.task-enter-active,
.task-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-spring),
    opacity var(--duration-medium);
}
.task-enter-from,
.task-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
.task-leave-active {
  position: absolute;
}
@media (max-width: 1023px) {
  .task-toolbar {
    align-items: flex-start;
    flex-direction: column;
  }
  .task-heading,
  .task-row {
    gap: 16px;
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) minmax(0, 0.8fr) 88px;
  }
}
@media (max-width: 767px) {
  .task-summary {
    grid-template-columns: minmax(0, 1fr);
    gap: 24px;
    padding-bottom: 32px;
  }
  .task-heading {
    display: none;
  }
  .task-row {
    grid-template-columns: minmax(0, 1fr) 88px;
    gap: 24px 16px;
  }
  .task-identity {
    grid-column: 1;
  }
  .task-actions {
    grid-column: 2;
    grid-row: 1;
    align-self: start;
  }
  .task-progress,
  .task-outcome {
    grid-column: 1 / -1;
  }
  .task-outcome {
    display: flex;
    gap: 16px;
    align-items: baseline;
  }
  .task-outcome p {
    margin: 0;
  }
  .task-fields {
    grid-template-columns: 88px minmax(0, 1fr);
  }
}
</style>
