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
  padding: 40px 28px;
  text-align: left;
}
.empty-state p {
  color: var(--c-muted);
  margin-bottom: 18px;
  max-width: 48ch;
  line-height: 1.6;
}
.empty-state--page h2 {
  font: 640 24px/1.15 var(--font-display);
  letter-spacing: -0.03em;
  margin: 0 0 10px;
}
.empty-state--section {
  padding: 36px 28px;
}
.empty-state--section h2 {
  font: 620 17px/1.4 var(--font-ui);
  letter-spacing: -0.015em;
  margin: 0 0 8px;
}
.empty-state--section p {
  font-size: 13.5px;
  margin-bottom: 16px;
}
.empty-state--inline {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 14px 0;
}
.empty-state--inline p {
  font: 400 13.5px/1.6 var(--font-ui);
  margin: 0;
  color: var(--c-muted);
}
</style>
