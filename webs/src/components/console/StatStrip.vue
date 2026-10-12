<template>
  <div class="stat-strip">
    <article
      v-for="(item, index) in items"
      :key="index"
      class="stat"
      :class="`tone-${item.tone || 'default'}`"
    >
      <span class="stat-value">
        {{ item.value }}<small v-if="item.unit">{{ item.unit }}</small>
      </span>
      <span class="stat-label">{{ item.label }}</span>
      <span v-if="item.hint" class="stat-hint">{{ item.hint }}</span>
    </article>
  </div>
</template>

<script setup lang="ts">
export type StatTone = "default" | "ok" | "warn" | "bad" | "info";

export interface StatItem {
  label: string;
  value: string | number;
  unit?: string;
  hint?: string;
  tone?: StatTone;
}

defineProps<{ items: StatItem[] }>();
</script>

<style lang="scss" scoped>
.stat-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.stat {
  position: relative;
  display: flex;
  flex: 1 1 130px;
  flex-direction: column;
  gap: 2px;
  min-width: 130px;
  padding: 12px 16px 11px 18px;
  overflow: hidden;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-md);
  background: var(--ui-surface-strong);
  transition:
    transform var(--dur-mid) var(--ease-out),
    box-shadow var(--dur-mid) ease,
    border-color var(--dur-mid) ease;

  &::before {
    position: absolute;
    top: 12px;
    bottom: 12px;
    left: 0;
    width: 3px;
    border-radius: var(--r-pill);
    background: var(--ui-text-muted);
    opacity: 0.45;
    content: "";
  }

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 14px 30px color-mix(in srgb, var(--ui-accent-shadow) 55%, transparent);
  }
}

.tone-ok::before {
  background: var(--el-color-success);
  opacity: 1;
}

.tone-warn::before {
  background: var(--el-color-warning);
  opacity: 1;
}

.tone-bad::before {
  background: var(--el-color-danger);
  opacity: 1;
}

.tone-info::before {
  background: var(--el-color-info);
  opacity: 0.9;
}

.tone-default::before {
  background: var(--ui-accent);
  opacity: 1;
}

.stat-value {
  color: var(--ui-text);
  font-family: var(--font-mono);
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.02em;
  line-height: 1.15;

  small {
    margin-left: 3px;
    color: var(--ui-text-muted);
    font-size: 11px;
  }
}

.stat-label {
  color: var(--ui-text-secondary);
  font-size: 11.5px;
  font-weight: 600;
}

.stat-hint {
  color: var(--ui-text-muted);
  font-size: 10.5px;
}
</style>
