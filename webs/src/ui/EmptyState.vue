<template>
  <section :class="['empty-state', `empty-state--${size}`]" role="status">
    <slot name="icon"></slot>
    <template v-if="size === 'inline'">
      <p>{{ description || title }}</p>
    </template>
    <template v-else>
      <h2>{{ title }}</h2>
      <p v-if="description">{{ description }}</p>
    </template>
    <slot name="action"></slot>
  </section>
</template>
<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string;
    description?: string;
    size?: "page" | "section" | "inline";
  }>(),
  {
    title: "暂无内容",
    size: "page",
  }
);
</script>
<style scoped>
.empty-state {
  padding: 32px 12px;
  text-align: left;
}
.empty-state p {
  color: var(--c-muted);
  margin-bottom: 16px;
}
.empty-state--page h2 {
  font: 400 28px/1.1 var(--font-display);
  margin: 0 0 12px;
}
.empty-state--section {
  padding: 20px 0;
}
.empty-state--section h2 {
  font: 500 16px/1.4 var(--font-ui);
  margin: 0 0 8px;
}
.empty-state--section p {
  font-size: 14px;
  margin-bottom: 12px;
}
.empty-state--inline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 12px 0;
}
.empty-state--inline p {
  font: 400 14px/1.6 var(--font-ui);
  margin: 0;
  color: var(--c-muted);
}
</style>
