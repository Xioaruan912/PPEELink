<template>
  <div ref="root" class="ui-segmented" role="radiogroup" :aria-label="label">
    <i ref="slider" class="ui-segmented__slider" aria-hidden="true"></i>
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
import { nextTick, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { useMotion } from "@/motion";
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
const root = ref<HTMLElement>(),
  slider = ref<HTMLElement>();
const { animate, stop } = useMotion();
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
async function position() {
  await nextTick();
  const button = root.value?.querySelector<HTMLElement>(
    '[aria-checked="true"]'
  );
  if (!slider.value) return;
  slider.value.style.opacity = button ? "1" : "0";
  if (!button) return;
  stop();
  slider.value.style.width = `${button.offsetWidth}px`;
  animate(
    slider.value,
    { x: Number(slider.value.dataset.x || button.offsetLeft) },
    { x: button.offsetLeft, duration: 0.56, ease: "elastic.out(1,.6)" }
  );
  slider.value.dataset.x = String(button.offsetLeft);
}
let observer: ResizeObserver | undefined;
onMounted(() => {
  position();
  observer = new ResizeObserver(position);
  if (root.value) observer.observe(root.value);
});
onBeforeUnmount(() => observer?.disconnect());
watch(() => [props.modelValue, props.options], position);
</script>
<style scoped>
.ui-segmented {
  position: relative;
  display: inline-flex;
  max-width: 100%;
  padding: 4px;
  gap: 4px;
  background: var(--c-surface-2);
  overflow-x: auto;
  border-radius: var(--radius-control);
}
.ui-segmented button {
  z-index: 1;
  min-width: 44px;
  min-height: 44px;
  padding: 8px 16px;
  border: 0;
  background: transparent;
  font-size: 15px;
  border-radius: var(--radius-control);
}
.ui-segmented button:disabled {
  color: var(--c-muted);
}
.ui-segmented__slider {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 0;
  background: var(--c-surface);
  border-radius: var(--radius-control);
  box-shadow: var(--shadow-panel);
}
</style>
