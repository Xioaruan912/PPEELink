<template>
  <section class="rule-picker" aria-labelledby="rule-picker-title">
    <header class="picker-head">
      <div>
        <h3 id="rule-picker-title">规则分类</h3>
        <p>勾选即添加对应规则集、规则行和专属分组；取消会一并移除。</p>
      </div>
      <el-select
        v-model="preset"
        class="preset-select"
        aria-label="规则预设"
        placeholder="套用预设"
        @change="applyPreset"
      >
        <el-option
          v-for="item in RULE_PRESETS"
          :key="item.value"
          :label="item.label"
          :value="item.value"
        />
      </el-select>
    </header>
    <p class="picker-count" role="status">
      已选 <strong class="mono">{{ enabledCount }}</strong> /
      {{ RULE_CATEGORIES.length }} 类
    </p>
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
        <span class="category-emoji" aria-hidden="true"
          ><component :is="category.icon"
        /></span>
        <span class="category-copy"
          ><strong>{{ category.label }}</strong
          ><small>{{ targetLabel(category) }}</small></span
        >
      </label>
    </div>
  </section>
</template>
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
const preset = ref("");
const grid = ref<HTMLElement>();
const stagger = useStagger();
const enabledCount = computed(
  () => RULE_CATEGORIES.filter((c) => categoryEnabled(c)).length
);
const targetLabel = (category: RuleCategory) =>
  ({
    main: "走主分组",
    DIRECT: "直连",
    REJECT: "拦截",
    group: `专属分组 ${category.group?.replace(/^\S+\s/, "")}`,
  })[category.target];
function applyPreset(value: string) {
  const item = RULE_PRESETS.find((p) => p.value === value);
  if (!item) return;
  applyCategoryPreset(item.keys);
  ElMessage.success(`已套用「${item.label}」规则`);
  if (grid.value) stagger(grid.value.querySelectorAll(".category.is-on"));
}
</script>
<style scoped>
.rule-picker {
  display: grid;
  gap: 12px;
  margin-bottom: 20px;
}
.picker-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.picker-head p,
.picker-count {
  color: var(--c-muted);
  font-size: 13px;
}
.preset-select {
  width: 160px;
}
.picker-count strong {
  color: var(--c-ink);
}
.category-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  gap: 8px;
}
.category {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 52px;
  padding: 8px 10px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background-color var(--duration-fast) var(--ease-out),
    transform var(--duration-medium) var(--ease-spring);
}
.category:hover {
  border-color: var(--c-muted);
}
.category:active {
  transform: scale(0.97);
}
.category.is-on {
  border-color: color-mix(in srgb, var(--c-focus) 70%, var(--c-line));
  background: var(--c-selected);
}
.category:focus-within {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.category input {
  position: absolute;
  opacity: 0;
  pointer-events: none;
}
.category-emoji {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  transition:
    transform var(--duration-medium) var(--ease-spring),
    background-color var(--duration-fast),
    color var(--duration-fast);
}
.category-emoji svg {
  width: 17px;
  height: 17px;
}
.is-on .category-emoji {
  transform: rotate(-8deg) scale(1.08);
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.category-copy {
  display: grid;
  min-width: 0;
  line-height: 1.3;
}
.category-copy strong {
  font-size: 13px;
  font-weight: 600;
}
.category-copy small {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
