<template>
  <header class="page-header" data-enter>
    <div class="page-header__grid">
      <div class="page-header__copy">
        <h1 class="page-header__title">
          <span class="page-header__mask"
            ><span class="page-header__line"
              ><slot>{{ title }}</slot></span
            ></span
          >
        </h1>
        <p v-if="description" class="page-header__description">
          {{ description }}
        </p>
      </div>
      <div v-if="$slots.actions" class="page-header__actions">
        <slot name="actions"></slot>
      </div>
    </div>
  </header>
</template>
<script setup lang="ts">
defineProps<{
  title?: string;
  eyebrow?: string;
  description?: string;
}>();
</script>
<style scoped>
.page-header {
  position: relative;
  margin-bottom: 32px;
}
.page-header__grid {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
}
.page-header__title {
  margin: 0;
  font-size: clamp(32px, 3.6vw, 46px);
  font-weight: 680;
  line-height: 1.06;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}
.page-header__mask {
  display: block;
  overflow: hidden;
  padding-bottom: 0.08em;
  margin-bottom: -0.08em;
}
.page-header__line {
  display: block;
}
@media (prefers-reduced-motion: no-preference) {
  .page-header__line {
    animation: mask-rise var(--duration-reveal) var(--ease-out) both;
  }
}
.page-header__copy {
  min-width: 0;
}
.page-header__description {
  color: var(--c-muted);
  font-size: 14.5px;
  line-height: 1.55;
  margin: 10px 0 0;
  max-width: 54ch;
}
.page-header__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
@media (max-width: 767px) {
  .page-header {
    margin-bottom: 22px;
  }
  .page-header__grid {
    align-items: start;
    flex-direction: column;
    gap: 14px;
  }
  .page-header__actions {
    width: 100%;
  }
  .page-header__actions :deep(button) {
    flex: 1 1 auto;
  }
}
</style>
