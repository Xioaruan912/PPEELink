<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { Close, VideoPlay } from "@element-plus/icons-vue";
import { EmptyState, UiButton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import { translateFriendlyError } from "@/utils/friendlyError";
import { getNodeOverview } from "@/api/subcription/node";
import { getTcp, saveTcp } from "@/utils/nodeTestStorage";
import {
  normalizeTarget,
  type ChinaTarget,
} from "@/components/node-tests/reach";
import ChinaTcpMap from "@/components/node-tests/ChinaTcpMap.vue";
import { useTestStream } from "./useTestStream";

defineOptions({ name: "TcpTest" });

type NodeItem = { id: number; name: string; valid: boolean };
interface ZStaticItem {
  port: number;
  ok: boolean;
  rtt?: number;
}

const nodes = ref<NodeItem[]>([]);
const nodesLoading = ref(true);
const selectedNodeId = ref<number>();
const targets = ref<ChinaTarget[]>([]);
const zstaticResults = ref<ZStaticItem[]>([]);
const selectedIsp = ref<string>("all");
const zstaticPort = ref("443");
const ranking = ref<"province" | "isp">("province");
const error = ref("");
const status = ref("等待开始");
const cachedAt = ref(0);

const { loading, run, cancel } = useTestStream();
let disposed = false;
let runVersion = 0;

const ispOptions = [
  { label: "全部运营商", value: "all" },
  { label: "电信", value: "电信" },
  { label: "联通", value: "联通" },
  { label: "移动", value: "移动" },
];

const filteredTargets = computed(() =>
  targets.value.filter(
    (item) => selectedIsp.value === "all" || item.isp === selectedIsp.value
  )
);

const reachable = computed(
  () => filteredTargets.value.filter((item) => item.ok).length
);

const validRtts = computed(() =>
  filteredTargets.value
    .filter((t) => t.ok && typeof t.rtt === "number" && t.rtt >= 0)
    .map((t) => t.rtt as number)
);

const latencyStats = computed(() => {
  const arr = validRtts.value;
  if (!arr.length) return { min: null, avg: null, max: null };
  const min = Math.min(...arr);
  const max = Math.max(...arr);
  const avg = Math.round(arr.reduce((a, b) => a + b, 0) / arr.length);
  return { min, avg, max };
});

function rttTone(rtt?: number, ok?: boolean) {
  if (!ok || (typeof rtt === "number" && rtt < 0)) return "bad";
  if (typeof rtt !== "number") return "ok";
  if (rtt <= 80) return "ok";
  if (rtt <= 160) return "warn";
  return "slow";
}

function formatRtt(rtt?: number, ok?: boolean) {
  if (!ok || (typeof rtt === "number" && rtt < 0)) return "超时";
  if (typeof rtt === "number") return `${rtt} ms`;
  return "可达";
}

function rttBarWidth(rtt?: number | null) {
  if (rtt === null || rtt === undefined || rtt < 0) return 100;
  return Math.min(100, Math.max(8, Math.round((rtt / 260) * 100)));
}

function aggregate(key: "province" | "isp") {
  const groups = new Map<string, ChinaTarget[]>();
  filteredTargets.value.forEach((item) => {
    const items = groups.get(item[key]) || [];
    items.push(item);
    groups.set(item[key], items);
  });
  return [...groups].map(([name, items]) => {
    const okItems = items.filter((i) => i.ok);
    const rtts = okItems
      .filter((i) => typeof i.rtt === "number" && i.rtt >= 0)
      .map((i) => i.rtt as number);
    const avgRtt = rtts.length
      ? Math.round(rtts.reduce((a, b) => a + b, 0) / rtts.length)
      : null;
    return {
      name,
      province: name,
      items,
      okCount: okItems.length,
      avgRtt,
    };
  });
}

const provinceGroups = computed(() => aggregate("province"));
const rankGroups = computed(() =>
  (ranking.value === "province" ? provinceGroups.value : aggregate("isp"))
    .slice()
    .sort((a, b) => {
      if (a.avgRtt !== null && b.avgRtt !== null) return a.avgRtt - b.avgRtt;
      if (a.avgRtt !== null) return -1;
      if (b.avgRtt !== null) return 1;
      return a.name.localeCompare(b.name, "zh");
    })
);

function persist(id: number) {
  if (!targets.value.length && !zstaticResults.value.length) return;
  cachedAt.value = Date.now();
  saveTcp(id, {
    targets: targets.value,
    zstatic: zstaticResults.value,
    zstaticPort: zstaticPort.value,
    provinces: [],
    isps: selectedIsp.value === "all" ? [] : [selectedIsp.value],
    updatedAt: cachedAt.value,
    status: status.value,
  });
}

function restoreCache() {
  const cached = selectedNodeId.value ? getTcp(selectedNodeId.value) : null;
  targets.value = (cached?.targets || []).map(normalizeTarget);
  zstaticResults.value = (cached?.zstatic || []).map((z: any) => {
    const rawRtt = typeof z.rtt === "number" ? z.rtt : Number(z.rtt);
    const hasRtt = Number.isFinite(rawRtt);
    return {
      port: z.port,
      rtt: hasRtt ? rawRtt : undefined,
      ok: typeof z.ok === "boolean" ? z.ok : hasRtt && rawRtt >= 0,
    };
  });
  cachedAt.value = cached?.updatedAt || 0;
  status.value = cached ? cached.status || "已恢复缓存" : "等待开始";
  error.value = "";
  if (cached?.zstaticPort) zstaticPort.value = cached.zstaticPort;
}

async function loadNodes() {
  nodesLoading.value = true;
  error.value = "";
  try {
    const { data } = await getNodeOverview();
    if (disposed) return;
    nodes.value = (data || [])
      .filter((node: NodeItem) => node.valid)
      .sort((a: NodeItem, b: NodeItem) =>
        a.name.localeCompare(b.name, "zh-CN")
      );
    selectedNodeId.value ||= nodes.value[0]?.id;
    restoreCache();
  } catch {
    if (!disposed) error.value = "节点列表读取失败";
  } finally {
    if (!disposed) nodesLoading.value = false;
  }
}

async function startTest() {
  if (loading.value) return;
  const id = selectedNodeId.value;
  if (!id) {
    ElMessage.warning("请先选择节点");
    return;
  }
  targets.value = [];
  zstaticResults.value = [];
  error.value = "";
  status.value = "TCP Ping 测速中";
  cachedAt.value = 0;

  const payload = new URLSearchParams({
    id: String(id),
    zstatic_port: zstaticPort.value,
  });
  if (selectedIsp.value !== "all") payload.append("isps", selectedIsp.value);

  const ownVersion = ++runVersion;
  let completed = false;
  try {
    await run("/api/v1/nodes/chinaping/stream", payload, (type, data) => {
      if (type === "province" && Array.isArray(data.targets)) {
        for (const item of data.targets) {
          const index = targets.value.findIndex(
            (old) => old.ip === item.ip && old.port === item.port
          );
          const target = normalizeTarget(item);
          if (index >= 0) targets.value[index] = target;
          else targets.value.push(target);
        }
        persist(id);
      }
      if (type === "zstatic") {
        zstaticResults.value = (data.zstatic || []).map((z: any) => ({
          port: z.port,
          rtt: typeof z.rtt === "number" ? z.rtt : undefined,
          ok: typeof z.ok === "boolean" ? z.ok : Number(z.rtt) >= 0,
        }));
        persist(id);
      }
      if (type === "done") {
        completed = true;
        status.value =
          data.perspective === "agent"
            ? "节点本机直测完成（Agent）"
            : "测速完成（经面板中转）";
        persist(id);
        ElMessage.success(
          data.perspective === "agent"
            ? "TCP Ping 测速完成（节点本机直测）"
            : "TCP Ping 测速完成"
        );
      }
    });
    if (
      !disposed &&
      ownVersion === runVersion &&
      !completed &&
      status.value !== "已停止"
    ) {
      status.value = "连接中断";
      error.value = "测速流提前结束，请重试";
    }
  } catch (cause: any) {
    if (disposed || ownVersion !== runVersion) return;
    error.value = cause.message || "测速失败";
    status.value = "测速失败";
    ElMessage.warning(translateFriendlyError(error.value).title);
  } finally {
    if (!disposed && ownVersion === runVersion) persist(id);
  }
}

function abortTest() {
  runVersion++;
  cancel();
  status.value = "已停止";
  if (selectedNodeId.value) persist(selectedNodeId.value);
}

onMounted(loadNodes);
onBeforeUnmount(() => {
  if (status.value === "TCP Ping 测速中" && selectedNodeId.value) {
    status.value = "已停止";
    persist(selectedNodeId.value);
  }
  disposed = true;
});
</script>

<template>
  <div class="tcp-itdog">
    <!-- Single-Row Control Bar -->
    <section class="tcp-bar">
      <div class="node-select-wrap">
        <el-select
          v-model="selectedNodeId"
          filterable
          :loading="nodesLoading"
          :disabled="loading || nodesLoading"
          placeholder="选择待测节点"
          style="width: 100%"
          @change="restoreCache"
        >
          <el-option
            v-for="node in nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
          />
        </el-select>
      </div>

      <div class="pill-group" role="group" aria-label="运营商筛选">
        <button
          v-for="opt in ispOptions"
          :key="opt.value"
          type="button"
          class="seg-pill"
          :class="{ 'is-active': selectedIsp === opt.value }"
          :disabled="loading"
          @click="selectedIsp = opt.value"
        >
          {{ opt.label }}
        </button>
      </div>

      <div class="pill-group" role="group" aria-label="zstaticcdn 端口">
        <button
          type="button"
          class="seg-pill mono"
          :class="{ 'is-active': zstaticPort === '443' }"
          :disabled="loading"
          @click="zstaticPort = '443'"
        >
          :443
        </button>
        <button
          type="button"
          class="seg-pill mono"
          :class="{ 'is-active': zstaticPort === '80' }"
          :disabled="loading"
          @click="zstaticPort = '80'"
        >
          :80
        </button>
      </div>

      <div class="bar-actions">
        <UiButton
          v-if="loading"
          variant="danger"
          :icon="Close"
          @click="abortTest"
        >
          停止
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="loading"
          :disabled="!selectedNodeId || nodesLoading"
          @click="startTest"
        >
          {{ loading ? "TCP Ping 中…" : "开始测速" }}
        </UiButton>
      </div>
    </section>

    <KineticNotice :raw="error" />

    <!-- itdog-Style Live Telemetry KPI Strip -->
    <dl v-if="filteredTargets.length || zstaticResults.length" class="ping-kpis">
      <div class="kpi-cell">
        <dt>最快延迟</dt>
        <dd class="mono tone-ok">
          {{ latencyStats.min !== null ? `${latencyStats.min} ms` : "—" }}
        </dd>
      </div>
      <div class="kpi-cell">
        <dt>平均延迟</dt>
        <dd class="mono">
          {{ latencyStats.avg !== null ? `${latencyStats.avg} ms` : "—" }}
        </dd>
      </div>
      <div class="kpi-cell">
        <dt>最慢延迟</dt>
        <dd class="mono tone-warn">
          {{ latencyStats.max !== null ? `${latencyStats.max} ms` : "—" }}
        </dd>
      </div>
      <div class="kpi-cell">
        <dt>三网可达</dt>
        <dd class="mono">
          {{ reachable }} / {{ filteredTargets.length }}
        </dd>
      </div>
      <div class="kpi-cell">
        <dt>zstaticcdn TCP</dt>
        <dd class="mono">
          <template v-if="zstaticResults.length">
            <span
              v-for="z in zstaticResults"
              :key="z.port"
              :class="`tone-${rttTone(z.rtt, z.ok)}`"
            >
              :{{ z.port }} · {{ formatRtt(z.rtt, z.ok) }}
            </span>
          </template>
          <span v-else>—</span>
        </dd>
      </div>
    </dl>

    <!-- Map + itdog-Style Real-Time Latency Board -->
    <div class="tcp-stage">
      <section class="map-col">
        <div class="tcp-map-wrap" :class="{ 'is-running': loading }">
          <ChinaTcpMap :groups="provinceGroups" />
        </div>
        <div class="legend-strip mono">
          <span><i class="dot-ok"></i>≤ 80ms 极快</span>
          <span><i class="dot-warn"></i>80–160ms 良好</span>
          <span><i class="dot-slow"></i>&gt; 160ms 偏高</span>
          <span><i class="dot-bad"></i>超时 / 不可达</span>
        </div>
      </section>

      <aside class="board-col">
        <div class="board-head">
          <strong class="board-title">TCP Ping 延迟排行</strong>
          <div class="pill-group">
            <button
              type="button"
              class="seg-pill"
              :class="{ 'is-active': ranking === 'province' }"
              @click="ranking = 'province'"
            >
              按省份
            </button>
            <button
              type="button"
              class="seg-pill"
              :class="{ 'is-active': ranking === 'isp' }"
              @click="ranking = 'isp'"
            >
              按运营商
            </button>
          </div>
        </div>

        <div v-if="rankGroups.length" class="ping-list">
          <article
            v-for="group in rankGroups"
            :key="group.name"
            class="ping-row"
          >
            <div class="ping-row-head">
              <strong>{{ group.name }}</strong>
              <span
                class="ping-avg mono"
                :class="`tone-${rttTone(group.avgRtt ?? -1, group.okCount > 0)}`"
              >
                {{
                  group.avgRtt !== null
                    ? `${group.avgRtt} ms`
                    : group.okCount > 0
                      ? "可达"
                      : "超时"
                }}
              </span>
            </div>

            <div class="ping-bar-track" aria-hidden="true">
              <i
                :class="`bar-${rttTone(group.avgRtt ?? -1, group.okCount > 0)}`"
                :style="{ width: `${rttBarWidth(group.avgRtt)}%` }"
              ></i>
            </div>

            <div class="city-chips">
              <span
                v-for="item in group.items"
                :key="`${item.ip}:${item.port}`"
                class="city-chip mono"
                :class="`tone-${rttTone(item.rtt, item.ok)}`"
              >
                {{ ranking === "province" ? `${item.city} ${item.isp}` : `${item.province} ${item.city}` }}
                <b>{{ formatRtt(item.rtt, item.ok) }}</b>
              </span>
            </div>
          </article>
        </div>

        <EmptyState
          v-else
          size="section"
          :title="loading ? '正在逐省返回 TCP Ping 延迟…' : '暂无测速数据'"
        >
          <template #action>
            <UiButton
              v-if="!loading"
              variant="primary"
              :disabled="!selectedNodeId"
              @click="startTest"
            >
              开始测速
            </UiButton>
          </template>
        </EmptyState>
      </aside>
    </div>
  </div>
</template>

<style scoped>
.tcp-itdog {
  display: grid;
  gap: 18px;
}

.tcp-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.node-select-wrap {
  width: min(280px, 100%);
  flex: 1 1 220px;
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

.bar-actions {
  margin-left: auto;
  display: flex;
  gap: 8px;
}

.inline-error {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-bad);
}

/* itdog KPI Strip */
.ping-kpis {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  margin: 0;
  padding: 14px 0;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
}

.kpi-cell {
  display: grid;
  gap: 4px;
  padding: 0 16px;
  border-left: 1px solid var(--c-line);
}

.kpi-cell:first-child {
  padding-left: 0;
  border-left: 0;
}

.kpi-cell dt {
  font-size: 11.5px;
  color: var(--c-muted);
}

.kpi-cell dd {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--c-ink);
  font-variant-numeric: tabular-nums;
}

.tone-ok {
  color: var(--c-ok);
}
.tone-warn {
  color: var(--c-warn);
}
.tone-slow {
  color: #ea580c;
}
.tone-bad {
  color: var(--c-bad);
}

/* Main Stage */
.tcp-stage {
  display: grid;
  grid-template-columns: minmax(0, 6.5fr) minmax(0, 5.5fr);
  gap: 24px;
  align-items: start;
}

.map-col {
  display: grid;
  gap: 12px;
}

.tcp-map-wrap {
  position: relative;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
}

.tcp-map-wrap :deep(.china-map) {
  height: 480px;
}

.tcp-map-wrap.is-running::after {
  content: "";
  position: absolute;
  inset: 0;
  width: 26%;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    transparent,
    rgb(21 184 90 / 0.14),
    transparent
  );
  animation: tcpScan 2s linear infinite;
}

.legend-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.legend-strip span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.legend-strip i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.dot-ok {
  background: var(--c-ok);
}
.dot-warn {
  background: var(--c-warn);
}
.dot-slow {
  background: #ea580c;
}
.dot-bad {
  background: var(--c-bad);
}

/* itdog-style Latency Board */
.board-col {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.board-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.board-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.ping-list {
  display: grid;
  max-height: 520px;
  overflow-y: auto;
  border-top: 1px solid var(--c-line);
  padding-right: 4px;
}

.ping-row {
  display: grid;
  gap: 8px;
  padding: 13px 0;
  border-bottom: 1px solid var(--c-line);
}

.ping-row-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.ping-row-head strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.ping-avg {
  font-size: 13.5px;
  font-weight: 600;
}

.ping-bar-track {
  height: 3px;
  border-radius: 3px;
  background: var(--c-line);
  overflow: hidden;
}

.ping-bar-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  transition: width 360ms var(--ease-out);
}

.bar-ok {
  background: var(--c-ok);
}
.bar-warn {
  background: var(--c-warn);
}
.bar-slow {
  background: #ea580c;
}
.bar-bad {
  background: var(--c-bad);
  opacity: 0.4;
}

.city-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.city-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  background: var(--c-surface);
  font-size: 11px;
  color: var(--c-muted);
}

.city-chip b {
  font-weight: 600;
  color: inherit;
}

.city-chip.tone-ok b {
  color: var(--c-ok);
}
.city-chip.tone-warn b {
  color: var(--c-warn);
}
.city-chip.tone-slow b {
  color: #ea580c;
}
.city-chip.tone-bad b {
  color: var(--c-bad);
}

@keyframes tcpScan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(420%);
  }
}

@media (max-width: 960px) {
  .ping-kpis {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }
  .kpi-cell {
    padding: 0;
    border-left: 0;
  }
  .tcp-stage {
    grid-template-columns: 1fr;
  }
  .tcp-map-wrap :deep(.china-map) {
    height: 340px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .tcp-map-wrap.is-running::after {
    display: none;
  }
}
</style>
