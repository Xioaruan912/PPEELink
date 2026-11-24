<script setup lang="ts">
import {
  computed,
  nextTick,
  onActivated,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import gsap from "gsap";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  CopyDocument,
  Delete,
  Download,
  Key,
  Plus,
  Refresh,
  VideoPlay,
} from "@element-plus/icons-vue";
import { EmptyState, StatusDot, UiButton, UiDialog } from "@/ui";
import CountryFlag from "@/ui/CountryFlag.vue";
import RowIconButton from "@/ui/RowIconButton.vue";
import { displayNodeName } from "@/utils/nodeDisplay";
import LatencySparkline from "./components/LatencySparkline.vue";
import LatencyDetailModal, {
  type DetailNodeTarget,
} from "./components/LatencyDetailModal.vue";
import {
  addProbe,
  deleteProbe,
  getProbes,
  getProbeSparklines,
  resetProbeToken,
  toggleProbeAuto,
  triggerUserProbe,
  updateProbe,
  type Probe,
  type ProbeCommands,
  type SparklinesResponse,
} from "@/api/subcription/probe";
import { getNodeOverview } from "@/api/subcription/node";

defineOptions({ name: "UserAgent" });

interface NodeProbeItem {
  id: number;
  name: string;
  region?: string;
  kind?: string;
  success: boolean;
  rtt: number;
  error?: string;
  checkedAt?: string;
}

interface NodeCardItem {
  id: number;
  name: string;
  server: string;
  port: number;
  country: string;
  countryCode: string;
  valid: boolean;
  probes?: NodeProbeItem[];
  userValid?: boolean;
  userRtt?: number;
  userKind?: string;
  userError?: string;
  userCheckedAt?: string;
  userProbeName?: string;
}

interface ResolvedNodeView {
  checked: boolean;
  valid: boolean;
  rtt?: number;
  kind?: string;
  error?: string;
  probeName?: string;
}

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const loading = ref(false);
const probingAll = ref(false);
const probingNodeId = ref<number | null>(null);
const updatingProbeId = ref<number | null>(null);
const togglingAutoId = ref<number | null>(null);
const selectedProbeId = ref<number | null>(null);
const probes = ref<Probe[]>([]);
const nodes = ref<NodeCardItem[]>([]);
const sparklines = ref<SparklinesResponse | null>(null);
const detailOpen = ref(false);
const detailNode = ref<DetailNodeTarget | null>(null);
const query = ref("");
const statusFilter = ref<"all" | "ok" | "fail" | "untested">("all");
let autoRefreshTimer: ReturnType<typeof setInterval> | null = null;

const addDialogVisible = ref(false);
const creating = ref(false);
const newName = ref("");
const newRegion = ref("");

const setupDialogVisible = ref(false);
const setupTitle = ref("");
const setupToken = ref("");
const setupCommands = ref<ProbeCommands>({ windows: "", macos: "", linux: "" });
const setupOS = ref<"windows" | "macos" | "linux">("windows");

const onlineProbes = computed(() => probes.value.filter((p) => p.online));
const hasOnlineProbe = computed(() => onlineProbes.value.length > 0);
const selectedProbe = computed(
  () => probes.value.find((p) => p.id === selectedProbeId.value) || null
);

function resolveNodeView(n: NodeCardItem): ResolvedNodeView {
  if (selectedProbeId.value !== null) {
    const matched = n.probes?.find((p) => p.id === selectedProbeId.value);
    if (!matched) {
      return { checked: false, valid: false, kind: n.userKind };
    }
    return {
      checked: true,
      valid: matched.success,
      rtt: matched.rtt,
      kind: matched.kind || n.userKind,
      error: matched.error,
      probeName: matched.name,
    };
  }
  if (n.probes && n.probes.length > 0) {
    const reachable = n.probes.filter((p) => p.success && p.rtt > 0);
    const best = reachable.length
      ? reachable.reduce((a, b) => (b.rtt < a.rtt ? b : a))
      : n.probes[0];
    return {
      checked: true,
      valid: best.success,
      rtt: best.rtt,
      kind: best.kind || n.userKind,
      error: best.error,
      probeName: best.name,
    };
  }
  if (!n.userCheckedAt) {
    return { checked: false, valid: false, kind: n.userKind };
  }
  return {
    checked: true,
    valid: Boolean(n.userValid),
    rtt: n.userRtt,
    kind: n.userKind,
    error: n.userError,
    probeName: n.userProbeName,
  };
}

function toggleSelectProbe(id: number) {
  selectedProbeId.value = selectedProbeId.value === id ? null : id;
}

const summary = computed(() => {
  let ok = 0;
  let fail = 0;
  let untested = 0;
  for (const n of nodes.value) {
    const v = resolveNodeView(n);
    if (!v.checked) untested++;
    else if (v.valid) ok++;
    else fail++;
  }
  return { total: nodes.value.length, ok, fail, untested };
});

const filteredNodes = computed(() => {
  const q = query.value.trim().toLowerCase();
  return nodes.value.filter((n) => {
    const v = resolveNodeView(n);
    if (statusFilter.value === "ok" && !(v.checked && v.valid)) return false;
    if (statusFilter.value === "fail" && !(v.checked && !v.valid)) return false;
    if (statusFilter.value === "untested" && v.checked) return false;
    if (!q) return true;
    return `${n.name} ${n.server} ${n.country}`.toLowerCase().includes(q);
  });
});

function rttRatio(rtt?: number) {
  if (rtt === undefined || rtt <= 0) return 0;
  return Math.min(100, Math.max(10, Math.round((rtt / 320) * 100)));
}

function rttTone(rtt?: number) {
  if (rtt === undefined || rtt <= 0) return "is-ok";
  if (rtt <= 120) return "is-ok";
  if (rtt <= 240) return "is-warn";
  return "is-bad";
}

function formatShortTime(raw?: string | null) {
  if (!raw) return "—";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return "—";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function animateEntrance() {
  if (!rootEl.value) return;
  gsapCtx?.revert();
  gsapCtx = gsap.context(() => {
    const mm = gsap.matchMedia();
    mm.add("(prefers-reduced-motion: no-preference)", () => {
      const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
      tl.from("[data-title-reveal]", {
        yPercent: 108,
        duration: 0.56,
        clearProps: "transform",
      })
        .from(
          "[data-top-rule]",
          {
            scaleX: 0,
            transformOrigin: "left center",
            duration: 0.62,
            clearProps: "transform",
          },
          "-=0.4"
        )
        .from(
          "[data-stagger-card]",
          {
            y: 14,
            opacity: 0,
            duration: 0.42,
            stagger: 0.035,
            clearProps: "transform,opacity",
          },
          "-=0.35"
        );
    });
  }, rootEl.value);
}

async function fetchSparklines() {
  try {
    const res = await getProbeSparklines({
      hours: 24,
      probeId: selectedProbeId.value || undefined,
    });
    sparklines.value = res.data || null;
  } catch {
    // Non-blocking: cards still render current RTT cleanly.
  }
}

async function refreshAll() {
  loading.value = true;
  try {
    const [probeRes, nodeRes] = await Promise.all([
      getProbes(),
      getNodeOverview(false),
    ]);
    probes.value = probeRes.data || [];
    if (
      selectedProbeId.value !== null &&
      !probes.value.some((p) => p.id === selectedProbeId.value)
    ) {
      selectedProbeId.value = null;
    }
    nodes.value = (nodeRes.data || []) as NodeCardItem[];
    await fetchSparklines();
  } catch (err: any) {
    ElMessage.error(err?.message || "加载失败");
  } finally {
    loading.value = false;
  }
}

async function handleToggleAuto(probe: Probe) {
  togglingAutoId.value = probe.id;
  const nextVal = !probe.autoProbe;
  try {
    const res = (await toggleProbeAuto(probe.id, nextVal)) as any;
    probe.autoProbe = nextVal;
    ElMessage.success(res?.msg || (nextVal ? "已开启 1 分钟定时记录" : "已关闭定时记录"));
    if (nextVal && probe.online) {
      setTimeout(() => {
        void refreshAll();
      }, 3000);
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "切换失败");
  } finally {
    togglingAutoId.value = null;
  }
}

function openNodeDetail(item: NodeCardItem) {
  const v = resolveNodeView(item);
  detailNode.value = {
    id: item.id,
    name: item.name,
    server: item.server,
    port: item.port,
    country: item.country,
    countryCode: item.countryCode,
    kind: v.kind,
  };
  detailOpen.value = true;
}

function nodeSparkline(nodeId: number) {
  return sparklines.value?.nodes?.[String(nodeId)] || null;
}

async function copyText(text: string, label = "已复制") {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text);
    } else {
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      document.execCommand("copy");
      document.body.removeChild(ta);
    }
    ElMessage.success(label);
  } catch {
    ElMessage.warning("请手动选中复制");
  }
}

async function submitAddProbe() {
  if (!newName.value.trim()) {
    ElMessage.warning("请填写名称");
    return;
  }
  creating.value = true;
  try {
    const { data } = await addProbe({
      name: newName.value.trim(),
      region: newRegion.value.trim(),
    });
    addDialogVisible.value = false;
    newName.value = "";
    newRegion.value = "";
    setupTitle.value = data.name;
    setupToken.value = data.token;
    setupCommands.value = data.commands || {
      windows: data.install,
      macos: data.install,
      linux: data.install,
    };
    setupOS.value = /win/i.test(navigator.platform)
      ? "windows"
      : /mac/i.test(navigator.platform)
        ? "macos"
        : "linux";
    setupDialogVisible.value = true;
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "创建失败");
  } finally {
    creating.value = false;
  }
}

async function handleResetToken(probe: Probe) {
  try {
    await ElMessageBox.confirm(`重置「${probe.name}」的令牌？`, "重置令牌", {
      type: "warning",
      confirmButtonText: "重置",
      cancelButtonText: "取消",
    });
    const { data } = await resetProbeToken(probe.id);
    setupTitle.value = probe.name;
    setupToken.value = data.token;
    setupCommands.value = data.commands || {
      windows: data.install,
      macos: data.install,
      linux: data.install,
    };
    setupDialogVisible.value = true;
    await refreshAll();
  } catch (err: any) {
    if (err?.message) ElMessage.error(err.message);
  }
}

async function handleUpdateProbe(probe: Probe) {
  if (!probe.online) {
    ElMessage.warning("设备离线，无法远程更新");
    return;
  }
  updatingProbeId.value = probe.id;
  try {
    const res = (await updateProbe(probe.id)) as any;
    ElMessage.success(res?.msg || "已下发更新");
    setTimeout(() => {
      void refreshAll();
    }, 3500);
  } catch (err: any) {
    ElMessage.error(err?.message || "更新失败");
  } finally {
    updatingProbeId.value = null;
  }
}

async function handleDeleteProbe(probe: Probe) {
  try {
    const tip = probe.online
      ? `将通知本机「${probe.name}」自动退出并无痕删除所有本地文件与开机自启项，同时从面板移除。`
      : `删除「${probe.name}」？（当前离线，仅从面板移除）`;
    await ElMessageBox.confirm(tip, "无痕卸载并删除", {
      type: "warning",
      confirmButtonText: probe.online ? "无痕卸载并删除" : "删除",
      cancelButtonText: "取消",
    });
    const res = (await deleteProbe(probe.id)) as any;
    ElMessage.success(res?.msg || "已删除");
    await refreshAll();
  } catch (err: any) {
    if (err?.message) ElMessage.error(err.message);
  }
}

async function runTestAll(probeId?: number) {
  if (!hasOnlineProbe.value) {
    ElMessage.warning("请先在本机启动用户 Agent");
    return;
  }
  const targetId = probeId ?? selectedProbeId.value ?? undefined;
  if (targetId && !probes.value.find((p) => p.id === targetId)?.online) {
    ElMessage.warning("当前选中设备离线，无法实测");
    return;
  }
  probingAll.value = true;
  try {
    if (targetId) {
      const res = (await triggerUserProbe({ probeId: targetId })) as any;
      ElMessage.success(res?.msg || "实测完成");
    } else if (onlineProbes.value.length > 1) {
      const settled = await Promise.allSettled(
        onlineProbes.value.map((p) => triggerUserProbe({ probeId: p.id }))
      );
      const okCount = settled.filter((r) => r.status === "fulfilled").length;
      ElMessage.success(`已完成 ${okCount}/${onlineProbes.value.length} 台设备实测`);
    } else {
      const res = (await triggerUserProbe({})) as any;
      ElMessage.success(res?.msg || "实测完成");
    }
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "实测失败");
  } finally {
    probingAll.value = false;
  }
}

async function runTestOne(node: NodeCardItem) {
  if (!hasOnlineProbe.value) {
    ElMessage.warning("请先在本机启动用户 Agent");
    return;
  }
  const targetId = selectedProbeId.value ?? undefined;
  if (targetId && !probes.value.find((p) => p.id === targetId)?.online) {
    ElMessage.warning("当前选中设备离线，无法实测");
    return;
  }
  probingNodeId.value = node.id;
  try {
    if (targetId) {
      const res = (await triggerUserProbe({
        probeId: targetId,
        nodeIds: [node.id],
      })) as any;
      ElMessage.success(res?.msg || "实测完成");
    } else if (onlineProbes.value.length > 1) {
      const settled = await Promise.allSettled(
        onlineProbes.value.map((p) =>
          triggerUserProbe({ probeId: p.id, nodeIds: [node.id] })
        )
      );
      const okCount = settled.filter((r) => r.status === "fulfilled").length;
      ElMessage.success(`已完成 ${okCount}/${onlineProbes.value.length} 台设备实测`);
    } else {
      const res = (await triggerUserProbe({ nodeIds: [node.id] })) as any;
      ElMessage.success(res?.msg || "实测完成");
    }
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "实测失败");
  } finally {
    probingNodeId.value = null;
  }
}

watch(selectedProbeId, () => {
  void fetchSparklines();
});

onMounted(async () => {
  await refreshAll();
  await nextTick();
  animateEntrance();
  autoRefreshTimer = setInterval(() => {
    if (probes.value.some((p) => p.autoProbe && p.online)) {
      void refreshAll();
    }
  }, 60000);
});

onActivated(refreshAll);

onBeforeUnmount(() => {
  if (autoRefreshTimer) {
    clearInterval(autoRefreshTimer);
    autoRefreshTimer = null;
  }
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="ua-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="ua-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>用户 Agent</h1>
        </div>
        <span
          class="online-pill mono"
          :class="{ 'is-online': hasOnlineProbe }"
        >
          <i aria-hidden="true"></i>
          {{ hasOnlineProbe ? `${onlineProbes.length} 在线` : "未连接" }}
        </span>
      </div>

      <div class="masthead-actions">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          aria-label="刷新"
          @click="refreshAll"
        />
        <UiButton variant="ghost" :icon="Plus" @click="addDialogVisible = true">
          添加设备
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="probingAll"
          :disabled="!hasOnlineProbe || !nodes.length"
          @click="runTestAll()"
        >
          {{
            selectedProbe
              ? `实测 · ${selectedProbe.name}`
              : onlineProbes.length > 1
                ? "全部设备实测"
                : "本机实测"
          }}
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Section 1: 本机设备卡片 -->
    <section class="ua-section" aria-label="本机设备">
      <div v-if="probes.length" class="device-grid">
        <article
          v-for="p in probes"
          :key="p.id"
          class="device-card"
          :class="{
            'is-online': p.online,
            'is-selected': selectedProbeId === p.id,
            'is-clickable': probes.length > 1,
          }"
          data-stagger-card
          @click="probes.length > 1 && toggleSelectProbe(p.id)"
        >
          <div class="device-head">
            <StatusDot
              :state="p.online ? 'ok' : 'idle'"
              :label="p.online ? '在线' : '离线'"
            />
            <span v-if="p.region" class="device-region">{{ p.region }}</span>
            <span v-if="p.version" class="device-ver mono">v{{ p.version }}</span>
            <span class="device-spacer"></span>
            <span
              v-if="selectedProbeId === p.id"
              class="device-active-tag"
            >
              当前视角
            </span>
            <span v-else class="device-time mono">{{
              formatShortTime(p.lastSeenAt)
            }}</span>
          </div>

          <div class="device-title-row">
            <strong class="device-name">{{ p.name }}</strong>
            <button
              type="button"
              class="auto-pill"
              :class="{ 'is-on': p.autoProbe }"
              :disabled="togglingAutoId === p.id"
              :title="
                p.autoProbe
                  ? '已开启 1 分钟定时记录（保留 7 天曲线），点击关闭'
                  : '开启 1 分钟定时记录，自动生成 7 天高峰/低峰曲线'
              "
              @click.stop="handleToggleAuto(p)"
            >
              <span class="auto-dot" aria-hidden="true"></span>
              <span>{{ p.autoProbe ? "1m 记录中" : "1m 曲线" }}</span>
            </button>
          </div>

          <footer class="device-foot" @click.stop>
            <UiButton
              size="sm"
              variant="secondary"
              :disabled="!p.online || probingAll"
              :loading="probingAll"
              @click="runTestAll(p.id)"
            >
              实测全部
            </UiButton>
            <div class="device-icons">
              <RowIconButton
                v-if="p.online"
                :icon="Download"
                :label="
                  p.updateAvailable
                    ? `更新至 v${p.latestVersion}`
                    : '远程更新'
                "
                :disabled="updatingProbeId === p.id"
                @click="handleUpdateProbe(p)"
              />
              <RowIconButton
                :icon="Key"
                label="重置令牌"
                @click="handleResetToken(p)"
              />
              <RowIconButton
                :icon="Delete"
                :label="p.online ? '无痕卸载并删除' : '删除设备'"
                @click="handleDeleteProbe(p)"
              />
            </div>
          </footer>
        </article>
      </div>

      <div v-else class="device-empty" data-stagger-card>
        <p>在本机运行单文件客户端，按需实测真实延迟。</p>
        <UiButton variant="primary" :icon="Plus" @click="addDialogVisible = true">
          添加设备
        </UiButton>
      </div>
    </section>

    <!-- Section 2: 节点实测卡片网格 -->
    <section class="ua-section" aria-label="节点实测结果">
      <header class="nodes-bar">
        <div class="filter-pills" role="group" aria-label="状态筛选">
          <button
            type="button"
            class="filter-pill"
            :class="{ 'is-active': statusFilter === 'all' }"
            @click="statusFilter = 'all'"
          >
            全部 <span class="mono">{{ summary.total }}</span>
          </button>
          <button
            type="button"
            class="filter-pill"
            :class="{ 'is-active': statusFilter === 'ok' }"
            @click="statusFilter = 'ok'"
          >
            可达 <span class="mono">{{ summary.ok }}</span>
          </button>
          <button
            type="button"
            class="filter-pill"
            :class="{ 'is-active': statusFilter === 'fail' }"
            @click="statusFilter = 'fail'"
          >
            不可达 <span class="mono">{{ summary.fail }}</span>
          </button>
          <button
            type="button"
            class="filter-pill"
            :class="{ 'is-active': statusFilter === 'untested' }"
            @click="statusFilter = 'untested'"
          >
            未测 <span class="mono">{{ summary.untested }}</span>
          </button>

          <button
            v-if="selectedProbe"
            type="button"
            class="view-pill"
            title="点击返回全部设备对比"
            @click="selectedProbeId = null"
          >
            视角：{{ selectedProbe.name }}
            <span aria-hidden="true">×</span>
          </button>
        </div>

        <input
          v-model="query"
          type="search"
          class="search-input"
          aria-label="搜索节点"
          placeholder="搜索节点…"
        />
      </header>

      <div v-if="filteredNodes.length" class="node-card-grid">
        <article
          v-for="item in filteredNodes"
          :key="item.id"
          class="node-card is-interactive"
          :class="{
            'is-ok': resolveNodeView(item).checked && resolveNodeView(item).valid,
            'is-bad': resolveNodeView(item).checked && !resolveNodeView(item).valid,
          }"
          data-stagger-card
          @click="openNodeDetail(item)"
        >
          <header class="nc-head">
            <div class="nc-identity">
              <CountryFlag :code="item.countryCode" :label="item.country" />
              <strong :title="item.name">{{ displayNodeName(item.name) }}</strong>
            </div>
            <span v-if="resolveNodeView(item).kind" class="nc-proto mono">
              {{ resolveNodeView(item).kind?.toUpperCase() }}
            </span>
          </header>

          <!-- Focal Metric Centerpiece -->
          <div class="nc-metric">
            <div class="nc-metric-top">
              <template
                v-if="
                  resolveNodeView(item).checked && resolveNodeView(item).valid
                "
              >
                <div
                  class="nc-rtt mono"
                  :class="rttTone(resolveNodeView(item).rtt)"
                >
                  <span>{{ resolveNodeView(item).rtt ?? 0 }}</span>
                  <small>ms</small>
                </div>
              </template>

              <template
                v-else-if="
                  resolveNodeView(item).checked && !resolveNodeView(item).valid
                "
              >
                <div class="nc-fail">
                  <strong>不可达</strong>
                  <small>{{ resolveNodeView(item).error || "超时" }}</small>
                </div>
              </template>

              <template v-else>
                <div class="nc-idle mono">—</div>
              </template>

              <span
                v-if="nodeSparkline(item.id)?.stats.samples"
                class="nc-uptime mono"
                :title="`近 24 小时可用率（${nodeSparkline(item.id)?.stats.samples} 次采样）`"
              >
                {{ nodeSparkline(item.id)?.stats.uptime }}%
              </span>
            </div>

            <!-- Bespoke Monotone Cubic Sparkline (hover to scrub, click card for 7d detail) -->
            <div
              v-if="nodeSparkline(item.id)?.points.length"
              class="nc-spark"
            >
              <LatencySparkline
                :uid="item.id"
                :points="nodeSparkline(item.id)!.points"
                :from="sparklines!.from"
                :to="sparklines!.to"
                :peak-bands="sparklines!.peakBands"
                :tone="
                  resolveNodeView(item).checked && !resolveNodeView(item).valid
                    ? 'is-bad'
                    : rttTone(resolveNodeView(item).rtt)
                "
              />
            </div>

            <div
              v-else-if="
                resolveNodeView(item).checked && resolveNodeView(item).valid
              "
              class="nc-bar"
              aria-hidden="true"
            >
              <i
                :class="rttTone(resolveNodeView(item).rtt)"
                :style="{ width: `${rttRatio(resolveNodeView(item).rtt)}%` }"
              ></i>
            </div>

            <!-- 多设备并排对比胶囊（全部对比模式下自动展示） -->
            <div
              v-if="
                selectedProbeId === null &&
                probes.length > 1 &&
                item.probes &&
                item.probes.length > 0
              "
              class="nc-compare"
              @click.stop
            >
              <button
                v-for="pr in item.probes"
                :key="pr.id"
                type="button"
                class="nc-probe-chip"
                :class="pr.success ? rttTone(pr.rtt) : 'is-bad'"
                :title="`切换到「${pr.name}」视角`"
                @click="selectedProbeId = pr.id"
              >
                <i aria-hidden="true"></i>
                <span>{{ pr.name }}</span>
                <b class="mono">{{ pr.success ? `${pr.rtt}ms` : "不可达" }}</b>
              </button>
            </div>
          </div>

          <footer class="nc-foot" @click.stop>
            <span class="nc-host mono" :title="`${item.server}:${item.port}`">
              {{ item.server }}:{{ item.port }}
            </span>
            <button
              type="button"
              class="nc-test-btn"
              :disabled="!hasOnlineProbe || probingAll || probingNodeId === item.id"
              @click="runTestOne(item)"
            >
              {{ probingNodeId === item.id ? "测速中…" : "实测" }}
            </button>
          </footer>
        </article>
      </div>

      <EmptyState
        v-else
        size="section"
        title="暂无节点"
        description="调整筛选条件或先添加节点。"
      />
    </section>

    <!-- 添加设备弹窗 -->
    <UiDialog v-model="addDialogVisible" title="添加设备" width="420px">
      <div class="form-stack">
        <label class="field-item">
          <span>名称</span>
          <input
            v-model="newName"
            type="text"
            class="text-input"
            placeholder="例如：家里电脑、软路由"
            @keydown.enter.prevent="submitAddProbe"
          />
        </label>
        <label class="field-item">
          <span>地区（可选）</span>
          <input
            v-model="newRegion"
            type="text"
            class="text-input"
            placeholder="例如：上海电信"
            @keydown.enter.prevent="submitAddProbe"
          />
        </label>
      </div>
      <template #footer>
        <UiButton @click="addDialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="creating" @click="submitAddProbe">
          创建
        </UiButton>
      </template>
    </UiDialog>

    <!-- 启动命令弹窗 -->
    <UiDialog
      v-model="setupDialogVisible"
      :title="`启动 · ${setupTitle}`"
      width="620px"
    >
      <div class="setup-stack">
        <div class="os-tabs" role="tablist">
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'windows' }"
            @click="setupOS = 'windows'"
          >
            Windows
          </button>
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'macos' }"
            @click="setupOS = 'macos'"
          >
            macOS
          </button>
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'linux' }"
            @click="setupOS = 'linux'"
          >
            Linux
          </button>
        </div>

        <pre class="cmd-box mono">{{ setupCommands[setupOS] }}</pre>

        <div class="setup-bar">
          <small>静默后台运行并自启，执行后即可关闭终端。</small>
          <div class="setup-actions">
            <UiButton
              variant="ghost"
              size="sm"
              @click="copyText(setupToken, '已复制令牌')"
            >
              复制令牌
            </UiButton>
            <UiButton
              variant="primary"
              size="sm"
              :icon="CopyDocument"
              @click="copyText(setupCommands[setupOS], '已复制命令')"
            >
              复制启动命令
            </UiButton>
          </div>
        </div>
      </div>
    </UiDialog>
    <!-- 7天精度延迟曲线详情抽屉 -->
    <LatencyDetailModal
      v-model="detailOpen"
      :node="detailNode"
      :initial-probe-id="selectedProbeId"
    />
  </div>
</template>

<style scoped>
.ua-gallery {
  display: grid;
  gap: 28px;
  padding-bottom: 32px;
}

.ua-masthead {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.title-mask {
  overflow: hidden;
}

.gallery-title {
  margin: 0;
  font-size: clamp(26px, 2.8vw, 34px);
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1.15;
  color: var(--c-ink);
}

.online-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.online-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
}

.online-pill.is-online {
  color: var(--c-ink);
}

.online-pill.is-online i {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px rgb(21 184 90 / 0.16);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-line);
  margin-top: -12px;
}

.ua-section {
  display: grid;
  gap: 18px;
}

/* Device Cards */
.device-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}

.device-card {
  display: grid;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
  transition: border-color 180ms var(--ease-out), transform 180ms var(--ease-out);
}

.device-card:hover {
  border-color: var(--c-ink);
  transform: translateY(-2px);
}

.device-card.is-online {
  border-color: color-mix(in srgb, var(--c-ok) 38%, var(--c-line));
}

.device-card.is-clickable {
  cursor: pointer;
}

.device-card.is-selected {
  border-color: var(--c-ink);
  box-shadow: 0 0 0 1px var(--c-ink);
}

.device-active-tag {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-ink);
  color: var(--c-surface);
  font-size: 11px;
  font-weight: 500;
}

.device-head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.device-region {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-surface-2);
  font-size: 11px;
  color: var(--c-muted);
}

.device-ver {
  font-size: 11px;
  color: var(--c-muted);
}

.device-spacer {
  flex: 1;
}

.device-time {
  font-size: 11px;
  color: var(--c-muted);
}

.device-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.device-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.02em;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auto-pill {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background 180ms var(--ease-out),
    border-color 180ms var(--ease-out),
    color 180ms var(--ease-out);
}

.auto-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.auto-pill.is-on {
  border-color: color-mix(in srgb, var(--c-ok) 45%, var(--c-line));
  background: color-mix(in srgb, var(--c-ok) 10%, var(--c-surface));
  color: var(--c-ok);
}

.auto-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  opacity: 0.45;
}

.auto-pill.is-on .auto-dot {
  opacity: 1;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--c-ok) 22%, transparent);
}

.device-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--c-line);
}

.device-icons {
  display: flex;
  align-items: center;
  gap: 2px;
}

.device-empty {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  padding: 22px 24px;
  border: 1px dashed var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
}

.device-empty p {
  margin: 0;
  font-size: 14px;
  color: var(--c-muted);
}

/* Filter Bar */
.nodes-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.filter-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.filter-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 13px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background 160ms var(--ease-out),
    color 160ms var(--ease-out),
    border-color 160ms var(--ease-out);
}

.filter-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.filter-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.view-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid color-mix(in srgb, var(--c-ink) 32%, var(--c-line));
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 150ms var(--ease-out), border-color 150ms var(--ease-out);
}

.view-pill:hover {
  border-color: var(--c-ink);
}

.search-input {
  height: 34px;
  width: 200px;
  max-width: 100%;
  box-sizing: border-box;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
  outline: none;
}

.search-input:focus {
  border-color: var(--c-ink);
}

/* Node Metric Cards */
.node-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.node-card {
  display: grid;
  gap: 18px;
  min-width: 0;
  box-sizing: border-box;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
  transition:
    border-color 180ms var(--ease-out),
    transform 180ms var(--ease-out);
}

.node-card.is-interactive {
  cursor: pointer;
}

.node-card:hover {
  border-color: var(--c-ink);
  transform: translateY(-2px);
}

.nc-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
}

.nc-identity {
  display: flex;
  align-items: center;
  gap: 9px;
  min-width: 0;
}

.nc-identity strong {
  font-size: 14.5px;
  font-weight: 600;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.nc-proto {
  flex-shrink: 0;
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--c-surface-2);
  font-size: 10.5px;
  color: var(--c-muted);
}

.nc-metric {
  display: grid;
  gap: 10px;
  min-height: 48px;
  align-content: center;
}

.nc-metric-top {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.nc-uptime {
  font-size: 11px;
  color: var(--c-muted);
  opacity: 0.85;
}

.nc-spark {
  margin-top: 2px;
}

.nc-rtt {
  display: flex;
  align-items: baseline;
  gap: 4px;
  line-height: 1;
}

.nc-rtt span {
  font-size: 30px;
  font-weight: 600;
  letter-spacing: -0.04em;
  font-variant-numeric: tabular-nums;
}

.nc-rtt small {
  font-size: 12.5px;
  opacity: 0.7;
}

.nc-rtt.is-ok {
  color: var(--c-ok);
}

.nc-rtt.is-warn {
  color: var(--c-warn, #d97706);
}

.nc-rtt.is-bad {
  color: var(--c-bad);
}

.nc-bar {
  height: 4px;
  border-radius: 999px;
  background: var(--c-surface-2);
  overflow: hidden;
}

.nc-bar i {
  display: block;
  height: 100%;
  border-radius: 999px;
  transition: width 320ms var(--ease-out);
}

.nc-bar i.is-ok {
  background: var(--c-ok);
}

.nc-bar i.is-warn {
  background: var(--c-warn, #d97706);
}

.nc-bar i.is-bad {
  background: var(--c-bad);
}

.nc-fail {
  display: flex;
  align-items: baseline;
  gap: 8px;
}

.nc-fail strong {
  font-size: 20px;
  font-weight: 600;
  color: var(--c-bad);
}

.nc-fail small {
  font-size: 12px;
  color: var(--c-muted);
}

.nc-idle {
  font-size: 26px;
  color: var(--c-muted);
  opacity: 0.5;
}

.nc-compare {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-top: 2px;
}

.nc-probe-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11px;
  line-height: 1.2;
  cursor: pointer;
  transition: border-color 150ms var(--ease-out), color 150ms var(--ease-out);
}

.nc-probe-chip:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.nc-probe-chip i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.nc-probe-chip.is-warn i {
  background: var(--c-warn, #d97706);
}

.nc-probe-chip.is-bad i {
  background: var(--c-bad);
}

.nc-probe-chip b {
  font-weight: 600;
  color: var(--c-ink);
}

.nc-probe-chip.is-bad b {
  color: var(--c-bad);
}

.nc-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid var(--c-line);
  min-width: 0;
}

.nc-host {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 11.5px;
  color: var(--c-muted);
}

.nc-test-btn {
  flex-shrink: 0;
  padding: 4px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
  transition:
    background 150ms var(--ease-out),
    color 150ms var(--ease-out),
    border-color 150ms var(--ease-out);
}

.nc-test-btn:hover:not(:disabled) {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.nc-test-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

/* Modals */
.form-stack,
.setup-stack {
  display: grid;
  gap: 14px;
}

.field-item {
  display: grid;
  gap: 6px;
  font-size: 12.5px;
  color: var(--c-muted);
}

.text-input {
  width: 100%;
  box-sizing: border-box;
  height: 38px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 13px;
  outline: none;
}

.text-input:focus {
  border-color: var(--c-ink);
}

.os-tabs {
  display: flex;
  gap: 6px;
}

.os-tab {
  padding: 5px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.os-tab.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.cmd-box {
  margin: 0;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-all;
}

.setup-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.setup-bar small {
  font-size: 12px;
  color: var(--c-muted);
}

.setup-actions {
  display: flex;
  gap: 8px;
}

@media (max-width: 640px) {
  .device-grid,
  .node-card-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .search-input {
    width: 100%;
  }
}
</style>
