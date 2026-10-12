<template>
  <span
    class="quality-ring"
    :style="{ width: `${size}px`, height: `${size}px` }"
  >
    <svg :viewBox="`0 0 ${size} ${size}`" aria-hidden="true">
      <circle
        class="track"
        :cx="center"
        :cy="center"
        :r="radius"
        :stroke-width="stroke"
        fill="none"
      />
      <circle
        class="bar"
        :cx="center"
        :cy="center"
        :r="radius"
        :stroke-width="stroke"
        fill="none"
        :stroke="tone"
        :stroke-dasharray="circumference"
        :stroke-dashoffset="dashOffset"
        stroke-linecap="round"
      />
    </svg>
    <b :style="{ color: tone }">{{ display }}</b>
  </span>
</template>

<script setup lang="ts">
const props = withDefaults(
  defineProps<{ value: number; size?: number; stroke?: number }>(),
  { size: 46, stroke: 3.5 }
);

const safeValue = computed(() =>
  Math.max(0, Math.min(100, Number(props.value) || 0))
);
const center = computed(() => props.size / 2);
const radius = computed(() => props.size / 2 - props.stroke - 1);
const circumference = computed(() => 2 * Math.PI * radius.value);
const ready = ref(false);
const dashOffset = computed(() => {
  if (!ready.value) return circumference.value;
  return circumference.value * (1 - safeValue.value / 100);
});
const tone = computed(() => {
  if (safeValue.value >= 80) return "var(--el-color-success)";
  if (safeValue.value >= 60) return "var(--ui-accent)";
  return "var(--el-color-danger)";
});
const display = computed(() => Math.round(safeValue.value));

onMounted(() => {
  requestAnimationFrame(() => requestAnimationFrame(() => (ready.value = true)));
});
</script>

<style lang="scss" scoped>
.quality-ring {
  position: relative;
  display: inline-grid;
  flex-shrink: 0;
  place-items: center;

  svg {
    position: absolute;
    inset: 0;
    transform: rotate(-90deg);
  }

  .track {
    stroke: color-mix(in srgb, var(--ui-border) 85%, transparent);
  }

  .bar {
    transition: stroke-dashoffset 900ms var(--ease-out);
  }

  b {
    font-family: var(--font-mono);
    font-size: 12px;
    font-weight: 700;
  }
}

@media (prefers-reduced-motion: reduce) {
  .bar {
    transition: none;
  }
}
</style>
