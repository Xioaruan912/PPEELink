<template>
  <div class="node-ping" :aria-busy="loading">
    <header class="ping-heading">
      <p>按需探测常见目标与全部节点。</p>
      <UiButton :icon="Refresh" :loading="loading" @click="load">{{
        loaded ? "刷新响应时间" : "加载延迟数据"
      }}</UiButton>
    </header>
    <EmptyState
      v-if="error"
      size="section"
      title="探测未完成"
      :description="error"
    >
      <template #action
        ><UiButton :disabled="loading" @click="load"
          >重新加载</UiButton
        ></template
      >
    </EmptyState>
    <EmptyState
      size="section"
      v-else-if="!loaded && !loading"
      title="准备好再测"
      description="手动加载，获取当前响应时间。"
    />
    <UiSkeleton v-if="loading" :lines="4" height="48px" />
    <template v-else-if="loaded && !error">
      <section class="ping-targets" aria-labelledby="target-heading">
        <h3 id="target-heading">常见目标延迟</h3>
        <div v-if="targets.length" class="target-list">
          <div
            v-for="target in targets"
            :key="target.name + target.addr"
            class="target-item"
          >
            <strong :title="target.addr">{{ target.name }}</strong>
            <StatusDot
              :state="pingState(target.rtt)"
              :label="pingLabel(target.rtt)"
              :running="target.rtt === -2"
            />
            <UiStat
              size="medium"
              v-if="target.rtt >= 0"
              label="响应时间"
              :value="target.rtt"
              unit="ms"
              :thresholds="{ warn: 100, bad: 300 }"
            />
          </div>
        </div>
        <EmptyState
          size="section"
          v-else
          class="ping-empty"
          title="暂无目标"
          description="当前没有常见目标的探测结果。"
        />
      </section>
      <section class="ping-nodes" aria-labelledby="node-heading">
        <div class="nodes-heading">
          <h3 id="node-heading">当前节点延迟</h3>
          <span>{{ nodes.length }} 个节点</span>
        </div>
        <template v-if="nodes.length">
          <div class="chart-scroll">
            <div
              ref="chartHost"
              class="ping-chart"
              :style="{
                height: `${Math.max(220, sortedNodes.length * 40 + 48)}px`,
              }"
              role="img"
              aria-label="节点响应时间条形图，数值与完整服务器地址见下方列表"
            ></div>
          </div>
          <div class="node-list" role="list" aria-label="节点探测详情">
            <div
              v-for="node in sortedNodes"
              :key="node.name + node.server"
              class="node-item"
              role="listitem"
            >
              <div class="node-address">
                <strong>{{ node.name }}</strong
                ><span>{{ node.server }}</span>
              </div>
              <div class="node-result">
                <StatusDot
                  :state="pingState(node.rtt)"
                  :label="pingLabel(node.rtt)"
                  :running="node.rtt === -2"
                />
                <UiStat
                  size="small"
                  v-if="node.rtt >= 0"
                  label="响应时间"
                  :value="node.rtt"
                  unit="ms"
                  :thresholds="{ warn: 100, bad: 300 }"
                />
              </div>
            </div>
          </div>
        </template>
        <EmptyState
          size="section"
          v-else
          class="ping-empty"
          title="暂无节点"
          description="添加节点后可在这里查看响应时间。"
        >
          <template #action
            ><UiButton @click="router.push('/subcription/nodes')"
              >添加节点</UiButton
            ></template
          >
        </EmptyState>
      </section>
    </template>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import {
  computed,
  nextTick,
  onActivated,
  onBeforeUnmount,
  ref,
  watch,
} from "vue";
import * as echarts from "echarts/core";
import { BarChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";
import { Refresh } from "@element-plus/icons-vue";
import { getNodePing } from "@/api/total";
import { EmptyState, StatusDot, UiButton, UiSkeleton, UiStat } from "@/ui";
import { usePrefersReducedMotion } from "@/motion";
import { useChartTheme } from "@/utils/chartTheme";

const router = useRouter();
defineOptions({ name: "NodePing" });
echarts.use([BarChart, GridComponent, TooltipComponent, CanvasRenderer]);
interface PingTarget {
  name: string;
  addr: string;
  rtt: number;
}
interface NodePingItem {
  name: string;
  server: string;
  rtt: number;
}
const loading = ref(false);
const loaded = ref(false);
const error = ref("");
const targets = ref<PingTarget[]>([]);
const nodes = ref<NodePingItem[]>([]);
const sortedNodes = computed(() =>
  [...nodes.value].sort((a, b) => {
    const av = a.rtt < 0 ? Number.MAX_SAFE_INTEGER : a.rtt;
    const bv = b.rtt < 0 ? Number.MAX_SAFE_INTEGER : b.rtt;
    return av - bv;
  })
);
const pingState = (rtt: number): "idle" | "ok" | "warn" | "bad" =>
  rtt === -2
    ? "idle"
    : rtt < 0 || rtt >= 300
      ? "bad"
      : rtt < 100
        ? "ok"
        : "warn";
const pingLabel = (rtt: number) =>
  rtt === -2
    ? "测试中…"
    : rtt < 0
      ? "超时"
      : rtt < 100
        ? "响应快"
        : rtt < 300
          ? "响应偏慢"
          : "响应慢";
const theme = useChartTheme();
const reduced = usePrefersReducedMotion();
const chartHost = ref<HTMLDivElement>();
let chart: echarts.ECharts | undefined;
let observer: ResizeObserver | undefined;
let disposed = false;
function disposeChart() {
  observer?.disconnect();
  observer = undefined;
  chart?.dispose();
  chart = undefined;
}
async function renderChart() {
  await nextTick();
  if (disposed) return;
  if (!chartHost.value) {
    disposeChart();
    return;
  }
  if (chart && chart.getDom() !== chartHost.value) disposeChart();
  if (!chart) {
    chart = echarts.init(chartHost.value);
    observer = new ResizeObserver(() => chart?.resize());
    observer.observe(chartHost.value);
  }
  const palette = theme.value;
  chart.setOption(
    {
      animation: !reduced.value,
      animationDuration: 560,
      animationDurationUpdate: 320,
      textStyle: { color: palette.ink, fontFamily: palette.font, fontSize: 14 },
      grid: { left: 8, right: 64, top: 8, bottom: 32, containLabel: true },
      tooltip: {
        trigger: "item",
        renderMode: "richText",
        backgroundColor: palette.surface,
        borderColor: palette.line,
        textStyle: { color: palette.ink, fontSize: 14 },
        formatter: (params: any) => `${params.name}\n${params.data.value} ms`,
      },
      xAxis: {
        type: "value",
        min: 0,
        axisLabel: {
          color: palette.muted,
          fontFamily: palette.mono,
          fontSize: 14,
          formatter: "{value} ms",
          hideOverlap: true,
        },
        axisLine: { show: false },
        axisTick: { show: false },
        splitLine: { lineStyle: { color: palette.grid } },
      },
      yAxis: {
        type: "category",
        inverse: true,
        data: sortedNodes.value.map((node) => node.name),
        axisLabel: {
          color: palette.muted,
          fontSize: 14,
          width: 120,
          overflow: "truncate",
        },
        axisLine: { show: false },
        axisTick: { show: false },
      },
      series: [
        {
          type: "bar",
          barMaxWidth: 8,
          emphasis: { disabled: true },
          label: {
            show: true,
            position: "right",
            color: palette.ink,
            fontFamily: palette.mono,
            fontSize: 14,
            formatter: "{c} ms",
          },
          data: sortedNodes.value.map((node) => ({
            value: node.rtt < 0 ? null : node.rtt,
            itemStyle: {
              color:
                node.rtt >= 300
                  ? palette.bad
                  : node.rtt >= 100
                    ? palette.warn
                    : palette.data,
              borderRadius: 4,
            },
          })),
        },
      ],
    },
    true
  );
  chart.resize();
}
const load = async () => {
  if (loading.value) return;
  loading.value = true;
  error.value = "";
  try {
    const { data } = await getNodePing();
    if (disposed) return;
    targets.value = data?.targets || [];
    nodes.value = data?.nodes || [];
    loaded.value = true;
  } catch {
    if (disposed) return;
    targets.value = [];
    nodes.value = [];
    error.value = "延迟数据暂不可用，请重试。";
  } finally {
    if (!disposed) loading.value = false;
  }
};
watch([chartHost, sortedNodes, theme, reduced], renderChart, { flush: "post" });
onActivated(() => chart?.resize());
onBeforeUnmount(() => {
  disposed = true;
  disposeChart();
});
</script>
<style scoped>
.ping-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding-bottom: 16px;
}
.ping-heading p {
  margin: 0;
  color: var(--c-muted);
  font-size: 14px;
}
.ping-targets {
  margin-bottom: 28px;
}
h3 {
  margin: 0 0 16px;
  font-size: 14px;
  font-weight: 500;
}
.target-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 28px;
}
.target-item {
  display: grid;
  grid-template-columns: auto auto;
  gap: 12px 16px;
  padding: 12px 0;
  min-width: 160px;
}
.target-item > strong {
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.target-item > :deep(.ui-stat) {
  grid-column: 1 / -1;
}
.target-item :deep(.ui-stat__number) {
  font-size: 24px;
}
.target-item :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.nodes-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}
.nodes-heading > span {
  font: 13px/1.6 var(--font-mono);
  color: var(--c-muted);
}
.chart-scroll {
  max-height: 360px;
  overflow-y: auto;
  border-bottom: 1px solid var(--c-line);
}
.ping-chart {
  width: 100%;
  min-width: 0;
}
.node-list {
  max-height: 260px;
  overflow-y: auto;
  margin-top: 16px;
}
.node-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  min-height: 64px;
  padding: 12px 8px;
  border-bottom: 1px solid var(--c-line);
}
.node-address {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.node-address strong {
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.node-address > span {
  font: 14px/1.6 var(--font-mono);
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.node-result {
  flex-shrink: 0;
  display: grid;
  gap: 8px;
  justify-items: end;
}
.node-result :deep(.ui-stat__number) {
  font-size: 14px;
}
.node-result :deep(.ui-stat__unit) {
  font-size: 14px;
}
.node-result :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.ping-empty {
  padding: 16px 0;
}
@media (max-width: 767px) {
  .ping-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .target-list {
    gap: 12px 16px;
  }
  .target-item {
    width: 100%;
  }
  .node-item {
    gap: 12px;
    padding-inline: 0;
  }
}
</style>
