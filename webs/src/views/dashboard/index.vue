<template>
  <div class="network-overview" :aria-busy="loading">
    <PageHeader title="我的网络">
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
        </template>
      </UiPanel>
      <WorldMap
        :points="mapPoints"
        :loading="loading"
        @refresh="loadOverview"
      />
    </div>

    <section class="region-section" aria-labelledby="region-title" data-enter>
      <header class="section-heading">
        <h2 id="region-title">所在地区</h2>
        <UiButton
          variant="ghost"
          :icon="Plus"
          @click="router.push('/subcription/airport')"
          >添加机场订阅</UiButton
        >
      </header>
      <UiSkeleton v-if="loading" :lines="4" height="40px" />
      <ol v-else-if="countries.length" class="region-list">
        <li v-for="country in countries.slice(0, 8)" :key="country.name">
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
        v-else
        size="inline"
        title="暂无地区"
        description="节点定位后会显示所在地区。"
      />
    </section>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, ref, onMounted } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import { getNodeQualitySummary } from "@/api/subcription/node";
import { Refresh, TopRight, Plus } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiPanel,
  UiStat,
  UiSkeleton,
  StatusDot,
  EmptyState,
} from "@/ui";
import WorldMap from "./components/WorldMap.vue";
import HealthStrip from "./components/HealthStrip.vue";
const router = useRouter();
defineOptions({ name: "Dashboard", inheritAttrs: false });
const loading = ref(true),
  loadError = ref(""),
  subTotal = ref(0);
const summary = ref({
  total: 0,
  healthy: 0,
  warning: 0,
  offline: 0,
  averageScore: 0,
});
const mapPoints = ref<
  { lat: number; lng: number; name: string; country: string }[]
>([]);
const survivalRate = computed(() => {
  const { total, offline } = summary.value;
  return total
    ? Math.round((Math.max(total - offline, 0) / total) * 1000) / 10
    : 0;
});
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
async function loadOverview() {
  loading.value = true;
  loadError.value = "";
  const results = await Promise.allSettled([
    getSubTotal(),
    getNodeQualitySummary(),
    getNodeMap(),
  ]);
  const [subs, quality, map] = results;
  if (subs.status === "fulfilled")
    subTotal.value = Number(subs.value.data) || 0;
  if (quality.status === "fulfilled")
    summary.value = { ...summary.value, ...quality.value.data };
  if (map.status === "fulfilled")
    mapPoints.value = Array.isArray(map.value.data) ? map.value.data : [];
  if (results.some((r) => r.status === "rejected"))
    loadError.value = "部分网络数据暂时无法加载，请检查服务状态后重试。";
  loading.value = false;
}
onMounted(loadOverview);
</script>
<style scoped>
.network-overview {
  min-width: 0;
}
.overview-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  margin-bottom: 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface);
}
.overview-stage {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
  gap: 16px;
  align-items: stretch;
}
.network-situation {
  display: flex;
  flex-direction: column;
}
.network-situation :deep(.ui-panel__header) {
  margin-bottom: 20px;
}
.total-stat :deep(.ui-stat__label) {
  margin-bottom: 8px;
}
.secondary-metrics {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.4fr);
  gap: 16px;
  margin-top: 20px;
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
  margin-top: 12px;
  font-size: 14px;
}
.situation-action {
  margin: 20px 0 0;
}
.network-situation :deep(.ui-panel__footer) {
  margin-top: auto;
  padding-top: 16px;
}
.region-section {
  margin-top: 24px;
}
.section-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
  min-height: 36px;
}
h2 {
  margin: 0;
  font-size: 17px;
  font-weight: 600;
}
.region-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.region-list {
  border-left: 1px solid var(--c-line);
  padding-left: 16px;
}
.region-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px 12px;
  position: relative;
  padding: 0 0 16px;
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
  font-size: 14px;
  overflow-wrap: anywhere;
}
.region-list :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.region-list :deep(.ui-stat__number) {
  font-size: 14px;
}
.region-list :deep(.ui-stat__unit) {
  font-size: 14px;
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
@media (min-width: 768px) and (max-width: 1100px) {
  .overview-stage {
    grid-template-columns: minmax(0, 5fr) minmax(0, 6fr);
    gap: 12px;
  }
  .secondary-metrics {
    gap: 8px;
  }
  .metric-link {
    gap: 4px;
  }
  .metric-link :deep(.ui-stat__number) {
    font-size: 20px;
  }
  .metric-link {
    flex-wrap: wrap;
  }
  .metric-link > svg {
    margin-top: 0;
  }
}
@media (max-width: 767px) {
  .overview-stage {
    grid-template-columns: minmax(0, 1fr);
  }
  .overview-stage {
    gap: 16px;
  }
  .network-situation :deep(.ui-panel__footer) {
    margin-top: 20px;
  }
  .overview-notice {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
