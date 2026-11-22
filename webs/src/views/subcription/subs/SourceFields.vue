<script setup lang="ts">
import { computed, ref } from "vue";
import { Search, Check } from "@element-plus/icons-vue";
import { EmptyState } from "@/ui";
import { countryFlag } from "@/utils/flag";
import { useSubscriptions } from "./useSubscriptions";
import { VueDraggable } from "vue-draggable-plus";
import { usePrefersReducedMotion } from "@/motion";

const reduced = usePrefersReducedMotion();
const {
  NodesList,
  value1,
  allGroups,
  selectedGroups,
  nodeQuality,
  nodeQualityByName,
  validLabel,
  removeNode,
  selectAllNodes,
  clearAllNodes,
  moveNode,
} = useSubscriptions();

const nodeQuery = ref("");
const activeCountry = ref("");

interface CountryBucket {
  country: string;
  code: string;
  nodes: typeof NodesList.value;
}

const countryBuckets = computed<CountryBucket[]>(() => {
  const map = new Map<string, CountryBucket>();
  for (const n of NodesList.value) {
    const q = nodeQuality(n);
    const country = q?.country || "未知地区";
    const code = q?.countryCode || "";
    let bucket = map.get(country);
    if (!bucket) {
      bucket = { country, code, nodes: [] };
      map.set(country, bucket);
    }
    bucket.nodes.push(n);
  }
  return [...map.values()].sort(
    (a, b) => b.nodes.length - a.nodes.length || a.country.localeCompare(b.country, "zh-CN")
  );
});

function countrySelectedCount(bucket: CountryBucket) {
  const picked = new Set(value1.value);
  return bucket.nodes.filter((n) => picked.has(n.Name)).length;
}

function toggleCountryAll(bucket: CountryBucket) {
  const picked = new Set(value1.value);
  const allPicked = bucket.nodes.every((n) => picked.has(n.Name));
  if (allPicked) {
    const toRemove = new Set(bucket.nodes.map((n) => n.Name));
    value1.value = value1.value.filter((name) => !toRemove.has(name));
  } else {
    for (const n of bucket.nodes) {
      if (!picked.has(n.Name)) {
        value1.value.push(n.Name);
        picked.add(n.Name);
      }
    }
  }
}

function toggleNode(name: string) {
  if (value1.value.includes(name)) {
    removeNode(name);
  } else {
    value1.value.push(name);
  }
}

function flattenLegacyGroups() {
  const groupNames = new Set(
    allGroups.value
      .filter((g) => selectedGroups.value.includes(g.ID))
      .map((g) => g.Name)
  );
  const picked = new Set(value1.value);
  for (const n of NodesList.value) {
    const inLegacy = (n.GroupNodes || []).some((g) => groupNames.has(g.Name));
    if (inLegacy && !picked.has(n.Name)) {
      value1.value.push(n.Name);
      picked.add(n.Name);
    }
  }
  selectedGroups.value = [];
}

const visibleNodes = computed(() => {
  const q = nodeQuery.value.trim().toLowerCase();
  const list = activeCountry.value
    ? countryBuckets.value.find((b) => b.country === activeCountry.value)?.nodes || []
    : NodesList.value;
  if (!q) return list;
  return list.filter((n) => {
    const meta = nodeQuality(n);
    return (
      n.Name.toLowerCase().includes(q) ||
      (meta?.country || "").toLowerCase().includes(q)
    );
  });
});
</script>

<template>
  <div class="sources-layout">
    <!-- Legacy group compatibility notice (only for old subscriptions that still have GroupRefs) -->
    <div v-if="selectedGroups.length" class="legacy-banner">
      <span>
        该订阅仍保留 {{ selectedGroups.length }} 个旧版手动分组绑定，可一键转换为节点清单。
      </span>
      <button type="button" class="legacy-btn" @click="flattenLegacyGroups">
        展平并移除旧分组
      </button>
    </div>

    <!-- Block 1: Country / Region Quick Select -->
    <section class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">按国家 / 地区快选</h3>
          <p class="block-sub">点击地区卡片一键勾选或取消该地区全部节点</p>
        </div>
        <div class="head-actions">
          <button
            v-if="NodesList.length"
            type="button"
            class="text-action"
            @click="selectAllNodes"
          >
            全选全部
          </button>
          <button
            v-if="value1.length"
            type="button"
            class="text-action is-danger"
            @click="clearAllNodes"
          >
            清空已选 ({{ value1.length }})
          </button>
        </div>
      </header>

      <div v-if="countryBuckets.length" class="country-grid">
        <button
          v-for="bucket in countryBuckets"
          :key="bucket.country"
          type="button"
          class="country-card"
          :class="{
            'is-selected':
              bucket.nodes.length > 0 &&
              countrySelectedCount(bucket) === bucket.nodes.length,
            'is-partial':
              countrySelectedCount(bucket) > 0 &&
              countrySelectedCount(bucket) < bucket.nodes.length,
          }"
          @click="toggleCountryAll(bucket)"
        >
          <div class="country-card-main">
            <strong class="country-card-name">
              <span>{{ countryFlag(bucket.code) }}</span>
              <span>{{ bucket.country }}</span>
            </strong>
            <span class="country-card-count mono">
              已选 {{ countrySelectedCount(bucket) }} / {{ bucket.nodes.length }}
            </span>
          </div>
          <span class="check-circle" aria-hidden="true">
            <el-icon
              v-if="
                bucket.nodes.length > 0 &&
                countrySelectedCount(bucket) === bucket.nodes.length
              "
            >
              <Check />
            </el-icon>
          </span>
        </button>
      </div>

      <EmptyState
        v-else
        size="section"
        title="暂无可用节点"
        description="添加节点后可按国家/地区一键勾选。"
      />
    </section>

    <!-- Block 2: Individual Node Picker -->
    <section v-if="NodesList.length" class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">按单个节点微调</h3>
          <p class="block-sub">支持按名称或地区检索，精确增删节点</p>
        </div>
      </header>

      <div class="picker-toolbar">
        <label class="search-box">
          <el-icon class="search-ico" aria-hidden="true"><Search /></el-icon>
          <input
            v-model="nodeQuery"
            type="search"
            class="search-input"
            placeholder="搜索节点名称或国家/地区…"
            aria-label="搜索节点"
          />
        </label>

        <div
          v-if="countryBuckets.length > 1"
          class="country-filter-rail"
          role="group"
          aria-label="按地区筛选节点"
        >
          <button
            type="button"
            class="filter-pill"
            :class="{ 'is-active': activeCountry === '' }"
            @click="activeCountry = ''"
          >
            全部 <span class="mono">{{ NodesList.length }}</span>
          </button>
          <button
            v-for="bucket in countryBuckets"
            :key="bucket.country"
            type="button"
            class="filter-pill"
            :class="{ 'is-active': activeCountry === bucket.country }"
            @click="
              activeCountry =
                activeCountry === bucket.country ? '' : bucket.country
            "
          >
            {{ countryFlag(bucket.code) }} {{ bucket.country }}
            <span class="mono">{{ bucket.nodes.length }}</span>
          </button>
        </div>
      </div>

      <div v-if="visibleNodes.length" class="node-pick-grid">
        <button
          v-for="item in visibleNodes"
          :key="item.Name"
          type="button"
          class="node-pick-card"
          :class="{ 'is-selected': value1.includes(item.Name) }"
          :aria-pressed="value1.includes(item.Name)"
          @click="toggleNode(item.Name)"
        >
          <span
            class="dot"
            :class="
              nodeQuality(item)?.valid === undefined
                ? 'is-idle'
                : nodeQuality(item)?.valid
                  ? 'is-ok'
                  : 'is-bad'
            "
            aria-hidden="true"
          ></span>
          <span class="pick-name" :title="item.Name">{{ item.Name }}</span>
          <span class="pick-check" aria-hidden="true">
            <el-icon v-if="value1.includes(item.Name)"><Check /></el-icon>
          </span>
        </button>
      </div>

      <p v-else class="empty-inline">没有匹配的节点，请调整搜索词。</p>
    </section>

    <!-- Block 3: Reorder Selected Nodes -->
    <section v-if="value1.length" class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">已选节点顺序</h3>
          <p class="block-sub">支持拖拽或点击调整下发顺序</p>
        </div>
        <span class="block-pill mono">{{ value1.length }} 个</span>
      </header>

      <VueDraggable
        v-model="value1"
        :animation="reduced ? 0 : 150"
        ghost-class="drag-ghost"
        class="order-stack"
      >
        <div
          v-for="(nodeName, index) in value1"
          :key="nodeName"
          class="order-row"
        >
          <span class="order-idx mono">{{ index + 1 }}</span>
          <span class="order-title" :title="nodeName">{{ nodeName }}</span>
          <span
            class="order-state"
            :class="{ 'is-ok': nodeQualityByName(nodeName)?.valid }"
          >
            {{ validLabel(nodeQualityByName(nodeName)) }}
          </span>
          <div class="order-ops">
            <button
              type="button"
              class="op-btn"
              :disabled="index === 0"
              :aria-label="`上移 ${nodeName}`"
              @click="moveNode(index, -1)"
            >
              ↑
            </button>
            <button
              type="button"
              class="op-btn"
              :disabled="index === value1.length - 1"
              :aria-label="`下移 ${nodeName}`"
              @click="moveNode(index, 1)"
            >
              ↓
            </button>
            <button
              type="button"
              class="op-btn is-del"
              :aria-label="`移除 ${nodeName}`"
              @click="removeNode(nodeName)"
            >
              ×
            </button>
          </div>
        </div>
      </VueDraggable>
    </section>
  </div>
</template>

<style scoped>
.sources-layout {
  display: flex;
  flex-direction: column;
  gap: 26px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

.legacy-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  padding: 12px 15px;
  border-radius: 10px;
  border: 1px solid color-mix(in srgb, #f59e0b 35%, transparent);
  background: color-mix(in srgb, #f59e0b 8%, var(--c-surface));
  font-size: 13px;
  color: var(--c-ink);
}

.legacy-btn {
  padding: 5px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-ink);
  background: var(--c-ink);
  color: var(--c-surface);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
}

.source-block {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.block-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px 16px;
}

.block-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}

.block-sub {
  margin: 3px 0 0;
  font-size: 12.5px;
  color: var(--c-muted);
}

.block-pill {
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--c-surface-2, rgba(0, 0, 0, 0.05));
  font-size: 12px;
  color: var(--c-ink);
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 14px;
}

.text-action {
  padding: 4px 2px;
  border: none;
  background: transparent;
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
  cursor: pointer;
  opacity: 0.78;
}

.text-action:hover {
  opacity: 1;
}

.text-action.is-danger {
  color: #dc2626;
}

.country-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(195px, 1fr));
  gap: 10px;
}

.country-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 13px 15px;
  border-radius: 12px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease;
}

.country-card:hover {
  border-color: color-mix(in srgb, var(--c-ink) 32%, transparent);
}

.country-card.is-selected {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ink) 4%, var(--c-surface));
}

.country-card.is-partial {
  border-color: color-mix(in srgb, var(--c-ink) 45%, transparent);
}

.country-card-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.country-card-name {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.country-card-count {
  font-size: 12px;
  color: var(--c-muted);
}

.check-circle {
  flex: none;
  width: 20px;
  height: 20px;
  border-radius: 999px;
  border: 1.5px solid var(--c-line);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  color: var(--c-surface);
}

.country-card.is-selected .check-circle {
  background: var(--c-ink);
  border-color: var(--c-ink);
}

.picker-toolbar {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.search-box {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
}

.search-box:focus-within {
  border-color: var(--c-ink);
}

.search-ico {
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

.country-filter-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.filter-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: transparent;
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
}

.filter-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.node-pick-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
  padding-right: 2px;
}

.node-pick-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 13px;
  border-radius: 10px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
}

.node-pick-card:hover {
  border-color: color-mix(in srgb, var(--c-ink) 30%, transparent);
}

.node-pick-card.is-selected {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ink) 4%, var(--c-surface));
}

.dot {
  flex: none;
  width: 7px;
  height: 7px;
  border-radius: 999px;
}

.dot.is-ok {
  background: #10b981;
}
.dot.is-bad {
  background: #ef4444;
}
.dot.is-idle {
  background: var(--c-muted);
  opacity: 0.45;
}

.pick-name {
  flex: 1;
  min-width: 0;
  font-size: 13px;
  color: var(--c-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pick-check {
  flex: none;
  width: 16px;
  height: 16px;
  border-radius: 4px;
  border: 1.5px solid var(--c-line);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: var(--c-surface);
}

.node-pick-card.is-selected .pick-check {
  background: var(--c-ink);
  border-color: var(--c-ink);
}

.empty-inline {
  margin: 0;
  padding: 18px 0;
  text-align: center;
  font-size: 13px;
  color: var(--c-muted);
}

.order-stack {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--c-line);
  max-height: 260px;
  overflow-y: auto;
}

.order-row {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 4px;
  border-bottom: 1px solid var(--c-line);
  cursor: grab;
}

.order-idx {
  width: 22px;
  font-size: 12px;
  color: var(--c-muted);
}

.order-title {
  flex: 1;
  min-width: 0;
  font-size: 13.5px;
  color: var(--c-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.order-state {
  font-size: 12px;
  color: var(--c-muted);
}

.order-state.is-ok {
  color: #059669;
}

.order-ops {
  display: inline-flex;
  gap: 4px;
}

.op-btn {
  width: 26px;
  height: 26px;
  border-radius: 6px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 13px;
  cursor: pointer;
}

.op-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.op-btn.is-del:hover {
  border-color: #ef4444;
  color: #dc2626;
}

.drag-ghost {
  opacity: 0.45;
  background: var(--c-surface-2);
}

@media (max-width: 640px) {
  .node-pick-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
