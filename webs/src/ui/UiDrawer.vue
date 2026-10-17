<template>
  <el-drawer
    v-bind="$attrs"
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
      ><slot name="footer"></slot></template
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
.overlay-heading {
  margin: 0;
  padding-right: 32px;
  font-size: 22px;
}
</style>
