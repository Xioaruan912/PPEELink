<script setup lang="ts">
import { computed } from "vue";
import { translateFriendlyError } from "@/utils/friendlyError";

const props = defineProps<{
  raw?: string | null;
}>();

const parsed = computed(() =>
  props.raw ? translateFriendlyError(props.raw) : null
);
</script>

<template>
  <Transition name="kinetic-pop">
    <div
      v-if="parsed"
      class="kinetic-notice"
      :class="`is-${parsed.tone}`"
      role="alert"
    >
      <span class="notice-orb" aria-hidden="true">
        <i></i>
      </span>
      <div class="notice-copy">
        <strong>{{ parsed.title }}</strong>
        <span>{{ parsed.hint }}</span>
      </div>
      <div v-if="$slots.action" class="notice-action">
        <slot name="action"></slot>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.kinetic-notice {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 14px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  box-shadow: 0 12px 32px -18px rgb(0 0 0 / 0.14);
}

.notice-orb {
  position: relative;
  flex: none;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 50%;
  background: rgb(180 83 9 / 0.12);
}

.notice-orb i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-warn);
  animation: orbPulse 1.6s infinite ease-in-out;
}

.kinetic-notice.is-bad .notice-orb {
  background: rgb(220 38 38 / 0.12);
}

.kinetic-notice.is-bad .notice-orb i {
  background: var(--c-bad);
}

.notice-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.notice-copy strong {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.notice-copy span {
  font-size: 12px;
  color: var(--c-muted);
}

.notice-action {
  margin-left: auto;
}

.kinetic-pop-enter-active,
.kinetic-pop-leave-active {
  transition:
    opacity 240ms var(--ease-out),
    transform 280ms cubic-bezier(0.16, 1, 0.3, 1);
}

.kinetic-pop-enter-from,
.kinetic-pop-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.98);
}

@keyframes orbPulse {
  0%,
  100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.35);
    opacity: 0.55;
  }
}

@media (prefers-reduced-motion: reduce) {
  .notice-orb i {
    animation: none;
  }
}
</style>
