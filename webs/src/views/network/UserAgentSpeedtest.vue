<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { Refresh, VideoPlay } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import {
  getProbes,
  runUserAgentSpeedtest,
  type Probe,
  type UserSpeedChannel,
  type UserSpeedtestResponse,
} from "@/api/subcription/probe";
import {
  buildAreaPath,
  buildMonotonePath,
  type PlotPoint,
} from "../monitor/components/spline";

defineOptions({ name: "UserAgentSpeedtest" });

const emit = defineEmits<{
  (e: "online-change", count: number): void;
}>();

const probes = ref<Probe[]>([]);
const loadingProbes = ref(true);
const selectedProbeId = ref<number | undefined>(undefined);
const mode = ref<"proxy" | "direct" | "both">("proxy");
const running = ref(false);
const elapsedSec = ref(0);
const errorMsg = ref("");
const result = ref<UserSpeedtestResponse | null>(null);
const hoverIdx = ref<number | null>(null);

let tickTimer: ReturnType<typeof setInterval> | null = null;

const onlineProbes = computed(() => probes.value.filter((p) => p.online));
const activeProbe = computed(() =>
  onlineProbes.value.find((p) => p.id === selectedProbeId.value) ||
  onlineProbes.value[0]
);

async function loadProbes() {
  loadingProbes.value = true;
  try {
    const res = await getProbes();
    probes.value = Array.isArray(res.data) ? res.data : [];
    const online = probes.value.filter((p) => p.online);
    emit("online-change", online.length);
    if (
      online.length &&
      (!selectedProbeId.value ||
        !online.some((p) => p.id === selectedProbeId.value))
    ) {
      selectedProbeId.value = online[0].id;
    }
  } catch {
    probes.value = [];
    emit("online-change", 0);
  } finally {
    loadingProbes.value = false;
  }
}

function stopTicker() {
  if (tickTimer) {
    clearInterval(tickTimer);
    tickTimer = null;
  }
}

async function startTest() {
  if (running.value || !onlineProbes.value.length) return;
  running.value = true;
  errorMsg.value = "";
  hoverIdx.value = null;
  elapsedSec.value = 0;
  stopTicker();
  tickTimer = setInterval(() => {
    elapsedSec.value += 1;
  }, 1000);
  try {
    const res = await runUserAgentSpeedtest({
      probeId: selectedProbeId.value,
      mode: mode.value,
    });
    if (res.data?.error) {
      errorMsg.value = res.data.error;
    } else {
      result.value = res.data;
    }
  } catch (e: any) {
    errorMsg.value =
      e?.response?.data?.msg ||
      e?.message ||
      "测速失败，请确认本机用户 Agent 已升级至 v1.4";
  } finally {
    stopTicker();
    running.value = false;
  }
}

const primaryChannel = computed<UserSpeedChannel | undefined>(() => {
  if (!result.value) return undefined;
  return result.value.proxy || result.value.direct;
});

const secondaryChannel = computed<UserSpeedChannel | undefined>(() => {
  if (!result.value) return undefined;
  if (result.value.proxy && result.value.direct) return result.value.direct;
  return undefined;
});

function formatMbps(v?: number): string {
  if (!v || !Number.isFinite(v)) return "0.0";
  return v >= 100 ? v.toFixed(0) : v.toFixed(1);
}

function formatMB(bytes?: number): string {
  if (!bytes || bytes <= 0) return "0 MB";
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}

// SVG chart geometry
const W = 760;
const H = 220;
const PAD_L = 44;
const PAD_R = 18;
const PAD_T = 18;
const PAD_B = 28;
const PLOT_W = W - PAD_L - PAD_R;
const PLOT_H = H - PAD_T - PAD_B;

const chartData = computed(() => {
  const ch1 = primaryChannel.value;
  const ch2 = secondaryChannel.value;
  const s1 = ch1?.samples || [];
  const s2 = ch2?.samples || [];
  if (!s1.length && !s2.length) return null;

  const allMbps = [...s1.map((s) => s.mbps), ...s2.map((s) => s.mbps)];
  const rawMax = Math.max(10, ...allMbps);
  const niceStep = rawMax <= 50 ? 10 : rawMax <= 200 ? 50 : rawMax <= 500 ? 100 : 200;
  const maxY = Math.ceil((rawMax * 1.12) / niceStep) * niceStep;

  const maxT = Math.max(
    1000,
    ...s1.map((s) => s.elapsedMs),
    ...s2.map((s) => s.elapsedMs)
  );

  const toPoints = (samples: { elapsedMs: number; mbps: number }[]): PlotPoint[] =>
    samples.map((s) => ({
      x: PAD_L + (s.elapsedMs / maxT) * PLOT_W,
      y: PAD_T + PLOT_H - (Math.min(s.mbps, maxY) / maxY) * PLOT_H,
    }));

  const p1 = toPoints(s1);
  const p2 = toPoints(s2);
  const baseY = PAD_T + PLOT_H;

  return {
    maxY,
    maxT,
    ticksY: [0, Math.round(maxY / 2), maxY],
    ticksX: [0, Math.round(maxT / 2000), Number((maxT / 1000).toFixed(1))],
    line1: buildMonotonePath(p1),
    area1: buildAreaPath(p1, baseY),
    pts1: p1,
    samples1: s1,
    line2: p2.length ? buildMonotonePath(p2) : "",
    pts2: p2,
    samples2: s2,
  };
});

function onSvgMove(e: MouseEvent) {
  const data = chartData.value;
  if (!data || !data.pts1.length) return;
  const svg = e.currentTarget as SVGSVGElement;
  const rect = svg.getBoundingClientRect();
  const relX = ((e.clientX - rect.left) / rect.width) * W;
  let best = 0;
  let bestDist = Infinity;
  for (let i = 0; i < data.pts1.length; i++) {
    const d = Math.abs(data.pts1[i].x - relX);
    if (d < bestDist) {
      bestDist = d;
      best = i;
    }
  }
  hoverIdx.value = best;
}

const hoveredPoint = computed(() => {
  const data = chartData.value;
  const idx = hoverIdx.value;
  if (!data || idx === null || !data.samples1[idx]) return null;
  const s1 = data.samples1[idx];
  const pt1 = data.pts1[idx];
  // nearest sample in series 2 by elapsedMs
  let s2Val: number | undefined;
  if (data.samples2.length) {
    let best = data.samples2[0];
    for (const cand of data.samples2) {
      if (Math.abs(cand.elapsedMs - s1.elapsedMs) < Math.abs(best.elapsedMs - s1.elapsedMs)) {
        best = cand;
      }
    }
    s2Val = best.mbps;
  }
  return {
    x: pt1.x,
    y: pt1.y,
    sec: (s1.elapsedMs / 1000).toFixed(1),
    mbps1: s1.mbps,
    mbps2: s2Val,
  };
});

const modeLabel: Record<"proxy" | "direct" | "both", string> = {
  proxy: "当前代理",
  direct: "物理直连基线",
  both: "代理 vs 直连对比",
};

onMounted(loadProbes);
onBeforeUnmount(stopTicker);
</script>

<template>
  <div class="ua-speed-stage">
    <!-- Single-Row Control Strip -->
    <section class="speed-bar">
      <div class="bar-left">
        <div class="pill-group" role="group" aria-label="测速通道">
          <button
            v-for="m in (['proxy', 'direct', 'both'] as const)"
            :key="m"
            type="button"
            class="seg-pill"
            :class="{ 'is-active': mode === m }"
            :disabled="running"
            @click="mode = m"
          >
            {{ modeLabel[m] }}
          </button>
        </div>

        <select
          v-if="onlineProbes.length > 0"
          v-model="selectedProbeId"
          class="probe-select"
          :disabled="running"
          aria-label="选择本机用户 Agent"
        >
          <option v-for="p in onlineProbes" :key="p.id" :value="p.id">
            {{ p.name }}{{ p.version ? ` (v${p.version})` : "" }}
          </option>
        </select>
        <span v-else-if="!loadingProbes" class="offline-hint">
          无在线 Agent
        </span>
      </div>

      <div class="bar-right">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loadingProbes"
          :disabled="running"
          @click="loadProbes"
        >
          刷新
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="running"
          :disabled="!onlineProbes.length || loadingProbes"
          @click="startTest"
        >
          {{
            running
              ? `测速中 ${elapsedSec}s`
              : result
                ? "重新测速"
                : "开始测速"
          }}
        </UiButton>
      </div>
    </section>

    <div v-if="errorMsg" class="inline-notice is-bad">
      {{ errorMsg }}
    </div>

    <!-- Running Pulse Stage -->
    <section v-if="running" class="running-card">
      <div class="pulse-ring" aria-hidden="true"><span></span></div>
      <strong class="running-title">正在测速…</strong>
      <span class="running-sec mono">{{ elapsedSec }}s</span>
    </section>

    <!-- Result Stage -->
    <template v-else-if="result && primaryChannel">
      <!-- Hero KPIs + Retention Banner -->
      <section class="kpi-section">
        <div class="result-meta">
          <span class="meta-dot" aria-hidden="true"></span>
          <strong>{{ result.probeName }}</strong>
          <span class="meta-tag">{{ modeLabel[result.mode] }}</span>
          <span v-if="primaryChannel.target" class="meta-tag">
            目标：{{ primaryChannel.target }}
          </span>
          <span
            v-if="secondaryChannel?.target && secondaryChannel.target !== primaryChannel.target"
            class="meta-tag"
          >
            基线：{{ secondaryChannel.target }}
          </span>
          <span
            v-if="result.retentionPct !== undefined && result.mode === 'both'"
            class="retention-pill mono"
            :class="{
              'is-great': result.retentionPct >= 75,
              'is-mid': result.retentionPct >= 40 && result.retentionPct < 75,
              'is-low': result.retentionPct < 40,
            }"
          >
            宽带保留率 {{ result.retentionPct.toFixed(1) }}%
          </span>
        </div>

        <div
          v-if="primaryChannel.error || secondaryChannel?.error"
          class="inline-notice is-warn"
        >
          {{
            [
              primaryChannel.error ? `${primaryChannel.mode === 'direct' ? '物理直连' : '代理通道'}异常：${primaryChannel.error}` : "",
              secondaryChannel?.error ? `物理直连基线异常：${secondaryChannel.error}` : "",
            ]
              .filter(Boolean)
              .join(" · ")
          }}
        </div>

        <div class="kpi-grid">
          <article class="kpi-card">
            <span class="kpi-label">
              {{ result.mode === "direct" ? "直连平均下载" : "代理平均下载" }}
            </span>
            <strong class="kpi-val mono">
              {{ formatMbps(primaryChannel.downloadMbps) }}
              <small>Mbps</small>
            </strong>
            <span class="kpi-sub mono">
              峰值 {{ formatMbps(primaryChannel.peakMbps) }} Mbps · {{ formatMB(primaryChannel.bytesDown) }}
            </span>
          </article>

          <article class="kpi-card">
            <span class="kpi-label">
              {{ result.mode === "direct" ? "直连平均上传" : "代理平均上传" }}
            </span>
            <strong class="kpi-val mono">
              {{ formatMbps(primaryChannel.uploadMbps) }}
              <small>Mbps</small>
            </strong>
            <span class="kpi-sub mono">
              已发送 {{ formatMB(primaryChannel.bytesUp) }}
            </span>
          </article>

          <article class="kpi-card">
            <span class="kpi-label">首包延迟 (TTFB)</span>
            <strong class="kpi-val mono">
              {{ primaryChannel.ttfbMs || 0 }}
              <small>ms</small>
            </strong>
          </article>

          <article class="kpi-card">
            <span class="kpi-label">网络抖动 (Jitter)</span>
            <strong class="kpi-val mono">
              {{ primaryChannel.jitterMs || 0 }}
              <small>ms</small>
            </strong>
          </article>
        </div>
      </section>

      <!-- Apple Monotone Speed Curve -->
      <section v-if="chartData" class="curve-card">
        <div class="curve-head">
          <div class="curve-title">
            <strong>速率曲线</strong>
          </div>
          <div class="curve-legend">
            <span class="legend-item">
              <i class="lg-dot is-primary"></i>
              {{ result.mode === "direct" ? "物理直连" : "当前代理" }}
            </span>
            <span v-if="secondaryChannel" class="legend-item">
              <i class="lg-dot is-secondary"></i>
              物理直连基线
            </span>
            <span v-if="hoveredPoint" class="scrub-readout mono">
              {{ hoveredPoint.sec }}s · {{ formatMbps(hoveredPoint.mbps1) }} Mbps
              <template v-if="hoveredPoint.mbps2 !== undefined">
                / 直连 {{ formatMbps(hoveredPoint.mbps2) }} Mbps
              </template>
            </span>
          </div>
        </div>

        <svg
          :viewBox="`0 0 ${W} ${H}`"
          class="speed-svg"
          role="img"
          aria-label="本机下载速率曲线"
          @mousemove="onSvgMove"
          @mouseleave="hoverIdx = null"
        >
          <defs>
            <linearGradient id="uaSpeedGrad" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stop-color="var(--c-ok)" stop-opacity="0.22" />
              <stop offset="100%" stop-color="var(--c-ok)" stop-opacity="0.01" />
            </linearGradient>
          </defs>

          <!-- Horizontal gridlines -->
          <g v-for="(val, i) in chartData.ticksY" :key="`gy-${i}`">
            <line
              :x1="PAD_L"
              :x2="W - PAD_R"
              :y1="PAD_T + PLOT_H - (val / chartData.maxY) * PLOT_H"
              :y2="PAD_T + PLOT_H - (val / chartData.maxY) * PLOT_H"
              class="grid-line"
            />
            <text
              :x="PAD_L - 8"
              :y="PAD_T + PLOT_H - (val / chartData.maxY) * PLOT_H + 4"
              text-anchor="end"
              class="axis-text mono"
            >
              {{ val }}
            </text>
          </g>

          <!-- X axis labels -->
          <text
            v-for="(sec, i) in chartData.ticksX"
            :key="`gx-${i}`"
            :x="PAD_L + (i / 2) * PLOT_W"
            :y="H - 6"
            :text-anchor="i === 0 ? 'start' : i === 2 ? 'end' : 'middle'"
            class="axis-text mono"
          >
            {{ sec }}s
          </text>

          <!-- Direct baseline dashed curve (in both mode) -->
          <path
            v-if="chartData.line2"
            :d="chartData.line2"
            class="curve-line-secondary"
          />

          <!-- Primary curve + subtle gradient area -->
          <path :d="chartData.area1" fill="url(#uaSpeedGrad)" />
          <path :d="chartData.line1" class="curve-line-primary" />

          <!-- Hover scrub indicator -->
          <g v-if="hoveredPoint">
            <line
              :x1="hoveredPoint.x"
              :x2="hoveredPoint.x"
              :y1="PAD_T"
              :y2="PAD_T + PLOT_H"
              class="scrub-line"
            />
            <circle
              :cx="hoveredPoint.x"
              :cy="hoveredPoint.y"
              r="4.5"
              class="scrub-dot"
            />
          </g>
        </svg>
      </section>

      <!-- Dual-Channel Side-by-Side Comparison Table -->
      <section v-if="result.proxy && result.direct" class="compare-card">
        <div class="compare-head">
          <strong>双通道对比</strong>
        </div>
        <div class="compare-grid mono">
          <div class="cmp-row is-head">
            <span>通道</span>
            <span>平均下载</span>
            <span>峰值下载</span>
            <span>平均上传</span>
            <span>TTFB</span>
            <span>抖动</span>
          </div>
          <div class="cmp-row">
            <span class="cmp-name"><i class="lg-dot is-primary"></i> 当前代理</span>
            <strong>{{ formatMbps(result.proxy.downloadMbps) }} Mbps</strong>
            <span>{{ formatMbps(result.proxy.peakMbps) }} Mbps</span>
            <span>{{ formatMbps(result.proxy.uploadMbps) }} Mbps</span>
            <span>{{ result.proxy.ttfbMs }} ms</span>
            <span>{{ result.proxy.jitterMs }} ms</span>
          </div>
          <div class="cmp-row">
            <span class="cmp-name"><i class="lg-dot is-secondary"></i> 物理直连</span>
            <strong>{{ formatMbps(result.direct.downloadMbps) }} Mbps</strong>
            <span>{{ formatMbps(result.direct.peakMbps) }} Mbps</span>
            <span>{{ formatMbps(result.direct.uploadMbps) }} Mbps</span>
            <span>{{ result.direct.ttfbMs }} ms</span>
            <span>{{ result.direct.jitterMs }} ms</span>
          </div>
        </div>
      </section>
    </template>

    <!-- Calm Ready Blueprint -->
    <section v-else class="ready-card">
      <div class="ready-icon mono">Mbps</div>
      <strong>本机真实带宽测速</strong>
    </section>
  </div>
</template>

<style scoped>
.ua-speed-stage {
  display: grid;
  gap: 18px;
}

.speed-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.bar-left,
.bar-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 3px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
}

.seg-pill {
  padding: 5px 12px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition: background 180ms ease, color 180ms ease;
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.seg-pill:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.probe-select {
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
  outline: none;
  cursor: pointer;
}

.offline-hint {
  font-size: 12.5px;
  color: var(--c-muted);
}

.inline-notice {
  padding: 11px 14px;
  border-radius: 12px;
  font-size: 12.5px;
  border: 1px solid var(--c-line);
}

.inline-notice.is-warn {
  background: color-mix(in srgb, var(--c-warn) 10%, var(--c-surface));
  border-color: color-mix(in srgb, var(--c-warn) 35%, transparent);
  color: var(--c-ink);
}

.inline-notice.is-bad {
  background: color-mix(in srgb, var(--c-bad) 9%, var(--c-surface));
  border-color: color-mix(in srgb, var(--c-bad) 35%, transparent);
  color: var(--c-bad);
}

.running-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 24px 22px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.pulse-ring {
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  border-radius: 50%;
  border: 1px solid color-mix(in srgb, var(--c-ok) 35%, transparent);
  display: grid;
  place-items: center;
}

.pulse-ring span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--c-ok);
  animation: speedPulse 1.4s ease-out infinite;
}

@keyframes speedPulse {
  0% {
    box-shadow: 0 0 0 0 rgb(21 184 90 / 0.45);
  }
  100% {
    box-shadow: 0 0 0 14px rgb(21 184 90 / 0);
  }
}

.running-copy {
  display: grid;
  gap: 4px;
  font-size: 13.5px;
}

.running-copy p {
  margin: 0;
  font-size: 12.5px;
}

.running-sec {
  margin-left: auto;
  font-size: 20px;
  font-weight: 600;
  color: var(--c-ink);
}

.kpi-section {
  display: grid;
  gap: 12px;
}

.result-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  font-size: 13px;
}

.meta-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-ok);
}

.meta-tag {
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  color: var(--c-muted);
  font-size: 11.5px;
}

.retention-pill {
  margin-left: auto;
  padding: 4px 11px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.retention-pill.is-great {
  background: color-mix(in srgb, var(--c-ok) 14%, transparent);
  color: var(--c-ok);
}

.retention-pill.is-mid {
  background: color-mix(in srgb, var(--c-warn) 14%, transparent);
  color: var(--c-warn);
}

.retention-pill.is-low {
  background: color-mix(in srgb, var(--c-bad) 14%, transparent);
  color: var(--c-bad);
}

.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 820px) {
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.kpi-card {
  display: grid;
  gap: 8px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}

.kpi-label {
  font-size: 12px;
  color: var(--c-muted);
}

.kpi-val {
  font-size: 28px;
  line-height: 1.05;
  font-weight: 600;
  color: var(--c-ink);
  font-variant-numeric: tabular-nums;
}

.kpi-val small {
  font-size: 12px;
  font-weight: 500;
  color: var(--c-muted);
  margin-left: 3px;
}

.kpi-sub {
  font-size: 11.5px;
  color: var(--c-muted);
}

.curve-card,
.compare-card {
  display: grid;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.curve-head,
.compare-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.curve-title {
  display: flex;
  align-items: baseline;
  gap: 10px;
  font-size: 14px;
}

.curve-legend {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12px;
  color: var(--c-muted);
  flex-wrap: wrap;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.lg-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.lg-dot.is-primary {
  background: var(--c-ok);
}

.lg-dot.is-secondary {
  background: #64748b;
}

.scrub-readout {
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 11.5px;
}

.speed-svg {
  width: 100%;
  height: auto;
  display: block;
  overflow: visible;
}

.grid-line {
  stroke: var(--c-line);
  stroke-width: 1;
  stroke-dasharray: 2 4;
}

.axis-text {
  fill: var(--c-muted);
  font-size: 11px;
}

.curve-line-primary {
  fill: none;
  stroke: var(--c-ok);
  stroke-width: 2.25;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.curve-line-secondary {
  fill: none;
  stroke: #64748b;
  stroke-width: 1.75;
  stroke-dasharray: 5 4;
  stroke-linecap: round;
}

.scrub-line {
  stroke: var(--c-ink);
  stroke-width: 1;
  stroke-opacity: 0.35;
}

.scrub-dot {
  fill: var(--c-ok);
  stroke: var(--c-surface);
  stroke-width: 2;
}

.compare-grid {
  display: grid;
  gap: 6px;
  font-size: 12.5px;
}

.cmp-row {
  display: grid;
  grid-template-columns: 1.3fr repeat(5, minmax(0, 1fr));
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 10px;
  background: var(--c-surface-muted);
}

.cmp-row.is-head {
  background: transparent;
  padding: 2px 12px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.cmp-name {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-family: inherit;
}

.ready-card {
  display: grid;
  justify-items: center;
  gap: 10px;
  padding: 56px 24px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
  text-align: center;
}

.ready-icon {
  padding: 8px 14px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface-muted);
  font-size: 12px;
  color: var(--c-ink);
  font-weight: 600;
}

.ready-card p {
  max-width: 520px;
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
}

.muted {
  color: var(--c-muted);
  font-size: 12px;
}
</style>
