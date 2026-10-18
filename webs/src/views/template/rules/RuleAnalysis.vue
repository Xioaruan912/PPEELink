<script setup lang="ts">
import { EmptyState, UiButton, UiSkeleton } from "@/ui";
import { useRules, ruleText, sourceLabel } from "./useRules";
defineProps<{ compact?: boolean }>();
const { focusedRule, previewLoading, preview, openPreview } = useRules();
</script>
<template>
  <div class="analysis-content" :class="{ compact }">
    <EmptyState
      v-if="!focusedRule"
      size="section"
      title="检查一项规则"
      description="从目录选择一项查看规则内容。"
    />
    <template v-else>
      <div class="analysis-heading">
        <h3>{{ focusedRule.name }}</h3>
        <span class="muted"
          >{{ sourceLabel(focusedRule.sourceKey) }} ·
          {{ focusedRule.platform }} · {{ focusedRule.format }}</span
        >
      </div>
      <template v-if="focusedRule">
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
    </template>
  </div>
</template>
<style scoped>
.analysis-content {
  min-width: 0;
  color: var(--c-ink);
}
h3 {
  font-size: 17px;
  line-height: 1.4;
  margin: 0 0 8px;
  overflow-wrap: anywhere;
}
h4 {
  font-size: 14px;
  margin: 20px 0 10px;
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
.count-line {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 14px;
}
.rule-code {
  margin: 10px 0;
  padding: 12px;
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
  margin: 16px 0;
  display: grid;
  gap: 10px;
}
.metadata > div {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  font-size: 14px;
}
.metadata dd {
  margin: 0;
}
.notice {
  padding: 10px 12px;
  margin: 12px 0;
  border-left: 2px solid var(--c-data);
  background: var(--c-surface-2);
  font-size: 14px;
  overflow-wrap: anywhere;
}
.notice.warn {
  border-color: var(--c-warn);
}
@media (max-width: 767px) {
  .diff-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
