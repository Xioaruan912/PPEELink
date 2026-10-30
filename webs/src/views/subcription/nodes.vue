<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { Plus, Refresh } from "@element-plus/icons-vue";
import SubscriptionHeader from "@/components/subscriptions/SubscriptionHeader.vue";
import { UiButton, UiPanel, UiSkeleton, UiDrawer, EmptyState } from "@/ui";
import IconPulse from "~icons/ph/pulse";
import { useMotion } from "@/motion";
import NodeUnlockDialog from "@/components/NodeUnlockDialog.vue";
import NodeTcpDialog from "@/components/NodeTcpDialog.vue";
import { provideNodes } from "./nodes/useNodes";
import NodeRow from "./nodes/NodeRow.vue";
import NodeDrawers from "./nodes/NodeDrawers.vue";
import NodeTools from "./nodes/NodeTools.vue";
import CompatibilityReport from "@/components/share/CompatibilityReport.vue";
import PauseDialog from "./nodes/PauseDialog.vue";
import ProbePolicyDialog from "./nodes/ProbePolicyDialog.vue";
defineOptions({ name: "NodesPage" });
const {
  loading,
  loadError,
  busy,
  overviewList,
  activeGroup,
  searchText,
  filterCountries,
  countryOptions,
  filteredNodes,
  groupTree,
  groupedCount,
  selectedCount,
  allSelected,
  toggleNodes,
  clearSelection,
  clearFilters,
  loadAll,
  checking,
  checkValidity,
  handleAddNode,
  openGroupManager,
  copySelected,
  bulk,
  openPause,
  selectedNodes,
  unlockVisible,
  tcpVisible,
  testNode,
  openTool,
} = provideNodes();
const compatibilityOpen = ref(false);
const compatibilityIds = ref<number[]>([]);
function openCompatibility() {
  compatibilityIds.value = selectedNodes.value.map((n) => n.ID);
  compatibilityOpen.value = true;
}
const route = useRoute();
watch(
  () => [route.path, route.query.group] as const,
  ([path, group]) => {
    if (path === "/subcription/nodes" && typeof group === "string") {
      activeGroup.value = group;
      searchText.value = "";
      filterCountries.value = [];
    }
  },
  { immediate: true }
);
const root = ref<HTMLElement>();
const { reduced, animate } = useMotion();
const validCount = computed(
  () => overviewList.value.filter((node) => node.valid).length
);
const blockedCount = computed(
  () => overviewList.value.filter((node) => node.blocked).length
);
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
    <SubscriptionHeader
      section="nodes"
      title="节点管理"
      description="按分组整理连接，检查可用性并控制订阅下发。"
      ><template #actions
        ><UiButton :icon="Refresh" :loading="loading" @click="loadAll"
          >刷新</UiButton
        ><UiButton :disabled="loading || busy" @click="openTool('dedupe', [])"
          >去重</UiButton
        ><UiButton :disabled="loading" @click="openCompatibility"
          >客户端兼容</UiButton
        ><UiButton variant="primary" :icon="Plus" @click="handleAddNode"
          >添加节点</UiButton
        ></template
      ></SubscriptionHeader
    >
    <div class="node-overview" aria-label="节点概览">
      <div>
        <span>全部节点</span
        ><strong>{{ loading || loadError ? "—" : overviewList.length }}</strong>
      </div>
      <div>
        <span>检测有效</span
        ><strong>{{ loading || loadError ? "—" : validCount }}</strong>
      </div>
      <div>
        <span>暂不下发</span
        ><strong>{{ loading || loadError ? "—" : blockedCount }}</strong>
      </div>
      <div>
        <span>分组</span
        ><strong>{{ loading || loadError ? "—" : groupTree.length }}</strong>
      </div>
    </div>
    <div class="workbench-grid">
      <UiPanel class="group-sidebar" title="分组" :count="groupTree.length"
        ><template #actions
          ><UiButton variant="ghost" @click="openGroupManager()"
            >管理</UiButton
          ></template
        >
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
        <p class="group-hint">下发开关控制订阅输出，关闭后节点仍保留。</p>
      </UiPanel>
      <UiPanel
        class="node-main"
        title="节点列表"
        :count="filteredNodes.length"
        description="点击节点名称编辑，勾选后可批量管理。"
        :aria-busy="loading"
      >
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
          <UiButton
            variant="ghost"
            :icon="IconPulse"
            :loading="checking"
            :disabled="loading || !overviewList.length"
            @click="checkValidity"
            >{{ checking ? "检测中" : "检测有效性" }}</UiButton
          ><span class="result-count mono"
            >{{ filteredNodes.length }} 个节点</span
          ><UiButton
            v-if="filteredNodes.length"
            variant="ghost"
            @click="toggleNodes(filteredNodes)"
            >{{ allSelected(filteredNodes) ? "取消全选" : "全选" }}</UiButton
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
            description="粘贴节点链接，或从订阅里导入节点。"
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
          <template v-else>
            <div class="list-heading" aria-hidden="true">
              <span>节点 / 主机</span><span>协议</span><span>状态</span
              ><span>下发</span><span class="actions-heading">操作</span>
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
        </div>
      </UiPanel>
    </div>
    <Transition :css="false" @enter="enterBar" @leave="leaveBar"
      ><div v-if="selectedCount" class="bulk-anchor">
        <div class="bulk-bar" role="region" aria-label="已选节点批量操作">
          <span class="mono" role="status">已选 {{ selectedCount }}</span
          ><UiButton variant="ghost" @click="clearSelection">取消选择</UiButton
          ><UiButton @click="copySelected">复制选中</UiButton>
          <details class="bulk-more">
            <summary>更多操作</summary>
            <div class="bulk-more-actions">
              <UiButton :disabled="busy" @click="openTool('rename')"
                >改名</UiButton
              ><UiButton :disabled="busy" @click="openTool('flags')"
                >开关</UiButton
              ><UiButton
                v-if="activeGroup !== '全部'"
                :disabled="busy"
                @click="bulk('unbind')"
                >移出分组</UiButton
              ><UiButton :disabled="busy" @click="openPause(selectedNodes)"
                >暂停下发</UiButton
              ><UiButton :disabled="busy" @click="bulk('enable-delivery')"
                >开启下发</UiButton
              ><UiButton :disabled="busy" @click="bulk('disable-delivery')"
                >关闭下发</UiButton
              ><UiButton
                variant="danger"
                :disabled="busy"
                @click="bulk('delete')"
                >删除选中</UiButton
              >
            </div>
          </details>
        </div>
      </div></Transition
    >
    <UiDrawer
      v-model="compatibilityOpen"
      :title="
        compatibilityIds.length
          ? `客户端兼容 · 已选 ${compatibilityIds.length} 个节点`
          : '客户端兼容 · 全部节点'
      "
      size="min(760px, 100vw)"
    >
      <CompatibilityReport
        v-if="compatibilityOpen"
        :node-ids="compatibilityIds"
      />
      <template #footer
        ><UiButton @click="compatibilityOpen = false">关闭</UiButton></template
      >
    </UiDrawer>
    <NodeDrawers />
    <PauseDialog />
    <ProbePolicyDialog />
    <NodeTools />
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
  </div>
</template>
<style scoped>
.nodes-workbench {
  min-width: 0;
  padding-bottom: 56px;
}
.workbench-grid {
  display: grid;
  grid-template-columns: 208px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}
.group-sidebar {
  position: sticky;
  top: 76px;
  min-width: 0;
}
.group-sidebar :deep(.ui-panel__header) {
  flex-wrap: nowrap;
  gap: 8px;
}
.group-sidebar :deep(.ui-panel__heading) {
  flex-basis: 0;
}
.group-sidebar :deep(.ui-panel__actions) {
  flex-shrink: 0;
}
.group-navigation button:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: -2px;
}
.mono {
  font: 400 13px/1.6 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.coverage,
.group-hint {
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.coverage {
  margin: 0 0 12px;
}
.group-hint {
  margin-top: 16px;
}
.group-navigation {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 10px;
  max-height: 50dvh;
  overflow-y: auto;
}
.group-navigation button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
  min-height: 34px;
  padding: 7px 10px;
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
  gap: 4px 10px;
  margin-bottom: 8px;
}
.result-count {
  color: var(--c-muted);
  margin-left: auto;
}
.list-heading {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 78px 112px 52px 132px;
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  color: var(--c-muted);
  font: 400 12px/1.4 var(--font-ui);
}
.actions-heading {
  text-align: right;
}
.node-list,
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
  left: calc(var(--sidebar-w, 0px) + 24px);
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
}
@media (max-width: 767px) {
  .workbench-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 20px;
  }
  .group-sidebar {
    position: static;
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
  .node-toolbar {
    grid-template-columns: minmax(0, 1fr);
  }
  .list-heading {
    display: none;
  }
  .bulk-anchor {
    left: 16px;
    right: 16px;
    bottom: calc(16px + env(safe-area-inset-bottom));
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

<style scoped>
.node-overview {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 18px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}
.node-overview > div {
  padding: 16px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-right: 1px solid var(--c-line);
}
.node-overview > div:last-child {
  border-right: 0;
}
.node-overview span {
  font-size: 12px;
  color: var(--c-muted);
}
.node-overview strong {
  font: 500 24px var(--font-mono);
}
.workbench-grid :deep(.ui-panel) {
  box-shadow: none;
  border-radius: 12px;
}
.group-sidebar {
  background: var(--c-canvas);
}
.group-navigation button {
  min-height: 40px;
}
.node-toolbar {
  grid-template-columns: minmax(0, 1fr) minmax(160px, 0.55fr);
}
.node-list {
  position: relative;
}
.bulk-more {
  position: static;
}
.bulk-more summary {
  cursor: pointer;
  font-size: 13px;
  padding: 10px 14px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  list-style: none;
}
.bulk-more summary:focus-visible {
  outline: 2px solid var(--c-focus);
}
.bulk-more[open] summary {
  color: var(--c-signal-text);
}
.bulk-more-actions {
  position: absolute;
  bottom: calc(100% + 12px);
  right: 0;
  width: 300px;
  max-width: calc(100vw - 48px);
  padding: 12px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: 12px;
  box-shadow: var(--shadow-panel);
  max-height: 50vh;
  overflow: auto;
}
.bulk-bar {
  position: relative;
  border-radius: 12px;
  padding: 10px 14px;
  gap: 8px;
  box-shadow: var(--shadow-panel);
}
@media (max-width: 767px) {
  .node-overview {
    grid-template-columns: 1fr 1fr;
  }
  .node-overview > div {
    padding: 12px;
  }
  .node-overview > div:nth-child(2) {
    border-right: 0;
  }
  .node-overview > div:nth-child(n + 3) {
    border-top: 1px solid var(--c-line);
  }
  .node-overview strong {
    font-size: 22px;
  }
  .node-toolbar {
    grid-template-columns: 1fr;
  }
  .group-navigation {
    flex-wrap: nowrap;
    overflow-x: auto;
    max-height: none;
    padding-bottom: 4px;
  }
  .group-navigation button {
    flex-shrink: 0;
    min-height: 44px;
    max-width: 210px;
  }
  .workbench-grid {
    gap: 12px;
  }
  .bulk-anchor {
    left: 12px;
    right: 12px;
    bottom: calc(12px + env(safe-area-inset-bottom));
  }
  .bulk-bar {
    gap: 6px;
    padding: 8px;
  }
  .bulk-bar :deep(button),
  .bulk-more summary {
    min-height: 44px;
    font-size: 12px;
    padding: 8px;
  }
  .nodes-workbench {
    padding-bottom: 115px;
  }
}
</style>
