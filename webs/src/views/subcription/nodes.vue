<script setup lang="ts">
import { ref, nextTick, watch } from "vue";
import { Plus, Refresh } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiSegmented,
  UiStat,
  UiSkeleton,
  EmptyState,
  StatusDot,
} from "@/ui";
import { useStagger, useMotion } from "@/motion";
import NodeUnlockDialog from "@/components/NodeUnlockDialog.vue";
import NodeTcpDialog from "@/components/NodeTcpDialog.vue";
import NodeQualityDialog from "@/components/NodeQualityDialog.vue";
import { provideNodes } from "./nodes/useNodes";
import NodeRow from "./nodes/NodeRow.vue";
import NodeDrawers from "./nodes/NodeDrawers.vue";
defineOptions({ name: "NodesPage" });
const {
  loading,
  loadError,
  busy,
  overviewList,
  viewMode,
  activeGroup,
  searchText,
  filterCountries,
  countryOptions,
  filteredNodes,
  cardGroups,
  groupTree,
  groupedCount,
  selectedCount,
  allSelected,
  toggleNodes,
  clearSelection,
  clearFilters,
  localTesting,
  loadAll,
  triggerLocalPing,
  handleAddNode,
  openGroupManager,
  hideActiveGroup,
  copySelected,
  bulk,
  unlockVisible,
  tcpVisible,
  qualityVisible,
  testNode,
  qualityNode,
} = provideNodes();
const root = ref<HTMLElement>();
const stagger = useStagger();
const { reduced, animate } = useMotion();
watch([loading, viewMode], async () => {
  if (!loading.value) {
    await nextTick();
    if (root.value) stagger(root.value.querySelectorAll(".node-row"));
  }
});
function enterBar(element: Element, done: () => void) {
  if (reduced.value) {
    done();
    return;
  }
  animate(
    element,
    { y: 40, opacity: 0, scale: 0.96 },
    {
      y: 0,
      opacity: 1,
      scale: 1,
      duration: 0.56,
      ease: "elastic.out(1,.6)",
      clearProps: "transform,opacity",
      onComplete: done,
      onInterrupt: done,
    }
  );
}
function leaveBar(element: Element, done: () => void) {
  if (reduced.value) {
    done();
    return;
  }
  animate(
    element,
    { y: 0, opacity: 1 },
    { y: 16, opacity: 0, duration: 0.18, onComplete: done, onInterrupt: done }
  );
}
</script>
<template>
  <div class="nodes-workbench">
    <PageHeader title="节点" description="按分组管理连接，检查链路与可用性。"
      ><template #actions
        ><UiButton :icon="Refresh" :loading="loading" @click="loadAll"
          >刷新</UiButton
        ><UiButton variant="primary" :icon="Plus" @click="handleAddNode"
          >添加节点</UiButton
        ></template
      ></PageHeader
    >
    <div class="workbench-grid" data-enter>
      <aside class="group-sidebar">
        <div class="group-heading">
          <span class="eyebrow">节点 / 分组</span
          ><UiButton variant="ghost" @click="openGroupManager()"
            >管理分组</UiButton
          >
        </div>
        <UiStat size="large" label="全部节点" :value="overviewList.length" />
        <p class="coverage">
          已分组
          <span class="mono"
            >{{ groupedCount }} / {{ overviewList.length }}</span
          >
        </p>
        <nav class="group-navigation" aria-label="节点分组">
          <button
            :class="{ active: activeGroup === '全部' }"
            :aria-current="activeGroup === '全部' ? 'true' : undefined"
            @click="activeGroup = '全部'"
          >
            <span>全部</span><span class="mono">{{ overviewList.length }}</span>
          </button>
          <button
            v-for="group in groupTree"
            :key="group.name"
            :class="{ active: activeGroup === group.name }"
            :aria-current="activeGroup === group.name ? 'true' : undefined"
            @click="activeGroup = group.name"
          >
            <span class="group-name">{{ group.name }}</span
            ><span class="mono">{{ group.count }}</span>
          </button>
        </nav>
        <UiButton
          v-if="activeGroup !== '全部'"
          variant="ghost"
          :disabled="busy"
          @click="hideActiveGroup"
          >隐藏当前分组</UiButton
        >
        <p class="group-hint">隐藏的节点与分组可在管理中恢复。</p>
      </aside>
      <section class="node-main" :aria-busy="loading" aria-label="节点列表">
        <div class="node-toolbar">
          <el-input
            v-model="searchText"
            placeholder="搜索名称或主机"
            clearable
            aria-label="搜索节点名称或主机"
          /><el-select
            v-model="filterCountries"
            multiple
            collapse-tags
            clearable
            placeholder="全部国家"
            aria-label="国家筛选"
            ><el-option
              v-for="country in countryOptions"
              :key="country"
              :label="country"
              :value="country"
          /></el-select>
        </div>
        <div class="view-toolbar">
          <UiSegmented
            v-model="viewMode"
            :options="[
              { label: '列表', value: 'list' },
              { label: '卡片', value: 'card' },
            ]"
            label="节点视图"
          /><span class="result-count mono"
            >{{ filteredNodes.length }} 个节点</span
          ><UiButton
            v-if="filteredNodes.length"
            variant="ghost"
            @click="toggleNodes(filteredNodes)"
            >{{ allSelected(filteredNodes) ? "取消全选" : "全选" }}</UiButton
          >
        </div>
        <div class="probe-status">
          <StatusDot
            :state="localTesting ? 'idle' : 'ok'"
            :running="localTesting"
            :label="localTesting ? '正在检测本地延迟' : '本地延迟'"
          /><UiButton
            variant="ghost"
            :disabled="loading || !overviewList.length"
            :loading="localTesting"
            @click="triggerLocalPing"
            >重新检测</UiButton
          >
        </div>
        <EmptyState
          v-if="loadError"
          size="section"
          title="无法加载节点"
          :description="loadError"
          ><template #action
            ><UiButton @click="loadAll">重新加载</UiButton></template
          ></EmptyState
        >
        <UiSkeleton v-else-if="loading" :lines="5" height="72px" />
        <div v-else ref="root">
          <EmptyState
            size="page"
            v-if="!overviewList.length"
            title="建立你的连接"
            description="粘贴节点链接，或从机场同步节点。"
            ><template #action
              ><UiButton @click="handleAddNode">添加节点</UiButton></template
            ></EmptyState
          >
          <EmptyState
            v-else-if="!filteredNodes.length"
            size="section"
            title="没有匹配节点"
            description="试试其他分组、国家或关键词。"
            ><template #action
              ><UiButton @click="clearFilters">清除筛选</UiButton></template
            ></EmptyState
          >
          <template v-else-if="viewMode === 'list'">
            <div class="list-heading" aria-hidden="true">
              <span>节点 / 主机</span><span>协议</span><span>延迟</span
              ><span>可见</span><span class="actions-heading">操作</span>
            </div>
            <TransitionGroup
              tag="div"
              :name="reduced ? '' : 'nodes'"
              class="node-list"
              ><NodeRow
                v-for="node in filteredNodes"
                :key="node.id"
                :node="node"
            /></TransitionGroup>
          </template>
          <template v-else>
            <section
              v-for="group in cardGroups"
              :key="group.country"
              class="country-section"
            >
              <header class="country-heading">
                <el-checkbox
                  :model-value="allSelected(group.items)"
                  :aria-label="`全选 ${group.country} 节点`"
                  @change="toggleNodes(group.items)"
                />
                <h2>{{ group.country }}</h2>
                <span class="mono">{{ group.items.length }}</span
                ><UiButton variant="ghost" @click="toggleNodes(group.items)">{{
                  allSelected(group.items) ? "取消全选" : "全选"
                }}</UiButton>
              </header>
              <TransitionGroup
                tag="div"
                :name="reduced ? '' : 'nodes'"
                class="card-grid"
                ><NodeRow
                  v-for="node in group.items"
                  :key="node.id"
                  :node="node"
                  card
              /></TransitionGroup>
            </section>
          </template>
        </div>
      </section>
    </div>
    <Transition :css="false" @enter="enterBar" @leave="leaveBar"
      ><div v-if="selectedCount" class="bulk-anchor">
        <div class="bulk-bar" role="region" aria-label="已选节点批量操作">
          <span class="mono" role="status">已选 {{ selectedCount }}</span
          ><UiButton variant="ghost" @click="clearSelection">取消选择</UiButton
          ><UiButton @click="copySelected">复制选中</UiButton
          ><UiButton
            v-if="activeGroup !== '全部'"
            :disabled="busy"
            @click="bulk('unbind')"
            >移出分组</UiButton
          ><UiButton :disabled="busy" @click="bulk('hide')">隐藏选中</UiButton
          ><UiButton variant="danger" :disabled="busy" @click="bulk('delete')"
            >删除选中</UiButton
          >
        </div>
      </div></Transition
    >
    <NodeDrawers />
    <NodeUnlockDialog
      v-if="testNode"
      v-model:visible="unlockVisible"
      :node-id="testNode.id"
      :node-name="testNode.name"
    />
    <NodeTcpDialog
      v-if="testNode"
      v-model:visible="tcpVisible"
      :node-id="testNode.id"
      :node-name="testNode.name"
    />
    <NodeQualityDialog v-model:visible="qualityVisible" :node="qualityNode" />
  </div>
</template>
<style scoped>
.nodes-workbench {
  min-width: 0;
  padding-bottom: 56px;
}
.workbench-grid {
  display: grid;
  grid-template-columns: minmax(230px, 4fr) minmax(0, 8fr);
  gap: 24px;
  align-items: start;
}
.group-sidebar {
  position: sticky;
  top: 32px;
  min-width: 0;
  padding-right: 20px;
  border-right: 1px solid var(--c-line);
}
.group-heading {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}
.eyebrow,
.mono {
  font: 400 13px/1.6 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.eyebrow {
  letter-spacing: 0.08em;
  color: var(--c-muted);
}
.coverage,
.group-hint {
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.6;
}
.coverage {
  margin: 12px 0 20px;
}
.group-hint {
  margin-top: 16px;
}
.group-navigation {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
  max-height: 50dvh;
  overflow-y: auto;
}
.group-navigation button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-height: 40px;
  padding: 10px 12px;
  border-radius: var(--radius-control);
  text-align: left;
  transition:
    background var(--duration-fast),
    color var(--duration-fast);
}
.group-navigation button:hover {
  background: var(--c-surface-2);
}
.group-navigation .active {
  background: var(--c-selected);
  color: var(--c-signal-text);
}
.group-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.node-main {
  min-width: 0;
}
.node-toolbar {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 10px;
  margin-bottom: 12px;
}
.view-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-bottom: 8px;
}
.result-count {
  color: var(--c-muted);
  margin-left: auto;
}
.probe-status {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}
.probe-status :deep(.ui-button) {
  font-size: 14px;
}
.list-heading {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 78px 112px 52px 132px;
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  color: var(--c-muted);
  font: 400 13px/1.4 var(--font-mono);
}
.actions-heading {
  text-align: right;
}
.country-section + .country-section {
  margin-top: 20px;
}
.country-heading {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}
.country-heading h2 {
  margin: 0;
  font: 600 15px/1.4 var(--font-ui);
}
.country-heading :deep(.el-checkbox) {
  margin: 0;
}
.country-heading .mono {
  color: var(--c-muted);
}
.country-heading :deep(.ui-button) {
  margin-left: auto;
}
.card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, 280px), 1fr));
  gap: 12px;
}
.node-list,
.card-grid {
  position: relative;
}
.nodes-move,
.nodes-enter-active,
.nodes-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-out),
    opacity var(--duration-medium);
}
.nodes-enter-from,
.nodes-leave-to {
  opacity: 0;
  transform: translateY(12px);
}
.nodes-leave-active {
  position: absolute;
  width: 100%;
}
.bulk-anchor {
  position: fixed;
  z-index: 1100;
  bottom: 24px;
  left: 100px;
  right: 24px;
  display: flex;
  justify-content: center;
  pointer-events: none;
}
.bulk-bar {
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 10px;
  max-width: 100%;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-overlay);
  pointer-events: auto;
}
.bulk-bar .mono {
  padding: 0 4px;
}
@media (max-width: 1199px) {
  .workbench-grid {
    gap: 16px;
    grid-template-columns: minmax(190px, 4fr) minmax(0, 8fr);
  }
  .group-sidebar {
    padding-right: 16px;
  }
  .list-heading {
    grid-template-columns: minmax(0, 1fr) 65px 100px 44px 132px;
    gap: 8px;
  }
}
@media (min-width: 768px) and (max-width: 1099px) {
  .workbench-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .group-sidebar {
    position: static;
    border: 0;
    padding-right: 0;
  }
  .group-navigation {
    flex-direction: row;
    flex-wrap: wrap;
  }
  .group-navigation button {
    width: auto;
  }
  .group-sidebar :deep(.ui-stat),
  .coverage,
  .group-hint {
    display: none;
  }
  .group-heading {
    margin-bottom: 10px;
  }
}
@media (max-width: 767px) {
  .workbench-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
  .group-sidebar {
    position: static;
    border: 0;
    padding: 0 0 16px;
    border-bottom: 1px solid var(--c-line);
  }
  .group-sidebar :deep(.ui-stat),
  .coverage,
  .group-hint {
    display: none;
  }
  .group-navigation {
    flex-direction: row;
    flex-wrap: wrap;
    max-height: 200px;
  }
  .group-navigation button {
    width: auto;
    max-width: 100%;
  }
  .group-heading {
    margin-bottom: 10px;
  }
  .node-toolbar {
    grid-template-columns: minmax(0, 1fr);
  }
  .list-heading {
    display: none;
  }
  .bulk-anchor {
    left: 16px;
    right: 16px;
    bottom: calc(84px + env(safe-area-inset-bottom));
  }
  .bulk-bar {
    justify-content: flex-start;
    gap: 8px;
    padding: 10px;
  }
  .bulk-bar :deep(.ui-button) {
    padding: 8px 12px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .nodes-move,
  .nodes-enter-active,
  .nodes-leave-active {
    transition: none;
  }
}
</style>
