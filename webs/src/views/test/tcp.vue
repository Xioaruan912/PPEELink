<template>
  <div class="test-page tcp-page">
    <PageHeader title="中国 TCP" description="从所选节点检测各省网络连通性。"
      ><template #actions
        ><UiButton
          v-if="loading"
          variant="danger"
          :icon="Close"
          @click="abortTest"
          >停止检测</UiButton
        ><UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="loading"
          :disabled="!selectedNodeId || nodesLoading"
          @click="startTest"
          >{{ loading ? "正在检测" : "开始测试" }}</UiButton
        ></template
      ></PageHeader
    >
    <div v-if="error" class="test-notice" role="alert">
      <span>{{ error }}</span
      ><UiButton v-if="!nodes.length" variant="ghost" @click="loadNodes"
        >重试节点</UiButton
      >
    </div>
    <section class="tcp-controls" aria-label="检测设置" data-enter>
      <div class="node-field">
        <label class="field-label" for="tcp-node">检测节点</label
        ><el-select
          id="tcp-node"
          v-model="selectedNodeId"
          aria-label="检测节点"
          filterable
          :loading="nodesLoading"
          :disabled="loading || nodesLoading"
          placeholder="选择节点"
          @change="restoreCache"
          ><el-option
            v-for="node in nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
            ><span class="node-option"
              ><span>{{ node.name }}</span></span
            ></el-option
          ></el-select
        >
      </div>
      <div>
        <label class="field-label" for="tcp-provinces">省份 · 默认全部</label
        ><el-select
          id="tcp-provinces"
          v-model="filterProvinces"
          aria-label="检测省份"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          :disabled="loading"
          ><el-option
            v-for="province in provinceList"
            :key="province"
            :label="province"
            :value="province"
        /></el-select>
      </div>
      <div>
        <label class="field-label" for="tcp-isps">运营商 · 默认全部</label
        ><el-select
          id="tcp-isps"
          v-model="filterIsps"
          aria-label="检测运营商"
          multiple
          collapse-tags
          collapse-tags-tooltip
          :disabled="loading"
          ><el-option
            v-for="isp in ispList"
            :key="isp"
            :label="isp"
            :value="isp"
        /></el-select>
      </div>
      <div>
        <label class="field-label" for="tcp-port">zstaticcdn 端口</label
        ><el-select
          id="tcp-port"
          v-model="zstaticPort"
          aria-label="zstaticcdn 端口"
          :disabled="loading"
          ><el-option label="443 · HTTPS" value="443" /><el-option
            label="80 · HTTP"
            value="80"
        /></el-select>
      </div>
    </section>
    <div class="tcp-meta" role="status">
      <StatusDot
        :state="loading ? 'warn' : targets.length ? 'ok' : 'idle'"
        :label="loading ? `已返回 ${provinceGroups.length} 个省份` : status"
      /><span v-if="cachedAt && !loading"
        >更新于 {{ formatDateTime(cachedAt) }}</span
      >
    </div>
    <div class="tcp-stage" data-enter>
      <section class="tcp-map-section" aria-labelledby="tcp-map-title">
        <header class="section-heading">
          <h2 id="tcp-map-title">连通分布</h2>
          <span class="muted"
            >{{ reachable }} / {{ filteredTargets.length }} 个目标可达</span
          >
        </header>
        <div class="tcp-map-wrap" :class="{ 'is-running': loading }">
          <ChinaTcpMap :groups="provinceGroups" />
        </div>
        <div class="latency-legend" aria-label="延迟图例">
          <span v-for="(label, index) in latencyLegend" :key="label"
            ><i :class="`latency-band-${index}`"></i>{{ label }}</span
          >
        </div>
        <div v-if="zstaticResults.length" class="zstatic-results">
          <h3>zstaticcdn</h3>
          <span
            v-for="item in zstaticResults"
            :key="item.port"
            class="mono"
            :class="`latency-text-${latencyBand(item.rtt)}`"
            >端口 {{ item.port }} ·
            {{ item.rtt >= 0 ? `${item.rtt} ms` : "超时" }}</span
          >
        </div>
      </section>
      <aside class="tcp-rankings" aria-labelledby="ranking-title">
        <header class="section-heading">
          <h2 id="ranking-title">延迟排行</h2>
          <UiSegmented
            v-model="ranking"
            label="排行类型"
            :options="[
              { label: '省份', value: 'province' },
              { label: '运营商', value: 'isp' },
            ]"
          />
        </header>
        <TransitionGroup
          appear
          name="tcp-result"
          tag="div"
          class="ranking-list"
          :css="!reduced"
          @enter="enter"
          @leave="leave"
          ><article
            v-for="(group, index) in rankGroups"
            :key="group.name"
            class="ranking-row"
            :data-index="index"
          >
            <div class="ranking-summary">
              <span class="rank-index mono">{{
                String(index + 1).padStart(2, "0")
              }}</span
              ><strong>{{ group.name }}</strong
              ><span
                class="rank-rtt mono"
                :class="`latency-text-${latencyBand(group.avg)}`"
                >{{ group.avg >= 0 ? `${group.avg} ms` : "超时" }}</span
              >
            </div>
            <p>{{ group.okCount }} / {{ group.items.length }} 可达</p>
            <div class="target-cells">
              <div
                v-for="item in group.items"
                :key="`${item.ip}:${item.port}`"
                class="target-cell"
              >
                <span
                  >{{ ranking === "province" ? item.city : item.province }} ·
                  {{ ranking === "province" ? item.isp : item.city }}</span
                ><b
                  class="mono"
                  :class="`latency-text-${latencyBand(item.rtt)}`"
                  >{{ item.rtt >= 0 ? `${item.rtt} ms` : "超时" }}</b
                >
              </div>
            </div>
          </article></TransitionGroup
        >
        <EmptyState
          v-if="!rankGroups.length"
          size="section"
          :title="loading ? '等待省份返回' : '暂无检测结果'"
          description="地图与排行会随检测结果逐省更新。"
          ><template #action
            ><UiButton
              v-if="!loading"
              :disabled="!selectedNodeId"
              @click="startTest"
              >开始测试</UiButton
            ></template
          ></EmptyState
        >
      </aside>
    </div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessage } from "element-plus";
import { Close, VideoPlay } from "@element-plus/icons-vue";
import { PageHeader, UiButton, UiSegmented, StatusDot, EmptyState } from "@/ui";
import { getNodeOverview } from "@/api/subcription/node";
import { getTcp, saveTcp } from "@/utils/nodeTestStorage";
import { formatDateTime } from "@/utils/date";
import { usePrefersReducedMotion } from "@/motion";
import { useResultMotion } from "@/components/node-tests/useNodeTestSession";
import {
  latencyBand,
  latencyLegend,
  type ChinaTarget,
} from "@/components/node-tests/latency";
import ChinaTcpMap from "@/components/node-tests/ChinaTcpMap.vue";
import { useTestStream } from "./useTestStream";
defineOptions({ name: "TcpTest" });
type NodeItem = { id: number; name: string; valid: boolean };
const nodes = ref<NodeItem[]>([]),
  nodesLoading = ref(true),
  selectedNodeId = ref<number>();
const targets = ref<ChinaTarget[]>([]),
  zstaticResults = ref<{ port: number; rtt: number }[]>([]);
const filterProvinces = ref<string[]>([]),
  filterIsps = ref<string[]>([]),
  zstaticPort = ref("443");
const ranking = ref("province"),
  error = ref(""),
  status = ref("等待开始"),
  cachedAt = ref(0);
const { loading, run, cancel } = useTestStream();
const reduced = usePrefersReducedMotion();
const { enter, leave } = useResultMotion();
let disposed = false;
let runVersion = 0;
const provinceList = [
  "北京",
  "天津",
  "上海",
  "重庆",
  "河北",
  "山西",
  "辽宁",
  "吉林",
  "黑龙江",
  "江苏",
  "浙江",
  "安徽",
  "福建",
  "江西",
  "山东",
  "河南",
  "湖北",
  "湖南",
  "广东",
  "海南",
  "四川",
  "贵州",
  "云南",
  "陕西",
  "甘肃",
  "青海",
  "内蒙古",
  "广西",
  "西藏",
  "宁夏",
  "新疆",
];
const ispList = ["电信", "联通", "移动"];
const filteredTargets = computed(() =>
  targets.value.filter(
    (item) =>
      (!filterProvinces.value.length ||
        filterProvinces.value.includes(item.province)) &&
      (!filterIsps.value.length || filterIsps.value.includes(item.isp))
  )
);
const reachable = computed(
  () => filteredTargets.value.filter((item) => item.rtt >= 0).length
);
function aggregate(key: "province" | "isp") {
  const groups = new Map<string, ChinaTarget[]>();
  filteredTargets.value.forEach((item) => {
    const items = groups.get(item[key]) || [];
    items.push(item);
    groups.set(item[key], items);
  });
  return [...groups].map(([name, items]) => {
    const ok = items.filter((item) => item.rtt >= 0);
    return {
      name,
      province: name,
      items,
      okCount: ok.length,
      avg: ok.length
        ? Math.round(ok.reduce((sum, item) => sum + item.rtt, 0) / ok.length)
        : -1,
    };
  });
}
const provinceGroups = computed(() => aggregate("province"));
const rankGroups = computed(() =>
  (ranking.value === "province" ? provinceGroups.value : aggregate("isp"))
    .slice()
    .sort(
      (a, b) =>
        (a.avg < 0 ? Infinity : a.avg) - (b.avg < 0 ? Infinity : b.avg) ||
        a.name.localeCompare(b.name, "zh")
    )
);
function persist(id: number) {
  if (!targets.value.length && !zstaticResults.value.length) return;
  cachedAt.value = Date.now();
  saveTcp(id, {
    targets: targets.value,
    zstatic: zstaticResults.value,
    zstaticPort: zstaticPort.value,
    provinces: filterProvinces.value,
    isps: filterIsps.value,
    updatedAt: cachedAt.value,
    status: status.value,
  });
}
function restoreCache() {
  const cached = selectedNodeId.value ? getTcp(selectedNodeId.value) : null;
  targets.value = cached?.targets || [];
  zstaticResults.value = cached?.zstatic || [];
  cachedAt.value = cached?.updatedAt || 0;
  status.value = cached
    ? cached.status === "检测中"
      ? "上次检测未完成"
      : cached.status || "已恢复缓存"
    : "等待开始";
  error.value = "";
  if (cached?.zstaticPort) zstaticPort.value = cached.zstaticPort;
  filterProvinces.value = cached?.provinces || [];
  filterIsps.value = cached?.isps || [];
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
    if (!disposed) error.value = "节点读取失败，请重试。";
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
  status.value = "检测中";
  cachedAt.value = 0;
  const payload = new URLSearchParams({
    id: String(id),
    zstatic_port: zstaticPort.value,
  });
  if (filterProvinces.value.length)
    payload.append("provinces", filterProvinces.value.join(","));
  if (filterIsps.value.length)
    payload.append("isps", filterIsps.value.join(","));
  const ownVersion = ++runVersion;
  let completed = false;
  try {
    await run("/api/v1/nodes/chinaping/stream", payload, (type, data) => {
      if (type === "province" && Array.isArray(data.targets)) {
        for (const item of data.targets) {
          const index = targets.value.findIndex(
            (old) => old.ip === item.ip && old.port === item.port
          );
          if (index >= 0) targets.value[index] = item;
          else targets.value.push(item);
        }
        persist(id);
      }
      if (type === "zstatic") {
        zstaticResults.value = data.zstatic || [];
        persist(id);
      }
      if (type === "done") {
        completed = true;
        status.value = "检测完成";
        persist(id);
        ElMessage.success("TCP 检测完成");
      }
    });
    if (
      !disposed &&
      ownVersion === runVersion &&
      !completed &&
      status.value !== "已停止"
    ) {
      status.value = "连接中断";
      error.value = "检测结果流提前结束，请重试。";
    }
  } catch (cause: any) {
    if (disposed || ownVersion !== runVersion) return;
    error.value = cause.message || "检测失败";
    status.value = "检测失败";
    ElMessage.error(error.value);
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
  if (status.value === "检测中" && selectedNodeId.value) {
    status.value = "已停止";
    persist(selectedNodeId.value);
  }
  disposed = true;
});
</script>
<style lang="scss" scoped>
@use "./test-page.scss";
.tcp-controls {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 1.4fr) minmax(0, 1fr) minmax(
      0,
      1fr
    );
  gap: 16px;
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.tcp-controls > div {
  min-width: 0;
}
.tcp-meta {
  display: flex;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 24px;
  color: var(--c-muted);
  font-size: 14px;
}
.tcp-stage {
  display: grid;
  grid-template-columns: minmax(0, 7fr) minmax(0, 5fr);
  gap: 24px;
}
.tcp-map-section,
.tcp-rankings {
  min-width: 0;
}
.muted {
  color: var(--c-muted);
  font-size: 14px;
}
.tcp-map-wrap {
  position: relative;
  overflow: hidden;
  border-radius: 16px;
}
.tcp-map-wrap :deep(.china-map) {
  height: 520px;
}
.tcp-map-wrap.is-running::after {
  content: "";
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 25%;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    transparent,
    color-mix(in srgb, var(--c-data) 8%, transparent),
    transparent
  );
  animation: tcp-scan 2.4s linear infinite;
}
.latency-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 16px;
  color: var(--c-muted);
  font: 400 14px var(--font-mono);
}
.latency-legend > span {
  display: flex;
  align-items: center;
  gap: 8px;
}
.latency-legend i {
  display: inline-block;
  width: 16px;
  height: 2px;
}
.latency-band-0 {
  background: var(--c-ok);
}
.latency-band-1 {
  background: var(--c-data);
}
.latency-band-2 {
  background: var(--c-warn);
}
.latency-band-3 {
  background: var(--c-signal);
}
.latency-band-4 {
  background: var(--c-bad);
}
.latency-band-5 {
  background: var(--c-muted);
}
.latency-text-0 {
  color: var(--c-ok);
}
.latency-text-1 {
  color: var(--c-data);
}
.latency-text-2 {
  color: var(--c-warn-ink);
}
.latency-text-3 {
  color: var(--c-signal-text);
}
.latency-text-4 {
  color: var(--c-bad);
}
.latency-text-5 {
  color: var(--c-muted);
}
.ranking-list {
  position: relative;
  max-height: 680px;
  overflow: auto;
  padding-right: 8px;
}
.ranking-row {
  padding: 16px 0;
  border-bottom: 1px solid var(--c-line);
}
.ranking-row:first-child {
  padding-top: 0;
}
.ranking-summary {
  display: flex;
  align-items: center;
  gap: 12px;
}
.rank-index {
  color: var(--c-muted);
  font-size: 14px;
}
.ranking-summary strong {
  font-size: 15px;
  font-weight: 500;
}
.rank-rtt {
  margin-left: auto;
  font-size: 15px;
}
.ranking-row p {
  color: var(--c-muted);
  font-size: 14px;
  margin: 8px 0 12px 34px;
}
.target-cells {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px 12px;
}
.target-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 0;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.target-cell > span {
  color: var(--c-muted);
  font-size: 14px;
}
.target-cell b {
  font-weight: 400;
  transition: color var(--duration-medium);
}
.zstatic-results {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
  font-size: 14px;
}
.zstatic-results h3 {
  font-size: 14px;
  margin: 0;
}
.tcp-result-move {
  transition: transform var(--duration-slow) var(--ease-spring);
}
.tcp-result-leave-active {
  position: absolute;
  width: 100%;
}
@keyframes tcp-scan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(500%);
  }
}
@media (max-width: 1023px) {
  .tcp-controls {
    grid-template-columns: 1fr 1fr;
  }
  .tcp-stage {
    gap: 16px;
  }
}
@media (max-width: 767px) {
  .tcp-controls {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .tcp-stage {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .tcp-map-wrap :deep(.china-map) {
    height: 330px;
  }
  .tcp-meta {
    margin-bottom: 20px;
  }
  .ranking-list {
    max-height: none;
    padding: 0;
  }
}
@media (prefers-reduced-motion: reduce) {
  .tcp-map-wrap.is-running::after {
    display: none;
  }
  .tcp-result-move {
    transition: none;
  }
}
:global(html[data-motion="reduced"]) .tcp-map-wrap.is-running::after {
  display: none;
}
:global(html[data-motion="reduced"]) .tcp-result-move {
  transition: none;
}
</style>
