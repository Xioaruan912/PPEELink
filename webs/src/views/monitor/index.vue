<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue";
import { Delete, Plus, Refresh } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  EmptyState,
  PageHeader,
  UiButton,
  UiDialog,
  UiPanel,
  UiSegmented,
  UiSkeleton,
  UiTag,
} from "@/ui";
import IconChartLine from "~icons/ph/chart-line";
import IconCheckCircle from "~icons/ph/check-circle";
import MetricChart from "@/components/monitor/MetricChart.vue";
import MonitorTerminal from "./MonitorTerminal.vue";
import SshSettingsDialog from "./SshSettingsDialog.vue";
import { formatBytes } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";
import {
  addMonitor,
  deleteMonitor,
  getMonitorDetail,
  getMonitors,
  resetMonitorToken,
  type MonitorDetail,
  type MonitorItem,
} from "@/api/monitor";

defineOptions({ name: "MonitorPage" });

const phase = ref<"loading" | "ready" | "error">("loading");
const error = ref("");
const items = ref<MonitorItem[]>([]);
const summary = reactive({ total: 0, online: 0, offline: 0 });

const avgCPU = computed(() => {
  const values = items.value
    .filter((item) => item.online && item.latest)
    .map((item) => item.latest!.cpuPercent);
  if (!values.length) return 0;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
});

async function load() {
  phase.value = "loading";
  error.value = "";
  try {
    const { data } = await getMonitors();
    items.value = data?.items || [];
    summary.total = data?.summary.total || 0;
    summary.online = data?.summary.online || 0;
    summary.offline = data?.summary.offline || 0;
    phase.value = "ready";
  } catch (err: any) {
    phase.value = "error";
    error.value = err?.message || "读取监控数据失败";
  }
}

onMounted(load);

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
const terminalVisible = ref(false);
const terminalTarget = ref<MonitorItem | null>(null);
const sshSettingsVisible = ref(false);
const sshTarget = ref<MonitorItem | null>(null);

function openTerminal(item: MonitorItem | null) {
  if (!item) return;
  if (!item.sshConfigured) {
    sshTarget.value = item;
    sshSettingsVisible.value = true;
    return;
  }
  terminalTarget.value = item;
  terminalVisible.value = true;
}

function openSSHSettings(item: MonitorItem | null) {
  if (!item) return;
  sshTarget.value = item;
  sshSettingsVisible.value = true;
}

async function onSSHSaved() {
  await load();
  const updated = items.value.find((item) => item.id === sshTarget.value?.id);
  if (updated) {
    sshTarget.value = updated;
    if (activeMonitor.value?.id === updated.id) activeMonitor.value = updated;
    if (terminalTarget.value?.id === updated.id) terminalTarget.value = updated;
  }
  if (detailVisible.value) await loadDetail();
}

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
  { name: "下行", data: rows.value.map((row) => row.netInSpeed), color: "#4fc3f7" },
  { name: "上行", data: rows.value.map((row) => row.netOutSpeed), color: "#ffb74d" },
]);
const loadSeries = computed(() => [
  { name: "1 分钟", data: rows.value.map((row) => row.load1) },
  { name: "5 分钟", data: rows.value.map((row) => row.load5), color: "#4fc3f7" },
  { name: "15 分钟", data: rows.value.map((row) => row.load15), color: "#ba68c8" },
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

const currentPercent = (used?: number, total?: number) =>
  total ? percent(used, total).toFixed(1) : "0.0";

async function removeMonitor(item: MonitorItem | null) {
  if (!item) return;
  try {
    await ElMessageBox.confirm(
      `删除监控「${item.name}」？历史指标会一起删除。`,
      "删除服务器监控",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  try {
    await deleteMonitor(item.id);
    if (activeMonitor.value?.id === item.id) closeDetail();
    ElMessage.success("服务器监控已删除");
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
    ElMessage.success("令牌已重置，请更新服务器上的监控命令");
  } catch {
    ElMessage.error("重置失败，请稍后重试");
  }
}
</script>
<template>
  <div class="monitor-page">
    <PageHeader
      title="监控"
      description="独立查看服务器 CPU、内存、磁盘与网络，不依赖节点探针。"
    >
      <template #actions>
        <UiButton :icon="Refresh" :loading="phase === 'loading'" @click="load"
          >刷新</UiButton
        >
        <UiButton variant="primary" :icon="Plus" @click="openAdd"
          >添加服务器</UiButton
        >
      </template>
    </PageHeader>

    <div class="summary-line">
      <div>
        <span>服务器</span>
        <strong>{{ summary.total }}</strong>
      </div>
      <div>
        <span>在线</span>
        <strong class="ok">{{ summary.online }}</strong>
      </div>
      <div>
        <span>离线</span>
        <strong>{{ summary.offline }}</strong>
      </div>
      <div>
        <span>平均 CPU</span>
        <strong>{{ avgCPU.toFixed(1) }}<i>%</i></strong>
      </div>
    </div>

    <UiPanel
      title="服务器"
      :count="items.length"
      description="点击卡片查看历史曲线与实时指标。"
      flush
    >
      <template #actions>
        <UiButton :icon="Plus" @click="openAdd">添加</UiButton>
      </template>
      <UiSkeleton
        v-if="phase === 'loading'"
        class="panel-pad"
        :lines="4"
        height="64px"
      />
      <div v-else-if="phase === 'error'" class="state-error panel-pad" role="alert">
        <span>{{ error }}</span>
        <UiButton @click="load">重试</UiButton>
      </div>
      <EmptyState
        v-else-if="!items.length"
        size="section"
        title="还没有监控服务器"
        description="在 VPS 上运行一条安装命令，就能看到 CPU、内存、磁盘和网络。"
      >
        <template #icon>
          <span class="empty-icon"><IconChartLine /></span>
        </template>
        <template #action>
          <UiButton :icon="Plus" @click="openAdd">添加第一台服务器</UiButton>
        </template>
      </EmptyState>
      <div v-else class="server-grid">
        <article
          v-for="item in items"
          :key="item.id"
          class="server-card"
          role="button"
          tabindex="0"
          @click="openDetail(item)"
          @keydown.enter="openDetail(item)"
        >
          <header class="server-head">
            <span class="status-dot" :class="item.online ? 'is-on' : 'is-off'"></span>
            <strong>{{ item.name }}</strong>
            <UiTag :tone="item.online ? 'ok' : 'neutral'">{{
              item.online ? "在线" : "离线"
            }}</UiTag>
          </header>
          <p class="server-sub">
            {{
              item.linked
                ? "自建节点 agent"
                : `${item.group || "未分组"} · ${item.os || "unknown"}/${item.arch || "?"}`
            }}
            <span v-if="item.lastSeenAt">
              · {{ formatDateTime(item.lastSeenAt) }}
            </span>
          </p>

          <template v-if="item.latest">
            <div class="metric-line">
              <span>CPU</span>
              <b>{{ item.latest.cpuPercent.toFixed(1) }}%</b>
            </div>
            <div class="bar"><i :style="{ width: `${Math.max(2, item.latest.cpuPercent)}%` }"></i></div>
            <div class="metric-line">
              <span>内存</span>
              <b
                >{{ currentPercent(item.latest.memUsed, item.latest.memTotal) }}%</b
              >
            </div>
            <div class="bar">
              <i
                :style="{
                  width: `${Math.max(2, percent(item.latest.memUsed, item.latest.memTotal))}%`,
                }"
              ></i>
            </div>
            <div class="metric-line">
              <span>磁盘</span>
              <b
                >{{ currentPercent(item.latest.diskUsed, item.latest.diskTotal) }}%</b
              >
            </div>
            <div class="bar">
              <i
                :style="{
                  width: `${Math.max(2, percent(item.latest.diskUsed, item.latest.diskTotal))}%`,
                }"
              ></i>
            </div>
            <footer class="server-foot">
              <span>↓ {{ formatBytes(item.latest.netInSpeed) }}/s</span>
              <span>↑ {{ formatBytes(item.latest.netOutSpeed) }}/s</span>
              <span>{{ formatUptime(item.latest.uptime) }}</span>
            </footer>
          </template>
          <p v-else class="server-empty">等待服务器首次上报…</p>
        </article>
      </div>
    </UiPanel>

    <UiDialog
      v-model="addVisible"
      :title="created ? '监控已创建' : '添加服务器监控'"
      width="680px"
    >
      <template v-if="created">
        <div class="install-success">
          <div class="install-head">
            <span class="success-icon"><IconCheckCircle /></span>
            <div class="install-title">
              <strong>{{ created.name }}</strong>
              <small>已创建，等待服务器首次上报</small>
            </div>
          </div>
          <div class="install-steps">
            <div><span>1</span>使用 root 登录服务器</div>
            <div><span>2</span>执行下面的安装命令</div>
          </div>
          <div class="command-box">
            <code>{{ created.install }}</code>
            <button
              type="button"
              class="copy-chip"
              @click="copy(created.install)"
            >
              复制
            </button>
          </div>
          <p class="install-hint">
            令牌只显示这一次。重置令牌后旧命令会失效；安装完成后约一分钟出现第一份数据。
          </p>
        </div>
      </template>
      <el-form v-else label-position="top" @submit.prevent="submitAdd">
        <p class="dialog-note">
          如果这台机器已经装了自建节点 agent，不用在这里添加：到「自建节点」编辑服务器，打开「服务器监控」即可复用同一个
          agent。
        </p>
        <el-form-item label="名称" required>
          <el-input
            v-model="form.name"
            maxlength="40"
            placeholder="例如 日本-01"
          />
        </el-form-item>
        <el-form-item label="分组">
          <el-input v-model="form.group" maxlength="20" placeholder="例如 日本" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input
            v-model="form.note"
            maxlength="80"
            placeholder="例如 主要落地机"
          />
        </el-form-item>
        <p v-if="addError" class="field-error" role="alert">{{ addError }}</p>
      </el-form>
      <template #footer>
        <UiButton @click="addVisible = false">{{
          created ? "完成" : "取消"
        }}</UiButton>
        <UiButton
          v-if="!created"
          variant="primary"
          :loading="adding"
          @click="submitAdd"
          >创建</UiButton
        >
      </template>
    </UiDialog>

    <UiDialog
      v-model="detailVisible"
      :title="detail?.monitor.name || activeMonitor?.name || '服务器详情'"
      width="1000px"
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
            ></span>
            <strong>{{ detail.monitor.name }}</strong>
            <UiTag :tone="detail.online ? 'ok' : 'neutral'">{{
              detail.online ? "在线" : "离线"
            }}</UiTag>
            <span class="detail-meta"
              >{{ detail.monitor.group || "未分组" }} ·
              {{ detail.monitor.os }}/{{ detail.monitor.arch }}</span
            >
          </div>
          <div class="detail-actions">
            <UiButton variant="primary" @click="openTerminal(activeMonitor)"
              >SSH 终端</UiButton
            >
            <UiButton @click="openSSHSettings(activeMonitor)">SSH 设置</UiButton>
            <UiButton
              :icon="Refresh"
              :loading="detailLoading"
              @click="loadDetail"
              >刷新</UiButton
            >
            <UiButton
              :icon="Refresh"
              @click="resetToken(activeMonitor)"
              >重置令牌</UiButton
            >
            <UiButton
              variant="danger"
              :icon="Delete"
              @click="removeMonitor(activeMonitor)"
              >删除</UiButton
            >
          </div>
        </div>

        <div class="detail-stats" v-if="latest">
          <div>
            <span>CPU</span>
            <strong>{{ latest.cpuPercent.toFixed(1) }}%</strong>
          </div>
          <div>
            <span>内存</span>
            <strong>{{ currentPercent(latest.memUsed, latest.memTotal) }}%</strong>
            <small>{{ formatBytes(latest.memUsed) }} / {{ formatBytes(latest.memTotal) }}</small>
          </div>
          <div>
            <span>磁盘</span>
            <strong>{{ currentPercent(latest.diskUsed, latest.diskTotal) }}%</strong>
            <small>{{ formatBytes(latest.diskUsed) }} / {{ formatBytes(latest.diskTotal) }}</small>
          </div>
          <div>
            <span>负载</span>
            <strong>{{ latest.load1.toFixed(2) }}</strong>
            <small>{{ latest.load5.toFixed(2) }} / {{ latest.load15.toFixed(2) }}</small>
          </div>
          <div>
            <span>运行时间</span>
            <strong>{{ formatUptime(latest.uptime) }}</strong>
            <small>{{ latest.processCount }} 进程 · {{ latest.tcpConns }} TCP</small>
          </div>
        </div>

        <div class="range-row">
          <UiSegmented v-model="range" label="时间范围" :options="rangeOptions" />
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

    <MonitorTerminal
      v-model="terminalVisible"
      :monitor="terminalTarget"
      @open-settings="openSSHSettings(terminalTarget)"
    />
    <SshSettingsDialog
      v-model="sshSettingsVisible"
      :monitor="sshTarget"
      @saved="onSSHSaved"
    />
  </div>
</template>
<style scoped>
.monitor-page {
  padding-bottom: 48px;
}
.summary-line {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0;
  margin-bottom: 12px;
  padding: 8px 16px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}
.summary-line > div {
  display: flex;
  align-items: baseline;
  gap: 7px;
  padding: 2px 22px;
  border-right: 1px solid var(--c-line);
}
.summary-line > div:first-child {
  padding-left: 0;
}
.summary-line > div:last-child {
  border-right: 0;
}
.summary-line span {
  color: var(--c-muted);
  font-size: 11.5px;
}
.summary-line strong {
  font: 600 18px/1.2 var(--font-ui);
}
.summary-line strong.ok {
  color: var(--c-ok);
}
.summary-line strong i {
  margin-left: 1px;
  color: var(--c-muted);
  font-size: 10px;
  font-style: normal;
}
.server-grid {

  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 12px;
  padding: 14px;
}
.server-card {
  display: grid;
  gap: 6px;
  padding: 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  cursor: pointer;
  transition:
    border-color var(--duration-fast),
    transform var(--duration-fast);
}
.server-card:hover {
  border-color: var(--c-focus);
  transform: translateY(-1px);
}
.server-head {
  display: flex;
  align-items: center;
  gap: 8px;
}
.server-head strong {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  font-size: 14px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-muted);
}
.status-dot.is-on {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--c-ok) 18%, transparent);
}
.status-dot.is-off {
  background: var(--c-bad);
}
.server-sub {
  margin: 0 0 4px;
  overflow: hidden;
  color: var(--c-muted);
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.metric-line {
  display: flex;
  justify-content: space-between;
  color: var(--c-muted);
  font-size: 11px;
}
.metric-line b {
  color: var(--c-ink);
  font-weight: 500;
}
.bar {
  height: 4px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--c-surface-2);
}
.bar i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--c-signal);
}
.server-foot {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-top: 4px;
  color: var(--c-muted);
  font-size: 11px;
}
.server-empty {
  margin: 8px 0 0;
  color: var(--c-muted);
  font-size: 12px;
}
.panel-pad {
  padding: 16px;
}
.state-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--c-bad);
  font-size: 13px;
}
.empty-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  margin-bottom: 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-signal) 18%, var(--c-surface-2));
  color: var(--c-signal-text);
}
.empty-icon svg {
  width: 20px;
  height: 20px;
}
.install-success {
  display: grid;
  gap: 14px;
}
.install-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.success-icon {
  display: grid;
  width: 40px;
  height: 40px;
  flex: none;
  place-items: center;
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-ok) 16%, var(--c-surface-2));
  color: var(--c-ok);
}
.success-icon svg {
  width: 20px;
  height: 20px;
}
.install-title {
  display: grid;
  gap: 2px;
}
.install-title strong {
  font-size: 15px;
  font-weight: 600;
}
.install-title small {
  color: var(--c-muted);
  font-size: 12px;
}
.install-steps {
  display: grid;
  gap: 6px;
  color: var(--c-muted);
  font-size: 12.5px;
}
.install-steps > div {
  display: flex;
  align-items: center;
  gap: 8px;
}
.install-steps span {
  display: grid;
  width: 18px;
  height: 18px;
  flex: none;
  place-items: center;
  border-radius: 50%;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 10px;
}
.command-box {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 12px 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface-2);
}
.command-box code {
  min-width: 0;
  flex: 1;
  color: var(--c-ink);
  font: 12px/1.75 var(--font-mono);
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
.copy-chip {
  flex: none;
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
}
.copy-chip:hover {
  border-color: var(--c-focus);
}
.install-hint {
  margin: 0;
  color: var(--c-subtle);
  font-size: 11px;
  line-height: 1.7;
}
.detail-loading {
  padding: 4px 0;
}
.detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.detail-title {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 8px;
}
.detail-title strong {
  font-size: 15px;
  font-weight: 500;
}
.detail-meta {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.detail-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}
.detail-stats {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  gap: 8px;
  margin-bottom: 14px;
}
.detail-stats > div {
  display: grid;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
}
.detail-stats span {
  color: var(--c-muted);
  font-size: 11px;
}
.detail-stats strong {
  font-size: 18px;
  font-weight: 600;
}
.detail-stats small {
  overflow: hidden;
  color: var(--c-subtle);
  font-size: 10px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.range-row {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 8px;
}
.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 18px;
}
.chart-wide {
  grid-column: 1 / -1;
}
.chart-empty {
  padding: 40px 0;
  color: var(--c-muted);
  font-size: 13px;
  text-align: center;
}
@media (max-width: 767px) {
  .summary-line {
    flex-wrap: nowrap;
    overflow-x: auto;
  }
  .summary-line > div {
    flex: none;
  }
  .detail-head {
    align-items: flex-start;
    flex-direction: column;
  }
  .detail-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .chart-grid {
    grid-template-columns: 1fr;
  }
  .server-grid {
    grid-template-columns: 1fr;
  }
}
</style>
