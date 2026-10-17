<template>
  <div
    ref="root"
    class="dock-group"
    @pointerenter="onPointerEnter"
    @pointerleave="onPointerLeave"
    @focusin="open = true"
    @focusout="onFocusOut"
    @keydown.esc.stop.prevent="close"
    @keydown="onKeydown"
  >
    <button
      v-if="item.children.length"
      ref="trigger"
      class="dock-trigger"
      :class="{ active }"
      :aria-label="item.label"
      :aria-expanded="open"
      :aria-controls="panelId"
      @click="focusLeaf(0)"
    >
      <SvgIcon :icon-class="item.icon" size="22px" /><span
        class="mobile-label"
        >{{ item.label }}</span
      >
    </button>
    <a
      v-else-if="external"
      :href="item.path"
      class="dock-trigger"
      :aria-label="item.label"
      ><SvgIcon :icon-class="item.icon" size="22px" /><span
        class="mobile-label"
        >{{ item.label }}</span
      ></a
    >
    <router-link
      v-else
      :to="item.path"
      class="dock-trigger"
      :class="{ active }"
      :aria-label="item.label"
      :aria-current="active ? 'page' : undefined"
      ><SvgIcon :icon-class="item.icon" size="22px" /><span
        class="mobile-label"
        >{{ item.label }}</span
      ></router-link
    >
    <Transition name="dock-panel"
      ><div
        v-if="open"
        :id="panelId"
        class="dock-panel"
        :class="{ 'dock-panel--label': !item.children.length }"
      >
        <p>{{ item.label }}</p>
        <nav v-if="item.children.length" :aria-label="item.label">
          <a
            v-for="child in leaves"
            :key="child.path"
            :href="
              /^(https?:)?\/\//.test(child.path) ? child.path : `#${child.path}`
            "
            :aria-current="route.path === child.path ? 'page' : undefined"
            @click="open = false"
            ><SvgIcon :icon-class="child.icon" /><span>{{
              child.label
            }}</span></a
          >
        </nav>
      </div></Transition
    >
  </div>
</template>
<script setup lang="ts">
import {
  computed,
  nextTick,
  ref,
  watch,
  onMounted,
  onBeforeUnmount,
  getCurrentInstance,
} from "vue";
import { useRoute } from "vue-router";
import SvgIcon from "@/components/SvgIcon/index.vue";
import { flattenNavigation, type NavigationItem } from "../navigation";
const props = defineProps<{ item: NavigationItem; active: boolean }>();
const route = useRoute();
const root = ref<HTMLElement>();
const open = ref(false),
  trigger = ref<HTMLButtonElement>();
const instanceId = getCurrentInstance()?.uid;
const panelId = computed(
  () => `dock-panel-${instanceId}-${encodeURIComponent(props.item.path)}`
);
const leaves = computed(() => flattenNavigation(props.item.children));
const external = computed(() => /^(https?:)?\/\//.test(props.item.path));
function onPointerEnter(event: PointerEvent) {
  if (event.pointerType !== "touch") open.value = true;
}
async function focusLeaf(index: number) {
  open.value = true;
  await nextTick();
  const links =
    root.value?.querySelectorAll<HTMLAnchorElement>(".dock-panel nav a");
  if (links?.length) links[(index + links.length) % links.length].focus();
}
function onKeydown(event: KeyboardEvent) {
  if (!props.item.children.length) return;
  const links = Array.from(
    root.value?.querySelectorAll(".dock-panel nav a") || []
  );
  const index = links.indexOf(document.activeElement as Element);
  let next: number;
  switch (event.key) {
    case "ArrowDown":
    case "ArrowRight":
      next = index + 1;
      break;
    case "ArrowUp":
    case "ArrowLeft":
      next = index < 0 ? leaves.value.length - 1 : index - 1;
      break;
    case "Home":
      next = 0;
      break;
    case "End":
      next = leaves.value.length - 1;
      break;
    default:
      return;
  }
  event.preventDefault();
  void focusLeaf(next);
}
watch(
  () => route.path,
  () => {
    open.value = false;
  }
);
function onPointerLeave() {
  if (!root.value?.contains(document.activeElement)) open.value = false;
}
function onFocusOut(event: FocusEvent) {
  if (
    !(event.currentTarget as HTMLElement).contains(event.relatedTarget as Node)
  )
    open.value = false;
}
function close() {
  trigger.value?.focus();
  open.value = false;
}
function outside(event: PointerEvent) {
  if (!root.value?.contains(event.target as Node)) open.value = false;
}
onMounted(() => document.addEventListener("pointerdown", outside));
onBeforeUnmount(() => document.removeEventListener("pointerdown", outside));
</script>
<style scoped>
.dock-group {
  position: relative;
}
.dock-group:hover,
.dock-group:focus-within {
  z-index: 1;
}
.dock-trigger {
  position: relative;
  width: 52px;
  min-height: 52px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  border: 0;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--c-muted);
  transition:
    transform var(--duration-slow) var(--ease-spring),
    background var(--duration-fast);
}
.dock-trigger:hover,
.dock-trigger:focus-visible {
  transform: scale(1.12);
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.dock-trigger.active {
  color: var(--c-ink);
  background: var(--c-selected);
}
.dock-trigger.active :deep(svg) {
  color: var(--c-signal);
}
.dock-panel {
  position: absolute;
  left: 100%;
  top: 0;
  width: 240px;
  max-height: 70dvh;
  overflow-y: auto;
  padding: 20px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-overlay);
}
.dock-panel p {
  margin: 0 0 8px;
  color: var(--c-muted);
  font-size: 14px;
}
.dock-panel nav {
  display: grid;
  gap: 4px;
}
.dock-panel a {
  display: flex;
  align-items: center;
  gap: 12px;
  min-height: 44px;
  padding: 8px;
  border-radius: var(--radius-control);
  color: var(--c-ink);
  font-size: 15px;
}
.dock-panel a:hover,
.dock-panel a[aria-current] {
  background: var(--c-selected);
}
.dock-panel--label {
  width: max-content;
  max-width: 240px;
  padding: 12px 20px;
}
.dock-panel--label p {
  margin: 0;
  color: var(--c-ink);
}
.mobile-label {
  display: none;
}
.dock-panel-enter-active,
.dock-panel-leave-active {
  transition:
    opacity var(--duration-fast),
    transform var(--duration-medium) var(--ease-out);
}
.dock-panel-enter-from,
.dock-panel-leave-to {
  opacity: 0;
  transform: translateX(-8px);
}
@media (max-width: 767px) {
  .dock-trigger {
    width: 100%;
    min-height: 56px;
    padding: 4px;
  }
  .mobile-label {
    display: block;
    font-size: 13px;
    max-width: 64px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dock-panel {
    position: fixed;
    top: auto;
    bottom: calc(var(--mobile-dock-height) + env(safe-area-inset-bottom) + 8px);
    left: 16px;
    right: 16px;
    width: auto;
    max-height: 50dvh;
  }
  .dock-panel--label {
    display: none;
  }
}
</style>
