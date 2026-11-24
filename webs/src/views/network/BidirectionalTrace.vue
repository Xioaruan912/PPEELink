<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { Position, Refresh } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import { getNodeOverview } from "@/api/subcription/node";
import {
  getProbes,
  runBidirectionalTrace,
  type BidirectionalTraceResponse,
  type Probe,
  type UserTraceHop,
} from "@/api/subcription/probe";
import { displayNodeName } from "@/utils/nodeDisplay";
import { countryFlag } from "@/utils/flag";

defineOptions({ name: "BidirectionalTrace" });

interface NodeOption {
  id: number;
  name: string;
  host: string;
  countryCode?: string;
}

const nodes = ref<NodeOption[]>([]);
const probes = ref<Probe[]>([]);
const loadingMeta = ref(true);
const selectedNodeId = ref<number | undefined>(undefined);
const selectedProbeId = ref<number | undefined>(undefined);
const running = ref(false);
const elapsedSec = ref(0);
const errorMsg = ref("");
const result = ref<BidirectionalTraceResponse | null>(null);

let tickTimer: ReturnType<typeof setInterval> | null = null;

const onlineProbes = computed(() => probes.value.filter((p) => p.online));
const canRun = computed(
  () =>
    !running.value &&
    !loadingMeta.value &&
    !!selectedNodeId.value &&
    onlineProbes.value.length > 0
);

function stopTicker() {
  if (tickTimer) {
    clearInterval(tickTimer);
    tickTimer = null;
  }
}

async function loadMeta() {
  loadingMeta.value = true;
  try {
    const [nodeRes, probeRes] = await Promise.allSettled([
      getNodeOverview(),
      getProbes(),
    ]);
    if (nodeRes.status === "fulfilled" && Array.isArray(nodeRes.value.data)) {
      nodes.value = nodeRes.value.data
        .filter((n: any) => n?.id && n?.server)
        .map((n: any) => {
          const clean = displayNodeName(n.name || "");
          const flag = n.countryCode ? countryFlag(n.countryCode) : "";
          return {
            id: n.id,
            name: flag ? `${flag} ${clean}` : clean,
            host: n.server || "",
            countryCode: n.countryCode || "",
          };
        });
      if (
        nodes.value.length &&
        (!selectedNodeId.value ||
          !nodes.value.some((n) => n.id === selectedNodeId.value))
      ) {
        selectedNodeId.value = nodes.value[0].id;
      }
    }
    if (probeRes.status === "fulfilled" && Array.isArray(probeRes.value.data)) {
      probes.value = probeRes.value.data;
      const online = probes.value.filter((p) => p.online);
      if (
        online.length &&
        (!selectedProbeId.value ||
          !online.some((p) => p.id === selectedProbeId.value))
      ) {
        selectedProbeId.value = online[0].id;
      }
    }
  } finally {
    loadingMeta.value = false;
  }
}

async function startTrace() {
  if (!canRun.value || !selectedNodeId.value) return;
  running.value = true;
  errorMsg.value = "";
  elapsedSec.value = 0;
  stopTicker();
  tickTimer = setInterval(() => {
    elapsedSec.value += 1;
  }, 1000);
  try {
    const res = await runBidirectionalTrace({
      nodeId: selectedNodeId.value,
      probeId: selectedProbeId.value,
    });
    result.value = res.data;
  } catch (e: any) {
    errorMsg.value =
      e?.response?.data?.msg ||
      e?.message ||
      "双向路由追踪失败，请确认本机用户 Agent 在线";
  } finally {
    stopTicker();
    running.value = false;
  }
}

function lineTone(line?: string): "ok" | "info" | "warn" | "muted" {
  if (!line) return "muted";
  if (/CN2 GIA|9929|CMIN2/i.test(line)) return "ok";
  if (/CN2 GT|CMI|4837/i.test(line)) return "info";
  if (/163|NTT|IIJ|Cogent|PCCW|HE/i.test(line)) return "warn";
  return "muted";
}

function maxRtt(hops?: UserTraceHop[]): number {
  if (!hops?.length) return 100;
  return Math.max(
    40,
    ...hops.filter((h) => !h.timeout && h.rtt > 0).map((h) => h.rtt)
  );
}

const outMaxRtt = computed(() => maxRtt(result.value?.outbound?.hops));
const retMaxRtt = computed(() => maxRtt(result.value?.returnPath?.hops));

function hopWidth(hop: UserTraceHop, cap: number): string {
  if (hop.timeout || hop.rtt <= 0) return "0%";
  return `${Math.max(6, Math.min(100, Math.round((hop.rtt / cap) * 100)))}%`;
}

function verdictTone(verdict?: string): "ok" | "warn" | "info" {
  if (!verdict) return "info";
  if (verdict.includes("双向对称")) return "ok";
  if (verdict.includes("不对称")) return "warn";
  return "info";
}

onMounted(loadMeta);
onBeforeUnmount(stopTicker);
</script>

<template>
  <div class="bi-stage">
    <!-- Control Strip -->
    <section class="bi-bar">
      <div class="bi-inputs">
        <div class="node-select-wrap">
          <el-select
            v-model="selectedNodeId"
            filterable
            placeholder="选择要追踪的节点"
            :disabled="running || loadingMeta"
            style="width: 100%"
          >
            <el-option
              v-for="n in nodes"
              :key="n.id"
              :value="n.id"
              :label="`${n.name} (${n.host})`"
            />
          </el-select>
        </div>

        <select
          v-if="onlineProbes.length > 0"
          v-model="selectedProbeId"
          class="probe-select"
          :disabled="running"
          aria-label="选择本机用户 Agent"
        >
          <option v-for="p in onlineProbes" :key="p.id" :value="p.id">
            {{ p.name }}{{ p.isp ? ` · ${p.isp}` : "" }}
          </option>
        </select>
        <span v-else-if="!loadingMeta" class="offline-hint">
          暂无在线用户 Agent（请先启动本机用户 Agent）
        </span>
      </div>

      <div class="bi-actions">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loadingMeta"
          :disabled="running"
          @click="loadMeta"
        >
          刷新
        </UiButton>
        <UiButton
          variant="primary"
          :icon="Position"
          :loading="running"
          :disabled="!canRun"
          @click="startTrace"
        >
          {{
            running
              ? `双向探测中 ${elapsedSec}s…`
              : result
                ? "重新双向追踪"
                : "发起双向追踪"
          }}
        </UiButton>
      </div>
    </section>

    <div v-if="errorMsg" class="inline-notice is-bad">{{ errorMsg }}</div>

    <!-- Running State -->
    <section v-if="running" class="running-card">
      <div class="pulse-ring" aria-hidden="true"><span></span></div>
      <div class="running-copy">
        <strong>正在并发采集「去程 + 回程」真实路由…</strong>
        <p class="muted">
          去程由本机用户 Agent 绕过 TUN 物理网卡直发；回程由目标节点对应的服务器 Agent 反向追踪至你的本机公网 IP（约 6–14 秒）。
        </p>
      </div>
      <span class="running-sec mono">{{ elapsedSec }}s</span>
    </section>

    <!-- Results Stage -->
    <template v-else-if="result">
      <!-- Top Symmetry Verdict Banner -->
      <section class="verdict-banner" :class="`tone-${verdictTone(result.verdict)}`">
        <div class="verdict-main">
          <span class="verdict-dot" aria-hidden="true"></span>
          <strong>{{ result.verdict || "双向路由采集完成" }}</strong>
        </div>
        <div class="verdict-meta mono">
          <span>节点：{{ result.nodeName }} ({{ result.nodeHost }})</span>
          <span v-if="result.clientIp">
            本机公网：{{ result.clientIp }}{{ result.probeIsp ? ` · ${result.probeIsp}` : "" }}
          </span>
          <span v-if="result.returnServer">回程探针：{{ result.returnServer }}</span>
        </div>
      </section>

      <!-- Dual Column Hop-by-Hop Comparison -->
      <section class="dual-grid">
        <!-- Outbound Card -->
        <article class="path-card">
          <header class="path-head">
            <div class="path-title">
              <span class="dir-pill">去程</span>
              <strong>本机 ({{ result.probeName }}) → 节点</strong>
            </div>
            <span
              class="line-badge"
              :class="`tone-${lineTone(result.outbound?.line)}`"
            >
              {{ result.outbound?.line || "未识别线路" }}
            </span>
          </header>

          <div v-if="result.outbound?.error" class="side-error">
            {{ result.outbound.error }}
          </div>

          <div v-else-if="result.outbound?.hops?.length" class="hop-list">
            <div
              v-for="h in result.outbound.hops"
              :key="`out-${h.ttl}`"
              class="hop-row"
              :class="{ 'is-timeout': h.timeout }"
            >
              <span class="hop-ttl mono">{{ String(h.ttl).padStart(2, "0") }}</span>
              <div class="hop-info">
                <div class="hop-top">
                  <span class="hop-ip mono">{{ h.timeout ? "*" : h.ip || "*" }}</span>
                  <span
                    v-if="h.lineTag"
                    class="hop-line-tag"
                    :class="`tone-${h.lineTone || 'muted'}`"
                  >
                    {{ h.lineTag }}
                  </span>
                  <span v-else-if="h.asn" class="hop-asn mono">{{ h.asn }}</span>
                </div>
                <span v-if="h.location || h.isp" class="hop-geo">
                  {{ [h.location, h.isp].filter(Boolean).join(" · ") }}
                </span>
              </div>
              <div class="hop-rtt-col">
                <span class="hop-rtt mono">
                  {{ h.timeout ? "超时" : `${h.rtt} ms` }}
                </span>
                <div class="rtt-track" aria-hidden="true">
                  <i :style="{ width: hopWidth(h, outMaxRtt) }"></i>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="side-empty">未返回去程跳点数据</div>
        </article>

        <!-- Return Path Card -->
        <article class="path-card">
          <header class="path-head">
            <div class="path-title">
              <span class="dir-pill is-ret">回程</span>
              <strong>
                {{ result.returnServer || "节点服务器" }} → 本机公网
              </strong>
            </div>
            <span
              class="line-badge"
              :class="`tone-${lineTone(result.returnPath?.line)}`"
            >
              {{ result.returnPath?.line || "待绑定服务器 Agent" }}
            </span>
          </header>

          <div v-if="result.returnPath?.error" class="side-error">
            {{ result.returnPath.error }}
          </div>

          <div v-else-if="result.returnPath?.hops?.length" class="hop-list">
            <div
              v-for="h in result.returnPath.hops"
              :key="`ret-${h.ttl}`"
              class="hop-row"
              :class="{ 'is-timeout': h.timeout }"
            >
              <span class="hop-ttl mono">{{ String(h.ttl).padStart(2, "0") }}</span>
              <div class="hop-info">
                <div class="hop-top">
                  <span class="hop-ip mono">{{ h.timeout ? "*" : h.ip || "*" }}</span>
                  <span
                    v-if="h.lineTag"
                    class="hop-line-tag"
                    :class="`tone-${h.lineTone || 'muted'}`"
                  >
                    {{ h.lineTag }}
                  </span>
                  <span v-else-if="h.asn" class="hop-asn mono">{{ h.asn }}</span>
                </div>
                <span v-if="h.location || h.isp" class="hop-geo">
                  {{ [h.location, h.isp].filter(Boolean).join(" · ") }}
                </span>
              </div>
              <div class="hop-rtt-col">
                <span class="hop-rtt mono">
                  {{ h.timeout ? "超时" : `${h.rtt} ms` }}
                </span>
                <div class="rtt-track" aria-hidden="true">
                  <i :style="{ width: hopWidth(h, retMaxRtt) }"></i>
                </div>
              </div>
            </div>
          </div>

          <div v-else class="side-empty">未返回回程跳点数据</div>
        </article>
      </section>
    </template>

    <!-- Ready State -->
    <section v-else class="ready-card">
      <div class="ready-icon mono">去程 ⇄ 回程</div>
      <strong>一眼看清节点是否真 CN2 GIA / 9929 / CMIN2</strong>
      <p class="muted">
        选择一个面板节点，系统将并发触发「本机用户 Agent 绕过 TUN 去程」与「服务器 Agent 回程追踪至你的公网 IP」，同屏对比逐跳路由与骨干网线路标签。
      </p>
    </section>
  </div>
</template>

<style scoped>
.bi-stage {
  display: grid;
  gap: 18px;
}

.bi-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.bi-inputs,
.bi-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.bi-inputs {
  flex: 1 1 360px;
}

.node-select-wrap {
  flex: 1 1 260px;
  max-width: 380px;
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

.inline-notice.is-bad {
  padding: 11px 14px;
  border-radius: 12px;
  font-size: 12.5px;
  background: color-mix(in srgb, var(--c-bad) 9%, var(--c-surface));
  border: 1px solid color-mix(in srgb, var(--c-bad) 35%, transparent);
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
  animation: biPulse 1.4s ease-out infinite;
}

@keyframes biPulse {
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
}

.verdict-banner {
  display: grid;
  gap: 8px;
  padding: 16px 20px;
  border-radius: 16px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
}

.verdict-banner.tone-ok {
  border-color: color-mix(in srgb, var(--c-ok) 38%, var(--c-line));
}

.verdict-banner.tone-warn {
  border-color: color-mix(in srgb, var(--c-warn) 42%, var(--c-line));
}

.verdict-main {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 15px;
  color: var(--c-ink);
}

.verdict-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
}

.verdict-banner.tone-warn .verdict-dot {
  background: var(--c-warn);
}

.verdict-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--c-muted);
}

.dual-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

@media (max-width: 960px) {
  .dual-grid {
    grid-template-columns: 1fr;
  }
}

.path-card {
  display: grid;
  gap: 12px;
  align-content: start;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  min-width: 0;
}

.path-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--c-line);
}

.path-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
  min-width: 0;
}

.dir-pill {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-ink);
  color: var(--c-surface);
  font-size: 11px;
  font-weight: 500;
  flex-shrink: 0;
}

.dir-pill.is-ret {
  background: color-mix(in srgb, var(--c-ok) 15%, transparent);
  color: var(--c-ok);
}

.line-badge {
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 600;
}

.line-badge.tone-ok,
.hop-line-tag.tone-ok {
  background: color-mix(in srgb, var(--c-ok) 14%, transparent);
  color: var(--c-ok);
}

.line-badge.tone-info,
.hop-line-tag.tone-info {
  background: color-mix(in srgb, #2563eb 13%, transparent);
  color: #2563eb;
}

.line-badge.tone-warn,
.hop-line-tag.tone-warn {
  background: color-mix(in srgb, var(--c-warn) 15%, transparent);
  color: var(--c-warn);
}

.line-badge.tone-muted,
.hop-line-tag.tone-muted {
  background: var(--c-surface-muted);
  color: var(--c-muted);
}

.hop-list {
  display: grid;
  gap: 4px;
}

.hop-row {
  display: grid;
  grid-template-columns: 26px minmax(0, 1fr) 86px;
  align-items: center;
  gap: 10px;
  padding: 7px 8px;
  border-radius: 10px;
  transition: background 140ms ease;
}

.hop-row:hover {
  background: var(--c-surface-muted);
}

.hop-row.is-timeout {
  opacity: 0.52;
}

.hop-ttl {
  font-size: 11.5px;
  color: var(--c-muted);
  text-align: right;
}

.hop-info {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.hop-top {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.hop-ip {
  font-size: 12.5px;
  color: var(--c-ink);
}

.hop-line-tag {
  padding: 1px 6px;
  border-radius: 5px;
  font-size: 10.5px;
  font-weight: 600;
}

.hop-asn {
  font-size: 10.5px;
  color: var(--c-muted);
}

.hop-geo {
  font-size: 11px;
  color: var(--c-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.hop-rtt-col {
  display: grid;
  gap: 4px;
  justify-items: end;
}

.hop-rtt {
  font-size: 11.5px;
  color: var(--c-ink);
  font-variant-numeric: tabular-nums;
}

.rtt-track {
  width: 68px;
  height: 3px;
  border-radius: 3px;
  background: var(--c-line);
  overflow: hidden;
}

.rtt-track i {
  display: block;
  height: 100%;
  background: var(--c-ok);
  border-radius: 3px;
}

.side-error {
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--c-surface-muted);
  color: var(--c-muted);
  font-size: 12.5px;
  line-height: 1.55;
}

.side-empty {
  padding: 28px 12px;
  text-align: center;
  font-size: 12.5px;
  color: var(--c-muted);
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
