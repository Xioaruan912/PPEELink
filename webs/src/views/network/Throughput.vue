<script setup lang="ts">
import {
  computed,
  onActivated,
  onBeforeUnmount,
  onDeactivated,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import gsap from "gsap";
import { Close, Download, Refresh, Setting, VideoPlay } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import {
  getThroughputSources,
  prepareThroughput,
  getThroughputDetail,
  createThroughput,
  cancelThroughput,
  type ThroughputSettings,
  type ThroughputSource,
  type ThroughputDetail,
  type ThroughputJob,
  type DirectionResult,
} from "@/api/network/throughput";
import {
  curvePoints,
  formatRate,
  formatBytes,
} from "@/utils/network-throughput";

defineOptions({ name: "NetworkThroughput" });

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const sources = ref<ThroughputSource[]>([]);
const detail = ref<ThroughputDetail | null>(null);
const showAdvanced = ref(false);

const settings = reactive<ThroughputSettings>({
  landingId: 0,
  sourceIds: [],
  protocol: "tcp",
  direction: "both",
  family: 4,
  port: 5201,
  duration: 10,
  streams: 1,
  rateMbps: 100,
});

const loading = ref(true);
const starting = ref(false);
const cancelling = ref(false);
const preparing = ref(false);
const error = ref("");
const sourceError = ref("");

let visible = true;
let timer: ReturnType<typeof setTimeout> | undefined;
let sequence = 0;
let selectedId = 0;
let sourceSequence = 0;

const candidates = computed(() =>
  sources.value.filter((s) => s.id !== settings.landingId)
);

const landingServer = computed(() =>
  sources.value.find((s) => s.id === settings.landingId)
);

const selected = computed(() =>
  sources.value.filter(
    (s) => settings.sourceIds.includes(s.id) || s.id === settings.landingId
  )
);

const notReady = computed(() =>
  selected.value.filter(
    (s) => !s.available || s.prepareRequested || s.preparing
  )
);

const isRunning = computed(() => detail.value?.run.status === "running");

const canStart = computed(
  () =>
    !loading.value &&
    !starting.value &&
    !isRunning.value &&
    settings.landingId > 0 &&
    settings.sourceIds.length > 0 &&
    settings.sourceIds.length <= 8 &&
    !notReady.value.length &&
    selected.value.length === settings.sourceIds.length + 1
);

const jobs = computed(() => detail.value?.jobs || []);

const activeJob = computed(
  () =>
    jobs.value.find((j) => j.status === "running" || j.status === "starting") ||
    jobs.value[jobs.value.length - 1]
);

const bestForward = computed(() => {
  const rates = jobs.value
    .map((j) => j.result?.forward?.bitsPerSecond || 0)
    .filter((v) => v > 0);
  return rates.length ? Math.max(...rates) : 0;
});

const bestReverse = computed(() => {
  const rates = jobs.value
    .map((j) => j.result?.reverse?.bitsPerSecond || 0)
    .filter((v) => v > 0);
  return rates.length ? Math.max(...rates) : 0;
});

const totalBytes = computed(
  () =>
    jobs.value.reduce(
      (sum, j) =>
        sum + (j.result?.forward?.bytes || 0) + (j.result?.reverse?.bytes || 0),
      0
    ) || 0
);

function directions(
  j: ThroughputJob
): { key: string; label: string; value: DirectionResult }[] {
  const r = j.result;
  if (!r) return [];
  return [
    ...(r.forward ? [{ key: "forward", label: "去程", value: r.forward }] : []),
    ...(r.reverse ? [{ key: "reverse", label: "回程", value: r.reverse }] : []),
  ];
}

const landingCandidates = computed(() =>
  sources.value.filter((s) => s.id !== settings.sourceIds[0])
);

function selectSource(id: number) {
  if (settings.sourceIds[0] === id) return;
  settings.sourceIds = [id];
  if (settings.landingId === id) {
    const other = sources.value.find((s) => s.id !== id);
    settings.landingId = other ? other.id : 0;
  }
}

function selectLanding(id: number) {
  if (settings.landingId === id) return;
  settings.landingId = id;
  if (settings.sourceIds[0] === id) {
    const other = sources.value.find((s) => s.id !== id);
    settings.sourceIds = other ? [other.id] : [];
  }
}

function clearTimer() {
  if (timer) clearTimeout(timer);
  timer = undefined;
}

function schedule() {
  clearTimer();
  if (
    visible &&
    (isRunning.value ||
      sources.value.some((s) => s.prepareRequested || s.preparing))
  ) {
    timer = setTimeout(() => {
      if (isRunning.value) void loadDetail();
      if (sources.value.some((s) => s.prepareRequested || s.preparing))
        void loadSources(false);
    }, 2000);
  }
}

async function loadSources(show = true) {
  const requestId = ++sourceSequence;
  if (show) loading.value = true;
  sourceError.value = "";
  try {
    const res = await getThroughputSources();
    if (requestId !== sourceSequence) return;
    sources.value = res.data || [];
    // Smart zero-config defaults: auto-select first landing & second source
    if (!settings.landingId && sources.value.length >= 1) {
      settings.landingId = sources.value[0].id;
    }
    if (!settings.sourceIds.length && sources.value.length >= 2) {
      const other = sources.value.find((s) => s.id !== settings.landingId);
      if (other) settings.sourceIds = [other.id];
    }
  } catch {
    if (requestId === sourceSequence) sourceError.value = "无法读取服务器列表";
  } finally {
    if (requestId === sourceSequence) {
      loading.value = false;
      schedule();
    }
  }
}

async function loadDetail() {
  if (!selectedId) return;
  const id = selectedId;
  const requestId = ++sequence;
  try {
    const res = await getThroughputDetail(id);
    if (requestId !== sequence || id !== selectedId) return;
    detail.value = res.data;
    error.value = "";
  } catch {
    if (requestId === sequence) error.value = "测速结果读取失败";
  } finally {
    if (requestId === sequence) schedule();
  }
}

async function start() {
  if (!canStart.value) return;
  starting.value = true;
  error.value = "";
  try {
    const res = await createThroughput({
      ...settings,
      sourceIds: [...settings.sourceIds],
    });
    clearTimer();
    sequence++;
    selectedId = res.data.id;
    detail.value = null;
    await loadDetail();
  } catch {
    error.value = "发起测速失败，请确认两端 Agent 在线且空闲";
  } finally {
    starting.value = false;
  }
}

async function cancel() {
  if (!selectedId) return;
  cancelling.value = true;
  try {
    await cancelThroughput(selectedId);
    await loadDetail();
  } finally {
    cancelling.value = false;
  }
}

async function prepare() {
  const ids = notReady.value
    .filter((s) => !(s.prepareRequested || s.preparing))
    .map((s) => s.id);
  if (!ids.length) return;
  preparing.value = true;
  error.value = "";
  try {
    await prepareThroughput(ids);
    ElMessage.success("已下发测速工具准备任务");
    await loadSources(false);
  } catch {
    error.value = "准备失败，请检查 Agent 在线状态";
  } finally {
    preparing.value = false;
  }
}

watch(
  () => settings.landingId,
  (id) => {
    settings.sourceIds = settings.sourceIds.filter((s) => s !== id);
  }
);

onMounted(() => {
  void loadSources();
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onActivated(() => {
  visible = true;
  schedule();
});
onDeactivated(() => {
  visible = false;
  clearTimer();
});
onBeforeUnmount(() => {
  visible = false;
  sequence++;
  sourceSequence++;
  clearTimer();
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="tp-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="tp-masthead">
      <div class="title-mask">
        <h1 class="gallery-title" data-title-reveal>吞吐互测</h1>
      </div>

      <div class="masthead-actions">
        <UiButton
          variant="ghost"
          :icon="Setting"
          @click="showAdvanced = !showAdvanced"
        >
          {{ showAdvanced ? "收起参数" : "高级参数" }}
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          @click="loadSources()"
        >
          刷新
        </UiButton>
        <UiButton
          v-if="isRunning"
          variant="danger"
          :icon="Close"
          :loading="cancelling"
          @click="cancel"
        >
          停止
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="starting || isRunning"
          :disabled="!canStart"
          @click="start"
        >
          {{ isRunning ? "互测中…" : "开始测速" }}
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Dead-Simple Point-to-Point Selector Strip -->
    <section class="simple-picker">
      <div class="picker-col">
        <span class="picker-label">测试起点（中转）</span>
        <div v-if="candidates.length" class="chip-rail" role="radiogroup" aria-label="测试起点">
          <button
            v-for="s in candidates"
            :key="s.id"
            type="button"
            role="radio"
            class="srv-pill"
            :class="{ 'is-active': settings.sourceIds[0] === s.id }"
            :aria-checked="settings.sourceIds[0] === s.id"
            @click="selectSource(s.id)"
          >
            <i class="s-dot" :class="s.available ? 'is-ok' : 'is-off'"></i>
            <span>{{ s.name }}</span>
          </button>
        </div>
        <span v-else class="muted">
          {{ loading ? "读取服务器中…" : "需至少 2 台在线 Agent 服务器" }}
        </span>
      </div>

      <span class="picker-arrow mono" aria-hidden="true">⇄</span>

      <div class="picker-col">
        <span class="picker-label">目标终点（落地）</span>
        <div v-if="landingCandidates.length" class="chip-rail" role="radiogroup" aria-label="目标终点">
          <button
            v-for="s in landingCandidates"
            :key="s.id"
            type="button"
            role="radio"
            class="srv-pill"
            :class="{ 'is-active': settings.landingId === s.id }"
            :aria-checked="settings.landingId === s.id"
            @click="selectLanding(s.id)"
          >
            <i class="s-dot" :class="s.available ? 'is-ok' : 'is-off'"></i>
            <span>{{ s.name }}</span>
          </button>
        </div>
        <span v-else class="muted">—</span>
      </div>
    </section>

    <!-- Collapsible Advanced Settings (Hidden by Default for Zero-Config Simplicity) -->
    <Transition name="fade-slide">
      <section v-if="showAdvanced" class="advanced-bar">
        <div class="adv-item">
          <span>协议</span>
          <div class="pill-group">
            <button
              type="button"
              class="seg-pill mono"
              :class="{ 'is-active': settings.protocol === 'tcp' }"
              @click="settings.protocol = 'tcp'"
            >
              TCP
            </button>
            <button
              type="button"
              class="seg-pill mono"
              :class="{ 'is-active': settings.protocol === 'udp' }"
              @click="settings.protocol = 'udp'"
            >
              UDP
            </button>
          </div>
        </div>

        <div class="adv-item">
          <span>时长</span>
          <div class="pill-group">
            <button
              v-for="sec in [5, 10, 20]"
              :key="sec"
              type="button"
              class="seg-pill mono"
              :class="{ 'is-active': settings.duration === sec }"
              @click="settings.duration = sec"
            >
              {{ sec }}s
            </button>
          </div>
        </div>

        <div class="adv-item">
          <span>并发</span>
          <div class="pill-group">
            <button
              v-for="st in [1, 4, 8]"
              :key="st"
              type="button"
              class="seg-pill mono"
              :class="{ 'is-active': settings.streams === st }"
              @click="settings.streams = st"
            >
              {{ st }} 线程
            </button>
          </div>
        </div>
      </section>
    </Transition>

    <!-- One-Click Prepare Banner if Selected Server Missing Tool -->
    <div v-if="notReady.length" class="prepare-bar">
      <span>
        {{ notReady.map((s) => s.name).join("、") }} 尚未就绪测速组件
      </span>
      <UiButton
        size="sm"
        variant="primary"
        :icon="Download"
        :loading="preparing"
        :disabled="notReady.every((s) => s.prepareRequested || s.preparing)"
        @click="prepare"
      >
        {{ notReady.every((s) => s.prepareRequested || s.preparing) ? "准备中…" : "一键安装测速工具" }}
      </UiButton>
    </div>

    <KineticNotice :raw="error || sourceError" />

    <!-- Apple-Cinematic Animated Bidirectional Tunnel Stage -->
    <section class="tunnel-stage" :class="{ 'is-running': isRunning }">
      <div class="tunnel-visual">
        <div class="tunnel-node">
          <span class="node-ring" :class="{ 'is-pulse': isRunning }"></span>
          <strong>
            {{
              activeJob?.sourceName ||
              candidates.find((c) => settings.sourceIds.includes(c.id))?.name ||
              "起点服务器"
            }}
          </strong>
          <small class="mono">SOURCE</small>
        </div>

        <div class="tunnel-beams" aria-hidden="true">
          <div class="beam-lane is-forward">
            <span class="beam-line"></span>
            <span v-if="isRunning" class="packet p1"></span>
            <span v-if="isRunning" class="packet p2"></span>
          </div>
          <div class="beam-lane is-reverse">
            <span class="beam-line"></span>
            <span v-if="isRunning" class="packet p1"></span>
            <span v-if="isRunning" class="packet p2"></span>
          </div>
        </div>

        <div class="tunnel-node">
          <span class="node-ring" :class="{ 'is-pulse': isRunning }"></span>
          <strong>{{ landingServer?.name || "终点落地" }}</strong>
          <small class="mono">LANDING</small>
        </div>
      </div>

      <!-- Live Speedometer Readout Strip -->
      <dl class="speed-kpis">
        <div class="kpi-cell">
          <dt>去程峰值（中转 → 落地）</dt>
          <dd class="mono tone-ok">
            {{ bestForward ? formatRate(bestForward) : "—" }}
          </dd>
        </div>
        <div class="kpi-cell">
          <dt>回程峰值（落地 → 中转）</dt>
          <dd class="mono">
            {{ bestReverse ? formatRate(bestReverse) : "—" }}
          </dd>
        </div>
        <div class="kpi-cell">
          <dt>本次传输量</dt>
          <dd class="mono">
            {{ totalBytes ? formatBytes(totalBytes) : "—" }}
          </dd>
        </div>
      </dl>

      <!-- Per-Route Animated Waveform Results (Once Jobs Exist) -->
      <div v-if="jobs.length" class="job-list">
        <article v-for="j in jobs" :key="j.id" class="job-row">
          <div class="job-head">
            <strong>{{ j.sourceName }} ⇄ {{ landingServer?.name || detail?.run.target }}</strong>
            <span class="job-state mono" :class="`is-${j.status}`">
              {{
                j.status === "completed"
                  ? "完成"
                  : j.status === "running"
                    ? "测速中…"
                    : j.status === "failed"
                      ? j.error || "失败"
                      : "排队中"
              }}
            </span>
          </div>

          <div v-if="directions(j).length" class="dir-grid">
            <div v-for="d in directions(j)" :key="d.key" class="dir-card">
              <div class="dir-meta">
                <span>{{ d.label }}</span>
                <strong class="mono">{{ formatRate(d.value.bitsPerSecond) }}</strong>
              </div>
              <svg
                v-if="d.value.intervals?.length"
                class="sparkline"
                viewBox="0 0 240 44"
                preserveAspectRatio="none"
              >
                <polyline
                  :points="curvePoints(d.value.intervals)"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="2"
                  stroke-linecap="round"
                  stroke-linejoin="round"
                />
              </svg>
            </div>
          </div>
        </article>
      </div>
    </section>
  </div>
</template>

<style scoped>
.tp-gallery {
  display: grid;
  gap: 18px;
  max-width: 1180px;
  min-width: 0;
}

.tp-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

/* Simple Picker Strip */
.simple-picker {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.picker-col {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.picker-label {
  font-size: 11.5px;
  color: var(--c-muted);
}

.picker-arrow {
  font-size: 18px;
  color: var(--c-muted);
}

.chip-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.srv-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.srv-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.s-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.s-dot.is-ok {
  background: var(--c-ok);
}

.s-dot.is-off {
  background: var(--c-warn);
}

/* Collapsible Advanced Strip */
.advanced-bar {
  display: flex;
  align-items: center;
  gap: 20px;
  flex-wrap: wrap;
  padding: 12px 18px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
}

.adv-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--c-muted);
}

.pill-group {
  display: inline-flex;
  gap: 3px;
}

.seg-pill {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11.5px;
  cursor: pointer;
}

.seg-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.prepare-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  font-size: 13px;
  color: var(--c-ink);
}

.inline-error {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-bad);
}

/* Animated Bidirectional Tunnel Stage */
.tunnel-stage {
  display: grid;
  gap: 26px;
  padding: 32px;
  border: 1px solid var(--c-line);
  border-radius: 22px;
  background: var(--c-surface);
}

.tunnel-visual {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr) 180px;
  align-items: center;
  gap: 20px;
  padding: 16px 0;
}

.tunnel-node {
  position: relative;
  display: grid;
  justify-items: center;
  gap: 6px;
  padding: 18px 14px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface-muted);
  text-align: center;
}

.tunnel-node strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.tunnel-node small {
  font-size: 10.5px;
  letter-spacing: 0.08em;
  color: var(--c-muted);
}

.node-ring {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--c-ok);
}

.node-ring.is-pulse {
  box-shadow: 0 0 0 6px rgb(21 184 90 / 0.18);
  animation: pulseNode 1.2s infinite ease-in-out;
}

.tunnel-beams {
  display: grid;
  gap: 18px;
}

.beam-lane {
  position: relative;
  height: 4px;
  border-radius: 4px;
  overflow: hidden;
  background: var(--c-line);
}

.beam-line {
  display: block;
  width: 100%;
  height: 100%;
}

.packet {
  position: absolute;
  top: 0;
  width: 28%;
  height: 100%;
  border-radius: 999px;
}

.beam-lane.is-forward .packet {
  background: linear-gradient(90deg, transparent, var(--c-ok), transparent);
  animation: flyRight 1.2s infinite linear;
}

.beam-lane.is-reverse .packet {
  background: linear-gradient(90deg, transparent, var(--c-ink), transparent);
  animation: flyLeft 1.2s infinite linear;
}

.packet.p2 {
  animation-delay: 0.6s !important;
}

/* Speedometer KPI Strip */
.speed-kpis {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  padding-top: 22px;
  border-top: 1px solid var(--c-line);
}

.kpi-cell {
  display: grid;
  gap: 5px;
  padding: 0 20px;
  border-left: 1px solid var(--c-line);
}

.kpi-cell:first-child {
  padding-left: 0;
  border-left: 0;
}

.kpi-cell dt {
  font-size: 12px;
  color: var(--c-muted);
}

.kpi-cell dd {
  margin: 0;
  font-size: 24px;
  font-weight: 500;
  color: var(--c-ink);
}

.tone-ok {
  color: var(--c-ok);
}

/* Job Waveforms */
.job-list {
  display: grid;
  gap: 14px;
  padding-top: 18px;
  border-top: 1px solid var(--c-line);
}

.job-row {
  display: grid;
  gap: 12px;
}

.job-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
}

.job-state {
  font-size: 12px;
  color: var(--c-muted);
}

.job-state.is-completed {
  color: var(--c-ok);
}

.job-state.is-failed {
  color: var(--c-bad);
}

.dir-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.dir-card {
  display: grid;
  gap: 8px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-muted);
}

.dir-meta {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  font-size: 12.5px;
  color: var(--c-muted);
}

.dir-meta strong {
  font-size: 16px;
  color: var(--c-ink);
}

.sparkline {
  width: 100%;
  height: 44px;
  color: var(--c-ok);
}

.muted {
  font-size: 12.5px;
  color: var(--c-muted);
}

@keyframes flyRight {
  0% {
    left: -30%;
  }
  100% {
    left: 110%;
  }
}

@keyframes flyLeft {
  0% {
    left: 110%;
  }
  100% {
    left: -30%;
  }
}

@keyframes pulseNode {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.25);
    opacity: 0.6;
  }
}

@media (max-width: 820px) {
  .simple-picker {
    grid-template-columns: 1fr;
  }
  .picker-arrow {
    display: none;
  }
  .tunnel-visual {
    grid-template-columns: 1fr;
  }
  .speed-kpis,
  .dir-grid {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .kpi-cell {
    padding: 0;
    border-left: 0;
  }
}
</style>
