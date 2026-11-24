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
  width: 34px;
  min-width: 34px;
  height: 34px;
  padding: 0;
  gap: 0;
  border-radius: 10px;
}
.icon-hit-area {
  display: inline-grid;
  place-items: center;
  width: 100%;
  height: 100%;
}
.row-icon {
  width: 16px;
  height: 16px;
}
</style>
