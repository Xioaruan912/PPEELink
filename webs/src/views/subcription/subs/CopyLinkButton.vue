<script setup lang="ts">
import { computed, ref, watch, onBeforeUnmount } from "vue";
import { ArrowDown, Check } from "@element-plus/icons-vue";
import { ElMessage } from "element-plus";
import { UiButton } from "@/ui";
import { useMotion } from "@/motion";
import { copySubscriptionText } from "./clipboard";
const props = defineProps<{
  value: string;
  modelValue: string;
  options: { label: string; value: string }[];
  label: string;
}>();
const emit = defineEmits<{ "update:modelValue": [value: string] }>();
const clientTooltipVisible = ref(false);
const copied = ref(false),
  copying = ref(false),
  feedback = ref<HTMLElement>();
const { animate, stop } = useMotion();
let alive = true;
onBeforeUnmount(() => {
  alive = false;
});
const clientLabel = computed(
  () =>
    props.options.find((option) => option.value === props.modelValue)?.label ||
    props.modelValue
);
watch(
  () => props.value,
  () => {
    copied.value = false;
  }
);
async function copy() {
  if (copying.value) return;
  copying.value = true;
  const value = props.value;
  try {
    await copySubscriptionText(value);
    if (!alive || value !== props.value) return;
    copied.value = true;
    ElMessage.success("已复制链接");
    if (feedback.value) {
      stop();
      animate(
        feedback.value,
        { y: 4, opacity: 0.4 },
        { y: 0, opacity: 1, duration: 0.32, clearProps: "transform,opacity" }
      );
    }
  } catch {
    if (alive) ElMessage.warning("复制失败，请在链接详情中手动复制");
  } finally {
    copying.value = false;
  }
}
function selectClient(value: string) {
  if (props.options.some((option) => option.value === value))
    emit("update:modelValue", value);
}
</script>
<template>
  <div class="copy-split" role="group" :aria-label="`${label}链接操作`">
    <el-tooltip
      :trigger="['hover', 'focus']"
      :trigger-keys="[]"
      :content="`复制${clientLabel}链接`"
      placement="top"
      ><UiButton
        class="copy-main"
        :disabled="copying || !value"
        :aria-busy="copying"
        :aria-label="`复制${label}链接，当前客户端：${clientLabel}`"
        @click="copy"
        ><span ref="feedback" role="status">{{
          copying ? "复制中" : copied ? "已复制" : "复制链接"
        }}</span></UiButton
      ></el-tooltip
    >
    <el-dropdown trigger="click" placement="bottom-end" @command="selectClient"
      ><UiButton
        class="copy-client"
        :aria-label="`选择${label}客户端，当前：${clientLabel}`"
        @mouseenter="clientTooltipVisible = true"
        @mouseleave="clientTooltipVisible = false"
        @focus="clientTooltipVisible = true"
        @blur="clientTooltipVisible = false"
        @click="clientTooltipVisible = false"
      >
        <template #icon>
          <el-tooltip
            :content="`选择${label}客户端，当前：${clientLabel}`"
            :visible="clientTooltipVisible"
            :trigger-keys="[]"
          >
            <span class="client-chevron" aria-hidden="true"><ArrowDown /></span>
          </el-tooltip>
        </template> </UiButton
      ><template #dropdown
        ><el-dropdown-menu :aria-label="`${label}客户端`"
          ><el-dropdown-item
            v-for="option in options"
            :key="option.value"
            :command="option.value"
            :aria-current="modelValue === option.value ? 'true' : undefined"
            ><span class="client-option"
              >{{ option.label
              }}<Check
                v-if="modelValue === option.value"
                aria-label="当前客户端"
                class="client-check" /></span></el-dropdown-item></el-dropdown-menu></template
    ></el-dropdown>
  </div>
</template>
<style scoped>
.copy-split {
  display: flex;
  width: 130px;
  min-width: 0;
}
.copy-main {
  flex: 1;
  min-width: 0;
  padding: 8px;
  border-radius: var(--radius-control) 0 0 var(--radius-control);
  gap: 0;
}
.copy-client {
  width: 44px;
  min-width: 44px;
  padding: 12px;
  gap: 0;
  border-left: 0;
  border-radius: 0 var(--radius-control) var(--radius-control) 0;
}
.client-chevron {
  display: inline-flex;
  width: 18px;
  height: 18px;
}
.client-option {
  min-width: 128px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}
.client-check {
  width: 16px;
  height: 16px;
  color: var(--c-data);
}
</style>
