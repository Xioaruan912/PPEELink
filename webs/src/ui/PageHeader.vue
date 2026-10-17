<template>
  <header class="page-header" data-enter>
    <div class="page-header__copy">
      <p v-if="eyebrow" class="page-header__eyebrow">{{ eyebrow }}</p>
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
defineProps<{ title?: string; eyebrow?: string; description?: string }>();
</script>
<style scoped>
.page-header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  padding: 16px 0 40px;
  margin-bottom: 48px;
  border-bottom: 1px solid var(--c-line);
}
h1 {
  margin: 0;
}
.page-header__copy {
  min-width: 0;
}
.page-header__eyebrow {
  margin-bottom: 24px;
  font: 400 13px/1.4 var(--font-mono);
  letter-spacing: 0.08em;
  color: var(--c-muted);
}
.page-header__description {
  color: var(--c-muted);
  margin: 24px 0 0;
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
    margin-bottom: 32px;
    padding-bottom: 32px;
  }
}
</style>
