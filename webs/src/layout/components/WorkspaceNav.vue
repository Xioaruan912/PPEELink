<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { RouterLink, useRoute } from "vue-router";
import { flattenNavigation, type NavigationItem } from "../navigation";

const props = defineProps<{ items: NavigationItem[] }>();
const emit = defineEmits<{ navigate: [] }>();
const route = useRoute();
const navEl = ref<HTMLElement>();
const pillStyle = ref<{ transform: string; height: string; opacity: number }>({
  transform: "translate3d(0, 0, 0)",
  height: "0px",
  opacity: 0,
});

function syncIndicator() {
  const container = navEl.value;
  if (!container) return;
  const active = container.querySelector<HTMLElement>('a[aria-current="page"]');
  if (!active) {
    pillStyle.value = { ...pillStyle.value, opacity: 0 };
    return;
  }
  const top = active.offsetTop;
  const height = active.offsetHeight;
  pillStyle.value = {
    transform: `translate3d(0, ${top}px, 0)`,
    height: `${height}px`,
    opacity: 1,
  };
}

let observer: ResizeObserver | undefined;
onMounted(() => {
  nextTick(syncIndicator);
  if (typeof ResizeObserver !== "undefined" && navEl.value) {
    observer = new ResizeObserver(syncIndicator);
    observer.observe(navEl.value);
  }
});
onBeforeUnmount(() => observer?.disconnect());
watch(
  () => [route.path, props.items.length],
  () => nextTick(syncIndicator)
);
</script>

<template>
  <nav ref="navEl" class="workspace-menu" aria-label="工作区导航">
    <span class="nav-indicator" :style="pillStyle" aria-hidden="true"></span>
    <section v-for="group in items" :key="group.path">
      <h2 v-if="group.children.length">{{ group.label }}</h2>
      <template v-for="item in flattenNavigation([group])" :key="item.path">
        <a
          v-if="/^(https?:)?\/\//.test(item.path)"
          :href="item.path"
          @click="emit('navigate')"
        >
          <component :is="item.icon" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </a>
        <RouterLink
          v-else
          :to="item.path"
          :aria-current="route.path === item.path ? 'page' : undefined"
          @click="emit('navigate')"
        >
          <component :is="item.icon" aria-hidden="true" />
          <span>{{ item.label }}</span>
        </RouterLink>
      </template>
    </section>
  </nav>
</template>

<style scoped>
.workspace-menu {
  position: relative;
  display: grid;
  gap: 20px;
  padding: 6px 14px 28px;
}
.nav-indicator {
  position: absolute;
  left: 14px;
  right: 14px;
  top: 0;
  border-radius: var(--radius-control);
  background: var(--rail-hover);
  pointer-events: none;
  transition:
    transform var(--duration-medium) var(--ease-in-out),
    height var(--duration-medium) var(--ease-in-out),
    opacity var(--duration-fast) ease;
}
.nav-indicator::before {
  content: "";
  position: absolute;
  left: 0;
  top: 9px;
  bottom: 9px;
  width: 2.5px;
  border-radius: 2px;
  background: var(--rail-accent);
}
.workspace-menu section {
  position: relative;
  z-index: 1;
  display: grid;
  gap: 2px;
}
.workspace-menu h2 {
  color: var(--rail-muted);
  font-size: 12px;
  font-weight: 500;
  line-height: 1.4;
  padding: 2px 12px 6px;
}
.workspace-menu a {
  display: flex;
  align-items: center;
  gap: 11px;
  min-height: 36px;
  position: relative;
  padding: 6px 12px;
  border-radius: var(--radius-control);
  color: var(--rail-text);
  font-size: 13.5px;
  line-height: 1.4;
  transition:
    color var(--duration-fast) ease,
    background-color var(--duration-fast) ease,
    transform var(--duration-press) var(--ease-out);
}
@media (hover: hover) and (pointer: fine) {
  .workspace-menu a:hover:not([aria-current="page"]) {
    color: var(--rail-active);
    background: color-mix(in srgb, var(--rail-hover) 55%, transparent);
  }
}
.workspace-menu a:active {
  transform: scale(0.985);
}
.workspace-menu a[aria-current="page"] {
  color: var(--rail-active);
  font-weight: 580;
}
.workspace-menu a[aria-current="page"] svg {
  color: var(--rail-accent);
}
.workspace-menu a:focus-visible {
  outline: 2px solid var(--rail-accent);
  outline-offset: -2px;
}
.workspace-menu svg {
  width: 16px;
  height: 16px;
  flex: none;
  opacity: 0.78;
  transition:
    color var(--duration-fast) ease,
    opacity var(--duration-fast) ease;
}
.workspace-menu a:hover svg,
.workspace-menu a[aria-current="page"] svg {
  opacity: 1;
}
@media (max-width: 900px) {
  .workspace-menu a {
    min-height: 44px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .nav-indicator,
  .workspace-menu a {
    transition: none;
  }
}
</style>
