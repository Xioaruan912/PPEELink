<template>
  <header class="page-header" data-enter>
    <div class="page-header__copy">
      <h1>
        <slot>{{ title }}</slot>
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
  padding: 4px 0 16px;
  margin-bottom: 20px;
  border-bottom: 1px solid var(--c-line);
}
h1 {
  margin: 0;
}
.page-header__copy {
  min-width: 0;
}
.page-header__description {
  color: var(--c-muted);
  font-size: 13px;
  margin: 4px 0 0;
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
    margin-bottom: 20px;
    padding-bottom: 20px;
  }
}
</style>
