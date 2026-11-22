<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { TraceHop, TraceView } from "@/api/network/trace";
import { Close } from "@element-plus/icons-vue";

const props = defineProps<{ views: TraceView[]; target: string; running?: boolean }>();

const colors = [
  "#15b85a",
  "#2563eb",
  "#9333ea",
  "#ea580c",
  "#db2777",
  "#0891b2",
];

const hidden = ref<number[]>([]);
const selected = ref("");
const land = ref("");
const zoom = ref(1);
const offset = ref({ x: 0, y: 0 });
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
const current = computed(() =>
  selected.value ? points.value.find((p) => p.key === selected.value) : undefined
);
const located = computed(() =>
  routes.value.reduce((sum, v) => sum + v.points.length, 0)
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
  const xs = ps.map((p) => p.x);
  const ys = ps.map((p) => p.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  zoom.value = Math.max(
    1,
    Math.min(
      6,
      Math.min(
        820 / Math.max(120, maxX - minX),
        340 / Math.max(70, maxY - minY)
      )
    )
  );
  offset.value = {
    x: (500 - (minX + maxX) / 2) * zoom.value,
    y: (250 - (minY + maxY) / 2) * zoom.value,
  };
}

function setZoom(delta: number) {
  zoom.value = Math.max(1, Math.min(8, zoom.value + delta));
}

function choose(key: string) {
  selected.value = selected.value === key ? "" : key;
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
  const bend = Math.min(36, Math.max(8, Math.abs(end - a.x) * 0.12));
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
    /* keep silent */
  }
}

onMounted(() => void loadMap());
onBeforeUnmount(() => {
  disposed = true;
  controller.abort();
});
</script>

<template>
  <section class="studio-map" aria-label="全球路由路径地图">
    <div class="map-stage" :class="{ 'is-running': running }">
      <svg
        ref="svg"
        class="world"
        :class="{ 'is-zoomed': zoom > 1 }"
        viewBox="0 0 1000 500"
        tabindex="0"
        role="group"
        aria-label="全球路由路径地图"
        @pointerdown="startPan"
        @pointermove="pan"
        @pointerup="stopPan"
        @pointercancel="stopPan"
      >
        <defs>
          <pattern
            id="trace-land-dots"
            width="5"
            height="5"
            patternUnits="userSpaceOnUse"
          >
            <circle class="land-dot" cx="2.5" cy="2.5" r="1.08" />
          </pattern>
        </defs>
        <g :transform="transform">
          <path :d="land" class="land" />
          <!-- Curved Kinetic Route Arcs -->
          <path
            v-for="line in lines"
            :key="line.key"
            :d="line.path"
            :transform="`translate(${line.shift} 0)`"
            class="route-arc"
            :class="{ 'is-gap': line.gap }"
            :stroke="line.color"
            vector-effect="non-scaling-stroke"
          />
          <!-- Interactive Hop Dots -->
          <g
            v-for="p in points"
            :key="p.key"
            class="map-hop"
            :class="{ 'is-selected': current?.key === p.key }"
            :style="{ '--hop-color': p.color }"
            tabindex="0"
            role="button"
            @click.stop="choose(p.key)"
            @keydown.enter.prevent="choose(p.key)"
          >
            <title>{{ p.source }} #{{ p.hop.ttl }} {{ p.hop.ip }}</title>
            <circle :cx="p.x" :cy="p.y" :r="15 / zoom" class="hop-hit" />
            <circle :cx="p.x" :cy="p.y" :r="9 / zoom" class="hop-halo" />
            <circle :cx="p.x" :cy="p.y" :r="4.2 / zoom" class="hop-dot" />
          </g>
        </g>
      </svg>

      <!-- Floating Top-Right Controls -->
      <div class="floating-controls">
        <button type="button" aria-label="放大" @click="setZoom(0.6)">+</button>
        <button type="button" aria-label="缩小" @click="setZoom(-0.6)">−</button>
        <button type="button" aria-label="适应路径" @click="fit">适应</button>
        <button type="button" aria-label="重置" @click="reset">全图</button>
      </div>

      <!-- Subtle Hint When No Hop Selected -->
      <div v-if="!current && !located" class="map-hint">
        <i aria-hidden="true"></i>
        <span>{{ running ? "正在追踪全球路由跳点…" : "选择目标并点击开始追踪" }}</span>
      </div>
      <div v-else-if="!current && located" class="map-hint">
        <i aria-hidden="true"></i>
        <span>点击地图跳点查看 IP / ASN / 延迟</span>
      </div>

      <!-- Floating Frosted Glass Hop Card -->
      <Transition name="hop-pop" mode="out-in">
        <aside v-if="current" :key="current.key" class="hop-card">
          <div class="hop-card-top">
            <span class="hop-ttl mono">
              <i :style="{ background: current.color }"></i>
              #{{ current.hop.ttl }} · {{ current.source }}
            </span>
            <button
              type="button"
              class="hop-close"
              aria-label="关闭跳点卡片"
              @click="selected = ''"
            >
              <el-icon><Close /></el-icon>
            </button>
          </div>
          <strong class="hop-loc">{{ location(current.hop) || "未知地区" }}</strong>
          <code class="hop-ip mono">{{ current.hop.ip }}</code>
          <div class="hop-meta mono">
            <span>{{ current.hop.asn ? `AS${current.hop.asn.replace(/^AS/i, "")}` : "—" }}</span>
            <span>{{ current.hop.owner || current.hop.isp || "—" }}</span>
            <strong class="hop-rtt">
              {{ current.hop.rtt === null ? "—" : `${current.hop.rtt.toFixed(1)} ms` }}
            </strong>
          </div>
        </aside>
      </Transition>
    </div>

    <!-- Compact Source Legend Pills -->
    <div v-if="routes.length" class="legend-rail">
      <button
        v-for="v in routes"
        :key="v.id"
        type="button"
        class="legend-pill"
        :class="{ 'is-muted': hidden.includes(v.id) }"
        @click="toggle(v.id)"
      >
        <i :style="{ background: v.color }"></i>
        <span>{{ v.sourceName }}</span>
        <small class="mono">{{ v.points.length }} 跳</small>
      </button>
    </div>
  </section>
</template>

<style scoped>
.studio-map {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.map-stage {
  position: relative;
  border: 1px solid var(--c-line);
  border-radius: 22px;
  background: var(--c-surface);
  overflow: hidden;
}

.map-stage.is-running::after {
  content: "";
  position: absolute;
  inset: 0;
  width: 24%;
  pointer-events: none;
  background: linear-gradient(90deg, transparent, rgb(21 184 90 / 0.12), transparent);
  animation: mapScan 2.2s linear infinite;
}

.world {
  display: block;
  width: 100%;
  height: clamp(380px, 52dvh, 560px);
  cursor: grab;
  touch-action: pan-y;
}

.world.is-zoomed {
  touch-action: none;
}

.world:active {
  cursor: grabbing;
}

.land {
  fill: url(#trace-land-dots);
  fill-rule: evenodd;
}

.land-dot {
  fill: color-mix(in srgb, var(--c-muted) 56%, transparent);
}

.route-arc {
  fill: none;
  stroke-width: 2;
  stroke-linecap: round;
  opacity: 0.85;
  stroke-dasharray: 6 4;
  animation: arcFlow 1.8s linear infinite;
}

.route-arc.is-gap {
  opacity: 0.45;
  stroke-dasharray: 3 6;
}

.map-hop {
  cursor: pointer;
  outline: none;
}

.hop-hit {
  fill: transparent;
}

.hop-halo {
  fill: none;
  stroke: var(--hop-color);
  stroke-width: 1.2;
  opacity: 0.28;
  vector-effect: non-scaling-stroke;
}

.hop-dot {
  fill: var(--hop-color);
  stroke: var(--c-surface);
  stroke-width: 1.4;
  vector-effect: non-scaling-stroke;
}

.map-hop.is-selected .hop-halo {
  opacity: 0.85;
  stroke-width: 2;
}

.floating-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 8;
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-surface) 90%, transparent);
  backdrop-filter: blur(14px);
  overflow: hidden;
}

.floating-controls button {
  padding: 6px 11px;
  border: 0;
  border-left: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.floating-controls button:first-child {
  border-left: 0;
}

.floating-controls button:hover {
  background: var(--c-surface-muted);
}

.map-hint {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 6;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 90%, transparent);
  backdrop-filter: blur(14px);
  font-size: 12px;
  color: var(--c-muted);
  pointer-events: none;
}

.map-hint i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.hop-card {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 10;
  width: min(320px, calc(100% - 40px));
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: color-mix(in srgb, var(--c-surface) 92%, transparent);
  backdrop-filter: blur(20px) saturate(160%);
  box-shadow: 0 20px 44px -16px rgb(0 0 0 / 0.18);
}

.hop-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.hop-ttl {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.hop-ttl i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.hop-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
}

.hop-close:hover {
  background: var(--c-surface-muted);
  color: var(--c-ink);
}

.hop-loc {
  font-size: 15.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.hop-ip {
  font-size: 12px;
  color: var(--c-muted);
}

.hop-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 8px;
  margin-top: 4px;
  border-top: 1px solid var(--c-line);
  font-size: 11.5px;
  color: var(--c-muted);
}

.hop-rtt {
  color: var(--c-ok);
}

.legend-rail {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-ink);
  cursor: pointer;
}

.legend-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.legend-pill small {
  color: var(--c-muted);
  font-size: 11px;
}

.legend-pill.is-muted {
  opacity: 0.45;
}

.hop-pop-enter-active,
.hop-pop-leave-active {
  transition: opacity 200ms var(--ease-out), transform 200ms var(--ease-out);
}

.hop-pop-enter-from,
.hop-pop-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

@keyframes arcFlow {
  to {
    stroke-dashoffset: -20;
  }
}

@keyframes mapScan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(460%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .route-arc,
  .map-stage.is-running::after {
    animation: none;
  }
}

@media (max-width: 760px) {
  .world {
    height: 300px;
  }
  .hop-card {
    position: static;
    width: 100%;
    border-radius: 0;
    border-width: 1px 0 0;
    box-shadow: none;
  }
}
</style>
