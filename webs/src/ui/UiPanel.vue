<template>
  <section
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
            {{ title
            }}<span v-if="count !== undefined" class="ui-panel__count">{{
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
  /** Body sits edge to edge (tables, lists). */
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
  box-shadow: var(--shadow-panel);
}
.ui-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px 16px;
  flex-wrap: wrap;
  padding: 14px 16px;
}
.is-collapsed .ui-panel__header {
  padding-bottom: 14px;
}
.ui-panel:not(.is-collapsed) .ui-panel__header {
  border-bottom: 1px solid var(--c-line);
}
.ui-panel__heading {
  min-width: 0;
  flex: 1 1 140px;
  text-align: left;
  color: inherit;
}
h2 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.ui-panel__count {
  padding: 0 7px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font: 500 12px/20px var(--font-mono);
}
.ui-panel__description {
  margin-top: 2px;
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
  width: 32px;
  height: 32px;
  border-radius: var(--radius-control);
  color: var(--c-muted);
}
.ui-panel__toggle:hover {
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.ui-panel__toggle svg {
  width: 16px;
  height: 16px;
  transition: transform var(--duration-medium) var(--ease-spring);
}
.ui-panel__toggle svg.is-open {
  transform: rotate(180deg);
}
.ui-panel__body {
  padding: 16px;
  min-width: 0;
}
.is-flush .ui-panel__body {
  padding: 0;
}
.ui-panel__footer {
  padding: 12px 16px;
  border-top: 1px solid var(--c-line);
}
@media (max-width: 767px) {
  .ui-panel__header,
  .ui-panel__body {
    padding: 12px;
  }
  .is-flush .ui-panel__body {
    padding: 0;
  }
}
</style>
