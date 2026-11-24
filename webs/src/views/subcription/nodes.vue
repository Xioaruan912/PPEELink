<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from "vue";
import { useRoute } from "vue-router";
import gsap from "gsap";
import { Plus, Refresh, Search, MoreFilled } from "@element-plus/icons-vue";
import { UiButton, UiSkeleton, UiDrawer, EmptyState } from "@/ui";
import { countryFlag } from "@/utils/flag";
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
  selectedCount,
  allSelected,
  toggleNodes,
  clearSelection,
  clearFilters,
  loadAll,
  checking,
  checkValidity,
  handleAddNode,
  copySelected,
  bulk,
  openPause,
  selectedNodes,
  unlockVisible,
  tcpVisible,
  testNode,
  openTool,
} = provideNodes();

const countryPills = computed(() => {
  const counts = new Map<string, { name: string; code: string; count: number }>();
  for (const n of overviewList.value) {
    const name = n.country || "未知";
    const entry = counts.get(name);
    if (entry) {
      entry.count++;
    } else {
      counts.set(name, { name, code: n.countryCode || "", count: 1 });
    }
  }
  return [...counts.values()].sort(
    (a, b) => b.count - a.count || a.name.localeCompare(b.name, "zh-CN")
  );
});

function selectCountryPill(country: string) {
  activeGroup.value = "全部";
  if (!country) {
    filterCountries.value = [];
  } else if (
    filterCountries.value.length === 1 &&
    filterCountries.value[0] === country
  ) {
    filterCountries.value = [];
  } else {
    filterCountries.value = [country];
  }
}

const compatibilityOpen = ref(false);
const compatibilityIds = ref<number[]>([]);
function openCompatibility() {
  compatibilityIds.value = selectedNodes.value.map((n) => n.id);
  compatibilityOpen.value = true;
}

const route = useRoute();
watch(
  () => [route.path, route.query.group, route.query.q] as const,
  ([path, group, q]) => {
    if (path !== "/subcription/nodes") return;
    if (typeof group === "string") {
      activeGroup.value = group;
      searchText.value = "";
      filterCountries.value = [];
    } else if (typeof q === "string" && q) {
      searchText.value = q;
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

const rootEl = ref<HTMLElement>();
const listEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;
let rowAnim: gsap.core.Tween | null = null;

function animateRowsIn() {
  rowAnim?.kill();
  if (
    !listEl.value ||
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    return;
  }
  const rows = listEl.value.querySelectorAll<HTMLElement>(".node-row");
  if (!rows.length) return;
  rowAnim = gsap.fromTo(
    rows,
    { y: 16, opacity: 0 },
    {
      y: 0,
      opacity: 1,
      duration: 0.48,
      stagger: 0.04,
      ease: "expo.out",
      clearProps: "transform,opacity",
    }
  );
}

onMounted(() => {
  if (!rootEl.value) return;
  gsapCtx = gsap.context(() => {
    const mm = gsap.matchMedia();
    mm.add("(prefers-reduced-motion: no-preference)", () => {
      const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
      tl.from("[data-title-reveal]", {
        yPercent: 108,
        duration: 0.62,
        clearProps: "transform",
      })
        .from(
          "[data-metric-item]",
          {
            y: 12,
            opacity: 0,
            duration: 0.46,
            stagger: 0.05,
            clearProps: "transform,opacity",
          },
          "-=0.42"
        )
        .from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.7,
            clearProps: "transform",
          },
          "-=0.44"
        );
    });
  }, rootEl.value);
  void nextTick(animateRowsIn);
});

watch(
  () => [loading.value, activeGroup.value, filteredNodes.value.length],
  ([busyNow]) => {
    if (!busyNow) {
      void nextTick(animateRowsIn);
    }
  }
);

onBeforeUnmount(() => {
  gsapCtx?.revert();
  gsapCtx = null;
  rowAnim?.kill();
});

function handleBulkCommand(command: string) {
  if (command === "rename") openTool("rename");
  else if (command === "flags") openTool("flags");
  else if (command === "unbind") bulk("unbind");
  else if (command === "pause") openPause(selectedNodes.value);
  else if (command === "enable-delivery") bulk("enable-delivery");
  else if (command === "disable-delivery") bulk("disable-delivery");
  else if (command === "delete") bulk("delete");
}

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
  <div ref="rootEl" class="nodes-gallery">
    <!-- Compact Pure Gallery Masthead -->
    <header class="nodes-masthead" aria-labelledby="nodes-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <div class="title-mask">
            <h1 id="nodes-title" class="gallery-title" data-title-reveal>
              节点资产
            </h1>
          </div>
          <dl class="inline-metrics mono" aria-label="节点概览指标">
            <div data-metric-item>
              <dt>全部</dt>
              <dd>{{ loading || loadError ? "—" : overviewList.length }}</dd>
            </div>
            <div data-metric-item>
              <dt>有效</dt>
              <dd class="is-ok">
                {{ loading || loadError ? "—" : validCount }}
              </dd>
            </div>
            <div v-if="blockedCount > 0" data-metric-item>
              <dt>暂不下发</dt>
              <dd class="is-warn">{{ blockedCount }}</dd>
            </div>
            <div data-metric-item>
              <dt>地区</dt>
              <dd>{{ loading || loadError ? "—" : countryPills.length }}</dd>
            </div>
          </dl>
        </div>
        <p class="gallery-lead">
          点击节点名称查看硬件级详情、探针延迟与下发策略。
        </p>
      </div>

      <div class="masthead-actions">
        <UiButton variant="primary" :icon="Plus" @click="handleAddNode">
          添加节点
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="IconPulse"
          :loading="checking"
          :disabled="loading || !overviewList.length"
          @click="checkValidity"
        >
          {{ checking ? "检测中" : "检测有效" }}
        </UiButton>
        <UiButton variant="ghost" :disabled="loading" @click="openCompatibility">
          客户端兼容
        </UiButton>
        <el-dropdown
          trigger="click"
          @command="
            (cmd: string) =>
              cmd === 'dedupe' ? openTool('dedupe', []) : loadAll()
          "
        >
          <UiButton variant="ghost" :icon="MoreFilled">更多</UiButton>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="dedupe" :disabled="loading || busy">
                去除重复节点
              </el-dropdown-item>
              <el-dropdown-item command="refresh" divided :disabled="loading">
                刷新全部节点
              </el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <!-- Sticky Hairline Filter & Group Pill Bar -->
    <section class="nodes-ledger" :aria-busy="loading" aria-label="节点名录">
      <header class="ledger-bar">
        <span class="bar-top-rule" data-top-rule aria-hidden="true"></span>

        <div class="group-pills" role="tablist" aria-label="按国家/地区筛选节点">
          <button
            type="button"
            role="tab"
            class="group-pill"
            :class="{ 'is-active': !filterCountries.length }"
            :aria-selected="!filterCountries.length"
            @click="selectCountryPill('')"
          >
            <span>全部地区</span>
            <small class="mono">{{ overviewList.length }}</small>
          </button>
          <button
            v-for="c in countryPills"
            :key="c.name"
            type="button"
            role="tab"
            class="group-pill"
            :class="{
              'is-active':
                filterCountries.length === 1 && filterCountries[0] === c.name,
            }"
            :aria-selected="
              filterCountries.length === 1 && filterCountries[0] === c.name
            "
            @click="selectCountryPill(c.name)"
          >
            <span>{{ countryFlag(c.code) }} {{ c.name }}</span>
            <small class="mono">{{ c.count }}</small>
          </button>
        </div>

        <div class="bar-right">
          <label class="inline-search">
            <Search class="search-icon" aria-hidden="true" />
            <input
              v-model="searchText"
              type="search"
              placeholder="搜索名称或主机"
              aria-label="搜索节点名称或主机"
            />
          </label>

          <button
            v-if="filteredNodes.length"
            type="button"
            class="select-all-btn"
            @click="toggleNodes(filteredNodes)"
          >
            {{ allSelected(filteredNodes) ? "取消全选" : "全选" }}
          </button>
        </div>
      </header>

      <div v-if="loadError" class="ledger-state">
        <EmptyState size="section" title="无法加载节点" :description="loadError">
          <template #action>
            <UiButton @click="loadAll">重新加载</UiButton>
          </template>
        </EmptyState>
      </div>

      <div v-else-if="loading" class="ledger-state">
        <UiSkeleton :lines="5" height="72px" />
      </div>

      <div v-else-if="!overviewList.length" class="ledger-state">
        <EmptyState
          size="page"
          title="建立你的连接"
          description="粘贴节点链接，或从订阅里导入节点。"
        >
          <template #action>
            <UiButton variant="primary" @click="handleAddNode">
              添加节点
            </UiButton>
          </template>
        </EmptyState>
      </div>

      <div v-else-if="!filteredNodes.length" class="ledger-state">
        <EmptyState
          size="section"
          title="没有匹配节点"
          description="试试其他分组、国家或关键词。"
        >
          <template #action>
            <UiButton @click="clearFilters">清除筛选</UiButton>
          </template>
        </EmptyState>
      </div>

      <template v-else>
        <div class="list-heading" aria-hidden="true">
          <span>节点 / 主机 · 共 {{ filteredNodes.length }}</span>
          <span>协议</span>
          <span>状态</span>
          <span>下发</span>
          <span class="actions-heading">操作</span>
        </div>

        <div ref="listEl" class="node-list">
          <NodeRow
            v-for="node in filteredNodes"
            :key="node.id"
            :node="node"
            @inspect="inspectedId = $event"
          />
        </div>
      </template>
    </section>

    <!-- Machined Obsidian Floating Bulk Action Pill -->
    <Transition :css="false" @enter="enterBar" @leave="leaveBar">
      <div v-if="selectedCount" class="bulk-anchor">
        <div class="bulk-bar" role="region" aria-label="已选节点批量操作">
          <span class="bulk-count mono" role="status">
            已选 {{ selectedCount }}
          </span>
          <span class="bulk-sep" aria-hidden="true"></span>
          <button type="button" class="bulk-btn" @click="copySelected">
            复制选中
          </button>
          <button type="button" class="bulk-btn" @click="openCompatibility">
            检查兼容
          </button>
          <el-dropdown
            trigger="click"
            placement="top"
            @command="handleBulkCommand"
          >
            <button type="button" class="bulk-btn is-accent">
              更多操作 ▴
            </button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="rename" :disabled="busy"
                  >批量改名…</el-dropdown-item
                >
                <el-dropdown-item command="flags" :disabled="busy"
                  >连接开关…</el-dropdown-item
                >
                <el-dropdown-item command="pause" divided :disabled="busy"
                  >暂停下发…</el-dropdown-item
                >
                <el-dropdown-item command="enable-delivery" :disabled="busy"
                  >开启下发</el-dropdown-item
                >
                <el-dropdown-item command="disable-delivery" :disabled="busy"
                  >关闭下发</el-dropdown-item
                >
                <el-dropdown-item command="delete" divided :disabled="busy">
                  <span class="danger">删除选中节点</span>
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
          <button type="button" class="bulk-btn is-muted" @click="clearSelection">
            取消
          </button>
        </div>
      </div>
    </Transition>

    <UiDrawer
      v-model="compatibilityOpen"
      :title="
        compatibilityIds.length
          ? `客户端兼容 · 已选 ${compatibilityIds.length} 个节点`
          : '客户端兼容 · 全部节点'
      "
      size="min(740px, 100vw)"
    >
      <CompatibilityReport
        v-if="compatibilityOpen"
        :node-ids="compatibilityIds"
      />
      <template #footer>
        <UiButton @click="compatibilityOpen = false">关闭</UiButton>
      </template>
    </UiDrawer>

    <NodeInspector v-model="inspectedId" />
    <NodeDrawers />
    <PauseDialog />
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
.nodes-gallery {
  display: grid;
  gap: 20px;
  max-width: 1200px;
  min-width: 0;
  padding-bottom: 64px;
}

.nodes-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  padding-top: 2px;
}

.masthead-left {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.title-cluster {
  display: flex;
  align-items: baseline;
  gap: 20px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.inline-metrics {
  display: inline-flex;
  align-items: baseline;
  gap: 16px;
  margin: 0;
}

.inline-metrics > div {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
}

.inline-metrics dt {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--c-muted);
}

.inline-metrics dd {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--c-ink);
}

.inline-metrics dd.is-ok {
  color: var(--c-ok);
}

.inline-metrics dd.is-warn {
  color: var(--c-warn);
}

.gallery-lead {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--c-muted);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.nodes-ledger {
  display: grid;
  min-width: 0;
}

.ledger-bar {
  position: sticky;
  top: 0;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px 20px;
  flex-wrap: wrap;
  padding: 12px 0;
  border-bottom: 1px solid var(--c-line);
  background: color-mix(in srgb, var(--c-canvas) 90%, transparent);
  backdrop-filter: blur(14px);
}

.bar-top-rule {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
  pointer-events: none;
}

.group-pills {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.group-pill {
  display: inline-flex;
  align-items: baseline;
  gap: 5px;
  padding: 6px 12px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition:
    color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.group-pill:hover {
  color: var(--c-ink);
}

.group-pill.is-active {
  background: var(--c-surface);
  border-color: var(--c-line);
  color: var(--c-ink);
  font-weight: 500;
}

.group-pill small {
  font-size: 11px;
  color: var(--c-muted);
}

.group-manage-link {
  margin-left: 4px;
  padding: 6px 8px;
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.group-manage-link:hover {
  color: var(--c-ink);
  text-decoration: underline;
  text-underline-offset: 3px;
}

.bar-right {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.country-select {
  width: 152px;
}

.country-select :deep(.el-select__wrapper) {
  border-radius: 999px;
  background: transparent;
  box-shadow: inset 0 0 0 1px var(--c-line);
  min-height: 32px;
  padding: 2px 12px;
}

.inline-search {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  width: 196px;
  padding: 5px 0;
  border-bottom: 1px solid var(--c-line);
  color: var(--c-muted);
  transition: border-color var(--duration-fast) var(--ease-out);
}

.inline-search:focus-within {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.search-icon {
  width: 14px;
  height: 14px;
  flex: none;
}

.inline-search input {
  width: 100%;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--c-ink);
  font-size: 13px;
}

.inline-search input::placeholder {
  color: var(--c-muted);
}

.select-all-btn {
  padding: 5px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
  transition:
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.select-all-btn:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.ledger-state {
  padding: 40px 0;
}

.list-heading {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 88px 110px 64px 124px;
  gap: 16px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 500;
}

.actions-heading {
  text-align: right;
}

.node-list {
  display: grid;
}

@media (hover: hover) and (pointer: fine) {
  .node-list:hover :deep(.node-row:not(:hover)) {
    opacity: 0.46;
  }
}

/* Machined Obsidian Floating Bulk Action Pill */
.bulk-anchor {
  position: fixed;
  z-index: 1100;
  left: 0;
  right: 0;
  bottom: 24px;
  display: flex;
  justify-content: center;
  pointer-events: none;
}

.bulk-bar {
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px 6px 16px;
  border-radius: 999px;
  background: rgb(12 12 14 / 0.94);
  backdrop-filter: blur(20px) saturate(150%);
  border: 1px solid rgb(255 255 255 / 0.14);
  color: #ffffff;
  box-shadow: 0 24px 60px -12px rgb(0 0 0 / 0.38);
}

.bulk-count {
  font-size: 12.5px;
  font-weight: 600;
  color: #29df7e;
}

.bulk-sep {
  width: 1px;
  height: 16px;
  margin: 0 4px;
  background: rgb(255 255 255 / 0.18);
}

.bulk-btn {
  padding: 6px 12px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: rgb(255 255 255 / 0.85);
  font-size: 12.5px;
  cursor: pointer;
  transition: background 180ms var(--ease-out), color 180ms var(--ease-out);
}

.bulk-btn:hover {
  background: rgb(255 255 255 / 0.12);
  color: #ffffff;
}

.bulk-btn.is-accent {
  background: rgb(255 255 255 / 0.12);
  color: #ffffff;
  font-weight: 500;
}

.bulk-btn.is-muted {
  color: rgb(255 255 255 / 0.55);
}

.danger {
  color: var(--c-bad);
}

@media (max-width: 960px) {
  .list-heading {
    display: none;
  }
}

@media (max-width: 640px) {
  .ledger-bar {
    position: static;
  }
  .bar-right {
    width: 100%;
  }
  .country-select,
  .inline-search {
    width: 100%;
  }
}
</style>
