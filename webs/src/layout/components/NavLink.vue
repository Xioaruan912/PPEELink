<template>
  <el-tooltip
    :content="item.label"
    placement="right"
    :disabled="!collapsed"
    :trigger-keys="[]"
  >
    <a
      v-if="external"
      class="nav-link"
      :href="item.path"
      target="_blank"
      rel="noopener"
      :aria-label="collapsed ? item.label : undefined"
      @click="emit('navigate')"
      ><SvgIcon :icon-class="item.icon" size="18px" /><span
        class="nav-link__label"
        >{{ item.label }}</span
      ></a
    >
    <router-link
      v-else
      class="nav-link"
      :class="{ active }"
      :to="item.path"
      :aria-current="active ? 'page' : undefined"
      :aria-label="collapsed ? item.label : undefined"
      @click="emit('navigate')"
      ><SvgIcon :icon-class="item.icon" size="18px" /><span
        class="nav-link__label"
        >{{ item.label }}</span
      ></router-link
    >
  </el-tooltip>
</template>
<script setup lang="ts">
import { computed } from "vue";
import SvgIcon from "@/components/SvgIcon/index.vue";
import type { NavigationItem } from "../navigation";
const props = defineProps<{
  item: NavigationItem;
  active?: boolean;
  collapsed?: boolean;
}>();
const emit = defineEmits<{ navigate: [] }>();
const external = computed(() => /^(https?:)?\/\//.test(props.item.path));
</script>
<style scoped>
.nav-link {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 36px;
  padding: 0 10px;
  border-radius: var(--radius-control);
  color: var(--c-muted);
  font-size: 14px;
  white-space: nowrap;
  transition:
    color var(--duration-fast) var(--ease-out),
    background-color var(--duration-fast) var(--ease-out);
}
.nav-link :deep(svg) {
  flex: 0 0 auto;
  transition: transform var(--duration-medium) var(--ease-spring);
}
.nav-link:hover {
  color: var(--c-ink);
  background: var(--c-surface-2);
}
.nav-link:hover :deep(svg) {
  transform: translateX(2px);
}
.nav-link.active {
  color: var(--c-ink);
  font-weight: 500;
  background: transparent;
}
.nav-link.active :deep(svg) {
  color: var(--c-signal-text);
}
.nav-link__label {
  overflow: hidden;
  text-overflow: ellipsis;
}
@media (prefers-reduced-motion: reduce) {
  .nav-link,
  .nav-link :deep(svg) {
    transition: none;
  }
}
</style>
