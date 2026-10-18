<script setup lang="ts">
import { computed } from "vue";
import { EmptyState, StatusDot, UiButton, UiSkeleton } from "@/ui";
import { useRules, ruleText, sourceLabel, fmtTime } from "./useRules";
defineProps<{ compact?: boolean }>();
const {
  focusedRule,
  analysisMode,
  previewLoading,
  preview,
  impactLoading,
  impact,
  snapshots,
  operationLoading,
  openPreview,
  openImpact,
  applyUpdateNow,
  rollbackSnapshot,
} = useRules();
const counts = computed(() => {
  const value = impact.value?.preview;
  return value
    ? [
        { label: "新增", value: `+${value.addedCount}`, kind: "added" },
        { label: "删除", value: `−${value.deletedCount}`, kind: "deleted" },
        { label: "变更", value: value.modifiedCount, kind: "changed" },
        { label: "重复", value: value.duplicateCount },
        { label: "前序覆盖", value: value.coveredCount },
      ]
    : [];
});
</script>
<template>
  <div class="analysis-content" :class="{ compact }">
    <EmptyState
      v-if="!focusedRule"
      size="section"
      title="检查一项规则"
      description="从目录选择预览或查看变化。"
    />
    <template v-else>
      <div class="analysis-heading">
        <h3>{{ focusedRule.name }}</h3>
        <span class="muted"
          >{{ sourceLabel(focusedRule.sourceKey) }} ·
          {{ focusedRule.platform }} · {{ focusedRule.format }}</span
        >
      </div>
      <div class="analysis-tabs" role="group" aria-label="规则分析视图">
        <UiButton
          :variant="analysisMode === 'preview' ? 'secondary' : 'ghost'"
          :aria-pressed="analysisMode === 'preview'"
          @click="openPreview(focusedRule)"
          >规则预览</UiButton
        >
        <UiButton
          :variant="analysisMode === 'impact' ? 'secondary' : 'ghost'"
          :aria-pressed="analysisMode === 'impact'"
          @click="openImpact(focusedRule)"
          >影响分析</UiButton
        >
      </div>
      <template v-if="analysisMode === 'preview'">
        <UiSkeleton v-if="previewLoading" :lines="5" height="32px" />
        <EmptyState
          v-else-if="preview?.error"
          size="section"
          title="预览失败"
          :description="preview.error"
          ><template #action
            ><UiButton @click="openPreview(focusedRule)"
              >重试预览</UiButton
            ></template
          ></EmptyState
        >
        <template v-else-if="preview">
          <div class="count-line">
            <span class="muted">规则样例</span
            ><span class="mono">{{ preview.ruleCount }} 条</span>
          </div>
          <div
            v-for="warning in preview.warnings || []"
            :key="warning"
            class="notice warn"
            role="status"
          >
            {{ warning }}
          </div>
          <pre v-if="preview.sample?.length" class="rule-code">{{
            preview.sample.map(ruleText).join("\n")
          }}</pre>
          <EmptyState
            v-else
            size="inline"
            description="此规则没有可显示的样例。"
          />
          <dl class="metadata">
            <div>
              <dt>来源</dt>
              <dd>{{ sourceLabel(preview.sourceKey) }}</dd>
            </div>
            <div>
              <dt>客户端</dt>
              <dd>{{ preview.platform }}</dd>
            </div>
            <div>
              <dt>格式</dt>
              <dd>{{ preview.format }}</dd>
            </div>
          </dl>
        </template>
        <EmptyState v-else size="inline" description="未返回可预览内容。" />
      </template>
      <template v-else>
        <UiSkeleton v-if="impactLoading" :lines="6" height="40px" />
        <EmptyState
          v-else-if="impact?.error"
          size="section"
          title="影响分析失败"
          :description="impact.error"
          ><template #action
            ><UiButton @click="openImpact(focusedRule)"
              >重新分析</UiButton
            ></template
          ></EmptyState
        >
        <template v-else-if="impact?.preview">
          <div class="count-line">
            <span class="muted">规则数量</span
            ><strong class="mono"
              >{{ impact.preview.oldCount }} →
              {{ impact.preview.newCount }}</strong
            >
          </div>
          <div class="impact-counts">
            <div v-for="count in counts" :key="count.label">
              <span>{{ count.label }}</span
              ><strong class="mono" :class="count.kind">{{
                count.value
              }}</strong>
            </div>
          </div>
          <StatusDot
            :state="impact.preview.changed ? 'warn' : 'ok'"
            :label="
              impact.preview.changed
                ? '远端内容有变化，应用前请检查影响。'
                : '与当前缓存一致，无需更新。'
            "
          />
          <div
            v-for="warning in impact.warnings || []"
            :key="warning"
            class="notice warn"
            role="status"
          >
            {{ warning }}
          </div>
          <h4>受影响模板</h4>
          <div
            v-for="item in impact.affectedTemplates || []"
            :key="`${item.template}-${item.policy}`"
            class="detail-row"
          >
            <span class="mono">{{ item.template }}</span
            ><span>{{ item.policy || "未指定策略" }}</span>
          </div>
          <EmptyState
            v-if="!impact.affectedTemplates?.length"
            size="inline"
            description="暂无模板引用。"
          />
          <h4>分流回归变化</h4>
          <div
            v-for="(row, index) in impact.regression || []"
            :key="`${row.template}-${row.domain}-${index}`"
            class="regression-row"
          >
            <strong class="mono">{{ row.domain }}</strong
            ><span>{{ row.target }} · {{ row.policy || "未指定策略" }}</span
            ><span class="mono">{{ row.template }}</span
            ><span :class="row.changed ? 'changed' : 'muted'"
              >{{ row.beforeInSet ? "命中" : "不命中" }} →
              {{ row.afterInSet ? "命中" : "不命中" }}</span
            >
          </div>
          <EmptyState
            v-if="!impact.regression?.length"
            size="inline"
            description="未发现分流回归变化。"
          />
          <div class="diff-grid">
            <section>
              <h4 class="added">新增样例</h4>
              <pre v-if="impact.preview.added?.length" class="rule-code">{{
                impact.preview.added.slice(0, 30).map(ruleText).join("\n")
              }}</pre>
              <EmptyState v-else size="inline" description="没有新增规则。" />
            </section>
            <section>
              <h4 class="deleted">删除样例</h4>
              <pre v-if="impact.preview.deleted?.length" class="rule-code">{{
                impact.preview.deleted.slice(0, 30).map(ruleText).join("\n")
              }}</pre>
              <EmptyState v-else size="inline" description="没有删除规则。" />
            </section>
          </div>
          <section v-if="impact.preview.modified?.length">
            <h4 class="changed">变更样例</h4>
            <div
              v-for="(change, index) in impact.preview.modified.slice(0, 30)"
              :key="index"
              class="modification"
            >
              <pre class="rule-code deleted">
− {{ ruleText(change.before) }}</pre>
              <pre class="rule-code added">+ {{ ruleText(change.after) }}</pre>
            </div>
          </section>
          <UiButton
            class="update-button"
            variant="danger"
            :loading="operationLoading"
            :disabled="!impact.preview.changed"
            @click="applyUpdateNow"
            >保存快照并应用更新</UiButton
          >
        </template>
        <EmptyState v-else size="inline" description="未返回影响分析。" />
        <section v-if="!impactLoading" aria-label="可回滚快照">
          <h4>可回滚快照</h4>
          <div v-if="impact?.snapshotError" class="notice warn" role="alert">
            {{ impact?.snapshotError }}
          </div>
          <div
            v-for="snapshot in snapshots"
            :key="snapshot.id"
            class="snapshot-row"
          >
            <div>
              <strong class="mono"
                >#{{ snapshot.id }} · {{ snapshot.ruleCount }} 条</strong
              ><time>{{ fmtTime(snapshot.createdAt) }}</time>
            </div>
            <UiButton
              variant="danger"
              :disabled="operationLoading"
              @click="rollbackSnapshot(snapshot.id)"
              >回滚</UiButton
            >
          </div>
          <EmptyState
            v-if="!snapshots.length && !impact?.snapshotError"
            size="inline"
            description="尚无历史快照；应用更新后将自动创建。"
          />
        </section>
      </template>
    </template>
  </div>
</template>
<style scoped>
.analysis-content {
  min-width: 0;
  color: var(--c-ink);
}
h3 {
  font-size: 22px;
  line-height: 1.4;
  margin: 0 0 8px;
  overflow-wrap: anywhere;
}
h4 {
  font-size: 16px;
  margin: 28px 0 12px;
}
.muted,
.metadata dt {
  color: var(--c-muted);
  font-size: 14px;
}
.mono,
.rule-code {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.analysis-tabs {
  display: flex;
  gap: 8px;
  margin: 20px 0 24px;
  flex-wrap: wrap;
}
.analysis-tabs :deep(.ui-button) {
  padding-inline: 12px;
}
.count-line,
.detail-row {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 14px;
}
.detail-row {
  flex-wrap: wrap;
  overflow-wrap: anywhere;
}
.rule-code {
  margin: 12px 0;
  padding: 16px;
  max-height: 360px;
  overflow: auto;
  background: var(--c-surface-2);
  color: var(--c-ink);
  border: 1px solid var(--c-line);
  border-radius: 10px;
  font-size: 14px;
  line-height: 1.8;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  min-width: 0;
}
.metadata {
  margin: 24px 0;
  display: grid;
  gap: 12px;
}
.metadata > div {
  display: flex;
  gap: 16px;
  justify-content: space-between;
  font-size: 14px;
}
.metadata dd {
  margin: 0;
}
.impact-counts {
  display: flex;
  flex-wrap: wrap;
  gap: 16px 24px;
  padding: 24px 0;
}
.impact-counts > div {
  display: grid;
  gap: 4px;
}
.impact-counts span {
  color: var(--c-muted);
  font-size: 14px;
}
.impact-counts strong {
  font-size: 22px;
}
.added {
  color: var(--c-ok);
}
.deleted {
  color: var(--c-bad);
}
.changed {
  color: var(--c-data);
}
.notice {
  padding: 12px 16px;
  margin: 16px 0;
  border-left: 2px solid var(--c-data);
  background: var(--c-surface-2);
  font-size: 14px;
  overflow-wrap: anywhere;
}
.notice.warn {
  border-color: var(--c-warn);
}
.regression-row {
  display: grid;
  gap: 8px;
  font-size: 14px;
  padding: 16px 0;
  border-bottom: 1px solid var(--c-line);
  overflow-wrap: anywhere;
}
.diff-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
}
.diff-grid section {
  min-width: 0;
}
.snapshot-row {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  align-items: center;
  padding: 16px 0;
  border-bottom: 1px solid var(--c-line);
}
.snapshot-row > div {
  min-width: 0;
  display: grid;
  gap: 4px;
  font-size: 14px;
}
.snapshot-row time {
  font-size: 14px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.update-button {
  margin-top: 24px;
  max-width: 100%;
  white-space: normal;
}
.compact .diff-grid {
  grid-template-columns: minmax(0, 1fr);
}
.modification .rule-code {
  margin: 0;
  border-radius: 0;
}
.modification {
  margin: 12px 0;
}
.modification .deleted {
  color: var(--c-bad);
}
.modification .added {
  color: var(--c-ok);
}
@media (max-width: 767px) {
  .diff-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
