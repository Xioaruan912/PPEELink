<template>
  <UiPanel title="节点分布" class="map-panel">
    <template #actions>
      <el-tooltip
        content="刷新节点地图"
        :trigger="['hover', 'focus']"
        :trigger-keys="[]"
        ><UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="busy"
          aria-label="刷新节点地图"
          @click="refresh"
      /></el-tooltip>
    </template>
    <div
      ref="stage"
      class="map-stage"
      :class="{ 'is-paused': scanPaused, 'is-reduced': reduced }"
      :aria-busy="busy || geographyLoading"
    >
      <svg
        class="world-map"
        viewBox="0 0 1000 500"
        role="group"
        tabindex="0"
        aria-label="节点世界地图，方向键平移，加减键缩放，空格暂停或播放，Home 复位"
        @keydown="onKey"
        @pointerdown="startPan"
        @pointermove="pan"
        @pointerup="stopPan"
        @pointercancel="stopPan"
      >
        <g
          :transform="`translate(${offset.x} ${offset.y}) translate(500 250) scale(${zoom}) translate(-500 -250)`"
        >
          <path class="land" :d="landPath" />
          <path class="graticule" :d="graticule" aria-hidden="true" />
          <g class="scan-lines" aria-hidden="true"><path :d="graticule" /></g>
          <rect
            class="scan-front"
            x="0"
            y="56"
            width="2"
            height="388"
            aria-hidden="true"
          />
          <g ref="leaders" class="map-leaders" aria-hidden="true">
            <path
              v-for="(point, index) in projected"
              :key="index"
              :d="leaderPath(point, index)"
            />
          </g>
          <g
            v-for="(point, index) in projected"
            :key="`${point.name}-${index}`"
            class="map-node"
            :class="{ 'is-selected': selectedIndex === index }"
            tabindex="0"
            role="button"
            :aria-label="`${point.name}，${point.country || '未识别地区'}${point.server ? '，' + point.server : ''}`"
            @mouseenter="selectedIndex = index"
            @focus="selectedIndex = index"
            @click="selectedIndex = index"
            @keydown.enter.prevent="selectedIndex = index"
            @keydown.space.stop.prevent="selectedIndex = index"
          >
            <title>
              {{ point.name }} / {{ point.country || "未识别地区" }} /
              {{ point.server || "" }}
            </title>
            <circle
              class="node-hit"
              :cx="point.x"
              :cy="point.y"
              r="1"
              vector-effect="non-scaling-stroke"
            />
            <circle class="node-halo" :cx="point.x" :cy="point.y" r="9" />
            <circle class="node-mark" :cx="point.x" :cy="point.y" r="3.5" />
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
        <template #action
          ><UiButton @click="router.push('/subcription/nodes')"
            >管理节点</UiButton
          ></template
        >
      </EmptyState>
      <div class="map-controls">
        <div class="zoom-controls">
          <el-tooltip
            content="放大地图"
            :trigger="['hover', 'focus']"
            :trigger-keys="[]"
            ><UiButton
              variant="ghost"
              :icon="Plus"
              aria-label="放大地图"
              @click="setZoom(0.25)"
          /></el-tooltip>
          <el-tooltip
            content="缩小地图"
            :trigger="['hover', 'focus']"
            :trigger-keys="[]"
            ><UiButton
              variant="ghost"
              :icon="Minus"
              aria-label="缩小地图"
              @click="setZoom(-0.25)"
          /></el-tooltip>
          <el-tooltip
            content="复位地图"
            :trigger="['hover', 'focus']"
            :trigger-keys="[]"
            ><UiButton
              variant="ghost"
              :icon="Aim"
              aria-label="复位地图"
              @click="reset"
          /></el-tooltip>
        </div>
        <el-tooltip
          :content="playing && !reduced ? '暂停地图动效' : '播放地图动效'"
          :trigger="['hover', 'focus']"
          :trigger-keys="[]"
          ><UiButton
            variant="secondary"
            :icon="playing && !reduced ? VideoPause : VideoPlay"
            :disabled="reduced"
            :aria-label="playing && !reduced ? '暂停地图动效' : '播放地图动效'"
            :aria-pressed="playing && !reduced"
            @click="togglePlayback"
        /></el-tooltip>
      </div>
    </div>
    <el-select
      v-if="validPoints.length && !busy"
      v-model="selectedIndex"
      class="map-selector"
      aria-label="选择地图节点"
      filterable
      @change="reset"
    >
      <el-option
        v-for="(point, index) in projected"
        :key="index"
        :value="index"
        :label="`${point.name} / ${point.country || '未识别地区'}${point.server ? ' / ' + point.server : ''}`"
      />
    </el-select>
    <div v-if="selected && !busy" class="map-location" aria-live="polite">
      <Location aria-hidden="true" />
      <div>
        <strong>{{ selected.country || "未识别地区" }}</strong
        ><span
          >{{ selected.name
          }}<template v-if="selected.server">
            / {{ selected.server }}</template
          ></span
        >
      </div>
      <span class="map-count">{{ validPoints.length }} 个节点</span>
    </div>
    <p v-if="mapError" class="map-error" role="alert">{{ mapError }}</p>
    <p
      v-else-if="geographyError && validPoints.length"
      class="map-error"
      role="status"
    >
      地图轮廓暂不可用，节点位置仍可查看。
    </p>
  </UiPanel>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onActivated,
  onDeactivated,
  onMounted,
  ref,
  watch,
} from "vue";
import {
  Aim,
  Location,
  Minus,
  Plus,
  Refresh,
  VideoPause,
  VideoPlay,
} from "@element-plus/icons-vue";
import { getNodeMap } from "@/api/total";
import { EmptyState, UiButton, UiPanel, UiSkeleton } from "@/ui";
import { useMotion } from "@/motion";

const router = useRouter();
interface MapPoint {
  name: string;
  country: string;
  lat: number;
  lng: number;
  server?: string;
  countryCode?: string;
}
interface ProjectedPoint extends MapPoint {
  x: number;
  y: number;
}
interface WorldFeature {
  geometry: { type: string; coordinates: number[][][] | number[][][][] };
}
const props = defineProps<{ points?: MapPoint[]; loading?: boolean }>();
const emit = defineEmits<{ refresh: [] }>();
const ownPoints = ref<MapPoint[]>([]);
const ownLoading = ref(false);
const busy = computed(() => props.loading || ownLoading.value);
const validPoints = computed(() =>
  (props.points ?? ownPoints.value).filter(
    (p) =>
      Number.isFinite(p.lat) &&
      Number.isFinite(p.lng) &&
      Math.abs(p.lat) <= 90 &&
      Math.abs(p.lng) <= 180
  )
);
const projected = computed(() => {
  // Preserve every node at coincident coordinates; fan markers around the true location.
  const groups = new Map<string, MapPoint[]>();
  validPoints.value.forEach((point) => {
    const key = `${point.lat.toFixed(3)},${point.lng.toFixed(3)}`;
    groups.set(key, [...(groups.get(key) || []), point]);
  });
  return [...groups.values()].flatMap((group) =>
    group.map((point, index) => {
      const angle = (index / group.length) * Math.PI * 2;
      const radius = group.length > 1 ? 5 : 0;
      return {
        ...point,
        x: ((point.lng + 180) / 360) * 1000 + Math.cos(angle) * radius,
        y: ((90 - point.lat) / 180) * 500 + Math.sin(angle) * radius,
      };
    })
  );
});
const selectedIndex = ref(0);
const selected = computed(
  () => projected.value[selectedIndex.value] || projected.value[0]
);
const landPath = ref("");
const geographyLoading = ref(true);
const geographyError = ref(false);
const mapError = ref("");
const stage = ref<HTMLElement>();
const leaders = ref<SVGGElement>();
const { animate, stop, reduced } = useMotion();
// Geographical scanning is independent of node data and uses only CSS motion.
const playing = ref(true);
const hidden = ref(false);
const inactive = ref(false);
const inView = ref(true);
const scanPaused = computed(
  () =>
    !playing.value ||
    reduced.value ||
    hidden.value ||
    inactive.value ||
    !inView.value
);
const graticule = [
  ...Array.from({ length: 11 }, (_, i) => `M${((i + 1) * 1000) / 12},56 V444`),
  ...Array.from({ length: 5 }, (_, i) => `M0,${((i + 1) * 500) / 6} H1000`),
].join(" ");
let visibilityObserver: IntersectionObserver | undefined;
const syncVisibility = () => {
  hidden.value = document.hidden;
};
let drawing: ReturnType<typeof animate>;
let disposed = false;
let drawVersion = 0;
const controller = new AbortController();
const zoom = ref(1);
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
  zoom.value = Math.max(1, Math.min(4, zoom.value + delta));
}
function reset() {
  zoom.value = 1;
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
  } else if (["+", "=", "-", "Home", " "].includes(event.key)) {
    event.preventDefault();
    if (event.key === "Home") reset();
    else if (event.key === " ") togglePlayback();
    else setZoom(event.key === "-" ? -0.25 : 0.25);
  }
}
function leaderPath(point: ProjectedPoint, index: number) {
  // These short leaders annotate actual coordinates, rather than implying traffic links.
  const direction = point.x > 750 ? -1 : 1;
  const rise = index % 2 ? -28 : 28;
  return `M${point.x},${point.y} l${direction * 20},${rise} h${direction * 32}`;
}
async function draw() {
  const version = ++drawVersion;
  stop();
  drawing = undefined;
  await nextTick();
  if (disposed || version !== drawVersion || reduced.value || !leaders.value)
    return;
  const paths = Array.from(leaders.value.children);
  if (!paths.length) return;
  drawing = animate(
    paths,
    { clipPath: "inset(0 100% 0 0)", opacity: 0 },
    {
      clipPath: "inset(0 0% 0 0)",
      opacity: 1,
      duration: 0.56,
      stagger: Math.min(0.04, 0.8 / paths.length),
      ease: "power3.out",
      clearProps: "clipPath,opacity",
    }
  );
  if (scanPaused.value) drawing?.pause();
}
function togglePlayback() {
  if (!reduced.value) playing.value = !playing.value;
}
watch(scanPaused, (paused) => {
  if (paused) drawing?.pause();
  else drawing?.resume();
});
async function refresh() {
  if (props.points !== undefined) {
    emit("refresh");
    return;
  }
  ownLoading.value = true;
  mapError.value = "";
  try {
    const { data } = await getNodeMap();
    if (!disposed) ownPoints.value = Array.isArray(data) ? data : [];
  } catch {
    if (!disposed) {
      ownPoints.value = [];
      mapError.value = "节点地图加载失败，请重试。";
    }
  } finally {
    if (!disposed) ownLoading.value = false;
  }
}
async function loadGeography() {
  geographyLoading.value = true;
  geographyError.value = false;
  try {
    const response = await fetch(`${import.meta.env.BASE_URL}world.json`, {
      signal: controller.signal,
    });
    if (!response.ok) throw new Error("Map unavailable");
    const world = (await response.json()) as { features: WorldFeature[] };
    if (disposed) return;
    landPath.value = world.features
      .flatMap(({ geometry }) => {
        const polygons =
          geometry.type === "Polygon"
            ? [geometry.coordinates as number[][][]]
            : geometry.type === "MultiPolygon"
              ? (geometry.coordinates as number[][][][])
              : [];
        return polygons.flatMap((polygon) =>
          polygon.map(
            (ring) =>
              ring
                .map(
                  ([lng, lat], index) =>
                    `${index ? "L" : "M"}${(((lng + 180) / 360) * 1000).toFixed(1)},${(((90 - lat) / 180) * 500).toFixed(1)}`
                )
                .join("") + "Z"
          )
        );
      })
      .join("");
  } catch {
    if (!disposed) geographyError.value = true;
  } finally {
    if (!disposed) geographyLoading.value = false;
  }
}
watch([projected, busy, geographyLoading], () => {
  selectedIndex.value = 0;
  draw();
});
watch(reduced, draw);
onActivated(() => {
  inactive.value = false;
});
onDeactivated(() => {
  inactive.value = true;
});
onMounted(() => {
  syncVisibility();
  document.addEventListener("visibilitychange", syncVisibility);
  if (typeof IntersectionObserver !== "undefined" && stage.value) {
    visibilityObserver = new IntersectionObserver(([entry]) => {
      inView.value = entry.isIntersecting;
    });
    visibilityObserver.observe(stage.value);
  }
  loadGeography();
  if (props.points === undefined) refresh();
  else draw();
});
onBeforeUnmount(() => {
  disposed = true;
  document.removeEventListener("visibilitychange", syncVisibility);
  visibilityObserver?.disconnect();
  controller.abort();
  drawing?.kill();
});
</script>
<style scoped>
.map-panel {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--c-surface-2);
}
.map-panel :deep(.ui-panel__header) {
  margin-bottom: 0;
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
  overflow: hidden;
  border-radius: var(--radius-control);
  cursor: grab;
  touch-action: pan-y;
}
.land {
  fill: var(--c-canvas);
  stroke: var(--c-line);
  stroke-width: 0.6;
  fill-rule: evenodd;
}
.graticule {
  fill: none;
  stroke: var(--c-line);
  stroke-width: 0.7;
}
.scan-lines {
  pointer-events: none;
}
.scan-lines path {
  fill: none;
  stroke: var(--c-data);
  stroke-width: 0.9;
  opacity: 0.45;
}
.scan-front {
  fill: var(--c-data);
  opacity: 0.25;
  pointer-events: none;
}
@media (prefers-reduced-motion: no-preference) {
  .scan-lines {
    animation: atlas-lines 12s linear infinite;
  }
  .scan-front {
    animation: atlas-scan 12s linear infinite;
  }
}
.is-paused .scan-lines,
.is-paused .scan-front {
  animation-play-state: paused;
}
.is-reduced .scan-lines,
.is-reduced .scan-front {
  animation: none;
}
.is-reduced .scan-front {
  opacity: 0;
}
@media (prefers-reduced-motion: reduce) {
  .scan-front {
    opacity: 0;
  }
}
@keyframes atlas-lines {
  0% {
    clip-path: inset(0 100% 0 0);
  }
  85%,
  100% {
    clip-path: inset(0 0 0 0);
  }
}
@keyframes atlas-scan {
  0% {
    transform: translateX(0);
    opacity: 0;
  }
  8% {
    opacity: 0.25;
  }
  85% {
    transform: translateX(1000px);
    opacity: 0.25;
  }
  100% {
    transform: translateX(1000px);
    opacity: 0;
  }
}
.map-leaders path {
  fill: none;
  stroke: var(--c-data);
  stroke-width: 1;
  opacity: 0.6;
  transform-box: fill-box;
}
.node-hit {
  fill: transparent;
  stroke: transparent;
  stroke-width: 44px;
}
.node-halo {
  fill: var(--c-selected);
}
.node-mark {
  fill: var(--c-signal);
  stroke: var(--c-surface);
  stroke-width: 1;
}
.map-node {
  cursor: pointer;
  outline: none;
}
.map-node:focus-visible .node-halo,
.map-node:hover .node-halo,
.map-node.is-selected .node-halo {
  stroke: var(--c-signal);
  stroke-width: 2;
}
.map-selector {
  width: 100%;
  min-width: 0;
  margin-bottom: 12px;
}
.map-skeleton {
  position: absolute;
  left: 0;
  bottom: 96px;
  width: 160px;
  pointer-events: none;
}
.map-empty {
  position: absolute;
  left: 0;
  bottom: 8px;
  max-width: calc(100% - 64px);
  padding: 8px 10px;
  gap: 8px 12px;
  background: color-mix(in srgb, var(--c-surface-2) 92%, transparent);
  border-radius: var(--radius-control);
}
.map-controls {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  flex-direction: column;
  position: absolute;
  inset: 8px 0;
  pointer-events: none;
}
.map-controls > * {
  pointer-events: auto;
}
.zoom-controls {
  display: flex;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
}
.map-controls :deep(.ui-button) {
  padding: 8px 12px;
}
.map-location {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  min-height: 64px;
  padding-top: 12px;
  border-top: 1px solid var(--c-line);
}
.map-location > svg {
  width: 18px;
  height: 18px;
  color: var(--c-signal);
  flex-shrink: 0;
  margin-top: 2px;
}
.map-location > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}
.map-location strong {
  font-size: 14px;
  font-weight: 500;
}
.map-location div span {
  font-size: 14px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.map-count {
  margin-left: auto;
  white-space: nowrap;
  font: 14px/1.6 var(--font-mono);
  color: var(--c-muted);
}
.map-error {
  font-size: 14px;
  color: var(--c-muted);
  margin-top: 12px;
}
@media (max-width: 767px) {
  .map-stage {
    min-height: 280px;
    height: 280px;
    flex: none;
  }
  .world-map {
    min-height: 0;
    height: 280px;
  }
  .map-empty {
    padding: 8px;
  }
  .map-location {
    min-height: 56px;
  }
}
</style>
