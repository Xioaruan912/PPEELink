<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts/core";
import { LineChart } from "echarts/charts";
import { GridComponent, TooltipComponent } from "echarts/components";
import { CanvasRenderer } from "echarts/renderers";

echarts.use([LineChart, GridComponent, TooltipComponent, CanvasRenderer]);

export interface ChartSeries {
  name: string;
  data: number[];
  color?: string;
}

const props = withDefaults(
  defineProps<{
    title: string;
    subtitle?: string;
    labels: string[];
    series: ChartSeries[];
    unit?: string;
    max?: number;
    height?: number;
    decimals?: number;
  }>(),
  { subtitle: "", unit: "", max: undefined, height: 170, decimals: 0 }
);

const root = ref<HTMLDivElement>();
const palette = ["#9ccc3c", "#4fc3f7", "#ffb74d", "#ba68c8"];
let chart: echarts.ECharts | undefined;
let observer: ResizeObserver | undefined;

function format(value: number) {
  if (props.unit === "%") return `${value.toFixed(1)}%`;
  if (props.unit === "MB/s") return `${(value / 1024 / 1024).toFixed(1)} MB/s`;
  return `${value.toFixed(2)}${props.unit}`;
}

function render() {
  if (!chart) return;
  chart.setOption(
    {
      animation: false,
      grid: { left: 46, right: 14, top: 12, bottom: 24 },
      tooltip: {
        trigger: "axis",
        backgroundColor: "rgba(20, 24, 18, 0.92)",
        borderWidth: 0,
        textStyle: { color: "#f4f6ef", fontSize: 12 },
        valueFormatter: (value: unknown) => format(Number(value)),
      },
      xAxis: {
        type: "category",
        boundaryGap: false,
        data: props.labels,
        axisLine: { lineStyle: { color: "rgba(120, 130, 110, 0.2)" } },
        axisTick: { show: false },
        axisLabel: { color: "#8a9183", fontSize: 10, hideOverlap: true },
      },
      yAxis: {
        type: "value",
        max: props.max,
        splitLine: { lineStyle: { color: "rgba(120, 130, 110, 0.12)" } },
        axisLabel: {
          color: "#8a9183",
          fontSize: 10,
          formatter: (value: number) =>
            props.unit === "%"
              ? `${Math.round(value)}%`
              : props.unit === "MB/s"
                ? `${(value / 1024 / 1024).toFixed(1)}`
                : value.toFixed(props.decimals),
        },
      },
      series: props.series.map((item, index) => ({
        name: item.name,
        type: "line",
        smooth: true,
        showSymbol: props.labels.length <= 2,
        symbolSize: 4,
        data: item.data,
        lineStyle: { width: 2, color: item.color || palette[index % palette.length] },
        itemStyle: { color: item.color || palette[index % palette.length] },
        areaStyle: { opacity: index === 0 ? 0.08 : 0 },
      })),
    },
    true
  );
}

onMounted(() => {
  if (!root.value) return;
  chart = echarts.init(root.value);
  render();
  observer = new ResizeObserver(() => chart?.resize());
  observer.observe(root.value);
});

watch(
  () => [props.labels, props.series],
  () => render(),
  { deep: true }
);

onBeforeUnmount(() => {
  observer?.disconnect();
  chart?.dispose();
});
</script>
<template>
  <div class="metric-chart">
    <header class="chart-head">
      <span>{{ title }}</span>
      <small v-if="subtitle">{{ subtitle }}</small>
    </header>
    <div ref="root" class="chart" :style="{ height: `${height}px` }"></div>
  </div>
</template>
<style scoped>
.metric-chart {
  min-width: 0;
}
.chart-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin-bottom: 2px;
  color: var(--c-muted);
  font-size: 12px;
}
.chart-head span {
  color: var(--c-ink);
  font-weight: 500;
}
.chart {
  width: 100%;
}
</style>
