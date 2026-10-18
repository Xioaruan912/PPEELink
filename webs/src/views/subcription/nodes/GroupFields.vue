<script setup lang="ts">
import { UiSegmented } from "@/ui";
import { useNodes } from "./useNodes";
const {
  groupMode,
  chosenGroups,
  newGroups,
  allGroupNames,
  groupDetails,
  dialogMode,
} = useNodes();
const options = computed(() =>
  groupMode.value === "existing" && dialogMode.value === "edit"
    ? groupDetails.value.map((g) => g.name)
    : allGroupNames.value
);
</script>
<template>
  <div class="group-fields">
    <UiSegmented
      v-model="groupMode"
      :options="[
        {
          label: '已有分组',
          value: 'existing',
          disabled: !groupDetails.length,
        },
        { label: '新分组', value: 'new' },
      ]"
      label="分组来源"
    />
    <el-form-item v-if="groupMode === 'existing'" label="选择分组" required>
      <el-select
        v-model="chosenGroups"
        multiple
        filterable
        placeholder="至少选择一个分组"
        aria-label="选择分组"
      >
        <el-option
          v-for="group in options"
          :key="group"
          :label="group"
          :value="group"
        />
      </el-select>
    </el-form-item>
    <el-form-item v-else label="新分组名称" required
      ><el-input v-model="newGroups" placeholder="多个分组用英文逗号分隔"
    /></el-form-item>
  </div>
</template>
<style scoped>
.group-fields :deep(.ui-segmented) {
  margin-bottom: 16px;
}
.group-fields :deep(.el-select) {
  width: 100%;
}
</style>
