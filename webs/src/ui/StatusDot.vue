<template>
  <span
    :class="['status-dot', `status-dot--${state}`, { 'is-running': running }]"
    role="status"
    ><i aria-hidden="true"></i
    ><span
      ><slot>{{ label || labels[state] }}</slot></span
    ></span
  >
</template>
<script setup lang="ts">
withDefaults(
  defineProps<{
    state?: "ok" | "warn" | "bad" | "idle";
    label?: string;
    running?: boolean;
  }>(),
  { state: "idle" }
);
const labels = { ok: "正常", warn: "注意", bad: "失败", idle: "空闲" };
</script>
<style scoped>
.status-dot {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  font-variant-numeric: tabular-nums;
  color: var(--c-muted);
}
.status-dot i {
  width: 7px;
  height: 7px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--c-muted);
  transition: background-color var(--duration-fast) ease;
}
.status-dot--ok {
  color: var(--c-ok);
}
.status-dot--ok i {
  background: var(--c-ok);
}
.status-dot--warn {
  color: var(--c-warn-ink);
}
.status-dot--warn i {
  background: var(--c-warn);
  border-radius: 2px;
}
.status-dot--bad {
  color: var(--c-bad);
}
.status-dot--bad i {
  background: var(--c-bad);
  border-radius: 1px;
  transform: rotate(45deg);
}
.is-running i {
  opacity: 0.45;
  animation: status-pulse 1.2s var(--ease-in-out) infinite alternate;
}
@keyframes status-pulse {
  from {
    opacity: 0.35;
  }
  to {
    opacity: 1;
  }
}
</style>
