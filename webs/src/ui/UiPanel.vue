<template>
  <section
    v-reveal
    class="ui-panel"
    :class="{ 'is-flush': flush, 'is-collapsed': collapsible && !open }"
    :aria-label="title"
  >
    <header v-if="title || $slots.header" class="ui-panel__header">
      <component
        :is="collapsible ? 'button' : 'div'"
        class="ui-panel__heading"
        :aria-expanded="collapsible ? open : undefined"
        @click="collapsible && (open = !open)"
      >
        <slot name="header"
          ><h2>
            <span>{{ title }}</span
            ><span v-if="count !== undefined" class="ui-panel__count">{{
              count
            }}</span>
          </h2>
          <p v-if="description" class="ui-panel__description">
            {{ description }}
          </p></slot
        >
      </component>
      <div v-if="$slots.actions || collapsible" class="ui-panel__actions">
        <slot name="actions"></slot>
        <button
          v-if="collapsible"
          class="ui-panel__toggle"
          :aria-label="open ? `收起${title}` : `展开${title}`"
          @click="open = !open"
        >
          <ArrowDown :class="{ 'is-open': open }" />
        </button>
      </div>
    </header>
    <div
      v-if="!collapsible || open"
      class="ui-panel__body"
      :class="{ 'has-header': title || $slots.header }"
    >
      <slot></slot>
    </div>
    <footer
      v-if="$slots.footer && (!collapsible || open)"
      class="ui-panel__footer"
    >
      <slot name="footer"></slot>
    </footer>
  </section>
</template>
<script setup lang="ts">
import { ref } from "vue";
import { ArrowDown } from "@element-plus/icons-vue";
const props = defineProps<{
  title?: string;
  description?: string;
  count?: number | string;
  flush?: boolean;
  collapsible?: boolean;
  defaultOpen?: boolean;
}>();
const open = ref(props.defaultOpen ?? !props.collapsible);
</script>
<style scoped>
.ui-panel {
  min-width: 0;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  overflow: clip;
}
.ui-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 20px;
  flex-wrap: wrap;
  padding: 20px 24px;
  border-bottom: 1px solid var(--c-line);
}
.is-collapsed .ui-panel__header {
  border-bottom: 0;
}
.ui-panel__heading {
  min-width: 0;
  flex: 1 1 180px;
  text-align: left;
  color: inherit;
}
h2 {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin: 0;
  font-size: 16px;
  font-weight: 680;
  letter-spacing: -0.02em;
}
.ui-panel__count {
  font: 600 13px/1 var(--font-mono);
  font-variant-numeric: tabular-nums;
  color: var(--c-muted);
}
.ui-panel__count::before {
  content: "[";
  margin-right: 2px;
}
.ui-panel__count::after {
  content: "]";
  margin-left: 2px;
}
.ui-panel__description {
  margin-top: 4px;
  color: var(--c-muted);
  font-size: 13px;
}
.ui-panel__actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}
.ui-panel__toggle {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  color: var(--c-muted);
  transition:
    background-color var(--duration-fast) ease,
    color var(--duration-fast) ease;
}
@media (hover: hover) and (pointer: fine) {
  .ui-panel__toggle:hover {
    background: var(--c-surface-2);
    color: var(--c-ink);
  }
}
.ui-panel__toggle svg {
  width: 15px;
  height: 15px;
  transition: transform var(--duration-medium) var(--ease-out);
}
.ui-panel__toggle svg.is-open {
  transform: rotate(180deg);
}
.ui-panel__body {
  padding: 24px;
  min-width: 0;
}
.is-flush .ui-panel__body {
  padding: 0;
}
.ui-panel__footer {
  padding: 16px 24px;
  border-top: 1px solid var(--c-line);
  background: var(--c-surface-2);
}
@media (max-width: 767px) {
  .ui-panel__header {
    padding: 16px;
  }
  .ui-panel__body {
    padding: 16px;
  }
  .is-flush .ui-panel__body {
    padding: 0;
  }
}
</style>
