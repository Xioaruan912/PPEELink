<template>
  <button
    :type="type"
    :class="['ui-button', `ui-button--${variant}`, `ui-button--${size}`]"
    :disabled="disabled || loading"
    :aria-busy="loading || undefined"
  >
    <span class="ui-button__curtain" aria-hidden="true"></span>
    <span v-if="loading" class="ui-button__loading" aria-hidden="true"></span>
    <slot v-else name="icon"
      ><component
        :is="icon"
        v-if="icon"
        class="ui-button__icon"
        aria-hidden="true"
    /></slot>
    <span class="ui-button__label"><slot></slot></span>
  </button>
</template>
<script setup lang="ts">
import type { Component } from "vue";
withDefaults(
  defineProps<{
    variant?: "primary" | "secondary" | "ghost" | "danger";
    type?: "button" | "submit" | "reset";
    size?: "sm" | "md";
    disabled?: boolean;
    loading?: boolean;
    icon?: Component;
  }>(),
  { variant: "secondary", type: "button", size: "md" }
);
</script>
<style scoped>
.ui-button {
  position: relative;
  overflow: hidden;
  min-height: 38px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 8px 18px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  color: var(--c-ink);
  background: var(--c-surface);
  font: 540 13.5px/1.3 var(--font-ui);
  white-space: nowrap;
  transition:
    color var(--duration-medium) var(--ease-out),
    border-color var(--duration-medium) var(--ease-out),
    background-color var(--duration-fast) ease,
    transform var(--duration-press) var(--ease-out);
}
.ui-button__curtain {
  position: absolute;
  inset: 0;
  background: var(--c-ink);
  transform: scaleX(0);
  transform-origin: right center;
  transition: transform var(--duration-medium) var(--ease-out);
  pointer-events: none;
  z-index: 0;
}
.ui-button__icon,
.ui-button__label,
.ui-button__loading {
  position: relative;
  z-index: 1;
}
.ui-button__label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
@media (hover: hover) and (pointer: fine) {
  .ui-button:hover:not(:disabled) {
    color: var(--c-surface);
    border-color: var(--c-ink);
  }
  .ui-button:hover:not(:disabled) .ui-button__curtain {
    transform: scaleX(1);
    transform-origin: left center;
  }
}
.ui-button--sm {
  min-height: 30px;
  padding: 4px 11px;
  gap: 5px;
  font-size: 12.5px;
}
.ui-button:active:not(:disabled) {
  transform: scale(0.98);
}
.ui-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.ui-button--primary {
  color: var(--c-on-signal);
  background: var(--c-signal);
  border-color: var(--c-signal);
}
.ui-button--primary .ui-button__curtain {
  background: var(--c-data);
}
@media (hover: hover) and (pointer: fine) {
  .ui-button--primary:hover:not(:disabled) {
    color: #ffffff;
    border-color: var(--c-data);
  }
}
.ui-button--primary:disabled {
  color: var(--c-muted);
  background: var(--c-surface-2);
  border-color: var(--c-line);
  opacity: 1;
}
.ui-button--ghost {
  border-color: transparent;
  background: transparent;
  color: var(--c-muted);
}
.ui-button--ghost .ui-button__curtain {
  background: var(--c-surface-2);
}
@media (hover: hover) and (pointer: fine) {
  .ui-button--ghost:hover:not(:disabled) {
    color: var(--c-ink);
    border-color: transparent;
  }
}
.ui-button--danger {
  color: var(--c-bad);
  border-color: color-mix(in srgb, var(--c-bad) 35%, var(--c-line));
  background: var(--c-surface);
}
.ui-button--danger .ui-button__curtain {
  background: var(--c-bad);
}
@media (hover: hover) and (pointer: fine) {
  .ui-button--danger:hover:not(:disabled) {
    color: #ffffff;
    border-color: var(--c-bad);
  }
}
.ui-button__icon {
  width: 16px;
  height: 16px;
}
.ui-button__loading {
  width: 15px;
  height: 15px;
  border: 1.5px solid currentColor;
  border-top-color: transparent;
  border-radius: 50%;
  animation: button-spin 0.6s linear infinite;
}
@keyframes button-spin {
  to {
    transform: rotate(360deg);
  }
}
@media (max-width: 767px) {
  .ui-button,
  .ui-button--sm {
    min-height: 44px;
  }
  .ui-button--sm {
    padding: 8px 12px;
  }
}
</style>
