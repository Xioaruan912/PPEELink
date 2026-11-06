<script setup lang="ts">
import {
  ref,
  computed,
  onMounted,
  onBeforeUnmount,
  onDeactivated,
  watch,
} from "vue";
import { PageHeader, UiButton, UiSkeleton, EmptyState } from "@/ui";
import { getNodeOverview, getUnlockServices } from "@/api/subcription/node";
import UnlockResults from "@/components/node-tests/UnlockResults.vue";
import {
  unlockGroups,
  type UnlockResult,
  type UnlockService,
  type UnlockCheckResult,
} from "@/components/node-tests/unlock";
import { runBrowserUnlock } from "@/components/node-tests/browserUnlock";
import { requestNodeUnlock } from "@/components/node-tests/nodeUnlock";
defineOptions({ name: "Unlock" });
const props = withDefaults(defineProps<{ mode?: "node" | "browser" }>(), {
  mode: "node",
});
const device = computed(() => props.mode === "browser");
const nodes = ref<{ id: number; name: string; valid: boolean }[]>([]);
const services = ref<UnlockService[]>([]),
  selectedNodeId = ref<number>(),
  group = ref("all");
const loading = ref(false),
  preparing = ref(true),
  loadError = ref(""),
  testError = ref("");
const result = ref<UnlockResult | null>(null);
const targets = computed(() =>
  services.value.filter((s) => group.value === "all" || s.group === group.value)
);
let alive = true,
  version = 0,
  controller: AbortController | undefined;
const sourceName = computed(() =>
  device.value
    ? "当前设备"
    : nodes.value.find((n) => n.id === selectedNodeId.value)?.name || "指定节点"
);
async function prepare() {
  preparing.value = true;
  loadError.value = "";
  try {
    const [catalog, overview] = await Promise.all([
      getUnlockServices(),
      device.value ? Promise.resolve(null) : getNodeOverview(),
    ]);
    if (!alive) return;
    services.value = catalog.data || [];
    nodes.value = (overview?.data || [])
      .filter((n: { valid: boolean }) => n.valid)
      .sort((a: { name: string }, b: { name: string }) =>
        a.name.localeCompare(b.name, "zh-CN")
      );
  } catch {
    if (alive) loadError.value = "无法加载检测项目，请重试。";
  } finally {
    if (alive) preparing.value = false;
  }
}
function stop() {
  version++;
  controller?.abort();
  controller = undefined;
  if (loading.value && result.value)
    result.value = {
      ...result.value,
      complete: false,
      error: "检测已停止，仅展示已完成项目。",
    };
  loading.value = false;
}
async function startTest() {
  if (loading.value || (!device.value && !selectedNodeId.value)) return;
  const own = new AbortController(),
    id = ++version;
  controller = own;
  loading.value = true;
  testError.value = "";
  const selected = targets.value,
    name = sourceName.value;
  result.value = {
    source: props.mode,
    nodeName: name,
    complete: false,
    ok: false,
    results: [],
  };
  const current = () => alive && id === version && !own.signal.aborted;
  const receive = (row: UnlockCheckResult) => {
    if (!current()) return;
    const rows = new Map(result.value!.results.map((r) => [r.key, r]));
    rows.set(row.key, row);
    const ordered = selected
      .map((s) => rows.get(s.key))
      .filter((r): r is UnlockCheckResult => !!r);
    result.value = {
      source: props.mode,
      nodeName: name,
      complete: false,
      ok: ordered.some((r) => r.ok),
      results: ordered,
    };
  };
  try {
    const data = device.value
      ? await runBrowserUnlock(selected, own.signal, receive)
      : await requestNodeUnlock(
          selectedNodeId.value!,
          group.value === "all" ? "" : group.value,
          own.signal,
          receive
        );
    if (current()) result.value = data;
  } catch (cause: any) {
    if (current()) {
      testError.value =
        cause?.name === "AbortError"
          ? "检测连接已超时，请重试。"
          : cause?.message || "检测失败，请重试。";
      if (result.value) result.value.error = testError.value;
    }
  } finally {
    if (id === version) {
      loading.value = false;
      controller = undefined;
    }
  }
}
watch([selectedNodeId, group], () => {
  stop();
  result.value = null;
  testError.value = "";
});
onMounted(prepare);
onDeactivated(stop);
onBeforeUnmount(() => {
  alive = false;
  stop();
});
</script>
<template>
  <div class="unlock-page">
    <PageHeader
      :title="device ? '当前设备检测' : '指定节点检测'"
      :description="
        device
          ? '检测请求由当前浏览器发出，跟随 Clash Verge 等客户端的代理与分流规则。请先启用需要测试的代理。'
          : '由面板通过所选节点检测服务，不使用当前设备的代理配置。'
      "
    />
    <p v-if="device" class="scope-note">
      结果仅保留在当前页面，不会写入节点记录。浏览器安全限制或扩展可能影响检测；仅可访问不代表服务已解锁。
    </p>
    <section class="test-controls" aria-label="检测设置">
      <div v-if="!device" class="picker node-picker">
        <label for="unlock-node">选择节点</label>
        <el-select
          id="unlock-node"
          v-model="selectedNodeId"
          filterable
          placeholder="搜索可访问节点"
          :loading="preparing"
          :disabled="preparing || loading"
          aria-label="选择要检测的节点"
        >
          <el-option
            v-for="node in nodes"
            :key="node.id"
            :label="node.name"
            :value="node.id"
          />
        </el-select>
      </div>
      <div class="picker">
        <label :for="device ? 'device-group' : 'node-group'">检测类别</label>
        <el-select
          :id="device ? 'device-group' : 'node-group'"
          v-model="group"
          :disabled="loading || preparing"
          aria-label="检测类别"
        >
          <el-option
            v-for="item in unlockGroups"
            :key="item.key"
            :value="item.key"
            :label="item.label"
          />
        </el-select>
      </div>
      <UiButton
        variant="primary"
        :loading="loading"
        :disabled="
          preparing ||
          !!loadError ||
          !targets.length ||
          (!device && !selectedNodeId)
        "
        @click="startTest"
        >{{ result ? "重新检测" : "开始检测" }}</UiButton
      >
      <UiButton v-if="loading" variant="danger" @click="stop"
        >停止检测</UiButton
      >
      <span class="progress" role="status" aria-live="polite">{{
        loading
          ? `正在检测 ${result?.results.length || 0} / ${targets.length} 项`
          : result
            ? result.complete
              ? "检测完成"
              : "检测未完成"
            : `共 ${targets.length} 项服务，手动开始检测`
      }}</span>
    </section>
    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载检测项目"
      :description="loadError"
      ><template #action
        ><UiButton @click="prepare">重新加载</UiButton></template
      ></EmptyState
    >
    <section v-else class="results-region" :aria-busy="loading || preparing">
      <UiSkeleton v-if="preparing" :lines="3" height="64px" />
      <template v-else-if="result">
        <header class="result-heading">
          <h2>{{ result.nodeName }}</h2>
          <span
            >{{ result.results.filter((r) => r.status === "available").length }}
            项接口或页面通过 ·
            {{ result.results.filter((r) => r.status === "inferred").length }}
            项推测可用</span
          >
        </header>
        <p
          v-if="loading && !result.results.length"
          class="scope-note"
          role="status"
        >
          正在连接目标服务，结果将逐项显示。
        </p>
        <UnlockResults :result="result" />
        <UiButton v-if="testError && !loading" @click="startTest"
          >重新检测</UiButton
        >
      </template>
      <EmptyState
        v-else-if="!device && !nodes.length"
        size="section"
        title="暂无可访问节点"
        description="请先添加节点或检查连接。"
        ><template #action
          ><UiButton @click="prepare">刷新节点</UiButton></template
        ></EmptyState
      >
      <EmptyState
        v-else
        size="section"
        :title="device ? '检查当前设备的实际访问情况' : '选择节点开始检测'"
        description="按服务展示检测依据与目标地区；请求失败会标记为无法判断。"
      />
    </section>
  </div>
</template>
<style scoped>
.unlock-page {
  min-width: 0;
}
.scope-note {
  margin: 0 0 16px;
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.7;
}
.test-controls {
  display: flex;
  align-items: flex-end;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 16px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.picker {
  width: 200px;
  max-width: 100%;
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.node-picker {
  width: 320px;
}
.picker label {
  font-size: 14px;
}
.picker :deep(.el-select) {
  width: 100%;
}
.progress {
  min-height: 36px;
  display: flex;
  align-items: center;
  color: var(--c-muted);
  font-size: 13px;
}
.results-region {
  min-height: 320px;
  padding-top: 20px;
}
.result-heading {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: space-between;
  gap: 12px;
}
.result-heading h2 {
  margin: 0;
  font: 600 17px/1.4 var(--font-ui);
  overflow-wrap: anywhere;
}
.result-heading span {
  color: var(--c-muted);
  font-size: 13px;
}
@media (max-width: 767px) {
  .test-controls {
    align-items: stretch;
    gap: 12px;
  }
  .picker {
    width: 100%;
  }
}
</style>
