<template>
  <div class="network-overview" :aria-busy="loading">
    <PageHeader title="网络总览" :description="greeting">
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

    <div class="stat-strip">
      <div class="stat-card">
        <span>节点总数</span>
        <strong>{{ loading ? "—" : summary.total }}</strong>
        <small>分布在 {{ countries.length }} 个地区</small>
      </div>
      <div class="stat-card">
        <span>有效节点</span>
        <strong class="stat-ok">{{ loading ? "—" : summary.valid }}</strong>
        <small>有效率 {{ loading ? "—" : `${validRate}%` }}</small>
      </div>
      <div class="stat-card">
        <span>订阅</span>
        <strong>{{ loading ? "—" : subTotal }}</strong>
        <small>{{ expiring.length }} 个已设置到期</small>
      </div>
      <div class="stat-card">
        <span>所在地区</span>
        <strong>{{ loading ? "—" : countries.length }}</strong>
        <small>{{ summary.invalid }} 个节点无效</small>
      </div>
    </div>

    <section class="task-hub" aria-label="常用功能">
      <div class="task-hub-heading"><h2>常用功能</h2><p>从服务器接入到订阅分发，按任务进入。</p></div>
      <div class="task-groups">
        <section v-for="group in taskGroups" :key="group.path" class="task-group">
          <h3><component :is="group.icon" aria-hidden="true" />{{ group.label }}</h3>
          <RouterLink v-for="item in group.children" :key="item.path" :to="item.path">
            <strong>{{ item.label }}</strong><small>{{ item.hint }}</small>
          </RouterLink>
        </section>
      </div>
    </section>

    <div class="dashboard-grid">
      <WorldMap
        class="dash-map"
        :points="mapPoints"
        :loading="loading"
        @refresh="loadOverview"
      />

      <aside class="side-stack">
        <section class="dash-panel side-panel" aria-label="节点状态">
          <header class="dash-head">
            <div>
              <h2>节点状态</h2>
              <p>{{ summary.total }} 个节点</p>
            </div>
          </header>
          <HealthStrip
            :summary="summary"
            :loading="loading"
            :rate="validRate"
          />
        </section>

        <section class="dash-panel side-panel" aria-label="订阅到期">
          <header class="dash-head">
            <div>
              <h2>订阅到期</h2>
              <p>{{ expiring.length }} 个已设置到期时间</p>
            </div>
            <UiButton variant="ghost" @click="router.push('/subcription/subs')"
              >全部</UiButton
            >
          </header>
          <UiSkeleton v-if="loading" :lines="2" height="32px" />
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
            title="没有到期时间"
            description="在订阅详情里设置后显示在这里。"
          />
        </section>

        <section class="dash-panel side-panel" aria-label="所在地区">
          <header class="dash-head">
            <div>
              <h2>所在地区</h2>
              <p>{{ countries.length }} 个地区</p>
            </div>
          </header>
          <UiSkeleton v-if="loading" :lines="3" height="24px" />
          <ol v-else-if="countries.length" class="region-list">
            <li v-for="country in countries.slice(0, 5)" :key="country.name">
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
            description="节点定位后显示在这里。"
          />
        </section>
      </aside>
    </div>

    <section class="dash-panel nodes-panel" aria-label="节点列表">
      <header class="dash-head nodes-head">
        <div>
          <h2>节点</h2>
          <p>最近检测状态与探针结果</p>
        </div>
        <UiButton variant="ghost" @click="router.push('/subcription/nodes')"
          >管理全部</UiButton
        >
      </header>
      <UiSkeleton v-if="loading" class="node-loading" :lines="3" height="40px" />
      <div v-else-if="!nodes.length" class="node-empty">
        <EmptyState
          size="inline"
          title="还没有节点"
          description="添加第一个节点后，这里显示可用性与探针结果。"
        />
      </div>
      <div v-else class="node-table">
        <RouterLink
          v-for="node in nodes.slice(0, 6)"
          :key="node.id"
          to="/subcription/nodes"
          class="node-row"
        >
          <span
            class="node-dot"
            :class="{ 'is-bad': !node.valid }"
            :aria-label="node.valid ? '有效' : '无效'"
          ></span>
          <div class="node-main">
            <strong>{{ node.name }}</strong>
            <small>{{ node.groups?.join(" · ") || "未分组" }}</small>
          </div>
          <span class="node-country">{{ node.country || "未识别" }}</span>
          <span class="node-probes mono">
            {{
              node.probes?.length
                ? `${node.probes.filter((probe) => probe.success).length}/${node.probes.length}`
                : "—"
            }}
          </span>
          <span class="node-valid" :class="{ 'is-bad': !node.valid }">{{
            node.blocked ? "暂不下发" : node.valid ? "有效" : "无效"
          }}</span>
        </RouterLink>
      </div>
    </section>
  </div>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, ref, onMounted } from "vue";
import { getSubTotal, getNodeMap } from "@/api/total";
import { getNodeOverview } from "@/api/subcription/node";
import type { OverviewItem } from "@/views/subcription/nodes/useNodes";
import { getNodeQualitySummary } from "@/api/subcription/node";
import { getSubs } from "@/api/subcription/subs";
import { Refresh, TopRight } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiSkeleton,
  StatusDot,
  EmptyState,
} from "@/ui";
import WorldMap from "./components/WorldMap.vue";
import HealthStrip from "./components/HealthStrip.vue";
import { usePermissionStore } from "@/store";
import { buildNavigation, organizeNavigation } from "@/layout/navigation";
const permission = usePermissionStore();
const taskGroups = computed(() => organizeNavigation(buildNavigation(permission.routes))
  .filter(group => ["服务器", "订阅与节点", "配置与规则", "网络诊断"].includes(group.label)));
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
const nodes = ref<OverviewItem[]>([]);
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
let inFlight = false;
async function loadOverview() {
  if (inFlight) return;
  inFlight = true;
  loading.value = true;
  loadError.value = "";
  const results = await Promise.allSettled([
    getSubTotal(),
    getNodeQualitySummary(),
    getNodeMap(),
    getSubs(),
    getNodeOverview(),
  ]);
  const [subsCount, quality, map, subList, overview] = results;
  if (subList.status === "fulfilled")
    subs.value = Array.isArray(subList.value.data) ? subList.value.data : [];
  if (subsCount.status === "fulfilled")
    subTotal.value = Number(subsCount.value.data) || 0;
  if (quality.status === "fulfilled")
    summary.value = { ...summary.value, ...quality.value.data };
  if (map.status === "fulfilled")
    mapPoints.value = Array.isArray(map.value.data) ? map.value.data : [];
  if (overview.status === "fulfilled")
    nodes.value = Array.isArray(overview.value.data) ? overview.value.data : [];
  if (results.some((r) => r.status === "rejected"))
    loadError.value = "部分网络数据暂时无法加载，请检查服务状态后重试。";
  loading.value = false;
  inFlight = false;
}
onMounted(loadOverview);
</script>
<style scoped>
.task-hub { padding: 18px; border: 1px solid var(--c-line); border-radius: 14px; background: var(--c-surface); }
.task-hub-heading { display: flex; align-items: baseline; gap: 12px; flex-wrap: wrap; margin-bottom: 14px; }
.task-hub-heading h2 { margin: 0; font-size: 15px; }
.task-hub-heading p { margin: 0; color: var(--c-muted); font-size: 12px; }
.task-groups { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 20px; }
.task-group { min-width: 0; }
.task-group h3 { display: flex; align-items: center; gap: 8px; margin: 0 0 8px; font-size: 13px; color: var(--c-muted); }
.task-group h3 svg { width: 17px; height: 17px; }
.task-group a { display: grid; gap: 3px; padding: 8px; margin: 0 -8px; border-radius: 8px; color: var(--c-ink); }
.task-group a:hover { background: var(--c-surface-2); }
.task-group a:focus-visible { outline: 2px solid var(--c-signal); outline-offset: 2px; }
.task-group strong { font-size: 13px; font-weight: 500; }
.task-group small { color: var(--c-muted); font-size: 11px; overflow-wrap: anywhere; }
@media (max-width: 1000px) { .task-groups { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 520px) { .task-groups { grid-template-columns: 1fr; gap: 14px; } .task-group + .task-group { border-top: 1px solid var(--c-line); padding-top: 12px; } }

.network-overview {
  min-width: 0;
  display: grid;
  gap: 14px;
}
.overview-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}
.stat-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.stat-card {
  display: grid;
  gap: 1px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
}
.stat-card span {
  color: var(--c-muted);
  font-size: 11.5px;
}
.stat-card strong {
  font: 600 28px/1.25 var(--font-ui);
  letter-spacing: -0.03em;
}
.stat-card strong.stat-ok {
  color: var(--c-ok);
}
.stat-card small {
  overflow: hidden;
  color: var(--c-subtle);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dashboard-grid {
  display: grid;
  grid-template-columns: minmax(0, 8fr) minmax(0, 4fr);
  gap: 14px;
  align-items: start;
}
.dash-panel {
  min-width: 0;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.dash-map {
  min-height: 380px;
}
.dash-map :deep(.map-stage),
.dash-map :deep(.world-map) {
  min-height: 260px;
}
.dash-map :deep(.map-selector),
.dash-map :deep(.map-location) {
  display: none;
}
.dash-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 0 0 12px;
}
.dash-head h2 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}
.dash-head p {
  margin: 2px 0 0;
  color: var(--c-muted);
  font-size: 11.5px;
}
.side-stack {
  display: grid;
  gap: 12px;
  align-content: start;
}
.side-panel {
  padding: 14px 16px;
}
.side-panel .dash-head {
  padding-bottom: 8px;
}
.dash-map {
  min-height: 340px;
}
.expiry-list {
  display: grid;
  gap: 6px;
}
.expiry-list li {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 10px;
  padding: 5px 0;
  border-bottom: 1px solid var(--c-line);
}
.expiry-list li:last-child {
  border-bottom: 0;
}
.expiry-name {
  overflow: hidden;
  font-size: 12.5px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.expiry-days {
  flex: none;
  color: var(--c-muted);
  font-size: 11.5px;
}
.expiry-days.is-soon {
  color: var(--c-warn);
}
.region-list {
  display: grid;
  gap: 8px;
}
.region-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 5px 10px;
}
.region-name {
  font-size: 12.5px;
}
.region-count {
  color: var(--c-muted);
  font-size: 11.5px;
}
.region-bar {
  grid-column: 1 / -1;
  height: 4px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--c-surface-2);
}
.region-bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--c-signal), var(--c-data));
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
.nodes-panel {
  overflow: hidden;
}
.nodes-head {
  padding: 14px 16px 10px;
}
.node-loading {
  padding: 12px 16px;
}
.node-empty {
  padding: 4px 16px 16px;
}
.node-table {
  display: grid;
}
.node-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
}
.node-dot.is-bad {
  border-radius: 0;
  background: var(--c-bad);
  transform: rotate(45deg);
}
.node-row {
  display: grid;
  grid-template-columns: 10px minmax(0, 1.6fr) minmax(0, 1fr) 70px 76px;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-top: 1px solid var(--c-line);
  color: var(--c-ink);
  text-decoration: none;
  transition: background var(--duration-fast);
}
.node-row:hover {
  background: var(--c-surface-2);
}
.node-main {
  display: grid;
  min-width: 0;
  gap: 1px;
}
.node-main strong {
  overflow: hidden;
  font-size: 12.5px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-main small {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-country,
.node-probes {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 11.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-valid {
  color: var(--c-ok);
  font-size: 11.5px;
  text-align: right;
}
.node-valid.is-bad {
  color: var(--c-bad);
}
@media (max-width: 1199px) {
  .dashboard-grid {
    grid-template-columns: 1fr;
  }
  .side-stack {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}
@media (max-width: 767px) {
  .stat-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .side-stack {
    grid-template-columns: 1fr;
  }
  .node-row {
    grid-template-columns: 10px minmax(0, 1fr) 68px;
  }
  .node-country,
  .node-probes {
    display: none;
  }
}
</style>
