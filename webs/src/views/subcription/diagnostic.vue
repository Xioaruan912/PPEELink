<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from "vue";
import request from "@/utils/request";
import {
  runDiagnosticQueue,
  diagnosticVerdict,
  canOpenDiagnosticDetails,
} from "./diagnosticBatch";
import { getNodes } from "@/api/subcription/node";
import { UiButton, UiSegmented, UiDrawer } from "@/ui";
const mode = ref("node"),
  selected = ref<number>(),
  link = ref(""),
  target = ref("cloudflare");
const nodes = ref<{ ID: number; Name: string; Link: string }[]>([]);
const loading = ref(false),
  listError = ref(""),
  error = ref("");
interface Step {
  key: string;
  title: string;
  status: string;
  detail: string;
  millis: number;
}
interface Result {
  protocol: string;
  transport: string;
  perspective: string;
  steps: Step[];
  summary: string;
  advice: string[];
  checkedAt: string;
}
const result = ref<Result | null>(null);
let controller: AbortController | undefined;
type BatchStatus =
  | "waiting"
  | "running"
  | "passed"
  | "failed"
  | "unknown"
  | "skipped"
  | "cancelled"
  | "error";
interface BatchRow {
  id: number;
  name: string;
  status: BatchStatus;
  note: string;
  result?: Result;
}
const batchRows = ref<BatchRow[]>([]);
const batchRunning = ref(false);
const detailVisible = ref(false);
const detailRow = ref<BatchRow | null>(null);
const detailResult = computed(() => detailRow.value?.result);
function openDetails(row: BatchRow) {
  if (!canOpenDiagnosticDetails(row)) return;
  detailRow.value = { ...row };
  detailVisible.value = true;
}
const batchStatusText: Record<BatchStatus, string> = {
  waiting: "等待中",
  running: "检测中",
  passed: "通过",
  failed: "失败",
  unknown: "待确认",
  skipped: "暂不支持",
  cancelled: "已取消",
  error: "请求错误",
};
const batchDone = computed(
  () =>
    batchRows.value.filter(
      (row) => !["waiting", "running"].includes(row.status)
    ).length
);
const batchCounts = computed(() => ({
  passed: batchRows.value.filter((r) => r.status === "passed").length,
  failed: batchRows.value.filter((r) => r.status === "failed").length,
  unknown: batchRows.value.filter((r) => r.status === "unknown").length,
  skipped: batchRows.value.filter((r) => r.status === "skipped").length,
  errors: batchRows.value.filter((r) => r.status === "error").length,
  cancelled: batchRows.value.filter((r) => r.status === "cancelled").length,
}));
async function runAll() {
  if (loading.value || !nodes.value.length) return;
  loading.value = true;
  batchRunning.value = true;
  detailVisible.value = false;
  detailRow.value = null;
  error.value = "";
  result.value = null;
  const abort = new AbortController();
  controller = abort;
  const batchTarget = target.value;
  batchRows.value = nodes.value.map((n) => ({
    id: n.ID,
    name: n.Name,
    status: "waiting",
    note: "",
  }));
  const links = new Map(nodes.value.map((n) => [n.ID, n.Link]));
  const queue = batchRows.value.filter((row) => {
    const uri = links.get(row.id) || "";
    let supported = false;
    try {
      const u = new URL(uri);
      supported =
        ["hy2:", "hysteria2:"].includes(u.protocol) ||
        (u.protocol === "vless:" &&
          u.searchParams.get("security") === "reality");
    } catch {}
    if (!supported) {
      row.status = "skipped";
      row.note = "当前深度诊断支持 Reality 和 HY2；此协议未发送诊断请求。";
    }
    return supported;
  });
  try {
    await runDiagnosticQueue(
      queue,
      async (row) => {
        const response = await request({
          url: "/api/v1/nodes/diagnose",
          method: "post",
          timeout: 48000,
          signal: abort.signal,
          data: { id: row.id, target: batchTarget },
        });
        return response.data as Result;
      },
      {
        signal: abort.signal,
        onStart(row) {
          row.status = "running";
          row.note = "正在检查配置、网络与代理通路…";
        },
        onResult(row, data) {
          row.result = data;
          row.status = diagnosticVerdict(data);
          row.note = data.summary;
        },
        onError(row, e: any) {
          row.status = "error";
          row.note = e?.message || "诊断请求失败，可单独重试";
        },
        onCancel(row) {
          row.status = "cancelled";
          row.note = "未完成的检测已取消";
        },
      }
    );
  } finally {
    loading.value = false;
    batchRunning.value = false;
    if (controller === abort) controller = undefined;
  }
}

const supported = computed(() =>
  nodes.value.filter((n) => /^(vless|hy2|hysteria2):\/\//i.test(n.Link))
);
async function load() {
  listError.value = "";
  try {
    const response = await getNodes(true);
    nodes.value = Array.isArray(response.data) ? response.data : [];
  } catch {
    listError.value = "节点加载失败，请重试，或切换到粘贴链接。";
  }
}
onMounted(load);
onBeforeUnmount(() => controller?.abort());
async function run() {
  if (loading.value) return;
  error.value = "";
  result.value = null;
  if (mode.value === "node" ? !selected.value : !link.value.trim()) {
    error.value = "请选择节点或填写完整的 Reality / HY2 链接。";
    return;
  }
  loading.value = true;
  controller = new AbortController();
  try {
    const response = await request({
      url: "/api/v1/nodes/diagnose",
      method: "post",
      timeout: 48000,
      signal: controller.signal,
      data: {
        id: mode.value === "node" ? selected.value : 0,
        link: mode.value === "link" ? link.value.trim() : "",
        target: target.value,
      },
    });
    result.value = response.data;
  } catch (e: any) {
    error.value =
      e?.code === "ERR_CANCELED"
        ? "诊断已取消。"
        : e?.message || "诊断请求失败，请重试。";
  } finally {
    loading.value = false;
    controller = undefined;
  }
}
const statusText: Record<string, string> = {
  passed: "通过",
  failed: "失败",
  unknown: "待确认",
};
function copyReport(report?: Result) {
  const r = report || result.value;
  if (!r) return;
  navigator.clipboard
    .writeText(
      [
        r.summary,
        `检测位置：${r.perspective}`,
        `${r.protocol} / ${r.transport}`,
        ...r.steps.map(
          (s) => `${s.title}：${statusText[s.status]} · ${s.detail}`
        ),
        ...r.advice,
      ].join("\n")
    )
    .then(() => ElMessage.success("已复制不含节点密码的诊断结果"))
    .catch(() => ElMessage.error("复制失败，请手动选择结果"));
}
</script>
<template>
  <main class="diagnostic-page">
    <header>
      <div>
        <h1>节点一键诊断</h1>
        <p>逐步检查 Reality / HY2，确认连接在哪一环失败。</p>
      </div>
      <span class="perspective">从面板服务器检测</span>
    </header>
    <section class="diagnostic-form">
      <UiSegmented
        v-model="mode"
        class="source-switch"
        label="节点来源"
        :options="[
          { label: '选择已有节点', value: 'node' },
          { label: '粘贴节点链接', value: 'link' },
        ]"
      />
      <el-form label-position="top" @submit.prevent="run">
        <el-form-item v-if="mode === 'node'" label="需要检查的节点"
          ><el-select
            v-model="selected"
            filterable
            placeholder="搜索 Reality / HY2 节点"
            style="width: 100%"
            ><el-option
              v-for="n in supported"
              :key="n.ID"
              :value="n.ID"
              :label="n.Name"
          /></el-select>
          <p v-if="listError" role="alert">
            {{ listError }} <UiButton @click="load">重试</UiButton>
          </p></el-form-item
        >
        <el-form-item v-else label="节点分享链接"
          ><el-input
            v-model="link"
            type="textarea"
            :rows="3"
            maxlength="16384"
            placeholder="vless://… 或 hy2://…"
          /><small
            >链接仅用于本次诊断，结果中不显示密码。请勿发送到公开场所。</small
          ></el-form-item
        >
        <el-form-item label="通过代理访问的目标"
          ><el-select v-model="target"
            ><el-option label="Cloudflare" value="cloudflare" /><el-option
              label="Google"
              value="google" /><el-option
              label="GitHub"
              value="github" /></el-select
        ></el-form-item>
        <div class="run-actions">
          <UiButton variant="primary" :loading="loading" @click="run">{{
            loading
              ? batchRunning
                ? "批量检测中…"
                : "正在逐步检查…"
              : "开始诊断"
          }}</UiButton>
          <UiButton :disabled="loading || !nodes.length" @click="runAll"
            >检测全部节点（{{ nodes.length }}）</UiButton
          >
          <UiButton v-if="loading" @click="controller?.abort()">取消</UiButton>
        </div>
      </el-form>
      <p class="context-note">
        支持 Reality TCP / Vision、XHTTP 等传输，以及 HY2。HY2 使用 UDP，TCP
        端口不通不代表 HY2 不可用。诊断约需 10–40 秒；首次准备内核可能需重试。
      </p>
      <p v-if="error" class="failure" role="alert">{{ error }}</p>
    </section>
    <section
      v-if="batchRows.length"
      class="diagnostic-result batch-result"
      aria-label="全部节点检测结果"
    >
      <div class="result-heading">
        <h2>
          {{
            batchRunning
              ? "正在检测全部节点"
              : batchCounts.cancelled
                ? "批量检测已取消"
                : "全部节点检测结果"
          }}
        </h2>
        <span>{{ batchDone }} / {{ batchRows.length }}</span>
      </div>
      <progress
        :value="batchDone"
        :max="batchRows.length"
        :aria-label="`已完成 ${batchDone} 个，共 ${batchRows.length} 个节点`"
      ></progress>
      <p aria-live="polite">
        通过 {{ batchCounts.passed }} · 失败 {{ batchCounts.failed }} · 待确认
        {{ batchCounts.unknown }} · 暂不支持 {{ batchCounts.skipped }} ·
        请求错误 {{ batchCounts.errors }} · 已取消 {{ batchCounts.cancelled }}
      </p>
      <p>
        每次最多同时检测两个节点；目标在开始时确定。取消或关闭页面会停止后续检测，已完成的结果保留。
      </p>
      <div class="batch-list">
        <div v-for="row in batchRows" :key="row.id" class="batch-row">
          <div class="batch-node">
            <strong>{{ row.name }}</strong>
            <p>{{ row.note || "等待安排检测" }}</p>
          </div>
          <span class="batch-status" :class="`batch-status-${row.status}`">{{
            batchStatusText[row.status]
          }}</span>
          <UiButton
            :disabled="!canOpenDiagnosticDetails(row)"
            @click="openDetails(row)"
            >{{
              row.status === "waiting"
                ? "等待检测"
                : row.status === "running"
                  ? "检测中…"
                  : "查看详情"
            }}</UiButton
          >
        </div>
      </div>
    </section>
    <section v-if="result" class="diagnostic-result" aria-live="polite">
      <div class="result-heading">
        <div>
          <h2>{{ result.summary }}</h2>
          <p>
            {{ result.protocol }} {{ result.transport }} ·
            {{ result.perspective }} ·
            {{ new Date(result.checkedAt).toLocaleString() }}
          </p>
        </div>
        <UiButton @click="copyReport()">复制诊断结果</UiButton>
      </div>
      <ol class="diagnostic-steps">
        <li
          v-for="step in result.steps"
          :key="step.key"
          class="diagnostic-step"
          :class="step.status"
        >
          <div class="step-status">{{ statusText[step.status] }}</div>
          <div>
            <h3>
              {{ step.title }} <small>{{ step.millis }} ms</small>
            </h3>
            <p>{{ step.detail }}</p>
          </div>
        </li>
      </ol>
      <aside v-if="result.advice.length">
        <h3>接下来检查</h3>
        <p v-for="advice in result.advice" :key="advice">{{ advice }}</p>
      </aside>
    </section>
    <UiDrawer
      v-model="detailVisible"
      :title="detailRow ? `${detailRow.name} · 诊断详情` : '诊断详情'"
      size="min(640px, 100vw)"
    >
      <section v-if="detailRow" class="detail-body">
        <p class="detail-status">{{ batchStatusText[detailRow.status] }}</p>
        <template v-if="detailResult">
          <h2>{{ detailResult.summary }}</h2>
          <p>
            {{ detailResult.protocol }} {{ detailResult.transport }} ·
            {{ detailResult.perspective }} ·
            {{ new Date(detailResult.checkedAt).toLocaleString() }}
          </p>
          <ol class="diagnostic-steps">
            <li
              v-for="step in detailResult.steps"
              :key="step.key"
              class="diagnostic-step"
              :class="step.status"
            >
              <div class="step-status">{{ statusText[step.status] }}</div>
              <div>
                <h3>
                  {{ step.title }} <small>{{ step.millis }} ms</small>
                </h3>
                <p>{{ step.detail }}</p>
              </div>
            </li>
          </ol>
          <aside v-if="detailResult.advice.length">
            <h3>接下来检查</h3>
            <p v-for="advice in detailResult.advice" :key="advice">
              {{ advice }}
            </p>
          </aside>
        </template>
        <template v-else>
          <h2>{{ detailRow.note || "本次没有生成完整的诊断报告" }}</h2>
          <p v-if="detailRow.status === 'error'">
            这是诊断请求失败，不代表节点已经损坏。请稍后重试，或单独选择这个节点检测。
          </p>
          <p v-else-if="detailRow.status === 'skipped'">
            这个节点尚未进行深度诊断，不能据此判定是否可用。
          </p>
          <p v-else-if="detailRow.status === 'cancelled'">
            检测已取消，没有足够结果判断节点是否可用。可以重新检测。
          </p>
        </template>
      </section>
      <template #footer
        ><UiButton v-if="detailResult" @click="copyReport(detailResult)"
          >复制诊断结果</UiButton
        ><UiButton @click="detailVisible = false">关闭</UiButton></template
      >
    </UiDrawer>
    <p class="context-note">
      面板可连通而手机无法访问时，还需检查手机网络、客户端参数和本地
      DNS。网站返回 403 / 429
      等状态不等于节点无法连接。单次超时不能唯一确认密码、握手或出口故障。
    </p>
  </main>
</template>
<style scoped>
.diagnostic-page {
  max-width: 1000px;
  margin: 0 auto;
  padding: 24px;
  color: var(--c-ink);
}
header,
.result-heading {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  align-items: center;
  flex-wrap: wrap;
}
h1 {
  font-size: 25px;
  margin: 0 0 8px;
}
h2 {
  font-size: 19px;
  margin: 0 0 8px;
}
h3 {
  margin: 0;
  font-size: 14px;
}
p {
  line-height: 1.7;
  margin: 6px 0;
  color: var(--c-muted);
  font-size: 13px;
}
.perspective {
  color: var(--c-muted);
  font-size: 12px;
}
.diagnostic-form,
.diagnostic-result {
  margin-top: 22px;
  padding: 22px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: 16px;
}
.diagnostic-form :deep(.el-form) {
  margin-top: 20px;
}
.diagnostic-form :deep(.el-select) {
  width: 100%;
}
.run-actions {
  flex-wrap: wrap;
  display: flex;
  gap: 10px;
}
.context-note {
  margin-top: 16px;
}
small {
  font-size: 12px;
  color: var(--c-muted);
}
.failure {
  color: #b42318;
}
.diagnostic-steps {
  padding: 0;
  list-style: none;
  margin: 20px 0 0;
}
.diagnostic-step {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  gap: 14px;
  padding: 18px 0;
  border-top: 1px solid var(--c-line);
}
.diagnostic-step p {
  overflow-wrap: anywhere;
}
.step-status {
  font-size: 12px;
  font-weight: 600;
  color: #946200;
}
.passed .step-status {
  color: #15803d;
}
.failed .step-status {
  color: #b42318;
}
h3 small {
  margin-left: 8px;
  font-weight: 400;
}
aside {
  border-top: 1px solid var(--c-line);
  padding-top: 18px;
}
@media (max-width: 600px) {
  .diagnostic-page {
    padding: 14px;
  }
  .diagnostic-form,
  .diagnostic-result {
    padding: 16px;
  }
  .run-actions :deep(button) {
    min-height: 44px;
  }
  .diagnostic-step {
    grid-template-columns: 48px minmax(0, 1fr);
    gap: 8px;
  }
}
</style>

<style scoped>
.detail-body {
  min-width: 0;
  overflow-wrap: anywhere;
}
.detail-status {
  font-weight: 600;
  color: var(--c-ink);
}
.source-switch {
  display: inline-grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  box-sizing: border-box;
  overflow: hidden;
}
.source-switch :deep(button) {
  min-width: 0;
  white-space: normal;
}
.batch-result progress {
  width: 100%;
  height: 10px;
  accent-color: var(--c-signal-text);
  margin: 14px 0 4px;
}
.batch-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 84px auto;
  gap: 14px;
  align-items: center;
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
}
.batch-node {
  min-width: 0;
}
.batch-node strong,
.batch-node p {
  overflow-wrap: anywhere;
}
.batch-status {
  font-size: 12px;
  color: var(--c-muted);
}
.batch-status-passed {
  color: #15803d;
}
.batch-status-failed,
.batch-status-error {
  color: #b42318;
}
.batch-status-unknown {
  color: #946200;
}
@media (max-width: 600px) {
  .batch-row {
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 8px;
  }
  .batch-node {
    grid-column: 1/-1;
  }
  .batch-row :deep(button) {
    min-height: 44px;
  }
}
</style>
