<template>
  <section class="ui-panel" :aria-label="title">
    <header v-if="title || $slots.header" class="ui-panel__header">
      <slot name="header"
        ><h2>{{ title }}</h2></slot
      ><slot name="actions"></slot>
    </header>
    <slot></slot>
    <footer v-if="$slots.footer" class="ui-panel__footer">
      <slot name="footer"></slot>
    </footer>
  </section>
</template>
<script setup lang="ts">
defineProps<{ title?: string }>();
</script>
<style scoped>
.ui-panel {
  min-width: 0;
  padding: 16px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  box-shadow: var(--shadow-panel);
}
.ui-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
h2 {
  margin: 0;
}
.ui-panel__footer {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--c-line);
}
@media (max-width: 767px) {
  .ui-panel {
    padding: 12px;
  }
}
</style>
