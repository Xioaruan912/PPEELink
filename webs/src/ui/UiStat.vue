<template>
  <div :class="['ui-stat', `ui-stat--${size}`]" data-enter>
    <p class="ui-stat__label">
      <slot name="label">{{ label }}</slot>
    </p>
    <div :class="['ui-stat__number', `is-${state}`]">
      <span aria-hidden="true">{{ empty ? "—" : display }}</span
      ><span class="sr-only">{{ empty ? "暂无数据" : value }}</span
      ><span v-if="unit && !empty" class="ui-stat__unit">{{ unit }}</span>
    </div>
    <p v-if="trend || $slots.trend" class="ui-stat__trend">
      <slot name="trend">{{ trend }}</slot>
    </p>
  </div>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { useCountUp } from "@/motion";
const props = withDefaults(
  defineProps<{
    label: string;
    value: number | null;
    size?: "hero" | "large" | "medium" | "small";
    decimals?: number;
    unit?: string;
    trend?: string;
    state?: "default" | "ok" | "warn" | "bad";
    thresholds?: { warn: number; bad: number };
  }>(),
  { decimals: 0, state: "default", size: "large" }
);
const empty = computed(
  () => props.value === null || !Number.isFinite(props.value)
);
const display = useCountUp(() => props.value ?? 0, props.decimals);
const state = computed(() =>
  empty.value
    ? "empty"
    : props.thresholds
      ? props.value! >= props.thresholds.bad
        ? "bad"
        : props.value! >= props.thresholds.warn
          ? "warn"
          : "ok"
      : props.state
);
</script>
<style scoped>
.ui-stat {
  min-width: 0;
}
.ui-stat__label {
  font-size: 13px;
  color: var(--c-muted);
  margin-bottom: 6px;
}
.ui-stat__number {
  display: flex;
  align-items: baseline;
  font: 600 26px/1.15 var(--font-ui);
  letter-spacing: -0.02em;
  white-space: nowrap;
  overflow-wrap: normal;
  font-variant-numeric: lining-nums tabular-nums;
  transition: color var(--duration-medium);
}
.ui-stat--hero .ui-stat__number {
  font: 400 clamp(44px, 4.4vw, 60px)/1 var(--font-display);
  letter-spacing: -0.03em;
}
.ui-stat--medium .ui-stat__number {
  font: 400 20px/1.1 var(--font-mono);
  font-variant-numeric: lining-nums tabular-nums;
}
.ui-stat--small .ui-stat__number {
  font: 400 16px/1.1 var(--font-mono);
  font-variant-numeric: lining-nums tabular-nums;
}
.ui-stat__unit {
  flex-shrink: 0;
  margin-left: 8px;
  font: 400 14px/1.4 var(--font-mono);
  letter-spacing: normal;
  color: var(--c-muted);
}
.ui-stat__trend {
  margin: 12px 0 0;
  font-size: 14px;
  color: var(--c-muted);
}
.is-ok {
  color: var(--c-ok);
}
.is-warn {
  color: var(--c-warn-ink);
}
.is-bad {
  color: var(--c-bad);
}
.is-empty {
  color: var(--c-muted);
}
</style>
