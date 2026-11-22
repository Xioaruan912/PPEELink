<template>
  <UiPanel title="节点分布" class="map-panel">
    <template #actions>
      <UiButton
        variant="ghost"
        :icon="Refresh"
        :loading="busy"
        aria-label="刷新节点地图"
        @click="refresh"
      />
    </template>
    <div
      ref="stage"
      class="map-stage"
      :class="{ 'is-paused': scanPaused, 'is-reduced': reduced }"
      :aria-busy="busy || geographyLoading"
    >
      <svg
        class="world-map"
        viewBox="0 15 1000 445"
        preserveAspectRatio="xMidYMid slice"
        role="group"
        tabindex="0"
        aria-label="节点世界地图，滚轮或加减键缩放，拖拽平移"
        @keydown="onKey"
        @wheel.prevent="onWheel"
        @pointerdown="startPan"
        @pointermove="pan"
        @pointerup="stopPan"
        @pointercancel="stopPan"
      >
        <defs>
          <!-- Realistic Ocean Bathymetry Gradient -->
          <radialGradient id="wm-ocean-light" cx="50%" cy="46%" r="68%">
            <stop offset="0%" stop-color="#e3eef8" />
            <stop offset="60%" stop-color="#d5e4f2" />
            <stop offset="100%" stop-color="#c4d7ea" />
          </radialGradient>
          <radialGradient id="wm-ocean-dark" cx="50%" cy="46%" r="68%">
            <stop offset="0%" stop-color="#112032" />
            <stop offset="65%" stop-color="#0b1623" />
            <stop offset="100%" stop-color="#070e17" />
          </radialGradient>

          <!-- Realistic Terrain Land Gradient -->
          <linearGradient id="wm-land-light" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#f7f5ee" />
            <stop offset="55%" stop-color="#efeadd" />
            <stop offset="100%" stop-color="#e4decb" />
          </linearGradient>
          <linearGradient id="wm-land-dark" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#223041" />
            <stop offset="60%" stop-color="#1a2533" />
            <stop offset="100%" stop-color="#141d28" />
          </linearGradient>

          <!-- Continental Shelf Relief Drop Shadow -->
          <filter id="wm-relief" x="-5%" y="-5%" width="110%" height="112%">
            <feDropShadow
              dx="0"
              dy="1.6"
              stdDeviation="1.8"
              flood-color="#1e3a5f"
              flood-opacity="0.18"
            />
          </filter>

          <!-- Node Pin Ground Shadow -->
          <filter id="wm-pin-shadow" x="-50%" y="-50%" width="200%" height="200%">
            <feDropShadow
              dx="0"
              dy="2"
              stdDeviation="2.2"
              flood-color="#000000"
              flood-opacity="0.3"
            />
          </filter>
        </defs>

        <!-- Ocean Backdrop -->
        <rect x="0" y="0" width="1000" height="500" class="ocean-bg" />

        <g
          :transform="`translate(${offset.x} ${offset.y}) translate(500 240) scale(${zoom}) translate(-500 -240)`"
        >
          <!-- Cartographic Latitude / Longitude Graticules -->
          <g class="graticules" aria-hidden="true">
            <line
              v-for="y in latLines"
              :key="`lat-${y}`"
              x1="0"
              :y1="y"
              x2="1000"
              :y2="y"
              :class="{ 'is-equator': y === 250 }"
              vector-effect="non-scaling-stroke"
            />
            <line
              v-for="x in lngLines"
              :key="`lng-${x}`"
              :x1="x"
              y1="0"
              :x2="x"
              y2="500"
              vector-effect="non-scaling-stroke"
            />
          </g>

          <!-- Cartographic Ocean Labels -->
          <g class="ocean-labels" aria-hidden="true">
            <text x="148" y="290">PACIFIC OCEAN</text>
            <text x="385" y="245">ATLANTIC OCEAN</text>
            <text x="678" y="335">INDIAN OCEAN</text>
            <text x="905" y="285">PACIFIC</text>
          </g>

          <!-- 216 Individual Topographic Countries with Relief & National Borders -->
          <g class="land-layer" filter="url(#wm-relief)">
            <path
              v-for="country in countries"
              :key="country.name"
              :d="country.d"
              class="country"
              vector-effect="non-scaling-stroke"
            >
              <title>{{ country.name }}</title>
            </path>
          </g>

          <!-- Subtle Great-Circle Fiber Mesh Arcs Connecting Active Nodes -->
          <g class="fiber-arcs" aria-hidden="true">
            <path
              v-for="arc in meshArcs"
              :key="arc.key"
              :d="arc.d"
              class="fiber-arc"
              vector-effect="non-scaling-stroke"
            />
          </g>

          <!-- Interactive 3D-Elevated Node Location Pins -->
          <g
            v-for="(point, index) in projected"
            :key="`${point.name}-${index}`"
            class="map-node"
            :class="{ 'is-selected': selectedIndex === index }"
            tabindex="0"
            role="button"
            :aria-label="`${point.name}，${point.country || '未识别地区'}${point.server ? '，' + point.server : ''}`"
            @click.stop="choosePoint(index)"
            @keydown.enter.prevent="choosePoint(index)"
            @keydown.space.stop.prevent="choosePoint(index)"
          >
            <title>
              {{ point.name }} / {{ point.country || "未识别地区" }} /
              {{ point.server || "" }}
            </title>
            <circle
              class="node-hit"
              :cx="point.x"
              :cy="point.y"
              :r="15 / zoom"
            />
            <circle
              class="node-halo"
              :cx="point.x"
              :cy="point.y"
              :r="11 / zoom"
            />
            <circle
              class="node-rim"
              :cx="point.x"
              :cy="point.y"
              :r="5.8 / zoom"
              filter="url(#wm-pin-shadow)"
            />
            <circle
              class="node-mark"
              :cx="point.x"
              :cy="point.y"
              :r="3.4 / zoom"
            />
          </g>
        </g>
      </svg>

      <UiSkeleton
        v-if="busy || geographyLoading"
        class="map-skeleton"
        :lines="2"
        height="16px"
      />
      <EmptyState
        v-if="!busy && !validPoints.length"
        size="inline"
        class="map-empty"
        title="暂无节点坐标"
      >
        <template #action>
          <UiButton @click="router.push('/subcription/nodes')">
            管理节点
          </UiButton>
        </template>
      </EmptyState>

      <!-- Floating Glass Zoom Controls -->
      <div class="map-controls">
        <div class="zoom-controls">
          <UiButton
            variant="ghost"
            :icon="Plus"
            aria-label="放大地图"
            @click="setZoom(0.4)"
          />
          <UiButton
            variant="ghost"
            :icon="Minus"
            aria-label="缩小地图"
            @click="setZoom(-0.4)"
          />
          <UiButton
            variant="ghost"
            :icon="Aim"
            aria-label="复位地图"
            @click="reset"
          />
        </div>
      </div>
    </div>
  </UiPanel>
</template>

<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Aim, Minus, Plus, Refresh } from "@element-plus/icons-vue";
import { EmptyState, UiButton, UiPanel, UiSkeleton } from "@/ui";
import { getNodeMap } from "@/api/total";
import { useMotion } from "@/motion";
import { loadWorldCountries, type CountryFeature } from "@/utils/worldGeo";

interface MapPoint {
  lat: number;
  lng: number;
  name: string;
  country: string;
  server?: string;
}

interface ProjectedPoint extends MapPoint {
  x: number;
  y: number;
}

const props = defineProps<{
  points?: MapPoint[];
  loading?: boolean;
  selectedName?: string;
}>();
const emit = defineEmits<{
  refresh: [];
  select: [point: MapPoint | undefined];
}>();

const router = useRouter();
const ownPoints = ref<MapPoint[]>([]);
const ownLoading = ref(false);
const countries = ref<CountryFeature[]>([]);
const geographyLoading = ref(true);
const stage = ref<HTMLElement>();
const { reduced } = useMotion();

const latLines = [83.3, 166.7, 250, 333.3, 416.7];
const lngLines = [166.7, 333.3, 500, 666.7, 833.3];

const busy = computed(() => Boolean(props.loading || ownLoading.value));
const sourcePoints = computed(() => props.points ?? ownPoints.value);
const validPoints = computed(() =>
  sourcePoints.value.filter(
    (p) =>
      Number.isFinite(p.lat) &&
      Number.isFinite(p.lng) &&
      Math.abs(p.lat) <= 90 &&
      Math.abs(p.lng) <= 180 &&
      (p.lat !== 0 || p.lng !== 0)
  )
);

const projected = computed<ProjectedPoint[]>(() => {
  const buckets = new Map<string, MapPoint[]>();
  for (const point of validPoints.value) {
    const key = `${point.lat.toFixed(2)},${point.lng.toFixed(2)}`;
    const list = buckets.get(key) || [];
    list.push(point);
    buckets.set(key, list);
  }
  return [...buckets.values()].flatMap((group) =>
    group.map((point, index) => {
      const angle = (index / group.length) * Math.PI * 2;
      const radius = group.length > 1 ? 6 : 0;
      return {
        ...point,
        x: ((point.lng + 180) / 360) * 1000 + Math.cos(angle) * radius,
        y: ((90 - point.lat) / 180) * 500 + Math.sin(angle) * radius,
      };
    })
  );
});

// Connect consecutive unique locations with subtle great-circle curves
const meshArcs = computed(() => {
  const pts = projected.value;
  if (pts.length < 2) return [];
  const arcs: { key: string; d: string }[] = [];
  for (let i = 0; i < pts.length - 1; i++) {
    const a = pts[i];
    const b = pts[i + 1];
    const dx = Math.abs(b.x - a.x);
    if (dx > 520) continue; // skip wrap-around lines
    const mx = (a.x + b.x) / 2;
    const my = (a.y + b.y) / 2 - Math.min(48, Math.max(14, dx * 0.16));
    arcs.push({
      key: `${i}-${i + 1}`,
      d: `M${a.x.toFixed(1)},${a.y.toFixed(1)} Q${mx.toFixed(1)},${my.toFixed(1)} ${b.x.toFixed(1)},${b.y.toFixed(1)}`,
    });
  }
  return arcs;
});

const selectedIndex = ref(-1);
function choosePoint(index: number) {
  selectedIndex.value = index;
  const pt = projected.value[index];
  if (pt) emit("select", pt);
}

watch(
  () => props.selectedName,
  (name) => {
    if (!name) {
      selectedIndex.value = -1;
      return;
    }
    selectedIndex.value = projected.value.findIndex((p) => p.name === name);
  },
  { immediate: true }
);

const zoom = ref(1.06);
const offset = ref({ x: 0, y: 0 });
let panStart: { x: number; y: number; pointer: number } | undefined;

function startPan(event: PointerEvent) {
  if ((event.target as Element).closest(".map-node")) return;
  const svg = event.currentTarget as SVGSVGElement;
  svg.setPointerCapture(event.pointerId);
  panStart = { x: event.clientX, y: event.clientY, pointer: event.pointerId };
}

function pan(event: PointerEvent) {
  if (!panStart || panStart.pointer !== event.pointerId) return;
  const svg = event.currentTarget as SVGSVGElement;
  const scale = 1000 / svg.getBoundingClientRect().width;
  offset.value = {
    x: offset.value.x + (event.clientX - panStart.x) * scale,
    y: offset.value.y + (event.clientY - panStart.y) * scale,
  };
  panStart = { x: event.clientX, y: event.clientY, pointer: event.pointerId };
}

function stopPan(event: PointerEvent) {
  const svg = event.currentTarget as SVGSVGElement;
  if (svg.hasPointerCapture(event.pointerId))
    svg.releasePointerCapture(event.pointerId);
  panStart = undefined;
}

function setZoom(delta: number) {
  zoom.value = Math.max(1, Math.min(5, Number((zoom.value + delta).toFixed(2))));
}

function onWheel(event: WheelEvent) {
  setZoom(event.deltaY < 0 ? 0.25 : -0.25);
}

function reset() {
  zoom.value = 1.06;
  offset.value = { x: 0, y: 0 };
}

function onKey(event: KeyboardEvent) {
  const steps: Record<string, [number, number]> = {
    ArrowLeft: [24, 0],
    ArrowRight: [-24, 0],
    ArrowUp: [0, 24],
    ArrowDown: [0, -24],
  };
  if (steps[event.key]) {
    event.preventDefault();
    const [x, y] = steps[event.key];
    offset.value = { x: offset.value.x + x, y: offset.value.y + y };
  } else if (["+", "=", "-", "Home"].includes(event.key)) {
    event.preventDefault();
    if (event.key === "Home") reset();
    else setZoom(event.key === "-" ? -0.3 : 0.3);
  }
}

const hidden = ref(false);
const inactive = ref(false);
const scanPaused = computed(() => reduced.value || hidden.value || inactive.value);
const syncVisibility = () => {
  hidden.value = document.hidden;
};

let disposed = false;

async function refresh() {
  if (props.points !== undefined) {
    emit("refresh");
    return;
  }
  ownLoading.value = true;
  try {
    const { data } = await getNodeMap();
    if (!disposed) ownPoints.value = Array.isArray(data) ? data : [];
  } finally {
    if (!disposed) ownLoading.value = false;
  }
}

onActivated(() => {
  inactive.value = false;
});
onDeactivated(() => {
  inactive.value = true;
});

onMounted(async () => {
  syncVisibility();
  document.addEventListener("visibilitychange", syncVisibility);
  try {
    const list = await loadWorldCountries();
    if (!disposed) countries.value = list;
  } finally {
    if (!disposed) geographyLoading.value = false;
  }
  if (props.points === undefined) void refresh();
});

onBeforeUnmount(() => {
  disposed = true;
  document.removeEventListener("visibilitychange", syncVisibility);
});
</script>

<style scoped>
.map-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.map-stage {
  position: relative;
  flex: 1;
  min-height: 360px;
  display: grid;
  align-items: center;
}

.world-map {
  width: 100%;
  height: 100%;
  min-height: 360px;
  display: block;
  cursor: grab;
  touch-action: none;
  outline: none;
}

.world-map:active {
  cursor: grabbing;
}

/* Realistic Ocean & Terrain Styling (Light + Dark Adaptive) */
.ocean-bg {
  fill: url(#wm-ocean-light);
}

:global(html.dark) .ocean-bg {
  fill: url(#wm-ocean-dark);
}

.graticules line {
  stroke: rgb(30 64 115 / 0.08);
  stroke-width: 0.7;
  stroke-dasharray: 2 4;
}

.graticules line.is-equator {
  stroke: rgb(30 64 115 / 0.16);
  stroke-dasharray: none;
}

:global(html.dark) .graticules line {
  stroke: rgb(148 184 230 / 0.08);
}

.ocean-labels text {
  fill: rgb(42 82 130 / 0.28);
  font-family: var(--font-display);
  font-size: 9.5px;
  font-style: italic;
  letter-spacing: 0.24em;
  user-select: none;
  pointer-events: none;
}

:global(html.dark) .ocean-labels text {
  fill: rgb(148 184 230 / 0.22);
}

.country {
  fill: url(#wm-land-light);
  stroke: #c6bfb0;
  stroke-width: 0.65;
  stroke-linejoin: round;
  transition: fill 160ms ease-out;
}

.country:hover {
  fill: #fbfaf5;
}

:global(html.dark) .country {
  fill: url(#wm-land-dark);
  stroke: #31445a;
}

:global(html.dark) .country:hover {
  fill: #2a3c52;
}

/* Subtle Fiber Mesh Arcs */
.fiber-arc {
  fill: none;
  stroke: rgb(21 184 90 / 0.36);
  stroke-width: 1.25;
  stroke-dasharray: 4 4;
  animation: fiberFlow 2.6s linear infinite;
}

/* 3D-Elevated Location Pins */
.map-node {
  cursor: pointer;
  outline: none;
}

.node-hit {
  fill: transparent;
}

.node-halo {
  fill: rgb(21 184 90 / 0.14);
  stroke: rgb(21 184 90 / 0.45);
  stroke-width: 1.2;
  vector-effect: non-scaling-stroke;
  opacity: 0;
  transition: opacity 220ms ease-out;
}

.map-node:hover .node-halo,
.map-node.is-selected .node-halo {
  opacity: 1;
  animation: haloPulse 1.8s ease-out infinite;
}

.node-rim {
  fill: #ffffff;
  stroke: rgb(15 23 42 / 0.22);
  stroke-width: 1;
  vector-effect: non-scaling-stroke;
}

.node-mark {
  fill: #111827;
  transition: fill 180ms ease-out;
}

.map-node:hover .node-mark,
.map-node.is-selected .node-mark {
  fill: var(--c-ok);
}

.map-skeleton,
.map-empty {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: color-mix(in srgb, var(--c-surface) 75%, transparent);
}

.map-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 8;
}

.zoom-controls {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-surface) 88%, transparent);
  backdrop-filter: blur(16px);
  overflow: hidden;
  box-shadow: 0 8px 20px -8px rgb(0 0 0 / 0.14);
}

@keyframes fiberFlow {
  to {
    stroke-dashoffset: -16;
  }
}

@keyframes haloPulse {
  0% {
    opacity: 0.75;
    stroke-width: 2;
  }
  100% {
    opacity: 0;
    stroke-width: 0.5;
  }
}

.is-paused .fiber-arc,
.is-paused .node-halo,
.is-reduced .fiber-arc,
.is-reduced .node-halo {
  animation: none;
}
</style>
