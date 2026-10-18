<template>
  <button
    v-magnetic="variant === 'primary'"
    :type="type"
    :class="['ui-button', `ui-button--${variant}`]"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
  >
    <span v-if="loading" class="ui-button__loading" aria-hidden="true"></span>
    <slot v-else name="icon"
      ><component
        :is="icon"
        v-if="icon"
        class="ui-button__icon"
        aria-hidden="true"
    /></slot>
    <span><slot></slot></span>
  </button>
</template>
<script setup lang="ts">
import type { Component } from "vue";
import { vMagnetic } from "@/motion";
withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "ghost" | "danger";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    loading?: boolean;
    icon?: Component;
  }>(),
  { variant: "secondary", type: "button" }
);
</script>
<style scoped>
.ui-button {
  min-height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  color: var(--c-ink);
  background: var(--c-surface);
  font: 500 14px/1.4 var(--font-ui);
  white-space: nowrap;
  transition:
    background var(--duration-fast),
    transform var(--duration-fast) var(--ease-out);
}
.ui-button:hover {
  background: var(--c-surface-2);
}
.ui-button:active:not(:disabled) {
  transform: scale(0.98);
}
.ui-button:disabled {
  opacity: 0.65;
}
.ui-button--primary {
  color: var(--c-on-signal);
  background: var(--c-signal);
  border-color: var(--c-signal);
  box-shadow:
    var(--edge-highlight),
    0 6px 18px -8px var(--c-glow);
}
.ui-button--primary:hover {
  background: color-mix(in srgb, var(--c-signal) 90%, var(--c-ink));
  box-shadow:
    var(--edge-highlight),
    0 10px 28px -8px var(--c-glow);
}
.ui-button--primary:disabled {
  color: var(--c-muted);
  background: var(--c-surface-2);
  border-color: var(--c-line);
  box-shadow: none;
  opacity: 1;
  cursor: not-allowed;
}
.ui-button--ghost {
  border-color: transparent;
  background: transparent;
}
.ui-button--danger {
  color: var(--c-bad);
  border-color: var(--c-bad);
}
.ui-button__icon {
  width: 18px;
  height: 18px;
}
.ui-button__loading {
  width: 18px;
  height: 18px;
  border: 1px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: button-spin 0.56s linear infinite;
}
@keyframes button-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
