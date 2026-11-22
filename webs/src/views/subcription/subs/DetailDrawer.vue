<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { Search } from "@element-plus/icons-vue";
import {
  UiDrawer,
  UiButton,
  UiSkeleton,
  StatusDot,
  EmptyState,
} from "@/ui";
import { countryFlag } from "@/utils/flag";
import { useSubscriptions } from "./useSubscriptions";

const {
  drawerVisible,
  drawerSub,
  drawerLoading,
  drawerGroupRefs,
  drawerNodesByCountry,
  previewNodesList,
  previewExcluded,
  detailSection,
  previewError,
  openDrawer,
  parseConfig,
  validLabel,
  expireClear,
  expirePick,
  expireSaving,
  saveExpire,
} = useSubscriptions();

const sections = [
  { label: "节点清单", value: "nodes" },
  { label: "模板与协议", value: "config" },
  { label: "有效期", value: "expire" },
] as const;

const nodeQuery = ref("");
const activeCountry = ref("");
const nodeScope = ref<"all" | "ok" | "excluded">("all");

watch(
  () => [drawerVisible.value, drawerSub.value?.ID],
  () => {
    nodeQuery.value = "";
    activeCountry.value = "";
    nodeScope.value = "all";
  }
);

const okCount = computed(() =>
  drawerNodesByCountry.value.reduce(
    (sum, g) => sum + g.items.filter((n) => n.valid === true).length,
    0
  )
);

const filteredGroups = computed(() => {
  const q = nodeQuery.value.trim().toLowerCase();
  return drawerNodesByCountry.value
    .filter((g) => !activeCountry.value || g.country === activeCountry.value)
    .map((g) => {
      const items = g.items.filter((node) => {
        if (nodeScope.value === "ok" && node.valid !== true) return false;
        if (!q) return true;
        return (
          node.name.toLowerCase().includes(q) ||
          (node.server || "").toLowerCase().includes(q) ||
          g.country.toLowerCase().includes(q)
        );
      });
      return { ...g, items };
    })
    .filter((g) => g.items.length > 0);
});

const filteredExcluded = computed(() => {
  const q = nodeQuery.value.trim().toLowerCase();
  if (!q) return previewExcluded.value;
  return previewExcluded.value.filter(
    (item) =>
      item.name.toLowerCase().includes(q) ||
      (item.reason || "").toLowerCase().includes(q)
  );
});

const parsedConfig = computed(() =>
  drawerSub.value ? parseConfig(drawerSub.value.Config) : { clash: "", surge: "", loon: "", udp: false, cert: false }
);

const configRows = computed(() => [
  { label: "Clash 模板", value: parsedConfig.value.clash || "未配置", muted: !parsedConfig.value.clash, mono: true },
  { label: "Surge 模板", value: parsedConfig.value.surge || "未配置", muted: !parsedConfig.value.surge, mono: true },
  { label: "Loon 模板", value: parsedConfig.value.loon || "未配置", muted: !parsedConfig.value.loon, mono: true },
  { label: "UDP 转发", value: parsedConfig.value.udp ? "已开启" : "已关闭", active: !!parsedConfig.value.udp, toggle: true },
  { label: "跳过证书校验", value: parsedConfig.value.cert ? "已开启" : "已关闭", active: !!parsedConfig.value.cert, toggle: true },
]);

const expirePresets = [
  { label: "永不过期", days: 0 },
  { label: "+7 天", days: 7 },
  { label: "+30 天", days: 30 },
  { label: "+90 天", days: 90 },
  { label: "+1 年", days: 365 },
];

function applyExpirePreset(days: number) {
  if (days === 0) {
    expireClear.value = true;
    expirePick.value = "";
    return;
  }
  expireClear.value = false;
  const target = new Date(Date.now() + days * 24 * 3600 * 1000);
  const pad = (n: number) => String(n).padStart(2, "0");
  expirePick.value = `${target.getFullYear()}-${pad(target.getMonth() + 1)}-${pad(target.getDate())} ${pad(target.getHours())}:${pad(target.getMinutes())}`;
}
</script>

<template>
  <UiDrawer
    v-model="drawerVisible"
    title="订阅详情"
    size="min(860px, 100vw)"
  >
    <div v-if="drawerSub" class="sub-detail">
      <!-- Pure Gallery Masthead -->
      <header class="detail-head">
        <div class="detail-title-row">
          <h2 class="detail-name">{{ drawerSub.Name }}</h2>
        </div>

        <div class="detail-metrics">
          <div class="metric-item">
            <span class="metric-label">下发节点</span>
            <strong class="metric-val mono">{{ previewNodesList.length }}</strong>
          </div>
          <div class="metric-item">
            <span class="metric-label">覆盖地区</span>
            <strong class="metric-val mono">{{ drawerNodesByCountry.length }}</strong>
          </div>
          <div v-if="previewExcluded.length" class="metric-item is-warn">
            <span class="metric-label">暂不下发</span>
            <strong class="metric-val mono">{{ previewExcluded.length }}</strong>
          </div>
        </div>
      </header>

      <!-- Editorial Underline Tabs -->
      <nav class="detail-tabs" role="tablist" aria-label="订阅详情分区">
        <button
          v-for="tab in sections"
          :key="tab.value"
          type="button"
          role="tab"
          class="detail-tab"
          :class="{ 'is-active': detailSection === tab.value }"
          :aria-selected="detailSection === tab.value"
          @click="detailSection = tab.value"
        >
          <span>{{ tab.label }}</span>
          <span v-if="tab.value === 'nodes'" class="tab-count mono">
            {{ previewNodesList.length }}
          </span>
        </button>
      </nav>

      <!-- Section 1: Nodes -->
      <section v-if="detailSection === 'nodes'" class="detail-panel">
        <UiSkeleton v-if="drawerLoading" :lines="6" height="48px" />

        <EmptyState
          v-else-if="previewError"
          size="section"
          title="节点加载失败"
          :description="previewError"
        >
          <template #action>
            <UiButton @click="openDrawer(drawerSub)">重新加载</UiButton>
          </template>
        </EmptyState>

        <template v-else-if="previewNodesList.length || previewExcluded.length">
          <!-- Calm Single-Row Search & Scope Filter -->
          <div class="node-toolbar">
            <label class="search-field">
              <el-icon class="search-ico" aria-hidden="true"><Search /></el-icon>
              <input
                v-model="nodeQuery"
                type="search"
                class="search-input"
                placeholder="搜索节点名称、主机或地区…"
                aria-label="搜索订阅节点"
              />
            </label>

            <div class="scope-pills" role="group" aria-label="节点状态筛选">
              <button
                type="button"
                class="scope-pill"
                :class="{ 'is-active': nodeScope === 'all' }"
                @click="nodeScope = 'all'"
              >
                全部 <span class="mono">{{ previewNodesList.length }}</span>
              </button>
              <button
                v-if="okCount > 0"
                type="button"
                class="scope-pill"
                :class="{ 'is-active': nodeScope === 'ok' }"
                @click="nodeScope = 'ok'"
              >
                正常 <span class="mono">{{ okCount }}</span>
              </button>
              <button
                v-if="previewExcluded.length"
                type="button"
                class="scope-pill is-warn"
                :class="{ 'is-active': nodeScope === 'excluded' }"
                @click="nodeScope = 'excluded'"
              >
                暂不下发 <span class="mono">{{ previewExcluded.length }}</span>
              </button>
            </div>
          </div>

          <!-- Region Filter Rail (only in normal node view with >1 country) -->
          <div
            v-if="nodeScope !== 'excluded' && drawerNodesByCountry.length > 1"
            class="region-rail"
            role="group"
            aria-label="按地区筛选"
          >
            <button
              type="button"
              class="region-pill"
              :class="{ 'is-active': activeCountry === '' }"
              @click="activeCountry = ''"
            >
              全部地区
              <span class="mono">{{ previewNodesList.length }}</span>
            </button>
            <button
              v-for="group in drawerNodesByCountry"
              :key="group.country"
              type="button"
              class="region-pill"
              :class="{ 'is-active': activeCountry === group.country }"
              @click="activeCountry = activeCountry === group.country ? '' : group.country"
            >
              <span>{{ countryFlag(group.items[0].countryCode) }}</span>
              <span>{{ group.country }}</span>
              <span class="mono">{{ group.items.length }}</span>
            </button>
          </div>

          <!-- Excluded Nodes List -->
          <template v-if="nodeScope === 'excluded'">
            <div v-if="filteredExcluded.length" class="excluded-list">
              <div
                v-for="item in filteredExcluded"
                :key="item.id"
                class="excluded-row"
              >
                <strong class="excluded-name">{{ item.name }}</strong>
                <span class="excluded-reason">
                  {{ item.reason
                  }}{{ item.until ? ` · ${item.until.slice(5, 10)} 恢复` : "" }}
                </span>
              </div>
            </div>
            <EmptyState
              v-else
              size="section"
              title="没有匹配的暂不下发节点"
            />
          </template>

          <!-- Spacious 2-Column Grouped Node Cards -->
          <template v-else>
            <div v-if="filteredGroups.length" class="country-stack">
              <section
                v-for="group in filteredGroups"
                :key="group.country"
                class="country-block"
              >
                <header class="country-head">
                  <h3 class="country-title">
                    <span class="flag">{{ countryFlag(group.items[0].countryCode) }}</span>
                    <span>{{ group.country }}</span>
                  </h3>
                  <span class="country-count mono">{{ group.items.length }} 节点</span>
                </header>

                <div
                  class="node-grid"
                  :class="{ 'is-single': group.items.length === 1 }"
                >
                  <article
                    v-for="node in group.items"
                    :key="node.id"
                    class="node-tile"
                  >
                    <div class="tile-main">
                      <strong class="tile-name" :title="node.name">
                        {{ node.name }}
                      </strong>
                      <span
                        v-if="node.server"
                        class="tile-host mono"
                        :title="node.server"
                      >
                        {{ node.server }}
                      </span>
                    </div>
                    <StatusDot
                      class="tile-status"
                      :state="
                        node.valid === undefined
                          ? 'idle'
                          : node.valid
                            ? 'ok'
                            : 'bad'
                      "
                      :label="validLabel(node)"
                    />
                  </article>
                </div>
              </section>
            </div>

            <EmptyState
              v-else
              size="section"
              title="没有匹配的节点"
              description="尝试清空搜索关键词或切换地区筛选。"
            >
              <template #action>
                <UiButton
                  variant="ghost"
                  @click="
                    nodeQuery = '';
                    activeCountry = '';
                    nodeScope = 'all';
                  "
                >
                  重置筛选
                </UiButton>
              </template>
            </EmptyState>
          </template>
        </template>

        <EmptyState
          v-else
          size="section"
          title="暂无节点"
          description="编辑订阅，添加节点或关联分组。"
        />
      </section>

      <!-- Section 2: Template & Protocol Spec Sheet -->
      <section v-if="detailSection === 'config'" class="detail-panel">
        <dl class="spec-sheet">
          <div v-for="row in configRows" :key="row.label" class="spec-row">
            <dt class="spec-key">{{ row.label }}</dt>
            <dd class="spec-val">
              <span
                v-if="row.toggle"
                class="spec-badge"
                :class="{ 'is-on': row.active }"
              >
                {{ row.value }}
              </span>
              <span
                v-else
                :class="{ mono: row.mono, 'is-muted': row.muted }"
              >
                {{ row.value }}
              </span>
            </dd>
          </div>
        </dl>
      </section>

      <!-- Section 3: Expiry Settings -->
      <section v-if="detailSection === 'expire'" class="detail-panel">
        <div class="expire-card">
          <div class="expire-presets" role="group" aria-label="快捷有效期预设">
            <button
              v-for="preset in expirePresets"
              :key="preset.label"
              type="button"
              class="preset-pill"
              :class="{ 'is-active': preset.days === 0 && expireClear }"
              @click="applyExpirePreset(preset.days)"
            >
              {{ preset.label }}
            </button>
          </div>

          <el-form
            label-position="top"
            class="expire-form"
            @submit.prevent="saveExpire"
          >
            <el-form-item label="到期时间">
              <el-date-picker
                v-model="expirePick"
                type="datetime"
                format="YYYY-MM-DD HH:mm"
                value-format="YYYY-MM-DD HH:mm"
                :disabled="expireClear"
                placeholder="选择具体过期时间（或上方选择永不过期）"
                class="expire-picker"
              />
            </el-form-item>

            <div class="expire-actions">
              <UiButton
                variant="primary"
                :loading="expireSaving"
                @click="saveExpire"
              >
                保存有效期设置
              </UiButton>
            </div>
          </el-form>
        </div>
      </section>
    </div>
  </UiDrawer>
</template>

<style scoped>
.sub-detail {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding-bottom: 24px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

/* Masthead */
.detail-head {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.detail-title-row {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.detail-name {
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  line-height: 1.25;
  letter-spacing: -0.025em;
  color: var(--c-ink);
  overflow-wrap: anywhere;
}

.source-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.source-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.source-pill em {
  font-style: normal;
  color: var(--c-ink);
  font-weight: 600;
}

.detail-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 12px 20px;
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
}

.metric-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.metric-label {
  font-size: 12px;
  color: var(--c-muted);
}

.metric-val {
  font-size: 16px;
  font-weight: 600;
  color: var(--c-ink);
}

.metric-item.is-warn .metric-val {
  color: var(--c-warn-ink, #b45309);
}

/* Tabs */
.detail-tabs {
  display: flex;
  align-items: center;
  gap: 24px;
  border-bottom: 1px solid var(--c-line);
}

.detail-tab {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 0 0 12px;
  border: none;
  background: transparent;
  font-size: 14px;
  font-weight: 500;
  color: var(--c-muted);
  cursor: pointer;
  transition: color 0.18s ease;
}

.detail-tab:hover {
  color: var(--c-ink);
}

.detail-tab.is-active {
  color: var(--c-ink);
  font-weight: 600;
}

.detail-tab.is-active::after {
  content: "";
  position: absolute;
  left: 0;
  right: 0;
  bottom: -1px;
  height: 2px;
  background: var(--c-ink);
  border-radius: 2px;
}

.tab-count {
  font-size: 12px;
  color: var(--c-muted);
}

/* Panel */
.detail-panel {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

/* Node Toolbar */
.node-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px 16px;
}

.search-field {
  flex: 1 1 220px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
  transition: border-color 0.18s ease;
}

.search-field:focus-within {
  border-color: var(--c-ink);
}

.search-ico {
  flex: none;
  color: var(--c-muted);
  font-size: 15px;
}

.search-input {
  width: 100%;
  border: none;
  outline: none;
  background: transparent;
  font-size: 13.5px;
  color: var(--c-ink);
}

.search-input::placeholder {
  color: var(--c-muted);
}

.scope-pills {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}

.scope-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: transparent;
  font-size: 13px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.18s ease;
}

.scope-pill:hover {
  color: var(--c-ink);
  border-color: color-mix(in srgb, var(--c-ink) 30%, transparent);
}

.scope-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.scope-pill.is-warn:not(.is-active) {
  color: var(--c-warn-ink, #b45309);
  border-color: color-mix(in srgb, var(--c-warn, #f59e0b) 35%, transparent);
}

/* Region Rail */
.region-rail {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.region-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 11px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.18s ease;
}

.region-pill:hover {
  color: var(--c-ink);
  border-color: color-mix(in srgb, var(--c-ink) 28%, transparent);
}

.region-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

/* Country Blocks & Spacious 2-Col Node Grid */
.country-stack {
  display: flex;
  flex-direction: column;
  gap: 26px;
}

.country-block {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.country-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
}

.country-title {
  margin: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.country-count {
  font-size: 12px;
  color: var(--c-muted);
}

.node-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.node-grid.is-single {
  grid-template-columns: minmax(0, 1fr);
}

.node-tile {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 13px 15px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
  transition: border-color 0.18s ease, background 0.18s ease;
}

.node-tile:hover {
  border-color: color-mix(in srgb, var(--c-ink) 24%, transparent);
}

.tile-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.tile-name {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tile-host {
  font-size: 11.5px;
  color: var(--c-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.tile-status {
  flex: none;
}

/* Excluded list */
.excluded-list {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--c-line);
}

.excluded-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 4px;
  border-bottom: 1px solid var(--c-line);
}

.excluded-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.excluded-reason {
  font-size: 13px;
  color: var(--c-warn-ink, #b45309);
  text-align: right;
}

/* Config Spec Sheet */
.spec-sheet {
  margin: 0;
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--c-line);
}

.spec-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 16px 4px;
  border-bottom: 1px solid var(--c-line);
}

.spec-key {
  font-size: 14px;
  color: var(--c-muted);
}

.spec-val {
  margin: 0;
  font-size: 14px;
  color: var(--c-ink);
  text-align: right;
  overflow-wrap: anywhere;
}

.spec-val .is-muted {
  color: var(--c-muted);
}

.spec-badge {
  display: inline-flex;
  align-items: center;
  padding: 3px 10px;
  border-radius: 999px;
  font-size: 12.5px;
  font-weight: 500;
  background: var(--c-surface-2, rgba(0, 0, 0, 0.04));
  color: var(--c-muted);
}

.spec-badge.is-on {
  background: color-mix(in srgb, #10b981 14%, transparent);
  color: #059669;
}

/* Expire Card */
.expire-card {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.expire-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.preset-pill {
  padding: 7px 14px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 13px;
  color: var(--c-ink);
  cursor: pointer;
  transition: all 0.18s ease;
}

.preset-pill:hover {
  border-color: color-mix(in srgb, var(--c-ink) 35%, transparent);
}

.preset-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.expire-form :deep(.el-date-editor.el-input) {
  width: 100%;
  max-width: 360px;
}

.expire-actions {
  margin-top: 8px;
}

@media (max-width: 640px) {
  .node-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .excluded-row,
  .spec-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 6px;
  }
  .excluded-reason,
  .spec-val {
    text-align: left;
  }
}
</style>
