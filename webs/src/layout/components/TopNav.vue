<template>
  <nav
    ref="root"
    class="top-nav"
    aria-label="主导航"
    @pointerleave="onLeave"
    @keydown.esc="close(true)"
  >
    <i ref="pill" class="nav-pill" aria-hidden="true"></i>
    <template v-for="(item, index) in items" :key="item.path">
      <div
        v-if="item.children.length"
        class="nav-entry"
        :data-index="index"
        @pointerenter="hover(index, true)"
      >
        <button
          :ref="(el) => (triggers[index] = el as HTMLElement)"
          class="nav-item"
          :class="{ active: activeIndex === index }"
          :aria-expanded="openIndex === index"
          :aria-controls="`nav-menu-${index}`"
          @click="toggle(index)"
          @keydown.down.prevent="openAndFocus(index)"
        >
          {{ item.label }}
          <IconCaret class="nav-caret" aria-hidden="true" />
        </button>
        <Transition name="menu">
          <div
            v-if="openIndex === index"
            :id="`nav-menu-${index}`"
            class="nav-menu"
            @keydown="onMenuKeydown"
          >
            <router-link
              v-for="leaf in flattenNavigation([item])"
              :key="leaf.path"
              :to="leaf.path"
              class="menu-link"
              :class="{ current: route.path === leaf.path }"
              :aria-current="route.path === leaf.path ? 'page' : undefined"
              @click="close()"
            >
              <span class="menu-icon"><component :is="leaf.icon" /></span>
              <span class="menu-copy"
                ><strong>{{ leaf.label }}</strong
                ><small v-if="leaf.hint">{{ leaf.hint }}</small></span
              >
            </router-link>
          </div>
        </Transition>
      </div>
      <router-link
        v-else
        :ref="(el) => (triggers[index] = (el as any)?.$el ?? el)"
        :to="item.path"
        class="nav-item"
        :class="{ active: activeIndex === index }"
        :aria-current="activeIndex === index ? 'page' : undefined"
        :data-index="index"
        @pointerenter="hover(index, false)"
        >{{ item.label }}</router-link
      >
    </template>
  </nav>
</template>
<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRoute } from "vue-router";
import gsap from "gsap";
import IconCaret from "~icons/ph/caret-down-bold";
import { useMotion } from "@/motion";
import { flattenNavigation, type NavigationItem } from "../navigation";
const props = defineProps<{ items: NavigationItem[] }>();
const route = useRoute();
const root = ref<HTMLElement>(),
  pill = ref<HTMLElement>();
const triggers: HTMLElement[] = [];
const openIndex = ref(-1),
  hoverIndex = ref(-1);
const { animate, reduced } = useMotion();
const activeIndex = computed(() =>
  props.items.findIndex((item) =>
    flattenNavigation([item]).some((leaf) => leaf.path === route.path)
  )
);
let placed = false;
// The pill springs to the hovered entry and settles back on the active one.
function movePill(index: number) {
  const target = triggers[index];
  if (!pill.value) return;
  if (!target || index < 0) {
    pill.value.style.opacity = "0";
    placed = false;
    return;
  }
  const base = root.value!.getBoundingClientRect();
  const rect = target.getBoundingClientRect();
  // Measure against the nav box: group triggers sit inside positioned wrappers.
  const to = { x: rect.left - base.left - 1, width: rect.width, opacity: 1 };
  if (!placed || reduced.value) gsap.set(pill.value, to);
  else
    animate(
      pill.value,
      {},
      { ...to, duration: 0.45, ease: "elastic.out(1,.8)" }
    );
  placed = true;
}
let closeTimer: ReturnType<typeof setTimeout> | undefined;
function hover(index: number, group: boolean) {
  clearTimeout(closeTimer);
  hoverIndex.value = index;
  movePill(index);
  openIndex.value = group ? index : -1;
}
function onLeave() {
  clearTimeout(closeTimer);
  closeTimer = setTimeout(() => {
    openIndex.value = -1;
    hoverIndex.value = -1;
    movePill(activeIndex.value);
  }, 140);
}
function toggle(index: number) {
  openIndex.value = openIndex.value === index ? -1 : index;
}
async function openAndFocus(index: number) {
  openIndex.value = index;
  await nextTick();
  root.value?.querySelector<HTMLElement>(".nav-menu .menu-link")?.focus();
}
function close(restore = false) {
  const index = openIndex.value;
  openIndex.value = -1;
  if (restore && index >= 0) triggers[index]?.focus();
}
function onMenuKeydown(event: KeyboardEvent) {
  const links = Array.from(
    root.value?.querySelectorAll<HTMLElement>(".nav-menu .menu-link") || []
  );
  const i = links.indexOf(document.activeElement as HTMLElement);
  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    event.preventDefault();
    const step = event.key === "ArrowDown" ? 1 : -1;
    links[(i + step + links.length) % links.length]?.focus();
  }
}
function onDocumentPointer(event: PointerEvent) {
  if (!root.value?.contains(event.target as Node)) openIndex.value = -1;
}
watch(
  () => route.path,
  async () => {
    openIndex.value = -1;
    await nextTick();
    movePill(activeIndex.value);
  }
);
let observer: ResizeObserver | undefined;
onMounted(async () => {
  await nextTick();
  movePill(activeIndex.value);
  document.addEventListener("pointerdown", onDocumentPointer);
  observer = new ResizeObserver(() => {
    placed = false;
    movePill(hoverIndex.value >= 0 ? hoverIndex.value : activeIndex.value);
  });
  if (root.value) observer.observe(root.value);
});
onBeforeUnmount(() => {
  clearTimeout(closeTimer);
  document.removeEventListener("pointerdown", onDocumentPointer);
  observer?.disconnect();
});
</script>
<style scoped>
.top-nav {
  position: relative;
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 4px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--c-surface) 80%, transparent);
  box-shadow: var(--edge-highlight);
}
.nav-pill {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 0;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  box-shadow: var(--edge-highlight);
  opacity: 0;
  pointer-events: none;
}
.nav-entry {
  position: relative;
}
.nav-item {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: var(--radius-pill);
  color: var(--c-muted);
  font-size: 14px;
  font-weight: 500;
  white-space: nowrap;
  transition: color var(--duration-fast) var(--ease-out);
}
.nav-item:hover,
.nav-item[aria-expanded="true"] {
  color: var(--c-ink);
}
.nav-item.active {
  color: var(--c-ink);
}
.nav-item.active::after {
  content: "";
  position: absolute;
  left: 50%;
  bottom: 3px;
  width: 14px;
  height: 2px;
  margin-left: -7px;
  border-radius: 2px;
  background: var(--c-focus);
}
.nav-caret {
  width: 10px;
  height: 10px;
  transition: transform var(--duration-medium) var(--ease-spring);
}
[aria-expanded="true"] > .nav-caret {
  transform: rotate(180deg);
}
.nav-menu {
  position: absolute;
  top: calc(100% + 12px);
  left: 50%;
  z-index: 10;
  display: grid;
  gap: 2px;
  width: 300px;
  margin-left: -150px;
  padding: 6px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-overlay), var(--edge-highlight);
}
/* Bridge the gap so the pointer can travel from trigger to menu. */
.nav-menu::before {
  content: "";
  position: absolute;
  inset: -14px 0 auto;
  height: 14px;
}
.menu-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border-radius: 12px;
  color: var(--c-ink);
  transition: background-color var(--duration-fast) var(--ease-out);
}
.menu-link:hover,
.menu-link:focus-visible {
  background: var(--c-surface-2);
}
.menu-icon {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 34px;
  height: 34px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-canvas);
  color: var(--c-muted);
  transition:
    color var(--duration-fast),
    transform var(--duration-medium) var(--ease-spring);
}
.menu-icon svg {
  width: 18px;
  height: 18px;
}
.menu-link:hover .menu-icon {
  color: var(--c-ink);
  transform: rotate(-6deg) scale(1.06);
}
.menu-link.current .menu-icon {
  border-color: transparent;
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.menu-copy {
  display: grid;
  min-width: 0;
  line-height: 1.35;
}
.menu-copy strong {
  font-size: 14px;
  font-weight: 500;
}
.menu-copy small {
  color: var(--c-muted);
  font-size: 12px;
}
.menu-enter-active,
.menu-leave-active {
  transition:
    opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-medium) var(--ease-spring);
}
.menu-enter-from,
.menu-leave-to {
  opacity: 0;
  transform: translateY(-6px) scale(0.97);
}
@media (prefers-reduced-motion: reduce) {
  .menu-enter-active,
  .menu-leave-active {
    transition: none;
  }
}
</style>
