<template>
  <header class="page-header" data-enter>
    <div class="page-header__copy">
      <p v-if="resolvedEyebrow" class="page-header__eyebrow">
        {{ resolvedEyebrow }}
      </p>
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
import { computed } from "vue";
import { useRoute } from "vue-router";
import { translateRouteTitle } from "@/utils/i18n";
const props = defineProps<{
  title?: string;
  eyebrow?: string;
  description?: string;
}>();
const route = useRoute();
const resolvedEyebrow = computed(() => {
  if (props.eyebrow !== undefined) return props.eyebrow;
  const matched = route?.matched || [];
  const current = matched[matched.length - 1];
  if (!current) return "";
  const parent = matched[matched.length - 2];
  const group = translateRouteTitle(
    parent?.meta.title || current.meta.title || ""
  );
  if (!group) return "";
  const record = current.aliasOf || current;
  const parentRecord = parent?.aliasOf || parent;
  const normalize = (path: string) => path.replace(/\/+$/, "") || "/";
  const index = parentRecord
    ? parentRecord.children.findIndex((child) => {
        if (child.name != null && child.name === record.name) return true;
        const path = child.path.startsWith("/")
          ? child.path
          : `${parentRecord.path.replace(/\/$/, "")}/${child.path}`;
        return normalize(path) === normalize(record.path);
      })
    : 0;
  return index < 0 ? group : `${group} / ${String(index + 1).padStart(2, "0")}`;
});
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
