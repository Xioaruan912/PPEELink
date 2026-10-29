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
import { useRouter } from "vue-router";
import { Refresh, Position } from "@element-plus/icons-vue";
import { PageHeader, UiButton, UiPanel, EmptyState, UiSkeleton } from "@/ui";
import {
  getThroughputSources,
  prepareThroughput,
  getThroughputHistory,
  getThroughputDetail,
  createThroughput,
  cancelThroughput,
  type ThroughputSettings,
  type ThroughputSource,
  type ThroughputRun,
  type ThroughputDetail,
  type ThroughputJob,
  type DirectionResult,
} from "@/api/network/throughput";
import {
  estimatedTraffic,
  curvePoints,
  formatRate,
  formatBytes,
} from "@/utils/network-throughput";
import { formatDateTime } from "@/utils/date";
const router = useRouter();
const sources = ref<ThroughputSource[]>([]),
  history = ref<ThroughputRun[]>([]),
  detail = ref<ThroughputDetail | null>(null);
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
const loading = ref(true),
  starting = ref(false),
  cancelling = ref(false),
  preparing = ref(false),
  detailLoading = ref(false);
const error = ref(""),
  sourceError = ref(""),
  historyError = ref("");
const sort = ref("order"),
  directionFilter = ref("forward");
let visible = true,
  timer: ReturnType<typeof setTimeout> | undefined,
  sequence = 0,
  selectedId = 0,
  sourceSequence = 0;
const candidates = computed(() =>
  sources.value.filter((s) => s.id !== settings.landingId)
);
const selected = computed(() =>
  sources.value.filter(
    (s) => settings.sourceIds.includes(s.id) || s.id === settings.landingId
  )
);
const notReady = computed(() =>
  selected.value.filter((s) => !s.available || s.prepareRequested)
);
const isRunning = computed(() => detail.value?.run.status === "running");
const estimated = computed(() =>
  estimatedTraffic(
    settings.rateMbps,
    settings.duration,
    settings.sourceIds.length,
    settings.direction
  )
);
const canStart = computed(
  () =>
    !loading.value &&
    !starting.value &&
    !isRunning.value &&
    settings.landingId > 0 &&
    settings.sourceIds.length > 0 &&
    settings.sourceIds.length <= 8 &&
    !notReady.value.length &&
    selected.value.length === settings.sourceIds.length + 1 &&
    settings.duration >= 5 &&
    settings.duration <= 30 &&
    settings.streams >= 1 &&
    settings.streams <= 8 &&
    settings.rateMbps >= 1 &&
    settings.rateMbps <= 10000 &&
    settings.port >= 1024 &&
    settings.port <= 65535 &&
    (settings.direction !== "bidir" ||
      selected.value.every((s) => s.bidirSupported))
);
const statuses: Record<ThroughputJob["status"], string> = {
  queued: "等待前一项",
  starting: "准备落地监听",
  running: "正在互传",
  completed: "已完成",
  failed: "失败",
  cancelled: "已取消",
  timeout: "超时",
};
const jobs = computed(() => {
  let list = [...(detail.value?.jobs || [])];
  if (sort.value === "rate")
    list = list
      .filter(
        (j) => j.direction === directionFilter.value || j.direction === "bidir"
      )
      .sort((a, b) => rateOf(b) - rateOf(a));
  return list;
});
const completed = computed(
  () => detail.value?.jobs.filter((j) => j.status === "completed").length || 0
);
const actualBytes = computed(
  () =>
    detail.value?.jobs.reduce(
      (sum, j) =>
        sum + (j.result?.forward?.bytes || 0) + (j.result?.reverse?.bytes || 0),
      0
    ) || 0
);
function rateOf(j: ThroughputJob) {
  return j.status === "completed"
    ? (directionFilter.value === "reverse"
        ? j.result?.reverse?.bitsPerSecond
        : j.result?.forward?.bitsPerSecond) || 0
    : -1;
}
function directions(
  j: ThroughputJob
): { key: string; label: string; value: DirectionResult }[] {
  const r = j.result;
  if (!r) return [];
  return [
    ...(r.forward
      ? [{ key: "forward", label: "中转 → 落地", value: r.forward }]
      : []),
    ...(r.reverse
      ? [{ key: "reverse", label: "落地 → 中转", value: r.reverse }]
      : []),
  ];
}
function clearTimer() {
  if (timer) clearTimeout(timer);
  timer = undefined;
}
function schedule() {
  clearTimer();
  if (
    visible &&
    (isRunning.value || sources.value.some((s) => s.prepareRequested))
  )
    timer = setTimeout(() => {
      if (isRunning.value) void loadDetail();
      if (sources.value.some((s) => s.prepareRequested))
        void loadSources(false);
    }, 2000);
}
async function loadSources(show = true) {
  const requestId = ++sourceSequence;
  if (show) loading.value = true;
  sourceError.value = "";
  try {
    const res = await getThroughputSources();
    if (requestId !== sourceSequence) return;
    sources.value = res.data || [];
  } catch {
    if (requestId === sourceSequence)
      sourceError.value = "无法读取服务器，请检查连接后重试。";
  } finally {
    if (requestId === sourceSequence) {
      loading.value = false;
      schedule();
    }
  }
}
async function loadHistory() {
  historyError.value = "";
  try {
    history.value = (await getThroughputHistory()).data || [];
  } catch {
    historyError.value = "历史记录读取失败。";
  }
}
async function loadDetail() {
  if (!selectedId) return;
  const id = selectedId,
    requestId = ++sequence;
  try {
    const res = await getThroughputDetail(id);
    if (requestId !== sequence || id !== selectedId) return;
    const wasRunning = isRunning.value;
    detail.value = res.data;
    error.value = "";
    if (wasRunning && !isRunning.value) void loadHistory();
  } catch {
    if (requestId === sequence)
      error.value = "结果读取失败，已收到的结果仍保留。";
  } finally {
    if (requestId === sequence) {
      detailLoading.value = false;
      schedule();
    }
  }
}
async function chooseRun(id: number) {
  clearTimer();
  sequence++;
  selectedId = id;
  detail.value = null;
  detailLoading.value = true;
  await loadDetail();
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
    history.value = [res.data, ...history.value].slice(0, 30);
    sort.value = "order";
    await chooseRun(res.data.id);
  } catch {
    error.value =
      "无法开始测速，请检查服务器状态、参数或是否正在参与其他测试。";
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
  } catch {
    error.value = "取消失败，请重试。";
  } finally {
    cancelling.value = false;
  }
}
async function prepare() {
  const ids = notReady.value
    .filter((s) => !s.prepareRequested)
    .map((s) => s.id);
  if (!ids.length) return;
  preparing.value = true;
  error.value = "";
  try {
    await prepareThroughput(ids);
    await loadSources(false);
  } catch {
    error.value = "准备请求失败，请检查 Agent 在线状态。";
  } finally {
    preparing.value = false;
  }
}
function viewPaths() {
  if (!detail.value) return;
  void router.push({
    path: "/network/traceroute",
    query: {
      target: detail.value.run.target,
      agentIds: [...new Set(detail.value.jobs.map((j) => j.sourceId))].join(
        ","
      ),
    },
  });
}
watch(
  () => settings.landingId,
  (id) => {
    settings.sourceIds = settings.sourceIds.filter((s) => s !== id);
  }
);
onMounted(() => {
  void loadSources();
  void loadHistory();
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
});
</script>
<template>
  <div class="throughput-page">
    <PageHeader
      title="节点互测"
      description="选一个落地，比较不同中转 VPS 的实际传输能力。数据在两台服务器之间直接传输。"
    >
      <template #actions
        ><UiButton
          :icon="Refresh"
          :loading="loading"
          @click="
            loadSources();
            loadHistory();
          "
          >刷新服务器</UiButton
        ></template
      >
    </PageHeader>
    <div class="lab-layout">
      <aside class="lab-side">
        <UiPanel
          title="选择线路"
          description="两端均需接入 Agent。候选中转会依次测试。"
        >
          <form class="lab-form" @submit.prevent="start">
            <label for="landing">落地服务器</label>
            <select id="landing" v-model.number="settings.landingId">
              <option :value="0" disabled>选择落地</option>
              <option v-for="s in sources" :key="s.id" :value="s.id">
                {{ s.name }} · {{ s.publicHost }}
              </option>
            </select>
            <div class="source-title">
              <strong>候选中转</strong
              ><span>{{ settings.sourceIds.length }} / 8</span>
            </div>
            <UiSkeleton v-if="loading" :lines="3" height="26px" />
            <p v-else-if="sourceError" class="error" role="alert">
              {{ sourceError }}
            </p>
            <div v-else-if="candidates.length" class="candidate-list">
              <label
                v-for="s in candidates"
                :key="s.id"
                class="candidate"
                :class="{ 'is-selected': settings.sourceIds.includes(s.id) }"
                ><input
                  v-model="settings.sourceIds"
                  type="checkbox"
                  :value="s.id"
                  :disabled="
                    settings.sourceIds.length >= 8 &&
                    !settings.sourceIds.includes(s.id)
                  " /><span
                  ><strong>{{ s.name }}</strong
                  ><small>{{ s.publicHost }}</small></span
                ><span
                  class="ready-dot"
                  :class="{ ready: s.available }"
                  :title="s.available ? '工具已就绪' : s.error"
                ></span
              ></label>
            </div>
            <p v-else-if="!loading" class="muted">
              至少需要两台已接入 Agent 的服务器。
            </p>
            <div v-if="notReady.length" class="tool-notice">
              <p v-for="s in notReady" :key="s.id">
                <strong>{{ s.name }}</strong
                >：{{
                  s.prepareRequested
                    ? "正在准备测速工具…"
                    : s.error || "测速工具未就绪"
                }}
              </p>
              <UiButton
                variant="ghost"
                :loading="preparing"
                :disabled="notReady.every((s) => s.prepareRequested)"
                @click="prepare"
                >准备测速工具</UiButton
              ><small>旧 Agent 请先到“监控与 Agent”更新。</small>
            </div>
            <div class="settings-grid">
              <label
                >协议<select v-model="settings.protocol">
                  <option value="tcp">TCP</option>
                  <option value="udp">UDP</option>
                </select></label
              >
              <label
                >IP 版本<select v-model.number="settings.family">
                  <option :value="4">IPv4</option>
                  <option :value="6">IPv6</option>
                </select></label
              >
              <label class="full"
                >测试方向<select v-model="settings.direction">
                  <option value="both">双向分别测试（推荐）</option>
                  <option value="forward">中转 → 落地</option>
                  <option value="reverse">落地 → 中转</option>
                  <option value="bidir">双向同时测试</option>
                </select></label
              >
              <label
                >每项时长（秒）<input
                  v-model.number="settings.duration"
                  type="number"
                  min="5"
                  max="30"
                  required
              /></label>
              <label
                >并行连接<input
                  v-model.number="settings.streams"
                  type="number"
                  min="1"
                  max="8"
                  required
              /></label>
              <label
                >总速率（Mbps）<input
                  v-model.number="settings.rateMbps"
                  type="number"
                  min="1"
                  max="10000"
                  step="1"
                  required
              /></label>
              <label
                >测试端口<input
                  v-model.number="settings.port"
                  type="number"
                  min="1024"
                  max="65535"
                  required
              /></label>
            </div>
            <p class="muted">
              落地需放行所选 TCP 端口；UDP 测试还需同端口
              UDP。临时监听在结束后关闭。
            </p>
            <div class="traffic-budget">
              <span>本批预计测试数据</span
              ><strong>{{ formatBytes(estimated) }}</strong
              ><small
                >按设定速率估算，不含协议开销。双向同时模式共享总速率；实际账单以服务商统计为准。</small
              >
            </div>
            <UiButton
              variant="primary"
              :icon="Position"
              type="submit"
              :loading="starting"
              :disabled="!canStart"
              >{{
                settings.sourceIds.length > 1 ? "开始中转对比" : "开始互测"
              }}</UiButton
            >
            <p v-if="!canStart && !starting" class="muted">
              {{
                isRunning
                  ? "当前测试结束或取消后，可以发起下一批。"
                  : "选择不同的两端，准备好工具后开始。"
              }}
            </p>
          </form>
        </UiPanel>
        <UiPanel title="最近测试" description="保留 7 天，显示最近 30 批。">
          <p v-if="historyError" class="error">{{ historyError }}</p>
          <EmptyState
            v-if="!history.length && !loading"
            size="inline"
            description="尚无测速记录。"
          />
          <div class="lab-history">
            <button
              v-for="r in history"
              :key="r.id"
              :class="{ 'is-active': detail?.run.id === r.id }"
              @click="chooseRun(r.id)"
            >
              <strong>{{ r.landingName }}</strong
              ><span
                >{{ r.protocol.toUpperCase() }} · IPv{{ r.family }} ·
                {{ formatDateTime(r.createdAt) }}</span
              >
            </button>
          </div>
        </UiPanel>
      </aside>
      <section
        class="measurements"
        aria-label="测速结果"
        :aria-busy="detailLoading"
      >
        <div v-if="error" class="result-error" role="alert">
          <span>{{ error }}</span
          ><UiButton v-if="selectedId" @click="loadDetail">重试</UiButton>
        </div>
        <UiSkeleton v-if="detailLoading && !detail" :lines="5" height="48px" />
        <UiPanel v-else-if="!detail" title="寻找更合适的中转"
          ><EmptyState
            size="section"
            title="同一落地，实际互传比较"
            description="选择落地与候选中转后，查看接收速率、重传或丢包，并结合路径拓扑判断线路。"
        /></UiPanel>
        <template v-else>
          <div class="measurement-heading">
            <div>
              <h2>到 {{ detail.run.landingName }} 的线路</h2>
              <p>
                {{ detail.run.target }} ·
                {{ detail.run.protocol.toUpperCase() }} · IPv{{
                  detail.run.family
                }}
                · {{ detail.run.streams }} 连接 · {{ detail.run.rateMbps }} Mbps
                上限 · {{ detail.run.duration }} 秒
              </p>
            </div>
            <UiButton v-if="isRunning" :loading="cancelling" @click="cancel"
              >停止测试</UiButton
            ><UiButton v-else @click="viewPaths">对比路径</UiButton>
          </div>
          <div class="measurement-strip">
            <span>{{
              isRunning
                ? "正在依次测试"
                : detail.run.status === "cancelled"
                  ? "本批已停止"
                  : detail.run.status === "failed"
                    ? "本批未完成"
                    : "本批已结束"
            }}</span
            ><span>{{ completed }} / {{ detail.jobs.length }} 项完成</span
            ><span>接收 {{ formatBytes(actualBytes) }}</span>
          </div>
          <p class="interpretation">
            这是 VPS 之间的直接网络能力，不能代表 Reality / HY2
            的完整代理速度。达到设定上限只说明当前速率可用；单连接和多连接应分开比较。CPU
            为 iperf3 进程占用，可能超过 100%。
          </p>
          <div class="result-toolbar">
            <label
              >展示顺序<select v-model="sort">
                <option value="order">按执行顺序</option>
                <option value="rate">按接收速率</option>
              </select></label
            ><label v-if="sort === 'rate'"
              >比较方向<select v-model="directionFilter">
                <option value="forward">中转 → 落地</option>
                <option value="reverse">落地 → 中转</option>
              </select></label
            >
          </div>
          <EmptyState
            v-if="!jobs.length"
            size="inline"
            description="当前方向尚无测试项。"
          />
          <article
            v-for="j in jobs"
            :key="j.id"
            class="measurement"
            :class="j.status"
          >
            <header>
              <div class="route-identity">
                <strong>{{ j.sourceName }}</strong
                ><span aria-hidden="true">{{
                  j.direction === "reverse"
                    ? "←"
                    : j.direction === "bidir"
                      ? "↔"
                      : "→"
                }}</span
                ><strong>{{ detail.run.landingName }}</strong>
              </div>
              <span class="job-status" role="status">{{
                statuses[j.status]
              }}</span>
            </header>
            <p v-if="j.error" class="error">{{ j.error }}</p>
            <p
              v-if="
                j.status === 'queued' ||
                j.status === 'starting' ||
                j.status === 'running'
              "
              class="muted"
            >
              {{
                j.status === "queued"
                  ? "等待前一项完成，避免争抢落地带宽。"
                  : j.status === "starting"
                    ? "等待落地 Agent 创建临时监听。"
                    : "两端正在直接互传，结果逐步返回；部分工具版本在结束后返回曲线。"
              }}
            </p>
            <div
              v-if="
                j.status === 'running' && j.result?.progressIntervals?.length
              "
              class="live-progress"
            >
              <span>{{
                j.result.progressObservation === "receiver"
                  ? "实时接收观察"
                  : "实时发送观察（最终以接收端为准）"
              }}</span
              ><svg
                class="rate-curve"
                viewBox="0 0 280 80"
                role="img"
                aria-label="当前测试实时速率曲线"
              >
                <path d="M0 70H280" class="baseline" />
                <polyline :points="curvePoints(j.result.progressIntervals)" />
              </svg>
            </div>
            <div v-if="j.result" class="direction-results">
              <div
                v-for="d in directions(j)"
                :key="d.key"
                class="direction-result"
              >
                <div class="rate-line">
                  <span>{{ d.label }}</span
                  ><strong>{{ formatRate(d.value.bitsPerSecond) }}</strong>
                </div>
                <svg
                  v-if="d.value.intervals?.length"
                  class="rate-curve"
                  viewBox="0 0 280 80"
                  role="img"
                  :aria-label="`${d.label}接收速率曲线，平均${formatRate(d.value.bitsPerSecond)}`"
                >
                  <path d="M0 70H280" class="baseline" />
                  <polyline :points="curvePoints(d.value.intervals)" />
                </svg>
                <dl>
                  <div>
                    <dt>接收数据</dt>
                    <dd>{{ formatBytes(d.value.bytes) }}</dd>
                  </div>
                  <template v-if="detail.run.protocol === 'udp'"
                    ><div>
                      <dt>丢包</dt>
                      <dd>{{ d.value.lossPercent.toFixed(2) }}%</dd>
                    </div>
                    <div>
                      <dt>抖动</dt>
                      <dd>{{ d.value.jitterMs.toFixed(2) }} ms</dd>
                    </div></template
                  ><template v-else
                    ><div>
                      <dt>TCP 重传</dt>
                      <dd>{{ d.value.retransmits }}</dd>
                    </div>
                    <div v-if="d.value.rttMs != null">
                      <dt>TCP 平均往返</dt>
                      <dd>{{ d.value.rttMs.toFixed(2) }} ms</dd>
                    </div></template
                  >
                </dl>
              </div>
            </div>
            <footer v-if="j.result">
              <span v-if="j.result.capReached" class="cap-badge"
                >已达测试上限，真实极限待测</span
              ><span v-else>设定总速率 {{ detail.run.rateMbps }} Mbps</span
              ><span
                >中转 CPU {{ j.result.cpuHost.toFixed(1) }}% · 对端
                {{ j.result.cpuRemote.toFixed(1) }}%</span
              >
            </footer>
            <p
              v-for="warning in j.result?.warnings || []"
              :key="warning"
              class="muted"
            >
              {{ warning }}
            </p>
          </article>
        </template>
      </section>
    </div>
  </div>
</template>
<style scoped>
.throughput-page {
  min-width: 0;
}
.lab-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 18px;
  align-items: start;
}
.lab-side,
.measurements {
  display: grid;
  gap: 14px;
  min-width: 0;
}
.lab-form {
  display: grid;
  gap: 12px;
}
.lab-form > label,
.source-title strong {
  font-size: 13px;
  font-weight: 600;
}
.throughput-page select,
.throughput-page input[type="number"] {
  box-sizing: border-box;
  width: 100%;
  min-width: 0;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-ink);
  border-radius: 9px;
  padding: 9px 10px;
  font: inherit;
  font-size: 13px;
  min-height: 40px;
}
.throughput-page select:focus-visible,
.throughput-page input:focus-visible,
.throughput-page button:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.source-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 12px;
}
.source-title span,
.muted,
.interpretation {
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.7;
  margin: 0;
}
.candidate-list {
  display: grid;
  gap: 6px;
  max-height: 280px;
  overflow-y: auto;
}
.candidate {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  cursor: pointer;
  min-width: 0;
}
.candidate.is-selected {
  border-color: var(--c-data);
  background: var(--c-selected);
}
.candidate input {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
  accent-color: var(--c-data);
}
.candidate > span:nth-child(2) {
  display: grid;
  gap: 3px;
  min-width: 0;
  flex: 1;
}
.candidate strong {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.candidate small {
  font-size: 11px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.ready-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
  flex-shrink: 0;
}
.ready-dot.ready {
  background: var(--c-ok);
}
.settings-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.settings-grid label {
  display: grid;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  color: var(--c-muted);
}
.settings-grid .full {
  grid-column: 1/-1;
}
.tool-notice {
  padding: 10px;
  background: var(--c-surface-2);
  border-radius: 10px;
  font-size: 12px;
  overflow-wrap: anywhere;
}
.tool-notice p {
  margin: 0 0 6px;
}
.tool-notice small {
  display: block;
  color: var(--c-muted);
  line-height: 1.7;
}
.traffic-budget {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 7px;
  border-top: 1px solid var(--c-line);
  padding-top: 14px;
  font-size: 12px;
}
.traffic-budget strong {
  color: var(--c-data);
  font-size: 15px;
}
.traffic-budget small {
  grid-column: 1/-1;
  color: var(--c-muted);
  line-height: 1.7;
}
.lab-history {
  display: grid;
  gap: 4px;
  max-height: 280px;
  overflow: auto;
}
.lab-history button {
  padding: 10px;
  text-align: left;
  display: grid;
  gap: 5px;
  border-radius: 9px;
  min-width: 0;
}
.lab-history button.is-active,
.lab-history button:hover {
  background: var(--c-selected);
}
.lab-history strong {
  font-size: 13px;
  overflow-wrap: anywhere;
}
.lab-history span {
  font-size: 11px;
  color: var(--c-muted);
}
.error {
  color: var(--c-warn-ink);
  font-size: 13px;
  overflow-wrap: anywhere;
  line-height: 1.7;
  margin: 0;
}
.result-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
}
.measurement-heading {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}
.measurement-heading > div {
  min-width: 0;
}
.measurement-heading h2 {
  font-size: 21px;
  letter-spacing: -0.5px;
  margin: 0;
  overflow-wrap: anywhere;
}
.measurement-heading p {
  font-size: 12px;
  color: var(--c-muted);
  margin: 6px 0 0;
  overflow-wrap: anywhere;
}
.measurement-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 20px;
  border-block: 1px solid var(--c-line);
  padding: 12px 0;
  font-size: 12px;
  color: var(--c-muted);
  font-variant-numeric: tabular-nums;
}
.measurement-strip span:first-child {
  color: var(--c-data);
}
.result-toolbar {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.result-toolbar label {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  min-width: 0;
}
.result-toolbar select {
  width: auto;
  font-size: 12px;
}
.measurement {
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  border-radius: 14px;
  padding: 18px;
  display: grid;
  gap: 14px;
  min-width: 0;
}
.measurement.running,
.measurement.starting {
  border-color: var(--c-data);
}
.measurement header {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: start;
}
.route-identity {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
  font-size: 14px;
  align-items: center;
}
.route-identity strong {
  overflow-wrap: anywhere;
}
.route-identity > span {
  color: var(--c-data);
  font-size: 19px;
}
.job-status {
  flex-shrink: 0;
  font-size: 12px;
  color: var(--c-muted);
}
.completed .job-status {
  color: var(--c-ok);
}
.failed .job-status,
.timeout .job-status {
  color: var(--c-warn-ink);
}
.direction-results {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 230px), 1fr));
  gap: 20px;
}
.direction-result {
  min-width: 0;
}
.rate-line {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 8px;
  align-items: baseline;
}
.rate-line > span {
  font-size: 12px;
  color: var(--c-muted);
}
.rate-line > strong {
  font-size: 25px;
  font-weight: 600;
  letter-spacing: -0.6px;
  font-variant-numeric: tabular-nums;
}
.rate-curve {
  width: 100%;
  height: 90px;
  display: block;
  margin-top: 12px;
  overflow: visible;
}
.rate-curve polyline {
  fill: none;
  stroke: var(--c-data);
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
}
.rate-curve .baseline {
  fill: none;
  stroke: var(--c-line);
  stroke-width: 1;
}
.direction-result dl {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
  margin: 10px 0 0;
}
.direction-result dl > div {
  display: grid;
  gap: 4px;
}
.direction-result dt {
  font-size: 11px;
  color: var(--c-muted);
}
.direction-result dd {
  margin: 0;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
}
.measurement footer {
  border-top: 1px solid var(--c-line);
  padding-top: 12px;
  display: flex;
  gap: 8px 20px;
  flex-wrap: wrap;
  font-size: 11px;
  color: var(--c-muted);
}
.cap-badge {
  color: var(--c-data);
}
.live-progress {
  font-size: 12px;
  color: var(--c-muted);
}
@media (max-width: 1050px) {
  .lab-layout {
    grid-template-columns: 280px minmax(0, 1fr);
  }
}
@media (max-width: 760px) {
  .lab-layout {
    grid-template-columns: 1fr;
  }
  .lab-side {
    grid-template-columns: minmax(0, 1fr);
  }
  .lab-history {
    max-height: 150px;
  }
  .measurement-heading h2 {
    font-size: 19px;
  }
}
@media (max-width: 400px) {
  .measurement {
    padding: 13px;
  }
  .measurement header {
    flex-direction: column;
    gap: 7px;
  }
  .result-toolbar label {
    width: 100%;
    justify-content: space-between;
  }
  .rate-line > strong {
    font-size: 23px;
  }
  .settings-grid {
    gap: 10px;
  }
}
</style>
