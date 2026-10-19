<template>
  <div class="health-strip">
    <p class="health-label">有效率</p>
    <UiSkeleton v-if="loading" :lines="2" height="24px" />
    <template v-else-if="summary.total">
      <UiStat
        size="medium"
        label="有效率"
        :value="rate"
        :decimals="1"
        unit="%"
      />
      <div
        class="health-ratio"
        role="img"
        :aria-label="`有效 ${summary.valid}，无效 ${summary.invalid}`"
      >
        <span
          v-for="item in states"
          :key="item.key"
          :class="item.key"
          :style="{ flex: item.value }"
        ></span>
      </div>
      <dl class="health-legend">
        <div v-for="item in states" :key="item.key">
          <dt><StatusDot :state="item.state" :label="item.label" /></dt>
          <dd>
            <UiStat size="small" :label="item.label" :value="item.value" />
          </dd>
        </div>
      </dl>
    </template>
    <EmptyState
      size="inline"
      v-else
      class="health-empty"
      title="等待连接"
      description="添加节点后查看节点是否有效。"
    />
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { EmptyState, StatusDot, UiSkeleton, UiStat } from "@/ui";
const props = defineProps<{
  summary: { total: number; valid: number; invalid: number };
  rate: number;
  loading: boolean;
}>();
const states = computed(() => [
  {
    key: "healthy",
    state: "ok" as const,
    label: "有效",
    value: props.summary.valid,
  },
  {
    key: "offline",
    state: "bad" as const,
    label: "无效",
    value: props.summary.invalid,
  },
]);
</script>
<style scoped>
.health-label {
  margin-bottom: 4px;
  font-size: 13px;
  color: var(--c-muted);
}
.health-strip > :deep(.ui-stat) {
  margin-bottom: 12px;
}
.health-strip > :deep(.ui-stat .ui-stat__label),
.health-legend :deep(.ui-stat__label) {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}
.health-strip > :deep(.ui-stat .ui-stat__number) {
  font-size: 24px;
}
.health-ratio {
  height: 8px;
  display: flex;
  overflow: hidden;
  border-radius: 999px;
  background: var(--c-line);
}
.healthy {
  background: var(--c-ok);
}
.warning {
  background: var(--c-warn);
}
.offline {
  background: var(--c-bad);
}
.health-legend {
  display: flex;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
  margin: 12px 0 0;
}
.health-legend > div {
  display: flex;
  align-items: center;
  gap: 8px;
}
dd {
  margin: 0;
}
.health-legend :deep(.ui-stat__number) {
  font-size: 14px;
}
.health-empty {
  padding: 12px 0 0;
}
.health-empty :deep(p) {
  margin-bottom: 0;
  font-size: 14px;
}
</style>
