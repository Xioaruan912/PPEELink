<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from "vue";
import gsap from "gsap";
import type { HistoryPoint, PeakBand } from "@/api/subcription/probe";
import { buildAreaPath, buildMonotonePath, type PlotPoint } from "./spline";

const props = defineProps<{
  points: HistoryPoint[];
  from: number;
  to: number;
  peakBands?: PeakBand[];
  tone?: "is-ok" | "is-warn" | "is-bad";
  uid: string | number;
}>();

const W = 240;
const H = 46;
const PAD_X = 4;
const PAD_TOP = 6;
const PAD_BOT = 6;
const BASE_Y = H - PAD_BOT;

const svgEl = ref<SVGSVGElement>();
const hoverIdx = ref<number | null>(null);

const strokeColor = computed(() => {
  if (props.tone === "is-bad") return "var(--c-bad, #dc2626)";
  if (props.tone === "is-warn") return "var(--c-warn, #d97706)";
  return "var(--c-ok, #16a34a)";
});

const timeSpan = computed(() => Math.max(1, props.to - props.from));

function mapX(t: number): number {
  const ratio = Math.min(1, Math.max(0, (t - props.from) / timeSpan.value));
  return PAD_X + ratio * (W - PAD_X * 2);
}

const yBounds = computed(() => {
  const rtts = props.points.filter((p) => p.ok && p.rtt > 0).map((p) => p.rtt);
  if (!rtts.length) return { min: 0, max: 100 };
  const rawMin = Math.min(...rtts);
  const rawMax = Math.max(...rtts);
  const spread = Math.max(24, rawMax - rawMin);
  return {
    min: Math.max(0, rawMin - spread * 0.18),
    max: rawMax + spread * 0.22,
  };
});

function mapY(rtt: number): number {
  const { min, max } = yBounds.value;
  const ratio = Math.min(1, Math.max(0, (rtt - min) / Math.max(1, max - min)));
  return BASE_Y - ratio * (BASE_Y - PAD_TOP);
}

// Split valid contiguous runs so timeouts create clean visible breaks.
const segments = computed(() => {
  const runs: PlotPoint[][] = [];
  let cur: PlotPoint[] = [];
  for (const pt of props.points) {
    if (pt.ok && pt.rtt > 0) {
      cur.push({ x: mapX(pt.t), y: mapY(pt.rtt) });
    } else if (cur.length > 0) {
      runs.push(cur);
      cur = [];
    }
  }
  if (cur.length > 0) runs.push(cur);
  return runs.map((run) => ({
    line: buildMonotonePath(run),
    area: buildAreaPath(run, BASE_Y),
    single: run.length === 1 ? run[0] : null,
  }));
});

const failTicks = computed(() =>
  props.points
    .filter((p) => !p.ok)
    .map((p) => ({ x: mapX(p.t), t: p.t }))
);

const lastValidPoint = computed(() => {
  for (let i = props.points.length - 1; i >= 0; i--) {
    const p = props.points[i];
    if (p.ok && p.rtt > 0) {
      return { x: mapX(p.t), y: mapY(p.rtt), rtt: p.rtt, t: p.t };
    }
  }
  return null;
});

const peakRects = computed(() =>
  (props.peakBands || []).map((b) => {
    const x1 = mapX(b.start);
    const x2 = mapX(b.end);
    return { x: x1, w: Math.max(2, x2 - x1) };
  })
);

const hoveredPoint = computed(() => {
  if (hoverIdx.value === null) return null;
  const p = props.points[hoverIdx.value];
  if (!p) return null;
  return {
    ...p,
    x: mapX(p.t),
    y: p.ok && p.rtt > 0 ? mapY(p.rtt) : BASE_Y - 2,
  };
});

function formatHHMM(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function handlePointerMove(e: PointerEvent) {
  if (!svgEl.value || !props.points.length) return;
  const rect = svgEl.value.getBoundingClientRect();
  const relX = ((e.clientX - rect.left) / Math.max(1, rect.width)) * W;
  let bestIdx = 0;
  let bestDist = Infinity;
  for (let i = 0; i < props.points.length; i++) {
    const d = Math.abs(mapX(props.points[i].t) - relX);
    if (d < bestDist) {
      bestDist = d;
      bestIdx = i;
    }
  }
  hoverIdx.value = bestIdx;
}

function handlePointerLeave() {
  hoverIdx.value = null;
}

function animateDraw() {
  if (!svgEl.value) return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const paths = svgEl.value.querySelectorAll<SVGPathElement>("[data-spark-line]");
  paths.forEach((p) => {
    const len = p.getTotalLength();
    if (!len) return;
    gsap.fromTo(
      p,
      { strokeDasharray: len, strokeDashoffset: len },
      {
        strokeDashoffset: 0,
        duration: 0.78,
        ease: "expo.out",
        clearProps: "strokeDasharray,strokeDashoffset",
      }
    );
  });
}

onMounted(() => {
  void nextTick(animateDraw);
});

watch(
  () => props.points,
  () => {
    void nextTick(animateDraw);
  }
);
</script>

<template>
  <div class="spark-wrap" @pointermove="handlePointerMove" @pointerleave="handlePointerLeave">
    <svg
      ref="svgEl"
      class="spark-svg"
      :viewBox="`0 0 ${W} ${H}`"
      preserveAspectRatio="none"
      role="img"
      aria-label="延迟趋势曲线"
    >
      <defs>
        <linearGradient :id="`sg-${uid}`" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" :stop-color="strokeColor" stop-opacity="0.18" />
          <stop offset="100%" :stop-color="strokeColor" stop-opacity="0" />
        </linearGradient>
      </defs>

      <!-- Subtle evening peak shading (20:00–23:30) -->
      <rect
        v-for="(band, idx) in peakRects"
        :key="`pk-${idx}`"
        :x="band.x"
        y="0"
        :width="band.w"
        :height="H"
        class="spark-peak"
      />

      <!-- Baseline whisper -->
      <line :x1="PAD_X" :y1="BASE_Y" :x2="W - PAD_X" :y2="BASE_Y" class="spark-base" />

      <!-- Area fills -->
      <path
        v-for="(seg, idx) in segments"
        :key="`a-${idx}`"
        v-show="seg.area"
        :d="seg.area"
        :fill="`url(#sg-${uid})`"
      />

      <!-- Monotone spline lines -->
      <path
        v-for="(seg, idx) in segments"
        :key="`l-${idx}`"
        v-show="!seg.single"
        data-spark-line
        :d="seg.line"
        fill="none"
        :stroke="strokeColor"
        stroke-width="1.85"
        stroke-linecap="round"
        stroke-linejoin="round"
        vector-effect="non-scaling-stroke"
      />

      <!-- Single isolated valid points -->
      <circle
        v-for="(seg, idx) in segments"
        :key="`s-${idx}`"
        v-show="seg.single"
        :cx="seg.single?.x"
        :cy="seg.single?.y"
        r="2.4"
        :fill="strokeColor"
      />

      <!-- Timeout / packet drop markers on baseline -->
      <rect
        v-for="(ft, idx) in failTicks"
        :key="`f-${idx}`"
        :x="ft.x - 1"
        :y="BASE_Y - 3"
        width="2"
        height="4.5"
        rx="1"
        class="spark-fail-tick"
      />

      <!-- Live breathing beacon at latest point -->
      <g v-if="lastValidPoint && !hoveredPoint">
        <circle
          :cx="lastValidPoint.x"
          :cy="lastValidPoint.y"
          r="5.5"
          :fill="strokeColor"
          class="spark-beacon-ring"
        />
        <circle
          :cx="lastValidPoint.x"
          :cy="lastValidPoint.y"
          r="2.4"
          :fill="strokeColor"
        />
      </g>

      <!-- Interactive scrub needle -->
      <g v-if="hoveredPoint">
        <line
          :x1="hoveredPoint.x"
          y1="2"
          :x2="hoveredPoint.x"
          :y2="BASE_Y"
          class="spark-needle"
        />
        <circle
          :cx="hoveredPoint.x"
          :cy="hoveredPoint.y"
          r="3.2"
          :fill="hoveredPoint.ok ? strokeColor : 'var(--c-bad, #dc2626)'"
          stroke="var(--c-surface, #fff)"
          stroke-width="1.5"
        />
      </g>
    </svg>

    <transition name="spark-tip-fade">
      <div v-if="hoveredPoint" class="spark-tip mono">
        <span :class="hoveredPoint.ok ? '' : 'is-bad'">
          {{ hoveredPoint.ok ? `${hoveredPoint.rtt}ms` : "不可达" }}
        </span>
        <em>{{ formatHHMM(hoveredPoint.t) }}</em>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.spark-wrap {
  position: relative;
  width: 100%;
  height: 46px;
  cursor: crosshair;
  touch-action: pan-y;
}

.spark-svg {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.spark-peak {
  fill: color-mix(in srgb, var(--c-warn, #d97706) 7%, transparent);
}

.spark-base {
  stroke: var(--c-line);
  stroke-width: 1;
  stroke-dasharray: 2 3;
  opacity: 0.7;
}

.spark-fail-tick {
  fill: var(--c-bad, #dc2626);
  opacity: 0.85;
}

.spark-needle {
  stroke: color-mix(in srgb, var(--c-ink) 35%, transparent);
  stroke-width: 1;
  stroke-dasharray: 2 2;
}

.spark-beacon-ring {
  opacity: 0.24;
  transform-box: fill-box;
  transform-origin: center;
  animation: spark-pulse 2.4s cubic-bezier(0.22, 1, 0.36, 1) infinite;
}

@keyframes spark-pulse {
  0%,
  100% {
    transform: scale(0.75);
    opacity: 0.28;
  }
  50% {
    transform: scale(1.35);
    opacity: 0.06;
  }
}

.spark-tip {
  position: absolute;
  top: -2px;
  right: 0;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 7px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-ink) 88%, transparent);
  color: var(--c-surface);
  font-size: 10.5px;
  line-height: 1.3;
  pointer-events: none;
  backdrop-filter: blur(8px);
}

.spark-tip em {
  font-style: normal;
  opacity: 0.68;
}

.spark-tip .is-bad {
  color: #fca5a5;
}

.spark-tip-fade-enter-active,
.spark-tip-fade-leave-active {
  transition: opacity 120ms ease, transform 120ms ease;
}

.spark-tip-fade-enter-from,
.spark-tip-fade-leave-to {
  opacity: 0;
  transform: translateY(2px);
}

@media (prefers-reduced-motion: reduce) {
  .spark-beacon-ring {
    animation: none;
  }
}
</style>
