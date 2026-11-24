<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import gsap from "gsap";
import { Close } from "@element-plus/icons-vue";
import CountryFlag from "@/ui/CountryFlag.vue";
import { displayNodeName } from "@/utils/nodeDisplay";
import {
  getProbeHistory,
  type HistoryPoint,
  type NodeHistoryResponse,
  type ProbeHistorySeries,
} from "@/api/subcription/probe";
import { buildAreaPath, buildMonotonePath, type PlotPoint } from "./spline";

export interface DetailNodeTarget {
  id: number;
  name: string;
  server: string;
  port: number;
  country: string;
  countryCode: string;
  kind?: string;
}

const props = defineProps<{
  modelValue: boolean;
  node: DetailNodeTarget | null;
  initialProbeId?: number | null;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
}>();

const RANGES = [
  { key: "1h", label: "1 小时" },
  { key: "6h", label: "6 小时" },
  { key: "24h", label: "24 小时" },
  { key: "7d", label: "7 天" },
] as const;

type RangeKey = (typeof RANGES)[number]["key"];

const PALETTE = [
  "#10b981", // Emerald
  "#2563eb", // Cobalt
  "#d97706", // Amber
  "#7c3aed", // Violet
  "#0891b2", // Cyan
];

const W = 720;
const H = 248;
const PAD_L = 44;
const PAD_R = 16;
const PAD_T = 22;
const PAD_B = 28;
const PLOT_W = W - PAD_L - PAD_R;
const PLOT_H = H - PAD_T - PAD_B;
const BASE_Y = PAD_T + PLOT_H;

const range = ref<RangeKey>("24h");
const loading = ref(false);
const history = ref<NodeHistoryResponse | null>(null);
const hiddenProbeIds = ref<Set<number>>(new Set());
const scrubTime = ref<number | null>(null);

const sheetEl = ref<HTMLElement>();
const svgEl = ref<SVGSVGElement>();
const needleEl = ref<SVGGElement>();
const segTrackEl = ref<HTMLElement>();
const segThumbEl = ref<HTMLElement>();

let quickNeedleX: gsap.QuickToFunc | null = null;

const visibleSeries = computed<ProbeHistorySeries[]>(() => {
  const list = history.value?.series || [];
  return list.filter((s) => !hiddenProbeIds.value.has(s.probeId));
});

const seriesColors = computed(() => {
  const map = new Map<number, string>();
  const list = history.value?.series || [];
  list.forEach((s, i) => {
    map.set(s.probeId, PALETTE[i % PALETTE.length]);
  });
  return map;
});

const timeFrom = computed(() => history.value?.from ?? Date.now() - 86400000);
const timeTo = computed(() => history.value?.to ?? Date.now());
const timeSpan = computed(() => Math.max(1, timeTo.value - timeFrom.value));

function mapX(t: number) {
  const ratio = Math.min(1, Math.max(0, (t - timeFrom.value) / timeSpan.value));
  return PAD_L + ratio * PLOT_W;
}

const yDomain = computed(() => {
  const rtts: number[] = [];
  for (const s of visibleSeries.value) {
    for (const p of s.points) {
      if (p.ok && p.rtt > 0) rtts.push(p.rtt);
    }
  }
  if (!rtts.length) return { min: 0, max: 200 };
  const rawMin = Math.min(...rtts);
  const rawMax = Math.max(...rtts);
  const spread = Math.max(30, rawMax - rawMin);
  const min = Math.max(0, Math.floor((rawMin - spread * 0.16) / 10) * 10);
  const max = Math.ceil((rawMax + spread * 0.22) / 10) * 10;
  return { min, max: Math.max(min + 20, max) };
});

function mapY(rtt: number) {
  const { min, max } = yDomain.value;
  const ratio = Math.min(1, Math.max(0, (rtt - min) / Math.max(1, max - min)));
  return BASE_Y - ratio * PLOT_H;
}

const yTicks = computed(() => {
  const { min, max } = yDomain.value;
  const mid = Math.round((min + max) / 2);
  return [
    { v: max, y: mapY(max) },
    { v: mid, y: mapY(mid) },
    { v: min, y: mapY(min) },
  ];
});

const xTicks = computed(() => {
  const count = 4;
  const out: { x: number; label: string }[] = [];
  for (let i = 0; i <= count; i++) {
    const t = timeFrom.value + (timeSpan.value * i) / count;
    out.push({ x: mapX(t), label: formatAxisTime(t) });
  }
  return out;
});

const peakRects = computed(() =>
  (history.value?.peakBands || []).map((b) => {
    const x1 = mapX(b.start);
    const x2 = mapX(b.end);
    return { x: x1, w: Math.max(4, x2 - x1) };
  })
);

const renderedSeries = computed(() =>
  visibleSeries.value.map((s) => {
    const runs: PlotPoint[][] = [];
    let cur: PlotPoint[] = [];
    const fails: { x: number; t: number }[] = [];
    for (const pt of s.points) {
      if (pt.ok && pt.rtt > 0) {
        cur.push({ x: mapX(pt.t), y: mapY(pt.rtt) });
      } else {
        fails.push({ x: mapX(pt.t), t: pt.t });
        if (cur.length > 0) {
          runs.push(cur);
          cur = [];
        }
      }
    }
    if (cur.length > 0) runs.push(cur);
    const color = seriesColors.value.get(s.probeId) || PALETTE[0];
    return {
      ...s,
      color,
      fails,
      segments: runs.map((run) => ({
        line: buildMonotonePath(run),
        area: buildAreaPath(run, BASE_Y),
        single: run.length === 1 ? run[0] : null,
      })),
    };
  })
);

function findNearestPoint(points: HistoryPoint[], targetT: number): HistoryPoint | null {
  if (!points.length) return null;
  let best = points[0];
  let bestDist = Math.abs(points[0].t - targetT);
  for (let i = 1; i < points.length; i++) {
    const d = Math.abs(points[i].t - targetT);
    if (d < bestDist) {
      bestDist = d;
      best = points[i];
    }
  }
  return best;
}

const scrubbedSnapshots = computed(() => {
  if (scrubTime.value === null) return [];
  const out: {
    probeId: number;
    probeName: string;
    color: string;
    point: HistoryPoint;
    x: number;
    y: number;
  }[] = [];
  for (const s of visibleSeries.value) {
    const pt = findNearestPoint(s.points, scrubTime.value);
    if (!pt) continue;
    out.push({
      probeId: s.probeId,
      probeName: s.probeName,
      color: seriesColors.value.get(s.probeId) || PALETTE[0],
      point: pt,
      x: mapX(pt.t),
      y: pt.ok && pt.rtt > 0 ? mapY(pt.rtt) : BASE_Y - 2,
    });
  }
  return out;
});

const aggregateStats = computed(() => {
  const list = visibleSeries.value;
  if (!list.length) {
    return { current: null as number | null, avg: 0, min: 0, max: 0, uptime: 0, samples: 0 };
  }
  let sumAvg = 0;
  let avgCnt = 0;
  let min = Infinity;
  let max = 0;
  let sumUp = 0;
  let samples = 0;
  let current: number | null = null;

  for (const s of list) {
    samples += s.stats.samples;
    sumUp += s.stats.uptime;
    if (s.stats.avg > 0) {
      sumAvg += s.stats.avg;
      avgCnt++;
    }
    if (s.stats.min > 0 && s.stats.min < min) min = s.stats.min;
    if (s.stats.max > max) max = s.stats.max;
    if (s.stats.current != null && (current === null || s.stats.current < current)) {
      current = s.stats.current;
    }
  }
  return {
    current,
    avg: avgCnt ? Math.round(sumAvg / avgCnt) : 0,
    min: min === Infinity ? 0 : min,
    max,
    uptime: Math.round((sumUp / list.length) * 10) / 10,
    samples,
  };
});

const heroReadout = computed(() => {
  if (scrubTime.value !== null && scrubbedSnapshots.value.length > 0) {
    const okSnaps = scrubbedSnapshots.value.filter((s) => s.point.ok && s.point.rtt > 0);
    const primary = okSnaps.length
      ? okSnaps.reduce((a, b) => (b.point.rtt < a.point.rtt ? b : a))
      : scrubbedSnapshots.value[0];
    const isPeak = isPeakTimestamp(primary.point.t);
    return {
      rtt: primary.point.ok ? primary.point.rtt : null,
      sub: `${formatFullTime(primary.point.t)}${isPeak ? " · 晚高峰" : ""}`,
      isScrubbing: true,
    };
  }
  return {
    rtt: aggregateStats.value.current,
    sub:
      aggregateStats.value.samples > 0
        ? `近 ${RANGES.find((r) => r.key === range.value)?.label} · ${aggregateStats.value.samples} 次采样`
        : "暂无历史采样，开启设备 1 分钟定时记录后自动绘制",
    isScrubbing: false,
  };
});

function isPeakTimestamp(ts: number) {
  const d = new Date(ts);
  const mins = d.getHours() * 60 + d.getMinutes();
  return mins >= 20 * 60 && mins <= 23 * 60 + 30;
}

function formatAxisTime(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, "0");
  if (range.value === "7d") {
    return `${d.getMonth() + 1}/${d.getDate()}`;
  }
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function formatFullTime(ts: number) {
  const d = new Date(ts);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function rttToneClass(rtt: number | null) {
  if (rtt === null) return "is-bad";
  if (rtt <= 120) return "is-ok";
  if (rtt <= 240) return "is-warn";
  return "is-bad";
}

function toggleSeries(probeId: number) {
  const next = new Set(hiddenProbeIds.value);
  if (next.has(probeId)) {
    next.delete(probeId);
  } else if (next.size < (history.value?.series.length || 1) - 1) {
    next.add(probeId);
  }
  hiddenProbeIds.value = next;
}

function moveSegThumb(animate = true) {
  if (!segTrackEl.value || !segThumbEl.value) return;
  const activeBtn = segTrackEl.value.querySelector<HTMLElement>("[data-seg-active='true']");
  if (!activeBtn) return;
  const x = activeBtn.offsetLeft;
  const w = activeBtn.offsetWidth;
  if (!animate || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    gsap.set(segThumbEl.value, { x, width: w });
    return;
  }
  gsap.to(segThumbEl.value, {
    x,
    width: w,
    duration: 0.36,
    ease: "expo.out",
  });
}

function animateCurves() {
  if (!svgEl.value) return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const lines = svgEl.value.querySelectorAll<SVGPathElement>("[data-detail-line]");
  lines.forEach((p, idx) => {
    const len = p.getTotalLength();
    if (!len) return;
    gsap.fromTo(
      p,
      { strokeDasharray: len, strokeDashoffset: len },
      {
        strokeDashoffset: 0,
        duration: 0.85,
        delay: idx * 0.06,
        ease: "expo.out",
        clearProps: "strokeDasharray,strokeDashoffset",
      }
    );
  });
}

function handlePointerMove(e: PointerEvent) {
  if (!svgEl.value || !visibleSeries.value.length) return;
  const rect = svgEl.value.getBoundingClientRect();
  const relX = ((e.clientX - rect.left) / Math.max(1, rect.width)) * W;
  const clampedX = Math.min(PAD_L + PLOT_W, Math.max(PAD_L, relX));
  const ratio = (clampedX - PAD_L) / PLOT_W;
  scrubTime.value = timeFrom.value + ratio * timeSpan.value;

  if (needleEl.value) {
    if (!quickNeedleX) {
      quickNeedleX = gsap.quickTo(needleEl.value, "x", {
        duration: 0.14,
        ease: "power3.out",
      });
    }
    const snapX = scrubbedSnapshots.value[0]?.x ?? clampedX;
    quickNeedleX(snapX);
  }
}

function handlePointerLeave() {
  scrubTime.value = null;
}

async function fetchHistory() {
  if (!props.node) return;
  loading.value = true;
  scrubTime.value = null;
  try {
    const res = await getProbeHistory({
      nodeId: props.node.id,
      range: range.value,
      probeId: props.initialProbeId || undefined,
    });
    history.value = res.data || null;
    hiddenProbeIds.value = new Set();
    await nextTick();
    quickNeedleX = null;
    animateCurves();
  } finally {
    loading.value = false;
  }
}

function selectRange(k: RangeKey) {
  if (range.value === k) return;
  range.value = k;
  void nextTick(() => moveSegThumb(true));
  void fetchHistory();
}

function close() {
  emit("update:modelValue", false);
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === "Escape" && props.modelValue) close();
}

watch(
  () => [props.modelValue, props.node?.id] as const,
  async ([open]) => {
    if (open) {
      window.addEventListener("keydown", onKeydown);
      await nextTick();
      moveSegThumb(false);
      if (sheetEl.value && !window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
        gsap.fromTo(
          sheetEl.value,
          { y: 18, scale: 0.982, opacity: 0 },
          { y: 0, scale: 1, opacity: 1, duration: 0.38, ease: "expo.out", clearProps: "transform" }
        );
      }
      await fetchHistory();
    } else {
      window.removeEventListener("keydown", onKeydown);
      scrubTime.value = null;
      quickNeedleX = null;
    }
  }
);

onBeforeUnmount(() => {
  window.removeEventListener("keydown", onKeydown);
});
</script>

<template>
  <teleport to="body">
    <transition name="sheet-fade">
      <div
        v-if="modelValue && node"
        class="telemetry-backdrop"
        role="dialog"
        aria-modal="true"
        :aria-label="`${displayNodeName(node.name)} 延迟曲线`"
        @click.self="close"
      >
        <section ref="sheetEl" class="telemetry-sheet">
          <!-- Top Bar -->
          <header class="ts-head">
            <div class="ts-title-group">
              <CountryFlag :code="node.countryCode" :label="node.country" />
              <div class="ts-title-text">
                <div class="ts-name-row">
                  <h2>{{ displayNodeName(node.name) }}</h2>
                  <span v-if="node.kind" class="ts-proto mono">{{
                    node.kind.toUpperCase()
                  }}</span>
                </div>
                <span class="ts-host mono">{{ node.server }}:{{ node.port }}</span>
              </div>
            </div>

            <div class="ts-head-right">
              <div ref="segTrackEl" class="seg-control" role="tablist" aria-label="时间范围">
                <span ref="segThumbEl" class="seg-thumb" aria-hidden="true"></span>
                <button
                  v-for="r in RANGES"
                  :key="r.key"
                  type="button"
                  role="tab"
                  class="seg-btn"
                  :aria-selected="range === r.key"
                  :data-seg-active="range === r.key"
                  @click="selectRange(r.key)"
                >
                  {{ r.label }}
                </button>
              </div>

              <button
                type="button"
                class="ts-close"
                aria-label="关闭"
                @click="close"
              >
                <el-icon><Close /></el-icon>
              </button>
            </div>
          </header>

          <!-- Apple Stocks Live Hero Readout -->
          <div class="ts-hero">
            <div class="ts-hero-metric">
              <div
                v-if="heroReadout.rtt !== null"
                class="ts-big-rtt mono"
                :class="rttToneClass(heroReadout.rtt)"
              >
                <span>{{ heroReadout.rtt }}</span>
                <small>ms</small>
              </div>
              <div v-else class="ts-big-rtt is-bad">
                <span>{{ aggregateStats.samples > 0 ? "不可达" : "—" }}</span>
              </div>
              <span class="ts-sub mono" :class="{ 'is-scrub': heroReadout.isScrubbing }">
                {{ heroReadout.sub }}
              </span>
            </div>

            <!-- Interactive Device Legend / Scrub Breakdown -->
            <div v-if="history?.series.length" class="ts-legend">
              <button
                v-for="s in history.series"
                :key="s.probeId"
                type="button"
                class="legend-pill"
                :class="{ 'is-muted': hiddenProbeIds.has(s.probeId) }"
                @click="toggleSeries(s.probeId)"
              >
                <i :style="{ background: seriesColors.get(s.probeId) }"></i>
                <span>{{ s.probeName }}</span>
                <b class="mono">
                  <template v-if="scrubTime !== null">
                    {{
                      (() => {
                        const snap = scrubbedSnapshots.find((x) => x.probeId === s.probeId);
                        if (!snap) return "—";
                        return snap.point.ok ? `${snap.point.rtt}ms` : "不可达";
                      })()
                    }}
                  </template>
                  <template v-else>
                    {{ s.stats.avg > 0 ? `${s.stats.avg}ms` : "—" }}
                  </template>
                </b>
              </button>
            </div>
          </div>

          <!-- High-Craft SVG Telemetry Canvas -->
          <div
            class="ts-canvas-wrap"
            :class="{ 'is-loading': loading }"
            @pointermove="handlePointerMove"
            @pointerleave="handlePointerLeave"
          >
            <svg
              ref="svgEl"
              class="ts-svg"
              :viewBox="`0 0 ${W} ${H}`"
              preserveAspectRatio="none"
            >
              <defs>
                <linearGradient
                  v-for="s in renderedSeries"
                  :key="`dg-${s.probeId}`"
                  :id="`dg-${node.id}-${s.probeId}`"
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="0%" :stop-color="s.color" stop-opacity="0.16" />
                  <stop offset="100%" :stop-color="s.color" stop-opacity="0" />
                </linearGradient>
              </defs>

              <!-- Evening Peak Shading (20:00–23:30) -->
              <g v-for="(band, idx) in peakRects" :key="`b-${idx}`">
                <rect
                  :x="band.x"
                  :y="PAD_T"
                  :width="band.w"
                  :height="PLOT_H"
                  rx="4"
                  class="ts-peak-band"
                />
                <text
                  v-if="band.w >= 34"
                  :x="band.x + band.w / 2"
                  :y="PAD_T - 6"
                  text-anchor="middle"
                  class="ts-peak-label"
                >
                  晚高峰
                </text>
              </g>

              <!-- Horizontal Hairline Grid -->
              <g v-for="tk in yTicks" :key="`y-${tk.v}`">
                <line
                  :x1="PAD_L"
                  :y1="tk.y"
                  :x2="W - PAD_R"
                  :y2="tk.y"
                  class="ts-grid-line"
                />
                <text :x="PAD_L - 8" :y="tk.y + 3.5" text-anchor="end" class="ts-axis-text mono">
                  {{ tk.v }}
                </text>
              </g>

              <!-- X Axis Labels -->
              <text
                v-for="(xt, idx) in xTicks"
                :key="`x-${idx}`"
                :x="xt.x"
                :y="H - 6"
                :text-anchor="idx === 0 ? 'start' : idx === xTicks.length - 1 ? 'end' : 'middle'"
                class="ts-axis-text mono"
              >
                {{ xt.label }}
              </text>

              <!-- Area Fills + Smooth Curves -->
              <g v-for="s in renderedSeries" :key="`ser-${s.probeId}`">
                <path
                  v-for="(seg, idx) in s.segments"
                  :key="`a-${idx}`"
                  v-show="seg.area"
                  :d="seg.area"
                  :fill="`url(#dg-${node.id}-${s.probeId})`"
                />
                <path
                  v-for="(seg, idx) in s.segments"
                  :key="`l-${idx}`"
                  v-show="!seg.single"
                  data-detail-line
                  :d="seg.line"
                  fill="none"
                  :stroke="s.color"
                  stroke-width="2.2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
                <circle
                  v-for="(seg, idx) in s.segments"
                  :key="`pt-${idx}`"
                  v-show="seg.single"
                  :cx="seg.single?.x"
                  :cy="seg.single?.y"
                  r="3.2"
                  :fill="s.color"
                />
                <!-- Timeout baseline markers -->
                <rect
                  v-for="(f, idx) in s.fails"
                  :key="`f-${idx}`"
                  :x="f.x - 1.5"
                  :y="BASE_Y - 5"
                  width="3"
                  height="6"
                  rx="1.5"
                  class="ts-fail-tick"
                />
              </g>

              <!-- Magnetic GSAP Scrubbing Needle -->
              <g v-show="scrubTime !== null" ref="needleEl">
                <line
                  x1="0"
                  :y1="PAD_T"
                  x2="0"
                  :y2="BASE_Y"
                  class="ts-needle"
                />
              </g>

              <!-- Active Snap Halos -->
              <g v-for="snap in scrubbedSnapshots" :key="`sn-${snap.probeId}`">
                <circle
                  :cx="snap.x"
                  :cy="snap.y"
                  r="8"
                  :fill="snap.point.ok ? snap.color : 'var(--c-bad, #dc2626)'"
                  opacity="0.18"
                />
                <circle
                  :cx="snap.x"
                  :cy="snap.y"
                  r="4"
                  :fill="snap.point.ok ? snap.color : 'var(--c-bad, #dc2626)'"
                  stroke="var(--c-surface, #fff)"
                  stroke-width="2"
                />
              </g>
            </svg>

            <div v-if="!loading && !renderedSeries.length" class="ts-empty-overlay">
              <span>暂无该时间段采样数据</span>
            </div>
          </div>

          <!-- 4 Precision KPI Tiles -->
          <footer class="ts-kpis">
            <div class="kpi-tile">
              <span>平均延迟</span>
              <strong class="mono">{{ aggregateStats.avg ? `${aggregateStats.avg} ms` : "—" }}</strong>
            </div>
            <div class="kpi-tile">
              <span>最低延迟</span>
              <strong class="mono">{{ aggregateStats.min ? `${aggregateStats.min} ms` : "—" }}</strong>
            </div>
            <div class="kpi-tile">
              <span>峰值延迟</span>
              <strong class="mono">{{ aggregateStats.max ? `${aggregateStats.max} ms` : "—" }}</strong>
            </div>
            <div class="kpi-tile">
              <span>可用率</span>
              <strong
                class="mono"
                :class="
                  aggregateStats.samples
                    ? aggregateStats.uptime >= 98
                      ? 'is-ok'
                      : aggregateStats.uptime >= 90
                        ? 'is-warn'
                        : 'is-bad'
                    : ''
                "
              >
                {{ aggregateStats.samples ? `${aggregateStats.uptime}%` : "—" }}
              </strong>
            </div>
          </footer>
        </section>
      </div>
    </transition>
  </teleport>
</template>

<style scoped>
.telemetry-backdrop {
  position: fixed;
  inset: 0;
  z-index: 2100;
  display: grid;
  place-items: center;
  padding: 20px;
  background: rgba(15, 17, 21, 0.36);
  backdrop-filter: blur(12px);
}

.telemetry-sheet {
  width: min(780px, 100%);
  max-height: 92vh;
  overflow-y: auto;
  display: grid;
  gap: 20px;
  padding: 26px 28px;
  border: 1px solid var(--c-line);
  border-radius: 26px;
  background: var(--c-surface);
  box-shadow: 0 28px 80px -24px rgba(0, 0, 0, 0.28);
}

.ts-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.ts-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.ts-title-text {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.ts-name-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.ts-name-row h2 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--c-ink);
}

.ts-proto {
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--c-surface-2);
  font-size: 10.5px;
  color: var(--c-muted);
}

.ts-host {
  font-size: 12px;
  color: var(--c-muted);
}

.ts-head-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

/* Apple iOS Segmented Control */
.seg-control {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 3px;
  border-radius: 999px;
  background: var(--c-surface-2);
  border: 1px solid var(--c-line);
}

.seg-thumb {
  position: absolute;
  top: 3px;
  left: 0;
  height: calc(100% - 6px);
  border-radius: 999px;
  background: var(--c-surface);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.1);
  pointer-events: none;
}

.seg-btn {
  position: relative;
  z-index: 1;
  padding: 5px 12px;
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: color 160ms ease;
}

.seg-btn[data-seg-active="true"] {
  color: var(--c-ink);
  font-weight: 600;
}

.ts-close {
  display: grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-muted);
  cursor: pointer;
  transition: color 150ms ease, border-color 150ms ease;
}

.ts-close:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

/* Hero Readout */
.ts-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.ts-hero-metric {
  display: grid;
  gap: 4px;
}

.ts-big-rtt {
  display: flex;
  align-items: baseline;
  gap: 5px;
  line-height: 1;
}

.ts-big-rtt span {
  font-size: 40px;
  font-weight: 600;
  letter-spacing: -0.04em;
  font-variant-numeric: tabular-nums;
}

.ts-big-rtt small {
  font-size: 14px;
  opacity: 0.7;
}

.ts-big-rtt.is-ok {
  color: var(--c-ok);
}

.ts-big-rtt.is-warn {
  color: var(--c-warn, #d97706);
}

.ts-big-rtt.is-bad {
  color: var(--c-bad);
}

.ts-sub {
  font-size: 12px;
  color: var(--c-muted);
  transition: color 150ms ease;
}

.ts-sub.is-scrub {
  color: var(--c-ink);
  font-weight: 500;
}

.ts-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
  transition: opacity 160ms ease, border-color 160ms ease;
}

.legend-pill:hover {
  border-color: var(--c-ink);
}

.legend-pill.is-muted {
  opacity: 0.4;
}

.legend-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.legend-pill b {
  font-weight: 600;
}

/* SVG Canvas */
.ts-canvas-wrap {
  position: relative;
  width: 100%;
  height: 248px;
  cursor: crosshair;
  touch-action: pan-y;
  transition: opacity 180ms ease;
}

.ts-canvas-wrap.is-loading {
  opacity: 0.55;
}

.ts-svg {
  display: block;
  width: 100%;
  height: 100%;
}

.ts-peak-band {
  fill: color-mix(in srgb, var(--c-warn, #d97706) 8%, transparent);
}

.ts-peak-label {
  font-size: 10px;
  fill: var(--c-warn, #d97706);
  opacity: 0.85;
}

.ts-grid-line {
  stroke: var(--c-line);
  stroke-width: 1;
  stroke-dasharray: 2 4;
}

.ts-axis-text {
  font-size: 10.5px;
  fill: var(--c-muted);
}

.ts-fail-tick {
  fill: var(--c-bad, #dc2626);
  opacity: 0.85;
}

.ts-needle {
  stroke: color-mix(in srgb, var(--c-ink) 38%, transparent);
  stroke-width: 1.2;
  stroke-dasharray: 3 3;
}

.ts-empty-overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  font-size: 13px;
  color: var(--c-muted);
  pointer-events: none;
}

/* KPI Strip */
.ts-kpis {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding-top: 16px;
  border-top: 1px solid var(--c-line);
}

.kpi-tile {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 14px;
  background: var(--c-surface-2);
}

.kpi-tile span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.kpi-tile strong {
  font-size: 16px;
  font-weight: 600;
  color: var(--c-ink);
}

.kpi-tile strong.is-ok {
  color: var(--c-ok);
}

.kpi-tile strong.is-warn {
  color: var(--c-warn, #d97706);
}

.kpi-tile strong.is-bad {
  color: var(--c-bad);
}

.sheet-fade-enter-active,
.sheet-fade-leave-active {
  transition: opacity 220ms ease;
}

.sheet-fade-enter-from,
.sheet-fade-leave-to {
  opacity: 0;
}

@media (max-width: 640px) {
  .telemetry-sheet {
    padding: 20px 18px;
    border-radius: 20px;
  }

  .ts-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
