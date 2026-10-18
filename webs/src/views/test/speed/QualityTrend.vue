<template>
  <section class="quality-trend" aria-labelledby="trend-title" data-enter>
    <header class="section-heading">
      <div>
        <h2 id="trend-title">连接趋势</h2>
        <p>近 7 天 · 按时段聚合</p>
      </div>
      <UiButton :icon="Refresh" variant="ghost" :loading="loading" @click="load"
        >刷新趋势</UiButton
      >
    </header>
    <UiSkeleton v-if="loading" :lines="3" height="40px" />
    <EmptyState
      v-else-if="error || !points.length"
      size="section"
      :title="error ? '趋势暂不可用' : '暂无历史采样'"
      :description="error || '产生质量样本后，这里会显示可用率与延迟。'"
    />
    <div
      v-show="!loading && points.length"
      ref="root"
      class="trend-chart"
      role="img"
      aria-label="近七天节点可用率和平均延迟折线图"
    ></div>
  </section>
</template>
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts/core";
import {
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from "echarts/components";
import { LineChart } from "echarts/charts";
import { CanvasRenderer } from "echarts/renderers";
import { Refresh } from "@element-plus/icons-vue";
import { UiButton, UiSkeleton, EmptyState } from "@/ui";
import { useChartTheme } from "@/utils/chartTheme";
import { formatDateTime } from "@/utils/date";
import { usePrefersReducedMotion } from "@/motion";
import { token } from "@/components/node-tests/useNodeTestSession";
echarts.use([
  TooltipComponent,
  GridComponent,
  LegendComponent,
  LineChart,
  CanvasRenderer,
]);
type Point = {
  hour: string;
  samples: number;
  successes: number;
  avgRtt: number;
};
const root = ref<HTMLDivElement>();
const points = ref<Point[]>([]);
const loading = ref(false);
const error = ref("");
const theme = useChartTheme();
const reduced = usePrefersReducedMotion();
let chart: echarts.ECharts | undefined;
let resize: ResizeObserver | undefined;
let controller: AbortController | undefined;
let disposed = false;
function render() {
  if (!root.value || !points.value.length || disposed) return;
  chart ||= echarts.init(root.value);
  const t = theme.value;
  chart.setOption(
    {
      animation: !reduced.value,
      textStyle: { fontFamily: t.font, fontSize: 14, color: t.muted },
      tooltip: {
        trigger: "axis",
        confine: true,
        backgroundColor: t.surface,
        borderColor: t.line,
        textStyle: { color: t.ink, fontSize: 14 },
      },
      legend: {
        top: 0,
        left: 0,
        textStyle: { color: t.muted, fontSize: 14 },
        data: ["可用率 (%)", "平均延迟 (ms)"],
      },
      grid: { left: 0, right: 0, top: 56, bottom: 0, containLabel: true },
      xAxis: {
        type: "category",
        data: points.value.map((p) =>
          /^\d{2}$/.test(p.hour)
            ? `${p.hour}:00`
            : formatDateTime(p.hour) || p.hour
        ),
        boundaryGap: false,
        axisTick: { show: false },
        axisLine: { lineStyle: { color: t.line } },
        axisLabel: {
          color: t.muted,
          fontSize: 14,
          hideOverlap: true,
          formatter: (value: string) =>
            value.length <= 5 ? value : value.slice(5, 16),
        },
      },
      yAxis: [
        {
          type: "value",
          min: 0,
          max: 100,
          axisLabel: { color: t.muted, fontSize: 14, formatter: "{value}%" },
          splitNumber: 2,
          splitLine: { lineStyle: { color: t.grid } },
        },
        {
          type: "value",
          axisLabel: { color: t.muted, fontSize: 14, formatter: "{value} ms" },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: "可用率 (%)",
          type: "line",
          showSymbol: false,
          smooth: true,
          lineStyle: { width: 2, color: t.ok },
          itemStyle: { color: t.ok },
          data: points.value.map((p) =>
            p.samples ? Math.round((p.successes / p.samples) * 1000) / 10 : null
          ),
        },
        {
          name: "平均延迟 (ms)",
          type: "line",
          yAxisIndex: 1,
          showSymbol: false,
          smooth: true,
          lineStyle: { width: 2, color: t.data },
          itemStyle: { color: t.data },
          data: points.value.map((p) =>
            p.samples ? Math.round(p.avgRtt || 0) : null
          ),
        },
      ],
    },
    true
  );
  chart.resize();
}
async function load() {
  controller?.abort();
  const own = new AbortController();
  controller = own;
  loading.value = true;
  error.value = "";
  try {
    const response = await fetch(
      import.meta.env.VITE_APP_BASE_API +
        "/api/v1/nodes/quality/trend?hours=168",
      { headers: { Authorization: token() }, signal: own.signal }
    );
    if (!response.ok) throw new Error(`请求失败: ${response.status}`);
    const json = await response.json();
    if (json.code && json.code !== "00000")
      throw new Error(json.msg || "读取趋势失败");
    if (!disposed && !own.signal.aborted)
      points.value = Array.isArray(json.data) ? json.data : [];
  } catch (cause: any) {
    if (!disposed && !own.signal.aborted) {
      error.value = cause.message || "读取趋势失败";
      points.value = [];
    }
  } finally {
    if (!disposed && controller === own) {
      loading.value = false;
      await nextTick();
      render();
    }
  }
}
onMounted(() => {
  resize = new ResizeObserver(() => chart?.resize());
  if (root.value) resize.observe(root.value);
  void load();
});
watch([theme, reduced], render);
onBeforeUnmount(() => {
  disposed = true;
  controller?.abort();
  resize?.disconnect();
  chart?.dispose();
  chart = undefined;
});
</script>
<style scoped>
.quality-trend {
  min-width: 0;
}
.section-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
h2 {
  font-size: 17px;
  margin: 0;
}
p {
  color: var(--c-muted);
  font-size: 14px;
  margin: 8px 0 0;
}
.trend-chart {
  height: 300px;
  width: 100%;
}
@media (max-width: 767px) {
  .section-heading {
    flex-wrap: wrap;
  }
  .trend-chart {
    height: 260px;
  }
}
</style>
