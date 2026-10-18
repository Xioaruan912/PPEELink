<script setup lang="ts">
import { UiButton, UiDrawer, UiSkeleton, EmptyState, UiTag } from "@/ui";
import { displayTime, maskSource } from "./useAirports";
import type { AirportDetail, AirportNode } from "./useAirports";
const props = defineProps<{
  modelValue: boolean;
  detail?: AirportDetail;
  loading: boolean;
  error: string;
  selected: string[];
  saving: boolean;
  saveError: string;
  allSelected: boolean;
  rtt: Record<string, number>;
  rttLoading: boolean;
  rttError: string;
  gemini: Record<string, "testing" | "ok" | "fail" | "busy">;
  geminiTesting: string;
  geminiError: string;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  retry: [];
  toggle: [name: string, checked: boolean];
  toggleAll: [];
  save: [];
  testGemini: [node: AirportNode];
  testLatency: [];
}>();
const rttText = (name: string) => {
  const value = props.rtt[name];
  return value === undefined
    ? "未测"
    : value === -2
      ? props.rttLoading
        ? "检测中"
        : "未测"
      : value < 0
        ? "超时"
        : `${value} ms`;
};
const rttTone = (name: string) => {
  const value = props.rtt[name];
  return value === undefined || value === -2
    ? "neutral"
    : value < 0
      ? "bad"
      : value < 100
        ? "ok"
        : value < 300
          ? "warn"
          : "bad";
};
const geminiText = {
  testing: "Gemini 检测中",
  ok: "Gemini 可用",
  fail: "Gemini 不可用",
  busy: "Gemini 待重试",
};
</script>
<template>
  <UiDrawer
    :model-value="modelValue"
    :title="detail?.name || '机场详情'"
    size="min(760px, 100vw)"
    :close-on-click-modal="!saving"
    :close-on-press-escape="!saving"
    :show-close="!saving"
    @update:model-value="emit('update:modelValue', $event)"
  >
    <UiSkeleton v-if="loading" :lines="5" height="64px" />
    <EmptyState
      v-else-if="error"
      size="section"
      title="无法加载详情"
      :description="error"
      ><template #action
        ><UiButton @click="emit('retry')">重新加载</UiButton></template
      ></EmptyState
    >
    <div v-else-if="detail" class="airport-detail">
      <dl class="detail-grid">
        <div>
          <dt>机场编号</dt>
          <dd class="mono">{{ detail.id }}</dd>
        </div>
        <div class="source-fact">
          <dt>订阅来源</dt>
          <dd class="mono">{{ maskSource(detail.url) }}</dd>
        </div>
        <div>
          <dt>上次同步</dt>
          <dd class="mono">{{ displayTime(detail.last_sync) }}</dd>
        </div>
        <div>
          <dt>自动清理死节点</dt>
          <dd>{{ detail.auto_cleanup ? "已开启" : "已关闭" }}</dd>
        </div>
        <div>
          <dt>国内专线</dt>
          <dd>
            {{ detail.is_dedicated ? "测活失败仍保留" : "按测活结果处理" }}
          </dd>
        </div>
        <div>
          <dt>同步保留节点</dt>
          <dd class="mono">{{ detail.node_count }}</dd>
        </div>
        <div>
          <dt>当前分组节点</dt>
          <dd class="mono">{{ detail.nodes.length }}</dd>
        </div>
      </dl>
      <section class="node-section" :aria-busy="rttLoading">
        <div class="section-heading">
          <div>
            <h3>选择节点</h3>
            <p>已选 {{ selected.length }} / {{ detail.nodes.length }}</p>
          </div>
          <UiButton
            variant="ghost"
            :disabled="!detail.nodes.length || saving"
            @click="emit('toggleAll')"
            >{{ allSelected ? "取消全选" : "全选" }}</UiButton
          >
        </div>
        <div class="latency-heading">
          <p>本地延迟由当前浏览器检测</p>
          <UiButton
            variant="ghost"
            :loading="rttLoading"
            :disabled="!detail.nodes.length"
            @click="emit('testLatency')"
            >重测延迟</UiButton
          >
        </div>
        <p v-if="rttError" class="error-text" role="alert">{{ rttError }}</p>
        <p v-if="geminiError" class="error-text" role="alert">
          {{ geminiError }}
        </p>
        <div v-if="detail.nodes.length" class="node-list">
          <article
            v-for="node in detail.nodes"
            :key="node.ID"
            class="node-row"
            :class="{ 'is-testing': gemini[node.Name] === 'testing' }"
          >
            <el-checkbox
              :model-value="selected.includes(node.Name)"
              :disabled="saving"
              :aria-label="`选择 ${node.Name}`"
              @update:model-value="emit('toggle', node.Name, !!$event)"
              ><span class="node-name" :title="node.Name">{{
                node.Name
              }}</span></el-checkbox
            >
            <span class="node-link mono" :title="node.Link">{{
              node.Link
            }}</span>
            <div class="node-metrics">
              <UiTag :tone="rttTone(node.Name)">{{ rttText(node.Name) }}</UiTag
              ><span
                v-if="gemini[node.Name]"
                class="gemini-result"
                :class="`gemini-${gemini[node.Name]}`"
                role="status"
                >{{ geminiText[gemini[node.Name]] }}</span
              ><UiButton
                variant="ghost"
                :loading="geminiTesting === node.Name"
                :disabled="!!geminiTesting"
                :aria-label="`检测 ${node.Name} 的 Gemini`"
                @click="emit('testGemini', node)"
                >测 Gemini</UiButton
              >
            </div>
          </article>
        </div>
        <EmptyState
          v-else
          size="section"
          title="还没有节点"
          description="同步机场后，在这里选择订阅节点。"
        />
      </section>
      <p class="selection-hint">不勾选节点时，订阅默认导入该机场全部节点。</p>
      <p v-if="saveError" role="alert" class="error-text">{{ saveError }}</p>
    </div>
    <template #footer
      ><UiButton :disabled="saving" @click="emit('update:modelValue', false)"
        >关闭</UiButton
      ><UiButton
        v-if="detail && !loading && !error"
        variant="primary"
        :loading="saving"
        @click="emit('save')"
        >保存选择</UiButton
      ></template
    >
  </UiDrawer>
</template>
<style scoped>
.airport-detail {
  min-width: 0;
}
.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
  margin: 0 0 24px;
}
.detail-grid > div {
  min-width: 0;
}
.source-fact {
  grid-column: 1 / -1;
}
dt {
  color: var(--c-muted);
  font-size: 14px;
  margin-bottom: 8px;
}
dd {
  margin: 0;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.mono {
  font: 14px/1.6 var(--font-mono);
}
.node-section {
  border-top: 1px solid var(--c-line);
  padding-top: 16px;
}
.section-heading,
.latency-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
h3 {
  margin: 0;
  font-size: 17px;
}
.section-heading p,
.latency-heading p,
.selection-hint {
  color: var(--c-muted);
  font-size: 14px;
  margin: 8px 0;
}
.latency-heading {
  margin: 12px 0;
}
.node-list {
  display: grid;
}
.node-row {
  min-width: 0;
  position: relative;
  padding: 12px 0;
  border-bottom: 1px solid var(--c-line);
  overflow: hidden;
}
.node-link {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--c-muted);
  margin: 4px 0 8px 26px;
  font-size: 14px;
}
.node-row :deep(.el-checkbox) {
  display: flex;
  width: 100%;
  min-height: 36px;
  height: auto;
  margin: 0;
}
.node-row :deep(.el-checkbox__label) {
  min-width: 0;
  width: 100%;
}
.node-name {
  display: block;
  font-size: 14px;
  white-space: normal;
  overflow-wrap: anywhere;
}
.node-metrics {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding-left: 16px;
}
.node-metrics :deep(.ui-tag) {
  font-family: var(--font-mono);
  transition:
    color var(--duration-medium),
    background var(--duration-medium);
}
.node-metrics > .ui-button {
  margin-left: auto;
}
.gemini-result {
  font-size: 14px;
  color: var(--c-muted);
}
.gemini-ok {
  color: var(--c-ok);
}
.gemini-fail,
.error-text {
  color: var(--c-bad);
}
.error-text {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.selection-hint {
  margin-top: 16px;
}
.is-testing::after {
  content: "";
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    transparent,
    var(--c-selected),
    transparent
  );
  animation: node-scan 1.6s var(--ease-out) infinite;
}
@keyframes node-scan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(100%);
  }
}
@media (max-width: 480px) {
  .detail-grid {
    gap: 16px 12px;
  }
  .latency-heading {
    align-items: flex-start;
  }
}
@media (prefers-reduced-motion: reduce) {
  .is-testing::after {
    animation: none;
  }
}
</style>
