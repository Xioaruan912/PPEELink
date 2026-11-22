<template>
  <el-drawer
    v-bind="$attrs"
    class="workspace-drawer"
    :model-value="modelValue"
    :title="title"
    :size="size"
    :direction="direction"
    :destroy-on-close="destroyOnClose"
    @update:model-value="emit('update:modelValue', $event)"
    ><template #header
      ><slot name="header"
        ><h2 class="overlay-heading">{{ title }}</h2></slot
      ></template
    ><slot></slot
    ><template v-if="$slots.footer" #footer
      ><div class="drawer-actions"><slot name="footer"></slot></div></template
  ></el-drawer>
</template>
<script setup lang="ts">
defineOptions({ inheritAttrs: false });
withDefaults(
  defineProps<{
    modelValue: boolean;
    title: string;
    size?: string;
    direction?: "rtl" | "ltr" | "ttb" | "btt";
    destroyOnClose?: boolean;
  }>(),
  { size: "480px", direction: "rtl", destroyOnClose: true }
);
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>
<style scoped>
.drawer-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}
.drawer-actions :deep(.el-button + .el-button) {
  margin-left: 0;
}
.overlay-heading {
  margin: 0;
  padding-right: 20px;
  font-size: 16px;
  color: var(--c-ink);
  font-weight: 600;
  letter-spacing: -0.02em;
}
</style>

<style>
.el-drawer.workspace-drawer {
  --el-drawer-padding-primary: clamp(18px, 3vw, 30px);
  border-radius: 0;
  background: var(--c-canvas);
}
.workspace-drawer .el-drawer__header {
  margin: 0;
  padding-block: 18px;
  background: var(--c-canvas);
}
.workspace-drawer .el-drawer__body {
  padding-top: 8px;
}
.workspace-drawer .el-drawer__footer {
  padding-block: 16px;
  background: var(--c-surface);
}
</style>
