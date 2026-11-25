<script setup lang="ts">
import { computed, ref } from "vue";
import {
  CopyDocument,
  Delete,
  Edit,
  Plus,
  Refresh,
  Search,
} from "@element-plus/icons-vue";
import { EmptyState, UiButton, UiSkeleton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { formatDateTime } from "@/utils/date";
import {
  useTemplates,
  tempType,
  fileSize,
  isSmartClashTemplate,
  type Temp,
} from "./useTemplates";

const {
  tableData,
  filteredData,
  searchKey,
  loading,
  loadError,
  gettemps,
  handleAddTemp,
  handleEdit,
  copyText,
  handleDel,
} = useTemplates();

const typeFilter = ref<"all" | "yaml" | "conf">("all");

const displayRows = computed<Temp[]>(() => {
  if (typeFilter.value === "all") return filteredData.value;
  return filteredData.value.filter((row) => {
    const ext = row.file.toLowerCase();
    if (typeFilter.value === "yaml")
      return ext.endsWith(".yaml") || ext.endsWith(".yml");
    return ext.endsWith(".conf") || ext.endsWith(".ini");
  });
});

function formatExt(filename: string) {
  const parts = filename.split(".");
  return parts.length > 1 ? parts[parts.length - 1].toUpperCase() : "TEXT";
}
</script>

<template>
  <section class="list-stage" :aria-busy="loading">
    <!-- Single-Line Filter & Search Bar -->
    <div class="control-bar">
      <div class="pill-group" role="group" aria-label="格式筛选">
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': typeFilter === 'all' }"
          @click="typeFilter = 'all'"
        >
          全部 ({{ filteredData.length }})
        </button>
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': typeFilter === 'yaml' }"
          @click="typeFilter = 'yaml'"
        >
          Clash · YAML
        </button>
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': typeFilter === 'conf' }"
          @click="typeFilter = 'conf'"
        >
          Loon / Surge
        </button>
      </div>

      <div class="bar-right">
        <label class="search-wrap">
          <el-icon class="search-icon"><Search /></el-icon>
          <input
            v-model="searchKey"
            type="search"
            class="search-input"
            placeholder="搜索模板文件名..."
            aria-label="搜索模板文件名"
          />
        </label>
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          @click="gettemps"
        >
          刷新
        </UiButton>
      </div>
    </div>

    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载模板"
      :description="loadError"
    >
      <template #action>
        <UiButton @click="gettemps">重新加载</UiButton>
      </template>
    </EmptyState>

    <UiSkeleton v-else-if="loading && !tableData.length" :lines="4" height="64px" />

    <EmptyState
      v-else-if="!tableData.length"
      size="page"
      title="暂无配置模板"
    >
      <template #action>
        <UiButton variant="primary" :icon="Plus" @click="handleAddTemp">
          新建模板
        </UiButton>
      </template>
    </EmptyState>

    <EmptyState
      v-else-if="!displayRows.length"
      size="section"
      title="未匹配到模板文件"
    >
      <template #action>
        <UiButton
          variant="ghost"
          @click="
            searchKey = '';
            typeFilter = 'all';
          "
        >
          清空筛选
        </UiButton>
      </template>
    </EmptyState>

    <!-- Pure Gallery Full-Width Borderless Rows -->
    <div v-else class="template-list" role="list">
      <article
        v-for="(row, idx) in displayRows"
        :key="row.file"
        class="template-row"
        role="listitem"
      >
        <span class="row-index mono">{{ String(idx + 1).padStart(2, "0") }}</span>

        <button
          type="button"
          class="row-main"
          @click="handleEdit(row)"
        >
          <div class="row-title-line">
            <strong class="row-filename mono">{{ row.file }}</strong>
            <span class="ext-chip mono">{{ formatExt(row.file) }}</span>
            <span v-if="isSmartClashTemplate(row.file)" class="smart-chip">
              智能分流
            </span>
          </div>
          <span class="row-sub">
            {{ tempType(row.file) }} · <span class="mono">{{ fileSize(row.text) }}</span>
          </span>
        </button>

        <time class="row-time mono" :datetime="row.create_date">
          {{ formatDateTime(row.create_date) || "—" }}
        </time>

        <div class="row-actions">
          <RowIconButton
            :icon="Edit"
            :label="`编辑 ${row.file}`"
            @click="handleEdit(row)"
          />
          <RowIconButton
            :icon="CopyDocument"
            :label="`复制 ${row.file}`"
            @click="copyText(row)"
          />
          <RowIconButton
            :icon="Delete"
            :label="`删除 ${row.file}`"
            @click="handleDel(row)"
          />
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.list-stage {
  display: grid;
  gap: 16px;
}

.control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.seg-pill {
  padding: 6px 13px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background 160ms var(--ease-out),
    color 160ms var(--ease-out),
    border-color 160ms var(--ease-out);
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

.bar-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.search-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: min(240px, 100%);
  padding: 6px 0;
  border-bottom: 1px solid var(--c-line);
  transition: border-color 160ms var(--ease-out);
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

/* Pure Gallery Borderless Rows */
.template-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.template-list:hover .template-row:not(:hover) {
  opacity: 0.56;
}

.template-row {
  position: relative;
  display: grid;
  grid-template-columns: 36px minmax(0, 1fr) 165px auto;
  align-items: center;
  gap: 16px;
  padding: 18px 10px;
  border-bottom: 1px solid var(--c-line);
  overflow: hidden;
  transition: opacity 220ms var(--ease-out);
}

.template-row::before {
  content: "";
  position: absolute;
  inset: 0;
  background: var(--c-surface);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 320ms cubic-bezier(0.16, 1, 0.3, 1);
  z-index: 0;
}

.template-row:hover::before {
  transform: scaleX(1);
}

.template-row > * {
  position: relative;
  z-index: 1;
}

.row-index {
  font-size: 12px;
  color: var(--c-muted);
}

.row-main {
  display: grid;
  gap: 4px;
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
  gap: 10px;
  min-width: 0;
}

.row-filename {
  font-size: 15px;
  font-weight: 500;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ext-chip {
  flex: none;
  padding: 2px 7px;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  background: var(--c-surface-muted);
  font-size: 10.5px;
  font-weight: 600;
  color: var(--c-muted);
}

.smart-chip {
  flex: none;
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-ok) 12%, transparent);
  font-size: 11px;
  font-weight: 500;
  color: var(--c-ok);
}

.row-sub {
  font-size: 12px;
  color: var(--c-muted);
}

.row-time {
  font-size: 12px;
  color: var(--c-muted);
  text-align: right;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 4px;
}

@media (max-width: 780px) {
  .template-row {
    grid-template-columns: 28px minmax(0, 1fr) auto;
    gap: 8px 12px;
  }
  .row-time {
    grid-column: 2 / -1;
    text-align: left;
  }
}
</style>
