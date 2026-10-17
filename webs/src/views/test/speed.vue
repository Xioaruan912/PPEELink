<template>
  <div class="test-page speed-page">
    <PageHeader title="真实测速" description="经节点下载，测量真实出口吞吐。">
      <template #actions
        ><UiButton
          v-if="loading"
          variant="danger"
          :icon="Close"
          @click="abortTest"
          >中断测速</UiButton
        ><UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="loading"
          :disabled="!selectedIds.length || nodesLoading"
          :aria-describedby="
            !selectedIds.length ? 'speed-start-hint' : undefined
          "
          @click="startTest"
          >{{ loading ? "正在测速" : "开始测速" }}</UiButton
        ><span
          v-if="!selectedIds.length"
          id="speed-start-hint"
          class="start-hint"
          >先选择节点</span
        ></template
      >
    </PageHeader>
    <div v-if="nodesError || testError" class="test-notice" role="alert">
      <span>{{ testError || nodesError }}</span
      ><UiButton v-if="nodesError" variant="ghost" @click="loadNodes"
        >重试节点</UiButton
      >
    </div>
    <div class="speed-stage" data-enter>
      <section
        class="speed-instrument"
        :class="{ 'is-running': loading }"
        :aria-busy="loading"
        aria-label="实时测速仪表"
      >
        <div class="instrument-heading">
          <StatusDot
            :state="loading ? 'warn' : results.length ? 'ok' : 'idle'"
            :label="
              loading ? '测量进行中' : results.length ? '上次测量' : '准备就绪'
            "
          /><span class="mono"
            >{{ results.length }} / {{ runTotal || selectedIds.length }}</span
          >
        </div>
        <div class="instrument-reading">
          <UiStat
            size="hero"
            :label="!loading && results.length ? '本轮最佳' : '下载速率'"
            :value="heroResult?.mbps ?? null"
            :decimals="2"
            unit="Mbps"
          />
          <p class="current-node">
            {{
              heroResult?.name ||
              (loading
                ? "等待有效读数返回"
                : results.length
                  ? "暂无有效读数"
                  : "选择节点，开始测量")
            }}
          </p>
          <p class="reading-note">
            {{
              loading
                ? "每个节点测量完成后更新读数"
                : cachedAt
                  ? `结果保存于 ${formatDateTime(cachedAt)}`
                  : "使用实际下载字节与耗时计算"
            }}
          </p>
        </div>
        <div class="instrument-scale" aria-hidden="true">
          <i
            v-for="tick in 25"
            :key="tick"
            :class="{
              'is-lit':
                heroResult &&
                tick / 25 <= heroResult.mbps / Math.max(maxThroughput, 1),
            }"
          ></i>
        </div>
        <div
          class="instrument-progress"
          role="progressbar"
          aria-label="已完成节点"
          :aria-valuenow="results.length"
          :aria-valuemin="0"
          :aria-valuemax="runTotal || selectedIds.length || 1"
        >
          <i
            :style="{
              transform: `scaleX(${results.length / Math.max(runTotal || selectedIds.length, 1)})`,
            }"
          ></i>
        </div>
        <div class="instrument-foot">
          <span>{{ loading ? "正在接收结果" : runStatus }}</span
          ><span>{{ sizeMB }} MB / {{ timeoutSec }} 秒</span>
        </div>
      </section>
      <section class="speed-controls" aria-labelledby="node-title">
        <header class="section-heading">
          <h2 id="node-title">选择节点</h2>
          <UiButton
            variant="ghost"
            :disabled="loading || nodesLoading || !nodes.length"
            @click="selectAll"
            >全选可用 · {{ nodes.length }}</UiButton
          >
        </header>
        <label for="speed-nodes" class="field-label"
          >测速节点 · 已选 {{ selectedIds.length }}</label
        >
        <el-select
          id="speed-nodes"
          v-model="selectedIds"
          aria-label="测速节点"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          :disabled="loading"
          :loading="nodesLoading"
          placeholder="搜索并选择节点"
        >
          <el-option
            v-for="node in nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
            ><span class="node-option"
              ><span>{{ node.name }}</span
              ><small>{{ node.rtt }} ms</small></span
            ></el-option
          >
        </el-select>
        <EmptyState
          v-if="!nodesLoading && !nodes.length && !nodesError"
          size="inline"
          description="暂无可用节点，请先在节点管理检查连接。"
        />
        <div class="speed-summary">
          <UiStat
            size="medium"
            label="峰值吞吐"
            :value="bestResult?.mbps ?? null"
            :decimals="1"
            unit="Mbps"
          /><UiStat
            size="small"
            label="平均速率"
            :value="avgThroughput"
            :decimals="1"
            unit="Mbps"
          />
        </div>
        <details class="speed-advanced">
          <summary>更多测速设置</summary>
          <div class="advanced-fields">
            <div class="field-heading">
              <label for="speed-target">测速地址</label
              ><UiButton
                variant="ghost"
                :disabled="loading"
                @click="target = speedTarget(defaultSpeedTarget, sizeMB)"
                >恢复默认</UiButton
              >
            </div>
            <el-input
              id="speed-target"
              v-model="target"
              :disabled="loading"
              placeholder="https://speed.cloudflare.com/__down?bytes=10000000"
            />
            <label class="field-label">测试下载量 · {{ sizeMB }} MB</label
            ><UiSegmented
              :model-value="sizeMB"
              label="测试下载量"
              :options="
                [10, 25, 50, 100].map((value) => ({
                  label: `${value} MB`,
                  value,
                  disabled: loading,
                }))
              "
              @update:model-value="setSize(Number($event))"
            />
            <label class="field-label">最长等待时间</label
            ><UiSegmented
              v-model="timeoutSec"
              label="最长等待时间"
              :options="
                [10, 20, 30, 60].map((value) => ({
                  label: `${value} 秒`,
                  value,
                  disabled: loading,
                }))
              "
            />
          </div>
        </details>
      </section>
    </div>
    <section class="results-section" aria-labelledby="results-title" data-enter>
      <header class="section-heading">
        <div>
          <h2 id="results-title">
            测速结果 <span class="section-count">{{ results.length }}</span>
          </h2>
          <p>下载速率与本次峰值对比</p>
        </div>
        <div class="result-filters">
          <el-input
            v-model="query"
            clearable
            aria-label="筛选测速结果"
            placeholder="搜索节点"
          /><el-select v-model="sort" aria-label="结果排序"
            ><el-option label="速度从高到低" value="fast" /><el-option
              label="速度从低到高"
              value="slow" /><el-option
              label="连接延迟"
              value="rtt" /><el-option label="返回顺序" value="arrival"
          /></el-select>
        </div>
      </header>
      <div v-if="results.length" class="result-columns" aria-hidden="true">
        <span>节点 / 状态</span><span>下载速度</span><span>连接响应</span
        ><span>下载量 / 耗时</span>
      </div>
      <TransitionGroup
        appear
        name="speed-result"
        tag="div"
        class="speed-results"
        :css="!reduced"
        @enter="enter"
        @leave="leave"
      >
        <article
          v-for="(row, index) in sortedResults"
          :key="row.id"
          :data-index="index"
          class="speed-result"
        >
          <div class="result-node">
            <CountryFlag
              :code="nodes.find((n) => n.id === row.id)?.countryCode"
            />
            <div>
              <strong>{{ row.name }}</strong
              ><span :class="row.ok ? 'result-ok' : 'result-error'">{{
                row.error || (row.ok ? "连接正常" : "待测")
              }}</span>
            </div>
          </div>
          <div class="result-speed">
            <span class="mono"
              >{{ row.ok ? row.mbps.toFixed(2) : "失败" }}
              <small v-if="row.ok">Mbps</small></span
            ><span class="speed-line" aria-hidden="true"
              ><i
                :style="{
                  transform: `scaleX(${row.ok ? row.mbps / Math.max(maxThroughput, 1) : 0})`,
                }"
              ></i
            ></span>
          </div>
          <span class="result-rtt mono" :class="rttClass(row.rtt)"
            ><small class="mobile-label">连接响应</small
            >{{ row.rtt >= 0 ? `${row.rtt} ms` : "超时" }}</span
          >
          <span class="result-volume mono"
            ><small class="mobile-label">下载量 / 耗时</small
            >{{ row.ok ? `${(row.bytes / 1048576).toFixed(1)} MB` : "未完成"
            }}<small>{{
              row.seconds ? `${row.seconds} s` : "无耗时记录"
            }}</small></span
          >
        </article>
      </TransitionGroup>
      <EmptyState
        v-if="!sortedResults.length"
        size="section"
        :title="
          loading
            ? '等待测量结果'
            : results.length
              ? '未找到节点'
              : '还没有测速结果'
        "
        :description="
          loading
            ? '服务端正在通过所选节点下载，完成后逐条显示。'
            : results.length
              ? '尝试其他关键词。'
              : '选择节点后开始测速，结果会在此保留一小时。'
        "
        ><template v-if="!results.length && !loading" #action
          ><UiButton
            :disabled="nodesLoading || !nodes.length"
            @click="selectAll"
            >全选可用节点</UiButton
          ></template
        ></EmptyState
      >
    </section>
    <QualityTrend />
  </div>
</template>
<script setup lang="ts">
import CountryFlag from "@/ui/CountryFlag.vue";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { Close, VideoPlay } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiStat,
  UiSegmented,
  StatusDot,
  EmptyState,
} from "@/ui";
import { getNodeOverview } from "@/api/subcription/node";
import { getSpeed, saveSpeed } from "@/utils/nodeTestStorage";
import { formatDateTime } from "@/utils/date";
import { usePrefersReducedMotion } from "@/motion";
import { useResultMotion } from "@/components/node-tests/useNodeTestSession";
import { useTestStream } from "./useTestStream";
import { defaultSpeedTarget, speedTarget } from "./speed/settings";
import QualityTrend from "./speed/QualityTrend.vue";
defineOptions({ name: "SpeedTest" });
type NodeItem = { id: number; name: string; rtt: number; countryCode?: string };
type SpeedResult = {
  id: number;
  name: string;
  rtt: number;
  ok: boolean;
  mbps: number;
  bytes: number;
  seconds?: number;
  error?: string;
  index?: number;
  total?: number;
};
type SpeedCache = {
  results: SpeedResult[];
  selectedIds: number[];
  target: string;
  sizeMB: number;
  timeoutSec: number;
  updatedAt: number;
  runTotal: number;
  status: string;
};
const cached = getSpeed<SpeedCache>();
const nodes = ref<NodeItem[]>([]),
  nodesLoading = ref(true),
  nodesError = ref("");
const selectedIds = ref<number[]>(cached?.selectedIds || []);
const target = ref(cached?.target || defaultSpeedTarget),
  sizeMB = ref(cached?.sizeMB || 10),
  timeoutSec = ref(cached?.timeoutSec || 20);
const results = ref<SpeedResult[]>(cached?.results || []),
  cachedAt = ref(cached?.updatedAt || 0);
const runTotal = ref(cached?.runTotal || 0),
  runStatus = ref(
    (cached?.status === "测量进行中" ? "上次测量未完成" : cached?.status) ||
      "等待开始"
  );
const testError = ref("");
const query = ref(""),
  sort = ref("fast");
const { loading, run, cancel } = useTestStream();
const { enter, leave } = useResultMotion();
const reduced = usePrefersReducedMotion();
let disposed = false;
let runVersion = 0;
const latest = computed(() => results.value.at(-1));
const successfulResults = computed(() =>
  results.value.filter((row) => row.ok && Number.isFinite(row.mbps))
);
const bestResult = computed(() =>
  successfulResults.value.reduce<SpeedResult | undefined>(
    (best, row) => (!best || row.mbps > best.mbps ? row : best),
    undefined
  )
);
const heroResult = computed(() =>
  loading.value
    ? latest.value?.ok && Number.isFinite(latest.value.mbps)
      ? latest.value
      : null
    : (bestResult.value ?? null)
);
const maxThroughput = computed(() => bestResult.value?.mbps ?? 0);
const avgThroughput = computed(() => {
  const ok = successfulResults.value;
  return ok.length
    ? ok.reduce((sum, row) => sum + row.mbps, 0) / ok.length
    : null;
});
const sortedResults = computed(() => {
  const rows = results.value.filter((row) =>
    row.name.toLowerCase().includes(query.value.toLowerCase())
  );
  return sort.value === "arrival"
    ? rows
    : [...rows].sort((a, b) =>
        sort.value === "rtt"
          ? (a.rtt < 0 ? Infinity : a.rtt) - (b.rtt < 0 ? Infinity : b.rtt)
          : sort.value === "slow"
            ? a.mbps - b.mbps
            : b.mbps - a.mbps
      );
});
function persist() {
  if (!results.value.length) return;
  cachedAt.value = Date.now();
  saveSpeed<SpeedCache>({
    results: results.value,
    selectedIds: selectedIds.value,
    target: target.value,
    sizeMB: sizeMB.value,
    timeoutSec: timeoutSec.value,
    updatedAt: cachedAt.value,
    runTotal: runTotal.value,
    status: runStatus.value,
  });
}
function setSize(mb: number) {
  sizeMB.value = mb;
  target.value = speedTarget(target.value, mb);
}
function selectAll() {
  selectedIds.value = nodes.value.map((node) => node.id);
}
function rttClass(rtt: number) {
  return rtt < 0
    ? "result-error"
    : rtt < 100
      ? "result-ok"
      : rtt < 250
        ? "result-warn"
        : "result-error";
}
async function loadNodes() {
  nodesLoading.value = true;
  nodesError.value = "";
  try {
    const { data } = await getNodeOverview();
    if (disposed) return;
    nodes.value = (data || [])
      .filter((node: NodeItem) => node.rtt >= 0)
      .sort((a: NodeItem, b: NodeItem) => a.rtt - b.rtt);
    selectedIds.value = selectedIds.value.filter((id) =>
      nodes.value.some((node) => node.id === id)
    );
  } catch {
    if (!disposed) nodesError.value = "节点读取失败，请重试。";
  } finally {
    if (!disposed) nodesLoading.value = false;
  }
}
async function startTest() {
  if (loading.value) return;
  if (!selectedIds.value.length) {
    ElMessage.warning("请先选择测速节点");
    return;
  }
  results.value = [];
  cachedAt.value = 0;
  query.value = "";
  runTotal.value = selectedIds.value.length;
  testError.value = "";
  runStatus.value = "测量进行中";
  const payload = new URLSearchParams({
    ids: selectedIds.value.join(","),
    target: target.value,
    size: String(sizeMB.value * 1048576),
    timeout: String(timeoutSec.value),
  });
  const ownVersion = ++runVersion;
  let completed = false;
  try {
    await run("/api/v1/nodes/speedtest/stream", payload, (type, data) => {
      if (type === "node") {
        const index = results.value.findIndex((row) => row.id === data.id);
        if (index >= 0) results.value[index] = data;
        else results.value.push(data);
        if (data.total) runTotal.value = data.total;
        persist();
      }
      if (type === "done") {
        completed = true;
        runStatus.value = "测量完成";
        persist();
        ElMessage.success("测速完成");
      }
    });
    if (
      !disposed &&
      ownVersion === runVersion &&
      !completed &&
      runStatus.value !== "已中断"
    ) {
      runStatus.value = "连接中断";
      testError.value = "结果流提前结束，可重新测速。";
    }
  } catch (cause: any) {
    if (disposed || ownVersion !== runVersion) return;
    testError.value = cause.message || "测速失败";
    runStatus.value = "测速失败";
    ElMessage.error(testError.value);
  } finally {
    if (!disposed && ownVersion === runVersion) persist();
  }
}
function abortTest() {
  runVersion++;
  cancel();
  runStatus.value = "已中断";
  persist();
  ElMessage.info("已中断测速，已返回结果保留");
}
onMounted(loadNodes);
onBeforeUnmount(() => {
  if (runStatus.value === "测量进行中") {
    runStatus.value = "已中断";
    persist();
  }
  disposed = true;
});
</script>
<style lang="scss" scoped>
@use "./test-page.scss";
.start-hint {
  align-self: center;
  font-size: 14px;
  color: var(--c-muted);
}
.speed-stage {
  display: grid;
  grid-template-columns: minmax(0, 7fr) minmax(0, 5fr);
  gap: 48px;
  margin-bottom: 64px;
}
.speed-instrument {
  position: relative;
  min-width: 0;
  overflow: hidden;
  padding: 28px 32px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}
.instrument-heading,
.instrument-foot {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  color: var(--c-muted);
  font-size: 14px;
}
.instrument-heading .mono {
  font-size: 13px;
}
.instrument-reading {
  padding: 32px 0 28px;
}
.instrument-reading :deep(.ui-stat--hero .ui-stat__number) {
  font-size: clamp(80px, 9vw, 132px);
  color: var(--c-ink);
}
.is-running
  .instrument-reading
  :deep(.ui-stat--hero .ui-stat__number:not(.is-empty)) {
  color: var(--c-signal);
}
.instrument-reading :deep(.ui-stat--hero .ui-stat__number.is-empty) {
  color: var(--c-muted);
}
.instrument-reading :deep(.ui-stat__label) {
  margin-bottom: 8px;
}
.current-node {
  font-size: 18px;
  margin: 24px 0 8px;
  overflow-wrap: anywhere;
}
.reading-note {
  min-height: 44px;
  color: var(--c-muted);
  font-size: 14px;
  margin: 0;
  overflow-wrap: anywhere;
}
.instrument-scale {
  height: 36px;
  display: grid;
  grid-template-columns: repeat(25, 1fr);
  gap: 6px;
  align-items: end;
  margin-bottom: 24px;
}
.instrument-scale i {
  height: 16px;
  background: var(--c-line);
  transition: background var(--duration-medium);
}
.instrument-scale i:nth-child(5n + 1) {
  height: 32px;
}
.instrument-scale .is-lit {
  background: var(--c-signal);
}
.instrument-progress {
  height: 2px;
  background: var(--c-line);
  overflow: hidden;
  margin-bottom: 16px;
}
.instrument-progress i {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--c-signal);
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
.is-running::after {
  content: "";
  position: absolute;
  inset: 0 auto 0 0;
  width: 30%;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    transparent,
    color-mix(in srgb, var(--c-signal) 7%, transparent),
    transparent
  );
  animation: instrument-scan 2s linear infinite;
}
.speed-controls {
  min-width: 0;
}
.speed-summary {
  display: grid;
  grid-template-columns: 1.2fr 1fr;
  align-items: end;
  gap: 16px;
  padding: 32px 0;
}
.speed-advanced {
  border-top: 1px solid var(--c-line);
  padding-top: 16px;
}
.speed-advanced summary {
  min-height: 44px;
  cursor: pointer;
  font-size: 15px;
  line-height: 44px;
}
.advanced-fields {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 16px;
}
.advanced-fields .field-label {
  margin: 8px 0 0;
}
.field-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
}
.results-section {
  margin-bottom: 64px;
}
.result-filters {
  display: flex;
  gap: 12px;
  max-width: 100%;
}
.result-filters .el-input {
  width: 180px;
}
.result-filters .el-select {
  width: 190px;
}
.result-columns,
.speed-result {
  display: grid;
  grid-template-columns:
    minmax(0, 2.4fr) minmax(0, 1.4fr) minmax(0, 1fr)
    minmax(0, 1.2fr);
  gap: 24px;
  align-items: center;
}
.result-columns {
  padding: 16px 0;
  color: var(--c-muted);
  font: 400 13px var(--font-mono);
  border-bottom: 1px solid var(--c-line);
}
.speed-results {
  position: relative;
}
.speed-result {
  min-height: 96px;
  border-bottom: 1px solid var(--c-line);
  padding: 20px 0;
}
.speed-result:hover {
  background: var(--c-surface-2);
}
.result-node {
  display: flex;
  gap: 12px;
  align-items: center;
  min-width: 0;
}
.result-node > div {
  min-width: 0;
}
.result-node strong {
  display: block;
  overflow-wrap: anywhere;
  font-size: 15px;
  font-weight: 500;
}
.result-node div > span {
  display: block;
  font-size: 13px;
  margin-top: 8px;
  overflow-wrap: anywhere;
}
.result-speed {
  min-width: 0;
}
.result-speed > .mono {
  font-size: 22px;
}
.result-speed small {
  font: 400 13px var(--font-mono);
  color: var(--c-muted);
}
.speed-line {
  display: block;
  height: 2px;
  background: var(--c-line);
  margin-top: 12px;
  overflow: hidden;
}
.speed-line i {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--c-data);
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
.speed-result-enter-from .speed-line i {
  transform: scaleX(0) !important;
}
.result-volume {
  font-size: 14px;
}
.result-volume small:not(.mobile-label) {
  display: block;
  font-size: 13px;
  color: var(--c-muted);
  margin-top: 4px;
}
.result-rtt {
  font-size: 14px;
}
.mobile-label {
  display: none;
}
.speed-result-move {
  transition: transform var(--duration-slow) var(--ease-spring);
}
.speed-result-leave-active {
  position: absolute;
  width: 100%;
}
@keyframes instrument-scan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(450%);
  }
}
@media (max-width: 1023px) {
  .speed-stage {
    gap: 24px;
  }
  .result-filters {
    flex-wrap: wrap;
  }
  .result-columns,
  .speed-result {
    gap: 16px;
  }
}
@media (max-width: 767px) {
  .speed-stage {
    grid-template-columns: 1fr;
    gap: 32px;
    margin-bottom: 48px;
  }
  .speed-instrument {
    padding: 24px;
  }
  .instrument-reading {
    padding: 24px 0;
  }
  .result-columns {
    display: none;
  }
  .result-filters {
    width: 100%;
  }
  .result-filters .el-input,
  .result-filters .el-select {
    width: 100%;
  }
  .speed-result {
    grid-template-columns: 1fr 1fr;
    gap: 24px 16px;
  }
  .result-node {
    grid-column: 1 / -1;
  }
  .result-speed {
    grid-column: 1 / -1;
  }
  .mobile-label,
  .result-volume small.mobile-label {
    display: block;
    color: var(--c-muted);
    font: 400 13px var(--font-ui);
    margin: 0 0 8px;
  }
  .instrument-foot {
    flex-wrap: wrap;
  }
  .speed-summary {
    padding: 24px 0;
  }
  .results-section {
    margin-bottom: 48px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .is-running::after {
    animation: none;
    display: none;
  }
  .instrument-progress i,
  .speed-line i,
  .speed-result-move,
  .instrument-scale i {
    transition: none;
  }
}
:global(html[data-motion="reduced"]) .is-running::after {
  display: none;
}
:global(html[data-motion="reduced"]) .speed-line i,
:global(html[data-motion="reduced"]) .instrument-progress i,
:global(html[data-motion="reduced"]) .speed-result-move {
  transition: none;
}
</style>
