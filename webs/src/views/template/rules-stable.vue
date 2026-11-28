<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from "vue";
import gsap from "gsap";
import {
  FolderChecked,
  Plus,
  Refresh,
  Search,
  View,
} from "@element-plus/icons-vue";
import { EmptyState, UiButton, UiDialog, UiSkeleton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import RuleDialogs from "./rules/RuleDialogs.vue";
import {
  provideRules,
  sourceLabel,
  fmtTime,
  type RuleItem,
} from "./rules/useRules";

defineOptions({ name: "RulesWorkbench" });

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;
const showSources = ref(false);
const addSourceOpen = ref(false);
const customForm = reactive({
  name: "",
  repo: "",
  branch: "main",
  pathPrefix: "",
  platform: "Clash",
  behavior: "classical",
});

const {
  loading,
  loadingMore,
  loadMoreError,
  hasMore,
  sourcesLoading,
  syncing,
  syncingKey,
  savingSource,
  errorText,
  sourcesError,
  rules,
  sources,
  total,
  keyword,
  source,
  platform,
  category,
  categories,
  selectedIds,
  selectedRules,
  focusedRule,
  loadSources,
  loadCatalog,
  loadMoreCatalog,
  resetFilters,
  toggleCategory,
  toggleSelected,
  doSync,
  addCustomSource,
  removeCustomSource,
  openPreview,
  openImport,
} = provideRules();

const sourceTabs = computed(() => [
  { label: "全部来源", value: "" },
  ...sources.value.map((s: any) => ({
    label: s.name || sourceLabel(s.key),
    value: s.key,
  })),
]);

function openAddSource() {
  Object.assign(customForm, {
    name: "",
    repo: "",
    branch: "main",
    pathPrefix: "",
    platform: platform.value || "Clash",
    behavior: "classical",
  });
  addSourceOpen.value = true;
}

async function submitCustomSource() {
  const ok = await addCustomSource({ ...customForm });
  if (ok) addSourceOpen.value = false;
}

const clientTabs = ["Clash", "Surge", "Loon"];

const allSelected = computed(
  () => !!rules.value.length && selectedIds.value.length === rules.value.length
);

function selectSource(val: string) {
  if (source.value === val) return;
  source.value = val;
  void loadCatalog();
}

function selectPlatform(val: string) {
  if (platform.value === val) return;
  platform.value = val;
  void loadCatalog();
}

function inspect(row: RuleItem) {
  void openPreview(row, true);
}

function selectAll() {
  selectedIds.value = allSelected.value
    ? []
    : rules.value.map((row) => row.externalId);
}

onMounted(() => {
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="rules-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="rules-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>规则仓库</h1>
        </div>
        <span class="count-pill mono">
          <i aria-hidden="true"></i>
          {{ total }} 条规则
        </span>
      </div>

      <div class="masthead-actions">
        <UiButton
          variant="ghost"
          @click="showSources = !showSources"
        >
          {{ showSources ? "收起规则源" : `规则源 (${sources.length})` }}
        </UiButton>
        <UiButton
          variant="primary"
          :icon="Refresh"
          :loading="syncing"
          @click="doSync()"
        >
          {{ syncing ? "同步中…" : "立即同步" }}
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <KineticNotice :raw="errorText || sourcesError" />

    <!-- On-Demand Collapsible Remote Sources Strip -->
    <Transition name="fade-slide">
      <section v-if="showSources" class="sources-strip" aria-label="远程规则源状态">
        <div class="sources-bar">
          <span class="muted">共 {{ sources.length }} 个规则源</span>
          <UiButton size="sm" variant="ghost" :icon="Plus" @click="openAddSource">
            添加自定义源
          </UiButton>
        </div>
        <UiSkeleton v-if="sourcesLoading" :lines="2" height="44px" />
        <div v-else-if="sources.length" class="source-grid">
          <article v-for="item in sources" :key="item.key" class="source-card">
            <div class="source-top">
              <strong>{{ item.name }}</strong>
              <span
                class="src-state"
                :class="item.status === 'ok' ? 'is-ok' : item.error ? 'is-bad' : ''"
              >
                <i aria-hidden="true"></i>
                {{ item.status === "ok" ? "已同步" : item.error ? "异常" : "待同步" }}
              </span>
            </div>
            <span class="src-repo mono">
              {{ item.repo }}<template v-if="item.pathPrefix">/{{ item.pathPrefix }}</template>
            </span>
            <div class="src-foot mono">
              <span>{{ item.count || 0 }} 条 · {{ fmtTime(item.lastSyncAt) }}</span>
              <div class="src-actions">
                <button
                  type="button"
                  class="src-link-btn"
                  :disabled="syncing || syncingKey === item.key"
                  @click="doSync(item.key)"
                >
                  {{ syncingKey === item.key ? "同步中" : "同步" }}
                </button>
                <button
                  v-if="item.custom"
                  type="button"
                  class="src-link-btn is-danger"
                  @click="removeCustomSource(item.key)"
                >
                  删除
                </button>
              </div>
            </div>
          </article>
        </div>
        <div v-else class="source-empty">
          <span>暂无规则源</span>
          <UiButton size="sm" variant="ghost" @click="loadSources">刷新</UiButton>
        </div>
      </section>
    </Transition>

    <!-- Single-Line Zero-Dropdown Filter Deck -->
    <section class="filter-deck">
      <div class="filter-top">
        <div class="pill-groups">
          <div class="pill-group" role="group" aria-label="目标客户端">
            <button
              v-for="c in clientTabs"
              :key="c"
              type="button"
              class="seg-pill"
              :class="{ 'is-active': platform === c }"
              @click="selectPlatform(c)"
            >
              {{ c }}
            </button>
          </div>

          <span class="bar-sep" aria-hidden="true"></span>

          <div class="pill-group" role="group" aria-label="规则来源">
            <button
              v-for="s in sourceTabs"
              :key="s.value"
              type="button"
              class="seg-pill"
              :class="{ 'is-active': source === s.value }"
              @click="selectSource(s.value)"
            >
              {{ s.label }}
            </button>
          </div>
        </div>

        <form class="search-wrap" @submit.prevent="loadCatalog">
          <el-icon class="search-icon"><Search /></el-icon>
          <input
            v-model="keyword"
            type="search"
            class="search-input"
            placeholder="搜索 OpenAI / Netflix / Telegram…"
            aria-label="搜索分流规则"
          />
        </form>
      </div>

      <!-- Category Quick Pills + Select All -->
      <div v-if="categories.length" class="category-bar">
        <div class="cat-pills" role="group" aria-label="规则分类">
          <button
            v-for="item in categories"
            :key="item"
            type="button"
            class="cat-pill"
            :class="{ 'is-active': category === item }"
            @click="toggleCategory(item)"
          >
            {{ item }}
          </button>
        </div>

        <button
          v-if="rules.length"
          type="button"
          class="select-all-btn"
          @click="selectAll"
        >
          {{ allSelected ? "取消全选" : "全选当前" }}
        </button>
      </div>
    </section>

    <!-- Full-Width Pure Gallery Rule Catalog -->
    <UiSkeleton v-if="loading" :lines="6" height="60px" />

    <EmptyState
      v-else-if="!rules.length"
      size="section"
      :title="errorText ? '目录加载失败' : '暂无匹配规则'"
    >
      <template #action>
        <UiButton variant="ghost" @click="errorText ? loadCatalog() : resetFilters()">
          {{ errorText ? "重试" : "重置筛选" }}
        </UiButton>
      </template>
    </EmptyState>

    <div v-else class="rule-list" role="list">
      <article
        v-for="row in rules"
        :key="row.externalId"
        class="rule-row"
        :class="{
          'is-selected': selectedIds.includes(row.externalId),
          'is-focused': focusedRule?.externalId === row.externalId,
        }"
        role="listitem"
      >
        <label class="row-check">
          <input
            type="checkbox"
            :aria-label="`选择规则 ${row.name}`"
            :checked="selectedIds.includes(row.externalId)"
            @change="toggleSelected(row.externalId)"
          />
        </label>

        <button type="button" class="row-main" @click="inspect(row)">
          <div class="row-title-line">
            <strong class="row-name">{{ row.name || "未命名规则" }}</strong>
            <span class="row-cat">{{ row.category || "其他" }}</span>
            <span class="row-fmt mono">
              {{ row.platform }} / {{ row.format
              }}<template v-if="row.behavior && row.behavior !== 'classical'">
                · {{ row.behavior }}</template
              >
            </span>
          </div>
          <span class="row-sub">
            {{ sourceLabel(row.sourceKey, sources) }}
            <template v-if="row.duplicateCount">
              · 已去重 +{{ row.duplicateCount }}
            </template>
            <template v-if="row.remoteUpdate">
              · <span class="mono">{{ row.remoteUpdate }}</span>
            </template>
          </span>
        </button>

        <span class="row-count mono">
          {{ row.checksum ? `${row.ruleCount} 条` : "缓存中" }}
        </span>

        <div class="row-actions">
          <button
            type="button"
            class="action-pill"
            @click="inspect(row)"
          >
            <el-icon><View /></el-icon>
            <span>预览</span>
          </button>
          <button
            type="button"
            class="action-pill is-primary"
            @click="openImport([row])"
          >
            <el-icon><Plus /></el-icon>
            <span>加入模板</span>
          </button>
        </div>
      </article>
    </div>

    <!-- Clean Load More Footer -->
    <footer v-if="!loading && rules.length" class="catalog-foot">
      <span class="mono muted">已加载 {{ rules.length }} / {{ total }}</span>
      <UiButton
        v-if="hasMore"
        variant="ghost"
        :loading="loadingMore"
        @click="loadMoreCatalog"
      >
        {{ loadMoreError ? "重试加载" : "加载更多" }}
      </UiButton>
    </footer>

    <!-- Floating Obsidian Bulk Import Dock -->
    <Transition name="dock-pop">
      <div v-if="selectedRules.length" class="bulk-dock">
        <span class="mono">已选 {{ selectedRules.length }} 条规则</span>
        <button
          type="button"
          class="dock-btn is-primary"
          @click="openImport(selectedRules)"
        >
          <el-icon><FolderChecked /></el-icon>
          <span>批量加入模板</span>
        </button>
        <button
          type="button"
          class="dock-btn"
          @click="selectedIds = []"
        >
          清空
        </button>
      </div>
    </Transition>

    <!-- Preserved Preview Drawer & Import Dialog -->
    <RuleDialogs />

    <!-- Minimal Custom GitHub Rule Source Dialog -->
    <UiDialog
      v-model="addSourceOpen"
      title="添加自定义 GitHub 规则源"
      width="500px"
    >
      <el-form label-position="top" @submit.prevent="submitCustomSource">
        <el-form-item label="名称" required>
          <el-input v-model="customForm.name" placeholder="例如：EverythingRules" />
        </el-form-item>
        <el-form-item label="GitHub 仓库" required>
          <el-input
            v-model="customForm.repo"
            placeholder="owner/repo（例如 Xioaruan912/EverythingRules）"
          />
        </el-form-item>
        <div class="src-form-grid">
          <el-form-item label="分支">
            <el-input v-model="customForm.branch" placeholder="main" />
          </el-form-item>
          <el-form-item label="规则目录前缀">
            <el-input v-model="customForm.pathPrefix" placeholder="例如 rules/clash" />
          </el-form-item>
          <el-form-item label="客户端平台">
            <el-select v-model="customForm.platform" style="width: 100%">
              <el-option label="Clash" value="Clash" />
              <el-option label="Surge" value="Surge" />
              <el-option label="Loon" value="Loon" />
            </el-select>
          </el-form-item>
          <el-form-item label="规则行为">
            <el-select v-model="customForm.behavior" style="width: 100%">
              <el-option label="classical（标准规则）" value="classical" />
              <el-option label="domain（纯域名）" value="domain" />
              <el-option label="ipcidr（纯 IP 段）" value="ipcidr" />
            </el-select>
          </el-form-item>
        </div>
      </el-form>
      <template #footer>
        <UiButton variant="ghost" @click="addSourceOpen = false">取消</UiButton>
        <UiButton
          variant="primary"
          :loading="savingSource"
          :disabled="!customForm.name.trim() || !customForm.repo.trim()"
          @click="submitCustomSource"
        >
          添加并同步
        </UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.rules-gallery {
  display: grid;
  gap: 18px;
  max-width: 1180px;
  min-width: 0;
  padding-bottom: 64px;
}

.rules-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: baseline;
  gap: 14px;
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
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.count-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.count-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

/* Collapsible Remote Sources */
.sources-strip {
  display: grid;
  gap: 12px;
}

.sources-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.src-actions {
  display: inline-flex;
  align-items: center;
  gap: 10px;
}

.src-link-btn {
  border: 0;
  padding: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
}

.src-link-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

.src-link-btn.is-danger {
  color: var(--c-bad);
}

.src-form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0 14px;
}

.source-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 12px;
}

.source-card {
  display: grid;
  gap: 6px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}

.source-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
}

.src-state {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.src-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.src-state.is-ok {
  color: var(--c-ok);
}

.src-state.is-bad {
  color: var(--c-bad);
}

.src-repo {
  font-size: 11.5px;
  color: var(--c-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.src-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: var(--c-muted);
  padding-top: 6px;
  border-top: 1px solid var(--c-line);
}

.source-empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  font-size: 12.5px;
  color: var(--c-muted);
}

/* Filter Deck */
.filter-deck {
  display: grid;
  gap: 12px;
}

.filter-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.pill-groups {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 3px;
}

.bar-sep {
  width: 1px;
  height: 16px;
  background: var(--c-line);
}

.seg-pill {
  padding: 6px 12px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
}

.seg-pill:hover {
  border-color: var(--c-line);
  color: var(--c-ink);
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.search-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: min(280px, 100%);
  padding: 6px 0;
  border-bottom: 1px solid var(--c-line);
}

.search-wrap:focus-within {
  border-bottom-color: var(--c-ink);
}

.search-icon {
  color: var(--c-muted);
}

.search-input {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 13px;
  outline: none !important;
}

.category-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.cat-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.cat-pill {
  padding: 4px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11.5px;
  cursor: pointer;
}

.cat-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.cat-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.select-all-btn {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.select-all-btn:hover {
  color: var(--c-ink);
}

/* Borderless Full-Width Gallery List */
.rule-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.rule-row {
  position: relative;
  display: grid;
  grid-template-columns: 24px minmax(0, 1fr) 88px auto;
  align-items: center;
  gap: 14px;
  padding: 15px 10px;
  border-bottom: 1px solid var(--c-line);
  transition: background 180ms var(--ease-out);
}

.rule-row:hover {
  background: var(--c-surface);
}

.row-check input {
  width: 15px;
  height: 15px;
  accent-color: var(--c-ink);
  cursor: pointer;
}

.row-main {
  display: grid;
  gap: 3px;
  min-width: 0;
  padding: 0;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.row-title-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.row-name {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.row-cat {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 11px;
  color: var(--c-muted);
}

.row-fmt {
  font-size: 11px;
  color: var(--c-muted);
}

.row-sub {
  font-size: 12px;
  color: var(--c-muted);
}

.row-count {
  font-size: 12px;
  color: var(--c-muted);
  text-align: right;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.action-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.action-pill:hover {
  border-color: var(--c-ink);
}

.action-pill.is-primary:hover {
  background: var(--c-ink);
  color: var(--c-surface);
}

.catalog-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-top: 8px;
}

.muted {
  font-size: 12px;
  color: var(--c-muted);
}

/* Floating Obsidian Bulk Dock */
.bulk-dock {
  position: fixed;
  left: 50%;
  bottom: 28px;
  transform: translateX(-50%);
  z-index: 40;
  display: inline-flex;
  align-items: center;
  gap: 12px;
  padding: 10px 14px 10px 20px;
  border-radius: 999px;
  background: #141416;
  color: #f4f4f6;
  font-size: 12.5px;
  box-shadow: 0 22px 48px -12px rgb(0 0 0 / 0.45);
}

.dock-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 13px;
  border: 1px solid rgb(255 255 255 / 0.16);
  border-radius: 999px;
  background: transparent;
  color: #f4f4f6;
  font-size: 12px;
  cursor: pointer;
}

.dock-btn.is-primary {
  background: #ffffff;
  border-color: #ffffff;
  color: #111113;
  font-weight: 600;
}

.dock-pop-enter-active,
.dock-pop-leave-active {
  transition: opacity 200ms var(--ease-out), transform 240ms cubic-bezier(0.16, 1, 0.3, 1);
}

.dock-pop-enter-from,
.dock-pop-leave-to {
  opacity: 0;
  transform: translate(-50%, 14px);
}

@media (max-width: 780px) {
  .rule-row {
    grid-template-columns: 24px minmax(0, 1fr);
    gap: 8px 10px;
  }
  .row-count,
  .row-actions {
    grid-column: 2;
    text-align: left;
  }
}
</style>
