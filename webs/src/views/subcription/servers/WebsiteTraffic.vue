<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, reactive, ref } from "vue";
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
  expanded.value = false;
  const window = recentHourWindow();
  await state.load({
    start: new Date(window.start).toISOString(),
    end: new Date(window.end).toISOString(),
    order: "total",
    limit: 100,
  });
}
function connectionChanged() {
  offline.value = !navigator.onLine;
}
onMounted(() => {
  window.addEventListener("online", connectionChanged);
  window.addEventListener("offline", connectionChanged);
  void load();
});
onBeforeUnmount(() => {
  state.cancel();
  window.removeEventListener("online", connectionChanged);
  window.removeEventListener("offline", connectionChanged);
});
</script>

<template>
  <UiPanel
    title="网站流量 · 仅代理转发"
    description="只读 · 按节点与域名统计代理流量。"
  >
    <div class="website-traffic">
      <p class="scope-note">
        仅统计代理转发流量，开启采集后开始计量，不回填历史。
      </p>
      <div class="overview-actions">
        <p class="scope-note">
          最近 24 小时 · {{ DISPLAY_TIME_ZONE_LABEL }} · 全部节点
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
            <div>
              <dt>上传</dt>
              <dd>{{ formatBytes(state.data.summary.up) }}</dd>
            </div>
            <div>
              <dt>下载</dt>
              <dd>{{ formatBytes(state.data.summary.down) }}</dd>
            </div>
            <div>
              <dt>合计</dt>
              <dd>{{ formatBytes(state.data.summary.total) }}</dd>
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
                  <th scope="col">服务器节点</th>
                  <th scope="col">域名</th>
                  <th scope="col">上传</th>
                  <th scope="col">下载</th>
                  <th scope="col">合计</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in visibleItems"
                  :key="`${item.serverId}:${item.domain}`"
                >
                  <td>{{ item.rank }}</td>
                  <td>{{ item.serverName || `节点 #${item.serverId}` }}</td>
                  <td>
                    {{
                      item.domain === state.data.unknownDomain
                        ? "未知（IP / 无法识别）"
                        : item.domain
                    }}
                  </td>
                  <td>{{ formatBytes(item.up) }}</td>
                  <td>{{ formatBytes(item.down) }}</td>
                  <td>{{ formatBytes(item.total) }}</td>
                </tr>
              </tbody>
            </table>
            <p v-if="hiddenCount" class="collapse-bar">
              已收起 {{ hiddenCount }} 条
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
        </template>
      </div>
    </div>
  </UiPanel>
</template>

<style scoped>
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
  display: flex;
  flex-wrap: wrap;
  gap: 32px;
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
td:nth-child(n + 4) {
  white-space: nowrap;
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
