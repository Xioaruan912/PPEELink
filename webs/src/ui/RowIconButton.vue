<script setup lang="ts">
import { ref, type Component } from "vue";
import UiButton from "./UiButton.vue";
defineOptions({ inheritAttrs: false });
defineProps<{ label: string; icon: Component }>();
const tooltipVisible = ref(false);
</script>
<template>
  <UiButton
    v-bind="$attrs"
    class="row-icon-button"
    variant="ghost"
    :aria-label="label"
    @mouseenter="tooltipVisible = true"
    @mouseleave="tooltipVisible = false"
    @focus="tooltipVisible = true"
    @blur="tooltipVisible = false"
    @click="tooltipVisible = false"
  >
    <template #icon>
      <el-tooltip
        :trigger-keys="[]"
        :content="label"
        :visible="tooltipVisible"
        placement="top"
      >
        <span class="icon-hit-area" aria-hidden="true"
          ><component :is="icon" class="row-icon"
        /></span>
      </el-tooltip>
    </template>
  </UiButton>
</template>
<style scoped>
.row-icon-button {
  width: 44px;
  min-width: 44px;
  height: 44px;
  padding: 0;
  gap: 0;
}
.icon-hit-area {
  display: inline-grid;
  place-items: center;
  width: 42px;
  height: 42px;
}
.row-icon {
  width: 18px;
  height: 18px;
}
</style>
