<template>
  <div ref="root" class="ui-segmented" role="radiogroup" :aria-label="label">
    <span
      class="ui-segmented__thumb"
      :style="thumbStyle"
      aria-hidden="true"
    ></span>
    <button
      v-for="(option, index) in options"
      :key="option.value"
      type="button"
      role="radio"
      :aria-checked="modelValue === option.value"
      :disabled="option.disabled"
      :tabindex="tabIndex(option, index)"
      @click="select(option)"
      @keydown="onKey($event, index)"
    >
      <slot :option="option">{{ option.label }}</slot>
    </button>
  </div>
</template>
<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";
interface Option {
  label: string;
  value: string | number;
  disabled?: boolean;
}
const props = defineProps<{
  modelValue: string | number;
  options: Option[];
  label: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string | number] }>();
const root = ref<HTMLElement>();
const thumbStyle = ref<{ transform: string; width: string; opacity: number }>({
  transform: "translate3d(0, 0, 0)",
  width: "0px",
  opacity: 0,
});

function syncThumb() {
  const container = root.value;
  if (!container) return;
  const active = container.querySelector<HTMLElement>(
    'button[aria-checked="true"]'
  );
  if (!active) {
    thumbStyle.value = { ...thumbStyle.value, opacity: 0 };
    return;
  }
  thumbStyle.value = {
    transform: `translate3d(${active.offsetLeft}px, 0, 0)`,
    width: `${active.offsetWidth}px`,
    opacity: 1,
  };
}

let observer: ResizeObserver | undefined;
onMounted(() => {
  nextTick(syncThumb);
  if (typeof ResizeObserver !== "undefined" && root.value) {
    observer = new ResizeObserver(syncThumb);
    observer.observe(root.value);
  }
});
onBeforeUnmount(() => observer?.disconnect());
watch(
  () => [props.modelValue, props.options.length],
  () => nextTick(syncThumb)
);

function tabIndex(option: Option, index: number) {
  return !option.disabled &&
    (option.value === props.modelValue ||
      (!props.options.some(
        (o) => o.value === props.modelValue && !o.disabled
      ) &&
        props.options.findIndex((o) => !o.disabled) === index))
    ? 0
    : -1;
}
function select(option: Option) {
  if (!option.disabled) emit("update:modelValue", option.value);
}
function onKey(event: KeyboardEvent, index: number) {
  if (
    ![
      "ArrowLeft",
      "ArrowRight",
      "ArrowUp",
      "ArrowDown",
      "Home",
      "End",
    ].includes(event.key)
  )
    return;
  event.preventDefault();
  const enabled = props.options
    .map((o, i) => (!o.disabled ? i : -1))
    .filter((i) => i >= 0);
  if (!enabled.length) return;
  const offset = ["ArrowLeft", "ArrowUp"].includes(event.key) ? -1 : 1;
  const target =
    event.key === "Home"
      ? enabled[0]
      : event.key === "End"
        ? enabled[enabled.length - 1]
        : enabled[
            (enabled.indexOf(index) + offset + enabled.length) % enabled.length
          ];
  select(props.options[target]);
  (root.value?.querySelectorAll("button")[target] as HTMLElement)?.focus();
}
</script>
<style scoped>
.ui-segmented {
  position: relative;
  display: inline-flex;
  align-items: center;
  max-width: 100%;
  overflow-x: auto;
  gap: 0;
  padding: 3px;
  background: var(--c-surface-2);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
}
.ui-segmented__thumb {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  border-radius: 4px;
  background: var(--c-surface);
  box-shadow: 0 1px 2px rgb(0 0 0 / 0.06);
  pointer-events: none;
  transition:
    transform var(--duration-medium) var(--ease-out),
    width var(--duration-medium) var(--ease-out),
    opacity var(--duration-fast) ease;
}
.ui-segmented button {
  position: relative;
  z-index: 1;
  flex: 1 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 30px;
  padding: 4px 14px;
  border-radius: 4px;
  white-space: nowrap;
  font-size: 13px;
  color: var(--c-muted);
  transition: color var(--duration-fast) var(--ease-out);
}
.ui-segmented button[aria-checked="true"] {
  color: var(--c-ink);
  font-weight: 620;
}
@media (hover: hover) and (pointer: fine) {
  .ui-segmented button:not(:disabled):not([aria-checked="true"]):hover {
    color: var(--c-ink);
  }
}
.ui-segmented button:disabled {
  opacity: 0.45;
}
.ui-segmented button:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: -2px;
}
@media (max-width: 640px) {
  .ui-segmented button {
    min-height: 40px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .ui-segmented__thumb,
  .ui-segmented button {
    transition: none;
  }
}
</style>
