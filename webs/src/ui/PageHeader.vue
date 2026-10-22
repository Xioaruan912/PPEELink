<template>
  <header class="page-header" data-enter>
    <div class="page-header__copy">
      <h1 :aria-label="title || undefined">
        <slot
          ><span
            v-for="(char, i) in Array.from(title || '')"
            :key="`${title}-${i}`"
            class="title-char"
            :style="{ '--i': i }"
            aria-hidden="true"
            >{{ char === " " ? "\u00a0" : char }}</span
          ></slot
        >
      </h1>
      <p v-if="description" class="page-header__description">
        {{ description }}
      </p>
    </div>
    <div v-if="$slots.actions" class="page-header__actions">
      <slot name="actions"></slot>
    </div>
  </header>
</template>
<script setup lang="ts">
defineProps<{
  title?: string;
  /** Kept for call-site compatibility; the eyebrow line is no longer rendered. */
  eyebrow?: string;
  description?: string;
}>();
</script>
<style scoped>
.page-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
}
h1 {
  margin: 0;
}
/* 标题逐字上浮：每个字错开 28ms，进入时从模糊到清晰 */
.title-char {
  display: inline-block;
  animation: title-rise 0.7s var(--ease-out) both;
  animation-delay: calc(var(--i) * 28ms);
}
@keyframes title-rise {
  from {
    opacity: 0;
    transform: translateY(0.45em) rotate(4deg);
    filter: blur(6px);
  }
}
@media (prefers-reduced-motion: reduce) {
  .title-char {
    animation: none;
  }
}
.page-header__copy {
  min-width: 0;
}
.page-header__description {
  color: var(--c-muted);
  font-size: 14px;
  margin: 6px 0 0;
}
.page-header__actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}
@media (max-width: 767px) {
  .page-header {
    align-items: start;
    flex-direction: column;
    gap: 12px;
  }
}
</style>
