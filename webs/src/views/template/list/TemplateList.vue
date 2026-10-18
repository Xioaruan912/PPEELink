<script setup lang="ts">
import { nextTick, ref, watch } from "vue";
import { Plus, Edit, MoreFilled } from "@element-plus/icons-vue";
import { UiButton, UiStat, UiSkeleton, EmptyState, StatusDot } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useMotion, useStagger } from "@/motion";
import { formatDateTime } from "@/utils/date";
import { useTemplates, tempType, fileSize, type Temp } from "./useTemplates";
const {
  tableData,
  filteredData,
  searchKey,
  loading,
  loadError,
  latestVersions,
  versionFailures,
  checkedTemplates,
  gettemps,
  handleAddTemp,
  handleEdit,
  copyText,
  handleDel,
} = useTemplates();
const root = ref<HTMLElement>();
const { reduced } = useMotion();
const stagger = useStagger();
watch(
  loading,
  async (busy) => {
    if (!busy) {
      await nextTick();
      if (root.value) stagger(root.value.querySelectorAll(".template-row"));
    }
  },
  { immediate: true }
);
const check = (row: Temp) => {
  const value = checkedTemplates.value[row.file];
  return value?.text === row.text
    ? value.valid
      ? "passed"
      : "failed"
    : "unchecked";
};
function command(value: string, row: Temp) {
  if (value === "copy") void copyText(row);
  else if (value === "delete") void handleDel(row);
}
</script>
<template>
  <section :aria-busy="loading">
    <div class="template-summary" data-enter>
      <UiStat size="large" label="模板总数" :value="tableData.length" />
      <p>以版本留住每次修改。<br />保存前，先检查规则与客户端兼容性。</p>
    </div>
    <div class="template-toolbar" data-enter>
      <el-input
        v-model="searchKey"
        clearable
        aria-label="搜索模板文件名"
        placeholder="搜索模板文件名"
      />
      <span class="muted mono"
        >{{ filteredData.length }} / {{ tableData.length }}</span
      >
      <UiButton :loading="loading" @click="gettemps">刷新</UiButton>
    </div>
    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载模板"
      :description="loadError"
      ><template #action
        ><UiButton @click="gettemps">重新加载</UiButton></template
      ></EmptyState
    >
    <UiSkeleton
      v-else-if="loading && !tableData.length"
      :lines="4"
      height="72px"
    />
    <div v-else ref="root">
      <EmptyState
        v-if="!tableData.length"
        size="page"
        title="写下你的连接规则"
        description="添加模板，或前往构建器生成配置。"
        ><template #action
          ><UiButton :icon="Plus" @click="handleAddTemp"
            >添加模板</UiButton
          ></template
        ></EmptyState
      >
      <EmptyState
        v-else-if="!filteredData.length"
        size="section"
        title="没有匹配模板"
        description="试试其他文件名。"
        ><template #action
          ><UiButton @click="searchKey = ''">清除搜索</UiButton></template
        ></EmptyState
      >
      <div
        v-if="filteredData.length"
        class="template-heading"
        aria-hidden="true"
      >
        <span>名称 / 客户端</span><span>更新时间</span><span>版本</span
        ><span>语法校验</span><span class="end">操作</span>
      </div>
      <TransitionGroup tag="div" :name="reduced ? '' : 'template-row'">
        <article
          v-for="row in filteredData"
          :key="row.file"
          class="template-row"
        >
          <button class="template-name" @click="handleEdit(row)">
            <strong>{{ row.file }}</strong
            ><span
              >{{ tempType(row.file) }}
              <span class="mono"> / {{ fileSize(row.text) }}</span></span
            >
          </button>
          <div class="row-value">
            <span class="mobile-label">更新时间</span
            ><time class="mono" :datetime="row.create_date">{{
              formatDateTime(row.create_date) || "未提供"
            }}</time>
          </div>
          <div class="row-value">
            <span class="mobile-label">版本</span
            ><span class="mono">{{
              versionFailures[row.file]
                ? "读取失败"
                : latestVersions[row.file] === undefined
                  ? "读取中"
                  : latestVersions[row.file] === null
                    ? "无历史"
                    : `#${latestVersions[row.file]}`
            }}</span>
          </div>
          <div class="row-value">
            <span class="mobile-label">语法校验</span
            ><StatusDot
              :state="
                check(row) === 'passed'
                  ? 'ok'
                  : check(row) === 'failed'
                    ? 'bad'
                    : 'idle'
              "
              :label="
                check(row) === 'passed'
                  ? '本次已通过'
                  : check(row) === 'failed'
                    ? '检查未通过'
                    : '未校验'
              "
            />
          </div>
          <div class="row-actions">
            <RowIconButton
              :icon="Edit"
              :label="`编辑 ${row.file}`"
              @click="handleEdit(row)"
            /><el-dropdown
              trigger="click"
              @command="(value: string) => command(value, row)"
              ><RowIconButton
                :icon="MoreFilled"
                :label="`${row.file} 更多操作`"
              /><template #dropdown
                ><el-dropdown-menu
                  ><el-dropdown-item command="copy">复制内容</el-dropdown-item
                  ><el-dropdown-item command="delete" divided
                    ><span class="danger-text">删除模板</span></el-dropdown-item
                  ></el-dropdown-menu
                ></template
              ></el-dropdown
            >
          </div>
        </article>
      </TransitionGroup>
    </div>
  </section>
</template>
<style scoped>
.template-summary {
  display: flex;
  align-items: end;
  gap: 64px;
  margin-bottom: 40px;
}
.template-summary p {
  margin: 0 0 4px;
  color: var(--c-muted);
  font-size: 14px;
}
.template-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}
.template-toolbar :deep(.el-input) {
  width: min(360px, 100%);
}
.template-toolbar > .ui-button {
  margin-left: auto;
}
.template-heading,
.template-row {
  display: grid;
  grid-template-columns:
    minmax(160px, 1.6fr) minmax(160px, 1fr) minmax(85px, 0.6fr)
    minmax(110px, 0.8fr) 88px;
  gap: 16px;
  align-items: center;
}
.template-heading {
  padding: 12px 16px;
  font: 400 13px/1.4 var(--font-mono);
  color: var(--c-muted);
  border-bottom: 1px solid var(--c-line);
}
.template-row {
  padding: 12px 16px;
  min-height: 80px;
  border-bottom: 1px solid var(--c-line);
}
.template-row:hover {
  background: var(--c-surface-2);
}
.template-name {
  min-width: 0;
  min-height: 44px;
  display: grid;
  text-align: left;
  gap: 4px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink);
}
.template-name strong {
  font-size: 15px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.template-name > span,
.row-value {
  font-size: 14px;
  color: var(--c-muted);
}
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.muted {
  color: var(--c-muted);
  font-size: 14px;
  white-space: nowrap;
}
.row-actions {
  display: flex;
  justify-content: end;
}
.end {
  text-align: right;
}
.danger-text {
  color: var(--c-bad);
}
.mobile-label {
  display: none;
}
.template-row-move,
.template-row-enter-active,
.template-row-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-spring),
    opacity var(--duration-medium);
}
.template-row-enter-from,
.template-row-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
@media (max-width: 1023px) {
  .template-heading,
  .template-row {
    grid-template-columns:
      minmax(140px, 1fr) minmax(155px, 1fr)
      85px 110px 88px;
    gap: 8px;
  }
}
@media (max-width: 1023px) {
  .template-summary {
    align-items: start;
    flex-direction: column;
    gap: 24px;
    margin-bottom: 32px;
  }
  .template-toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }
  .template-toolbar :deep(.el-input) {
    flex: 1 1 100%;
    width: 100%;
  }
  .template-heading {
    display: none;
  }
  .template-row {
    grid-template-columns: minmax(0, 1fr) 88px;
    gap: 12px;
    padding: 16px 0;
  }
  .template-name {
    grid-column: 1;
  }
  .row-value {
    grid-column: 1 / -1;
    display: flex;
    justify-content: space-between;
    gap: 16px;
  }
  .mobile-label {
    display: inline;
  }
  .row-actions {
    grid-column: 2;
    grid-row: 1;
  }
}
@media (prefers-reduced-motion: reduce) {
  .template-row-move,
  .template-row-enter-active,
  .template-row-leave-active {
    transition: none;
  }
}
</style>
