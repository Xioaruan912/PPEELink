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
  gap: 8px;
  font-size: 14px;
  color: var(--c-muted);
}
.status-dot i {
  width: 8px;
  height: 8px;
  flex: 0 0 auto;
  border-radius: 50%;
  background: var(--c-muted);
}
.status-dot--ok i {
  background: var(--c-ok);
  animation: status-breathe 2s var(--ease-out) infinite;
}
.status-dot--warn i {
  background: var(--c-warn);
  border-radius: 2px;
}
.status-dot--bad i {
  background: var(--c-bad);
  border-radius: 0;
  transform: rotate(45deg);
}
.is-running {
  overflow: hidden;
  position: relative;
}
.is-running::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg,
    transparent,
    var(--c-selected),
    transparent
  );
  animation: status-scan 1.6s var(--ease-out) infinite;
}
@keyframes status-breathe {
  50% {
    opacity: 0.4;
    transform: scale(0.75);
  }
}
@keyframes status-scan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(100%);
  }
}
</style>
