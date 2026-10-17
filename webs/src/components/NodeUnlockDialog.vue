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
                ? '有可解锁服务'
                : '无可解锁服务'
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
import { UnlockTest } from "@/api/subcription/node";
import UnlockResults from "./node-tests/UnlockResults.vue";
import type { UnlockResult } from "./node-tests/unlock";
import { saveUnlock, getUnlock } from "@/utils/nodeTestStorage";
import UiDialog from "@/ui/UiDialog.vue";
import UiButton from "@/ui/UiButton.vue";
import StatusDot from "@/ui/StatusDot.vue";
import EmptyState from "@/ui/EmptyState.vue";
import {
  token,
  useNodeTestSession,
  readEvents,
} from "./node-tests/useNodeTestSession";
const props = defineProps<{
  visible: boolean;
  nodeId: number;
  nodeName: string;
}>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
const result = ref<UnlockResult | null>(null);
const streaming = ref(false);
const { loading, run, abort, stop } = useNodeTestSession();
async function startTest() {
  const nodeId = props.nodeId;
  const nodeName = props.nodeName;
  try {
    await run(async (signal, current, setReader) => {
      result.value = null;
      streaming.value = false;
      // JSON is the current Go contract. A compatible SSE response is read as
      // genuine arrivals, with the same cancellation and node-version guards.
      let json: any;
      if (typeof fetch !== "function") {
        const form = new FormData();
        form.append("id", String(nodeId));
        json = await UnlockTest(form);
      } else {
        const response = await fetch(
          import.meta.env.VITE_APP_BASE_API + "/api/v1/nodes/unlock",
          {
            method: "POST",
            headers: {
              "Content-Type": "application/x-www-form-urlencoded",
              Authorization: token(),
            },
            body: new URLSearchParams({ id: String(nodeId) }),
            signal,
          }
        );
        if (!current()) return;
        if (!response.ok) {
          json = await response.json().catch(() => null);
          throw new Error(json?.msg || "解锁检测失败");
        }
        if (
          response.headers.get("content-type")?.includes("text/event-stream")
        ) {
          streaming.value = true;
          await readEvents(
            response,
            (type, payload) => {
              if (
                type === "error" ||
                (payload?.code && payload.code !== "00000")
              )
                throw new Error(payload?.msg || "解锁检测失败");
              const data = payload?.data ?? payload;
              if (Array.isArray(data?.results)) {
                result.value = { ...data, nodeName: data.nodeName || nodeName };
              } else if (typeof data?.key === "string") {
                const rows = new Map(
                  (result.value?.results || []).map((row) => [row.key, row])
                );
                rows.set(data.key, data);
                const results = [...rows.values()];
                result.value = {
                  nodeName,
                  ok: results.some((row) => row.ok),
                  results,
                };
              }
            },
            current,
            setReader
          );
          if (current()) {
            if (!result.value) throw new Error("未收到解锁检测结果");
            saveUnlock(nodeId, result.value);
          }
          return;
        }
        json = await response.json();
      }
      if (json?.code !== "00000") throw new Error(json?.msg || "解锁检测失败");
      if (current()) {
        result.value = json.data;
        saveUnlock(nodeId, json.data);
      }
    });
  } catch (cause: any) {
    if (cause?.name !== "AbortError")
      ElMessage.error(cause?.message || "解锁检测失败");
  }
}
async function stopTest() {
  try {
    await stop();
  } catch {
    ElMessage.error("停止失败，请稍后重试");
  }
}
function restore() {
  result.value = getUnlock(props.nodeId) || null;
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
    if (visible && !loading.value) restore();
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
