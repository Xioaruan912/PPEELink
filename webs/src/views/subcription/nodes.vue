<script setup lang="ts">
import { ref, computed, watch } from "vue";
import { useRoute } from "vue-router";
import { Plus, Refresh } from "@element-plus/icons-vue";
import PageHeader from "@/ui/PageHeader.vue";
import { UiButton, UiPanel, UiSkeleton, UiDrawer, EmptyState } from "@/ui";
import IconPulse from "~icons/ph/pulse";
import { useMotion } from "@/motion";
import NodeUnlockDialog from "@/components/NodeUnlockDialog.vue";
import NodeTcpDialog from "@/components/NodeTcpDialog.vue";
import { provideNodes } from "./nodes/useNodes";
import NodeInspector from "./nodes/NodeInspector.vue";
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
  compatibilityIds.value = selectedNodes.value.map((n) => n.id);
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
const inspectedId = ref<number | null>(null);
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
    { y: 20, opacity: 0, scale: 0.98 },
    {
      y: 0,
      opacity: 1,
      scale: 1,
      duration: 0.28,
      ease: "power3.out",
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
    <aside class="nodes-masthead" data-enter>
      <PageHeader
        title="节点资产"
        description="按分组整理连接，检查可用性并控制订阅下发。"
        ><template #actions
          ><UiButton variant="primary" :icon="Plus" @click="handleAddNode"
            >添加节点</UiButton
          ><UiButton :icon="Refresh" :loading="loading" @click="loadAll"
            >刷新</UiButton
          ><UiButton :disabled="loading || busy" @click="openTool('dedupe', [])"
            >去重</UiButton
          ><UiButton :disabled="loading" @click="openCompatibility"
            >客户端兼容</UiButton
          ></template
        ></PageHeader
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
    </aside>
    <UiPanel
      class="node-main"
      title="节点列表"
      description="点击节点名称查看连接、诊断和下发策略。"
      flush
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
        <el-select v-model="activeGroup" filterable aria-label="节点分组">
          <el-option label="全部分组" value="全部" />
          <el-option
            v-for="group in groupTree"
            :key="group.name"
            :label="`${group.name} · ${group.count}`"
            :value="group.name"
          />
        </el-select>
        <UiButton @click="openGroupManager()">管理分组</UiButton>
      </div>
      <div class="view-toolbar">
        <UiButton
          variant="ghost"
          :icon="IconPulse"
          :loading="checking"
          :disabled="loading || !overviewList.length"
          @click="checkValidity"
          >{{ checking ? "检测中" : "检测有效性" }}</UiButton
        ><span class="result-count mono">{{ filteredNodes.length }} 个节点</span
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
      <div v-else>
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
              @inspect="inspectedId = $event"
          /></TransitionGroup>
        </template>
      </div>
    </UiPanel>
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
    <NodeInspector v-model="inspectedId" />
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
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 40px;
  align-items: start;
  min-width: 0;
  padding-bottom: 56px;
}
.nodes-masthead {
  position: sticky;
  top: 28px;
  display: grid;
  gap: 28px;
}
.nodes-masthead :deep(.page-header) {
  margin-bottom: 0;
}
.nodes-masthead :deep(.page-header__grid) {
  flex-direction: column;
  align-items: stretch;
  gap: 18px;
}
.nodes-masthead :deep(.page-header__actions) {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.nodes-masthead :deep(.page-header__actions > :first-child) {
  grid-column: 1 / -1;
}
.mono {
  font: 400 13px/1.6 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.node-main {
  min-width: 0;
}
.node-toolbar {
  display: grid;
  grid-template-columns: minmax(160px, 2fr) minmax(125px, 1fr) minmax(
      125px,
      1fr
    ) auto;
  gap: 10px;
  padding: 18px 20px 8px;
}
.view-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 10px;
  padding: 0 20px 14px;
}
.result-count {
  color: var(--c-muted);
  margin-left: auto;
}
.list-heading {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 92px 116px 68px 136px;
  align-items: center;
  gap: 14px;
  padding: 12px 20px;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
  background: var(--c-surface-2);
  color: var(--c-muted);
  font: 600 12px/1.4 var(--font-ui);
}
.actions-heading {
  text-align: right;
  padding-right: 8px;
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
  .list-heading {
    grid-template-columns: minmax(0, 1fr) 65px 100px 44px 132px;
    gap: 8px;
  }
}
@media (max-width: 1160px) {
  .node-toolbar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
@media (max-width: 767px) {
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
  border-top: 1px solid var(--c-ink);
}
.node-overview > div {
  padding: 15px 2px;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--c-line);
}
.node-overview span {
  font-size: 13px;
  font-weight: 540;
  color: var(--c-muted);
}
.node-overview strong {
  font: 700 28px/1 var(--font-display);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}
@media (max-width: 1024px) {
  .nodes-workbench {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .nodes-masthead {
    position: static;
    gap: 20px;
  }
  .node-overview {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    border: 1px solid var(--c-line);
    border-radius: var(--radius-panel);
    background: var(--c-surface);
  }
  .node-overview > div {
    padding: 14px 16px;
    border-bottom: 0;
    border-right: 1px solid var(--c-line);
  }
  .node-overview > div:last-child {
    border-right: 0;
  }
}

.node-list {
  position: relative;
  display: grid;
  gap: 0;
}
.bulk-more {
  position: static;
}
.bulk-more summary {
  cursor: pointer;
  font-size: 13px;
  padding: 9px 14px;
  background: var(--c-surface-2);
  border-radius: var(--radius-control);
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
  border-radius: var(--radius-panel);
  box-shadow: var(--shadow-overlay);
  max-height: 50vh;
  overflow: auto;
}
.bulk-bar {
  position: relative;
  border: 0;
  border-radius: var(--radius-panel);
  padding: 10px 16px;
  gap: 8px;
  box-shadow: var(--shadow-overlay);
}
@media (max-width: 767px) {
  .node-overview {
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }
  .node-overview > div {
    padding: 14px;
  }
  .node-overview strong {
    font-size: 24px;
  }
  .node-toolbar {
    grid-template-columns: 1fr;
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
