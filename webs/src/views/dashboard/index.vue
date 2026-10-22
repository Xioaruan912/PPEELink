<template>
  <div class="network-overview" :aria-busy="loading">
    <PageHeader title="我的网络" :description="greeting">
      <template #actions>
        <UiButton :icon="Refresh" :loading="loading" @click="loadOverview">
          {{ loading ? "同步中" : "刷新" }}
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

    <div class="bento">
      <section class="cell cell-hero" aria-label="节点总数" data-enter>
        <UiStat
          size="hero"
          label="节点总数"
          :value="loading ? null : summary.total"
        />
        <p class="hero-caption">
          {{
            summary.total
              ? `分布在 ${countries.length} 个地区`
              : "添加节点后显示网络规模"
          }}
        </p>
        <div class="hero-health">
          <HealthStrip
            :summary="summary"
            :loading="loading"
            :rate="validRate"
          />
        </div>
      </section>

      <RouterLink
        to="/subcription/subs"
        class="cell cell-link cell-subs"
        data-enter
      >
        <span class="cell-icon"><IconLink /></span>
        <UiStat size="large" label="订阅" :value="loading ? null : subTotal" />
        <IconArrow class="cell-arrow" aria-hidden="true" />
      </RouterLink>

      <RouterLink
        to="/subcription/nodes"
        class="cell cell-link cell-score"
        data-enter
      >
        <div class="score-ring" aria-hidden="true">
          <svg viewBox="0 0 120 120">
            <circle class="ring-track" cx="60" cy="60" r="52" />
            <circle
              class="ring-value"
              cx="60"
              cy="60"
              r="52"
              :style="{ '--score': validRatio }"
            />
          </svg>
        </div>
        <UiStat
          size="large"
          label="有效节点"
          :value="loading || !summary.total ? null : summary.valid"
          :unit="`/${summary.total}`"
        />
        <IconArrow class="cell-arrow" aria-hidden="true" />
      </RouterLink>

      <section
        v-reveal
        class="cell cell-traffic"
        aria-label="订阅到期"
        data-enter
      >
        <header class="cell-head">
          <div>
            <h2>订阅到期</h2>
            <p>{{ expiring.length }} 个订阅已设置到期时间</p>
          </div>
          <UiButton variant="ghost" @click="router.push('/subcription/subs')"
            >全部订阅</UiButton
          >
        </header>
        <UiSkeleton v-if="loading" :lines="2" height="40px" />
        <ul v-else-if="expiring.length" class="expiry-list">
          <li v-for="sub in expiring" :key="sub.ID">
            <span class="expiry-name">{{ sub.Name }}</span>
            <span
              class="expiry-days mono"
              :class="{ 'is-soon': daysLeft(sub.ExpiresAt) <= 7 }"
              >{{ expiryLabel(sub.ExpiresAt) }}</span
            >
          </li>
        </ul>
        <EmptyState
          v-else
          size="inline"
          title="还没有到期时间"
          description="在订阅详情里设置过期时间后，这里显示最近的续期提醒。"
        />
      </section>

      <WorldMap
        class="cell-map"
        :points="mapPoints"
        :loading="loading"
        data-enter
        @refresh="loadOverview"
      />

      <section
        v-reveal
        class="cell cell-regions"
        aria-label="所在地区"
        data-enter
      >
        <header class="cell-head">
          <div>
            <h2>所在地区</h2>
            <p>{{ countries.length }} 个地区</p>
          </div>
        </header>
        <UiSkeleton v-if="loading" :lines="5" height="28px" />
        <ol v-else-if="countries.length" class="region-list">
          <li v-for="country in countries.slice(0, 8)" :key="country.name">
            <span class="region-name">{{ country.name }}</span>
            <span class="region-count mono">{{ country.count }}</span>
            <span class="region-bar" aria-hidden="true"
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
      </section>
    </div>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, ref, onMounted } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import { getNodeQualitySummary } from "@/api/subcription/node";
import { getSubs } from "@/api/subcription/subs";
import { Refresh, TopRight } from "@element-plus/icons-vue";
import IconLink from "~icons/ph/link-simple";
import IconArrow from "~icons/ph/arrow-up-right";
import {
  PageHeader,
  UiButton,
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
  valid: 0,
  invalid: 0,
});
const mapPoints = ref<
  { lat: number; lng: number; name: string; country: string }[]
>([]);
type SubSummary = { ID: number; Name: string; ExpiresAt: string | null };
const subs = ref<SubSummary[]>([]);
const expiring = computed(() =>
  subs.value
    .filter((sub) => !!sub.ExpiresAt)
    .sort(
      (a, b) =>
        new Date(a.ExpiresAt || 0).getTime() -
        new Date(b.ExpiresAt || 0).getTime()
    )
    .slice(0, 4)
);
function daysLeft(value: string | null) {
  if (!value) return Number.POSITIVE_INFINITY;
  return Math.ceil((new Date(value).getTime() - Date.now()) / 86400000);
}
function expiryLabel(value: string | null) {
  const days = daysLeft(value);
  if (!Number.isFinite(days)) return "";
  if (days < 0) return "已过期";
  if (days === 0) return "今天到期";
  return `${days} 天后`;
}
const validRatio = computed(() =>
  summary.value.total ? summary.value.valid / summary.value.total : 0
);
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
  return `${part}，网络运行概况如下。`;
});
const validRate = computed(() => {
  const { total, invalid: offline } = summary.value;
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
    getSubs(),
  ]);
  const [subsCount, quality, map, subList] = results;
  if (subList.status === "fulfilled")
    subs.value = Array.isArray(subList.value.data) ? subList.value.data : [];
  if (subsCount.status === "fulfilled")
    subTotal.value = Number(subsCount.value.data) || 0;
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
  padding: 12px 16px;
  margin-bottom: 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.bento {
  display: grid;
  grid-template-columns: repeat(12, minmax(0, 1fr));
  grid-auto-rows: minmax(150px, auto);
  gap: 14px;
}
.cell {
  position: relative;
  min-width: 0;
  padding: 20px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
  overflow: hidden;
}
.cell-hero {
  grid-column: span 5;
  grid-row: span 2;
  display: flex;
  flex-direction: column;
  padding: 24px;
  background:
    radial-gradient(
      90% 70% at 100% 0%,
      color-mix(in srgb, var(--c-signal) 26%, transparent),
      transparent 65%
    ),
    radial-gradient(
      60% 60% at 0% 100%,
      color-mix(in srgb, var(--c-data) 16%, transparent),
      transparent 70%
    ),
    var(--c-surface);
}
.cell-hero :deep(.ui-stat--hero .ui-stat__number) {
  font: 600 clamp(72px, 7.4vw, 112px) / 0.9 var(--font-ui);
  letter-spacing: -0.06em;
}
.hero-caption {
  margin-top: 10px;
  color: var(--c-muted);
  font-size: 14px;
}
.hero-health {
  margin-top: auto;
  padding-top: 20px;
}
.cell-link {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  color: var(--c-ink);
  transition:
    translate var(--duration-medium) var(--ease-spring),
    transform var(--duration-slow) var(--ease-out),
    border-color var(--duration-fast);
}
.cell-link:hover {
  translate: 0 -3px;
  border-color: color-mix(in srgb, var(--c-focus) 60%, var(--c-line));
}
.cell-subs {
  grid-column: span 3;
}
.cell-score {
  grid-column: span 4;
  flex-direction: row;
  align-items: flex-end;
  justify-content: flex-start;
  gap: 18px;
}
.cell-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.cell-icon svg {
  width: 20px;
  height: 20px;
}
.cell-arrow {
  position: absolute;
  top: 18px;
  right: 18px;
  width: 18px;
  height: 18px;
  color: var(--c-muted);
  transition: transform var(--duration-medium) var(--ease-spring);
}
.cell-link:hover .cell-arrow {
  color: var(--c-ink);
  transform: translate(3px, -3px);
}
.score-ring {
  width: 92px;
  height: 92px;
  flex: 0 0 auto;
}
.score-ring svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}
.ring-track,
.ring-value {
  fill: none;
  stroke-width: 10;
}
.ring-track {
  stroke: var(--c-surface-2);
}
.ring-value {
  stroke: var(--c-signal);
  stroke-linecap: round;
  stroke-dasharray: 326.7;
  stroke-dashoffset: calc(326.7 * (1 - var(--score)));
  transition: stroke-dashoffset 1.2s var(--ease-out);
}
.cell-traffic {
  grid-column: span 7;
  background:
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--c-surface-2) 70%, transparent),
      transparent 60%
    ),
    var(--c-surface);
}
.cell-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.cell-head h2 {
  font-size: 15px;
  font-weight: 600;
}
.cell-head p {
  margin-top: 2px;
  color: var(--c-muted);
  font-size: 13px;
}
.cell-head strong {
  color: var(--c-ink);
  font-weight: 500;
}
.expiry-list {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
  gap: 10px 20px;
}
.expiry-list li {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--c-line);
}
.expiry-name {
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cell-map {
  grid-column: span 8;
}
.cell-regions {
  grid-column: span 4;
}
.region-list {
  display: grid;
  gap: 12px;
}
.region-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 6px 12px;
}
.region-name {
  font-size: 14px;
}
.region-count {
  color: var(--c-muted);
  font: 13px var(--font-mono);
}
.region-bar {
  grid-column: 1 / -1;
  height: 6px;
}
.region-bar i {
  display: block;
  height: 100%;
  border-radius: var(--radius-pill);
  background: linear-gradient(90deg, var(--c-signal), var(--c-data));
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
@media (max-width: 1199px) {
  .cell-hero {
    grid-column: span 12;
    grid-row: auto;
  }
  .cell-subs {
    grid-column: span 5;
  }
  .cell-score {
    grid-column: span 7;
  }
  .cell-traffic,
  .cell-map,
  .cell-regions {
    grid-column: span 12;
  }
}
@media (max-width: 767px) {
  .bento {
    grid-template-columns: minmax(0, 1fr);
    grid-auto-rows: auto;
    gap: 10px;
  }
  .cell-subs {
    flex-direction: row;
    align-items: center;
    justify-content: flex-start;
    gap: 14px;
  }
  .score-ring {
    width: 64px;
    height: 64px;
  }
  .cell-hero,
  .cell-subs,
  .cell-score,
  .cell-traffic,
  .cell-map,
  .cell-regions {
    grid-column: auto;
  }
  .cell {
    padding: 16px;
  }
  .overview-notice {
    align-items: flex-start;
    flex-direction: column;
  }
}
@media (prefers-reduced-motion: reduce) {
  .ring-value,
  .cell-link {
    transition: none;
  }
}
</style>
