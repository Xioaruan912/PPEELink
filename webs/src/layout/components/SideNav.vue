<template>
  <nav
    ref="root"
    class="side-nav"
    :class="{ collapsed }"
    aria-label="主导航"
    @keydown="onKeydown"
  >
    <i ref="indicator" class="side-indicator" aria-hidden="true"></i>
    <template v-for="group in items" :key="group.path">
      <section v-if="group.children.length" class="side-group">
        <h2 class="side-group__label">{{ group.label }}</h2>
        <NavLink
          v-for="leaf in flattenNavigation([group])"
          :key="leaf.path"
          :item="leaf"
          :active="route.path === leaf.path"
          :collapsed="collapsed"
          @navigate="emit('navigate')"
        />
      </section>
      <NavLink
        v-else
        :item="group"
        :active="route.path === group.path"
        :collapsed="collapsed"
        @navigate="emit('navigate')"
      />
    </template>
  </nav>
</template>
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute } from "vue-router";
import gsap from "gsap";
import { useMotion } from "@/motion";
import NavLink from "./NavLink.vue";
import { flattenNavigation, type NavigationItem } from "../navigation";
const props = defineProps<{ items: NavigationItem[]; collapsed?: boolean }>();
const emit = defineEmits<{ navigate: [] }>();
const route = useRoute();
const root = ref<HTMLElement>(),
  indicator = ref<HTMLElement>();
const { animate, stop } = useMotion();
let previous: { y: number; h: number } | null = null;
// The indicator springs between active rows; collapse/resize just re-measures.
async function place() {
  await nextTick();
  const el = root.value?.querySelector<HTMLElement>(".nav-link.active");
  const bar = indicator.value;
  if (!bar) return;
  if (!el) {
    bar.style.opacity = "0";
    previous = null;
    return;
  }
  bar.style.opacity = "1";
  const next = { y: el.offsetTop, h: el.offsetHeight };
  stop();
  if (previous)
    animate(
      bar,
      { y: previous.y, height: previous.h },
      { y: next.y, height: next.h, duration: 0.5, ease: "elastic.out(1,.75)" }
    );
  else gsap.set(bar, { y: next.y, height: next.h });
  previous = next;
}
function onKeydown(event: KeyboardEvent) {
  const links = Array.from(
    root.value?.querySelectorAll<HTMLElement>(".nav-link") || []
  );
  const index = links.indexOf(event.target as HTMLElement);
  if (index < 0) return;
  const map: Record<string, number> = {
    ArrowDown: (index + 1) % links.length,
    ArrowUp: (index - 1 + links.length) % links.length,
    Home: 0,
    End: links.length - 1,
  };
  if (!(event.key in map)) return;
  event.preventDefault();
  links[map[event.key]]?.focus();
}
let observer: ResizeObserver | undefined;
onMounted(() => {
  place();
  observer = new ResizeObserver(() => {
    previous = null;
    place();
  });
  if (root.value) observer.observe(root.value);
});
onBeforeUnmount(() => observer?.disconnect());
watch(() => [route.path, props.items, props.collapsed], place);
</script>
<style scoped>
.side-nav {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 4px 12px;
}
.side-indicator {
  position: absolute;
  top: 0;
  left: 12px;
  right: 12px;
  height: 36px;
  border-radius: var(--radius-control);
  background: var(--c-selected);
  box-shadow: inset 3px 0 0 var(--c-signal);
  pointer-events: none;
  opacity: 0;
}
.side-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 14px;
}
.side-group__label {
  margin: 0 0 4px;
  padding: 0 10px;
  color: var(--c-muted);
  font: 500 12px/1.6 var(--font-mono);
  letter-spacing: 0.06em;
  white-space: nowrap;
  overflow: hidden;
}
.collapsed .side-group__label {
  height: 1px;
  margin: 6px 10px 8px;
  padding: 0;
  background: var(--c-line);
  color: transparent;
}
</style>
