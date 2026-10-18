<template>
  <div class="network-overview" :aria-busy="loading">
    <PageHeader title="我的网络" :description="greeting">
      <template #actions>
        <UiButton :icon="Refresh" :loading="loading" @click="loadOverview">
          {{ loading ? "正在同步" : "刷新" }}
        </UiButton>
        <UiButton
          variant="primary"
          :icon="TopRight"
          @click="router.push('/subcription/nodes')"
          >管理节点</UiButton
        >
      </template>
    </PageHeader>

    <div v-if="loadError" class="overview-notice" role="alert">
      <StatusDot state="warn" :label="loadError" />
      <UiButton variant="ghost" :disabled="loading" @click="loadOverview"
        >重新加载</UiButton
      >
    </div>

    <div class="overview-hero" data-enter>
      <div class="hero-total">
        <UiStat
          size="hero"
          label="节点总数"
          :value="loading ? null : summary.total"
        />
        <p class="hero-caption">
          {{
            summary.total
              ? `分布在 ${countries.length} 个地区，连接正常率 ${survivalRate}%`
              : "添加节点或同步机场后，这里会显示网络规模。"
          }}
        </p>
      </div>
      <RouterLink to="/subcription/subs" class="hero-metric">
        <UiStat size="large" label="订阅" :value="loading ? null : subTotal" />
        <TopRight aria-hidden="true" />
      </RouterLink>
      <RouterLink to="/subcription/nodes" class="hero-metric">
        <UiStat
          size="large"
          label="连接评分"
          :value="
            loading || !summary.total ? null : Math.round(summary.averageScore)
          "
          unit="/100"
        />
        <TopRight aria-hidden="true" />
      </RouterLink>
      <RouterLink to="/subcription/airport" class="hero-metric">
        <UiStat
          size="large"
          label="机场剩余流量"
          :value="traffic.reported ? remaining.value : null"
          :decimals="1"
          :unit="remaining.unit"
        />
        <TopRight aria-hidden="true" />
      </RouterLink>
    </div>

    <div class="overview-grid" data-enter>
      <WorldMap
        class="overview-map"
        :points="mapPoints"
        :loading="loading"
        @refresh="loadOverview"
      />
      <div class="overview-side">
        <UiPanel title="连接健康" description="最近一次质量采样的结果。">
          <HealthStrip
            :summary="summary"
            :loading="loading"
            :rate="survivalRate"
          />
        </UiPanel>
        <UiPanel
          title="机场流量"
          :count="airports.length"
          description="同步机场时自动读取。"
          flush
        >
          <template #actions
            ><UiButton
              variant="ghost"
              @click="router.push('/subcription/airport')"
              >全部</UiButton
            ></template
          >
          <UiSkeleton v-if="loading" :lines="3" height="40px" />
          <ul v-else-if="airports.length" class="airport-traffic">
            <li v-for="airport in airports.slice(0, 5)" :key="airport.ID">
              <span class="airport-name">{{ airport.Name }}</span>
              <TrafficMeter :traffic="trafficOf(airport)" />
            </li>
          </ul>
          <EmptyState
            v-else
            size="inline"
            title="还没有机场"
            description="添加机场订阅后显示流量与到期。"
          />
        </UiPanel>
      </div>
    </div>

    <UiPanel
      class="region-panel"
      title="所在地区"
      :count="countries.length"
      description="按节点定位统计。"
      data-enter
    >
      <template #actions
        ><UiButton
          variant="ghost"
          :icon="Plus"
          @click="router.push('/subcription/airport')"
          >添加机场订阅</UiButton
        ></template
      >
      <UiSkeleton v-if="loading" :lines="4" height="32px" />
      <ol v-else-if="countries.length" class="region-list">
        <li v-for="country in countries.slice(0, 12)" :key="country.name">
          <span class="region-name">{{ country.name }}</span>
          <span class="region-count mono">{{ country.count }}</span>
          <span class="region-track" aria-hidden="true"
            ><i
              :style="{
                transform: `scaleX(${country.count / (countries[0]?.count || 1)})`,
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
    </UiPanel>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, ref, onMounted } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import { getNodeQualitySummary } from "@/api/subcription/node";
import { getAirports } from "@/api/subcription/airport";
import TrafficMeter from "@/views/subcription/airport/TrafficMeter.vue";
import type { Airport } from "@/views/subcription/airport/useAirports";
import { formatBytes, trafficOf } from "@/utils/traffic";
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
const airports = ref<Airport[]>([]);
const traffic = computed(() => {
  const reports = airports.value.map(trafficOf).filter((t) => !!t);
  return {
    reported: reports.length > 0,
    remaining: reports.reduce((sum, t) => sum + t!.remaining, 0),
  };
});
const remaining = computed(() => {
  const [value, unit] = formatBytes(traffic.value.remaining).split(" ");
  return { value: Number(value), unit };
});
const greeting = computed(() => {
  const hour = new Date().getHours();
  const part =
    hour < 6
      ? "夜深了"
      : hour < 12
        ? "早上好"
        : hour < 18
          ? "下午好"
          : "晚上好";
  return `${part}，这是你的节点、订阅与机场流量概况。`;
});
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
    getAirports(),
  ]);
  const [subs, quality, map, airportList] = results;
  if (airportList.status === "fulfilled")
    airports.value = Array.isArray(airportList.value.data)
      ? airportList.value.data
      : [];
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
.overview-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) repeat(3, minmax(0, 1fr));
  margin-bottom: 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  overflow: hidden;
}
.hero-total {
  position: relative;
  padding: 20px 24px;
  background:
    radial-gradient(
      120% 140% at 0% 100%,
      color-mix(in srgb, var(--c-signal) 14%, transparent),
      transparent 60%
    ),
    var(--c-surface);
}
.hero-caption {
  margin-top: 8px;
  color: var(--c-muted);
  font-size: 13px;
}
.hero-metric {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  padding: 20px;
  border-left: 1px solid var(--c-line);
  color: var(--c-ink);
  transition: background-color var(--duration-fast) var(--ease-out);
}
.hero-metric:hover {
  background: var(--c-surface-2);
}
.hero-metric > svg {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
  color: var(--c-muted);
  transition: transform var(--duration-medium) var(--ease-spring);
}
.hero-metric:hover > svg {
  color: var(--c-data);
  transform: translate(2px, -2px);
}
.hero-metric :deep(.ui-stat) {
  align-self: end;
}
.overview-grid {
  display: grid;
  grid-template-columns: minmax(0, 7fr) minmax(300px, 4fr);
  gap: 16px;
  align-items: start;
  margin-bottom: 16px;
}
.overview-side {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.airport-traffic li {
  display: grid;
  gap: 6px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--c-line);
}
.airport-traffic li:last-child {
  border-bottom: 0;
}
.airport-name {
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.region-list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 14px 24px;
}
.region-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 6px 12px;
}
.region-name {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.region-count {
  color: var(--c-muted);
  font: 13px var(--font-mono);
}
.region-track {
  grid-column: 1 / -1;
  height: 4px;
  border-radius: 999px;
  background: var(--c-surface-2);
  overflow: hidden;
}
.region-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--c-data);
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
@media (max-width: 1199px) {
  .overview-hero {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  .hero-total {
    grid-column: 1 / -1;
    border-bottom: 1px solid var(--c-line);
  }
  .hero-metric:nth-child(2) {
    border-left: 0;
  }
  .overview-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
@media (max-width: 767px) {
  .overview-hero {
    grid-template-columns: minmax(0, 1fr);
  }
  .hero-metric {
    border-left: 0;
    border-top: 1px solid var(--c-line);
    padding: 14px 16px;
  }
  .hero-metric:nth-child(2) {
    border-top: 0;
  }
  .hero-total {
    padding: 16px;
  }
  .overview-notice {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
