<template>
  <span class="status-pill" :class="`tone-${tone}`">
    <i aria-hidden="true"></i>{{ text }}
  </span>
</template>

<script setup lang="ts">
export type PillTone = "ok" | "warn" | "bad" | "info" | "live";

withDefaults(defineProps<{ text: string; tone?: PillTone }>(), {
  tone: "info",
});
</script>

<style lang="scss" scoped>
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-pill);
  background: var(--ui-surface);
  color: var(--ui-text-secondary);
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;

  i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--ui-text-muted);
  }
}

.tone-ok {
  border-color: color-mix(in srgb, var(--el-color-success) 32%, transparent);
  background: color-mix(in srgb, var(--el-color-success) 10%, transparent);
  color: var(--el-color-success);

  i {
    background: var(--el-color-success);
  }
}

.tone-warn {
  border-color: color-mix(in srgb, var(--el-color-warning) 36%, transparent);
  background: color-mix(in srgb, var(--el-color-warning) 12%, transparent);
  color: var(--el-color-warning);

  i {
    background: var(--el-color-warning);
  }
}

.tone-bad {
  border-color: color-mix(in srgb, var(--el-color-danger) 34%, transparent);
  background: color-mix(in srgb, var(--el-color-danger) 10%, transparent);
  color: var(--el-color-danger);

  i {
    background: var(--el-color-danger);
  }
}

.tone-live {
  border-color: color-mix(in srgb, var(--ui-accent) 36%, transparent);
  background: var(--ui-accent-soft);
  color: var(--ui-accent-strong);

  i {
    background: var(--ui-accent);
    animation: pill-ping 2.4s ease-out infinite;
  }
}

@keyframes pill-ping {
  70%,
  100% {
    box-shadow: 0 0 0 6px transparent;
  }
}

@media (prefers-reduced-motion: reduce) {
  .tone-live i {
    animation: none;
  }
}
</style>
