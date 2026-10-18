<template>
  <el-dialog
    v-bind="$attrs"
    :model-value="modelValue"
    :title="title"
    :width="width"
    :destroy-on-close="destroyOnClose"
    @update:model-value="emit('update:modelValue', $event)"
    ><template #header
      ><slot name="header"
        ><h2 class="overlay-heading">{{ title }}</h2></slot
      ></template
    ><slot></slot
    ><template v-if="$slots.footer" #footer
      ><slot name="footer"></slot></template
  ></el-dialog>
</template>
<script setup lang="ts">
defineOptions({ inheritAttrs: false });
withDefaults(
  defineProps<{
    modelValue: boolean;
    title: string;
    width?: string;
    destroyOnClose?: boolean;
  }>(),
  { width: "560px", destroyOnClose: true }
);
const emit = defineEmits<{ "update:modelValue": [value: boolean] }>();
</script>
<style scoped>
.overlay-heading {
  margin: 0;
  padding-right: 20px;
  font-size: 17px;
}
</style>
