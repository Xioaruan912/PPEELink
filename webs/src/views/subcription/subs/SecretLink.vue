<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { UiButton } from "@/ui";
import { useMotion } from "@/motion";
import { copySubscriptionText, maskSubscriptionLink } from "./clipboard";
const props = withDefaults(defineProps<{ value: string; label?: string }>(), {
  label: "订阅链接",
});
const revealed = ref(false),
  copied = ref(false),
  feedback = ref<HTMLElement>();
const { animate, stop } = useMotion();
watch(
  () => props.value,
  () => {
    revealed.value = false;
    copied.value = false;
  }
);
async function copy() {
  try {
    await copySubscriptionText(props.value);
    copied.value = true;
    ElMessage.success("已复制");
    if (feedback.value) {
      stop();
      animate(
        feedback.value,
        { y: 4, opacity: 0.4 },
        { y: 0, opacity: 1, duration: 0.32, clearProps: "transform,opacity" }
      );
    }
  } catch {
    ElMessage.warning("复制失败，请显示链接后手动复制");
  }
}
</script>
<template>
  <div class="secret-link">
    <el-input
      :model-value="revealed ? value : maskSubscriptionLink(value)"
      :aria-label="label"
      readonly
      class="secret-input"
    />
    <div class="secret-actions">
      <UiButton
        variant="ghost"
        :aria-label="`${revealed ? '隐藏' : '显示'}${label}`"
        :aria-pressed="revealed"
        @click="revealed = !revealed"
        >{{ revealed ? "隐藏" : "显示" }}</UiButton
      >
      <UiButton :disabled="!value" @click="copy"
        ><span ref="feedback" role="status">{{
          copied ? "已复制" : "复制"
        }}</span></UiButton
      >
      <slot></slot>
    </div>
  </div>
</template>
<style scoped>
.secret-link {
  min-width: 0;
  display: grid;
  gap: 8px;
}
.secret-input :deep(input) {
  font: 13px/1.6 var(--font-mono);
}
.secret-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
