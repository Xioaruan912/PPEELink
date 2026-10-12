<template>
  <span ref="host" class="count-up">{{ display }}</span>
</template>

<script setup lang="ts">
import { useCountUp, useCountUpInView } from "./useCountUp";

const props = withDefaults(
  defineProps<{
    value: number;
    duration?: number;
    decimals?: number;
    separator?: boolean;
  }>(),
  { duration: 1.2, decimals: 0, separator: true }
);

const host = ref<HTMLElement>();
const counter = useCountUp(() => props.value, {
  duration: props.duration,
  decimals: props.decimals,
  separator: props.separator,
});
const display = counter.display;
const { register } = useCountUpInView(host);
register(counter.start);
watch(
  () => props.value,
  () => counter.refresh()
);
</script>

<style scoped>
.count-up {
  font-variant-numeric: tabular-nums;
}
</style>
