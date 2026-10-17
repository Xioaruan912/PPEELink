<template>
  <span :class="['ui-tag', `ui-tag--${tone}`]"><slot></slot></span>
</template>
<script setup lang="ts">
withDefaults(
  defineProps<{ tone?: "neutral" | "signal" | "ok" | "warn" | "bad" }>(),
  { tone: "neutral" }
);
</script>
<style scoped>
.ui-tag {
  --tag-color: var(--c-muted);
  display: inline-flex;
  align-items: center;
  gap: 4px;
  border: 1px solid color-mix(in srgb, var(--tag-color) 35%, var(--c-line));
  padding: 4px 12px;
  border-radius: 999px;
  font-size: 13px;
  line-height: 1.4;
  color: var(--c-ink);
  background: color-mix(in srgb, var(--tag-color) 10%, var(--c-surface));
}
.ui-tag--signal {
  --tag-color: var(--c-signal);
}
.ui-tag--ok {
  --tag-color: var(--c-ok);
}
.ui-tag--warn {
  --tag-color: var(--c-warn);
}
.ui-tag--bad {
  --tag-color: var(--c-bad);
}
</style>
