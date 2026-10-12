<template>
  <section class="panel-section">
    <header v-if="title || $slots.actions" class="section-head">
      <div class="section-titles">
        <h2 v-if="title">{{ title }}</h2>
        <p v-if="desc">{{ desc }}</p>
      </div>
      <div v-if="$slots.actions" class="section-actions">
        <slot name="actions" />
      </div>
    </header>
    <div class="section-body" :class="{ flush }">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{ title?: string; desc?: string; flush?: boolean }>(),
  { flush: false }
);
</script>

<style lang="scss" scoped>
.panel-section {
  margin-bottom: 16px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-lg);
  background: var(--ui-surface-strong);
  box-shadow: 0 10px 30px color-mix(in srgb, var(--ui-accent-shadow) 30%, transparent);
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 15px 20px 13px;
  border-bottom: 1px solid color-mix(in srgb, var(--ui-border) 72%, transparent);
}

.section-titles {
  min-width: 0;

  h2 {
    margin: 0;
    color: var(--ui-text);
    font-size: 15px;
    font-weight: 700;
  }

  p {
    margin: 4px 0 0;
    color: var(--ui-text-muted);
    font-size: 12px;
  }
}

.section-actions {
  display: flex;
  flex-shrink: 0;
  gap: 8px;
}

.section-body {
  padding: 18px 20px;
}

.section-body.flush {
  padding: 0;
}

@media (max-width: 720px) {
  .section-head {
    flex-direction: column;
    align-items: stretch;
  }

  .section-actions {
    flex-wrap: wrap;
  }
}
</style>
