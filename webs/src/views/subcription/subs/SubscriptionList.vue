<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import gsap from "gsap";
import { Search, Grid, Share, MoreFilled } from "@element-plus/icons-vue";
import { UiButton, UiSkeleton, EmptyState, StatusDot } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
import CopyLinkButton from "./CopyLinkButton.vue";
import RowIconButton from "@/ui/RowIconButton.vue";

const {
  tableData,
  filteredSubs,
  loading,
  loadError,
  getsubs,
  searchKey,
  statusFilter,
  activeClient,
  clientOptions,
  currentUrl,
  handleQrcode,
  openImport,
  openDrawer,
  onCardCommand,
  fmtTime,
  handleAddSub,
  nodeCounts,
} = useSubscriptions();

const filters = [
  { label: "全部", value: "all" },
  { label: "有效", value: "active" },
  { label: "已过期", value: "expired" },
] as const;

const expired = (value: string | null) =>
  !!value && new Date(value).getTime() <= Date.now();

const sectionEl = ref<HTMLElement>();
const listEl = ref<HTMLElement>();
const tabsEl = ref<HTMLElement>();
const tabGliderStyle = ref({
  transform: "translate3d(0, 0, 0)",
  width: "0px",
  opacity: 0,
});

let rowAnim: gsap.core.Tween | null = null;
let ruleAnim: gsap.core.Tween | null = null;

function syncTabGlider() {
  const el = tabsEl.value;
  if (!el) return;
  const active = el.querySelector<HTMLElement>(".filter-tab.is-active");
  if (!active) return;
  tabGliderStyle.value = {
    transform: `translate3d(${active.offsetLeft}px, 0, 0)`,
    width: `${active.offsetWidth}px`,
    opacity: 1,
  };
}

function animateRowsIn() {
  rowAnim?.kill();
  if (
    !listEl.value ||
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    return;
  }
  const rows = listEl.value.querySelectorAll<HTMLElement>(".ledger-row");
  if (!rows.length) return;
  rowAnim = gsap.fromTo(
    rows,
    { y: 18, opacity: 0 },
    {
      y: 0,
      opacity: 1,
      duration: 0.52,
      stagger: 0.055,
      ease: "expo.out",
      clearProps: "transform,opacity",
    }
  );
}

onMounted(() => {
  void nextTick(() => {
    syncTabGlider();
    if (
      sectionEl.value &&
      !window.matchMedia("(prefers-reduced-motion: reduce)").matches
    ) {
      const rule = sectionEl.value.querySelector<HTMLElement>(".bar-top-rule");
      if (rule) {
        ruleAnim = gsap.fromTo(
          rule,
          { scaleX: 0 },
          {
            scaleX: 1,
            duration: 0.72,
            ease: "expo.out",
            clearProps: "transform",
          }
        );
      }
    }
    animateRowsIn();
  });
});

watch(statusFilter, () => {
  void nextTick(syncTabGlider);
});

watch(
  () => [loading.value, filteredSubs.value.map((s) => s.ID).join(",")],
  ([busy]) => {
    if (!busy) {
      void nextTick(animateRowsIn);
    }
  }
);

onBeforeUnmount(() => {
  rowAnim?.kill();
  ruleAnim?.kill();
});
</script>

<template>
  <section
    ref="sectionEl"
    class="gallery-ledger"
    :aria-busy="loading"
    aria-labelledby="ledger-heading"
  >
    <!-- Sticky Translucent Hairline Control Bar with Drawn Top Rule & Gliding Filter Pill -->
    <header class="ledger-bar">
      <span class="bar-top-rule" aria-hidden="true"></span>
      <div class="bar-left">
        <h2 id="ledger-heading" class="bar-heading">名录</h2>
        <span class="bar-count mono">{{ filteredSubs.length }}</span>
        <div
          ref="tabsEl"
          class="filter-tabs"
          role="tablist"
          aria-label="订阅状态筛选"
        >
          <span
            class="filter-glider"
            :style="tabGliderStyle"
            aria-hidden="true"
          ></span>
          <button
            v-for="item in filters"
            :key="item.value"
            type="button"
            role="tab"
            class="filter-tab"
            :class="{ 'is-active': statusFilter === item.value }"
            :aria-selected="statusFilter === item.value"
            @click="statusFilter = item.value"
          >
            {{ item.label }}
          </button>
        </div>
      </div>

      <label class="inline-search">
        <Search class="search-icon" aria-hidden="true" />
        <input
          v-model="searchKey"
          type="search"
          placeholder="搜索订阅名称"
          aria-label="搜索订阅名称"
        />
      </label>
    </header>

    <div v-if="loadError" class="ledger-state">
      <EmptyState size="section" title="无法加载订阅" :description="loadError">
        <template #action>
          <UiButton @click="getsubs">重新加载</UiButton>
        </template>
      </EmptyState>
    </div>

    <div v-else-if="loading" class="ledger-state">
      <UiSkeleton :lines="4" height="72px" />
    </div>

    <div v-else-if="!tableData.length" class="ledger-state">
      <EmptyState
        size="section"
        title="连接你的设备"
        description="新建订阅，选择节点并生成配置。"
      >
        <template #action>
          <UiButton variant="primary" @click="handleAddSub">
            创建第一份订阅
          </UiButton>
        </template>
      </EmptyState>
    </div>

    <div v-else-if="!filteredSubs.length" class="ledger-state">
      <EmptyState
        size="section"
        title="没有匹配订阅"
        description="试试其他名称或状态。"
      >
        <template #action>
          <UiButton
            @click="
              searchKey = '';
              statusFilter = 'all';
            "
            >清除筛选</UiButton
          >
        </template>
      </EmptyState>
    </div>

    <!-- Borderless Swiss Editorial Rows with union-tec.jp Shutter Wipe & Sibling Dimming -->
    <div v-else ref="listEl" class="ledger-list">
      <article
        v-for="sub in filteredSubs"
        :key="sub.ID"
        class="ledger-row"
        :aria-label="sub.Name"
      >
        <div class="row-identity">
          <div class="row-title-line">
            <button
              type="button"
              class="row-name-btn"
              :title="sub.Name"
              @click="openDrawer(sub)"
            >
              {{ sub.Name }}
            </button>
            <StatusDot
              :state="expired(sub.ExpiresAt) ? 'bad' : 'ok'"
              :label="expired(sub.ExpiresAt) ? '已过期' : '有效'"
            />
          </div>
          <p class="row-source">
            {{
              sub.GroupRefs?.length
                ? `${sub.GroupRefs.length} 个动态分组`
                : "固定节点来源"
            }}
            <template v-if="sub.GroupRefs?.length && sub.Nodes.length">
              · {{ sub.Nodes.length }} 个固定节点
            </template>
          </p>
        </div>

        <div class="row-metric">
          <span class="metric-label">节点</span>
          <strong class="mono">
            {{ nodeCounts[sub.ID] ?? `${sub.Nodes.length} 固定` }}
          </strong>
        </div>

        <div class="row-metric row-expiry">
          <span class="metric-label">有效期</span>
          <time class="mono" :datetime="sub.ExpiresAt || undefined">
            {{ sub.ExpiresAt ? fmtTime(sub.ExpiresAt) : "永不过期" }}
          </time>
        </div>

        <div class="row-controls">
          <CopyLinkButton
            :model-value="activeClient[sub.ID] || 'auto'"
            :value="currentUrl(sub)"
            :options="clientOptions"
            :label="sub.Name"
            @update:model-value="activeClient[sub.ID] = $event"
          />

          <div class="precision-tools">
            <RowIconButton
              :icon="Grid"
              :label="`${sub.Name} 二维码`"
              @click="handleQrcode(currentUrl(sub), sub.Name)"
            />
            <RowIconButton
              :icon="Share"
              :label="`分享 ${sub.Name}`"
              @click="openImport(sub)"
            />
            <el-dropdown
              trigger="click"
              @command="(command: string) => onCardCommand(command, sub)"
            >
              <RowIconButton
                :icon="MoreFilled"
                :label="`${sub.Name} 更多操作`"
              />
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="edit">编辑订阅</el-dropdown-item>
                  <el-dropdown-item command="reset" divided
                    >重置订阅链接</el-dropdown-item
                  >
                  <el-dropdown-item command="delete">
                    <span class="danger-text">删除订阅</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.gallery-ledger {
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
  gap: 20px;
  flex-wrap: wrap;
  padding: 14px 0;
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

.bar-left {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.bar-heading {
  margin: 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
}

.bar-count {
  font-size: 12px;
  color: var(--c-muted);
}

.filter-tabs {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  margin-left: 8px;
}

.filter-glider {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  border-radius: 999px;
  background: var(--c-surface);
  box-shadow: inset 0 0 0 1px var(--c-line);
  pointer-events: none;
  transition:
    transform var(--duration-medium) var(--ease-out),
    width var(--duration-medium) var(--ease-out),
    opacity var(--duration-fast) ease;
}

.filter-tab {
  position: relative;
  z-index: 1;
  padding: 6px 13px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.filter-tab:hover {
  color: var(--c-ink);
}

.filter-tab.is-active {
  color: var(--c-ink);
  font-weight: 500;
}

.inline-search {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 210px;
  padding: 6px 0;
  border-bottom: 1px solid var(--c-line);
  color: var(--c-muted);
  transition: border-color var(--duration-fast) var(--ease-out);
}

.inline-search:focus-within {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.search-icon {
  width: 15px;
  height: 15px;
  flex: none;
}

.inline-search input {
  width: 100%;
  border: 0;
  outline: none;
  background: transparent;
  color: var(--c-ink);
  font-size: 13.5px;
}

.inline-search input::placeholder {
  color: var(--c-muted);
}

.ledger-state {
  padding: 40px 0;
}

.ledger-list {
  display: grid;
}

.ledger-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.5fr) 108px 168px auto;
  align-items: center;
  gap: 24px;
  padding: 22px 10px;
  margin: 0 -10px;
  border-bottom: 1px solid var(--c-line);
  transition: opacity 280ms var(--ease-out);
}

.ledger-row::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: rgb(10 10 10 / 0.028);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 360ms var(--ease-out);
  pointer-events: none;
  z-index: 0;
}

.ledger-row::after {
  content: "";
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: -1px;
  height: 1px;
  background: var(--c-ink);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 340ms var(--ease-out);
  pointer-events: none;
  z-index: 1;
}

.ledger-row > * {
  position: relative;
  z-index: 1;
}

.row-identity {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.row-title-line {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.row-name-btn {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-display);
  font-size: 21px;
  font-weight: 450;
  letter-spacing: -0.03em;
  line-height: 1.2;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: transform 240ms var(--ease-out);
}

.row-source {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-muted);
}

.row-metric {
  display: grid;
  gap: 4px;
}

.metric-label {
  font-size: 11px;
  color: var(--c-muted);
}

.row-metric strong,
.row-metric time {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.row-controls {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}

.precision-tools {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  transition:
    opacity 240ms var(--ease-out),
    transform 240ms var(--ease-out);
}

@media (hover: hover) and (pointer: fine) {
  .ledger-list:hover .ledger-row:not(:hover) {
    opacity: 0.46;
  }
  .ledger-row:hover::before,
  .ledger-row:focus-within::before,
  .ledger-row:hover::after,
  .ledger-row:focus-within::after {
    transform: scaleX(1);
  }
  .ledger-row:hover .row-name-btn {
    transform: translateX(4px);
  }
  .precision-tools {
    opacity: 0.28;
    transform: translateX(8px);
  }
  .ledger-row:hover .precision-tools,
  .ledger-row:focus-within .precision-tools {
    opacity: 1;
    transform: translateX(0);
  }
}

.danger-text {
  color: var(--c-bad);
}

@media (max-width: 1020px) {
  .ledger-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 14px 20px;
    padding: 22px 0;
    margin: 0;
  }
  .ledger-row::after {
    left: 0;
    right: 0;
  }
  .row-metric {
    grid-column: 1;
    display: inline-flex;
    align-items: baseline;
    gap: 8px;
  }
  .row-expiry {
    margin-top: -6px;
  }
  .row-controls {
    grid-column: 1 / -1;
    justify-content: space-between;
    padding-top: 4px;
  }
}

@media (max-width: 640px) {
  .ledger-bar {
    position: static;
  }
  .inline-search {
    width: 100%;
  }
  .row-name-btn {
    font-size: 18px;
  }
}
</style>
