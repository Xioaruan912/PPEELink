<script setup lang="ts">
import {
  computed,
  onActivated,
  onBeforeUnmount,
  onDeactivated,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRoute } from "vue-router";
import gsap from "gsap";
import { Download, Position, Refresh } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import { getNodeOverview } from "@/api/subcription/node";
import {
  getTraceSources,
  getTraceDetail,
  createTrace,
  cancelTrace,
  prepareTraceAgents,
  type TraceSources,
  type TraceDetail,
} from "@/api/network/trace";
import PathMap from "./PathMap.vue";
import BidirectionalTrace from "./BidirectionalTrace.vue";

defineOptions({ name: "NetworkTraceroute" });

const route = useRoute();
const activeTab = ref<"bidirectional" | "multi">(
  route.query.target || route.query.agentId || route.query.sourceAgentId
    ? "multi"
    : "bidirectional"
);
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const target = ref("1.1.1.1");
const includePanel = ref(true);
const agentIds = ref<number[]>([]);
const probeIds = ref<number[]>([]);
const protocol = ref<"icmp" | "tcp" | "udp">("icmp");
const port = ref(443);
const family = ref<"auto" | "4" | "6">("4");

const cnPresets = [
  { label: "百度", host: "baidu.com" },
  { label: "阿里 DNS", host: "223.5.5.5" },
  { label: "腾讯", host: "qq.com" },
  { label: "广州电信", host: "gd.189.cn" },
];
const globalPresets = [
  { label: "Cloudflare", host: "1.1.1.1" },
  { label: "Google", host: "8.8.8.8" },
  { label: "GitHub", host: "github.com" },
  { label: "OpenAI", host: "api.openai.com" },
];

const legacyOptions = computed(
  () =>
    protocol.value === "udp" && port.value === 33434 && family.value === "auto"
);

function supportsOptions(source: {
  available: boolean;
  optionsSupported?: boolean;
  preparing?: boolean;
}) {
  return (
    source.available &&
    !source.preparing &&
    (source.optionsSupported || legacyOptions.value)
  );
}

function prefill() {
  const q = route.query;
  if (typeof q.target === "string" && q.target.trim()) target.value = q.target;
  const raw = q.sourceAgentId || q.agentId || q.agentIds;
  if (typeof raw === "string") {
    agentIds.value = raw
      .split(",")
      .map(Number)
      .filter((id) => Number.isInteger(id) && id > 0);
    includePanel.value = false;
  }
  if (q.family === "4" || q.family === "6") family.value = q.family;
}
watch(() => route.query, prefill, { immediate: true });

watch(protocol, (value) => {
  if (value === "tcp") port.value = 443;
  if (value === "udp") port.value = 33434;
});

watch([protocol, port, family], () => {
  agentIds.value = agentIds.value.filter((id) =>
    sources.value.agents.some((a) => a.id === id && supportsOptions(a))
  );
  probeIds.value = probeIds.value.filter((id) =>
    sources.value.probes.some((p) => p.id === id && supportsOptions(p))
  );
});

const sources = ref<TraceSources>({ panelTool: "", probes: [], agents: [] });
const panelNodes = ref<{ id: number; name: string; server: string }[]>([]);
const detail = ref<TraceDetail | null>(null);
const selectedId = ref<number>();
const loading = ref(true);
const starting = ref(false);
const cancelling = ref(false);
const sourceError = ref("");
const detailError = ref("");
const preparingId = ref<number>();

let prepareTimer: ReturnType<typeof setTimeout> | undefined;
let timer: ReturnType<typeof setTimeout> | undefined;
let visible = true;
let detailSequence = 0;

function clearPrepareTimer() {
  if (prepareTimer) clearTimeout(prepareTimer);
  prepareTimer = undefined;
}

function schedulePreparation() {
  clearPrepareTimer();
  if (visible && sources.value.agents.some((a) => a.preparing))
    prepareTimer = setTimeout(() => void refreshPreparation(), 3000);
}

async function refreshPreparation() {
  try {
    const res = await getTraceSources();
    sources.value = res.data;
  } catch {
    /* ignore */
  } finally {
    schedulePreparation();
  }
}

async function prepareAgent(id: number) {
  preparingId.value = id;
  detailError.value = "";
  try {
    await prepareTraceAgents([id]);
    ElMessage.success("已下发 NextTrace 安装任务");
    await refreshPreparation();
  } catch {
    ElMessage.error("无法下发安装任务，请确认 Agent 在线");
  } finally {
    preparingId.value = undefined;
  }
}

function installPanelTool() {
  const cmd = "curl -sL nxtrace.org/nt | bash";
  navigator.clipboard
    .writeText(cmd)
    .then(() =>
      ElMessage.info("后端暂未开放面板端自动安装；已复制 NextTrace 一键安装命令")
    )
    .catch(() =>
      ElMessage.info("后端暂未开放面板端自动安装，请在服务器运行：curl -sL nxtrace.org/nt | bash")
    );
}

const running = computed(
  () =>
    detail.value?.views.some(
      (v) => v.status === "queued" || v.status === "running"
    ) || false
);

const remoteCount = computed(
  () => agentIds.value.length + probeIds.value.length
);

const canStart = computed(
  () =>
    !loading.value &&
    !sourceError.value &&
    !starting.value &&
    !running.value &&
    target.value.trim().length > 0 &&
    (includePanel.value || remoteCount.value > 0)
);

function toggleAgent(id: number) {
  if (agentIds.value.includes(id)) {
    agentIds.value = agentIds.value.filter((x) => x !== id);
  } else if (remoteCount.value < 8) {
    agentIds.value = [...agentIds.value, id];
  }
}

function toggleProbe(id: number) {
  if (probeIds.value.includes(id)) {
    probeIds.value = probeIds.value.filter((x) => x !== id);
  } else if (remoteCount.value < 8) {
    probeIds.value = [...probeIds.value, id];
  }
}

function clearTimer() {
  if (timer) clearTimeout(timer);
  timer = undefined;
}

function schedule() {
  clearTimer();
  if (visible && running.value)
    timer = setTimeout(() => void loadDetail(), 2000);
}

async function loadSources() {
  loading.value = true;
  sourceError.value = "";
  try {
    const [srcRes, nodeRes] = await Promise.allSettled([
      getTraceSources(),
      getNodeOverview(),
    ]);
    if (srcRes.status === "fulfilled") {
      sources.value = srcRes.value.data;
      agentIds.value = agentIds.value.filter((id) =>
        sources.value.agents.some((a) => a.id === id && supportsOptions(a))
      );
      probeIds.value = probeIds.value.filter((id) =>
        sources.value.probes.some((p) => p.id === id && supportsOptions(p))
      );
      if (!agentIds.value.length) {
        const firstReady = sources.value.agents.find((a) => supportsOptions(a));
        if (firstReady) agentIds.value = [firstReady.id];
      }
      includePanel.value = !agentIds.value.length && Boolean(sources.value.panelTool);
    } else {
      sourceError.value = "无法读取追踪来源";
    }
    if (nodeRes.status === "fulfilled") {
      const seenServers = new Set<string>();
      panelNodes.value = (nodeRes.value.data || [])
        .filter((n: any) => {
          const srv = String(n?.server || "").trim().toLowerCase();
          if (!srv || seenServers.has(srv)) return false;
          seenServers.add(srv);
          return true;
        })
        .map((n: any) => ({ id: n.id, name: n.name, server: n.server }));
    }
  } finally {
    loading.value = false;
    schedulePreparation();
  }
}

async function loadDetail() {
  if (!selectedId.value) return;
  const id = selectedId.value;
  const seq = ++detailSequence;
  detailError.value = "";
  try {
    const res = await getTraceDetail(id);
    if (seq !== detailSequence || id !== selectedId.value) return;
    detail.value = res.data;
  } catch {
    if (seq === detailSequence) detailError.value = "追踪结果读取失败";
  } finally {
    if (seq === detailSequence) schedule();
  }
}

async function start() {
  if (!canStart.value) return;
  starting.value = true;
  detailError.value = "";
  try {
    const res = await createTrace({
      target: target.value.trim(),
      includePanel: includePanel.value,
      probeIds: probeIds.value,
      agentIds: agentIds.value,
      protocol: protocol.value,
      port: protocol.value === "icmp" ? 0 : port.value,
      family: family.value,
    });
    clearTimer();
    detailSequence++;
    selectedId.value = res.data.id;
    detail.value = null;
    await loadDetail();
  } catch {
    detailError.value = "发起追踪失败，请确认目标与来源";
  } finally {
    starting.value = false;
  }
}

async function cancel() {
  if (!selectedId.value) return;
  cancelling.value = true;
  try {
    await cancelTrace(selectedId.value);
    await loadDetail();
  } finally {
    cancelling.value = false;
  }
}

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
  schedulePreparation();
});
onDeactivated(() => {
  visible = false;
  clearTimer();
  clearPrepareTimer();
});
onBeforeUnmount(() => {
  visible = false;
  detailSequence++;
  clearTimer();
  clearPrepareTimer();
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="trace-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="trace-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>路由追踪</h1>
        </div>

        <div class="pill-group mode-tabs" role="tablist" aria-label="追踪模式">
          <button
            type="button"
            role="tab"
            class="seg-pill"
            :class="{ 'is-active': activeTab === 'bidirectional' }"
            :aria-selected="activeTab === 'bidirectional'"
            @click="activeTab = 'bidirectional'"
          >
            本机双向路由
          </button>
          <button
            type="button"
            role="tab"
            class="seg-pill"
            :class="{ 'is-active': activeTab === 'multi' }"
            :aria-selected="activeTab === 'multi'"
            @click="activeTab = 'multi'"
          >
            多源单向追踪
          </button>
        </div>
      </div>

      <div v-if="activeTab === 'multi'" class="masthead-actions">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          @click="loadSources"
        >
          刷新来源
        </UiButton>
        <UiButton
          v-if="running"
          variant="danger"
          :loading="cancelling"
          @click="cancel"
        >
          取消追踪
        </UiButton>
        <UiButton
          variant="primary"
          :icon="Position"
          :loading="starting || running"
          :disabled="!canStart"
          @click="start"
        >
          {{ running ? "追踪中…" : "开始追踪" }}
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <BidirectionalTrace v-show="activeTab === 'bidirectional'" />

    <div v-show="activeTab === 'multi'" class="multi-trace-stack">
    <!-- Compact Target + Protocol + Quick Presets Deck -->
    <section class="control-deck">
      <div class="target-row">
        <div class="target-input-wrap">
          <el-input
            v-model="target"
            placeholder="输入目标域名或 IP（或点击下方预设）"
            maxlength="253"
            :disabled="starting"
            @keyup.enter="start"
          />
        </div>

        <div class="pill-group" role="group" aria-label="探测协议">
          <button
            v-for="p in (['icmp', 'tcp', 'udp'] as const)"
            :key="p"
            type="button"
            class="seg-pill mono"
            :class="{ 'is-active': protocol === p }"
            @click="protocol = p"
          >
            {{ p.toUpperCase() }}
          </button>
        </div>

        <div class="pill-group" role="group" aria-label="IP 版本">
          <button
            type="button"
            class="seg-pill mono"
            :class="{ 'is-active': family === '4' }"
            @click="family = '4'"
          >
            IPv4
          </button>
          <button
            type="button"
            class="seg-pill mono"
            :class="{ 'is-active': family === '6' }"
            @click="family = '6'"
          >
            IPv6
          </button>
        </div>
      </div>

      <!-- 1-Click Domestic & Global Standard Target Presets -->
      <div class="preset-row">
        <span class="row-label">国内</span>
        <button
          v-for="item in cnPresets"
          :key="item.host"
          type="button"
          class="preset-chip"
          :class="{ 'is-active': target === item.host }"
          @click="target = item.host"
        >
          <span>{{ item.label }}</span>
          <small class="mono">{{ item.host }}</small>
        </button>

        <span class="row-sep" aria-hidden="true"></span>

        <span class="row-label">国外</span>
        <button
          v-for="item in globalPresets"
          :key="item.host"
          type="button"
          class="preset-chip"
          :class="{ 'is-active': target === item.host }"
          @click="target = item.host"
        >
          <span>{{ item.label }}</span>
          <small class="mono">{{ item.host }}</small>
        </button>
      </div>

      <!-- My Nodes Quick Targets -->
      <div v-if="panelNodes.length" class="preset-row">
        <span class="row-label">我的节点</span>
        <button
          v-for="n in panelNodes"
          :key="n.id"
          type="button"
          class="preset-chip"
          :class="{ 'is-active': target === n.server }"
          @click="target = n.server"
        >
          <span>{{ n.name }}</span>
          <small class="mono">{{ n.server }}</small>
        </button>
      </div>

      <!-- Trace Sources Strip (Remote Agents & Probes, NOT Panel Server First) -->
      <div class="sources-row">
        <span class="row-label">追踪节点</span>

        <!-- Remote Agents -->
        <div
          v-for="agent in sources.agents"
          :key="`agent-${agent.id}`"
          class="source-chip"
          :class="{ 'is-selected': agentIds.includes(agent.id) }"
        >
          <button
            type="button"
            class="source-main"
            :disabled="!supportsOptions(agent)"
            @click="toggleAgent(agent.id)"
          >
            <i class="s-dot" :class="supportsOptions(agent) ? 'is-ok' : 'is-off'"></i>
            <span>{{ agent.name }}</span>
            <small v-if="agent.tool" class="mono">{{ agent.tool }}</small>
          </button>
          <button
            v-if="(!agent.available || !agent.enriched) && agent.optionsSupported"
            type="button"
            class="install-btn"
            :disabled="!!agent.preparing || preparingId === agent.id"
            @click="prepareAgent(agent.id)"
          >
            <el-icon><Download /></el-icon>
            <span>{{ agent.preparing || preparingId === agent.id ? "安装中…" : "安装 NextTrace" }}</span>
          </button>
        </div>

        <!-- Remote Probes -->
        <div
          v-for="probe in sources.probes"
          :key="`probe-${probe.id}`"
          class="source-chip"
          :class="{ 'is-selected': probeIds.includes(probe.id) }"
        >
          <button
            type="button"
            class="source-main"
            :disabled="!supportsOptions(probe)"
            @click="toggleProbe(probe.id)"
          >
            <i class="s-dot" :class="supportsOptions(probe) ? 'is-ok' : 'is-off'"></i>
            <span>{{ probe.name }}</span>
          </button>
        </div>

        <span v-if="!sources.agents.length && !sources.probes.length && !loading" class="muted-source">
          尚未接入远程 Agent 服务器
          <button
            v-if="!sources.panelTool"
            type="button"
            class="install-inline-btn"
            @click="installPanelTool"
          >
            <el-icon><Download /></el-icon>
            <span>安装 NextTrace</span>
          </button>
        </span>
      </div>
    </section>

    <KineticNotice :raw="detailError || sourceError" />

    <!-- Full-Width Upgraded Dot-Matrix World Route Map -->
    <PathMap
      :views="detail?.views || []"
      :target="detail?.run.target || target"
      :running="running"
    />
    </div>
  </div>
</template>

<style scoped>
.trace-gallery {
  display: grid;
  gap: 18px;
  max-width: 1220px;
  min-width: 0;
}

.multi-trace-stack {
  display: grid;
  gap: 18px;
}

.trace-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: center;
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

/* Compact Control Deck */
.control-deck {
  display: grid;
  gap: 12px;
}

.target-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.target-input-wrap {
  flex: 1 1 280px;
  min-width: 0;
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
  font-size: 12px;
  cursor: pointer;
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.preset-row,
.sources-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.row-label {
  font-size: 11.5px;
  font-weight: 500;
  color: var(--c-muted);
  margin-right: 2px;
}

.row-sep {
  width: 1px;
  height: 14px;
  background: var(--c-line);
  margin: 0 4px;
}

.preset-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.preset-chip small {
  font-size: 11px;
  color: var(--c-muted);
}

.preset-chip:hover {
  border-color: var(--c-ink);
}

.preset-chip.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.preset-chip.is-active small {
  color: color-mix(in srgb, var(--c-surface) 72%, transparent);
}

/* Source Chips + One-Click Install */
.source-chip {
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  overflow: hidden;
  transition: border-color var(--duration-fast) var(--ease-out);
}

.source-chip.is-selected {
  border-color: var(--c-ink);
  background: var(--c-ink);
}

.source-main {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
}

.source-chip.is-selected .source-main {
  color: var(--c-surface);
}

.source-main:disabled {
  cursor: default;
  opacity: 0.65;
}

.source-main small {
  font-size: 11px;
  opacity: 0.7;
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

.install-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 11px;
  border: 0;
  border-left: 1px solid var(--c-line);
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
}

.install-btn:hover,
.install-inline-btn:hover {
  background: var(--c-ink);
  color: var(--c-surface);
}

.muted-source {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--c-muted);
}

.install-inline-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
}

.inline-error {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-bad);
}
</style>
