<template>
  <header class="page-hero">
    <div class="hero-main">
      <h1>{{ title }}</h1>
      <p v-if="desc">{{ desc }}</p>
    </div>
    <div v-if="$slots.actions" class="hero-actions">
      <slot name="actions" />
    </div>
    <div v-if="$slots.stats" class="hero-stats">
      <slot name="stats" />
    </div>
  </header>
</template>

<script setup lang="ts">
defineProps<{ title: string; desc?: string }>();
</script>

<style lang="scss" scoped>
.page-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px 16px;
  align-items: end;
  margin-bottom: 18px;
  padding: 8px 2px 18px;
  border-bottom: 1px solid var(--ui-border);
}

.hero-main {
  min-width: 0;

  h1 {
    margin: 0;
    color: var(--ui-text);
    font-size: 22px;
    font-weight: 700;
    letter-spacing: -0.01em;
    line-height: 1.2;
  }

  p {
    max-width: 72ch;
    margin: 7px 0 0;
    color: var(--ui-text-muted);
    font-size: 12.5px;
    line-height: 1.6;
  }
}

.hero-actions {
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

.hero-stats {
  display: flex;
  grid-column: 1 / -1;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 4px;
}

@media (max-width: 720px) {
  .page-hero {
    grid-template-columns: 1fr;
    align-items: stretch;
  }

  .hero-actions {
    flex-wrap: wrap;
  }
}
</style>
