<template>
  <div class="recommendations" :class="{ 'is-reduced': reduced }">
    <UiSegmented v-model="scene" :options="options" label="推荐用途" />
    <div
      v-if="!loading && activeNodes.length"
      class="recommendation-head"
      aria-hidden="true"
    >
      <span>推荐节点 / 原因</span><span>连接评分</span>
    </div>
    <UiSkeleton v-if="loading" :lines="5" height="64px" />
    <template v-else-if="activeNodes.length">
      <TransitionGroup
        ref="list"
        name="node"
        tag="div"
        class="recommendation-list"
      >
        <RouterLink
          v-for="(node, index) in activeNodes.slice(0, 5)"
          :key="node.nodeId"
          to="/subcription/nodes"
          class="recommendation-row"
          :style="{ '--row': index }"
        >
          <span class="node-copy">
            <strong>{{ node.name }}</strong>
            <span>{{
              simpleReason(node.reasons || []) || "暂无推荐原因"
            }}</span>
          </span>
          <span class="node-score">
            <UiStat
              size="small"
              label="连接评分"
              :value="node.score"
              unit="分"
            />
            <TopRight aria-hidden="true" />
          </span>
        </RouterLink>
      </TransitionGroup>
    </template>
    <EmptyState
      size="section"
      v-else
      title="还没有推荐节点"
      description="添加节点并测速后查看推荐结果。"
    >
      <template #action>
        <UiButton :icon="Plus" @click="router.push('/subcription/nodes')"
          >添加节点</UiButton
        >
      </template>
    </EmptyState>
  </div>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import { computed, nextTick, ref, watch } from "vue";
import { Plus, TopRight } from "@element-plus/icons-vue";
import { EmptyState, UiButton, UiSegmented, UiSkeleton, UiStat } from "@/ui";
import { useStagger, usePrefersReducedMotion } from "@/motion";

const router = useRouter();
export interface Recommendation {
  key: string;
  name: string;
  nodes: { nodeId: number; name: string; reasons: string[]; score: number }[];
}
const props = defineProps<{
  scenes: Recommendation[];
  modelValue: string;
  loading: boolean;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const scene = computed({
  get: () => props.modelValue,
  set: (value: string | number) => emit("update:modelValue", String(value)),
});
const options = computed(() =>
  props.scenes.map((s) => ({
    value: s.key,
    label: s.nodes?.length ? `${s.name} ${s.nodes.length}` : s.name,
  }))
);
const activeNodes = computed(
  () => props.scenes.find((s) => s.key === scene.value)?.nodes || []
);
const list = ref<{ $el: HTMLElement }>();
const stagger = useStagger();
const reduced = usePrefersReducedMotion();
watch(
  () => props.loading,
  async (loading) => {
    if (loading) return;
    await nextTick();
    const rows = list.value?.$el.querySelectorAll<HTMLElement>(
      ".recommendation-row"
    );
    if (rows) stagger(rows);
  },
  { immediate: true }
);
function simpleReason(reasons: string[]) {
  const primary = reasons.find((reason) => !/P95|抖动/.test(reason)) || "";
  return primary
    .replace("24h 可用率", "近一天连接成功率")
    .replace("平均延迟", "平均响应")
    .replace("场景可达率", "访问成功率")
    .replace("场景平均", "平均响应")
    .replace("已解锁", "可访问")
    .replace("尚无解锁样本", "还未检测服务");
}
</script>

<style scoped>
.recommendations {
  min-width: 0;
  --list-duration: 320ms;
}
.is-reduced {
  --list-duration: 0ms;
}
.is-reduced .node-enter-active {
  transition-delay: 0ms;
}
.recommendation-head {
  display: flex;
  justify-content: space-between;
  padding: 32px 0 16px;
  font: 13px/1.4 var(--font-mono);
  color: var(--c-muted);
}
.recommendation-list {
  position: relative;
}
.recommendation-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  min-height: 96px;
  padding: 16px 8px;
  border-top: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  color: var(--c-ink);
}
.recommendation-row:hover {
  background: var(--c-surface-2);
}
.node-copy {
  display: grid;
  gap: 8px;
  min-width: 0;
}
.node-copy strong {
  font-size: 16px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.node-copy > span {
  font-size: 14px;
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.node-score {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}
.node-score svg {
  width: 16px;
  height: 16px;
  color: var(--c-data);
}
.node-score :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.node-score :deep(.ui-stat__number) {
  font-size: 24px;
}
.node-score :deep(.ui-stat__unit) {
  font-size: 13px;
}
.node-move,
.node-enter-active,
.node-leave-active {
  transition:
    transform var(--list-duration) var(--ease-out),
    opacity var(--list-duration) var(--ease-out);
}
.node-enter-active {
  transition-delay: calc(var(--row) * 40ms);
}
.node-enter-from,
.node-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
.node-leave-active {
  position: absolute;
  width: 100%;
}
@media (prefers-reduced-motion: reduce) {
  .recommendations {
    --list-duration: 0ms;
  }
  .node-enter-active {
    transition-delay: 0ms;
  }
}
@media (max-width: 767px) {
  .recommendation-row {
    gap: 16px;
    padding-inline: 0;
  }
  .node-score {
    gap: 8px;
  }
}
</style>
