<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { useStagger } from "@/motion";
import { useTemplateBuilder } from "./useTemplateBuilder";
import {
  RULE_CATEGORIES,
  RULE_PRESETS,
  type RuleCategory,
} from "./ruleCategories";

const { categoryEnabled, toggleCategory, applyCategoryPreset } =
  useTemplateBuilder();
const activePreset = ref("");
const grid = ref<HTMLElement>();
const stagger = useStagger();

const enabledCount = computed(
  () => RULE_CATEGORIES.filter((c) => categoryEnabled(c)).length
);

const targetLabel = (category: RuleCategory) =>
  ({
    main: "主分组",
    DIRECT: "直连",
    REJECT: "拦截",
    group: `${category.group?.replace(/^\S+\s/, "") || "专属组"}`,
  })[category.target];

function applyPreset(value: string) {
  const item = RULE_PRESETS.find((p) => p.value === value);
  if (!item) return;
  activePreset.value = value;
  applyCategoryPreset(item.keys);
  ElMessage.success(`已套用「${item.label}」`);
  if (grid.value) stagger(grid.value.querySelectorAll(".category.is-on"));
}
</script>

<template>
  <section class="rule-picker" aria-label="分流规则分类">
    <!-- Single-Line Preset Pills + Active Counter (Zero Dropdown) -->
    <header class="picker-bar">
      <div class="preset-pills" role="group" aria-label="规则预设">
        <span class="bar-label">一键预设</span>
        <button
          v-for="item in RULE_PRESETS"
          :key="item.value"
          type="button"
          class="preset-pill"
          :class="{ 'is-active': activePreset === item.value }"
          @click="applyPreset(item.value)"
        >
          {{ item.label }}
        </button>
      </div>

      <span class="picker-count mono" role="status">
        已启用 <strong>{{ enabledCount }}</strong> / {{ RULE_CATEGORIES.length }} 类
      </span>
    </header>

    <!-- Full-Width 5-Column Tactile Bento Grid -->
    <div ref="grid" class="category-grid">
      <label
        v-for="category in RULE_CATEGORIES"
        :key="category.key"
        class="category"
        :class="{ 'is-on': categoryEnabled(category) }"
      >
        <input
          type="checkbox"
          :checked="categoryEnabled(category)"
          @change="
            toggleCategory(
              category,
              ($event.target as HTMLInputElement).checked
            )
          "
        />
        <span class="category-emoji" aria-hidden="true">
          <component :is="category.icon" />
        </span>
        <span class="category-copy">
          <strong>{{ category.label }}</strong>
          <small>{{ targetLabel(category) }}</small>
        </span>
        <span class="category-check" aria-hidden="true"></span>
      </label>
    </div>
  </section>
</template>

<style scoped>
.rule-picker {
  display: grid;
  gap: 16px;
}

.picker-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.preset-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.bar-label {
  font-size: 12px;
  color: var(--c-muted);
  margin-right: 4px;
}

.preset-pill {
  padding: 6px 13px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    transform 100ms ease-out,
    background 180ms var(--ease-out),
    color 180ms var(--ease-out),
    border-color 180ms var(--ease-out);
}

.preset-pill:hover {
  border-color: var(--c-ink);
}

.preset-pill:active {
  transform: scale(0.97);
}

.preset-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.picker-count {
  font-size: 12.5px;
  color: var(--c-muted);
}

.picker-count strong {
  color: var(--c-ok);
}

.category-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(185px, 1fr));
  gap: 10px;
}

.category {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 14px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  cursor: pointer;
  user-select: none;
  transition:
    transform 120ms ease-out,
    border-color 180ms var(--ease-out),
    background 180ms var(--ease-out),
    box-shadow 180ms var(--ease-out);
}

.category:hover {
  border-color: var(--c-ink);
}

.category:active {
  transform: scale(0.97);
}

.category.is-on {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ok) 6%, var(--c-surface));
  box-shadow: 0 10px 24px -16px rgb(0 0 0 / 0.14);
}

.category input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}

.category-emoji {
  display: grid;
  place-items: center;
  flex: none;
  width: 34px;
  height: 34px;
  border-radius: 10px;
  background: var(--c-surface-muted);
  color: var(--c-muted);
  transition:
    transform 240ms cubic-bezier(0.16, 1, 0.3, 1),
    background 180ms var(--ease-out),
    color 180ms var(--ease-out);
}

.category-emoji svg {
  width: 17px;
  height: 17px;
}

.category.is-on .category-emoji {
  transform: scale(1.06);
  background: var(--c-ink);
  color: var(--c-surface);
}

.category-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.category-copy strong {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.category-copy small {
  font-size: 11.5px;
  color: var(--c-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.category-check {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1.5px solid var(--c-line);
  transition:
    background 180ms var(--ease-out),
    border-color 180ms var(--ease-out);
}

.category.is-on .category-check {
  background: var(--c-ok);
  border-color: var(--c-ok);
}
</style>
