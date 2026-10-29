<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { TraceHop, TraceView } from "@/api/network/trace";
import { UiButton } from "@/ui";
const props = defineProps<{ views: TraceView[]; target: string }>();
const colors = [
  "#2563eb",
  "#0d9488",
  "#9333ea",
  "#dc6b20",
  "#d43672",
  "#4f46e5",
  "#528113",
  "#087f9c",
  "#805a38",
];
const hidden = ref<number[]>([]),
  selected = ref(""),
  land = ref(""),
  mapError = ref("");
const zoom = ref(1),
  offset = ref({ x: 0, y: 0 });
const svg = ref<SVGSVGElement>();
let disposed = false;
const controller = new AbortController();
interface Point {
  key: string;
  viewId: number;
  source: string;
  hop: TraceHop;
  x: number;
  y: number;
  color: string;
  index: number;
}
function positioned(h: TraceHop): boolean {
  return (
    h.responded &&
    typeof h.latitude === "number" &&
    typeof h.longitude === "number" &&
    Number.isFinite(h.latitude) &&
    Number.isFinite(h.longitude) &&
    Math.abs(h.latitude) <= 90 &&
    Math.abs(h.longitude) <= 180 &&
    (h.latitude !== 0 || h.longitude !== 0)
  );
}
const routes = computed(() =>
  props.views.map((view, i) => ({
    ...view,
    color: colors[i % colors.length],
    points: view.hops.flatMap((hop, index): Point[] =>
      positioned(hop)
        ? [
            {
              key: `${view.id}-${index}`,
              viewId: view.id,
              source: view.sourceName,
              hop,
              index,
              color: colors[i % colors.length],
              x: ((hop.longitude! + 180) / 360) * 1000,
              y: ((90 - hop.latitude!) / 180) * 500,
            },
          ]
        : []
    ),
  }))
);
const visibleRoutes = computed(() =>
  routes.value.filter((v) => !hidden.value.includes(v.id))
);
const points = computed(() => visibleRoutes.value.flatMap((v) => v.points));
const current = computed(
  () => points.value.find((p) => p.key === selected.value) || points.value[0]
);
const located = computed(() =>
  routes.value.reduce((sum, v) => sum + v.points.length, 0)
);
const total = computed(() =>
  props.views.reduce((sum, v) => sum + v.hops.length, 0)
);
const transform = computed(
  () =>
    `translate(${offset.value.x} ${offset.value.y}) translate(500 250) scale(${zoom.value}) translate(-500 -250)`
);
const location = (h: TraceHop) =>
  [h.country, h.province, h.city]
    .filter((v, i, a) => v && a.indexOf(v) === i)
    .join(" · ");
function toggle(id: number) {
  hidden.value = hidden.value.includes(id)
    ? hidden.value.filter((v) => v !== id)
    : [...hidden.value, id];
}
function reset() {
  zoom.value = 1;
  offset.value = { x: 0, y: 0 };
}
function fit() {
  const ps = points.value;
  if (!ps.length) {
    reset();
    return;
  }
  const xs = ps.map((p) => p.x),
    ys = ps.map((p) => p.y);
  const minX = Math.min(...xs),
    maxX = Math.max(...xs),
    minY = Math.min(...ys),
    maxY = Math.max(...ys);
  zoom.value = Math.max(
    1,
    Math.min(
      7,
      Math.min(
        820 / Math.max(110, maxX - minX),
        340 / Math.max(65, maxY - minY)
      )
    )
  );
  offset.value = {
    x: (500 - (minX + maxX) / 2) * zoom.value,
    y: (250 - (minY + maxY) / 2) * zoom.value,
  };
}
function setZoom(delta: number) {
  zoom.value = Math.max(1, Math.min(10, zoom.value + delta));
}
function choose(key: string) {
  selected.value = key;
}
function segment(
  a: Point,
  b: Point
): { path: string; shift: number; gap: boolean }[] {
  let end = b.x;
  const diff = end - a.x;
  if (diff > 500) end -= 1000;
  else if (diff < -500) end += 1000;
  const mid = (a.x + end) / 2;
  const bend = Math.min(34, Math.max(7, Math.abs(end - a.x) * 0.1));
  const path = `M${a.x},${a.y} Q${mid},${(a.y + b.y) / 2 - bend} ${end},${b.y}`;
  const result = [{ path, shift: 0, gap: b.index - a.index > 1 }];
  if (end < 0) result.push({ path, shift: 1000, gap: b.index - a.index > 1 });
  if (end > 1000)
    result.push({ path, shift: -1000, gap: b.index - a.index > 1 });
  return result;
}
const lines = computed(() =>
  visibleRoutes.value.flatMap((v) =>
    v.points
      .slice(1)
      .flatMap((p, i) =>
        segment(v.points[i], p).map((line, j) => ({
          ...line,
          key: `${v.id}-${i}-${j}`,
          color: v.color,
        }))
      )
  )
);
watch(
  () => props.views,
  (next, old) => {
    if (next.map((v) => v.id).join(",") !== old?.map((v) => v.id).join(",")) {
      hidden.value = [];
      selected.value = "";
      reset();
    }
  }
);
watch(points, (ps) => {
  if (!ps.some((p) => p.key === selected.value))
    selected.value = ps[0]?.key || "";
});
let panStart: { x: number; y: number; pointer: number } | undefined;
function startPan(e: PointerEvent) {
  if ((e.target as Element).closest(".map-hop")) return;
  if (e.pointerType === "mouse" && e.button !== 0) return;
  panStart = { x: e.clientX, y: e.clientY, pointer: e.pointerId };
  (e.currentTarget as SVGSVGElement).setPointerCapture(e.pointerId);
}
function pan(e: PointerEvent) {
  if (!panStart || panStart.pointer !== e.pointerId || !svg.value) return;
  const scale = 1000 / svg.value.getBoundingClientRect().width;
  offset.value = {
    x: offset.value.x + (e.clientX - panStart.x) * scale,
    y: offset.value.y + (e.clientY - panStart.y) * scale,
  };
  panStart = { x: e.clientX, y: e.clientY, pointer: e.pointerId };
}
function stopPan(e: PointerEvent) {
  const el = e.currentTarget as SVGSVGElement;
  if (el.hasPointerCapture(e.pointerId)) el.releasePointerCapture(e.pointerId);
  panStart = undefined;
}
function onKey(e: KeyboardEvent) {
  const keys: Record<string, [number, number]> = {
    ArrowLeft: [24, 0],
    ArrowRight: [-24, 0],
    ArrowUp: [0, 24],
    ArrowDown: [0, -24],
  };
  if (keys[e.key]) {
    e.preventDefault();
    const [x, y] = keys[e.key];
    offset.value = { x: offset.value.x + x, y: offset.value.y + y };
  } else if (["+", "=", "-", "Home"].includes(e.key)) {
    e.preventDefault();
    if (e.key === "Home") fit();
    else setZoom(e.key === "-" ? -0.5 : 0.5);
  }
}
async function loadMap() {
  try {
    const response = await fetch(`${import.meta.env.BASE_URL}world.json`, {
      signal: controller.signal,
    });
    if (!response.ok) throw new Error();
    const world = await response.json();
    if (disposed) return;
    land.value = world.features
      .flatMap(
        (f: {
          geometry: {
            type: string;
            coordinates: number[][][] | number[][][][];
          };
        }) => {
          const g = f.geometry;
          const polygons =
            g.type === "Polygon"
              ? [g.coordinates as number[][][]]
              : g.type === "MultiPolygon"
                ? (g.coordinates as number[][][][])
                : [];
          return polygons.flatMap((poly) =>
            poly.map(
              (ring) =>
                ring
                  .map(
                    ([lng, lat], i) =>
                      `${i ? "L" : "M"}${(((lng + 180) / 360) * 1000).toFixed(1)},${(((90 - lat) / 180) * 500).toFixed(1)}`
                  )
                  .join("") + "Z"
            )
          );
        }
      )
      .join("");
  } catch {
    if (!disposed)
      mapError.value = "地图轮廓暂时无法加载，已定位跳点仍会显示。";
  }
}
onMounted(() => void loadMap());
onBeforeUnmount(() => {
  disposed = true;
  controller.abort();
});
</script>
<template>
  <section class="path-map" aria-label="路由地理地图">
    <div class="map-toolbar">
      <div>
        <strong>世界路径地图</strong
        ><span>{{ located }} / {{ total }} 跳已定位</span>
      </div>
      <div class="map-actions">
        <UiButton variant="ghost" aria-label="缩小地图" @click="setZoom(-0.5)"
          >−</UiButton
        ><UiButton variant="ghost" aria-label="放大地图" @click="setZoom(0.5)"
          >+</UiButton
        ><UiButton variant="ghost" @click="fit">适应路径</UiButton
        ><UiButton variant="ghost" @click="reset">世界全图</UiButton>
      </div>
    </div>
    <div class="map-stage">
      <svg
        ref="svg"
        class="world"
        :class="{ 'is-zoomed': zoom > 1 }"
        viewBox="0 0 1000 500"
        tabindex="0"
        role="group"
        aria-label="路由世界地图，可拖动，使用方向键平移、加减键缩放、Home 适应路径"
        @keydown="onKey"
        @pointerdown="startPan"
        @pointermove="pan"
        @pointerup="stopPan"
        @pointercancel="stopPan"
      >
        <defs>
          <pattern
            id="route-map-grid"
            width="83.333"
            height="83.333"
            patternUnits="userSpaceOnUse"
          >
            <path d="M83.333 0H0V83.333" class="graticule" />
          </pattern>
        </defs>
        <g :transform="transform">
          <rect
            x="0"
            y="0"
            width="1000"
            height="500"
            fill="url(#route-map-grid)"
          />
          <path :d="land" class="land" />
          <path
            v-for="line in lines"
            :key="line.key"
            :d="line.path"
            :transform="`translate(${line.shift} 0)`"
            class="route-line"
            :class="{ gap: line.gap }"
            :stroke="line.color"
            vector-effect="non-scaling-stroke"
          />
          <g
            v-for="p in points"
            :key="p.key"
            class="map-hop"
            :class="{ 'is-selected': current?.key === p.key }"
            :style="{ '--hop-color': p.color }"
            tabindex="0"
            role="button"
            :aria-label="`${p.source}第${p.hop.ttl}跳，${p.hop.ip}，${location(p.hop) || '位置未知'}`"
            @click.stop="choose(p.key)"
            @keydown.enter.prevent="choose(p.key)"
            @keydown.space.stop.prevent="choose(p.key)"
          >
            <title>
              {{ p.source }} · 第 {{ p.hop.ttl }} 跳 · {{ p.hop.ip }} ·
              {{ location(p.hop) }}
            </title>
            <circle :cx="p.x" :cy="p.y" :r="14 / zoom" class="hop-hit" />
            <circle :cx="p.x" :cy="p.y" :r="5 / zoom" class="hop-circle" />
            <text
              :x="p.x"
              :y="p.y - 10 / zoom"
              :font-size="12 / zoom"
              text-anchor="middle"
              class="hop-label"
            >
              {{ p.hop.ttl }}
            </text>
          </g>
        </g>
      </svg>
      <div v-if="!located" class="map-empty">
        <strong>{{
          total ? "本次追踪尚无可定位跳点" : "先追踪一条线路"
        }}</strong
        ><span>{{
          total
            ? "需要 NextTrace 返回地理坐标。未定位的跳点仍可在逐跳模式查看。"
            : "选择落地和来源，路径将在地图上展开。"
        }}</span>
      </div>
    </div>
    <div v-if="routes.length" class="map-legend" aria-label="显示的追踪来源">
      <button
        v-for="v in routes"
        :key="v.id"
        :aria-pressed="!hidden.includes(v.id)"
        :class="{ muted: hidden.includes(v.id) }"
        @click="toggle(v.id)"
      >
        <i :style="{ background: v.color }" /><span>{{ v.sourceName }}</span
        ><small>{{ v.points.length }} 跳定位</small>
      </button>
    </div>
    <div v-if="points.length" class="map-inspector">
      <label for="map-hop-select"
        >查看跳点<select id="map-hop-select" v-model="selected">
          <option v-for="p in points" :key="p.key" :value="p.key">
            {{ p.source }} · 第 {{ p.hop.ttl }} 跳 · {{ p.hop.ip }}
          </option>
        </select></label
      >
      <div v-if="current" class="hop-detail">
        <strong>{{ location(current.hop) || "位置未知" }}</strong
        ><code>{{ current.hop.ip }}</code
        ><span
          >{{
            current.hop.asn ? `AS${current.hop.asn.replace(/^AS/i, "")}` : ""
          }}
          {{ current.hop.owner || current.hop.isp || "" }}</span
        ><span
          >第 {{ current.hop.ttl }} 跳 ·
          {{
            current.hop.rtt === null
              ? "未返回耗时"
              : `${current.hop.rtt.toFixed(2)} ms`
          }}</span
        >
      </div>
    </div>
    <p v-if="mapError" class="map-notice" role="status">{{ mapError }}</p>
    <p class="map-note">
      位置来自 IP
      地理信息，连线按跳数顺序绘制，不代表实际光缆。虚线表示中间有未定位跳点；未定位跳点不会被放到地图上的虚构位置。
    </p>
  </section>
</template>
<style scoped>
.path-map {
  min-width: 0;
  overflow: hidden;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: 16px;
}
.map-toolbar {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--c-line);
  flex-wrap: wrap;
}
.map-toolbar > div:first-child {
  display: flex;
  align-items: baseline;
  gap: 10px;
  flex-wrap: wrap;
}
.map-toolbar strong {
  font-size: 14px;
}
.map-toolbar span {
  font-size: 12px;
  color: var(--c-muted);
}
.map-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
.map-actions :deep(.ui-button) {
  min-height: 32px;
  padding: 6px 9px;
  font-size: 12px;
}
.map-stage {
  position: relative;
  background: color-mix(in srgb, var(--c-data) 4%, var(--c-surface));
  overflow: hidden;
}
.world {
  display: block;
  width: 100%;
  height: clamp(300px, 38vw, 500px);
  cursor: grab;
  touch-action: pan-y;
  outline-offset: -3px;
}
.world.is-zoomed {
  touch-action: none;
}
.world:active {
  cursor: grabbing;
}
.graticule {
  fill: none;
  stroke: color-mix(in srgb, var(--c-data) 10%, transparent);
  stroke-width: 0.5;
}
.land {
  fill: color-mix(in srgb, var(--c-data) 10%, var(--c-surface-2));
  stroke: color-mix(in srgb, var(--c-muted) 40%, var(--c-line));
  stroke-width: 0.45;
  fill-rule: evenodd;
}
.route-line {
  fill: none;
  stroke-width: 2.2;
  stroke-linecap: round;
  opacity: 0.8;
}
.route-line.gap {
  stroke-dasharray: 5 5;
}
.map-hop {
  cursor: pointer;
  outline: none;
}
.hop-hit {
  fill: transparent;
}
.hop-circle {
  fill: var(--hop-color);
  stroke: var(--c-surface);
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}
.hop-label {
  fill: var(--c-ink);
  paint-order: stroke;
  stroke: var(--c-surface);
  stroke-width: 3;
  stroke-linejoin: round;
  font-weight: 600;
  pointer-events: none;
}
.map-hop:hover .hop-circle,
.map-hop:focus-visible .hop-circle,
.map-hop.is-selected .hop-circle {
  stroke: var(--hop-color);
  stroke-width: 4;
}
.map-empty {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 24px;
  pointer-events: none;
  text-align: center;
  background: color-mix(in srgb, var(--c-surface) 50%, transparent);
}
.map-empty strong {
  font-size: 16px;
}
.map-empty span {
  font-size: 12px;
  line-height: 1.7;
  color: var(--c-muted);
  max-width: 340px;
}
.map-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  padding: 12px 16px;
  border-top: 1px solid var(--c-line);
}
.map-legend button {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  max-width: 100%;
  font-size: 12px;
  padding: 5px 0;
}
.map-legend button > span {
  overflow-wrap: anywhere;
}
.map-legend i {
  height: 8px;
  width: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.map-legend small {
  color: var(--c-muted);
  white-space: nowrap;
}
.map-legend .muted {
  opacity: 0.45;
}
.map-inspector {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  padding: 14px 16px;
  border-top: 1px solid var(--c-line);
  align-items: start;
}
.map-inspector label {
  display: grid;
  gap: 7px;
  font-size: 12px;
  color: var(--c-muted);
  min-width: 0;
}
.map-inspector select {
  width: 100%;
  min-width: 0;
  max-width: 100%;
  padding: 8px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: 8px;
  font-size: 12px;
  color: var(--c-ink);
}
.hop-detail {
  display: grid;
  gap: 4px;
  min-width: 0;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.hop-detail code {
  white-space: normal;
  overflow-wrap: anywhere;
}
.hop-detail span {
  font-size: 11px;
  color: var(--c-muted);
}
.map-note,
.map-notice {
  font-size: 11px;
  line-height: 1.7;
  color: var(--c-muted);
  padding: 0 16px 12px;
  margin: 0;
}
.map-notice {
  color: var(--c-warn-ink);
}
.map-legend button:focus-visible,
.map-inspector select:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
@media (max-width: 600px) {
  .map-toolbar {
    padding: 12px;
  }
  .map-inspector {
    grid-template-columns: 1fr;
    padding: 12px;
  }
  .world {
    height: 300px;
  }
  .map-legend {
    padding: 10px 12px;
  }
  .map-note,
  .map-notice {
    padding-inline: 12px;
  }
}
</style>
