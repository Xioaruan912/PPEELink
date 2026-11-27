<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import gsap from "gsap";
import { ElMessage } from "element-plus";
import { Refresh, VideoPlay } from "@element-plus/icons-vue";
import { UiButton, UiDrawer } from "@/ui";
import {
  getProbeClashBatchStatus,
  getProbeClashStatus,
  pushProbeConfig,
  runProbeNetDiag,
  startProbeClashBatch,
  switchProbeClashNode,
  type ClashNodeTestItem,
  type ClashStatus,
  type NetDiagReport,
  type Probe,
} from "@/api/subcription/probe";

const props = defineProps<{
  modelValue: boolean;
  probes: Probe[];
  activeProbeId?: number;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "refresh"): void;
}>();

const open = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const onlineProbes = computed(() => props.probes.filter((p) => p.online));
const selectedProbeId = ref<number | undefined>(undefined);
const loadingStatus = ref(false);
const switchingNode = ref("");
const batchRunning = ref(false);
const batchTotal = ref(0);
const currentTestingNode = ref("");
const errorMsg = ref("");
const status = ref<ClashStatus | null>(null);
const results = ref<ClashNodeTestItem[]>([]);
const showConfig = ref(false);
const clashApiInput = ref("http://127.0.0.1:9090");
const clashSecretInput = ref("");
const savingConfig = ref(false);
const diagLoading = ref(false);
const diagReport = ref<NetDiagReport | null>(null);

const matrixListEl = ref<HTMLElement>();
const progressBarEl = ref<HTMLElement>();
const diagCardEl = ref<HTMLElement>();
let pollTimer: ReturnType<typeof setInterval> | null = null;
let lastAnimatedCount = 0;

const activeProbe = computed(() =>
  props.probes.find((p) => p.id === selectedProbeId.value)
);

const progressRatio = computed(() => {
  const total = batchTotal.value || status.value?.proxies?.length || 0;
  if (!total) return 0;
  return Math.min(1, results.value.length / total);
});

function prefersReducedMotion() {
  return window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
}

function animateProgressFill(ratio: number) {
  if (!progressBarEl.value) return;
  if (prefersReducedMotion()) {
    progressBarEl.value.style.transform = `scaleX(${ratio})`;
    return;
  }
  gsap.to(progressBarEl.value, {
    scaleX: Math.max(0.03, ratio),
    duration: 0.42,
    ease: "power3.out",
    overwrite: "auto",
  });
}

function animateNewRows(prevCount: number) {
  if (!matrixListEl.value || prefersReducedMotion()) return;
  const rows = matrixListEl.value.querySelectorAll<HTMLElement>("[data-stream-row]");
  const fresh: HTMLElement[] = [];
  rows.forEach((el, idx) => {
    if (idx >= prevCount) fresh.push(el);
  });
  if (!fresh.length) return;
  gsap.fromTo(
    fresh,
    { y: 12, opacity: 0, scale: 0.99 },
    {
      y: 0,
      opacity: 1,
      scale: 1,
      duration: 0.4,
      stagger: 0.045,
      ease: "power3.out",
      clearProps: "transform,opacity",
    }
  );
}

function stopBatchPoll() {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
}

function syncClashInputs() {
  clashApiInput.value =
    status.value?.controller ||
    activeProbe.value?.clashApi ||
    "http://127.0.0.1:9090";
}

async function fetchStatus() {
  if (!onlineProbes.value.length) return;
  loadingStatus.value = true;
  errorMsg.value = "";
  try {
    const res = await getProbeClashStatus(selectedProbeId.value);
    status.value = res.data?.status || { connected: false };
    syncClashInputs();
    if (!status.value.connected) {
      errorMsg.value =
        status.value.error ||
        "未检测到本机 Clash / Mihomo 外部控制接口（9090 / 9097），如已设置访问密钥请在下方填写";
      showConfig.value = true;
    } else {
      showConfig.value = false;
    }
  } catch (e: any) {
    errorMsg.value = e?.response?.data?.msg || e?.message || "读取 Clash 状态失败";
    showConfig.value = true;
  } finally {
    loadingStatus.value = false;
  }
}

async function saveAndReconnect() {
  if (!selectedProbeId.value || savingConfig.value) return;
  let api = clashApiInput.value.trim();
  if (api && !api.startsWith("http://") && !api.startsWith("https://")) {
    api = "http://" + api;
    clashApiInput.value = api;
  }
  savingConfig.value = true;
  try {
    const { data } = await pushProbeConfig({
      probeIds: [selectedProbeId.value],
      clashApi: api,
      clashSecret: clashSecretInput.value.trim(),
    });
    if ((data?.okCount || 0) > 0) {
      ElMessage.success("Clash 连接参数已下发，正在重连…");
    } else {
      ElMessage.warning("参数已保存，请先将用户 Agent 升级至最新版本后重试");
    }
    emit("refresh");
    await fetchStatus();
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || e?.message || "保存失败");
  } finally {
    savingConfig.value = false;
  }
}

async function handleSwitch(nodeName: string) {
  if (!nodeName || switchingNode.value) return;
  switchingNode.value = nodeName;
  try {
    const res = await switchProbeClashNode({
      probeId: selectedProbeId.value,
      group: status.value?.group,
      node: nodeName,
    });
    if (res.data?.status) status.value = res.data.status;
    ElMessage.success(`已切换至 ${res.data?.switched || nodeName}`);
    emit("refresh");
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || e?.message || "切换失败");
  } finally {
    switchingNode.value = "";
  }
}

async function runBatch() {
  if (batchRunning.value || !status.value?.connected) return;
  stopBatchPoll();
  batchRunning.value = true;
  errorMsg.value = "";
  results.value = [];
  lastAnimatedCount = 0;
  batchTotal.value = Math.min(status.value.proxies?.length || 0, 16);
  currentTestingNode.value = status.value.proxies?.[0] || "";
  await nextTick();
  animateProgressFill(0.04);

  try {
    const startRes = await startProbeClashBatch({
      probeId: selectedProbeId.value,
      group: status.value.group,
    });
    const jobId = startRes.data?.jobId;
    if (!jobId) throw new Error("未能启动流式实测任务");

    const startedAt = Date.now();
    pollTimer = setInterval(async () => {
      if (Date.now() - startedAt > 125_000) {
        stopBatchPoll();
        batchRunning.value = false;
        currentTestingNode.value = "";
        errorMsg.value = "全节点实测超时，请确认用户 Agent 在线";
        return;
      }
      try {
        const stRes = await getProbeClashBatchStatus(jobId);
        const snap = stRes.data;
        if (!snap) return;
        if (snap.total) batchTotal.value = snap.total;
        currentTestingNode.value = snap.currentNode || "";
        const nextItems = snap.items || [];
        if (nextItems.length > lastAnimatedCount) {
          const prev = lastAnimatedCount;
          results.value = nextItems;
          lastAnimatedCount = nextItems.length;
          await nextTick();
          animateNewRows(prev);
          animateProgressFill(progressRatio.value);
        }
        if (snap.done) {
          stopBatchPoll();
          batchRunning.value = false;
          currentTestingNode.value = "";
          if (snap.status) status.value = snap.status;
          if (snap.error && !nextItems.length) {
            errorMsg.value = snap.error;
          } else {
            animateProgressFill(1);
            const savedTip = snap.savedSnapshots
              ? `，已融合更新 ${snap.savedSnapshots} 个节点解锁矩阵`
              : "";
            ElMessage.success(
              `已完成 ${nextItems.length} 个节点实测并恢复原节点${savedTip}`
            );
            emit("refresh");
          }
        }
      } catch {
        // 单次轮询抖动静默重试
      }
    }, 850);
  } catch (e: any) {
    stopBatchPoll();
    batchRunning.value = false;
    currentTestingNode.value = "";
    errorMsg.value = e?.response?.data?.msg || e?.message || "全节点实测失败";
  }
}

async function runNetDiagnostic() {
  if (diagLoading.value || !onlineProbes.value.length) return;
  diagLoading.value = true;
  errorMsg.value = "";
  try {
    const res = await runProbeNetDiag(selectedProbeId.value);
    diagReport.value = res.data || null;
    await nextTick();
    if (diagCardEl.value && !prefersReducedMotion()) {
      const items = diagCardEl.value.querySelectorAll<HTMLElement>("[data-diag-item]");
      gsap.fromTo(
        items,
        { y: 10, opacity: 0 },
        {
          y: 0,
          opacity: 1,
          duration: 0.36,
          stagger: 0.05,
          ease: "power3.out",
          clearProps: "transform,opacity",
        }
      );
    }
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.msg || e?.message || "网络体检失败（需用户 Agent v1.14+）");
  } finally {
    diagLoading.value = false;
  }
}

function unlockBadge(raw?: string): { text: string; tone: "ok" | "warn" | "bad" | "muted" } {
  if (!raw) return { text: "—", tone: "muted" };
  const [st, region] = raw.split(":");
  if (st === "available") return { text: region || "✓", tone: "ok" };
  if (st === "originals") return { text: region ? `自制·${region}` : "自制", tone: "warn" };
  if (st === "blocked") return { text: "✕", tone: "bad" };
  return { text: "—", tone: "muted" };
}

watch(
  () => props.modelValue,
  (val) => {
    if (val) {
      selectedProbeId.value =
        props.activeProbeId || onlineProbes.value[0]?.id;
      void fetchStatus();
    } else {
      stopBatchPoll();
    }
  }
);

onBeforeUnmount(() => {
  stopBatchPoll();
});
</script>

<template>
  <UiDrawer
    v-model="open"
    title="Clash 联动 · 全节点真测速与解锁"
    size="min(980px, 96vw)"
    append-to-body
  >
    <div class="clash-shell">
      <header class="clash-bar">
        <div class="bar-left">
          <select
            v-if="onlineProbes.length > 1"
            v-model="selectedProbeId"
            class="probe-select"
            :disabled="batchRunning"
            @change="fetchStatus"
          >
            <option v-for="p in onlineProbes" :key="p.id" :value="p.id">
              {{ p.name }}
            </option>
          </select>

          <span
            v-if="status?.connected"
            class="conn-pill is-ok mono"
          >
            <i></i>
            {{ status.group || "Selector" }} · {{ status.current || "—" }}
          </span>
          <span v-else class="conn-pill">
            <i></i>
            未连接
          </span>
        </div>

        <div class="bar-right">
          <UiButton
            variant="ghost"
            :loading="diagLoading"
            :disabled="batchRunning || !onlineProbes.length"
            @click="runNetDiagnostic"
          >
            网络体检
          </UiButton>
          <UiButton
            variant="ghost"
            :disabled="batchRunning"
            @click="showConfig = !showConfig"
          >
            连接设置
          </UiButton>
          <UiButton
            variant="ghost"
            :icon="Refresh"
            :loading="loadingStatus"
            :disabled="batchRunning"
            @click="fetchStatus"
          >
            刷新
          </UiButton>
          <UiButton
            variant="primary"
            :icon="VideoPlay"
            :loading="batchRunning"
            :disabled="!status?.connected || !(status.proxies?.length)"
            @click="runBatch"
          >
            {{ batchRunning ? "轮切测速中…" : "一键全节点实测" }}
          </UiButton>
        </div>
      </header>

      <div v-if="errorMsg" class="err-strip">{{ errorMsg }}</div>

      <section v-if="showConfig" class="clash-cfg-card">
        <div class="clash-cfg-fields">
          <el-input
            v-model="clashApiInput"
            placeholder="http://127.0.0.1:9090"
            clearable
          />
          <el-input
            v-model="clashSecretInput"
            type="password"
            show-password
            :placeholder="
              activeProbe?.hasClashSecret
                ? 'API 访问密钥（已配置，留空保持不变）'
                : 'API 访问密钥（如有）'
            "
            clearable
            @keyup.enter="saveAndReconnect"
          />
          <UiButton
            variant="primary"
            size="sm"
            :loading="savingConfig"
            @click="saveAndReconnect"
          >
            保存并重连
          </UiButton>
        </div>
      </section>

      <!-- Quick Node Selector Rail -->
      <section v-if="status?.connected && status.proxies?.length" class="node-rail">
        <button
          v-for="name in status.proxies"
          :key="name"
          type="button"
          class="node-pill"
          :class="{ 'is-active': status.current === name }"
          :disabled="batchRunning || switchingNode === name"
          @click="handleSwitch(name)"
        >
          {{ name }}
        </button>
      </section>

      <!-- 3-Second Proxy / DNS / TUN Self-Diagnostic Sheet -->
      <section v-if="diagReport" ref="diagCardEl" class="diag-card">
        <div class="diag-head">
          <strong>{{ diagReport.summary }}</strong>
          <button type="button" class="diag-close" @click="diagReport = null">收起</button>
        </div>
        <div class="diag-list">
          <div
            v-for="item in diagReport.checks"
            :key="item.key"
            data-diag-item
            class="diag-row"
          >
            <span class="diag-dot" :class="`is-${item.status}`" aria-hidden="true"></span>
            <div class="diag-copy">
              <div class="diag-line">
                <strong>{{ item.label }}</strong>
                <span class="mono diag-detail">{{ item.detail }}</span>
              </div>
              <p v-if="item.advice" class="diag-advice">{{ item.advice }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Streaming Progress Banner -->
      <section v-if="batchRunning" class="batch-progress-wrap" aria-live="polite">
        <div class="batch-progress">
          <span class="bp-dot" aria-hidden="true"></span>
          <span class="bp-text">
            <template v-if="currentTestingNode">
              正在实测 <strong class="mono">{{ currentTestingNode }}</strong>
            </template>
            <template v-else>
              正在全局模式逐一实测节点真实带宽与解锁…
            </template>
          </span>
          <span class="bp-count mono">
            {{ results.length }} / {{ batchTotal || status?.proxies?.length || 0 }}
          </span>
        </div>
        <div class="bp-track">
          <div ref="progressBarEl" class="bp-fill"></div>
        </div>
      </section>

      <!-- Batch Matrix Table -->
      <section v-if="results.length || (batchRunning && currentTestingNode)" class="matrix-card">
        <div ref="matrixListEl" class="matrix-table">
          <div class="m-row is-head">
            <span class="col-left">节点</span>
            <span class="col-center">出口</span>
            <span class="col-center">TTFB</span>
            <span class="col-center">下载</span>
            <span class="col-center">GPT</span>
            <span class="col-center">Claude</span>
            <span class="col-center">Gemini</span>
            <span class="col-center">NF</span>
            <span class="col-center">Disney</span>
            <span class="col-center">YT</span>
            <span class="col-right">操作</span>
          </div>

          <div
            v-for="item in results"
            :key="item.name"
            data-stream-row
            class="m-row"
          >
            <strong class="m-name col-left" :title="item.name">{{ item.name }}</strong>
            <span class="mono m-sub col-center">{{ item.country || "—" }}</span>
            <span class="mono col-center">{{ item.ttfbMs ? `${item.ttfbMs} ms` : "—" }}</span>
            <strong class="mono m-speed col-center">
              {{ item.downloadMbps ? `${item.downloadMbps.toFixed(1)} Mbps` : "—" }}
            </strong>
            <span
              v-for="(val, k) in [item.chatgpt, item.claude, item.gemini, item.netflix, item.disney, item.youtube]"
              :key="k"
              class="u-tag mono"
              :class="`tone-${unlockBadge(val).tone}`"
            >
              {{ unlockBadge(val).text }}
            </span>
            <div class="col-right">
              <button
                type="button"
                class="sw-btn"
                :class="{ 'is-now': status?.current === item.name }"
                :disabled="batchRunning || switchingNode === item.name"
                @click="handleSwitch(item.name)"
              >
                {{ status?.current === item.name ? "当前" : "切换" }}
              </button>
            </div>
          </div>

          <!-- Live Scanning Placeholder Row -->
          <div
            v-if="batchRunning && currentTestingNode && !results.some((r) => r.name === currentTestingNode)"
            class="m-row is-scanning"
          >
            <strong class="m-name col-left">{{ currentTestingNode }}</strong>
            <span class="scan-bar" style="grid-column: 2 / -1"></span>
          </div>
        </div>
      </section>

      <div v-else-if="!batchRunning && status?.connected" class="empty-stage">
        点击「一键全节点实测」自动轮切并测量全部 {{ status.proxies?.length || 0 }} 个节点的真实带宽与解锁
      </div>
    </div>
  </UiDrawer>
</template>

<style scoped>
.clash-shell {
  display: grid;
  gap: 16px;
}

.clash-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.bar-left,
.bar-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.probe-select {
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
}

.conn-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface-muted);
  font-size: 12px;
  color: var(--c-muted);
}

.conn-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-muted);
}

.conn-pill.is-ok {
  color: var(--c-ink);
}

.conn-pill.is-ok i {
  background: var(--c-ok);
}

.err-strip {
  padding: 10px 14px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-bad) 8%, var(--c-surface));
  color: var(--c-bad);
  font-size: 12.5px;
}

.clash-cfg-card {
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface-2);
}

.clash-cfg-fields {
  display: grid;
  grid-template-columns: 1fr 1fr auto;
  gap: 10px;
  align-items: center;
}

@media (max-width: 640px) {
  .clash-cfg-fields {
    grid-template-columns: 1fr;
  }
}

.node-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.node-pill {
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
  transition: background 150ms ease, color 150ms ease, transform 100ms ease;
}

.node-pill:active {
  transform: scale(0.97);
}

.node-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.batch-progress-wrap {
  display: grid;
  gap: 8px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
}

.batch-progress {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12.5px;
  color: var(--c-ink);
}

.bp-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.bp-count {
  font-size: 12px;
  color: var(--c-muted);
}

.bp-track {
  position: relative;
  height: 3px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-ink) 10%, transparent);
  overflow: hidden;
}

.bp-fill {
  height: 100%;
  width: 100%;
  border-radius: 999px;
  background: var(--c-ink);
  transform: scaleX(0.03);
  transform-origin: left center;
  will-change: transform;
}

.m-row.is-scanning {
  background: color-mix(in srgb, var(--c-ink) 2%, transparent);
}

.scan-bar {
  position: relative;
  height: 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-ink) 8%, transparent);
  overflow: hidden;
}

.scan-bar::after {
  content: "";
  position: absolute;
  inset: 0;
  width: 38%;
  border-radius: 999px;
  background: linear-gradient(
    90deg,
    transparent,
    color-mix(in srgb, var(--c-ink) 22%, transparent),
    transparent
  );
  animation: scan-sweep 1.25s cubic-bezier(0.4, 0, 0.2, 1) infinite;
}

@keyframes scan-sweep {
  from {
    transform: translateX(-120%);
  }
  to {
    transform: translateX(320%);
  }
}

.diag-card {
  display: grid;
  gap: 12px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface-2);
}

.diag-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: var(--c-ink);
}

.diag-close {
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.diag-list {
  display: grid;
  gap: 10px;
}

.diag-row {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding-top: 8px;
  border-top: 1px solid var(--c-line);
}

.diag-row:first-child {
  padding-top: 0;
  border-top: 0;
}

.diag-dot {
  width: 8px;
  height: 8px;
  margin-top: 5px;
  border-radius: 50%;
  flex: none;
}

.diag-dot.is-ok {
  background: var(--c-ok);
}

.diag-dot.is-warn {
  background: var(--c-warn);
}

.diag-dot.is-fail {
  background: var(--c-bad);
}

.diag-copy {
  display: grid;
  gap: 3px;
  min-width: 0;
  flex: 1;
}

.diag-line {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  font-size: 12.5px;
}

.diag-detail {
  color: var(--c-muted);
  font-size: 12px;
}

.diag-advice {
  margin: 0;
  font-size: 12px;
  color: var(--c-warn);
}

@media (prefers-reduced-motion: reduce) {
  .bp-dot,
  .scan-bar::after {
    animation: none;
  }
}

.bp-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
  flex: none;
  animation: bp-pulse 1.2s ease-in-out infinite;
}

@keyframes bp-pulse {
  0%,
  100% {
    opacity: 0.35;
    transform: scale(0.9);
  }
  50% {
    opacity: 1;
    transform: scale(1.1);
  }
}

.matrix-card {
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  overflow-x: auto;
}

.matrix-table {
  min-width: 720px;
  width: 100%;
  display: grid;
}

.m-row {
  display: grid;
  grid-template-columns: minmax(120px, 1.45fr) 44px 64px 80px repeat(6, minmax(48px, 1fr)) 54px;
  align-items: center;
  gap: 6px;
  padding: 11px 14px;
  border-top: 1px solid var(--c-line);
  font-size: 12px;
}

.m-row.is-head {
  border-top: 0;
  color: var(--c-muted);
  font-size: 11px;
  font-weight: 500;
  background: var(--c-surface-muted);
}

.col-left {
  text-align: left;
  justify-self: start;
}

.col-center {
  text-align: center;
  justify-self: center;
}

.col-right {
  display: flex;
  justify-content: flex-end;
  justify-self: end;
}

.m-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  font-weight: 500;
  max-width: 100%;
}

.m-sub {
  color: var(--c-muted);
}

.m-speed {
  color: var(--c-ink);
  white-space: nowrap;
}

.u-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 24px;
  padding: 2px 6px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 500;
  white-space: nowrap;
}

.u-tag.tone-ok {
  background: color-mix(in srgb, var(--c-ok) 14%, transparent);
  color: var(--c-ok);
}

.u-tag.tone-warn {
  background: color-mix(in srgb, var(--c-warn) 15%, transparent);
  color: var(--c-warn);
}

.u-tag.tone-bad {
  background: color-mix(in srgb, var(--c-bad) 12%, transparent);
  color: var(--c-bad);
}

.u-tag.tone-muted {
  color: var(--c-muted);
}

.sw-btn {
  padding: 4px 8px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  background: transparent;
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
}

.sw-btn.is-now {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.empty-stage {
  padding: 44px 16px;
  text-align: center;
  color: var(--c-muted);
  font-size: 13px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
}
</style>
