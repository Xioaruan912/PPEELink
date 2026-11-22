<script setup lang="ts">
import { computed, ref } from "vue";
import { Search, Check } from "@element-plus/icons-vue";
import { EmptyState } from "@/ui";
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
  groupedNodes,
  removeNode,
  selectAllNodes,
  selectGroupNodes,
  clearAllNodes,
  moveNode,
} = useSubscriptions();

const nodeQuery = ref("");
const activeGroup = ref("");

function toggleGroup(id: number) {
  const idx = selectedGroups.value.indexOf(id);
  if (idx >= 0) {
    selectedGroups.value.splice(idx, 1);
  } else {
    selectedGroups.value.push(id);
  }
}

function toggleNode(name: string) {
  if (value1.value.includes(name)) {
    removeNode(name);
  } else {
    value1.value.push(name);
  }
}

const visibleNodes = computed(() => {
  const q = nodeQuery.value.trim().toLowerCase();
  const groups = activeGroup.value
    ? groupedNodes.value.filter((g) => g.name === activeGroup.value)
    : groupedNodes.value;
  const seen = new Set<string>();
  const list: typeof NodesList.value = [];
  for (const g of groups) {
    for (const n of g.nodes) {
      if (seen.has(n.Name)) continue;
      if (q && !n.Name.toLowerCase().includes(q)) continue;
      seen.add(n.Name);
      list.push(n);
    }
  }
  return list;
});

const activeGroupObj = computed(() =>
  groupedNodes.value.find((g) => g.name === activeGroup.value)
);
</script>

<template>
  <div class="sources-layout">
    <!-- Block 1: Dynamic Node Groups -->
    <section class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">关联动态分组</h3>
          <p class="block-sub">
            勾选后拉取订阅时自动跟进分组内的最新节点
          </p>
        </div>
        <span v-if="selectedGroups.length" class="block-pill mono">
          已选 {{ selectedGroups.length }} 组
        </span>
      </header>

      <div v-if="allGroups.length" class="group-grid">
        <button
          v-for="g in allGroups"
          :key="g.ID"
          type="button"
          class="group-card"
          :class="{ 'is-selected': selectedGroups.includes(g.ID) }"
          :aria-pressed="selectedGroups.includes(g.ID)"
          @click="toggleGroup(g.ID)"
        >
          <div class="group-card-main">
            <strong class="group-card-name">{{ g.Name }}</strong>
            <span class="group-card-count mono">{{ g.NodeCount }} 节点</span>
          </div>
          <span class="check-circle" aria-hidden="true">
            <el-icon v-if="selectedGroups.includes(g.ID)"><Check /></el-icon>
          </span>
        </button>
      </div>

      <EmptyState
        v-else
        size="section"
        title="暂无节点分组"
        description="创建分组后可自动同步节点变化。"
      />
    </section>

    <!-- Block 2: Fixed Nodes Picker -->
    <section class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">固定节点</h3>
          <p class="block-sub">按需勾选额外固定下发的单个节点</p>
        </div>
        <div class="head-actions">
          <button
            v-if="activeGroupObj"
            type="button"
            class="text-action"
            @click="selectGroupNodes(activeGroupObj.nodes)"
          >
            全选本组
          </button>
          <button
            v-else-if="NodesList.length"
            type="button"
            class="text-action"
            @click="selectAllNodes"
          >
            全选
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

      <template v-if="NodesList.length">
        <div class="picker-toolbar">
          <label class="search-box">
            <el-icon class="search-ico" aria-hidden="true"><Search /></el-icon>
            <input
              v-model="nodeQuery"
              type="search"
              class="search-input"
              placeholder="搜索节点名称…"
              aria-label="搜索固定节点"
            />
          </label>

          <div
            v-if="groupedNodes.length > 1"
            class="group-filter-rail"
            role="group"
            aria-label="按分组筛选节点"
          >
            <button
              type="button"
              class="filter-pill"
              :class="{ 'is-active': activeGroup === '' }"
              @click="activeGroup = ''"
            >
              全部 <span class="mono">{{ NodesList.length }}</span>
            </button>
            <button
              v-for="g in groupedNodes"
              :key="g.name"
              type="button"
              class="filter-pill"
              :class="{ 'is-active': activeGroup === g.name }"
              @click="activeGroup = activeGroup === g.name ? '' : g.name"
            >
              {{ g.name }} <span class="mono">{{ g.nodes.length }}</span>
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
      </template>

      <EmptyState
        v-else
        size="section"
        title="暂无节点"
        description="添加节点后可在此选择。"
      />
    </section>

    <!-- Block 3: Reorder Selected Fixed Nodes (only when > 0) -->
    <section v-if="value1.length" class="source-block">
      <header class="block-head">
        <div>
          <h3 class="block-title">已选固定节点顺序</h3>
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
  gap: 28px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
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
  transition: opacity 0.16s ease;
}

.text-action:hover {
  opacity: 1;
}

.text-action.is-danger {
  color: #dc2626;
}

/* Dynamic Groups Grid */
.group-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(210px, 1fr));
  gap: 10px;
}

.group-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.18s ease, background 0.18s ease;
}

.group-card:hover {
  border-color: color-mix(in srgb, var(--c-ink) 32%, transparent);
}

.group-card.is-selected {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ink) 4%, var(--c-surface));
}

.group-card-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.group-card-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.group-card-count {
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
  transition: all 0.16s ease;
}

.group-card.is-selected .check-circle {
  background: var(--c-ink);
  border-color: var(--c-ink);
}

/* Fixed Node Picker */
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

.group-filter-rail {
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
  transition: all 0.16s ease;
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
  transition: border-color 0.16s ease, background 0.16s ease;
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

/* Reorder Stack */
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
