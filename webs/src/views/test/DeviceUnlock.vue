<script setup lang="ts">
import { computed, onBeforeUnmount, onDeactivated, onMounted, ref } from "vue";
import { Close, CopyDocument, VideoPlay } from "@element-plus/icons-vue";
import { EmptyState, UiButton, UiDialog, UiSkeleton } from "@/ui";
import { getNodeOverview, getUnlockServices } from "@/api/subcription/node";
import {
  getProbes,
  runUserAgentUnlock,
  type Probe,
} from "@/api/subcription/probe";
import {
  unlockGroups,
  type UnlockCheckResult,
  type UnlockService,
} from "@/components/node-tests/unlock";
import { runBrowserUnlock } from "@/components/node-tests/browserUnlock";

defineOptions({ name: "DeviceUnlock" });

interface DeviceEgress {
  ip: string;
  loc: string;
  colo: string;
  rtt: number;
  isp?: string;
  asn?: string;
  hosting?: boolean;
  matchedNode?: string;
}

interface RichCheckResult extends UnlockCheckResult {
  latencyMs?: number;
  probeUrl?: string;
  probeKind?: string;
}

const services = ref<UnlockService[]>([]);
const panelNodes = ref<{ name: string; server?: string }[]>([]);
const onlineProbes = ref<Probe[]>([]);
const selectedProbeId = ref<number | undefined>(undefined);
const engine = ref<"agent" | "browser">("browser");
const group = ref("all");
const preparing = ref(true);
const loading = ref(false);
const loadError = ref("");
const egress = ref<DeviceEgress | null>(null);
const egressLoading = ref(false);
const results = ref<RichCheckResult[]>([]);
const selectedCard = ref<RichCheckResult | null>(null);
const detailOpen = ref(false);

let alive = true;
let version = 0;
let controller: AbortController | undefined;

const targets = computed(() =>
  services.value.filter((s) => group.value === "all" || s.group === group.value)
);

const progressPct = computed(() =>
  targets.value.length
    ? Math.round((results.value.length / targets.value.length) * 100)
    : 0
);

const stats = computed(() => ({
  unlocked: results.value.filter((r) =>
    ["available", "inferred", "originals"].includes(r.status || "")
  ).length,
  reachable: results.value.filter((r) => r.status === "reachable").length,
  blocked: results.value.filter((r) =>
    ["blocked", "region_blocked"].includes(r.status || "")
  ).length,
}));

function statusMeta(status?: string) {
  if (status === "available") return { label: "原生解锁", tone: "ok" };
  if (status === "originals") return { label: "仅自制剧", tone: "warn" };
  if (status === "inferred") return { label: "地区支持", tone: "ok" };
  if (status === "reachable") return { label: "可访问", tone: "info" };
  if (status === "region_blocked") return { label: "地区受限", tone: "warn" };
  if (status === "blocked") return { label: "明确拒绝", tone: "bad" };
  return { label: "受限 / 未返回", tone: "muted" };
}

function evidenceLabel(kind?: string) {
  if (kind === "native-http") return "用户 Agent 原生 HTTP（无 CORS）";
  if (kind === "openai" || kind === "dazn" || kind === "tvb") return "CORS API 直读";
  if (kind === "trace") return "边缘 Trace 定位";
  return "HTTP 可达探测";
}

async function detectEgress() {
  egressLoading.value = true;
  const started = performance.now();
  try {
    const res = await fetch("https://www.cloudflare.com/cdn-cgi/trace", {
      cache: "no-store",
      credentials: "omit",
    });
    const text = await res.text();
    const rtt = Math.round(performance.now() - started);
    const kv: Record<string, string> = {};
    for (const line of text.split(/\r?\n/)) {
      const idx = line.indexOf("=");
      if (idx > 0) kv[line.slice(0, idx).trim()] = line.slice(idx + 1).trim();
    }
    const ip = kv.ip || "";
    const matched = ip
      ? panelNodes.value.find(
          (n) => n.server && (n.server === ip || n.server.includes(ip))
        )?.name
      : undefined;
    if (alive) {
      egress.value = {
        ip: ip || "未知",
        loc: kv.loc || "—",
        colo: kv.colo || "—",
        rtt,
        matchedNode: matched,
      };
    }
  } catch {
    if (alive) egress.value = null;
  } finally {
    if (alive) egressLoading.value = false;
  }
}

async function prepare() {
  preparing.value = true;
  loadError.value = "";
  try {
    const [catalog, overview, probeRes] = await Promise.allSettled([
      getUnlockServices(),
      getNodeOverview(),
      getProbes(),
    ]);
    if (!alive) return;
    if (catalog.status === "fulfilled") {
      services.value = catalog.value.data || [];
    } else {
      loadError.value = "无法加载检测项目，请重试。";
    }
    if (overview.status === "fulfilled") {
      panelNodes.value = (overview.value.data || []).map((n: any) => ({
        name: n.name,
        server: n.server,
      }));
    }
    if (probeRes.status === "fulfilled") {
      onlineProbes.value = (probeRes.value.data || []).filter((p) => p.online);
      if (onlineProbes.value.length > 0) {
        engine.value = "agent";
        if (!selectedProbeId.value) {
          selectedProbeId.value = onlineProbes.value[0].id;
        }
      }
    }
    void detectEgress();
  } finally {
    if (alive) preparing.value = false;
  }
}

function stop() {
  version++;
  controller?.abort();
  controller = undefined;
  loading.value = false;
}

async function startTest() {
  if (loading.value) return;
  if (engine.value === "agent") {
    if (!onlineProbes.value.length) {
      ElMessage.warning("当前无在线用户 Agent，已切换浏览器探测");
      engine.value = "browser";
    } else {
      const id = ++version;
      loading.value = true;
      results.value = [];
      try {
        const { data } = await runUserAgentUnlock(selectedProbeId.value);
        if (!alive || id !== version) return;
        if (data?.egress?.ip) {
          egress.value = {
            ip: data.egress.ip,
            loc: data.egress.loc || data.egress.country || "—",
            colo: data.egress.colo || data.egress.city || "—",
            rtt: data.egress.rtt || 0,
            isp: data.egress.isp,
            asn: data.egress.asn,
            hosting: data.egress.hosting,
            matchedNode: data.egress.matchedNode,
          };
        }
        const rawItems = data?.items || [];
        const filtered =
          group.value === "all"
            ? rawItems
            : rawItems.filter((it) => it.group === group.value);
        results.value = filtered.map((it) => ({
          key: it.key,
          name: it.name,
          group: it.group,
          ok: it.ok,
          status: it.status as any,
          region: it.region,
          evidence: "api",
          note: it.note,
          checkedAt: it.checkedAt,
          latencyMs: it.latencyMs,
          probeKind: "native-http",
        }));
      } catch (err: any) {
        ElMessage.error(err?.message || "用户 Agent 原生解锁检测失败");
      } finally {
        if (id === version) loading.value = false;
      }
      return;
    }
  }

  if (!targets.value.length) return;
  const own = new AbortController();
  const id = ++version;
  controller = own;
  loading.value = true;
  results.value = [];

  void detectEgress();

  const selected = targets.value;
  const serviceMap = new Map(selected.map((s) => [s.key, s]));
  const startTimes = new Map<string, number>();
  const batchStart = performance.now();

  const receive = (row: UnlockCheckResult) => {
    if (!alive || id !== version || own.signal.aborted) return;
    const svc = serviceMap.get(row.key);
    const now = performance.now();
    const latency = Math.max(
      1,
      Math.round(now - (startTimes.get(row.key) ?? batchStart))
    );
    const enriched: RichCheckResult = {
      ...row,
      latencyMs: latency,
      probeUrl: svc?.url,
      probeKind: svc?.kind,
    };
    const next = new Map(results.value.map((r) => [r.key, r]));
    next.set(row.key, enriched);
    results.value = selected
      .map((s) => next.get(s.key))
      .filter((r): r is RichCheckResult => !!r);
  };

  try {
    await runBrowserUnlock(selected, own.signal, receive);
  } finally {
    if (id === version) {
      loading.value = false;
      controller = undefined;
    }
  }
}

function openCard(item: RichCheckResult) {
  selectedCard.value = item;
  detailOpen.value = true;
}

function copyCardDetail() {
  const c = selectedCard.value;
  if (!c) return;
  const lines = [
    `服务：${c.name}`,
    `结果：${statusMeta(c.status).label}${c.region ? ` (${c.region})` : ""}`,
    `耗时：${c.latencyMs ?? "-"} ms`,
    egress.value
      ? `出口：${egress.value.ip} · ${egress.value.loc} (${egress.value.colo})${egress.value.matchedNode ? ` · 节点 ${egress.value.matchedNode}` : ""}`
      : "",
    c.probeUrl ? `探测地址：${c.probeUrl}` : "",
    c.note ? `返回摘要：${c.note}` : "",
  ].filter(Boolean);
  navigator.clipboard
    .writeText(lines.join("\n"))
    .then(() => ElMessage.success("已复制检测详情"))
    .catch(() => ElMessage.error("复制失败"));
}

onMounted(prepare);
onDeactivated(stop);
onBeforeUnmount(() => {
  alive = false;
  stop();
});
</script>

<template>
  <div class="device-stage">
    <!-- Live Egress Identity + Controls Strip -->
    <section class="device-bar">
      <div class="egress-strip">
        <span class="egress-dot" :class="{ 'is-live': !!egress }" aria-hidden="true"></span>
        <template v-if="egress">
          <strong class="mono">{{ egress.ip }}</strong>
          <span class="egress-tag mono">{{ egress.loc }} · {{ egress.colo }}</span>
          <span v-if="egress.asn" class="egress-tag mono">{{ egress.asn }}</span>
          <span
            v-if="egress.hosting !== undefined"
            class="egress-tag"
          >
            {{ egress.hosting ? "机房 IP" : "非机房/原生倾向" }}
          </span>
          <span v-if="egress.matchedNode" class="egress-node">
            命中节点：{{ egress.matchedNode }}
          </span>
          <span class="egress-rtt mono">{{ egress.rtt }} ms</span>
        </template>
        <span v-else class="muted">
          {{ egressLoading ? "正在识别当前设备出口…" : "当前设备代理出口" }}
        </span>
      </div>

      <div class="bar-right">
        <div class="pill-group" role="group" aria-label="检测引擎">
          <button
            type="button"
            class="seg-pill"
            :class="{ 'is-active': engine === 'agent' }"
            :disabled="loading || !onlineProbes.length"
            :title="
              onlineProbes.length
                ? '使用本机用户 Agent 发起无 CORS 真实 HTTP 解锁检测'
                : '请先在本机启动用户 Agent v1.4'
            "
            @click="engine = 'agent'"
          >
            Agent 原生{{ onlineProbes.length ? ` (${onlineProbes.length})` : "" }}
          </button>
          <button
            type="button"
            class="seg-pill"
            :class="{ 'is-active': engine === 'browser' }"
            :disabled="loading"
            @click="engine = 'browser'"
          >
            浏览器探测
          </button>
        </div>

        <select
          v-if="engine === 'agent' && onlineProbes.length > 1"
          v-model="selectedProbeId"
          class="probe-select"
          :disabled="loading"
          aria-label="选择用户 Agent"
        >
          <option v-for="p in onlineProbes" :key="p.id" :value="p.id">
            {{ p.name }}
          </option>
        </select>

        <div class="pill-group" role="group" aria-label="服务分类">
          <button
            v-for="item in unlockGroups"
            :key="item.key"
            type="button"
            class="seg-pill"
            :class="{ 'is-active': group === item.key }"
            :disabled="loading"
            @click="group = item.key"
          >
            {{ item.label }}
          </button>
        </div>

        <UiButton
          v-if="loading"
          variant="danger"
          :icon="Close"
          @click="stop"
        >
          停止
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="loading"
          :disabled="preparing || !!loadError || !targets.length"
          @click="startTest"
        >
          {{ loading ? `探测中 ${progressPct}%` : results.length ? "重新检测" : "开始检测" }}
        </UiButton>
      </div>
    </section>

    <!-- Animated Sweep Progress Bar while Probing -->
    <div v-if="loading" class="sweep-track" aria-hidden="true">
      <span class="sweep-fill" :style="{ width: `${Math.max(8, progressPct)}%` }"></span>
    </div>

    <!-- Summary Strip Once Results Arrive -->
    <div v-if="results.length" class="summary-strip mono">
      <span class="s-ok">● 解锁 / 支持 {{ stats.unlocked }}</span>
      <span class="s-info">● 可访问 {{ stats.reachable }}</span>
      <span class="s-bad">● 受限 / 拒绝 {{ stats.blocked }}</span>
      <span class="s-hint">点击任意卡片查看出口与返回详情</span>
    </div>

    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载检测项目"
      :description="loadError"
    >
      <template #action>
        <UiButton @click="prepare">重新加载</UiButton>
      </template>
    </EmptyState>

    <UiSkeleton v-else-if="preparing" :lines="4" height="72px" />

    <!-- Animated Service Cards Grid -->
    <TransitionGroup
      v-else-if="results.length"
      name="card-pop"
      tag="div"
      class="service-grid"
    >
      <button
        v-for="item in results"
        :key="item.key"
        type="button"
        class="service-card"
        :class="`tone-${statusMeta(item.status).tone}`"
        @click="openCard(item)"
      >
        <div class="card-top">
          <strong class="card-name">{{ item.name }}</strong>
          <span v-if="item.region" class="card-region mono">{{ item.region }}</span>
        </div>

        <div class="card-bottom">
          <span class="status-badge" :class="`tone-${statusMeta(item.status).tone}`">
            <i aria-hidden="true"></i>
            {{ statusMeta(item.status).label }}
          </span>
          <span v-if="item.latencyMs" class="card-ms mono">
            {{ item.latencyMs }} ms
          </span>
        </div>
      </button>
    </TransitionGroup>

    <!-- Calm Ready State Before First Run -->
    <div v-else class="ready-blueprint">
      <div class="radar-ring" :class="{ 'is-scanning': loading }" aria-hidden="true">
        <span></span>
      </div>
      <p>
        {{
          engine === "agent"
            ? `点击「开始检测」，由本机用户 Agent 绕过浏览器 CORS，对当前代理出口逐项发起原生 HTTP 流媒体与 AI 解锁检测`
            : `点击「开始检测」，以当前浏览器代理逐项探测 ${targets.length} 项流媒体与 AI 服务`
        }}
      </p>
    </div>

    <!-- Rich Click-to-Inspect Service Detail Card -->
    <UiDialog
      v-model="detailOpen"
      :title="selectedCard ? selectedCard.name : '检测详情'"
      width="520px"
    >
      <div v-if="selectedCard" class="card-detail">
        <div class="detail-badges">
          <span
            class="status-badge"
            :class="`tone-${statusMeta(selectedCard.status).tone}`"
          >
            <i aria-hidden="true"></i>
            {{ statusMeta(selectedCard.status).label }}
          </span>
          <span v-if="selectedCard.region" class="chip mono">
            地区 {{ selectedCard.region }}
          </span>
          <span v-if="selectedCard.latencyMs" class="chip mono">
            {{ selectedCard.latencyMs }} ms
          </span>
          <button type="button" class="copy-btn" @click="copyCardDetail">
            <el-icon><CopyDocument /></el-icon>
            <span>复制</span>
          </button>
        </div>

        <dl class="detail-grid">
          <div>
            <dt>当前出口 IP / 节点</dt>
            <dd class="mono">
              {{
                egress
                  ? `${egress.ip} (${egress.loc} · ${egress.colo})${egress.matchedNode ? ` · ${egress.matchedNode}` : ""}`
                  : "当前浏览器代理出口"
              }}
            </dd>
          </div>
          <div>
            <dt>探测方式</dt>
            <dd>{{ evidenceLabel(selectedCard.probeKind) }}</dd>
          </div>
          <div class="is-wide">
            <dt>探测目标地址</dt>
            <dd class="mono">{{ selectedCard.probeUrl || "—" }}</dd>
          </div>
          <div class="is-wide">
            <dt>返回内容与判定依据</dt>
            <dd>{{ selectedCard.note || "已收到响应" }}</dd>
          </div>
        </dl>
      </div>
    </UiDialog>
  </div>
</template>

<style scoped>
.device-stage {
  display: grid;
  gap: 18px;
}

.device-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}

.egress-strip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 8px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-ink);
  flex-wrap: wrap;
}

.egress-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
}

.egress-dot.is-live {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px rgb(21 184 90 / 0.16);
}

.egress-tag {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 11px;
  color: var(--c-muted);
}

.egress-node {
  font-size: 12px;
  color: var(--c-ok);
  font-weight: 500;
}

.egress-rtt {
  font-size: 11.5px;
  color: var(--c-muted);
}

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
  font-size: 12px;
  cursor: pointer;
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.seg-pill:disabled {
  opacity: 0.42;
  cursor: not-allowed;
}

.probe-select {
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  outline: none;
  cursor: pointer;
}

.probe-select:focus-visible {
  border-color: var(--c-ink);
}

.sweep-track {
  height: 3px;
  border-radius: 3px;
  background: var(--c-line);
  overflow: hidden;
}

.sweep-fill {
  display: block;
  height: 100%;
  background: linear-gradient(90deg, var(--c-ink), var(--c-ok));
  transition: width 300ms var(--ease-out);
}

.summary-strip {
  display: flex;
  align-items: center;
  gap: 18px;
  flex-wrap: wrap;
  font-size: 12px;
}

.s-ok {
  color: var(--c-ok);
}
.s-info {
  color: var(--c-ink);
}
.s-bad {
  color: var(--c-bad);
}
.s-hint {
  margin-left: auto;
  color: var(--c-muted);
}

/* Service Cards Grid */
.service-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.service-card {
  display: grid;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition:
    transform 220ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 220ms var(--ease-out),
    box-shadow 220ms var(--ease-out);
}

.service-card:hover {
  transform: translateY(-2px);
  border-color: var(--c-ink);
  box-shadow: 0 12px 28px -14px rgb(0 0 0 / 0.14);
}

.card-top,
.card-bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.card-name {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.card-region {
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--c-surface-muted);
  font-size: 11px;
  color: var(--c-ink);
}

.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.status-badge.tone-ok {
  color: var(--c-ok);
}
.status-badge.tone-info {
  color: var(--c-ink);
}
.status-badge.tone-warn {
  color: var(--c-warn);
}
.status-badge.tone-bad {
  color: var(--c-bad);
}
.status-badge.tone-muted {
  color: var(--c-muted);
}

.card-ms {
  font-size: 11.5px;
  color: var(--c-muted);
}

/* Calm Radar Blueprint Ready State */
.ready-blueprint {
  display: grid;
  justify-items: center;
  gap: 16px;
  padding: 64px 20px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 13.5px;
  text-align: center;
}

.radar-ring {
  position: relative;
  width: 56px;
  height: 56px;
  border-radius: 50%;
  border: 1px solid var(--c-line);
  display: grid;
  place-items: center;
}

.radar-ring span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--c-ok);
  box-shadow: 0 0 0 6px rgb(21 184 90 / 0.14);
}

/* Detail Dialog */
.card-detail {
  display: grid;
  gap: 18px;
}

.detail-badges {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.chip {
  padding: 3px 9px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--c-ink);
}

.copy-btn {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.copy-btn:hover {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 16px;
  margin: 0;
  padding-top: 14px;
  border-top: 1px solid var(--c-line);
}

.detail-grid .is-wide {
  grid-column: 1 / -1;
}

.detail-grid dt {
  font-size: 11px;
  color: var(--c-muted);
  margin-bottom: 4px;
}

.detail-grid dd {
  margin: 0;
  font-size: 13px;
  color: var(--c-ink);
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.muted {
  color: var(--c-muted);
}

.card-pop-enter-active {
  transition:
    opacity 280ms var(--ease-out),
    transform 320ms cubic-bezier(0.16, 1, 0.3, 1);
}

.card-pop-enter-from {
  opacity: 0;
  transform: translateY(10px) scale(0.98);
}

@media (max-width: 640px) {
  .detail-grid {
    grid-template-columns: 1fr;
  }
}
</style>
