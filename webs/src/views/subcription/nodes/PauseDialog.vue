<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { UiButton, UiDialog, UiSegmented } from "@/ui";
import { pauseNodes } from "@/api/subcription/node";
import { useNodes } from "./useNodes";

/**
 * 暂停下发：节点本月流量用完等情况，暂停到恢复日。期间不出现在任何订阅里，到日子自动恢复。
 */
const { pauseDialog, pauseTargets, loadAll } = useNodes();
const mode = ref<string | number>("next-month");
const resetDay = ref(1);
const pickedDate = ref("");
const note = ref("本月流量已用完");
const saving = ref(false);

const pad = (n: number) => String(n).padStart(2, "0");
const fmt = (d: Date) =>
  `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
// 下一个“每月 day 号”（今天已过或就是今天则取下个月）
function nextMonthlyDay(day: number) {
  const now = new Date();
  const d = new Date(now.getFullYear(), now.getMonth(), day);
  if (d <= now) d.setMonth(d.getMonth() + 1);
  return d;
}
const until = computed(() => {
  if (mode.value === "next-month") return fmt(nextMonthlyDay(1));
  if (mode.value === "monthly") return fmt(nextMonthlyDay(resetDay.value));
  return pickedDate.value;
});
const disabledDate = (d: Date) => d.getTime() <= Date.now();
watch(pauseDialog, (open) => {
  if (open) {
    mode.value = "next-month";
    pickedDate.value = "";
  }
});
const names = computed(() =>
  pauseTargets.value
    .slice(0, 3)
    .map((n) => n.name)
    .join("、")
);
async function submit() {
  if (!until.value) {
    ElMessage.warning("请选择恢复日期");
    return;
  }
  saving.value = true;
  try {
    const res: any = await pauseNodes(
      pauseTargets.value.map((n) => n.id),
      until.value,
      note.value
    );
    ElMessage.success(res?.msg || "已暂停");
    pauseDialog.value = false;
    await loadAll();
  } finally {
    saving.value = false;
  }
}
</script>
<template>
  <UiDialog v-model="pauseDialog" title="暂停下发" width="480px">
    <p class="hint">
      {{ names
      }}{{ pauseTargets.length > 3 ? ` 等 ${pauseTargets.length} 个节点` : "" }}
      暂停期间不会出现在任何订阅里，到恢复日自动恢复下发。适合节点本月流量用完、等下月重置的情况。
    </p>
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item label="恢复时间">
        <UiSegmented
          v-model="mode"
          label="恢复时间"
          :options="[
            { label: '下月 1 号', value: 'next-month' },
            { label: '每月重置日', value: 'monthly' },
            { label: '指定日期', value: 'date' },
          ]"
        />
      </el-form-item>
      <el-form-item v-if="mode === 'monthly'" label="每月几号重置">
        <el-input-number v-model="resetDay" :min="1" :max="28" />
      </el-form-item>
      <el-form-item v-if="mode === 'date'" label="恢复日期">
        <el-date-picker
          v-model="pickedDate"
          type="date"
          value-format="YYYY-MM-DD"
          :disabled-date="disabledDate"
          placeholder="选择日期"
        />
      </el-form-item>
      <el-form-item label="备注（可选）">
        <el-input v-model="note" maxlength="60" show-word-limit />
      </el-form-item>
    </el-form>
    <p v-if="until" class="until">
      将在 <b class="mono">{{ until }}</b> 起恢复下发
    </p>
    <template #footer>
      <UiButton @click="pauseDialog = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" @click="submit"
        >暂停下发</UiButton
      >
    </template>
  </UiDialog>
</template>
<style scoped>
.hint {
  margin: 0 0 14px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
}
.until {
  margin: 0;
  font-size: 13px;
}
.mono {
  font-family: var(--font-mono);
}
</style>
