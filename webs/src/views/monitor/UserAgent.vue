<script setup lang="ts">
import { computed, onActivated, onMounted, ref } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  CopyDocument,
  Delete,
  Key,
  Plus,
  Refresh,
  VideoPlay,
} from "@element-plus/icons-vue";
import {
  EmptyState,
  PageHeader,
  StatusDot,
  UiButton,
  UiDialog,
  UiPanel,
  UiTag,
} from "@/ui";
import CountryFlag from "@/ui/CountryFlag.vue";
import {
  addProbe,
  deleteProbe,
  getProbes,
  resetProbeToken,
  triggerUserProbe,
  type Probe,
  type ProbeCommands,
} from "@/api/subcription/probe";
import { getNodeOverview } from "@/api/subcription/node";

defineOptions({ name: "UserAgent" });

interface NodeRowItem {
  id: number;
  name: string;
  server: string;
  port: number;
  country: string;
  countryCode: string;
  valid: boolean;
  userValid?: boolean;
  userRtt?: number;
  userKind?: string;
  userError?: string;
  userCheckedAt?: string;
  userProbeName?: string;
}

const loading = ref(false);
const probingAll = ref(false);
const probingNodeId = ref<number | null>(null);
const probes = ref<Probe[]>([]);
const nodes = ref<NodeRowItem[]>([]);
const query = ref("");
const statusFilter = ref<"all" | "ok" | "fail" | "untested">("all");

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

const summary = computed(() => {
  let ok = 0;
  let fail = 0;
  let untested = 0;
  for (const n of nodes.value) {
    if (!n.userCheckedAt) untested++;
    else if (n.userValid) ok++;
    else fail++;
  }
  return { total: nodes.value.length, ok, fail, untested };
});

const filteredNodes = computed(() => {
  const q = query.value.trim().toLowerCase();
  return nodes.value.filter((n) => {
    if (statusFilter.value === "ok" && !(n.userCheckedAt && n.userValid))
      return false;
    if (statusFilter.value === "fail" && !(n.userCheckedAt && !n.userValid))
      return false;
    if (statusFilter.value === "untested" && n.userCheckedAt) return false;
    if (!q) return true;
    return `${n.name} ${n.server} ${n.country} ${n.userProbeName || ""}`
      .toLowerCase()
      .includes(q);
  });
});

async function refreshAll() {
  loading.value = true;
  try {
    const [probeRes, nodeRes] = await Promise.all([
      getProbes(),
      getNodeOverview(false),
    ]);
    probes.value = probeRes.data || [];
    nodes.value = (nodeRes.data || []) as NodeRowItem[];
  } catch (err: any) {
    ElMessage.error(err?.message || "加载数据失败");
  } finally {
    loading.value = false;
  }
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
    ElMessage.warning("复制失败，请手动选中复制");
  }
}

async function submitAddProbe() {
  if (!newName.value.trim()) {
    ElMessage.warning("请填写用户 Agent 名称");
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
    await ElMessageBox.confirm(
      `重置「${probe.name}」的令牌后，旧进程将断开并需使用新令牌重新启动。`,
      "重置令牌",
      { type: "warning", confirmButtonText: "重置", cancelButtonText: "取消" }
    );
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

async function handleDeleteProbe(probe: Probe) {
  try {
    await ElMessageBox.confirm(
      `删除用户 Agent「${probe.name}」及其历史实测记录？`,
      "删除用户 Agent",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
    await deleteProbe(probe.id);
    ElMessage.success("已删除");
    await refreshAll();
  } catch (err: any) {
    if (err?.message) ElMessage.error(err.message);
  }
}

async function runTestAll(probeId?: number) {
  if (!hasOnlineProbe.value) {
    ElMessage.warning("请先在你的电脑或软路由上启动用户 Agent");
    return;
  }
  probingAll.value = true;
  try {
    const res = (await triggerUserProbe(probeId ? { probeId } : {})) as any;
    ElMessage.success(res?.msg || "本机实测完成");
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "触发实测失败");
  } finally {
    probingAll.value = false;
  }
}

async function runTestOne(node: NodeRowItem) {
  if (!hasOnlineProbe.value) {
    ElMessage.warning("请先在你的电脑或软路由上启动用户 Agent");
    return;
  }
  probingNodeId.value = node.id;
  try {
    const res = (await triggerUserProbe({ nodeIds: [node.id] })) as any;
    ElMessage.success(res?.msg || `「${node.name}」实测完成`);
    await refreshAll();
  } catch (err: any) {
    ElMessage.error(err?.message || "实测失败");
  } finally {
    probingNodeId.value = null;
  }
}

function formatTime(raw?: string | null) {
  if (!raw) return "从未";
  const d = new Date(raw);
  if (Number.isNaN(d.getTime())) return raw;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

onMounted(refreshAll);
onActivated(refreshAll);
</script>

<template>
  <div class="user-agent-page">
    <PageHeader
      title="用户 Agent"
      description="运行在你的本机或家庭软路由上的极简按需探针：仅在你点击实测时发起一次探测，空闲时不发包，真实反映用户主机到各节点服务器的连通性与延迟。"
    >
      <template #actions>
        <UiButton variant="ghost" :icon="Refresh" :loading="loading" @click="refreshAll">
          刷新
        </UiButton>
        <UiButton variant="secondary" :icon="Plus" @click="addDialogVisible = true">
          添加用户 Agent
        </UiButton>
        <UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="probingAll"
          :disabled="!hasOnlineProbe || !nodes.length"
          @click="runTestAll()"
        >
          本机一键实测
        </UiButton>
      </template>
    </PageHeader>

    <!-- 1. 用户 Agent 设备列表 -->
    <UiPanel
      title="已配置的用户主机"
      :count="`${onlineProbes.length} / ${probes.length} 在线`"
      description="独立 userAgent 分支单文件客户端（支持 Windows / macOS / Linux），空闲时仅保持轻量心跳"
    >
      <div v-if="probes.length" class="probe-grid">
        <article
          v-for="p in probes"
          :key="p.id"
          class="probe-card"
          :class="{ 'is-online': p.online }"
        >
          <div class="probe-top">
            <div class="probe-title">
              <StatusDot
                :state="p.online ? 'ok' : 'idle'"
                :label="p.online ? '在线 · 待命中' : '离线'"
              />
              <strong>{{ p.name }}</strong>
              <UiTag v-if="p.region" tone="neutral">{{ p.region }}</UiTag>
              <span v-if="p.version" class="mono ver-tag">v{{ p.version }}</span>
            </div>
            <div class="probe-ops">
              <UiButton
                size="sm"
                variant="primary"
                :disabled="!p.online || probingAll"
                :loading="probingAll"
                @click="runTestAll(p.id)"
              >
                实测全部
              </UiButton>
              <UiButton
                size="sm"
                variant="ghost"
                :icon="Key"
                @click="handleResetToken(p)"
              >
                重置令牌
              </UiButton>
              <UiButton
                size="sm"
                variant="danger"
                :icon="Delete"
                @click="handleDeleteProbe(p)"
              >
                删除
              </UiButton>
            </div>
          </div>
          <p class="probe-meta mono">
            最近心跳：{{ formatTime(p.lastSeenAt) }}
          </p>
        </article>
      </div>
      <EmptyState
        v-else
        size="section"
        title="尚未添加用户 Agent"
        description="创建一个用户 Agent 并在你的电脑或软路由运行，即可直接从本机实测到各个节点服务器的真实延迟。"
      >
        <template #action>
          <UiButton variant="primary" :icon="Plus" @click="addDialogVisible = true">
            立即添加用户 Agent
          </UiButton>
        </template>
      </EmptyState>
    </UiPanel>

    <!-- 2. 用户主机 -> 节点服务器实测列表 -->
    <UiPanel
      title="用户主机 → 节点服务器实测"
      :count="`${summary.ok} 可达 · ${summary.fail} 不可达 · ${summary.untested} 未实测`"
      description="支持 TCP（VLESS / VMess / Trojan / SS / Mieru）与 UDP QUIC（Hysteria2 / TUIC）实测"
    >
      <template #actions>
        <div class="table-controls">
          <div class="filter-pills" role="group" aria-label="按状态筛选">
            <button
              type="button"
              class="filter-pill"
              :class="{ 'is-active': statusFilter === 'all' }"
              @click="statusFilter = 'all'"
            >
              全部 {{ summary.total }}
            </button>
            <button
              type="button"
              class="filter-pill"
              :class="{ 'is-active': statusFilter === 'ok' }"
              @click="statusFilter = 'ok'"
            >
              可达 {{ summary.ok }}
            </button>
            <button
              type="button"
              class="filter-pill"
              :class="{ 'is-active': statusFilter === 'fail' }"
              @click="statusFilter = 'fail'"
            >
              不可达 {{ summary.fail }}
            </button>
            <button
              type="button"
              class="filter-pill"
              :class="{ 'is-active': statusFilter === 'untested' }"
              @click="statusFilter = 'untested'"
            >
              未实测 {{ summary.untested }}
            </button>
          </div>
          <input
            v-model="query"
            type="search"
            class="search-input"
            placeholder="搜索节点名、主机或地区…"
          />
        </div>
      </template>

      <div v-if="filteredNodes.length" class="node-probe-list">
        <div
          v-for="item in filteredNodes"
          :key="item.id"
          class="node-probe-row"
        >
          <div class="col-node">
            <CountryFlag :code="item.countryCode" :label="item.country" />
            <div class="node-copy">
              <strong>{{ item.name }}</strong>
              <small class="mono">{{ item.server }}:{{ item.port }}</small>
            </div>
          </div>

          <div class="col-kind">
            <UiTag v-if="item.userKind" tone="neutral">
              {{ item.userKind.toUpperCase() }}
            </UiTag>
            <span v-else class="muted-dash">—</span>
          </div>

          <div class="col-result">
            <template v-if="item.userCheckedAt">
              <span
                class="rtt-pill mono"
                :class="item.userValid ? 'is-ok' : 'is-bad'"
              >
                {{
                  item.userValid
                    ? `${item.userRtt ?? 0} ms`
                    : item.userError || "连接失败"
                }}
              </span>
              <small class="tested-by">
                {{ item.userProbeName || "用户 Agent" }} ·
                {{ formatTime(item.userCheckedAt) }}
              </small>
            </template>
            <span v-else class="untested-text">未实测</span>
          </div>

          <div class="col-act">
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="!hasOnlineProbe || probingAll || probingNodeId === item.id"
              :loading="probingNodeId === item.id"
              @click="runTestOne(item)"
            >
              单点实测
            </UiButton>
          </div>
        </div>
      </div>

      <EmptyState
        v-else
        size="section"
        title="没有匹配的节点"
        description="调整筛选条件，或先在「自建实例 / 节点资产」中添加节点。"
      />
    </UiPanel>

    <!-- 新建用户 Agent 弹窗 -->
    <UiDialog v-model="addDialogVisible" title="添加用户 Agent" width="460px">
      <div class="form-stack">
        <label class="field-item">
          <span>设备名称</span>
          <input
            v-model="newName"
            type="text"
            class="text-input"
            placeholder="例如：家里 Windows、MacBook、软路由"
            @keydown.enter.prevent="submitAddProbe"
          />
        </label>
        <label class="field-item">
          <span>网络位置 / 运营商（可选）</span>
          <input
            v-model="newRegion"
            type="text"
            class="text-input"
            placeholder="例如：上海电信、广州联通"
            @keydown.enter.prevent="submitAddProbe"
          />
        </label>
        <p class="form-hint">
          用户 Agent 仅在你手动点击「本机一键实测」或「单点实测」时发包，空闲时零节点流量。
        </p>
      </div>
      <template #footer>
        <UiButton @click="addDialogVisible = false">取消</UiButton>
        <UiButton variant="primary" :loading="creating" @click="submitAddProbe">
          创建并获取启动命令
        </UiButton>
      </template>
    </UiDialog>

    <!-- 启动命令与下载弹窗 -->
    <UiDialog
      v-model="setupDialogVisible"
      :title="`启动用户 Agent · ${setupTitle}`"
      width="680px"
    >
      <div class="setup-stack">
        <p class="setup-warn">
          令牌仅展示这一次，请复制并保存。运行后回到本页即可看到「在线 · 待命中」。
        </p>

        <div class="os-tabs" role="tablist">
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'windows' }"
            @click="setupOS = 'windows'"
          >
            Windows (PowerShell)
          </button>
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'macos' }"
            @click="setupOS = 'macos'"
          >
            macOS (Apple Silicon)
          </button>
          <button
            type="button"
            class="os-tab"
            :class="{ 'is-active': setupOS === 'linux' }"
            @click="setupOS = 'linux'"
          >
            Linux (amd64)
          </button>
        </div>

        <pre class="cmd-box mono">{{ setupCommands[setupOS] }}</pre>

        <div class="setup-actions">
          <UiButton
            variant="primary"
            :icon="CopyDocument"
            @click="copyText(setupCommands[setupOS], '已复制启动命令')"
          >
            复制一键启动命令
          </UiButton>
          <UiButton
            variant="ghost"
            @click="copyText(setupToken, '已复制令牌')"
          >
            仅复制令牌
          </UiButton>
        </div>
      </div>
      <template #footer>
        <UiButton variant="primary" @click="setupDialogVisible = false">
          我已复制
        </UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.user-agent-page {
  display: grid;
  gap: 18px;
}

.probe-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 12px;
}

.probe-card {
  display: grid;
  gap: 10px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
}

.probe-card.is-online {
  border-color: color-mix(in srgb, var(--c-ok) 45%, var(--c-line));
}

.probe-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.probe-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.probe-title strong {
  font-size: 14px;
  font-weight: 600;
}

.ver-tag {
  font-size: 11px;
  color: var(--c-muted);
}

.probe-ops {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.probe-meta {
  margin: 0;
  font-size: 11.5px;
  color: var(--c-muted);
}

.table-controls {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.filter-pills {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.filter-pill {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.filter-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.search-input {
  height: 32px;
  width: 210px;
  max-width: 100%;
  box-sizing: border-box;
  padding: 0 10px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12.5px;
  outline: none;
}

.search-input:focus {
  border-color: var(--c-ink);
}

.node-probe-list {
  display: grid;
  gap: 6px;
}

.node-probe-row {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) 72px minmax(0, 1.1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}

.col-node {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.node-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.node-copy strong {
  font-size: 13.5px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.node-copy small {
  font-size: 11.5px;
  color: var(--c-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.col-result {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.rtt-pill {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
}

.rtt-pill.is-ok {
  background: rgba(21, 184, 90, 0.14);
  color: var(--c-ok);
}

.rtt-pill.is-bad {
  background: rgba(239, 68, 68, 0.12);
  color: var(--c-bad);
}

.tested-by,
.untested-text,
.muted-dash {
  font-size: 11.5px;
  color: var(--c-muted);
}

.form-stack,
.setup-stack {
  display: grid;
  gap: 12px;
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
  height: 36px;
  padding: 0 11px;
  border: 1px solid var(--c-line);
  border-radius: 9px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 13px;
  outline: none;
}

.text-input:focus {
  border-color: var(--c-ink);
}

.form-hint,
.setup-warn {
  margin: 0;
  font-size: 12px;
  color: var(--c-muted);
  line-height: 1.6;
}

.os-tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.os-tab {
  padding: 5px 12px;
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
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}

.setup-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .probe-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .node-probe-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
