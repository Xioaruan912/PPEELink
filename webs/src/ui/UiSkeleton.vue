<template>
  <div class="ui-skeleton" role="status" aria-label="正在加载" aria-busy="true">
    <slot
      ><span
        v-for="line in lines"
        :key="line"
        :style="{ height, width: line === lines ? '65%' : '100%' }"
      ></span
    ></slot>
  </div>
</template>
<script setup lang="ts">
withDefaults(defineProps<{ lines?: number; height?: string }>(), {
  lines: 3,
  height: "24px",
});
</script>
<style scoped>
.ui-skeleton {
  display: grid;
  gap: 12px;
}
.ui-skeleton span {
  display: block;
  position: relative;
  overflow: hidden;
  border-radius: var(--radius-control);
  background: var(--c-surface-2);
}
.ui-skeleton span::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg,
    transparent,
    color-mix(in srgb, var(--c-surface) 80%, transparent),
    transparent
  );
  animation: skeleton-sweep 1.6s var(--ease-out) infinite;
}
@keyframes skeleton-sweep {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(100%);
  }
}
</style>
