<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { formatDateTime } from "@/utils/date";
import { getNodeQualityHistory } from "@/api/subcription/node";
import UiDialog from "@/ui/UiDialog.vue";
import UiButton from "@/ui/UiButton.vue";
import UiSegmented from "@/ui/UiSegmented.vue";
import UiStat from "@/ui/UiStat.vue";
import StatusDot from "@/ui/StatusDot.vue";
import EmptyState from "@/ui/EmptyState.vue";
import { useResultMotion } from "./node-tests/useNodeTestSession";

interface Sample {
  checkedAt: string;
  success: boolean;
  rtt: number;
}
const props = defineProps<{ visible: boolean; node: any | null }>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
const loading = ref(false);
const samples = ref<Sample[]>([]);
const hours = ref<string | number>(24);
const ranges = [
  { label: "24 小时", value: 24 },
  { label: "7 天", value: 168 },
  { label: "30 天", value: 720 },
];
const { enter, leave } = useResultMotion();
let request = 0;
let disposed = false;
async function load() {
  const version = ++request;
  const id = props.node?.id;
  samples.value = [];
  if (!id || !props.visible) {
    loading.value = false;
    return;
  }
  loading.value = true;
  try {
    const response: any = await getNodeQualityHistory(id, Number(hours.value));
    if (!disposed && version === request) samples.value = response?.data || [];
  } catch {
    if (!disposed && version === request) ElMessage.error("质量历史加载失败");
  } finally {
    if (!disposed && version === request) loading.value = false;
  }
}
watch(() => [props.visible, props.node?.id, hours.value], load, {
  immediate: true,
});
onBeforeUnmount(() => {
  disposed = true;
  request++;
});
const stats = computed(() => {
  const success = samples.value.filter((item) => item.success && item.rtt >= 0);
  const sorted = success.map((item) => item.rtt).sort((a, b) => a - b);
  let failures = 0;
  for (
    let i = samples.value.length - 1;
    i >= 0 && !samples.value[i].success;
    i--
  )
    failures++;
  return {
    availability: samples.value.length
      ? Math.round((success.length * 1000) / samples.value.length) / 10
      : 0,
    p95: sorted.length ? sorted[Math.ceil(sorted.length * 0.95) - 1] : -1,
    failures,
  };
});
const points = computed(() => {
  const values = samples.value.slice(-40);
  const max = Math.max(
    500,
    ...values.filter((x) => x.success).map((x) => x.rtt)
  );
  return values
    .map((item, index) => {
      const x = values.length === 1 ? 280 : (index * 560) / (values.length - 1);
      const rtt = item.success ? Math.max(0, item.rtt) : max;
      return `${x.toFixed(1)},${(104 - (Math.min(rtt, max) / max) * 98).toFixed(1)}`;
    })
    .join(" ");
});
const scoreState = computed(() =>
  props.node?.score >= 80 ? "ok" : props.node?.score >= 60 ? "warn" : "bad"
);
const recent = computed(() => samples.value.slice(-10).reverse());
</script>

<template>
  <UiDialog
    :model-value="visible"
    title="质量详情"
    width="760px"
    align-center
    @update:model-value="emit('update:visible', $event)"
  >
    <div v-if="node" v-loading="loading" :aria-busy="loading">
      <div class="test-context">
        <strong>{{ node.name }}</strong
        ><StatusDot
          :state="scoreState"
          :label="
            node.score >= 80
              ? '质量优秀'
              : node.score >= 60
                ? '质量一般'
                : '需要关注'
          "
        />
      </div>
      <div class="test-toolbar">
        <span class="test-context">服务端检测记录</span
        ><UiSegmented v-model="hours" label="质量历史周期" :options="ranges" />
      </div>
      <div class="quality-metrics">
        <UiStat
          class="quality-score"
          label="综合评分"
          :value="node.score || 0"
          size="hero"
          :state="scoreState"
        />
        <div class="quality-secondary">
          <UiStat
            label="周期可用率"
            :value="stats.availability"
            unit="%"
            :decimals="1"
            size="small"
          />
          <UiStat
            v-if="stats.p95 >= 0"
            label="P95 延迟"
            :value="stats.p95"
            unit="ms"
            size="small"
          />
          <div v-else class="empty-p95">
            <span>P95 延迟</span><strong class="test-mono">暂无</strong>
          </div>
          <UiStat
            label="连续失败"
            :value="stats.failures"
            unit="次"
            size="small"
            :state="stats.failures ? 'bad' : 'default'"
          />
        </div>
      </div>
      <section class="test-section">
        <div class="test-section-heading">
          <h3>延迟趋势</h3>
          <span>{{ samples.length }} 个样本 · 最近 40 次</span>
        </div>
        <svg
          v-if="points"
          viewBox="0 0 560 110"
          preserveAspectRatio="none"
          class="trend-chart"
          role="img"
          aria-label="最近40次检测延迟趋势，失败记录位于顶端"
        >
          <polyline
            :points="points"
            fill="none"
            stroke="var(--c-data)"
            stroke-width="3"
            stroke-linejoin="round"
            vector-effect="non-scaling-stroke"
          />
        </svg>
        <EmptyState
          v-else
          size="section"
          title="样本正在积累"
          description="约 10 分钟后可看到检测趋势"
        />
      </section>
      <section class="test-section">
        <div class="test-section-heading">
          <h3>最近检测</h3>
          <span>最近 10 次</span>
        </div>
        <div class="quality-table-head">
          <span>检测时间</span><span>状态</span><span>延迟</span>
        </div>
        <TransitionGroup
          tag="div"
          class="quality-list"
          name="result-list"
          appear
          :css="true"
          @enter="enter"
          @leave="leave"
        >
          <article
            v-for="(row, index) in recent"
            :key="row.checkedAt"
            class="quality-row"
            :data-index="index"
          >
            <time class="test-mono" :datetime="row.checkedAt">{{
              formatDateTime(row.checkedAt) || "未知时间"
            }}</time
            ><StatusDot
              :state="row.success ? 'ok' : 'bad'"
              :label="row.success ? '正常' : '超时'"
            /><span class="test-mono">{{
              row.rtt >= 0 ? `${row.rtt} ms` : "无数据"
            }}</span>
          </article>
        </TransitionGroup>
        <EmptyState
          v-if="!recent.length"
          size="inline"
          description="暂无检测记录"
        />
      </section>
    </div>
    <template #footer
      ><UiButton @click="emit('update:visible', false)"
        >关闭</UiButton
      ></template
    >
  </UiDialog>
</template>

<style scoped lang="scss">
@use "./node-tests/dialog.scss";
.quality-metrics {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: 32px;
  padding: 24px 0;
}
.empty-p95 span {
  display: block;
  margin-bottom: 16px;
  color: var(--c-muted);
  font-size: 14px;
}
.empty-p95 strong {
  font-size: 24px;
  font-weight: 400;
}
.quality-score {
  border-right: 1px solid var(--c-line);
}
.quality-secondary {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
  align-content: center;
}
.quality-secondary > :last-child {
  grid-column: 1 / -1;
}
.trend-chart {
  display: block;
  width: 100%;
  height: 150px;
  border-bottom: 1px solid var(--c-line);
  overflow: visible;
}
.quality-row,
.quality-table-head {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 100px 100px;
  gap: 16px;
  align-items: center;
}
.quality-list {
  position: relative;
}
.quality-table-head {
  padding: 0 0 16px;
  color: var(--c-muted);
  font: 400 13px/1.5 var(--font-mono);
}
.quality-row {
  min-height: 56px;
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
  font-size: 14px;
}
.quality-row:hover {
  background: var(--c-surface-2);
}
@media (max-width: 767px) {
  .quality-metrics {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .quality-score {
    border-right: 0;
  }
  .quality-secondary {
    grid-template-columns: 1fr 1fr;
  }
  .quality-secondary > :last-child {
    grid-column: auto;
  }
  .quality-table-head {
    display: none;
  }
  .quality-row {
    grid-template-columns: 1fr auto;
    padding: 16px;
    margin-bottom: 16px;
    border: 1px solid var(--c-line);
    border-radius: 16px;
  }
  .quality-row time {
    grid-column: 1 / -1;
    overflow-wrap: anywhere;
  }
}
</style>
