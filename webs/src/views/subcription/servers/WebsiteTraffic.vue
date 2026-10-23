<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, onActivated, onDeactivated, reactive, ref } from "vue";
import { WebsiteTrafficState } from "@/api/proxy/websiteTraffic";
import { UiPanel, UiSkeleton, UiButton } from "@/ui";
import { formatBytes } from "@/utils/traffic";
import {
  DISPLAY_TIME_ZONE_LABEL,
  formatDateTime,
  recentHourWindow,
} from "@/utils/datetime";

const state = reactive(new WebsiteTrafficState());
const offline = ref(typeof navigator !== "undefined" && !navigator.onLine);
const disabledReason = computed(() =>
  offline.value
    ? "当前离线，请联网后刷新。"
    : state.status === "loading"
      ? "正在刷新，请稍候。"
      : ""
);
/** 默认收起，只展示前 5 条；展开后显示本次查询返回的全部排行。 */
const COLLAPSED_ROWS = 5;
const expanded = ref(false);
const visibleItems = computed(() => {
  const items = state.data?.items ?? [];
  return expanded.value ? items : items.slice(0, COLLAPSED_ROWS);
});
const hiddenCount = computed(() =>
  Math.max(0, (state.data?.items.length ?? 0) - COLLAPSED_ROWS)
);
const displayDate = (value: string) => formatDateTime(value) || "—";
async function load() {
  if (disabledReason.value) return;
  const window = recentHourWindow();
  await state.load({
    start: new Date(window.start).toISOString(),
    end: new Date(window.end).toISOString(),
    order: "total",
    limit: 100,
  });
}
const share = (total: number) => state.data?.summary.total
  ? Math.min(100, Math.max(0, total / state.data.summary.total * 100)) : 0;
let active = false;
let refreshTimer: ReturnType<typeof setTimeout> | undefined;
const REFRESH_MS = 60000;
function stopRefresh() {
  active = false;
  clearTimeout(refreshTimer);
}
function scheduleRefresh() {
  clearTimeout(refreshTimer);
  if (!active || offline.value || document.visibilityState === "hidden") return;
  refreshTimer = setTimeout(async () => {
    if (!active || offline.value || document.visibilityState === "hidden") return;
    await load();
    scheduleRefresh();
  }, REFRESH_MS);
}
function resumeRefresh() {
  active = true;
  scheduleRefresh();
  refreshIfStale();
}
function refreshIfStale() {
  if (active && !offline.value && document.visibilityState !== "hidden"
    && (!state.updatedAt || Date.now() - Date.parse(state.updatedAt) >= REFRESH_MS)) void load();
}
function connectionChanged() {
  offline.value = !navigator.onLine;
  refreshIfStale();
  scheduleRefresh();
}
function visibilityChanged() {
  refreshIfStale();
  scheduleRefresh();
}
onMounted(() => {
  window.addEventListener("online", connectionChanged);
  window.addEventListener("offline", connectionChanged);
  document.addEventListener("visibilitychange", visibilityChanged);
  resumeRefresh();
});
onActivated(resumeRefresh);
onDeactivated(stopRefresh);
onBeforeUnmount(() => {
  stopRefresh();
  state.cancel();
  document.removeEventListener("visibilitychange", visibilityChanged);
  window.removeEventListener("online", connectionChanged);
  window.removeEventListener("offline", connectionChanged);
});
</script>

<template>
  <UiPanel
    title="网站流量排行"
    description="最近 24 小时 · 全部节点 · 每分钟自动刷新"
  >
    <div class="website-traffic">
      <div class="overview-actions">
        <p class="scope-note">
          只读 · 仅代理转发 · {{ DISPLAY_TIME_ZONE_LABEL }}
        </p>
        <UiButton
          :disabled="Boolean(disabledReason)"
          :title="disabledReason"
          @click="load"
          >{{ state.status === "loading" ? "刷新中…" : "刷新" }}</UiButton
        >
      </div>
      <p v-if="disabledReason" id="website-query-reason" role="status">
        {{ disabledReason }}
      </p>
      <p v-if="offline" role="status" class="notice">
        当前离线。{{
          state.data
            ? "下方为本次会话上次成功数据，联网后请刷新。"
            : "暂无缓存数据，请联网后刷新。"
        }}
      </p>
      <p v-if="state.status === 'error'" role="alert" class="notice">
        无法加载网站流量。请检查网络、权限及后端接口是否已升级后重试。<UiButton
          :disabled="Boolean(disabledReason)"
          :title="disabledReason"
          @click="load"
          >重试</UiButton
        >
      </p>
      <div :aria-busy="state.status === 'loading'" aria-live="polite">
        <div
          v-if="state.status === 'loading' && !state.data"
          role="status"
          aria-label="正在加载网站流量"
        >
          <UiSkeleton :lines="4" />
        </div>
        <template v-if="state.data">
          <p
            v-if="state.status === 'error' || state.status === 'loading'"
            class="notice"
          >
            当前显示的是上次查询结果。
          </p>
          <p class="result-meta">
            上次查询 {{ displayDate(state.updatedAt) }} · 范围
            {{ displayDate(state.data.start) }} 至
            {{ displayDate(state.data.end) }}
          </p>
          <dl class="summary">
            <div class="summary-total">
              <dt>总流量</dt>
              <dd>{{ formatBytes(state.data.summary.total) }}</dd>
            </div>
            <div>
              <dt>上传</dt>
              <dd>{{ formatBytes(state.data.summary.up) }}</dd>
            </div>
            <div>
              <dt>下载</dt>
              <dd>{{ formatBytes(state.data.summary.down) }}</dd>
            </div>
          </dl>
          <div v-if="!state.data.items.length" class="empty" role="status">
            <strong>最近 24 小时暂无代理网站流量</strong>
            <p>开启节点采集后开始计量。</p>
          </div>
          <div v-else class="table-scroll">
            <table>
              <caption class="sr-only">
                代理转发网站流量排行（只读）
              </caption>
              <thead>
                <tr>
                  <th scope="col">排名</th>
                  <th scope="col">网站 / 服务器</th>
                  <th scope="col">上传</th>
                  <th scope="col">下载</th>
                  <th scope="col">合计 / 占总流量</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in visibleItems"
                  :key="`${item.serverId}:${item.domain}`"
                >
                  <td>{{ item.rank }}</td>
                  <td class="website-name">
                    <strong>{{
                      item.domain === state.data.unknownDomain
                        ? "未知（IP / 无法识别）"
                        : item.domain
                    }}</strong>
                    <small>{{ item.serverName || `节点 #${item.serverId}` }}</small>
                  </td>
                  <td>{{ formatBytes(item.up) }}</td>
                  <td>{{ formatBytes(item.down) }}</td>
                  <td class="traffic-total">
                    <strong>{{ formatBytes(item.total) }}</strong>
                    <small>{{ share(item.total).toFixed(1) }}%</small>
                    <span class="share-track" aria-hidden="true"><i :style="{ width: share(item.total) + '%' }"></i></span>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-if="hiddenCount" class="collapse-bar">
              {{ expanded ? `已显示 ${state.data.items.length} 条` : `已收起 ${hiddenCount} 条` }}
              <UiButton
                size="sm"
                variant="ghost"
                :aria-expanded="expanded"
                @click="expanded = !expanded"
                >{{
                  expanded
                    ? "收起，只看前 5 条"
                    : `展开全部 ${state.data.items.length} 条`
                }}</UiButton
              >
            </p>
            <p v-else-if="expanded" class="collapse-bar">
              <UiButton size="sm" variant="ghost" @click="expanded = false"
                >收起，只看前 5 条</UiButton
              >
            </p>
          </div>
          <p class="scope-note footnote">最多展示流量最高的 100 个网站与服务器组合，占比按全部流量计算。仅统计代理转发流量，开启采集后开始计量，不回填历史；IP 直连或无法识别的域名归为未知。</p>
        </template>
      </div>
    </div>
  </UiPanel>
</template>

<style scoped>
.summary > div { padding: 12px 14px; border-radius: 10px; background: var(--c-surface-2); min-width: 0; }
.summary-total dd { color: var(--c-signal); }
.website-name { min-width: 160px; max-width: 420px; }
.website-name strong { display: block; font-weight: 500; color: var(--c-ink); }
.website-name small, .traffic-total small { display: block; margin-top: 3px; color: var(--c-muted); font-size: 11px; }
.traffic-total { min-width: 120px; }
.share-track { display: block; height: 3px; margin-top: 5px; border-radius: 3px; background: var(--c-surface-2); }
.share-track i { display: block; height: 100%; border-radius: inherit; background: var(--c-signal); }
.footnote { margin-top: 14px !important; font-size: 11px !important; }
tbody tr:hover { background: var(--c-surface-2); }
@media (max-width: 600px) {
  .summary { gap: 6px !important; }
  .summary > div { padding: 10px 8px; }
  .summary dd { font-size: 17px !important; }
  th, td { padding: 10px 8px !important; }
}

.collapse-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin: 8px 0 0;
  color: var(--text-secondary, #64748b);
  font-size: 13px;
}
.website-traffic {
  display: grid;
  gap: 14px;
}
.scope-note,
.result-meta {
  margin: 0;
  color: var(--text-secondary, #64748b);
  font-size: 13px;
  line-height: 1.7;
}
.overview-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.notice {
  margin: 0;
  padding: 10px;
  border-left: 3px solid var(--el-color-warning);
  background: var(--el-fill-color-light);
  font-size: 13px;
}
.summary {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin: 16px 0;
}
.summary dt {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}
.summary dd {
  margin: 4px 0 0;
  font-size: 20px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}
.table-scroll {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
th,
td {
  padding: 12px;
  text-align: left;
  border-bottom: 1px solid var(--el-border-color-light);
  overflow-wrap: anywhere;
}
th {
  white-space: nowrap;
}
th:nth-child(n + 3),
td:nth-child(n + 3) {
  white-space: nowrap;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
.empty {
  padding: 24px 12px;
  text-align: center;
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
</style>
