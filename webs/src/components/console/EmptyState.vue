<template>
  <div class="empty-state">
    <span class="empty-mark" aria-hidden="true"><i></i></span>
    <b>{{ title }}</b>
    <p v-if="desc">{{ desc }}</p>
    <div v-if="$slots.default" class="empty-extra">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ title: string; desc?: string }>(), { desc: "" });
</script>

<style lang="scss" scoped>
.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding: 40px 20px;
  text-align: center;

  b {
    margin-top: 12px;
    color: var(--ui-text);
    font-size: 13.5px;
    font-weight: 700;
  }

  p {
    max-width: 46ch;
    margin: 0;
    color: var(--ui-text-muted);
    font-size: 12px;
    line-height: 1.6;
  }
}

.empty-mark {
  position: relative;
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  border: 1px dashed color-mix(in srgb, var(--ui-accent) 45%, var(--ui-border));
  border-radius: 50%;
  animation: empty-breathe 3.4s ease-in-out infinite;

  i {
    width: 9px;
    height: 9px;
    border-radius: 50%;
    background: var(--ui-accent);
    box-shadow: 0 0 0 6px color-mix(in srgb, var(--ui-accent) 14%, transparent);
  }
}

@keyframes empty-breathe {
  50% {
    transform: scale(1.06);
    border-color: color-mix(in srgb, var(--ui-accent) 70%, var(--ui-border));
  }
}

.empty-extra {
  margin-top: 12px;
}

@media (prefers-reduced-motion: reduce) {
  .empty-mark {
    animation: none;
  }
}
</style>
