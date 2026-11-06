<template>
  <UiDialog
    :model-value="visible"
    title="解锁检测"
    width="920px"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="test-context">
      <strong>{{ nodeName }}</strong
      ><StatusDot
        :state="loading ? 'warn' : result ? (result.ok ? 'ok' : 'bad') : 'idle'"
        :running="loading"
        :label="
          loading
            ? '正在检测服务'
            : result
              ? result.ok
                ? '有接口或页面通过'
                : '检测完成，请查看各项依据'
              : '尚未检测'
        "
      />
    </div>
    <div class="test-toolbar">
      <div class="test-context">
        <UiButton variant="primary" :loading="loading" @click="startTest">{{
          result ? "重新检测" : "开始检测"
        }}</UiButton
        ><UiButton v-if="loading" variant="danger" @click="stopTest"
          >停止测试</UiButton
        >
      </div>
      <p v-if="loading" class="loading-hint" role="status">
        {{
          streaming
            ? "正在接收服务检测结果"
            : "等待服务检测完成，结果将统一显示"
        }}
      </p>
      <span v-else-if="result" class="test-context"
        >{{ result.results?.filter((item) => item.ok).length || 0 }} /
        {{ result.results?.length || 0 }} 项可用</span
      >
    </div>
    <UnlockResults v-if="result" :result="result" />
    <EmptyState
      v-else
      size="section"
      :title="loading ? '正在检查解锁能力' : '暂无解锁结果'"
      :description="
        loading ? '完整检测可能需要一些时间' : '开始检测 AI、影视及论坛服务'
      "
    />
    <template #footer
      ><UiButton @click="emit('update:visible', false)"
        >关闭</UiButton
      ></template
    >
  </UiDialog>
</template>
<script setup lang="ts">
import { ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { requestNodeUnlock } from "./node-tests/nodeUnlock";
import UnlockResults from "./node-tests/UnlockResults.vue";
import type { UnlockResult } from "./node-tests/unlock";
import { saveUnlock, getUnlock } from "@/utils/nodeTestStorage";
import UiDialog from "@/ui/UiDialog.vue";
import UiButton from "@/ui/UiButton.vue";
import StatusDot from "@/ui/StatusDot.vue";
import EmptyState from "@/ui/EmptyState.vue";
import { useNodeTestSession } from "./node-tests/useNodeTestSession";
const props = defineProps<{
  visible: boolean;
  nodeId: number;
  nodeName: string;
}>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
const result = ref<UnlockResult | null>(null);
const streaming = ref(false);
const { loading, run, abort } = useNodeTestSession();
async function startTest() {
  const nodeId = props.nodeId;
  const nodeName = props.nodeName;
  try {
    await run(async (signal, current) => {
      result.value = null;
      streaming.value = true;
      const data = await requestNodeUnlock(nodeId, "", signal, (row) => {
        if (!current()) return;
        const rows = new Map(
          (result.value?.results || []).map((item) => [item.key, item])
        );
        rows.set(row.key, row);
        const results = [...rows.values()];
        result.value = {
          source: "node",
          nodeName,
          complete: false,
          ok: results.some((item) => item.ok),
          results,
        };
      });
      if (current()) {
        result.value = data;
        if (data.complete && !data.error) saveUnlock(nodeId, data);
      }
    });
  } catch (cause: any) {
    if (cause?.name !== "AbortError") {
      const message = cause?.message || "解锁检测失败";
      if (result.value) result.value.error = message;
      ElMessage.error(message);
    }
  }
}
async function stopTest() {
  try {
    abort();
    if (result.value)
      result.value = {
        ...result.value,
        complete: false,
        error: "检测已停止，仅展示已完成项目。",
      };
  } catch {
    ElMessage.error("停止失败，请稍后重试");
  }
}
function restore() {
  const stored = getUnlock(props.nodeId);
  result.value = stored?.results?.every(
    (row: { evidence?: string }) => !!row.evidence
  )
    ? stored
    : null;
}
watch(
  () => props.nodeId,
  () => {
    abort();
    restore();
  }
);
watch(
  () => props.visible,
  (visible) => {
    if (!visible) abort();
    else if (!loading.value) restore();
  },
  { immediate: true }
);
</script>
<style scoped lang="scss">
@use "./node-tests/dialog.scss";
.loading-hint {
  margin: 0;
  color: var(--c-muted);
  font-size: 14px;
}
</style>
