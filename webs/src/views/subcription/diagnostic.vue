<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from "vue";
import { useRoute } from "vue-router";
import gsap from "gsap";
import request from "@/utils/request";
import {
  runDiagnosticQueue,
  diagnosticVerdict,
  canOpenDiagnosticDetails,
} from "./diagnosticBatch";
import { getNodes, getNodeOverview } from "@/api/subcription/node";
import {
  diagnoseNodeFromUserAgent,
  type LocalDiagnoseResponse,
} from "@/api/subcription/probe";
import { countryFlag } from "@/utils/flag";
import { UiButton, UiDrawer } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import {
  Check,
  Close,
  CopyDocument,
  QuestionFilled,
  VideoPlay,
} from "@element-plus/icons-vue";

defineOptions({ name: "SubcriptionDiagnostic" });

const route = useRoute();
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const mode = ref<"node" | "link">("node");
const selected = ref<number>();
const link = ref("");
const target = ref("cloudflare");
const nodes = ref<{ ID: number; Name: string; Link: string }[]>([]);
const countryMap = ref<Map<number, string>>(new Map());
const hasFlagEmoji = (s: string) => /\p{RI}{2}/u.test(s);
function displayNodeName(id: number, name: string) {
  if (!name || hasFlagEmoji(name)) return name;
  const code = countryMap.value.get(id);
  if (!code) return name;
  const flag = countryFlag(code);
  return flag ? `${flag} ${name}` : name;
}
const loading = ref(false);
const listError = ref("");
const error = ref("");
const showRawLogs = ref(false);
const scanStep = ref(0);
let scanTimer: ReturnType<typeof setInterval> | null = null;

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
  route?: { name: string; protocol: string; transport: string }[];
}

const result = ref<Result | null>(null);
const localDiag = ref<LocalDiagnoseResponse | null>(null);
const localLoading = ref(false);
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

const targetOptions = [
  { label: "Cloudflare", value: "cloudflare" },
  { label: "Google", value: "google" },
  { label: "GitHub", value: "github" },
];

const defaultHops = [
  { key: "config", label: "面板出发" },
  { key: "dns", label: "DNS 解析" },
  { key: "transport", label: "入口连通" },
  { key: "tunnel", label: "协议握手" },
  { key: "https", label: "目标出口" },
];

interface VisualHop {
  key: string;
  label: string;
  status: "idle" | "scanning" | "passed" | "failed" | "unknown";
  millis?: number;
  detail?: string;
}

function shortHopLabel(step: Step, idx: number): string {
  const t = step.title || "";
  if (/内核|Xray/i.test(t) || step.key === "kernel") return "内核就绪";
  if (/配置|解析参数/.test(t) || step.key === "config" || step.key === "parse")
    return "节点配置";
  if (/DNS/i.test(t) || step.key === "dns") return "DNS 解析";
  if (/可达|入口|TCP|UDP/i.test(t) || step.key === "transport") return "入口连通";
  if (/握手|认证|出口/i.test(t) || step.key === "tunnel") return "协议握手";
  if (/HTTPS|目标/i.test(t) || step.key === "https") return "目标出口";
  return defaultHops[idx]?.label || t.slice(0, 6);
}

function buildVisualHops(r: Result | null | undefined, scanning: boolean): VisualHop[] {
  if (scanning) {
    return defaultHops.map((h, i) => ({
      key: h.key,
      label: h.label,
      status:
        i < scanStep.value
          ? "passed"
          : i === scanStep.value
            ? "scanning"
            : "idle",
    }));
  }
  if (!r || !r.steps?.length) {
    return defaultHops.map((h) => ({
      key: h.key,
      label: h.label,
      status: "idle",
    }));
  }
  return r.steps.map((s, idx) => ({
    key: s.key || `step-${idx}`,
    label: shortHopLabel(s, idx),
    status:
      s.status === "passed"
        ? "passed"
        : s.status === "failed"
          ? "failed"
          : "unknown",
    millis: s.millis,
    detail: s.detail,
  }));
}

const activeHops = computed(() =>
  buildVisualHops(result.value, loading.value && !batchRunning.value)
);

const drawerHops = computed(() => buildVisualHops(detailResult.value, false));

const activeVerdict = computed(() => {
  if (loading.value && !batchRunning.value) {
    return { state: "scanning", text: "正在从面板逐跳探测链路…" };
  }
  if (!result.value) {
    return { state: "idle", text: "选择节点后点击开始诊断" };
  }
  const v = diagnosticVerdict(result.value);
  const firstFailed = activeHops.value.find((h) => h.status === "failed");
  const firstUnknown = activeHops.value.find((h) => h.status === "unknown");
  const firstBad = firstFailed || firstUnknown;
  if (v === "passed") {
    return { state: "passed", text: "全链路畅通" };
  }
  if (v === "failed") {
    return {
      state: "failed",
      text: firstBad ? `故障点：${firstBad.label}` : "链路异常",
    };
  }
  return {
    state: "unknown",
    text: firstBad ? `卡顿 / 待确认：${firstBad.label}` : "链路待确认",
  };
});

const seg2Status = computed<"scanning" | "passed" | "failed" | "unknown" | "idle">(() => {
  if (loading.value && !batchRunning.value) return "scanning";
  if (!result.value) return "idle";
  const v = diagnosticVerdict(result.value);
  return v === "passed" ? "passed" : v === "failed" ? "failed" : "unknown";
});

const twoSegmentVerdict = computed(() => {
  const ld = localDiag.value;
  if (localLoading.value || (loading.value && !batchRunning.value)) {
    return {
      tone: "scanning",
      title: "正在并发执行两段式诊断…",
      detail: "段 1 由本机用户 Agent 实测直连节点入口；段 2 由面板验证节点协议握手与目标出口。",
    };
  }
  if (!ld || !ld.available) return null;

  const seg1Ok = !!ld.success && !ld.ipv6Mismatch;
  const seg2Ok = seg2Status.value === "passed";
  const seg2Fail = seg2Status.value === "failed";

  if (ld.ipv6Mismatch) {
    return {
      tone: "bad",
      title: "本机无 IPv6 出口，无法直连该纯 IPv6 节点",
      detail: seg2Ok
        ? "面板侧验证节点本身完全正常；问题出在你当前宽带未分配公网 IPv6，建议改用 IPv4 节点或开启路由器 IPv6。"
        : "本机缺少公网 IPv6 地址，且面板侧探测亦未通过。",
    };
  }
  if (seg1Ok && seg2Ok) {
    return {
      tone: "ok",
      title: "全链路双段通畅",
      detail: `你的电脑（${ld.probeName || "本机"}）到节点入口 ${ld.rtt || 0} ms（${(ld.kind || "tcp").toUpperCase()}）直连正常，且节点出口访问 ${target.value.toUpperCase()} 畅通。`,
    };
  }
  if (!seg1Ok && seg2Ok) {
    return {
      tone: "warn",
      title: "故障定位：本地宽带去程被阻断或墙拦截",
      detail: `节点服务器本身完全正常（面板实测出口畅通），但你的电脑无法直连 ${ld.host}:${ld.port}${ld.error ? `（${ld.error}）` : ""}。建议更换节点协议/端口或改走中转线路。`,
    };
  }
  if (seg1Ok && seg2Fail) {
    return {
      tone: "warn",
      title: "故障定位：节点协议配置或落地出口异常",
      detail: `你的电脑可以直连节点入口（${ld.rtt || 0} ms），但面板协议握手或目标站访问失败，说明服务端入站配置、密钥或落地机出口存在问题。`,
    };
  }
  if (!seg1Ok && seg2Fail) {
    return {
      tone: "bad",
      title: "故障定位：节点服务器疑似宕机或端口未监听",
      detail: `本机用户 Agent 直连与面板探测均无法到达 ${ld.host}:${ld.port}，请检查服务器运行状态、防火墙端口或内核进程。`,
    };
  }
  return {
    tone: seg1Ok ? "ok" : "warn",
    title: seg1Ok ? "本机去程可达，面板出口待确认" : "本机去程受阻，面板出口待确认",
    detail: ld.error || `本机到节点耗时 ${ld.rtt || 0} ms`,
  };
});

function totalMillis(r: Result | null | undefined): number {
  if (!r?.steps?.length) return 0;
  return r.steps.reduce((acc, s) => acc + (s.millis || 0), 0);
}

function startScanAnimation() {
  stopScanAnimation();
  scanStep.value = 0;
  scanTimer = setInterval(() => {
    scanStep.value = (scanStep.value + 1) % defaultHops.length;
  }, 1100);
}

function stopScanAnimation() {
  if (scanTimer) {
    clearInterval(scanTimer);
    scanTimer = null;
  }
}

function openDetails(row: BatchRow) {
  if (!canOpenDiagnosticDetails(row)) return;
  detailRow.value = { ...row };
  detailVisible.value = true;
}

const batchStatusText: Record<BatchStatus, string> = {
  waiting: "等待中",
  running: "探测中",
  passed: "畅通",
  failed: "故障",
  unknown: "待确认",
  skipped: "暂不支持",
  cancelled: "已取消",
  error: "异常",
};

const batchDone = computed(
  () =>
    batchRows.value.filter((row) => !["waiting", "running"].includes(row.status))
      .length
);

const batchCounts = computed(() => ({
  passed: batchRows.value.filter((r) => r.status === "passed").length,
  failed: batchRows.value.filter((r) => r.status === "failed").length,
  unknown: batchRows.value.filter((r) => r.status === "unknown").length,
  skipped: batchRows.value.filter((r) => r.status === "skipped").length,
  errors: batchRows.value.filter((r) => r.status === "error").length,
  cancelled: batchRows.value.filter((r) => r.status === "cancelled").length,
}));

const supported = computed(() =>
  nodes.value.filter((n) => /^(vless|hy2|hysteria2):\/\//i.test(n.Link))
);

async function load() {
  listError.value = "";
  try {
    const [nodeRes, overviewRes] = await Promise.allSettled([
      getNodes(true),
      getNodeOverview(),
    ]);
    if (nodeRes.status !== "fulfilled") throw nodeRes.reason;
    nodes.value = Array.isArray(nodeRes.value.data) ? nodeRes.value.data : [];
    if (overviewRes.status === "fulfilled" && Array.isArray(overviewRes.value.data)) {
      const m = new Map<number, string>();
      for (const item of overviewRes.value.data) {
        if (item?.id && item?.countryCode) m.set(item.id, item.countryCode);
      }
      countryMap.value = m;
    }
    if (!selected.value && supported.value.length) {
      selected.value = supported.value[0].ID;
    }
  } catch {
    listError.value = "节点列表加载失败";
  }
}

async function run() {
  if (loading.value) return;
  error.value = "";
  result.value = null;
  localDiag.value = null;
  batchRows.value = [];
  showRawLogs.value = false;
  if (mode.value === "node" ? !selected.value : !link.value.trim()) {
    error.value = "请选择节点或输入 Reality / HY2 链接";
    return;
  }
  loading.value = true;
  startScanAnimation();
  controller = new AbortController();

  // Segment 1: fire User Agent local reachability check in parallel for saved nodes
  let localPromise: Promise<void> = Promise.resolve();
  if (mode.value === "node" && selected.value) {
    const nodeId = selected.value;
    localLoading.value = true;
    localPromise = diagnoseNodeFromUserAgent({ nodeId })
      .then((res) => {
        localDiag.value = res.data || null;
      })
      .catch(() => {
        localDiag.value = null;
      })
      .finally(() => {
        localLoading.value = false;
      });
  }

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
        ? "已取消诊断"
        : e?.message || "诊断请求失败";
  } finally {
    await localPromise;
    stopScanAnimation();
    loading.value = false;
    controller = undefined;
  }
}

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
    name: displayNodeName(n.ID, n.Name),
    status: "waiting",
    note: "",
  }));
  const links = new Map(nodes.value.map((n) => [n.ID, n.Link]));
  const queue = batchRows.value.filter((row) => {
    const uri = links.get(row.id) || "";
    let ok = false;
    try {
      const u = new URL(uri);
      ok =
        ["hy2:", "hysteria2:"].includes(u.protocol) ||
        (u.protocol === "vless:" &&
          u.searchParams.get("security") === "reality");
    } catch {}
    if (!ok) {
      row.status = "skipped";
      row.note = "协议暂不支持";
    }
    return ok;
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
          row.note = "链路探测中…";
        },
        onResult(row, data) {
          row.result = data;
          row.status = diagnosticVerdict(data);
          row.note = data.summary;
        },
        onError(row, e: any) {
          row.status = "error";
          row.note = e?.message || "请求失败";
        },
        onCancel(row) {
          row.status = "cancelled";
          row.note = "已取消";
        },
      }
    );
  } finally {
    loading.value = false;
    batchRunning.value = false;
    if (controller === abort) controller = undefined;
  }
}

const statusText: Record<string, string> = {
  passed: "通过",
  failed: "故障",
  unknown: "待确认",
};

function copyReport(report?: Result) {
  const r = report || result.value;
  if (!r) return;
  navigator.clipboard
    .writeText(
      [
        r.summary,
        `${r.protocol} / ${r.transport}`,
        ...r.steps.map(
          (s) => `${s.title}：${statusText[s.status] || s.status} (${s.millis} ms) · ${s.detail}`
        ),
      ].join("\n")
    )
    .then(() => ElMessage.success("已复制诊断摘要"))
    .catch(() => ElMessage.error("复制失败"));
}

onMounted(async () => {
  await load();
  const id = Number(route.query.id);
  if (Number.isInteger(id) && nodes.value.some((n) => n.ID === id)) {
    selected.value = id;
  }
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  stopScanAnimation();
  controller?.abort();
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <main ref="rootEl" class="diag-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="diag-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>节点一键诊断</h1>
        </div>
      </div>

      <div class="masthead-actions">
        <UiButton
          v-if="loading"
          variant="ghost"
          @click="controller?.abort()"
        >
          取消检测
        </UiButton>
        <UiButton
          variant="ghost"
          :disabled="loading || !nodes.length"
          @click="runAll"
        >
          全部检测 ({{ nodes.length }})
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="loading && !batchRunning"
          @click="run"
        >
          {{ loading && !batchRunning ? "探测中…" : "开始诊断" }}
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Compact Single-Strip Control Bar -->
    <section class="control-strip">
      <div class="pill-group" role="group" aria-label="节点来源">
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': mode === 'node' }"
          @click="mode = 'node'"
        >
          已有节点
        </button>
        <button
          type="button"
          class="seg-pill"
          :class="{ 'is-active': mode === 'link' }"
          @click="mode = 'link'"
        >
          粘贴链接
        </button>
      </div>

      <div class="source-field">
        <el-select
          v-if="mode === 'node'"
          v-model="selected"
          filterable
          placeholder="选择 Reality / HY2 节点"
          style="width: 100%"
        >
          <el-option
            v-for="n in supported"
            :key="n.ID"
            :value="n.ID"
            :label="displayNodeName(n.ID, n.Name)"
          />
        </el-select>
        <el-input
          v-else
          v-model="link"
          placeholder="粘贴 vless:// 或 hy2:// 节点链接"
          clearable
        />
      </div>

      <div class="pill-group" role="group" aria-label="探测目标">
        <button
          v-for="opt in targetOptions"
          :key="opt.value"
          type="button"
          class="seg-pill"
          :class="{ 'is-active': target === opt.value }"
          @click="target = opt.value"
        >
          {{ opt.label }}
        </button>
      </div>
    </section>

    <KineticNotice :raw="error || listError" />

    <!-- Two-Segment User Agent + Panel Root-Cause Pipeline -->
    <section
      v-if="!batchRows.length && mode === 'node' && (localLoading || (localDiag && localDiag.available))"
      class="two-seg-card"
      :class="`tone-${twoSegmentVerdict?.tone || 'scanning'}`"
      aria-live="polite"
    >
      <header class="two-seg-head">
        <div class="two-seg-title">
          <span class="seg-dot" aria-hidden="true"></span>
          <strong>两段式双视角定位（用户 Agent + 面板）</strong>
        </div>
        <span v-if="localDiag?.probeName" class="two-seg-probe mono">
          {{ localDiag.probeName }}{{ localDiag.probeIsp ? ` · ${localDiag.probeIsp}` : "" }}
        </span>
      </header>

      <div class="three-node-rail">
        <div class="seg-node">
          <span class="seg-kicker">段 1 起点</span>
          <strong>你的电脑</strong>
          <small class="mono">{{ localDiag?.probeIsp || "本机物理宽带" }}</small>
        </div>

        <div
          class="seg-link"
          :class="
            localLoading
              ? 'is-scanning'
              : localDiag?.success && !localDiag?.ipv6Mismatch
                ? 'is-ok'
                : 'is-bad'
          "
        >
          <span class="seg-line" aria-hidden="true"></span>
          <span class="seg-badge mono">
            {{
              localLoading
                ? "直连实测中…"
                : localDiag?.ipv6Mismatch
                  ? "本机无 IPv6"
                  : localDiag?.success
                    ? `${localDiag.rtt || 0} ms · ${(localDiag.kind || "tcp").toUpperCase()}`
                    : "本机不可达"
            }}
          </span>
        </div>

        <div class="seg-node">
          <span class="seg-kicker">中继入口</span>
          <strong>节点服务器</strong>
          <small class="mono">
            {{ localDiag?.host ? `${localDiag.host}:${localDiag.port || ""}` : "目标节点" }}
          </small>
        </div>

        <div
          class="seg-link"
          :class="
            seg2Status === 'scanning'
              ? 'is-scanning'
              : seg2Status === 'passed'
                ? 'is-ok'
                : seg2Status === 'failed'
                  ? 'is-bad'
                  : 'is-warn'
          "
        >
          <span class="seg-line" aria-hidden="true"></span>
          <span class="seg-badge mono">
            {{
              seg2Status === "scanning"
                ? "出口验证中…"
                : seg2Status === "passed"
                  ? `出口畅通 · ${totalMillis(result)} ms`
                  : seg2Status === "failed"
                    ? "握手/出口失败"
                    : "待确认"
            }}
          </span>
        </div>

        <div class="seg-node">
          <span class="seg-kicker">段 2 终点</span>
          <strong>{{ target.toUpperCase() }}</strong>
          <small>目标站点</small>
        </div>
      </div>

      <div v-if="twoSegmentVerdict" class="root-cause-box">
        <strong>{{ twoSegmentVerdict.title }}</strong>
        <p>{{ twoSegmentVerdict.detail }}</p>
      </div>
    </section>

    <!-- Visual Animated Link Pipeline Stage -->
    <section
      v-if="!batchRows.length"
      class="pipeline-stage"
      :class="`is-${activeVerdict.state}`"
      aria-live="polite"
    >
      <div class="stage-head">
        <span class="verdict-pill" :class="`is-${activeVerdict.state}`">
          <i class="verdict-dot" aria-hidden="true"></i>
          <span>{{ activeVerdict.text }}</span>
        </span>

        <div v-if="result" class="stage-meta">
          <span class="meta-tag mono">
            {{ result.protocol.toUpperCase() }} · {{ result.transport.toUpperCase() }}
          </span>
          <span class="meta-tag mono">{{ totalMillis(result) }} ms</span>
          <button type="button" class="icon-text-btn" @click="copyReport()">
            <el-icon><CopyDocument /></el-icon>
            <span>复制</span>
          </button>
        </div>
      </div>

      <!-- Kinetic Hop-by-Hop Visual Topology -->
      <div class="hop-track" :class="{ 'is-scanning': loading && !batchRunning }">
        <div
          v-for="(hop, idx) in activeHops"
          :key="hop.key"
          class="hop-item"
          :class="`is-${hop.status}`"
        >
          <!-- Animated Beam Connector to Next Hop -->
          <div
            v-if="idx < activeHops.length - 1"
            class="hop-beam"
            :class="`is-${hop.status}`"
            aria-hidden="true"
          >
            <span class="beam-fill"></span>
            <span
              v-if="loading && !batchRunning"
              class="beam-pulse"
              :style="{ animationDelay: `${idx * 0.22}s` }"
            ></span>
          </div>

          <!-- Node Circle -->
          <div class="hop-node">
            <span v-if="hop.status === 'scanning' || hop.status === 'failed' || hop.status === 'unknown'" class="hop-halo" aria-hidden="true"></span>
            <el-icon v-if="hop.status === 'passed'"><Check /></el-icon>
            <el-icon v-else-if="hop.status === 'failed'"><Close /></el-icon>
            <el-icon v-else-if="hop.status === 'unknown'"><QuestionFilled /></el-icon>
            <span v-else class="hop-index mono">{{ idx + 1 }}</span>
          </div>

          <!-- Minimal Label + Latency -->
          <div class="hop-caption">
            <strong>{{ hop.label }}</strong>
            <small v-if="hop.millis !== undefined" class="mono">
              {{ hop.millis }} ms
            </small>
            <small v-else-if="hop.status === 'scanning'" class="mono">探测中</small>
          </div>
        </div>
      </div>

      <!-- Collapsible Raw Details (Hidden by Default) -->
      <div v-if="result" class="stage-foot">
        <button
          type="button"
          class="raw-toggle"
          @click="showRawLogs = !showRawLogs"
        >
          {{ showRawLogs ? "收起原始明细" : "查看原始明细" }}
        </button>

        <div v-if="showRawLogs" class="raw-list">
          <div
            v-for="s in result.steps"
            :key="s.key"
            class="raw-row"
            :class="`is-${s.status}`"
          >
            <span class="raw-badge">{{ statusText[s.status] || s.status }}</span>
            <strong>{{ s.title }}</strong>
            <span class="raw-ms mono">{{ s.millis }} ms</span>
            <p>{{ s.detail }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- Batch Mode: Compact Progress + Mini Visual Pipeline per Node -->
    <section
      v-else
      class="batch-stage"
      aria-label="全部节点检测结果"
    >
      <div class="batch-head">
        <div class="batch-counts mono">
          <span>{{ batchDone }} / {{ batchRows.length }}</span>
          <em class="c-ok">畅通 {{ batchCounts.passed }}</em>
          <em class="c-bad">故障 {{ batchCounts.failed }}</em>
          <em class="c-warn">待确认 {{ batchCounts.unknown }}</em>
        </div>
        <div class="batch-bar" aria-hidden="true">
          <i :style="{ width: `${(batchDone / Math.max(1, batchRows.length)) * 100}%` }"></i>
        </div>
      </div>

      <div class="batch-list">
        <button
          v-for="row in batchRows"
          :key="row.id"
          type="button"
          class="batch-row"
          :disabled="!canOpenDiagnosticDetails(row)"
          @click="openDetails(row)"
        >
          <strong class="batch-name">{{ row.name }}</strong>

          <!-- Inline Mini Visual Pipeline Dots -->
          <div class="mini-dots" aria-hidden="true">
            <template v-if="row.result?.steps?.length">
              <i
                v-for="s in row.result.steps"
                :key="s.key"
                :class="`is-${s.status}`"
              ></i>
            </template>
            <span v-else-if="row.status === 'running'" class="mini-scan"></span>
          </div>

          <span class="batch-pill" :class="`is-${row.status}`">
            {{ batchStatusText[row.status] }}
          </span>
        </button>
      </div>
    </section>

    <!-- Batch Row Visual Pipeline Drawer -->
    <UiDrawer
      v-model="detailVisible"
      :title="detailRow ? detailRow.name : '链路诊断'"
      size="min(580px, 100vw)"
    >
      <div v-if="detailRow" class="drawer-stage">
        <div v-if="detailResult" class="hop-track is-compact">
          <div
            v-for="(hop, idx) in drawerHops"
            :key="hop.key"
            class="hop-item"
            :class="`is-${hop.status}`"
          >
            <div
              v-if="idx < drawerHops.length - 1"
              class="hop-beam"
              :class="`is-${hop.status}`"
              aria-hidden="true"
            >
              <span class="beam-fill"></span>
            </div>
            <div class="hop-node">
              <el-icon v-if="hop.status === 'passed'"><Check /></el-icon>
              <el-icon v-else-if="hop.status === 'failed'"><Close /></el-icon>
              <el-icon v-else-if="hop.status === 'unknown'"><QuestionFilled /></el-icon>
              <span v-else class="hop-index mono">{{ idx + 1 }}</span>
            </div>
            <div class="hop-caption">
              <strong>{{ hop.label }}</strong>
              <small v-if="hop.millis !== undefined" class="mono">{{ hop.millis }} ms</small>
            </div>
          </div>
        </div>
        <p v-else class="drawer-empty">{{ detailRow.note || "暂无链路报告" }}</p>
      </div>
      <template #footer>
        <UiButton v-if="detailResult" variant="ghost" @click="copyReport(detailResult)">
          复制摘要
        </UiButton>
        <UiButton variant="primary" @click="detailVisible = false">关闭</UiButton>
      </template>
    </UiDrawer>
  </main>
</template>

<style scoped>
.diag-gallery {
  display: grid;
  gap: 20px;
  max-width: 1120px;
  min-width: 0;
}

.diag-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

/* Control Strip */
.control-strip {
  display: grid;
  grid-template-columns: auto minmax(220px, 1fr) auto;
  align-items: center;
  gap: 14px;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
}

.seg-pill {
  padding: 6px 13px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.seg-pill:hover {
  color: var(--c-ink);
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.source-field {
  min-width: 0;
}

.inline-error {
  margin: 0;
  font-size: 13px;
  color: var(--c-bad);
}

/* Animated Link Pipeline Stage */
.pipeline-stage {
  display: grid;
  gap: 36px;
  padding: 28px 32px;
  border: 1px solid var(--c-line);
  border-radius: 22px;
  background: var(--c-surface);
}

.stage-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.verdict-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 14px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
}

.verdict-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
}

.verdict-pill.is-scanning .verdict-dot {
  background: var(--c-ok);
  animation: pulseDot 1.1s infinite ease-in-out;
}

.verdict-pill.is-passed {
  color: var(--c-ok);
}
.verdict-pill.is-passed .verdict-dot {
  background: var(--c-ok);
}

.verdict-pill.is-failed {
  color: var(--c-bad);
}
.verdict-pill.is-failed .verdict-dot {
  background: var(--c-bad);
}

.verdict-pill.is-unknown {
  color: var(--c-warn);
}
.verdict-pill.is-unknown .verdict-dot {
  background: var(--c-warn);
}

.stage-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.meta-tag {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.icon-text-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.icon-text-btn:hover {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

/* Horizontal Kinetic Hop Track */
.hop-track {
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: 1fr;
  align-items: start;
  padding: 18px 0 8px;
}

.hop-item {
  position: relative;
  display: grid;
  justify-items: center;
  gap: 14px;
  text-align: center;
}

.hop-beam {
  position: absolute;
  top: 22px;
  left: calc(50% + 24px);
  right: calc(-50% + 24px);
  height: 2px;
  background: var(--c-line);
  overflow: hidden;
  border-radius: 2px;
}

.beam-fill {
  display: block;
  width: 0;
  height: 100%;
  transition: width 420ms cubic-bezier(0.16, 1, 0.3, 1);
}

.hop-beam.is-passed .beam-fill {
  width: 100%;
  background: var(--c-ok);
}

.hop-beam.is-failed {
  background: repeating-linear-gradient(
    90deg,
    var(--c-bad) 0 6px,
    transparent 6px 12px
  );
  opacity: 0.55;
}

.hop-beam.is-unknown .beam-fill {
  width: 100%;
  background: var(--c-warn);
  opacity: 0.45;
}

.beam-pulse {
  position: absolute;
  top: -1px;
  left: -30%;
  width: 36%;
  height: 4px;
  border-radius: 999px;
  background: linear-gradient(90deg, transparent, var(--c-ok), transparent);
  animation: beamTravel 1.3s infinite linear;
}

.hop-node {
  position: relative;
  z-index: 2;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  border: 1.5px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 16px;
  transition:
    border-color 280ms var(--ease-out),
    background 280ms var(--ease-out),
    color 280ms var(--ease-out),
    transform 280ms var(--ease-out);
}

.hop-index {
  font-size: 13px;
  font-weight: 500;
}

.hop-item.is-scanning .hop-node {
  border-color: var(--c-ok);
  color: var(--c-ok);
  transform: scale(1.08);
}

.hop-item.is-passed .hop-node {
  border-color: var(--c-ok);
  background: var(--c-ok);
  color: #fff;
}

.hop-item.is-failed .hop-node {
  border-color: var(--c-bad);
  background: var(--c-bad);
  color: #fff;
  transform: scale(1.08);
}

.hop-item.is-unknown .hop-node {
  border-color: var(--c-warn);
  background: var(--c-warn);
  color: #fff;
}

.hop-halo {
  position: absolute;
  inset: -7px;
  border-radius: 50%;
  border: 2px solid currentColor;
  opacity: 0.35;
  animation: haloRipple 1.6s infinite cubic-bezier(0.16, 1, 0.3, 1);
}

.hop-item.is-failed .hop-halo {
  color: var(--c-bad);
}
.hop-item.is-unknown .hop-halo {
  color: var(--c-warn);
}
.hop-item.is-scanning .hop-halo {
  color: var(--c-ok);
}

.hop-caption {
  display: grid;
  gap: 3px;
}

.hop-caption strong {
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
}

.hop-caption small {
  font-size: 11.5px;
  color: var(--c-muted);
}

/* Optional Collapsible Raw Logs */
.stage-foot {
  display: grid;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--c-line);
}

.raw-toggle {
  justify-self: end;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.raw-toggle:hover {
  color: var(--c-ink);
}

.raw-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.raw-row {
  display: grid;
  grid-template-columns: 56px minmax(0, 1fr) auto;
  gap: 4px 12px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 12px;
}

.raw-row p {
  grid-column: 2 / -1;
  margin: 0;
  color: var(--c-muted);
}

.raw-badge {
  font-weight: 600;
  color: var(--c-warn);
}
.raw-row.is-passed .raw-badge {
  color: var(--c-ok);
}
.raw-row.is-failed .raw-badge {
  color: var(--c-bad);
}

.raw-ms {
  color: var(--c-muted);
}

/* Batch Stage */
.batch-stage {
  display: grid;
  gap: 16px;
}

.batch-head {
  display: grid;
  gap: 10px;
}

.batch-counts {
  display: flex;
  align-items: center;
  gap: 16px;
  font-size: 13px;
  color: var(--c-ink);
}

.batch-counts em {
  font-style: normal;
  font-size: 12px;
}

.c-ok {
  color: var(--c-ok);
}
.c-bad {
  color: var(--c-bad);
}
.c-warn {
  color: var(--c-warn);
}

.batch-bar {
  height: 3px;
  border-radius: 3px;
  background: var(--c-line);
  overflow: hidden;
}

.batch-bar i {
  display: block;
  height: 100%;
  background: var(--c-ink);
  transition: width 260ms var(--ease-out);
}

.batch-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.batch-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto 76px;
  align-items: center;
  gap: 16px;
  width: 100%;
  padding: 14px 6px;
  border: 0;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.batch-row:disabled {
  cursor: default;
  opacity: 0.7;
}

.batch-row:not(:disabled):hover {
  background: var(--c-surface);
}

.batch-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-dots {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.mini-dots i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-line);
}

.mini-dots i.is-passed {
  background: var(--c-ok);
}
.mini-dots i.is-failed {
  background: var(--c-bad);
}
.mini-dots i.is-unknown {
  background: var(--c-warn);
}

.mini-scan {
  width: 48px;
  height: 3px;
  border-radius: 3px;
  background: linear-gradient(90deg, transparent, var(--c-ok), transparent);
  animation: beamTravel 1s infinite linear;
}

.batch-pill {
  text-align: right;
  font-size: 12px;
  font-weight: 500;
  color: var(--c-muted);
}

.batch-pill.is-passed {
  color: var(--c-ok);
}
.batch-pill.is-failed,
.batch-pill.is-error {
  color: var(--c-bad);
}
.batch-pill.is-unknown {
  color: var(--c-warn);
}

.drawer-stage {
  padding: 12px 0;
}

.drawer-empty {
  color: var(--c-muted);
  font-size: 13px;
}

/* Two-Segment User Agent + Panel Root-Cause Card */
.two-seg-card {
  display: grid;
  gap: 16px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
}

.two-seg-card.tone-ok {
  border-color: color-mix(in srgb, var(--c-ok) 36%, var(--c-line));
}

.two-seg-card.tone-warn {
  border-color: color-mix(in srgb, var(--c-warn) 44%, var(--c-line));
}

.two-seg-card.tone-bad {
  border-color: color-mix(in srgb, var(--c-bad) 42%, var(--c-line));
}

.two-seg-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.two-seg-title {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13.5px;
  color: var(--c-ink);
}

.seg-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
}

.two-seg-card.tone-warn .seg-dot {
  background: var(--c-warn);
}

.two-seg-card.tone-bad .seg-dot {
  background: var(--c-bad);
}

.two-seg-card.tone-scanning .seg-dot {
  animation: pulseDot 1.2s ease-in-out infinite;
}

.two-seg-probe {
  font-size: 12px;
  color: var(--c-muted);
}

.three-node-rail {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(110px, 0.9fr) minmax(0, 1fr) minmax(110px, 0.9fr) minmax(0, 1fr);
  align-items: center;
  gap: 10px;
}

.seg-node {
  display: grid;
  gap: 3px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-muted);
  min-width: 0;
}

.seg-kicker {
  font-size: 10.5px;
  color: var(--c-muted);
}

.seg-node strong {
  font-size: 13.5px;
  color: var(--c-ink);
}

.seg-node small {
  font-size: 11px;
  color: var(--c-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.seg-link {
  position: relative;
  display: grid;
  place-items: center;
}

.seg-line {
  position: absolute;
  left: 0;
  right: 0;
  top: 50%;
  height: 2px;
  background: var(--c-line);
  transform: translateY(-50%);
}

.seg-link.is-ok .seg-line {
  background: color-mix(in srgb, var(--c-ok) 50%, transparent);
}

.seg-link.is-bad .seg-line {
  background: color-mix(in srgb, var(--c-bad) 50%, transparent);
}

.seg-badge {
  position: relative;
  z-index: 1;
  padding: 4px 10px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 11px;
  font-weight: 500;
  color: var(--c-muted);
  white-space: nowrap;
}

.seg-link.is-ok .seg-badge {
  border-color: color-mix(in srgb, var(--c-ok) 38%, transparent);
  color: var(--c-ok);
}

.seg-link.is-bad .seg-badge {
  border-color: color-mix(in srgb, var(--c-bad) 38%, transparent);
  color: var(--c-bad);
}

.seg-link.is-warn .seg-badge {
  border-color: color-mix(in srgb, var(--c-warn) 42%, transparent);
  color: var(--c-warn);
}

.root-cause-box {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 12px;
  background: var(--c-surface-muted);
  font-size: 13px;
}

.root-cause-box p {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-muted);
  line-height: 1.55;
}

@media (max-width: 820px) {
  .three-node-rail {
    grid-template-columns: 1fr;
  }
  .seg-line {
    display: none;
  }
}

@keyframes beamTravel {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(360%);
  }
}

@keyframes haloRipple {
  0% {
    transform: scale(0.92);
    opacity: 0.45;
  }
  100% {
    transform: scale(1.36);
    opacity: 0;
  }
}

@keyframes pulseDot {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.4;
    transform: scale(0.8);
  }
}

@media (prefers-reduced-motion: reduce) {
  .beam-pulse,
  .hop-halo,
  .verdict-dot,
  .mini-scan {
    animation: none;
  }
}

@media (max-width: 820px) {
  .control-strip {
    grid-template-columns: 1fr;
  }
  .pipeline-stage {
    padding: 20px 16px;
  }
  .hop-track {
    grid-auto-flow: row;
    gap: 18px;
  }
  .hop-item {
    grid-template-columns: 44px minmax(0, 1fr);
    justify-items: start;
    text-align: left;
    align-items: center;
  }
  .hop-beam {
    display: none;
  }
}
</style>
