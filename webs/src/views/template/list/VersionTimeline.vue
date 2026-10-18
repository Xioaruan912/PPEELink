<script setup lang="ts">
import { UiButton, UiSkeleton, EmptyState } from "@/ui";
import { formatDateTime } from "@/utils/date";
import { useTemplates } from "./useTemplates";
const {
  versions,
  versionLoading,
  versionError,
  loadVersions,
  compareVersion,
  rollbackVersion,
} = useTemplates();
</script>
<template>
  <div aria-label="模板版本时间线" :aria-busy="versionLoading">
    <UiSkeleton v-if="versionLoading" :lines="3" height="56px" />
    <EmptyState
      v-else-if="versionError"
      size="section"
      title="版本读取失败"
      :description="versionError"
      ><template #action
        ><UiButton @click="loadVersions">重试</UiButton></template
      ></EmptyState
    >
    <EmptyState
      v-else-if="!versions.length"
      size="section"
      title="暂无历史版本"
      description="保存修改后会创建版本。"
    />
    <ol v-else class="timeline">
      <li v-for="item in versions" :key="item.id">
        <button class="version-link" @click="compareVersion(item)">
          <strong>#{{ item.id }}</strong
          ><span>{{ item.action }}</span
          ><time :datetime="item.createdAt">{{
            formatDateTime(item.createdAt)
          }}</time>
        </button>
        <div class="version-actions">
          <UiButton variant="ghost" @click="compareVersion(item)">比较</UiButton
          ><UiButton
            variant="ghost"
            class="danger-text"
            @click="rollbackVersion(item)"
            >回滚</UiButton
          >
        </div>
      </li>
    </ol>
  </div>
</template>
<style scoped>
.timeline {
  list-style: none;
  padding: 0 0 0 12px;
  margin: 0;
  border-left: 1px solid var(--c-line);
}
.timeline li {
  position: relative;
  padding: 0 0 16px;
}
.timeline li::before {
  content: "";
  position: absolute;
  left: -20px;
  top: 16px;
  width: 7px;
  height: 7px;
  border: 1px solid var(--c-muted);
  border-radius: 50%;
  background: var(--c-surface);
}
.version-link {
  width: 100%;
  min-height: 36px;
  display: flex;
  text-align: left;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 8px;
  border: 0;
  padding: 8px 0;
  background: transparent;
  color: var(--c-ink);
}
.version-link strong,
.version-link time {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.version-link strong {
  font-size: 14px;
  color: var(--c-data);
}
.version-link span {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.version-link time {
  flex-basis: 100%;
  font-size: 14px;
  color: var(--c-muted);
}
.version-actions {
  display: flex;
  gap: 4px;
}
.version-actions .ui-button {
  padding: 8px 10px;
}
.danger-text {
  color: var(--c-bad);
}
</style>
