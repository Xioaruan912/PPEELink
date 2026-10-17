<template>
  <section class="empty-state" role="status">
    <slot name="icon"></slot>
    <h2>{{ title }}</h2>
    <p v-if="description">{{ description }}</p>
    <slot name="action"></slot>
  </section>
</template>
<script setup lang="ts">
withDefaults(defineProps<{ title?: string; description?: string }>(), {
  title: "暂无内容",
});
</script>
<style scoped>
.empty-state {
  padding: 64px 16px;
  text-align: left;
}
.empty-state h2 {
  font: 400 40px/1.1 var(--font-display);
  margin: 0 0 16px;
}
.empty-state p {
  color: var(--c-muted);
  margin-bottom: 24px;
}
</style>
