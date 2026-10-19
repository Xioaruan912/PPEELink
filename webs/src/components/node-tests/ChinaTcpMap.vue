<template>
  <div class="map-shell">
    <div
      ref="root"
      class="china-map"
      role="img"
      aria-label="国内各省可达性分布"
    ></div>
    <p v-if="error" class="map-note" role="status">{{ error }}</p>
    <p v-else-if="!groups.length" class="map-note">等待各省检测结果</p>
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
import { reachColor, type ProvinceGroup } from "./reach";
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
function render() {
  if (!chart) return;
  const styles = getComputedStyle(document.documentElement);
  const color = (name: string) => styles.getPropertyValue(name).trim();
  const data = props.groups.map((group) => ({
    name: group.province,
    value: group.okCount,
    targets: group.items,
    itemStyle: {
      areaColor: reachColor(group.okCount, group.items.length, color),
    },
  }));
  chart.setOption(
    {
      animation: !reduced.value,
      animationDurationUpdate: 320,
      textStyle: {
        fontFamily: styles.getPropertyValue("--font-ui"),
        fontSize: 14,
        color: color("--c-ink"),
      },
      tooltip: {
        trigger: "item",
        backgroundColor: color("--c-surface"),
        borderColor: color("--c-line"),
        textStyle: { color: color("--c-ink"), fontSize: 14 },
        confine: true,
        formatter: (params: any) => {
          const items = params.data?.targets || [];
          const isps = [...new Set(items.map((item: any) => item.isp))];
          return (
            `<strong>${escape(params.name || "")}</strong>` +
            isps
              .map((isp) => {
                const list = items.filter((item: any) => item.isp === isp);
                const ok = list.filter((item: any) => item.ok).length;
                const state =
                  ok === list.length
                    ? "可达"
                    : ok
                      ? `${ok}/${list.length} 可达`
                      : "不可达";
                return `<br>${escape(String(isp))} · ${state} · ${escape([...new Set(list.map((item: any) => item.city))].join(" / "))}`;
              })
              .join("")
          );
        },
      },
      series: [
        {
          name: "国内可达性",
          type: "map",
          map: "node-test-china",
          roam: true,
          scaleLimit: { min: 0.9, max: 6 },
          label: { show: false },
          emphasis: {
            label: { show: true, color: color("--c-ink"), fontSize: 14 },
            itemStyle: { areaColor: color("--c-surface-2") },
          },
          itemStyle: {
            areaColor: color("--c-surface-2"),
            borderColor: color("--c-line"),
            borderWidth: 1,
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
  border-radius: 16px;
  overflow: hidden;
  background: var(--c-surface);
}
.china-map {
  width: 100%;
  height: 440px;
}
.map-note {
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: 16px;
  margin: 0;
  color: var(--c-muted);
  font-size: 14px;
  pointer-events: none;
}
@media (max-width: 767px) {
  .china-map {
    height: 300px;
  }
}
</style>
