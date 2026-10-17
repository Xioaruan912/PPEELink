<template>
  <div class="network-overview" :aria-busy="loading">
    <header class="overview-heading">
      <div>
        <h1>我的网络</h1>
      </div>
      <button class="outline-button" :disabled="loading" @click="loadOverview">
        <WorkIcon name="refresh" :class="{ 'is-refreshing': loading }" />{{
          loading ? "正在同步" : "刷新"
        }}
      </button>
    </header>
    <div v-if="loadError" class="overview-notice" role="alert">
      <span>{{ loadError }}</span
      ><button @click="loadOverview">重新加载</button>
    </div>
    <section class="atlas-studio">
      <div class="atlas-metrics">
        <div class="primary-metric">
          <span>节点总数</span
          ><strong
            class="data-number"
            :class="{ 'metric-skeleton': loading }"
            >{{ loading ? "—" : summary.total }}</strong
          >
        </div>
        <div class="secondary-metrics">
          <router-link to="/subcription/subs"
            ><span>订阅</span
            ><strong class="data-number">{{ loading ? "—" : subTotal }}</strong
            ><WorkIcon name="diagonal" /></router-link
          ><router-link to="/subcription/nodes"
            ><span>连接评分</span
            ><strong class="data-number"
              >{{
                loading || !summary.total
                  ? "—"
                  : Math.round(summary.averageScore)
              }}<small> / 100</small></strong
            ><WorkIcon name="diagonal"
          /></router-link>
        </div>
        <router-link
          v-magnetic
          class="atlas-primary-action"
          to="/subcription/nodes"
          ><span>管理节点</span><WorkIcon name="arrow"
        /></router-link>
      </div>
      <div class="atlas-visual">
        <div class="atlas-coordinate">
          <span>全球分布</span>
        </div>
        <NetworkGlobe :points="mapPoints" contrast />
      </div>
      <aside class="atlas-quality">
        <div class="quality-heading">
          <span>连接正常率</span>
        </div>
        <div class="quality-gauge">
          <svg viewBox="0 0 180 180" aria-hidden="true">
            <circle class="gauge-track" cx="90" cy="90" r="73" />
            <circle
              class="gauge-fill"
              cx="90"
              cy="90"
              r="73"
              :style="{ strokeDasharray: `${survivalRate * 4.587} 458.7` }"
            />
          </svg>
          <div>
            <strong class="data-number"
              >{{ summary.total ? survivalRate : "—"
              }}<small v-if="summary.total">%</small></strong
            >
          </div>
        </div>
        <div class="quality-legend">
          <span
            ><i class="legend-healthy"></i>正常<strong class="data-number">{{
              summary.healthy
            }}</strong></span
          ><span
            ><i class="legend-warning"></i>待检查<strong class="data-number">{{
              summary.warning
            }}</strong></span
          ><span
            ><i class="legend-offline"></i>不可用<strong class="data-number">{{
              summary.offline
            }}</strong></span
          >
        </div>
        <button class="quality-settings" @click="openAlerts">
          <WorkIcon name="settings" />异常提醒设置<WorkIcon name="chevron" />
        </button>
      </aside>
    </section>
    <div class="overview-lower">
      <section class="routing-section">
        <header class="overview-section-head">
          <div>
            <h2>推荐节点</h2>
          </div>
          <router-link to="/test/speed"
            >测试速度<WorkIcon name="diagonal"
          /></router-link>
        </header>
        <div
          ref="sceneTabs"
          class="routing-tabs"
          role="tablist"
          aria-label="推荐用途"
        >
          <button
            v-for="scene in sceneOptions"
            :key="scene.key"
            role="tab"
            :aria-selected="activeScene === scene.key"
            :tabindex="activeScene === scene.key ? 0 : -1"
            :class="{ active: activeScene === scene.key }"
            @click="activeScene = scene.key"
            @keydown.right.prevent="changeScene(1)"
            @keydown.left.prevent="changeScene(-1)"
          >
            {{ scene.name
            }}<span v-if="scene.nodes?.length">{{ scene.nodes.length }}</span>
          </button>
        </div>
        <div class="routing-table">
          <div class="routing-table-head">
            <span>推荐节点</span><span>推荐原因</span><span>评分</span>
          </div>
          <div v-if="loading" class="routing-skeleton">
            <div v-for="i in 3" :key="i"><i></i><i></i><i></i></div>
          </div>
          <template v-else-if="activeNodes.length"
            ><router-link
              v-for="(node, i) in activeNodes.slice(0, 5)"
              :key="node.nodeId"
              to="/subcription/nodes"
              class="routing-row"
              :style="{ '--row': i }"
              ><span
                ><i class="node-connection-icon"><WorkIcon name="network" /></i
                ><strong>{{ node.name }}</strong></span
              ><span class="routing-reasons">{{
                simpleReason(node.reasons || [])
              }}</span
              ><span class="routing-score data-number"
                >{{ node.score }}<small>分</small
                ><WorkIcon name="diagonal" /></span></router-link
          ></template>
          <div v-else class="routing-empty">
            <span class="empty-orbit"><WorkIcon name="network" /></span>
            <div>
              <strong>还没有推荐节点</strong>
              <p>添加节点并测速后，这里会显示推荐结果。</p>
              <router-link to="/subcription/nodes"
                >添加节点<WorkIcon name="plus"
              /></router-link>
            </div>
          </div>
        </div>
        <div class="routing-note">
          <button @click="diagnosticsVisible = !diagnosticsVisible">
            {{ diagnosticsVisible ? "收起响应时间" : "查看响应时间"
            }}<WorkIcon name="down" />
          </button>
        </div>
        <NodePing v-if="diagnosticsVisible" />
      </section>
      <aside class="region-section">
        <header class="overview-section-head">
          <div>
            <h2>所在地区</h2>
          </div>
          <WorkIcon name="globe" />
        </header>
        <div v-if="countries.length" class="region-list">
          <div
            v-for="country in countries.slice(0, 5)"
            :key="country.name"
            class="region-item"
          >
            <div>
              <span>{{ country.name || "未识别地区" }}</span
              ><strong class="data-number">{{ country.count }}</strong>
            </div>
            <div class="region-bar">
              <span
                :style="{
                  transform: `scaleX(${country.count / Math.max(mapPoints.length, 1)})`,
                }"
              ></span>
            </div>
          </div>
        </div>
        <div v-else class="region-empty">
          <div class="region-orbits" aria-hidden="true">
            <span></span><span></span><span></span><i></i>
          </div>
          <strong>暂无地区数据</strong>
        </div>
        <router-link to="/subcription/airport" class="region-source-link"
          ><WorkIcon name="plus" />添加机场订阅<WorkIcon name="diagonal"
        /></router-link>
      </aside>
    </div>
    <section class="activity-section">
      <header class="overview-section-head">
        <div>
          <h2>最近变化</h2>
        </div>
        <button class="text-button" @click="openAlerts">
          <WorkIcon name="settings" />异常提醒
        </button>
      </header>
      <div v-if="events.length" class="activity-list">
        <div
          v-for="event in events.slice(0, 4)"
          :key="event.id"
          class="activity-item"
        >
          <i :class="event.type"></i><strong>{{ event.nodeName }}</strong
          ><span>{{ event.message }}</span
          ><time class="data-number">{{ formatTime(event.createdAt) }}</time>
        </div>
      </div>
      <div v-else class="activity-empty">
        <span class="activity-line"><i></i><i></i><i></i></span
        ><span>{{ loading ? "正在加载…" : "暂无记录" }}</span>
      </div>
    </section>
    <el-dialog v-model="alertVisible" title="异常提醒设置" width="540px"
      ><el-form label-position="top"
        ><el-form-item
          ><el-switch
            v-model="alertForm.enabled"
            active-text="开启异常提醒" /></el-form-item
        ><el-form-item label="通知接收地址"
          ><el-input
            v-model="alertForm.webhookUrl"
            placeholder="https://…" /></el-form-item
        ><el-form-item label="连续连接失败次数"
          ><el-input-number
            v-model="alertForm.failureThreshold"
            :min="1"
            :max="20" /></el-form-item
        ><el-row :gutter="12"
          ><el-col :span="12"
            ><el-form-item label="免打扰开始"
              ><el-time-select
                v-model="alertForm.maintenanceStart"
                start="00:00"
                step="00:30"
                end="23:30"
                clearable /></el-form-item></el-col
          ><el-col :span="12"
            ><el-form-item label="免打扰结束"
              ><el-time-select
                v-model="alertForm.maintenanceEnd"
                start="00:00"
                step="00:30"
                end="23:30"
                clearable /></el-form-item></el-col></el-row
        ><el-alert
          title="维护期间继续记录探测样本，暂停发送 Webhook 告警。"
          type="info"
          :closable="false" /></el-form
      ><template #footer
        ><el-button @click="alertVisible = false">取消</el-button
        ><el-button type="primary" :loading="savingAlert" @click="saveAlerts"
          >保存设置</el-button
        ></template
      ></el-dialog
    >
  </div>
</template>
<script setup lang="ts">
import { computed, ref, onMounted, defineAsyncComponent, nextTick } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import {
  getNodeQualitySummary,
  getNodeRecommendations,
  getNodeHealthEvents,
  getAlertSetting,
  updateAlertSetting,
} from "@/api/subcription/node";
import { ElMessage } from "element-plus";
import WorkIcon from "@/components/workspace/WorkIcon.vue";
import NetworkGlobe from "@/components/workspace/NetworkGlobe.vue";
import { vMagnetic } from "@/utils/magnetic";
const NodePing = defineAsyncComponent(
  () => import("./components/NodePing.vue")
);
defineOptions({ name: "Dashboard", inheritAttrs: false });
interface Recommendation {
  key: string;
  name: string;
  nodes: { nodeId: number; name: string; reasons: string[]; score: number }[];
}
const loading = ref(true),
  loadError = ref(""),
  subTotal = ref(0),
  diagnosticsVisible = ref(false);
const summary = ref({
  total: 0,
  healthy: 0,
  warning: 0,
  offline: 0,
  averageScore: 0,
});
const recommendations = ref<Recommendation[]>([]),
  activeScene = ref("");
const mapPoints = ref<
  { lat: number; lng: number; name: string; country: string }[]
>([]);
const events = ref<
  {
    id: number;
    nodeName: string;
    type: string;
    message: string;
    createdAt: string;
  }[]
>([]);
const aliveNodes = computed(() =>
  Math.max(summary.value.total - summary.value.offline, 0)
);
const survivalRate = computed(() =>
  summary.value.total
    ? Math.round((aliveNodes.value / summary.value.total) * 1000) / 10
    : 0
);
const countries = computed(() => {
  const groups = new Map<string, number>();
  mapPoints.value.forEach((p) => {
    const name = p.country || "未识别地区";
    groups.set(name, (groups.get(name) || 0) + 1);
  });
  return [...groups]
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count);
});
const sceneOptions = computed<Recommendation[]>(() =>
  recommendations.value.length
    ? recommendations.value
    : [
        { key: "general", name: "综合推荐", nodes: [] },
        { key: "streaming", name: "流媒体", nodes: [] },
        { key: "latency", name: "低延迟", nodes: [] },
      ]
);
const activeNodes = computed(
  () => sceneOptions.value.find((s) => s.key === activeScene.value)?.nodes || []
);
const sceneTabs = ref<HTMLElement>();
function changeScene(direction: number) {
  const options = sceneOptions.value;
  const current = options.findIndex((scene) => scene.key === activeScene.value);
  activeScene.value =
    options[(current + direction + options.length) % options.length].key;
  nextTick(() =>
    sceneTabs.value
      ?.querySelector<HTMLButtonElement>('[aria-selected="true"]')
      ?.focus()
  );
}
async function loadOverview() {
  loading.value = true;
  loadError.value = "";
  const results = await Promise.allSettled([
    getSubTotal(),
    getNodeQualitySummary(),
    getNodeRecommendations(),
    getNodeHealthEvents(12),
    getNodeMap(),
  ]);
  const [subs, quality, scenes, healthEvents, map] = results;
  if (subs.status === "fulfilled")
    subTotal.value = Number(subs.value.data) || 0;
  if (quality.status === "fulfilled")
    summary.value = { ...summary.value, ...quality.value.data };
  if (scenes.status === "fulfilled") {
    recommendations.value = scenes.value.data || [];
    if (!recommendations.value.some((s) => s.key === activeScene.value))
      activeScene.value = sceneOptions.value[0].key;
  }
  if (healthEvents.status === "fulfilled")
    events.value = healthEvents.value.data || [];
  if (map.status === "fulfilled")
    mapPoints.value = Array.isArray(map.value.data) ? map.value.data : [];
  if (results.some((r) => r.status === "rejected"))
    loadError.value = "部分网络数据暂时无法加载，请检查服务状态后重试。";
  loading.value = false;
}
const alertVisible = ref(false),
  savingAlert = ref(false),
  alertForm = ref({
    enabled: false,
    webhookUrl: "",
    failureThreshold: 3,
    maintenanceStart: "",
    maintenanceEnd: "",
  });
async function openAlerts() {
  try {
    const { data } = await getAlertSetting();
    alertForm.value = { ...alertForm.value, ...data };
    alertVisible.value = true;
  } catch {
    ElMessage.error("无法读取告警设置，请重试");
  }
}
async function saveAlerts() {
  savingAlert.value = true;
  try {
    await updateAlertSetting(alertForm.value);
    ElMessage.success("告警设置已保存");
    alertVisible.value = false;
  } catch {
    ElMessage.error("告警设置未保存，请重试");
  } finally {
    savingAlert.value = false;
  }
}
function simpleReason(reasons: string[]) {
  const primary = reasons.find((reason) => !/P95|抖动/.test(reason)) || "";
  return primary
    .replace("24h 可用率", "近一天连接成功率")
    .replace("平均延迟", "平均响应")
    .replace("场景可达率", "访问成功率")
    .replace("场景平均", "平均响应")
    .replace("已解锁", "可访问")
    .replace("尚无解锁样本", "还未检测服务");
}
function formatTime(value: string) {
  return new Date(value).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}
onMounted(() => {
  activeScene.value = sceneOptions.value[0].key;
  loadOverview();
});
</script>
<style scoped>
.network-overview {
  padding-top: 34px;
}
.overview-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-bottom: 30px;
}
.overview-date {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
  color: var(--ui-text-muted);
}
.tiny-cross {
  font-size: 15px;
  line-height: 1;
}
.overview-separator {
  height: 9px;
  width: 1px;
  background: var(--ui-border-strong);
}
.overview-heading h1 {
  font-size: 36px;
  font-weight: 600;
  letter-spacing: -1.7px;
  line-height: 1.25;
  margin: 13px 0 10px;
}
.overview-heading p {
  font-size: 16px;
  color: var(--ui-text-secondary);
  margin: 0;
}
.outline-button {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 9px 13px;
  border: 1px solid var(--ui-border-strong);
  background: transparent;
  border-radius: 6px;
  font-size: 15px;
}
.outline-button:hover {
  background: var(--ui-surface);
}
.outline-button svg {
  width: 13px;
  height: 13px;
}
.overview-notice {
  display: flex;
  justify-content: space-between;
  padding: 12px;
  background: var(--ui-warning-soft);
  color: var(--ui-warning);
  margin-bottom: 16px;
  border-radius: 6px;
  font-size: 16px;
}
.overview-notice button {
  background: transparent;
  text-decoration: underline;
}
.atlas-studio {
  display: grid;
  grid-template-columns: 235px minmax(0, 1fr) 220px;
  min-height: 350px;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border);
  border-radius: 10px;
  overflow: hidden;
  animation: atlas-enter 0.9s var(--ease-workspace);
}
.atlas-metrics {
  padding: 24px;
  display: flex;
  flex-direction: column;
  position: relative;
  z-index: 1;
}
.atlas-section-name {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 15px;
  color: var(--ui-text-secondary);
}
.atlas-section-name svg {
  height: 14px;
  width: 14px;
}
.primary-metric {
  margin-top: 24px;
}
.primary-metric > span {
  font-size: 15px;
  color: var(--ui-text-muted);
  display: block;
}
.primary-metric > strong {
  font-size: 64px;
  line-height: 1.35;
  font-weight: 400;
  display: block;
  color: var(--ui-text);
}
.primary-metric > small {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ui-text-secondary);
}
.secondary-metrics {
  margin-top: 22px;
  display: flex;
  gap: 24px;
}
.secondary-metrics > a {
  position: relative;
  flex: 1;
}
.secondary-metrics > a > span {
  display: block;
  font-size: 14px;
  color: var(--ui-text-muted);
  margin-bottom: 7px;
}
.secondary-metrics strong {
  font-size: 23px;
  line-height: 1.2;
  font-weight: 400;
}
.secondary-metrics strong small {
  font-family: var(--el-font-family);
  font-size: 14px;
  color: var(--ui-text-muted);
  display: block;
  letter-spacing: 0;
  margin-top: 4px;
}
.secondary-metrics > a > svg {
  position: absolute;
  right: 0;
  top: 0;
  width: 10px;
  height: 10px;
  color: var(--ui-text-muted);
}
.atlas-primary-action {
  margin-top: 25px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 15px;
  background: var(--ui-accent);
  color: var(--ui-surface);
  border-radius: 5px;
  padding: 11px 12px;
  transition:
    background 0.2s,
    transform 0.3s var(--ease-spring);
}
.atlas-primary-action:hover {
  background: var(--ui-accent-strong);
  color: var(--ui-surface);
  transform: translateY(-2px);
}
.atlas-primary-action svg {
  height: 15px;
  width: 15px;
}
.atlas-visual {
  position: relative;
  background: var(--ui-surface-strong);
  min-width: 0;
  min-height: 350px;
  overflow: hidden;
}
.atlas-coordinate {
  position: absolute;
  left: 20px;
  right: 20px;
  top: 17px;
  display: flex;
  justify-content: space-between;
  font-size: 14px;
  color: var(--ui-text-muted);
  z-index: 1;
  pointer-events: none;
}
.atlas-watermark {
  position: absolute;
  font-size: 37px;
  line-height: 1;
  letter-spacing: -2px;
  font-weight: 500;
  color: var(--ui-text-secondary);
  opacity: 0.08;
  bottom: 62px;
  left: 22px;
  pointer-events: none;
  white-space: nowrap;
}
.atlas-quality {
  padding: 25px 22px 18px;
  display: flex;
  flex-direction: column;
}
.quality-heading {
  display: flex;
  justify-content: space-between;
  font-size: 15px;
}
.quality-period {
  font-size: 14px;
  color: var(--ui-text-muted);
}
.quality-gauge {
  position: relative;
  width: 160px;
  height: 160px;
  margin: 18px auto 12px;
}
.quality-gauge > svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.gauge-track,
.gauge-fill {
  fill: none;
  stroke-width: 5;
  stroke: var(--ui-border-subtle);
}
.gauge-fill {
  stroke: var(--ui-accent);
  stroke-linecap: round;
  transition: stroke-dasharray 1.4s var(--ease-workspace);
}
.quality-gauge > div {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 5px;
}
.quality-gauge strong {
  font-weight: 400;
  font-size: 35px;
  line-height: 1.2;
}
.quality-gauge strong small {
  font-size: 15px;
  margin-left: 3px;
}
.quality-gauge > div > span {
  font-size: 14px;
  color: var(--ui-text-muted);
}
.quality-legend {
  display: flex;
  flex-direction: column;
  gap: 11px;
}
.quality-legend > span {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  color: var(--ui-text-secondary);
}
.quality-legend i {
  height: 5px;
  width: 5px;
  border-radius: 1px;
}
.legend-healthy {
  background: var(--ui-success);
}
.legend-warning {
  background: var(--ui-warning);
}
.legend-offline {
  background: var(--ui-danger);
}
.quality-legend strong {
  margin-left: auto;
  font-size: 16px;
  font-weight: 400;
}
.quality-settings {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  color: var(--ui-text-muted);
  padding: 12px 0 0;
  margin-top: 15px;
  background: none;
  border-top: 1px solid var(--ui-border-subtle);
}
.quality-settings svg {
  width: 12px;
  height: 12px;
}
.quality-settings svg:last-child {
  margin-left: auto;
  width: 10px;
}
.overview-lower {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 290px;
  gap: 32px;
  margin-top: 32px;
}
.overview-section-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 19px;
  gap: 12px;
}
.overview-section-head h2 {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.5px;
  margin: 0 0 6px;
}
.overview-section-head p {
  font-size: 14px;
  color: var(--ui-text-muted);
  margin: 0;
}
.overview-section-head > a {
  display: flex;
  align-items: center;
  gap: 7px;
  white-space: nowrap;
  font-size: 14px;
  color: var(--ui-text-secondary);
}
.overview-section-head > a > svg,
.overview-section-head > svg {
  height: 14px;
  width: 14px;
}
.routing-tabs {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 0 0 13px;
}
.routing-tabs button {
  background: none;
  padding: 6px 13px;
  font-size: 15px;
  color: var(--ui-text-secondary);
  border-radius: 5px;
  position: relative;
}
.routing-tabs button.active {
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
  font-weight: 600;
}
.routing-tabs button:hover {
  background: var(--ui-hover);
}
.routing-tabs button span {
  margin-left: 9px;
  font-family: var(--font-data);
  font-size: 14px;
}
.routing-table {
  border-block: 1px solid var(--ui-border);
  min-height: 210px;
}
.routing-table-head,
.routing-row {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr) 85px;
  gap: 15px;
  align-items: center;
}
.routing-table-head {
  font-size: 14px;
  color: var(--ui-text-muted);
  padding: 11px 4px;
  border-bottom: 1px solid var(--ui-border-subtle);
}
.routing-table-head > span:last-child {
  text-align: right;
}
.routing-row {
  padding: 14px 4px;
  border-bottom: 1px solid var(--ui-border-subtle);
  font-size: 15px;
  animation: row-enter 0.5s var(--ease-workspace) both;
  animation-delay: calc(var(--row) * 60ms);
}
.routing-row:last-child {
  border-bottom: 0;
}
.routing-row:hover {
  background: var(--ui-hover);
}
.routing-row > span:first-child {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.routing-row strong {
  font-size: 15px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-connection-icon {
  display: grid;
  place-items: center;
  height: 25px;
  width: 25px;
  flex-shrink: 0;
  background: var(--ui-accent-soft);
  color: var(--ui-accent);
  border-radius: 5px;
}
.node-connection-icon svg {
  height: 13px;
  width: 13px;
}
.routing-reasons {
  font-size: 14px;
  color: var(--ui-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.routing-score {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  font-size: 17px;
  color: var(--ui-accent);
}
.routing-score small {
  font-size: 14px;
  font-family: var(--el-font-family);
  margin-left: 2px;
}
.routing-score svg {
  margin-left: 12px;
  width: 11px;
  height: 11px;
}
.routing-empty {
  padding: 35px 8px;
  display: flex;
  align-items: center;
  gap: 18px;
  min-height: 173px;
}
.empty-orbit {
  display: grid;
  place-items: center;
  height: 65px;
  width: 65px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 50%;
  position: relative;
  flex-shrink: 0;
}
.empty-orbit:before {
  content: "";
  position: absolute;
  inset: 7px;
  border: 1px dashed var(--ui-border);
  border-radius: 50%;
  animation: orbit-spin 35s linear infinite;
}
.empty-orbit svg {
  width: 23px;
  height: 23px;
  color: var(--ui-text-secondary);
}
.routing-empty strong {
  font-size: 16px;
  font-weight: 500;
}
.routing-empty p {
  font-size: 14px;
  color: var(--ui-text-muted);
  margin: 8px 0 13px;
  line-height: 1.7;
}
.routing-empty a {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  font-weight: 600;
  color: var(--ui-accent);
}
.routing-empty a svg {
  height: 11px;
  width: 11px;
}
.routing-note {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ui-text-muted);
  margin-top: 12px;
}
.routing-note > svg {
  width: 11px;
  height: 11px;
  flex-shrink: 0;
}
.routing-note button {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-left: auto;
  white-space: nowrap;
  background: none;
  color: var(--ui-text-secondary);
  font-size: 14px;
}
.routing-note button svg {
  height: 11px;
  width: 11px;
}
.region-section {
  padding-left: 26px;
  border-left: 1px solid var(--ui-border);
}
.region-list {
  padding-top: 8px;
  min-height: 210px;
}
.region-item {
  margin-bottom: 20px;
}
.region-item > div:first-child {
  display: flex;
  justify-content: space-between;
  font-size: 15px;
}
.region-item strong {
  font-size: 15px;
  font-weight: 400;
}
.region-bar {
  height: 3px;
  background: var(--ui-border-subtle);
  margin-top: 8px;
  border-radius: 3px;
  overflow: hidden;
}
.region-bar > span {
  display: block;
  width: 100%;
  height: 100%;
  background: var(--ui-accent);
  transform-origin: left;
  transition: transform 1s var(--ease-workspace);
}
.region-empty {
  min-height: 210px;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
}
.region-empty strong {
  font-size: 15px;
  font-weight: 500;
}
.region-empty p {
  font-size: 14px;
  color: var(--ui-text-muted);
  margin: 7px 0;
}
.region-orbits {
  position: relative;
  height: 85px;
  width: 110px;
}
.region-orbits span {
  position: absolute;
  inset: 6px 22px 16px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 50%;
}
.region-orbits span:nth-child(2) {
  transform: rotate(60deg) scaleX(0.35);
}
.region-orbits span:nth-child(3) {
  transform: rotate(-60deg) scaleX(0.35);
}
.region-orbits i {
  position: absolute;
  left: 40px;
  top: 12px;
  width: 5px;
  height: 5px;
  background: var(--ui-accent);
  border-radius: 50%;
  animation: region-drift 4s ease-in-out infinite alternate;
}
.region-source-link {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  border-top: 1px solid var(--ui-border);
  padding-top: 13px;
}
.region-source-link svg {
  height: 12px;
  width: 12px;
}
.region-source-link svg:last-child {
  margin-left: auto;
}
.activity-section {
  margin-top: 34px;
  padding-top: 26px;
  border-top: 1px solid var(--ui-border);
}
.text-button {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  color: var(--ui-text-secondary);
  background: none;
}
.text-button svg {
  height: 13px;
  width: 13px;
}
.activity-item {
  display: flex;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid var(--ui-border-subtle);
  padding: 12px 0;
  font-size: 15px;
}
.activity-item > i {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--ui-warning);
  flex-shrink: 0;
}
.activity-item > i.recovered {
  background: var(--ui-success);
}
.activity-item strong {
  font-size: 15px;
  font-weight: 500;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.activity-item > span {
  font-size: 14px;
  color: var(--ui-text-muted);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.activity-item time {
  margin-left: auto;
  white-space: nowrap;
  font-size: 14px;
  color: var(--ui-text-muted);
}
.activity-empty {
  display: flex;
  align-items: center;
  gap: 16px;
  background: var(--ui-surface-strong);
  padding: 17px 20px;
  border-radius: 6px;
  font-size: 14px;
  color: var(--ui-text-secondary);
}
.activity-line {
  display: flex;
  gap: 3px;
  align-items: center;
}
.activity-line i {
  display: block;
  width: 3px;
  height: 10px;
  background: var(--ui-accent);
  opacity: 0.45;
  border-radius: 2px;
  animation: signal-wave 1.8s ease-in-out infinite alternate;
}
.activity-line i:nth-child(2) {
  height: 17px;
  animation-delay: 0.3s;
}
.activity-line i:nth-child(3) {
  height: 8px;
  animation-delay: 0.6s;
}
.activity-empty-tag {
  margin-left: auto;
  color: var(--ui-text-muted);
  font-size: 14px;
  white-space: nowrap;
}
.workflow-strip {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  margin-top: 28px;
  border-top: 1px solid var(--ui-border);
  border-bottom: 1px solid var(--ui-border);
}
.workflow-strip > a {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 21px 22px 21px 0;
}
.workflow-strip > a:not(:first-child) {
  padding-left: 25px;
  border-left: 1px solid var(--ui-border);
}
.workflow-strip > a > svg {
  width: 19px;
  height: 19px;
  color: var(--ui-text-secondary);
  transition: transform 0.3s var(--ease-spring);
}
.workflow-strip > a:hover > svg:last-child {
  transform: translate(3px, -3px);
}
.workflow-strip > a > svg:last-child {
  margin-left: auto;
  width: 14px;
  height: 14px;
}
.workflow-strip strong {
  display: block;
  font-size: 15px;
  font-weight: 550;
}
.workflow-strip span {
  display: block;
  font-size: 14px;
  color: var(--ui-text-muted);
  margin-top: 5px;
}
.routing-skeleton > div {
  display: grid;
  grid-template-columns: 1.2fr 1fr 0.3fr;
  gap: 20px;
  padding: 19px 5px;
}
.routing-skeleton i {
  height: 14px;
  background: var(--ui-border-subtle);
  animation: skeleton-wave 1.5s ease-in-out infinite alternate;
  border-radius: 3px;
}
.metric-skeleton {
  animation: skeleton-wave 1.5s ease-in-out infinite alternate;
}
@keyframes atlas-enter {
  from {
    opacity: 0;
    transform: translateY(20px) scale(0.985);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes row-enter {
  from {
    opacity: 0;
    transform: translateX(-12px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes orbit-spin {
  to {
    transform: rotate(360deg);
  }
}
@keyframes signal-wave {
  to {
    transform: scaleY(0.45);
    opacity: 0.8;
  }
}
@keyframes region-drift {
  to {
    transform: translate(30px, 38px);
  }
}
@keyframes skeleton-wave {
  to {
    opacity: 0.4;
  }
}
.is-refreshing {
  animation: orbit-spin 1s linear infinite;
}
@media (min-width: 1450px) {
  .atlas-studio {
    grid-template-columns: 260px minmax(0, 1fr) 245px;
    min-height: 390px;
  }
  .atlas-visual {
    min-height: 390px;
  }
  .atlas-metrics {
    padding: 30px;
  }
  .atlas-quality {
    padding: 30px;
  }
  .quality-gauge {
    width: 175px;
    height: 175px;
    margin-block: 22px 16px;
  }
  .primary-metric {
    margin-top: 30px;
  }
  .secondary-metrics {
    margin-top: 25px;
  }
  .atlas-primary-action {
    margin-top: auto;
  }
  .overview-lower {
    grid-template-columns: minmax(0, 1fr) 320px;
  }
}
@media (max-width: 1100px) {
  .atlas-studio {
    grid-template-columns: 200px minmax(0, 1fr) 185px;
  }
  .atlas-metrics {
    padding: 20px;
  }
  .atlas-quality {
    padding: 22px 15px 18px;
  }
  .quality-gauge {
    width: 145px;
    height: 145px;
  }
  .secondary-metrics {
    gap: 14px;
  }
  .atlas-primary-action {
    font-size: 14px;
  }
  .overview-lower {
    gap: 22px;
    grid-template-columns: minmax(0, 1fr) 240px;
  }
  .region-section {
    padding-left: 20px;
  }
  .atlas-watermark {
    font-size: 29px;
  }
  .routing-note > span {
    max-width: 210px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
@media (max-width: 850px) {
  .atlas-studio {
    grid-template-columns: 185px minmax(0, 1fr);
  }
  .atlas-quality {
    grid-column: 1/-1;
    flex-direction: row;
    align-items: center;
    gap: 22px;
    padding: 15px 20px;
    border-top: 1px solid var(--ui-border);
  }
  .quality-gauge {
    height: 65px;
    width: 65px;
    margin: 0;
    order: -1;
    flex-shrink: 0;
  }
  .quality-gauge strong {
    font-size: 18px;
  }
  .quality-gauge strong small {
    font-size: 14px;
  }
  .quality-gauge > div > span {
    display: none;
  }
  .quality-heading {
    flex-direction: column;
    gap: 6px;
    min-width: 70px;
  }
  .quality-legend {
    flex: 1;
    flex-direction: row;
    justify-content: space-around;
    gap: 15px;
  }
  .quality-legend > span {
    flex-wrap: wrap;
  }
  .quality-legend strong {
    width: 100%;
    margin: 0 0 0 12px;
  }
  .quality-settings {
    margin: 0;
    padding: 0;
    border: 0;
  }
  .quality-settings > svg:last-child {
    display: none;
  }
  .overview-lower {
    grid-template-columns: minmax(0, 1fr) 210px;
  }
  .routing-table-head,
  .routing-row {
    grid-template-columns: minmax(0, 1fr) 75px;
  }
  .routing-table-head > span:nth-child(2),
  .routing-reasons {
    display: none;
  }
  .region-section {
    padding-left: 18px;
  }
  .routing-empty {
    gap: 13px;
  }
  .empty-orbit {
    height: 48px;
    width: 48px;
  }
  .routing-empty p {
    font-size: 14px;
  }
  .overview-section-head h2 {
    font-size: 14px;
  }
}
@media (max-width: 650px) {
  .network-overview {
    padding-top: 25px;
  }
  .overview-heading {
    padding-bottom: 23px;
    align-items: flex-start;
  }
  .overview-date {
    font-size: 14px;
    gap: 7px;
  }
  .overview-date > .overview-separator,
  .overview-date > .overview-separator + * {
    display: none;
  }
  .overview-heading h1 {
    font-size: 29px;
    letter-spacing: -1.2px;
    margin: 12px 0 10px;
  }
  .overview-heading p {
    font-size: 14px;
    max-width: 23ch;
    line-height: 1.8;
  }
  .outline-button {
    font-size: 0;
    padding: 9px;
    margin-top: 37px;
  }
  .outline-button svg {
    width: 15px;
    height: 15px;
  }
  .atlas-studio {
    grid-template-columns: 1fr;
    min-height: 0;
  }
  .atlas-metrics {
    padding: 20px;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0 20px;
  }
  .atlas-section-name {
    grid-column: 1/-1;
    font-size: 14px;
  }
  .primary-metric {
    margin-top: 18px;
  }
  .primary-metric > strong {
    font-size: 50px;
  }
  .primary-metric > span {
    font-size: 14px;
  }
  .primary-metric > small {
    font-size: 14px;
    max-width: 125px;
    line-height: 1.7;
    align-items: baseline;
  }
  .secondary-metrics {
    margin-top: 21px;
    gap: 15px;
  }
  .secondary-metrics strong {
    font-size: 22px;
  }
  .secondary-metrics > a > span {
    font-size: 14px;
  }
  .atlas-primary-action {
    grid-column: 1/-1;
    margin-top: 18px;
    padding: 10px 12px;
  }
  .atlas-visual {
    min-height: 290px;
    height: 290px;
  }
  .atlas-coordinate {
    font-size: 14px;
    left: 16px;
    right: 16px;
  }
  .atlas-watermark {
    font-size: 30px;
    left: 16px;
    bottom: 53px;
  }
  .atlas-quality {
    padding: 15px;
    gap: 13px;
    flex-wrap: wrap;
  }
  .quality-gauge {
    width: 58px;
    height: 58px;
  }
  .quality-heading {
    font-size: 14px;
    min-width: 64px;
  }
  .quality-period {
    font-size: 14px;
  }
  .quality-legend {
    gap: 10px;
  }
  .quality-legend > span {
    font-size: 14px;
    gap: 5px;
  }
  .quality-legend strong {
    font-size: 15px;
    margin-left: 10px;
  }
  .quality-settings {
    width: 100%;
    justify-content: center;
    padding-top: 12px;
    border-top: 1px solid var(--ui-border-subtle);
    font-size: 14px;
  }
  .overview-lower {
    grid-template-columns: 1fr;
    margin-top: 26px;
    gap: 27px;
  }
  .overview-section-head {
    margin-bottom: 17px;
  }
  .overview-section-head h2 {
    font-size: 15px;
  }
  .overview-section-head p {
    font-size: 14px;
    line-height: 1.7;
  }
  .overview-section-head > a {
    font-size: 14px;
  }
  .routing-tabs {
    gap: 1px;
  }
  .routing-tabs button {
    padding: 6px 12px;
    font-size: 14px;
  }
  .routing-empty {
    padding: 27px 3px;
    min-height: 172px;
    gap: 15px;
  }
  .routing-empty strong {
    font-size: 15px;
  }
  .routing-empty p {
    font-size: 14px;
    max-width: 25ch;
  }
  .routing-note {
    font-size: 14px;
    gap: 5px;
  }
  .routing-note > span {
    max-width: 160px;
  }
  .routing-note button {
    font-size: 14px;
  }
  .region-section {
    padding: 23px 0 0;
    border-left: 0;
    border-top: 1px solid var(--ui-border);
  }
  .region-empty {
    min-height: 160px;
  }
  .region-list {
    min-height: 0;
  }
  .region-source-link {
    padding-top: 13px;
  }
  .activity-section {
    margin-top: 27px;
    padding-top: 23px;
  }
  .activity-empty {
    padding: 16px 13px;
    gap: 10px;
    font-size: 14px;
    line-height: 1.8;
  }
  .activity-empty-tag {
    display: none;
  }
  .activity-item {
    flex-wrap: wrap;
    gap: 8px;
  }
  .activity-item strong {
    max-width: calc(100% - 80px);
  }
  .activity-item > span {
    order: 3;
    flex-basis: 100%;
    padding-left: 13px;
    white-space: normal;
  }
  .activity-item time {
    font-size: 14px;
  }
  .workflow-strip {
    grid-template-columns: 1fr;
    margin-top: 23px;
  }
  .workflow-strip > a {
    padding: 17px 0 !important;
    border-left: 0 !important;
  }
  .workflow-strip > a:not(:last-child) {
    border-bottom: 1px solid var(--ui-border);
  }
  .workflow-strip > a > div {
    display: flex;
    align-items: center;
    gap: 15px;
  }
  .workflow-strip span {
    margin: 0;
  }
  .workflow-strip strong {
    font-size: 15px;
  }
  .routing-table {
    min-height: 210px;
  }
}
</style>
