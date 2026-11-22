<template>
  <div class="map-shell">
    <div
      ref="root"
      class="china-map"
      role="img"
      aria-label="国内各省 TCP Ping 延迟分布"
    ></div>
    <p v-if="error" class="map-note" role="status">{{ error }}</p>
    <p v-else-if="!groups.length" class="map-note">等待各省 TCP Ping 测速结果</p>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import * as echarts from "echarts/core";
import {
  TooltipComponent,
  GeoComponent,
  VisualMapComponent,
} from "echarts/components";
import { MapChart } from "echarts/charts";
import { CanvasRenderer } from "echarts/renderers";
import { usePrefersReducedMotion } from "@/motion";
import { type ProvinceGroup } from "./reach";

echarts.use([
  TooltipComponent,
  GeoComponent,
  VisualMapComponent,
  MapChart,
  CanvasRenderer,
]);

const props = defineProps<{ groups: ProvinceGroup[] }>();
const root = ref<HTMLDivElement>();
const error = ref("");
const reduced = usePrefersReducedMotion();
const controller = new AbortController();

let chart: echarts.ECharts | undefined;
let resize: ResizeObserver | undefined;
let theme: MutationObserver | undefined;
let disposed = false;

const escape = (value: string) =>
  value.replace(
    /[&<>"']/g,
    (char) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        char
      ]!
  );

function latencyColor(group: ProvinceGroup, isDark: boolean) {
  if (!group.okCount) return isDark ? "#b91c1c" : "#ef4444";
  const rtts = group.items
    .filter((i) => i.ok && typeof i.rtt === "number" && i.rtt >= 0)
    .map((i) => i.rtt as number);
  if (!rtts.length) return isDark ? "#15803d" : "#22c55e";
  const avg = rtts.reduce((a, b) => a + b, 0) / rtts.length;
  if (avg <= 80) return isDark ? "#15803d" : "#22c55e";
  if (avg <= 160) return isDark ? "#b45309" : "#f59e0b";
  return isDark ? "#c2410c" : "#f97316";
}

function render() {
  if (!chart) return;
  const isDark = document.documentElement.classList.contains("dark");
  const styles = getComputedStyle(document.documentElement);
  const color = (name: string) => styles.getPropertyValue(name).trim();

  const data = props.groups.map((group) => ({
    name: group.province,
    value: group.okCount,
    targets: group.items,
    itemStyle: {
      areaColor: latencyColor(group, isDark),
    },
  }));

  chart.setOption(
    {
      animation: !reduced.value,
      animationDurationUpdate: 320,
      backgroundColor: isDark
        ? new echarts.graphic.RadialGradient(0.5, 0.45, 0.7, [
            { offset: 0, color: "#112032" },
            { offset: 1, color: "#070e17" },
          ])
        : new echarts.graphic.RadialGradient(0.5, 0.45, 0.7, [
            { offset: 0, color: "#e5eff9" },
            { offset: 1, color: "#cbddf0" },
          ]),
      textStyle: {
        fontFamily: styles.getPropertyValue("--font-ui"),
        fontSize: 13,
        color: color("--c-ink"),
      },
      tooltip: {
        trigger: "item",
        backgroundColor: isDark ? "rgba(18,24,34,0.94)" : "rgba(255,255,255,0.95)",
        borderColor: color("--c-line"),
        borderWidth: 1,
        padding: [10, 14],
        textStyle: { color: color("--c-ink"), fontSize: 13 },
        confine: true,
        formatter: (params: any) => {
          const items = params.data?.targets || [];
          if (!items.length) {
            return `<strong>${escape(params.name || "")}</strong><br/><span style="opacity:.65">未测速</span>`;
          }
          const lines = items.map((item: any) => {
            const val =
              !item.ok || (typeof item.rtt === "number" && item.rtt < 0)
                ? `<span style="color:#dc2626">超时</span>`
                : typeof item.rtt === "number"
                  ? `<strong style="color:${item.rtt <= 80 ? "#16a34a" : item.rtt <= 160 ? "#d97706" : "#ea580c"}">${item.rtt} ms</strong>`
                  : `<span style="color:#16a34a">可达</span>`;
            return `${escape(item.city || "")} ${escape(item.isp || "")}：${val}`;
          });
          return `<strong>${escape(params.name || "")}</strong><br/>${lines.join("<br/>")}`;
        },
      },
      series: [
        {
          name: "国内 TCP Ping",
          type: "map",
          map: "node-test-china",
          roam: true,
          zoom: 1.18,
          scaleLimit: { min: 0.9, max: 6 },
          label: {
            show: true,
            fontSize: 9.5,
            color: isDark ? "rgba(226,232,240,0.65)" : "rgba(51,65,85,0.65)",
          },
          emphasis: {
            label: { show: true, color: color("--c-ink"), fontSize: 11.5, fontWeight: 600 },
            itemStyle: {
              areaColor: isDark ? "#2c3f56" : "#fbf9f2",
              shadowBlur: 12,
              shadowColor: "rgba(15, 23, 42, 0.25)",
            },
          },
          itemStyle: {
            areaColor: isDark ? "#1d2938" : "#f4f1e6",
            borderColor: isDark ? "#33475e" : "#c4bdad",
            borderWidth: 1,
            shadowColor: isDark ? "rgba(0,0,0,0.45)" : "rgba(30,58,95,0.16)",
            shadowBlur: 8,
            shadowOffsetY: 3,
          },
          data,
        },
      ],
    },
    false
  );
}

onMounted(async () => {
  try {
    const response = await fetch("/static/china.json", {
      signal: controller.signal,
    });
    if (!response.ok) throw new Error("地图暂不可用，检测结果仍会保留");
    const json = await response.json();
    if (disposed || !root.value) return;
    echarts.registerMap("node-test-china", json);
    chart = echarts.init(root.value);
    render();
    resize = new ResizeObserver(() => chart?.resize());
    resize.observe(root.value);
    theme = new MutationObserver(render);
    theme.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class", "style"],
    });
  } catch (cause: any) {
    if (!disposed && cause.name !== "AbortError")
      error.value = "地图暂不可用，检测结果仍会保留";
  }
});

watch(() => props.groups, render, { deep: true });
watch(reduced, render);

onBeforeUnmount(() => {
  disposed = true;
  controller.abort();
  resize?.disconnect();
  theme?.disconnect();
  chart?.dispose();
  chart = undefined;
});
</script>

<style scoped>
.map-shell {
  position: relative;
  min-width: 0;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  overflow: hidden;
  background: var(--c-surface);
}

.china-map {
  width: 100%;
  height: 480px;
}

.map-note {
  position: absolute;
  left: 16px;
  bottom: 14px;
  margin: 0;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, transparent);
  backdrop-filter: blur(12px);
  color: var(--c-muted);
  font-size: 12px;
  pointer-events: none;
}

@media (max-width: 767px) {
  .china-map {
    height: 320px;
  }
}
</style>
