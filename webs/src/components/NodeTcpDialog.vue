<template>
  <UiDialog
    :model-value="visible"
    title="链路检测"
    width="1120px"
    top="5vh"
    :close-on-click-modal="false"
    :destroy-on-close="false"
    @update:model-value="emit('update:visible', $event)"
  >
    <div class="test-context">
      <strong>{{ nodeName }}</strong
      ><StatusDot
        :state="loading ? 'warn' : targets.length ? 'ok' : 'idle'"
        :running="loading"
        :label="
          loading
            ? '正在检测'
            : targets.length
              ? `${targets.length} 个目标`
              : '尚未检测'
        "
      />
    </div>
    <div class="test-toolbar">
      <div class="test-context">
        <UiButton variant="primary" :loading="loading" @click="startTest">{{
          targets.length ? "重新测速" : "开始测速"
        }}</UiButton
        ><UiButton v-if="loading" variant="danger" @click="stopTest"
          >停止测试</UiButton
        >
      </div>
      <UiSegmented
        v-model="zstaticPort"
        label="zstaticcdn 检测端口"
        :options="ports"
      />
    </div>
    <p class="method-hint">VPS → 节点 → 国内目标 · 双样本均值</p>
    <div class="legend" aria-label="延迟图例">
      <span
        v-for="(label, index) in latencyLegend"
        :key="label"
        :class="`latency-${index}`"
        ><i aria-hidden="true"></i>{{ label }}</span
      >
    </div>
    <div class="tcp-layout">
      <ChinaTcpMap v-if="visible" :groups="provinceGroups" />
      <div class="province-scroll" aria-live="polite" aria-atomic="false">
        <TransitionGroup
          name="result-list"
          tag="div"
          appear
          @enter="enter"
          @leave="leave"
          ><section
            v-for="(group, index) in provinceGroups"
            :key="group.province"
            class="province"
            :data-index="index"
          >
            <div class="province-heading">
              <h3>{{ group.province }}</h3>
              <span
                >{{ group.okCount }}/{{ group.items.length }} 可达 ·
                {{ group.avg >= 0 ? `${group.avg} ms` : "超时" }}</span
              >
            </div>
            <TransitionGroup
              name="result-list"
              tag="div"
              class="target-list"
              appear
              @enter="enter"
              @leave="leave"
              ><article
                v-for="(target, targetIndex) in group.items"
                :key="`${target.ip}:${target.port}`"
                class="target"
                :class="`latency-${latencyBand(target.rtt)}`"
                :data-index="targetIndex"
              >
                <div>
                  <strong>{{ target.city || group.province }}</strong
                  ><span>{{ target.isp }}</span>
                </div>
                <StatusDot
                  class="test-mono target-rtt"
                  :state="
                    target.rtt < 0 || target.rtt >= 300
                      ? 'bad'
                      : target.rtt < 100
                        ? 'ok'
                        : 'warn'
                  "
                  :label="target.rtt < 0 ? '超时' : `${target.rtt} ms`"
                /></article
            ></TransitionGroup></section></TransitionGroup
        ><EmptyState
          v-if="!provinceGroups.length"
          size="section"
          :title="loading ? '正在探测各省' : '暂无检测结果'"
          :description="
            loading ? '结果将按省份陆续显示' : '开始测速以查看国内链路'
          "
        />
      </div>
    </div>
    <section v-if="zstaticResults.length" class="test-section">
      <div class="test-section-heading">
        <h3>zstaticcdn</h3>
        <span>缓存随节点保存</span>
      </div>
      <div class="zstatic-results">
        <article
          v-for="item in zstaticResults"
          :key="item.port"
          :class="`latency-${latencyBand(item.rtt)}`"
        >
          <span
            >端口 <strong class="test-mono">{{ item.port }}</strong></span
          ><StatusDot
            :state="item.rtt < 0 ? 'bad' : 'ok'"
            :label="item.rtt < 0 ? '超时' : `${item.rtt} ms`"
          />
        </article>
      </div>
    </section>
    <template #footer
      ><UiButton @click="emit('update:visible', false)"
        >关闭</UiButton
      ></template
    >
  </UiDialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import { saveTcp, getTcp } from "@/utils/nodeTestStorage";
import UiDialog from "@/ui/UiDialog.vue";
import UiButton from "@/ui/UiButton.vue";
import UiSegmented from "@/ui/UiSegmented.vue";
import StatusDot from "@/ui/StatusDot.vue";
import EmptyState from "@/ui/EmptyState.vue";
import ChinaTcpMap from "./node-tests/ChinaTcpMap.vue";
import {
  latencyBand,
  latencyLegend,
  type ChinaTarget,
} from "./node-tests/latency";
import {
  readEvents,
  token,
  useNodeTestSession,
  useResultMotion,
} from "./node-tests/useNodeTestSession";
defineOptions({ name: "NodeTcpDialog" });
const props = defineProps<{
  visible: boolean;
  nodeId: number;
  nodeName: string;
}>();
const emit = defineEmits<{ "update:visible": [value: boolean] }>();
const targets = ref<ChinaTarget[]>([]);
const zstaticResults = ref<{ port: number; rtt: number }[]>([]);
const zstaticPort = ref<string | number>("443");
const ports = [
  { label: "zstaticcdn · 443", value: "443" },
  { label: "端口 80", value: "80" },
];
const { loading, run, abort, stop } = useNodeTestSession();
const { enter, leave } = useResultMotion();
const provinceGroups = computed(() => {
  const groups = new Map<string, ChinaTarget[]>();
  targets.value.forEach((target) =>
    groups.set(target.province, [
      ...(groups.get(target.province) || []),
      target,
    ])
  );
  return [...groups]
    .sort(([a], [b]) => a.localeCompare(b, "zh-CN"))
    .map(([province, items]) => {
      const ok = items.filter((item) => item.rtt >= 0);
      return {
        province,
        items,
        okCount: ok.length,
        avg: ok.length
          ? Math.round(ok.reduce((sum, item) => sum + item.rtt, 0) / ok.length)
          : -1,
      };
    });
});
async function startTest() {
  const nodeId = props.nodeId;
  const port = String(zstaticPort.value);
  try {
    await run(async (signal, current, setReader) => {
      targets.value = [];
      zstaticResults.value = [];
      const response = await fetch(
        import.meta.env.VITE_APP_BASE_API + "/api/v1/nodes/chinaping/stream",
        {
          method: "POST",
          headers: {
            "Content-Type": "application/x-www-form-urlencoded",
            Authorization: token(),
          },
          body: new URLSearchParams({ id: String(nodeId), zstatic_port: port }),
          signal,
        }
      );
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        throw new Error(body?.msg || `请求失败 · ${response.status}`);
      }
      await readEvents(
        response,
        (type, data) => {
          if (type === "province" && Array.isArray(data?.targets)) {
            const merged = new Map(
              targets.value.map((target) => [
                `${target.ip}:${target.port}`,
                target,
              ])
            );
            data.targets.forEach((target: ChinaTarget) =>
              merged.set(`${target.ip}:${target.port}`, target)
            );
            targets.value = [...merged.values()];
          } else if (type === "zstatic" && Array.isArray(data?.zstatic))
            zstaticResults.value = data.zstatic;
          else if (type === "error") throw new Error(data?.msg || "测速失败");
        },
        current,
        setReader
      );
      if (current())
        saveTcp(nodeId, {
          targets: targets.value,
          zstatic: zstaticResults.value,
        });
    });
  } catch (cause: any) {
    if (cause?.name !== "AbortError")
      ElMessage.error(cause?.message || "测速失败");
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
  const cached = getTcp(props.nodeId);
  targets.value = cached?.targets || [];
  zstaticResults.value = cached?.zstatic || [];
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
.method-hint {
  margin: 0 0 16px;
  color: var(--c-muted);
  font-size: 14px;
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin: 0 0 16px;
  font: 400 14px/1.5 var(--font-mono);
  color: var(--c-muted);
}
.legend > span {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.legend i {
  height: 8px;
  width: 16px;
  border-radius: 4px;
  background: var(--latency);
}
.latency-0 {
  --latency: var(--c-ok);
}
.latency-1 {
  --latency: var(--c-data);
}
.latency-2 {
  --latency: var(--c-warn);
}
.latency-3 {
  --latency: var(--c-signal);
}
.latency-4 {
  --latency: var(--c-bad);
}
.latency-5 {
  --latency: var(--c-muted);
}
.tcp-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 16px;
}
.province-scroll {
  max-height: 440px;
  overflow: auto;
  padding-right: 8px;
}
.province {
  margin-bottom: 16px;
}
.province-heading {
  display: flex;
  align-items: baseline;
  flex-wrap: wrap;
  gap: 8px 12px;
  padding: 0 0 8px;
  border-bottom: 1px solid var(--c-line);
}
.province-heading h3 {
  margin: 0;
  font-size: 14px;
}
.province-heading > span {
  font-size: 14px;
  color: var(--c-muted);
}
.target-list {
  position: relative;
}
.target {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  align-items: center;
  min-height: 56px;
  padding: 8px 0;
  border-bottom: 1px solid var(--c-line);
}
.target > div {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.target strong {
  font-size: 14px;
  font-weight: 500;
}
.target div > span {
  font-size: 14px;
  color: var(--c-muted);
}
.target-rtt {
  font-size: 14px;
  white-space: nowrap;
  border-left: 3px solid var(--latency);
  padding-left: 8px;
}
.zstatic-results {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
.zstatic-results article {
  display: flex;
  gap: 16px;
  align-items: center;
  padding: 12px;
  border: 1px solid var(--c-line);
  border-left: 3px solid var(--latency);
  border-radius: 10px;
  font-size: 14px;
}
@media (max-width: 767px) {
  .tcp-layout {
    grid-template-columns: 1fr;
  }
  .province-scroll {
    max-height: none;
    overflow: visible;
    padding: 0;
  }
  .target {
    padding: 12px;
    margin-top: 8px;
    border: 1px solid var(--c-line);
    border-radius: 10px;
  }
  .zstatic-results article {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
