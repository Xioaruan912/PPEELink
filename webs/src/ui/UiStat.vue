<template>
  <div class="ui-stat" data-enter>
    <p class="ui-stat__label">
      <slot name="label">{{ label }}</slot>
    </p>
    <div :class="['ui-stat__number', `is-${state}`]">
      <span aria-hidden="true">{{ display }}</span
      ><span class="sr-only">{{ value }}</span
      ><span v-if="unit" class="ui-stat__unit">{{ unit }}</span>
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
    value: number;
    decimals?: number;
    unit?: string;
    trend?: string;
    state?: "default" | "ok" | "warn" | "bad";
    thresholds?: { warn: number; bad: number };
  }>(),
  { decimals: 0, state: "default" }
);
const display = useCountUp(() => props.value, props.decimals);
const state = computed(() =>
  props.thresholds
    ? props.value >= props.thresholds.bad
      ? "bad"
      : props.value >= props.thresholds.warn
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
  font-size: 14px;
  color: var(--c-muted);
  margin-bottom: 16px;
}
.ui-stat__number {
  font: 400 clamp(32px, 4vw, 56px)/1.1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  transition: color var(--duration-medium);
}
.ui-stat__unit {
  margin-left: 8px;
  font-size: 15px;
  color: var(--c-muted);
}
.ui-stat__trend {
  margin: 16px 0 0;
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
</style>
