<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import { useMediaQuery } from "@vueuse/core";
import { Refresh, FullScreen } from "@element-plus/icons-vue";
import { PageHeader, UiButton, StatusDot, EmptyState, UiSkeleton } from "@/ui";
import { useMotion, useStagger } from "@/motion";
import RuleAnalysis from "./rules/RuleAnalysis.vue";
import RuleDialogs from "./rules/RuleDialogs.vue";
import {
  provideRules,
  sourceLabel,
  metadataSummary,
  fmtTime,
  type RuleItem,
} from "./rules/useRules";
defineOptions({ name: "RulesWorkbench" });
const {
  loading,
  loadingMore,
  loadMoreError,
  hasMore,
  sourcesLoading,
  syncing,
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
  detailOpen,
  loadSources,
  loadCatalog,
  loadMoreCatalog,
  resetFilters,
  toggleCategory,
  toggleSelected,
  doSync,
  openPreview,
  openImport,
} = provideRules();
const root = ref<HTMLElement>();
const mobile = useMediaQuery("(max-width: 767px)");
function onCatalogScroll(event: Event) {
  if (
    mobile.value ||
    loading.value ||
    loadingMore.value ||
    loadMoreError.value ||
    !hasMore.value
  )
    return;
  const viewport = event.currentTarget as HTMLElement;
  if (viewport.scrollHeight - viewport.clientHeight - viewport.scrollTop <= 64)
    void loadMoreCatalog();
}
const { reduced } = useMotion();
const stagger = useStagger();
watch([loading, sourcesLoading], async () => {
  await nextTick();
  if (root.value)
    stagger(root.value.querySelectorAll(".source-row, .rule-row"));
});
function inspect(row: RuleItem) {
  void openPreview(row, mobile.value);
}
function selectAll() {
  selectedIds.value =
    selectedIds.value.length === rules.value.length
      ? []
      : rules.value.map((row) => row.externalId);
}
</script>
<template>
  <div ref="root" class="rules-page">
    <PageHeader title="规则中心" description="从规则源到模板，先检查，再应用。"
      ><template #actions
        ><UiButton
          variant="primary"
          :icon="Refresh"
          :loading="syncing"
          @click="doSync"
          >{{ syncing ? "同步中" : "立即同步" }}</UiButton
        ></template
      ></PageHeader
    >
    <p v-if="errorText" class="notice error" role="alert">{{ errorText }}</p>
    <section class="sources-section" aria-labelledby="sources-title" data-enter>
      <div class="section-heading">
        <h2 id="sources-title">规则源</h2>
        <StatusDot
          :state="syncing ? 'warn' : 'idle'"
          :running="syncing"
          :label="syncing ? '正在同步目录' : `${sources.length} 个来源`"
        />
      </div>
      <EmptyState
        v-if="sourcesError"
        size="section"
        title="无法读取规则源"
        :description="sourcesError"
        ><template #action
          ><UiButton @click="loadSources">重新加载</UiButton></template
        ></EmptyState
      >
      <UiSkeleton v-else-if="sourcesLoading" :lines="2" height="64px" />
      <EmptyState
        v-else-if="!sources.length"
        size="section"
        title="尚无规则源"
        description="同步后查看可用的远程规则目录。"
        ><template #action
          ><UiButton :loading="syncing" @click="doSync"
            >同步规则源</UiButton
          ></template
        ></EmptyState
      >
      <TransitionGroup
        v-else
        tag="div"
        :name="reduced ? '' : 'rules'"
        class="source-list"
      >
        <article v-for="item in sources" :key="item.key" class="source-row">
          <div class="source-name">
            <strong>{{ item.name }}</strong
            ><span class="mono muted">{{ item.repo }}</span
            ><span v-if="item.error" class="source-error">{{
              item.error
            }}</span>
          </div>
          <StatusDot
            :state="item.status === 'ok' ? 'ok' : item.error ? 'bad' : 'idle'"
            :label="
              item.status === 'ok'
                ? '已同步'
                : item.error
                  ? '同步失败'
                  : '尚未同步'
            "
          />
          <div class="source-count mono">
            <span>{{ item.count || 0 }} 条目录</span
            ><small>已缓存 {{ item.cachedCount || 0 }}</small>
          </div>
          <time class="source-time mono">{{ fmtTime(item.lastSyncAt) }}</time>
        </article>
      </TransitionGroup>
    </section>
    <div class="workbench">
      <section
        class="catalog-section"
        aria-labelledby="catalog-title"
        data-enter
      >
        <div class="section-heading">
          <h2 id="catalog-title">
            规则目录 <span class="mono total">{{ total }}</span>
          </h2>
          <span class="muted load-count"
            >已加载 {{ rules.length }} / {{ total }}</span
          >
        </div>
        <form class="filter-form" @submit.prevent="loadCatalog">
          <el-input
            v-model="keyword"
            clearable
            class="search-input"
            aria-label="搜索分流规则"
            placeholder="搜索 OpenAI / Netflix / Telegram…"
          /><el-select
            v-model="source"
            aria-label="规则来源"
            placeholder="全部来源"
            @change="loadCatalog"
            ><el-option label="全部来源" value="" /><el-option
              label="ShuntRules"
              value="shunt_rules" /><el-option
              label="ios_rule_script"
              value="ios_rule_script" /></el-select
          ><el-select
            v-model="platform"
            aria-label="目标客户端"
            @change="loadCatalog"
            ><el-option
              v-for="client in ['Clash', 'Surge', 'Loon']"
              :key="client"
              :label="client"
              :value="client"
          /></el-select>
          <div class="filter-actions">
            <UiButton type="submit" :loading="loading">搜索</UiButton
            ><UiButton variant="ghost" @click="resetFilters">重置</UiButton>
          </div>
        </form>
        <div class="category-list" role="group" aria-label="规则分类">
          <button
            v-for="item in categories"
            :key="item"
            type="button"
            :aria-pressed="category === item"
            :class="{ active: category === item }"
            @click="toggleCategory(item)"
          >
            {{ item }}
          </button>
        </div>
        <div class="batch-bar">
          <label class="select-all"
            ><input
              type="checkbox"
              :checked="!!rules.length && selectedIds.length === rules.length"
              :disabled="loading || !rules.length"
              @change="selectAll"
            />全选当前目录</label
          ><UiButton
            :disabled="!selectedRules.length"
            @click="openImport(selectedRules)"
            >导入已选
            <span class="mono">{{ selectedRules.length }}</span></UiButton
          >
        </div>
        <div
          class="catalog-viewport"
          role="region"
          aria-label="规则目录滚动列表"
          tabindex="0"
          :aria-busy="loading || loadingMore"
          @scroll.passive="onCatalogScroll"
        >
          <UiSkeleton v-if="loading" :lines="6" height="72px" />
          <EmptyState
            v-else-if="!rules.length"
            size="section"
            :title="errorText ? '目录加载失败' : '没有匹配规则'"
            :description="errorText || '试试其他关键词、来源或客户端。'"
            ><template #action
              ><UiButton @click="errorText ? loadCatalog() : resetFilters()">{{
                errorText ? "重新加载" : "重置筛选"
              }}</UiButton></template
            ></EmptyState
          >
          <TransitionGroup
            v-else
            tag="div"
            class="catalog-list"
            :name="reduced ? '' : 'rules'"
          >
            <article
              v-for="row in rules"
              :key="row.externalId"
              class="rule-row"
              :class="{
                'is-focused': focusedRule?.externalId === row.externalId,
              }"
            >
              <label class="row-check"
                ><input
                  type="checkbox"
                  :aria-label="`选择规则 ${row.name}`"
                  :checked="selectedIds.includes(row.externalId)"
                  @change="toggleSelected(row.externalId)"
              /></label>
              <div class="rule-description">
                <strong>{{ row.name || "未命名规则" }}</strong
                ><span
                  >{{ sourceLabel(row.sourceKey) }} ·
                  {{ row.category || "其他" }} ·
                  <span class="mono"
                    >{{ row.platform }} / {{ row.format }}</span
                  ></span
                ><small v-if="metadataSummary(row)">{{
                  metadataSummary(row)
                }}</small
                ><small v-if="row.remoteUpdate" class="mono">{{
                  row.remoteUpdate
                }}</small>
              </div>
              <div class="rule-size mono">
                {{ row.checksum ? `${row.ruleCount} 条` : "自动缓存中…" }}
              </div>
              <div class="row-actions">
                <UiButton variant="ghost" @click="inspect(row)">预览</UiButton
                ><UiButton variant="ghost" @click="openImport([row])"
                  >加入模板</UiButton
                >
              </div>
            </article>
          </TransitionGroup>
          <div
            v-if="!loading && rules.length"
            class="catalog-pagination"
            aria-live="polite"
          >
            <p v-if="loadMoreError" class="notice error" role="alert">
              {{ loadMoreError }}
            </p>
            <span class="muted mono"
              >已加载 {{ rules.length }} / {{ total }}</span
            >
            <UiButton
              v-if="hasMore"
              :loading="loadingMore"
              @click="loadMoreCatalog"
              >{{ loadMoreError ? "重试加载" : "加载更多" }}</UiButton
            >
            <span v-else class="muted">已加载全部规则</span>
          </div>
        </div>
        <section
          v-if="selectedRules.length"
          class="selected-section"
          aria-label="已选规则"
        >
          <h3>
            已选规则 <span class="mono">{{ selectedRules.length }}</span>
          </h3>
          <TransitionGroup tag="div" :name="reduced ? '' : 'rules'"
            ><div
              v-for="row in selectedRules"
              :key="row.externalId"
              class="selected-row"
            >
              <div>
                <strong>{{ row.name }}</strong
                ><span class="muted"
                  >{{ sourceLabel(row.sourceKey) }} · {{ row.platform }}</span
                >
              </div>
              <UiButton
                variant="ghost"
                :aria-label="`移除 ${row.name}`"
                @click="toggleSelected(row.externalId)"
                >移除</UiButton
              >
            </div></TransitionGroup
          >
        </section>
      </section>
      <aside
        class="analysis-section"
        aria-labelledby="analysis-title"
        data-enter
      >
        <div class="section-heading">
          <h2 id="analysis-title">预览与影响</h2>
          <UiButton
            v-if="focusedRule"
            variant="ghost"
            :icon="FullScreen"
            aria-label="展开规则分析"
            @click="detailOpen = true"
            >展开</UiButton
          >
        </div>
        <RuleAnalysis compact />
      </aside>
    </div>
    <RuleDialogs />
  </div>
</template>
<style scoped src="./rules/workbench.scss"></style>
