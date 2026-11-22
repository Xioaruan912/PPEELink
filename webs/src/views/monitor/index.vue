<script setup lang="ts">
import {
  computed,
  nextTick,
  onMounted,
  onBeforeUnmount,
  onActivated,
  onDeactivated,
  reactive,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";
import gsap from "gsap";
import { Delete, Plus, Refresh } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  EmptyState,
  UiButton,
  UiDialog,
  UiDrawer,
  UiSegmented,
  UiSkeleton,
  UiTag,
} from "@/ui";
import IconChartLine from "~icons/ph/chart-line";
import IconCheckCircle from "~icons/ph/check-circle";
import MetricChart from "@/components/monitor/MetricChart.vue";
import { formatBytes } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";
import {
  addMonitor,
  deleteMonitor,
  getMonitorDetail,
  getMonitors,
  resetMonitorToken,
  updateMonitorAgent,
  type MonitorDetail,
  type MonitorItem,
} from "@/api/monitor";

defineOptions({ name: "MonitorPage" });

const router = useRouter();
const rootEl = ref<HTMLElement>();
const gridEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;
let cardAnim: gsap.core.Tween | null = null;

const phase = ref<"loading" | "ready" | "error">("loading");
const error = ref("");
const items = ref<MonitorItem[]>([]);
const agentManagerVisible = ref(false);
const latestAgentVersion = ref("");
const checkingAgents = ref(false);

async function checkAgents() {
  checkingAgents.value = true;
  try {
    await load(true, true);
  } finally {
    checkingAgents.value = false;
  }
}

const summary = reactive({ total: 0, online: 0, offline: 0 });

const avgCPU = computed(() => {
  const values = items.value
    .filter((item) => item.online && item.latest)
    .map((item) => item.latest!.cpuPercent);
  if (!values.length) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
});

function animateCardsIn() {
  cardAnim?.kill();
  if (
    !gridEl.value ||
    window.matchMedia("(prefers-reduced-motion: reduce)").matches
  ) {
    return;
  }
  const cards = gridEl.value.querySelectorAll<HTMLElement>(".server-card");
  if (!cards.length) return;
  cardAnim = gsap.fromTo(
    cards,
    { y: 16, opacity: 0 },
    {
      y: 0,
      opacity: 1,
      duration: 0.48,
      stagger: 0.055,
      ease: "expo.out",
      clearProps: "transform,opacity",
    }
  );
}

let listLoading = false;
let hasAnimatedInitial = false;
async function load(silent = false, showError = false) {
  if (listLoading) return;
  listLoading = true;
  if (!silent) phase.value = "loading";
  error.value = "";
  try {
    const { data } = await getMonitors();
    items.value = data?.items || [];
    latestAgentVersion.value = data?.agentVersion || "";
    summary.total = data?.summary.total || 0;
    summary.online = data?.summary.online || 0;
    summary.offline = data?.summary.offline || 0;
    phase.value = "ready";
    if (!hasAnimatedInitial) {
      hasAnimatedInitial = true;
      void nextTick(animateCardsIn);
    }
  } catch (err: any) {
    if (showError) error.value = err?.message || "检查更新失败，请重试";
    if (!silent || !items.value.length) {
      phase.value = "error";
      error.value = err?.message || "读取监控数据失败";
    }
  } finally {
    listLoading = false;
  }
}

let refreshTimer: ReturnType<typeof setTimeout> | undefined;
let active = true;
function stopRefresh() {
  active = false;
  clearTimeout(refreshTimer);
}
function scheduleRefresh() {
  clearTimeout(refreshTimer);
  if (!active) return;
  refreshTimer = setTimeout(async () => {
    if (document.visibilityState !== "hidden") await load(true);
    scheduleRefresh();
  }, 15000);
}
function resumeRefresh() {
  active = true;
  scheduleRefresh();
}

onMounted(() => {
  void load();
  resumeRefresh();
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

onActivated(resumeRefresh);
onDeactivated(stopRefresh);
onBeforeUnmount(() => {
  stopRefresh();
  gsapCtx?.revert();
  gsapCtx = null;
  cardAnim?.kill();
});

const upgrading = ref<number | null>(null);
async function upgrade(item: MonitorItem, cancel = false) {
  if (upgrading.value !== null) return;
  upgrading.value = item.id;
  try {
    await updateMonitorAgent(item.id, cancel);
    ElMessage.success(
      cancel ? "已撤销升级请求" : "升级已安排，等待 Agent 完成更新并回报"
    );
    await load();
  } catch (err: any) {
    ElMessage.error(err?.message || "升级请求失败，请重试");
  } finally {
    upgrading.value = null;
  }
}

const addVisible = ref(false);
const adding = ref(false);
const addError = ref("");
const form = reactive({ name: "", group: "", note: "" });
const created = ref<{ name: string; token: string; install: string } | null>(
  null
);

function openAdd() {
  form.name = "";
  form.group = "";
  form.note = "";
  addError.value = "";
  created.value = null;
  addVisible.value = true;
}

async function submitAdd() {
  if (adding.value) return;
  if (!form.name.trim()) {
    addError.value = "请填写服务器名称。";
    return;
  }
  adding.value = true;
  addError.value = "";
  try {
    const { data } = await addMonitor({
      name: form.name.trim(),
      group: form.group.trim(),
      note: form.note.trim(),
    });
    created.value = {
      name: data?.name || form.name.trim(),
      token: data?.token || "",
      install: data?.install || "",
    };
    await load();
  } catch (err: any) {
    addError.value = err?.message || "创建失败，请稍后重试。";
  } finally {
    adding.value = false;
  }
}

async function copy(text: string) {
  if (!text) return;
  await navigator.clipboard.writeText(text);
  ElMessage.success("安装命令已复制");
}

watch(addVisible, (open) => {
  if (!open) created.value = null;
});

const detailVisible = ref(false);
const detail = ref<MonitorDetail | null>(null);
const detailLoading = ref(false);
const detailError = ref("");
const range = ref<string | number>("24h");
const activeMonitor = ref<MonitorItem | null>(null);

const rangeOptions = [
  { label: "1 小时", value: "1h" },
  { label: "6 小时", value: "6h" },
  { label: "24 小时", value: "24h" },
  { label: "7 天", value: "7d" },
];

async function openDetail(item: MonitorItem) {
  activeMonitor.value = item;
  detailVisible.value = true;
  range.value = "24h";
  await loadDetail();
}

async function loadDetail() {
  if (!activeMonitor.value) return;
  detailLoading.value = true;
  detailError.value = "";
  try {
    const { data } = await getMonitorDetail(
      activeMonitor.value.id,
      String(range.value)
    );
    detail.value = data || null;
  } catch (err: any) {
    detailError.value = err?.message || "读取详情失败";
  } finally {
    detailLoading.value = false;
  }
}

watch(range, () => {
  if (detailVisible.value) loadDetail();
});

function closeDetail() {
  detailVisible.value = false;
  detail.value = null;
}

const latest = computed(() => {
  if (detail.value?.latest) return detail.value.latest;
  return activeMonitor.value?.latest;
});

const labels = computed(() => {
  const rows = detail.value?.metrics || [];
  return rows.map((row) => {
    const date = new Date(row.collectedAt);
    const pad = (part: number) => String(part).padStart(2, "0");
    if (range.value === "7d") {
      return `${date.getMonth() + 1}-${date.getDate()} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
    }
    return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
  });
});

const rows = computed(() => detail.value?.metrics || []);
const percent = (used = 0, total = 0) =>
  total > 0 ? Math.min(100, (used / total) * 100) : 0;

function barTone(value: number) {
  if (value >= 90) return "is-bad";
  if (value >= 75) return "is-warn";
  return "";
}

const cpuSeries = computed(() => [
  { name: "CPU", data: rows.value.map((row) => row.cpuPercent) },
]);
const memSeries = computed(() => [
  {
    name: "内存",
    data: rows.value.map((row) => percent(row.memUsed, row.memTotal)),
  },
]);
const diskSeries = computed(() => [
  {
    name: "磁盘",
    data: rows.value.map((row) => percent(row.diskUsed, row.diskTotal)),
  },
]);
const netSeries = computed(() => [
  {
    name: "下行",
    data: rows.value.map((row) => row.netInSpeed),
    color: "#296edf",
  },
  {
    name: "上行",
    data: rows.value.map((row) => row.netOutSpeed),
    color: "#15b85a",
  },
]);
const loadSeries = computed(() => [
  { name: "1 分钟", data: rows.value.map((row) => row.load1) },
  {
    name: "5 分钟",
    data: rows.value.map((row) => row.load5),
    color: "#296edf",
  },
  {
    name: "15 分钟",
    data: rows.value.map((row) => row.load15),
    color: "#b45309",
  },
]);

function formatUptime(seconds?: number) {
  if (!seconds) return "--";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days} 天 ${hours} 小时`;
  if (hours > 0) return `${hours} 小时 ${minutes} 分`;
  return `${minutes} 分`;
}

function agentLabel(item: MonitorItem) {
  return item.agentVersion
    ? `v${item.agentVersion}`
    : item.version || item.agentHash
      ? "旧版本"
      : "等待上报";
}

function agentState(item: MonitorItem) {
  if (item.removing) return "等待卸载";
  if (item.agentUpdateError) return "升级失败";
  if (item.agentUpdateRequested)
    return item.online ? "等待更新回报" : "等待服务器上线";
  if (item.agentUpgradeable === false) return "随面板更新";
  if (item.agentUpdateConfirmed) return "已是最新";
  if (item.agentUpdateAvailable) return "有可用更新";
  return "等待上报";
}

const currentPercent = (used?: number, total?: number) =>
  total ? percent(used, total).toFixed(1) : "0.0";

async function removeMonitor(item: MonitorItem | null) {
  if (!item) return;
  try {
    await ElMessageBox.confirm(
      item.ownsAgent
        ? `删除「${item.name}」将卸载其 Agent 和代理服务，并删除历史指标。`
        : `删除监控「${item.name}」？历史指标会一起删除。`,
      "删除服务器监控",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  try {
    const { data } = await deleteMonitor(item.id);
    if (activeMonitor.value?.id === item.id) closeDetail();
    ElMessage.success(
      data?.pending
        ? "已排队卸载，等待 Agent 回报后自动删除"
        : "服务器监控已删除"
    );
    await load();
  } catch {
    ElMessage.error("删除失败，请稍后重试");
  }
}

async function resetToken(item: MonitorItem | null) {
  if (!item) return;
  try {
    const { data } = await resetMonitorToken(item.id);
    created.value = {
      name: item.name,
      token: data?.token || "",
      install: data?.install || "",
    };
    addVisible.value = true;
    ElMessage.success("令牌已重置");
  } catch {
    ElMessage.error("重置失败，请稍后重试");
  }
}
</script>

<template>
  <div ref="rootEl" class="monitor-gallery">
    <!-- Compact Editorial Masthead -->
    <header class="monitor-masthead" aria-labelledby="monitor-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <div class="title-mask">
            <h1 id="monitor-title" class="gallery-title" data-title-reveal>
              主机与 Agent
            </h1>
          </div>
          <dl class="inline-metrics mono" aria-label="服务器运行概览">
            <div>
              <dt>服务器</dt>
              <dd>{{ summary.total }}</dd>
            </div>
            <div>
              <dt>在线</dt>
              <dd class="is-ok">{{ summary.online }}</dd>
            </div>
            <div v-if="summary.offline > 0">
              <dt>离线</dt>
              <dd class="is-bad">{{ summary.offline }}</dd>
            </div>
            <div>
              <dt>平均 CPU</dt>
              <dd>{{ avgCPU.toFixed(1) }}%</dd>
            </div>
          </dl>
        </div>
        <p class="gallery-lead">
          实时监控服务器硬件负载与网络吞吐，点击任意卡片查看历史曲线。
        </p>
      </div>

      <div class="masthead-actions">
        <UiButton variant="primary" :icon="Plus" @click="openAdd">
          添加服务器
        </UiButton>
        <UiButton variant="ghost" @click="agentManagerVisible = true">
          Agent 管理
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="phase === 'loading'"
          @click="load()"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <div v-if="phase === 'loading'" class="state-pad">
      <UiSkeleton :lines="4" height="84px" />
    </div>

    <div v-else-if="phase === 'error'" class="state-error" role="alert">
      <span>{{ error }}</span>
      <UiButton @click="load()">重试</UiButton>
    </div>

    <EmptyState
      v-else-if="!items.length"
      size="section"
      title="还没有监控服务器"
      description="在 VPS 上运行一条安装命令，即可实时查看 CPU、内存、磁盘与网络。"
    >
      <template #icon>
        <span class="empty-icon"><IconChartLine /></span>
      </template>
      <template #action>
        <UiButton variant="primary" :icon="Plus" @click="openAdd">
          添加第一台服务器
        </UiButton>
      </template>
    </EmptyState>

    <!-- Pure Telemetry Server Cards -->
    <div v-else ref="gridEl" class="server-grid">
      <article
        v-for="item in items"
        :key="item.id"
        class="server-card"
        :class="{ 'is-offline': !item.online }"
        role="button"
        tabindex="0"
        @click="openDetail(item)"
        @keydown.enter="openDetail(item)"
      >
        <header class="card-head">
          <div class="card-title-wrap">
            <span
              class="status-dot"
              :class="item.online ? 'is-on' : 'is-off'"
              aria-hidden="true"
            ></span>
            <div class="card-identity">
              <h2>{{ item.name }}</h2>
              <span class="card-meta mono">
                {{
                  [
                    item.group,
                    item.os && item.arch
                      ? `${item.os}/${item.arch}`
                      : item.os || "待上报",
                  ]
                    .filter(Boolean)
                    .join(" · ")
                }}
              </span>
            </div>
          </div>
          <UiTag :tone="item.online ? 'ok' : 'neutral'">
            {{ item.online ? "在线" : "离线" }}
          </UiTag>
        </header>

        <div v-if="item.latest" class="card-metrics">
          <div class="metric-item">
            <div class="metric-row">
              <span>CPU</span>
              <strong class="mono">{{ item.latest.cpuPercent.toFixed(1) }}%</strong>
            </div>
            <div class="metric-track">
              <i
                :class="barTone(item.latest.cpuPercent)"
                :style="{
                  transform: `scaleX(${Math.max(0.02, Math.min(1, item.latest.cpuPercent / 100))})`,
                }"
              ></i>
            </div>
          </div>

          <div class="metric-item">
            <div class="metric-row">
              <span>内存</span>
              <strong class="mono">
                {{ currentPercent(item.latest.memUsed, item.latest.memTotal) }}%
              </strong>
            </div>
            <div class="metric-track">
              <i
                :class="barTone(percent(item.latest.memUsed, item.latest.memTotal))"
                :style="{
                  transform: `scaleX(${Math.max(0.02, Math.min(1, percent(item.latest.memUsed, item.latest.memTotal) / 100))})`,
                }"
              ></i>
            </div>
          </div>

          <div class="metric-item">
            <div class="metric-row">
              <span>磁盘</span>
              <strong class="mono">
                {{ currentPercent(item.latest.diskUsed, item.latest.diskTotal) }}%
              </strong>
            </div>
            <div class="metric-track">
              <i
                :class="barTone(percent(item.latest.diskUsed, item.latest.diskTotal))"
                :style="{
                  transform: `scaleX(${Math.max(0.02, Math.min(1, percent(item.latest.diskUsed, item.latest.diskTotal) / 100))})`,
                }"
              ></i>
            </div>
          </div>
        </div>
        <p v-else class="card-empty">等待服务器首次上报…</p>

        <footer class="card-foot mono">
          <div v-if="item.latest" class="net-speed">
            <span>↓ {{ formatBytes(item.latest.netInSpeed) }}/s</span>
            <span>↑ {{ formatBytes(item.latest.netOutSpeed) }}/s</span>
          </div>
          <span v-else class="net-speed">--</span>
          <span class="uptime">
            {{ item.latest ? formatUptime(item.latest.uptime) : "未上线" }}
          </span>
        </footer>
      </article>
    </div>

    <!-- Clean Agent Manager Drawer (Zero Bottom Fine-Print Clutter) -->
    <UiDrawer
      v-model="agentManagerVisible"
      title="Agent 管理"
      size="min(620px, 100vw)"
    >
      <div class="agent-manager">
        <div class="agent-release">
          <div>
            <span>当前发布版本</span>
            <strong class="mono">
              {{ latestAgentVersion ? `v${latestAgentVersion}` : "等待获取" }}
            </strong>
          </div>
          <UiButton
            variant="ghost"
            :icon="Refresh"
            :loading="checkingAgents"
            @click="checkAgents"
          >
            检查更新
          </UiButton>
        </div>

        <p v-if="error" class="agent-error" role="alert">{{ error }}</p>

        <EmptyState
          v-if="!items.length"
          size="inline"
          description="添加服务器后，可在这里统一管理 Agent 版本。"
        />

        <div v-else class="agent-list">
          <article v-for="item in items" :key="item.id" class="agent-row">
            <div class="agent-row-main">
              <div class="agent-row-top">
                <strong>{{ item.name }}</strong>
                <UiTag :tone="item.online ? 'ok' : 'neutral'">
                  {{ item.online ? "在线" : "离线" }}
                </UiTag>
              </div>
              <div class="agent-version-line mono">
                <span>{{ agentLabel(item) }}</span>
                <span
                  class="agent-state"
                  :class="{ failed: !!item.agentUpdateError }"
                >
                  {{ agentState(item) }}
                </span>
              </div>
              <p v-if="item.agentUpdateError" class="agent-error">
                {{ item.agentUpdateError }}
              </p>
            </div>

            <div class="agent-row-actions">
              <UiButton
                v-if="
                  item.agentUpgradeable !== false &&
                  (item.agentUpdateAvailable ||
                    item.agentUpdateRequested ||
                    item.agentUpdateError)
                "
                size="sm"
                :variant="item.agentUpdateRequested ? 'ghost' : 'primary'"
                :loading="upgrading === item.id"
                :disabled="upgrading !== null || item.removing"
                @click="
                  upgrade(
                    item,
                    !!item.agentUpdateRequested && !item.agentUpdateError
                  )
                "
              >
                {{
                  item.agentUpdateError
                    ? "重试升级"
                    : item.agentUpdateRequested
                      ? "撤销升级"
                      : "升级 Agent"
                }}
              </UiButton>
              <UiButton
                v-if="item.proxyServerId"
                size="sm"
                variant="ghost"
                @click="
                  agentManagerVisible = false;
                  router.push({ name: 'ProxyServers' });
                "
              >
                代理配置
              </UiButton>
            </div>
          </article>
        </div>
      </div>
    </UiDrawer>

    <!-- Add Server Monitor Dialog -->
    <UiDialog
      v-model="addVisible"
      :title="created ? '监控已创建' : '添加服务器监控'"
      width="620px"
    >
      <template v-if="created">
        <div class="install-success">
          <div class="install-head">
            <span class="success-icon"><IconCheckCircle /></span>
            <div class="install-title">
              <strong>{{ created.name }}</strong>
              <small>已创建，以 root 登录服务器执行下方命令即可接入</small>
            </div>
          </div>
          <div class="command-box">
            <code class="mono">{{ created.install }}</code>
            <button
              type="button"
              class="copy-chip"
              @click="copy(created.install)"
            >
              复制
            </button>
          </div>
        </div>
      </template>

      <el-form v-else label-position="top" @submit.prevent="submitAdd">
        <div class="dialog-grid-2">
          <el-form-item label="服务器名称" required>
            <el-input
              v-model="form.name"
              maxlength="40"
              placeholder="例如：日本-01"
            />
          </el-form-item>
          <el-form-item label="分组">
            <el-input
              v-model="form.group"
              maxlength="20"
              placeholder="例如：亚太"
            />
          </el-form-item>
        </div>
        <el-form-item label="备注">
          <el-input
            v-model="form.note"
            maxlength="80"
            placeholder="例如：主要落地机"
          />
        </el-form-item>
        <p v-if="addError" class="field-error" role="alert">{{ addError }}</p>
      </el-form>

      <template #footer>
        <UiButton variant="ghost" @click="addVisible = false">
          {{ created ? "完成" : "取消" }}
        </UiButton>
        <UiButton
          v-if="!created"
          variant="primary"
          :loading="adding"
          @click="submitAdd"
        >
          创建并生成命令
        </UiButton>
      </template>
    </UiDialog>

    <!-- Pure Telemetry Detail Dialog (No Redundant Terminal/Files/Docker Buttons) -->
    <UiDialog
      v-model="detailVisible"
      :title="detail?.monitor.name || activeMonitor?.name || '服务器详情'"
      width="980px"
      @closed="closeDetail"
    >
      <div v-if="detailLoading" class="detail-loading">
        <UiSkeleton :lines="5" height="56px" />
      </div>
      <div v-else-if="detailError" class="state-error" role="alert">
        <span>{{ detailError }}</span>
        <UiButton @click="loadDetail">重试</UiButton>
      </div>
      <template v-else-if="detail">
        <div class="detail-head">
          <div class="detail-title">
            <span
              class="status-dot"
              :class="detail.online ? 'is-on' : 'is-off'"
              aria-hidden="true"
            ></span>
            <strong>{{ detail.monitor.name }}</strong>
            <UiTag :tone="detail.online ? 'ok' : 'neutral'">
              {{ detail.online ? "在线" : "离线" }}
            </UiTag>
            <span class="detail-meta mono">
              {{ detail.monitor.group || "未分组" }} ·
              {{ detail.monitor.os }}/{{ detail.monitor.arch }}
            </span>
          </div>
          <div class="detail-actions">
            <UiButton
              size="sm"
              variant="ghost"
              :icon="Refresh"
              :loading="detailLoading"
              @click="loadDetail"
            >
              刷新
            </UiButton>
            <UiButton
              size="sm"
              variant="ghost"
              @click="resetToken(activeMonitor)"
            >
              重置令牌
            </UiButton>
            <UiButton
              size="sm"
              variant="danger"
              :icon="Delete"
              @click="removeMonitor(activeMonitor)"
            >
              删除
            </UiButton>
          </div>
        </div>

        <div v-if="latest" class="detail-stats">
          <div>
            <span>CPU</span>
            <strong class="mono">{{ latest.cpuPercent.toFixed(1) }}%</strong>
          </div>
          <div>
            <span>内存</span>
            <strong class="mono">
              {{ currentPercent(latest.memUsed, latest.memTotal) }}%
            </strong>
            <small class="mono">
              {{ formatBytes(latest.memUsed) }} /
              {{ formatBytes(latest.memTotal) }}
            </small>
          </div>
          <div>
            <span>磁盘</span>
            <strong class="mono">
              {{ currentPercent(latest.diskUsed, latest.diskTotal) }}%
            </strong>
            <small class="mono">
              {{ formatBytes(latest.diskUsed) }} /
              {{ formatBytes(latest.diskTotal) }}
            </small>
          </div>
          <div>
            <span>负载</span>
            <strong class="mono">{{ latest.load1.toFixed(2) }}</strong>
            <small class="mono">
              {{ latest.load5.toFixed(2) }} / {{ latest.load15.toFixed(2) }}
            </small>
          </div>
          <div>
            <span>运行时间</span>
            <strong class="mono">{{ formatUptime(latest.uptime) }}</strong>
            <small class="mono">
              {{ latest.processCount }} 进程 · {{ latest.tcpConns }} TCP
            </small>
          </div>
        </div>

        <div class="range-row">
          <UiSegmented
            v-model="range"
            label="时间范围"
            :options="rangeOptions"
          />
        </div>

        <div v-if="!rows.length" class="chart-empty">
          还没有历史指标，等待下一次上报。
        </div>
        <div v-else class="chart-grid">
          <MetricChart
            title="CPU"
            unit="%"
            :max="100"
            :labels="labels"
            :series="cpuSeries"
          />
          <MetricChart
            title="内存"
            unit="%"
            :max="100"
            :labels="labels"
            :series="memSeries"
          />
          <MetricChart
            title="磁盘"
            unit="%"
            :max="100"
            :labels="labels"
            :series="diskSeries"
          />
          <MetricChart
            title="网络"
            unit="MB/s"
            :labels="labels"
            :series="netSeries"
          />
          <MetricChart
            title="负载"
            :labels="labels"
            :series="loadSeries"
            :decimals="1"
            class="chart-wide"
          />
        </div>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.monitor-gallery {
  display: grid;
  gap: 20px;
  max-width: 1200px;
  min-width: 0;
  padding-bottom: 56px;
}

.monitor-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.masthead-left {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.title-cluster {
  display: flex;
  align-items: baseline;
  gap: 20px;
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
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.inline-metrics {
  display: inline-flex;
  align-items: baseline;
  gap: 16px;
  margin: 0;
}

.inline-metrics > div {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
}

.inline-metrics dt {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--c-muted);
}

.inline-metrics dd {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--c-ink);
}

.inline-metrics dd.is-ok {
  color: var(--c-ok);
}

.inline-metrics dd.is-bad {
  color: var(--c-bad);
}

.gallery-lead {
  margin: 0;
  font-size: 13.5px;
  color: var(--c-muted);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.state-pad {
  padding: 20px 0;
}

.state-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 18px;
  border: 1px solid rgb(223 41 41 / 0.25);
  border-radius: 14px;
  background: rgb(223 41 41 / 0.05);
  color: var(--c-bad);
  font-size: 13.5px;
}

.empty-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: var(--c-surface-2);
  color: var(--c-ink);
}

/* Sculptural Server Telemetry Cards */
.server-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 16px;
}

.server-card {
  display: grid;
  gap: 18px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out),
    opacity var(--duration-fast) var(--ease-out);
}

.server-card.is-offline {
  opacity: 0.72;
}

@media (hover: hover) and (pointer: fine) {
  .server-grid:hover .server-card:not(:hover) {
    opacity: 0.62;
  }
  .server-card:hover {
    border-color: var(--c-ink);
    transform: translateY(-2px);
    opacity: 1;
  }
}

.card-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.card-title-wrap {
  display: flex;
  align-items: center;
  gap: 11px;
  min-width: 0;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex: none;
  background: var(--c-muted);
}

.status-dot.is-on {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px rgb(21 184 90 / 0.16);
}

.card-identity {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.card-identity h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 16.5px;
  font-weight: 500;
  letter-spacing: -0.02em;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-meta {
  font-size: 11.5px;
  color: var(--c-muted);
}

.card-metrics {
  display: grid;
  gap: 12px;
}

.metric-item {
  display: grid;
  gap: 5px;
}

.metric-row {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  font-size: 12px;
  color: var(--c-muted);
}

.metric-row strong {
  font-size: 12.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.metric-track {
  height: 3px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--c-surface-2);
}

.metric-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--c-ok);
  transform-origin: left;
  transition: transform var(--duration-medium) var(--ease-out);
}

.metric-track i.is-warn {
  background: var(--c-warn);
}

.metric-track i.is-bad {
  background: var(--c-bad);
}

.card-empty {
  margin: 0;
  padding: 20px 0;
  font-size: 13px;
  color: var(--c-muted);
}

.card-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--c-line);
  font-size: 11.5px;
  color: var(--c-muted);
}

.net-speed {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}

/* Agent Manager Drawer */
.agent-manager {
  display: grid;
  gap: 18px;
}

.agent-release {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
}

.agent-release span {
  display: block;
  font-size: 11.5px;
  color: var(--c-muted);
}

.agent-release strong {
  font-size: 18px;
  font-weight: 500;
  color: var(--c-ink);
}

.agent-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.agent-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 0;
  border-bottom: 1px solid var(--c-line);
}

.agent-row-main {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.agent-row-top {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-row-top strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.agent-version-line {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 12px;
  color: var(--c-muted);
}

.agent-state.failed {
  color: var(--c-bad);
}

.agent-error {
  margin: 0;
  font-size: 12px;
  color: var(--c-bad);
}

.agent-row-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex: none;
}

/* Add Server Modal */
.dialog-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.install-success {
  display: grid;
  gap: 16px;
}

.install-head {
  display: flex;
  align-items: center;
  gap: 12px;
}

.success-icon {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgb(21 184 90 / 0.12);
  color: var(--c-ok);
}

.install-title {
  display: grid;
  gap: 2px;
}

.install-title small {
  color: var(--c-muted);
  font-size: 12.5px;
}

.command-box {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 12px;
  background: #0c0c0f;
  color: #f0f0ed;
  font-size: 12px;
  line-height: 1.6;
  word-break: break-all;
}

.copy-chip {
  flex: none;
  padding: 5px 12px;
  border: 1px solid rgb(255 255 255 / 0.18);
  border-radius: 999px;
  background: rgb(255 255 255 / 0.08);
  color: #ffffff;
  font-size: 12px;
  cursor: pointer;
}

.copy-chip:hover {
  background: rgb(255 255 255 / 0.16);
}

.field-error {
  margin: 0;
  color: var(--c-bad);
  font-size: 12.5px;
}

/* Detail Modal */
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--c-line);
}

.detail-title {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.detail-title strong {
  font-size: 16px;
  font-weight: 500;
}

.detail-meta {
  font-size: 12px;
  color: var(--c-muted);
}

.detail-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.detail-stats {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 12px;
  padding: 16px 0;
  border-bottom: 1px solid var(--c-line);
}

.detail-stats > div {
  display: grid;
  gap: 3px;
}

.detail-stats span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.detail-stats strong {
  font-size: 18px;
  font-weight: 500;
  color: var(--c-ink);
}

.detail-stats small {
  font-size: 11px;
  color: var(--c-muted);
}

.range-row {
  padding: 16px 0 12px;
}

.chart-empty {
  padding: 36px 0;
  text-align: center;
  color: var(--c-muted);
  font-size: 13px;
}

.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.chart-wide {
  grid-column: 1 / -1;
}

@media (max-width: 780px) {
  .server-grid,
  .chart-grid,
  .dialog-grid-2 {
    grid-template-columns: 1fr;
  }
  .detail-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
