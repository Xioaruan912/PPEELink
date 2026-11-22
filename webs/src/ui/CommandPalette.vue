<template>
  <UiDialog
    :model-value="modelValue"
    title="快速跳转"
    width="640px"
    :show-close="false"
    class="command-palette"
    @update:model-value="emit('update:modelValue', $event)"
    @open-auto-focus="onOpenAutoFocus"
    @closed="onClosed"
  >
    <div class="command-search">
      <Search aria-hidden="true" /><input
        ref="input"
        v-model="query"
        role="combobox"
        aria-label="搜索页面或操作"
        aria-autocomplete="list"
        :aria-expanded="modelValue"
        :aria-controls="listId"
        :aria-activedescendant="
          filtered[selected] ? `${listId}-${selected}` : undefined
        "
        placeholder="搜索页面或操作"
        @keydown.down.prevent="move(1)"
        @keydown.up.prevent="move(-1)"
        @keydown.enter.prevent="execute(filtered[selected])"
      /><kbd>Esc</kbd>
    </div>
    <ul
      :id="listId"
      class="command-results"
      role="listbox"
      aria-label="搜索结果"
    >
      <li
        v-for="(item, index) in filtered"
        :id="`${listId}-${index}`"
        :key="item.id"
        role="option"
        :aria-selected="selected === index"
      >
        <button
          type="button"
          :tabindex="-1"
          :class="{ selected: selected === index }"
          @pointermove="selected = index"
          @click="execute(item)"
        >
          <component
            :is="item.icon"
            v-if="item.icon"
            class="command-icon"
          /><span
            >{{ item.label
            }}<small v-if="item.group">{{ item.group }}</small></span
          ><span aria-hidden="true">↵</span>
        </button>
      </li>
    </ul>
    <EmptyState
      v-if="!filtered.length"
      size="section"
      title="没有结果"
      description="试试其他页面名称。"
    />
    <template #footer
      ><p class="command-help">↑ ↓ 选择 <span>Enter 打开</span></p></template
    >
  </UiDialog>
</template>
<script lang="ts">
import type { Component } from "vue";
export interface CommandItem {
  id: string;
  label: string;
  group?: string;
  icon?: Component;
  keywords?: string;
  run: () => void | Promise<unknown>;
}
let paletteCount = 0;
</script>
<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { Search } from "@element-plus/icons-vue";
import UiDialog from "./UiDialog.vue";
import EmptyState from "./EmptyState.vue";
const props = defineProps<{ modelValue: boolean; items: CommandItem[] }>();
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
const listId = `command-results-${++paletteCount}`;
const input = ref<HTMLInputElement>(),
  query = ref(""),
  selected = ref(0);
function fuzzy(needle: string, haystack: string) {
  let at = 0;
  for (const char of haystack.toLocaleLowerCase()) {
    if (char === needle[at]) at++;
    if (at === needle.length) return true;
  }
  return needle.length === 0;
}
const filtered = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase().replace(/\s+/g, "");
  return props.items.filter((item) =>
    fuzzy(needle, `${item.label} ${item.group || ""} ${item.keywords || ""}`)
  );
});
function move(by: number) {
  if (filtered.value.length)
    selected.value =
      (selected.value + by + filtered.value.length) % filtered.value.length;
}
let pendingAction: CommandItem["run"] | undefined;
function execute(item?: CommandItem) {
  if (!item) return;
  pendingAction = item.run;
  emit("update:modelValue", false);
}
function onClosed() {
  reset();
  const action = pendingAction;
  pendingAction = undefined;
  void action?.();
}
function reset() {
  query.value = "";
  selected.value = 0;
}
function focusSearch() {
  nextTick(() => input.value?.focus());
}
function onOpenAutoFocus(event?: Event) {
  event?.preventDefault();
  focusSearch();
}
watch(query, () => {
  selected.value = 0;
});
watch(filtered, () => {
  selected.value = Math.min(
    selected.value,
    Math.max(0, filtered.value.length - 1)
  );
});
watch(selected, () =>
  nextTick(() =>
    document
      .getElementById(`${listId}-${selected.value}`)
      ?.scrollIntoView({ block: "nearest" })
  )
);
function shortcut(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    emit("update:modelValue", !props.modelValue);
  }
}
onMounted(() => window.addEventListener("keydown", shortcut));
onBeforeUnmount(() => window.removeEventListener("keydown", shortcut));
</script>
<style scoped>
.command-search {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 2px 0 14px;
  border-bottom: 1px solid var(--c-line);
}
.command-search svg {
  width: 18px;
  height: 18px;
  color: var(--c-subtle);
  flex: 0 0 auto;
}
.command-search input {
  width: 100%;
  min-width: 0;
  min-height: 36px;
  border: 0;
  outline: none;
  color: var(--c-ink);
  background: transparent;
  font-size: 15px;
  letter-spacing: -0.01em;
}
.command-search input::placeholder {
  color: var(--c-subtle);
}
kbd {
  padding: 3px 7px;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 1;
}
.command-results {
  list-style: none;
  padding: 10px 0 4px;
  margin: 0;
  max-height: 46dvh;
  overflow-y: auto;
  display: grid;
  gap: 2px;
}
.command-results button {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  padding: 10px 14px;
  min-height: 48px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--c-ink);
  cursor: pointer;
  text-align: left;
  transition:
    background var(--t-fast) var(--ease-expo),
    border-color var(--t-fast) var(--ease-expo),
    transform var(--t-fast) var(--ease-expo);
}
.command-results button.selected {
  background: var(--c-surface-2);
  border-color: var(--c-line);
  transform: translateX(2px);
}
.command-icon {
  width: 17px;
  height: 17px;
  color: var(--c-muted);
  flex: 0 0 auto;
}
.command-results button.selected .command-icon {
  color: var(--c-ink);
}
.command-results button > span:first-of-type {
  flex: 1;
  min-width: 0;
  font-size: 13.5px;
  font-weight: 500;
}
.command-results button > span:last-of-type {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--c-subtle);
  opacity: 0;
  transition: opacity var(--t-fast) var(--ease-expo);
}
.command-results button.selected > span:last-of-type {
  opacity: 1;
  color: var(--c-signal);
}
.command-results small {
  display: block;
  margin-top: 2px;
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 400;
}
.command-help {
  display: flex;
  gap: 16px;
  color: var(--c-subtle);
  font-family: var(--font-mono);
  font-size: 11px;
  letter-spacing: 0.04em;
  margin: 0;
}
</style>
