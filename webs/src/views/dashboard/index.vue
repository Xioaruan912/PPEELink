<template>
  <div class="network-overview" :aria-busy="loading">
    <PageHeader eyebrow="总览" title="我的网络">
      <template #actions>
        <UiButton :icon="Refresh" :loading="loading" @click="loadOverview">
          {{ loading ? "正在同步" : "刷新" }}
        </UiButton>
      </template>
    </PageHeader>

    <div v-if="loadError" class="overview-notice" role="alert">
      <StatusDot state="warn" :label="loadError" />
      <UiButton variant="ghost" :disabled="loading" @click="loadOverview"
        >重新加载</UiButton
      >
    </div>

    <div class="overview-stage" data-enter>
      <UiPanel title="网络态势" class="network-situation">
        <UiSkeleton v-if="loading" :lines="3" height="56px" />
        <template v-else>
          <UiStat
            size="hero"
            class="total-stat"
            label="节点总数"
            :value="summary.total"
          />
          <div class="secondary-metrics">
            <RouterLink to="/subcription/subs" class="metric-link">
              <UiStat size="medium" label="订阅数" :value="subTotal" />
              <TopRight aria-hidden="true" />
            </RouterLink>
            <RouterLink to="/subcription/nodes" class="metric-link">
              <UiStat
                size="medium"
                v-if="summary.total"
                label="连接评分"
                :value="Math.round(summary.averageScore)"
                unit="/100"
              />
              <span v-else class="score-empty"
                >连接评分<span>暂无评分</span></span
              >
              <TopRight aria-hidden="true" />
            </RouterLink>
          </div>
        </template>
        <div class="situation-action">
          <UiButton
            variant="primary"
            :icon="TopRight"
            @click="router.push('/subcription/nodes')"
            >管理节点</UiButton
          >
        </div>
        <template #footer>
          <HealthStrip
            :summary="summary"
            :loading="loading"
            :rate="survivalRate"
          />
          <UiButton
            class="alert-action"
            variant="ghost"
            :icon="Setting"
            :loading="readingAlert"
            @click="openAlerts"
            >异常提醒设置</UiButton
          >
        </template>
      </UiPanel>
      <WorldMap
        :points="mapPoints"
        :loading="loading"
        @refresh="loadOverview"
      />
    </div>

    <div class="overview-details" data-enter>
      <section aria-labelledby="recommendation-title" class="routing-section">
        <header class="section-heading">
          <h2 id="recommendation-title">推荐节点</h2>
          <UiButton
            variant="ghost"
            :icon="TopRight"
            @click="router.push('/test/speed')"
            >测试速度</UiButton
          >
        </header>
        <RecommendationList
          v-model="activeScene"
          :scenes="sceneOptions"
          :loading="loading"
        />
      </section>
      <aside class="network-history">
        <section aria-labelledby="region-title">
          <header class="section-heading">
            <h2 id="region-title">所在地区</h2>
          </header>
          <UiSkeleton v-if="loading" :lines="4" height="40px" />
          <ol v-else-if="countries.length" class="region-list">
            <li v-for="country in countries.slice(0, 5)" :key="country.name">
              <span class="region-name">{{ country.name }}</span>
              <UiStat
                size="small"
                label="节点数"
                :value="country.count"
                unit="个"
              />
              <span class="region-track" aria-hidden="true"
                ><i
                  :style="{
                    transform: `scaleX(${country.count / mapPoints.length})`,
                  }"
                ></i
              ></span>
            </li>
          </ol>
          <EmptyState
            size="inline"
            v-else
            class="aside-empty"
            title="暂无地区"
            description="节点定位后会显示所在地区。"
          />
          <UiButton
            class="region-source"
            variant="ghost"
            :icon="Plus"
            @click="router.push('/subcription/airport')"
            >添加机场订阅</UiButton
          >
        </section>
        <section aria-labelledby="activity-title" class="activity-section">
          <header class="section-heading">
            <h2 id="activity-title">最近变化</h2>
            <UiButton
              variant="ghost"
              :icon="Setting"
              :loading="readingAlert"
              @click="openAlerts"
              aria-label="异常提醒设置"
            />
          </header>
          <UiSkeleton v-if="loading" :lines="4" height="48px" />
          <ol v-else-if="events.length" class="activity-list">
            <li v-for="event in events.slice(0, 4)" :key="event.id">
              <StatusDot
                :state="eventState(event.type)"
                :label="event.nodeName"
              />
              <p>{{ event.message }}</p>
              <time :datetime="event.createdAt">{{
                formatTime(event.createdAt)
              }}</time>
            </li>
          </ol>
          <EmptyState
            size="inline"
            v-else
            class="aside-empty"
            title="暂无变化"
            description="连接状态变化会记录在这里。"
          />
        </section>
      </aside>
    </div>

    <section
      class="diagnostics-section"
      aria-labelledby="diagnostics-title"
      data-enter
    >
      <header class="section-heading">
        <div>
          <p class="section-eyebrow">连接诊断</p>
          <h2 id="diagnostics-title">响应时间</h2>
        </div>
        <UiButton
          variant="ghost"
          :icon="ArrowDown"
          :aria-expanded="diagnosticsVisible"
          aria-controls="network-diagnostics"
          @click="diagnosticsVisible = !diagnosticsVisible"
        >
          {{ diagnosticsVisible ? "收起响应时间" : "查看响应时间" }}
        </UiButton>
      </header>
      <div id="network-diagnostics"><NodePing v-if="diagnosticsVisible" /></div>
    </section>

    <UiDialog
      v-model="alertVisible"
      title="异常提醒设置"
      width="540px"
      :close-on-click-modal="!savingAlert"
    >
      <el-form label-position="top" @submit.prevent="saveAlerts">
        <el-form-item
          ><el-switch v-model="alertForm.enabled" active-text="开启异常提醒"
        /></el-form-item>
        <el-form-item label="通知接收地址"
          ><el-input v-model="alertForm.webhookUrl" placeholder="https://…"
        /></el-form-item>
        <el-form-item label="连续连接失败次数"
          ><el-input-number
            v-model="alertForm.failureThreshold"
            :min="1"
            :max="20"
        /></el-form-item>
        <div class="maintenance-fields">
          <el-form-item label="免打扰开始"
            ><el-time-select
              v-model="alertForm.maintenanceStart"
              start="00:00"
              step="00:30"
              end="23:30"
              clearable
          /></el-form-item>
          <el-form-item label="免打扰结束"
            ><el-time-select
              v-model="alertForm.maintenanceEnd"
              start="00:00"
              step="00:30"
              end="23:30"
              clearable
          /></el-form-item>
        </div>
        <el-alert
          title="维护期间继续记录探测样本，暂停发送 Webhook 告警。"
          type="info"
          :closable="false"
        />
      </el-form>
      <template #footer>
        <UiButton @click="alertVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="savingAlert" @click="saveAlerts"
          >保存设置</UiButton
        >
      </template>
    </UiDialog>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, ref, onMounted, defineAsyncComponent } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import {
  getNodeQualitySummary,
  getNodeRecommendations,
  getNodeHealthEvents,
  getAlertSetting,
  updateAlertSetting,
} from "@/api/subcription/node";
import { ElMessage } from "element-plus";
import {
  Refresh,
  TopRight,
  Setting,
  ArrowDown,
  Plus,
} from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiPanel,
  UiStat,
  UiSkeleton,
  StatusDot,
  EmptyState,
  UiDialog,
} from "@/ui";
import WorldMap from "./components/WorldMap.vue";
import RecommendationList from "./components/RecommendationList.vue";
import HealthStrip from "./components/HealthStrip.vue";
const router = useRouter();
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
  readingAlert = ref(false),
  savingAlert = ref(false),
  alertForm = ref({
    enabled: false,
    webhookUrl: "",
    failureThreshold: 3,
    maintenanceStart: "",
    maintenanceEnd: "",
  });
async function openAlerts() {
  if (readingAlert.value) return;
  readingAlert.value = true;
  try {
    const { data } = await getAlertSetting();
    alertForm.value = { ...alertForm.value, ...data };
    alertVisible.value = true;
  } catch {
    ElMessage.error("无法读取告警设置，请重试");
  } finally {
    readingAlert.value = false;
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
function formatTime(value: string) {
  return new Date(value).toLocaleTimeString("zh-CN", {
    hour: "2-digit",
    minute: "2-digit",
  });
}
function eventState(type: string): "ok" | "warn" | "bad" | "idle" {
  if (/recover|healthy|online/.test(type)) return "ok";
  if (/offline|fail|down/.test(type)) return "bad";
  if (/degrad|warn|slow/.test(type)) return "warn";
  return "idle";
}
onMounted(() => {
  activeScene.value = sceneOptions.value[0].key;
  loadOverview();
});
</script>
<style scoped>
.network-overview {
  min-width: 0;
}
.overview-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  margin-bottom: 24px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface);
}
.overview-stage {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
  gap: 24px;
  align-items: stretch;
}
.network-situation {
  display: flex;
  flex-direction: column;
}
.network-situation :deep(.ui-panel__header) {
  margin-bottom: 32px;
}
.total-stat :deep(.ui-stat__label) {
  margin-bottom: 8px;
}
.secondary-metrics {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr);
  gap: 24px;
  margin-top: 32px;
}
.metric-link {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  color: var(--c-ink);
  min-width: 0;
  border-radius: var(--radius-control);
}
.metric-link:hover {
  color: var(--c-data);
}
.metric-link > svg {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  margin-top: 4px;
  color: var(--c-data);
}
.metric-link :deep(.ui-stat__number) {
  font-size: clamp(26px, 2.5vw, 36px);
}
.metric-link :deep(.ui-stat__unit) {
  font-size: 14px;
  margin-left: 4px;
}
.score-empty {
  font-size: 14px;
  color: var(--c-muted);
}
.score-empty > span {
  display: block;
  margin-top: 16px;
  font-size: 16px;
}
.situation-action {
  margin: 32px 0 0;
}
.network-situation :deep(.ui-panel__footer) {
  margin-top: auto;
  padding-top: 24px;
}
.alert-action {
  margin-top: 16px;
  margin-left: -16px;
}
.overview-details {
  display: grid;
  grid-template-columns: minmax(0, 8fr) minmax(0, 4fr);
  gap: 64px;
  margin-top: 64px;
  padding-top: 48px;
  border-top: 1px solid var(--c-line);
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 24px;
  min-height: 44px;
}
h2 {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}
.network-history {
  min-width: 0;
}
.region-list,
.activity-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.region-list {
  border-left: 1px solid var(--c-line);
  padding-left: 24px;
}
.region-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px 16px;
  position: relative;
  padding: 0 0 24px;
}
.region-list li::before {
  content: "";
  position: absolute;
  left: -29px;
  top: 8px;
  width: 9px;
  height: 9px;
  border: 1px solid var(--c-muted);
  background: var(--c-canvas);
  border-radius: 50%;
}
.region-name {
  font-size: 15px;
  overflow-wrap: anywhere;
}
.region-source {
  margin-left: -16px;
}
.region-list :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.region-list :deep(.ui-stat__number) {
  font-size: 16px;
}
.region-list :deep(.ui-stat__unit) {
  font-size: 13px;
}
.region-track {
  grid-column: 1 / -1;
  height: 2px;
  background: var(--c-line);
}
.region-track i {
  display: block;
  height: 100%;
  background: var(--c-data);
  transform-origin: left;
}
.activity-section {
  margin-top: 32px;
  padding-top: 32px;
  border-top: 1px solid var(--c-line);
}
.activity-list {
  border-left: 1px solid var(--c-line);
  margin-left: 4px;
}
.activity-list li {
  position: relative;
  padding: 0 0 24px 24px;
}
.activity-list li:last-child {
  padding-bottom: 0;
}
.activity-list :deep(.status-dot) {
  display: flex;
  align-items: flex-start;
  font-size: 15px;
  color: var(--c-ink);
  overflow-wrap: anywhere;
}
.activity-list :deep(.status-dot i) {
  position: absolute;
  left: -4px;
  top: 8px;
}
.activity-list p {
  margin: 8px 0;
  font-size: 14px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.activity-list time {
  font: 13px/1.6 var(--font-mono);
  color: var(--c-muted);
}
.aside-empty {
  padding: 24px 0;
}
.aside-empty :deep(p) {
  font-size: 14px;
}
.diagnostics-section {
  margin-top: 64px;
  padding-top: 48px;
  border-top: 1px solid var(--c-line);
}
.section-eyebrow {
  font: 13px/1.4 var(--font-mono);
  letter-spacing: 0.08em;
  color: var(--c-muted);
  margin: 0 0 16px;
}
.maintenance-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
@media (min-width: 768px) and (max-width: 1100px) {
  .overview-stage {
    grid-template-columns: minmax(0, 5fr) minmax(0, 6fr);
    gap: 16px;
  }
  .overview-details {
    gap: 32px;
  }
  .secondary-metrics {
    gap: 8px;
  }
  .metric-link {
    gap: 4px;
  }
  .metric-link :deep(.ui-stat__number) {
    font-size: 26px;
  }
  .metric-link {
    flex-wrap: wrap;
  }
  .metric-link > svg {
    margin-top: 0;
  }
}
@media (max-width: 767px) {
  .overview-stage,
  .overview-details {
    grid-template-columns: minmax(0, 1fr);
  }
  .overview-stage {
    gap: 24px;
  }
  .overview-details {
    gap: 48px;
    margin-top: 48px;
    padding-top: 32px;
  }
  .network-situation :deep(.ui-panel__footer) {
    margin-top: 32px;
  }
  .overview-notice {
    align-items: flex-start;
    flex-direction: column;
  }
  .network-history {
    display: grid;
    gap: 48px;
  }
  .activity-section {
    margin-top: 0;
  }
  .diagnostics-section {
    margin-top: 48px;
    padding-top: 32px;
  }
  .diagnostics-section .section-heading {
    flex-wrap: wrap;
  }
  .maintenance-fields {
    grid-template-columns: 1fr;
    gap: 0;
  }
}
</style>
