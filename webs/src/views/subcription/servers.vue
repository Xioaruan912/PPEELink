<script setup lang="ts">
import PageHeader from "@/ui/PageHeader.vue";
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
} from "vue";
import IconPlus from "~icons/ph/plus";
import IconCloud from "~icons/ph/cloud-arrow-up";
import IconTerminal from "~icons/ph/terminal-window";
import IconPencil from "~icons/ph/pencil-simple";
import IconTrash from "~icons/ph/trash";
import IconWarning from "~icons/ph/warning";
import IconPower from "~icons/ph/power";
import IconRefresh from "~icons/ph/arrows-clockwise";
import IconWrench from "~icons/ph/wrench";
import IconDots from "~icons/ph/dots-three";
import {
  deleteProxyInbound,
  deleteProxyServer,
  getProxyTraffic,
  requestAgentUpdate,
  listProxyAccounts,
  listProxyServers,
  resetServerToken,
  runProxyServerUnlockCheck,
  saveProxyInbound,
  saveProxyServer,
  type DailyTraffic,
  type ProxyAccount,
  type ProxyInbound,
  type ProxyServer,
  type ServerUnlockItem,
} from "@/api/proxy";
import {
  EmptyState,
  UiButton,
  UiDialog,
  UiPanel,
  UiSkeleton,
  UiTag,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import SecretLink from "./subs/SecretLink.vue";
import InboundDrawer from "./servers/InboundDrawer.vue";
import AccountDrawer from "./servers/AccountDrawer.vue";
import ServerOpsDrawer from "./servers/ServerOpsDrawer.vue";
import TrafficBars from "./servers/TrafficBars.vue";
import Onboarding from "./servers/Onboarding.vue";
import { formatBytes } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "ProxyServers" });
const servers = ref<ProxyServer[]>([]);
const accounts = ref<ProxyAccount[]>([]);
const traffic = ref<DailyTraffic[]>([]);
const loading = ref(true);
const loadError = ref("");
const root = ref<HTMLElement>();

async function load(quiet = false) {
  if (!quiet) loading.value = true;
  loadError.value = "";
  try {
    const [s, a, t] = await Promise.all([
      listProxyServers(),
      listProxyAccounts(),
      getProxyTraffic(30),
    ]);
    servers.value = s.data || [];
    accounts.value = a.data || [];
    traffic.value = t.data || [];
  } catch (e: any) {
    loadError.value = e?.message || "请检查网络后重试";
  } finally {
    if (!quiet) {
      loading.value = false;
      await nextTick();
    }
  }
}
// 服务器每分钟回报一次，页面每 20 秒静默刷新状态
let timer: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  void load();
  timer = setInterval(() => void load(true), 20000);
});
onBeforeUnmount(() => clearInterval(timer));

const todayTotal = computed(() =>
  servers.value.reduce(
    (s, x) => s + (Number(x.todayUp) || 0) + (Number(x.todayDown) || 0),
    0
  )
);
const onlineCount = computed(
  () =>
    servers.value.filter((s) => s.online || (s.kind === "local" && s.running))
      .length
);
const inboundCount = computed(() =>
  servers.value.reduce((n, s) => n + s.inbounds.length, 0)
);
const todayParts = computed(() => formatBytes(todayTotal.value).split(" "));
// 最近 7 天总流量折线
const spark = computed(() => {
  const totals = new Map<string, number>();
  for (const r of traffic.value)
    totals.set(r.day, (totals.get(r.day) || 0) + r.up + r.down);
  const keys = [...totals.keys()].sort().slice(-7);
  const vals = keys.map((k) => totals.get(k) || 0);
  while (vals.length < 7) vals.unshift(0);
  const max = Math.max(1, ...vals);
  return vals.map((v, i) => `${(i * 70) / 6},${22 - (v / max) * 20}`).join(" ");
});
const openAccounts = computed(
  () => accounts.value.filter((a) => a.enabled).length
);

function status(s: ProxyServer) {
  if (s.removing) return { tone: "warn" as const, text: "正在卸载" };
  if (s.quota?.over)
    return {
      tone: "warn" as const,
      text: `本月流量用完 · ${s.quota.resetAt.slice(5, 10)} 恢复`,
    };
  if (!s.enabled) return { tone: "neutral" as const, text: "已停用" };
  if (s.kind === "agent" && !s.online)
    return {
      tone: "bad" as const,
      text: s.lastSeenAt ? "离线" : "等待 agent 连接",
    };
  if (s.lastError) return { tone: "warn" as const, text: "运行异常" };
  if (!s.inbounds.some((i) => i.enabled))
    return { tone: "neutral" as const, text: "未配置入站" };
  return s.running
    ? { tone: "ok" as const, text: "运行中" }
    : { tone: "warn" as const, text: "未运行" };
}
const hasSpark = computed(() =>
  traffic.value.some((r) => (Number(r.up) || 0) + (Number(r.down) || 0) > 0)
);
const templateLabel = (t: string) =>
  (
    ({
      "vless-reality-vision": "Reality Vision",
      "vless-xhttp-reality": "XHTTP Reality",
      "vless-grpc-reality": "gRPC Reality",
      "vless-ws-tls": "VLESS WS",
      "vmess-ws-tls": "VMess WS",
      "vmess-tcp": "VMess TCP",
      "trojan-reality": "Trojan Reality",
      "trojan-tls": "Trojan TLS",
      "shadowsocks-2022": "SS-2022",
      hysteria2: "Hysteria2",
    }) as Record<string, string>
  )[t] || t;

// ---- 服务器编辑 ----
const serverDialog = ref(false);
const serverForm = reactive({
  id: 0,
  name: "",
  kind: "agent" as "agent",
  publicHost: "",
  enabled: true,
  trafficLimitGb: 0,
  resetDay: 1,
  monitorEnabled: false,
  websiteTrafficEnabled: false,
  autoFirewall: false,
});
const opsDrawer = ref(false);
const opsServer = ref<ProxyServer | null>(null);
function openOps(s: ProxyServer) {
  opsServer.value = s;
  opsDrawer.value = true;
}
const GB = 1024 ** 3;
const QUOTA_PRESETS = [
  { label: "不限", gb: 0 },
  { label: "500 GB", gb: 500 },
  { label: "1 TB", gb: 1024 },
  { label: "2 TB", gb: 2048 },
];
const savingServer = ref(false);
const install = ref<{ name: string; command: string } | null>(null);
const serverDialogTitle = computed(() =>
  serverForm.id ? "编辑代理节点" : "添加代理节点"
);
function openServer(s?: ProxyServer) {
  Object.assign(
    serverForm,
    s
      ? {
          id: s.id,
          name: s.name,
          kind: "agent" as const,
          publicHost: s.publicHost,
          enabled: s.enabled,
          trafficLimitGb: Math.round((s.trafficLimit / GB) * 100) / 100,
          resetDay: s.resetDay || 1,
          monitorEnabled: s.monitorEnabled,
          websiteTrafficEnabled: s.websiteTrafficEnabled ?? false,
          autoFirewall: s.autoFirewall ?? false,
        }
      : {
          id: 0,
          name: "",
          kind: "agent" as const,
          publicHost: "",
          enabled: true,
          trafficLimitGb: 0,
          resetDay: 1,
          monitorEnabled: false,
          websiteTrafficEnabled: false,
          autoFirewall: false,
        }
  );
  serverDialog.value = true;
}
async function submitServer() {
  savingServer.value = true;
  try {
    const { data } = await saveProxyServer({
      ...serverForm,
      id: serverForm.id || undefined,
    });
    serverDialog.value = false;
    if (data?.install)
      install.value = { name: serverForm.name, command: data.install };
    else ElMessage.success("已保存");
    await load(true);
  } finally {
    savingServer.value = false;
  }
}
async function toggleServer(s: ProxyServer) {
  await saveProxyServer({
    id: s.id,
    name: s.name,
    publicHost: s.publicHost,
    enabled: !s.enabled,
    trafficLimitGb: s.trafficLimit / GB,
    resetDay: s.resetDay || 1,
    monitorEnabled: s.monitorEnabled,
    websiteTrafficEnabled: s.websiteTrafficEnabled ?? false,
    autoFirewall: s.autoFirewall ?? false,
  });
  ElMessage.success(
    s.enabled
      ? "已保存停用配置，等待 Agent 应用；以运行状态 / 最近回报为准"
      : "已保存启用配置，等待 Agent 应用；以运行状态 / 最近回报为准"
  );
  await load(true);
}
async function updateAgent(s: ProxyServer) {
  if (s.agentUpdateRequested) {
    await requestAgentUpdate(s.id, true);
    ElMessage.success("已撤销更新请求");
    await load(true);
    return;
  }
  try {
    await ElMessageBox.confirm(
      `将请求 ${s.name} 的 agent 下载面板当前版本，校验后覆盖旧程序并自动重启。更新期间节点可能短暂重连。`,
      "更新 Agent",
      {
        type: "warning",
        confirmButtonText: "下发更新",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  await requestAgentUpdate(s.id);
  ElMessage.success("已下发更新请求，agent 下次心跳时更新");
  await load(true);
}
async function showInstall(s: ProxyServer) {
  try {
    await ElMessageBox.confirm(
      "重新生成安装命令会使旧令牌失效，已安装的 agent 需要用新命令重装。",
      "重置 agent 令牌",
      {
        type: "warning",
        confirmButtonText: "重置",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  const { data } = await resetServerToken(s.id);
  install.value = { name: s.name, command: data.install };
}
const uninstall = ref<{ name: string; command: string } | null>(null);
function removeHint(s: ProxyServer) {
  const common = "入站与「自建」分组中对应的节点会一起删除。";
  if (s.lastSeenAt)
    return `agent 会在一分钟内卸载 xray 和它自己，完成后记录自动移除。${common}`;
  return `这台服务器从未连上面板，将直接删除记录；如果已经执行过安装命令，删除后按提示在服务器上手动卸载。${common}`;
}
async function removeServer(s: ProxyServer, force = false) {
  try {
    await ElMessageBox.confirm(
      force
        ? `agent 没有回应卸载请求时使用：直接删除 ${s.name} 的记录，之后需要在服务器上手动执行卸载命令。`
        : `删除服务器 ${s.name}？${removeHint(s)}`,
      force ? "强制删除" : "删除服务器",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      }
    );
  } catch {
    return;
  }
  const res: any = await deleteProxyServer(s.id, force);
  if (res?.data?.uninstall)
    uninstall.value = { name: s.name, command: res.data.uninstall };
  else ElMessage.success(res?.msg || "已删除");
  await load(true);
}
async function copyUninstall() {
  if (!uninstall.value) return;
  try {
    await navigator.clipboard.writeText(uninstall.value.command);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败，请手动选中复制");
  }
}

// ---- 入站 ----
const inboundDrawer = ref(false);
const inboundServer = ref(0);
const editingInbound = ref<ProxyInbound | null>(null);
function openInbound(s: ProxyServer, inbound?: ProxyInbound) {
  inboundServer.value = s.id;
  editingInbound.value = inbound || null;
  inboundDrawer.value = true;
}
async function toggleInbound(inbound: ProxyInbound) {
  await saveProxyInbound({ ...inbound, enabled: !inbound.enabled });
  await load(true);
}
async function removeInbound(inbound: ProxyInbound) {
  try {
    await ElMessageBox.confirm(
      `删除入站 ${inbound.remark || inbound.port}？`,
      "删除入站",
      {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  await deleteProxyInbound(inbound.id);
  await load(true);
}

// ---- 账户 ----
const accountDrawer = ref(false);
const editingAccount = ref<ProxyAccount | null>(null);
function openAccount(a: ProxyAccount) {
  editingAccount.value = a;
  accountDrawer.value = true;
}
const usageRatio = (a: ProxyAccount) =>
  a.trafficLimit ? Math.min(a.trafficUsed / a.trafficLimit, 1) : 0;

// ---- 原生解锁矩阵 ----
const checkingUnlock = ref(false);
const batchUpdatingAgents = ref(false);
const updatableAgentServers = computed(() =>
  servers.value.filter((s) => {
    if (s.kind !== "agent" || s.removing || !s.online || s.agentUpdateRequested)
      return false;
    if (s.allowAutoUpdate === false) return false;
    if (s.latestAgentVersion && s.agentVersion) {
      return s.agentVersion.replace(/^v/i, "") !== s.latestAgentVersion.replace(/^v/i, "");
    }
    return true;
  })
);
async function updateAllAgents() {
  const targets = [...updatableAgentServers.value];
  if (!targets.length || batchUpdatingAgents.value) return;
  const targetVer = targets[0]?.latestAgentVersion;
  try {
    await ElMessageBox.confirm(
      `将向 ${targets.length} 台在线代理服务器下发 Agent 更新${targetVer ? `（至 v${targetVer}）` : ""}，更新期间节点可能短暂重连。`,
      "一键更新全部 Agent",
      {
        type: "warning",
        confirmButtonText: "立即更新全部",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  batchUpdatingAgents.value = true;
  try {
    let okCount = 0;
    let firstErr = "";
    await Promise.allSettled(
      targets.map(async (s) => {
        try {
          await requestAgentUpdate(s.id, false);
          okCount++;
        } catch (err: any) {
          if (!firstErr) firstErr = err?.message || "下发失败";
        }
      })
    );
    if (okCount === targets.length) {
      ElMessage.success(`已向 ${okCount} 台服务器下发 Agent 更新`);
    } else if (okCount > 0) {
      ElMessage.warning(`已下发 ${okCount}/${targets.length} 台（${firstErr}）`);
    } else {
      ElMessage.error(firstErr || "批量更新下发失败");
    }
    await load(true);
  } finally {
    batchUpdatingAgents.value = false;
  }
}

const shortUnlockLabel: Record<string, string> = {
  chatgpt: "GPT",
  claude: "Claude",
  gemini: "Gemini",
  netflix: "Netflix",
  disney: "Disney+",
  youtube: "YouTube",
  tiktok: "TikTok",
  spotify: "Spotify",
};
function visibleUnlockItems(items?: ServerUnlockItem[]) {
  if (!items?.length) return [];
  return items.filter(
    (it) => it.status === "available" || it.status === "originals"
  );
}
function formatUnlockBadge(it: ServerUnlockItem) {
  const base = shortUnlockLabel[it.key] || it.name;
  if (it.status === "originals") return `${base}自制`;
  return base;
}
async function checkServerUnlock(serverId?: number) {
  if (checkingUnlock.value) return;
  checkingUnlock.value = true;
  try {
    const { data } = await runProxyServerUnlockCheck(
      serverId ? { serverId } : { all: true }
    );
    const okCount = data?.okCount || 0;
    const total = data?.total || 0;
    if (okCount === 0 && total > 0) {
      const firstErr =
        data?.results?.find((r) => !r.ok && r.error)?.error ||
        "请先更新服务器 Agent 至最新版本";
      ElMessage.warning(`0/${total} 台检测成功：${firstErr}`);
    } else if (okCount < total) {
      ElMessage.warning(`解锁检测完成：${okCount}/${total} 台成功`);
    } else {
      ElMessage.success(`解锁检测完成：${okCount}/${total} 台已更新`);
    }
    await load(true);
  } catch (err: any) {
    ElMessage.error(err?.message || "解锁检测失败");
  } finally {
    checkingUnlock.value = false;
  }
}
</script>
<template>
  <div class="servers-page server-experience">
    <header class="servers-masthead" aria-labelledby="servers-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <h1 id="servers-title" class="gallery-title">自建实例</h1>
          <dl v-if="servers.length" class="inline-metrics mono" aria-label="实例概览">
            <div>
              <dt>在线</dt>
              <dd class="is-ok">{{ onlineCount }}/{{ servers.length }}</dd>
            </div>
            <div>
              <dt>入站</dt>
              <dd>{{ inboundCount }}</dd>
            </div>
            <div>
              <dt>今日流量</dt>
              <dd>{{ todayParts[0] }} {{ todayParts[1] }}</dd>
            </div>
          </dl>
        </div>
        <p class="gallery-lead">
          在自有 VPS 上运行 xray-core，按账户发放代理节点并统计流量。
        </p>
      </div>

      <div class="masthead-actions">
        <UiButton variant="primary" :icon="IconPlus" @click="openServer()">
          添加代理节点
        </UiButton>
        <UiButton
          v-if="updatableAgentServers.length"
          variant="ghost"
          :loading="batchUpdatingAgents"
          @click="updateAllAgents()"
        >
          更新全部 Agent ({{ updatableAgentServers.length }})
        </UiButton>
        <UiButton
          v-if="servers.length"
          variant="ghost"
          :loading="checkingUnlock"
          @click="checkServerUnlock()"
        >
          测全服解锁
        </UiButton>
        <UiButton variant="ghost" :icon="IconRefresh" :loading="loading" @click="load()">
          刷新
        </UiButton>
      </div>
    </header>

    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载"
      :description="loadError"
      ><template #action
        ><UiButton @click="load()">重新加载</UiButton></template
      ></EmptyState
    >
    <UiSkeleton v-else-if="loading" :lines="3" height="140px" />
    <Onboarding v-else-if="!servers.length" @add="openServer()" />
    <template v-else>
      <section class="telemetry" data-enter aria-label="概览">
        <div class="tm">
          <span class="tm-label"
            ><i class="pulse" :class="{ live: onlineCount > 0 }"></i
            >在线服务器</span
          >
          <span class="tm-value mono"
            >{{ onlineCount }}<small>/ {{ servers.length }}</small></span
          >
        </div>
        <div class="tm">
          <span class="tm-label">入站</span>
          <span class="tm-value mono">{{ inboundCount }}</span>
        </div>
        <div class="tm">
          <span class="tm-label">开通账户</span>
          <span class="tm-value mono"
            >{{ openAccounts }}<small>/ {{ accounts.length }}</small></span
          >
        </div>
        <div class="tm tm-traffic">
          <span class="tm-label">今日流量</span>
          <div class="tm-traffic-right">
            <svg
              v-if="hasSpark"
              class="spark"
              viewBox="0 0 70 24"
              preserveAspectRatio="none"
              aria-hidden="true"
            >
              <polyline :points="spark" />
            </svg>
            <span class="tm-value mono"
              >{{ todayParts[0] }}<small>{{ todayParts[1] }}</small></span
            >
          </div>
        </div>
      </section>
      <div ref="root" class="server-grid">
        <article
          v-for="(s, index) in servers"
          :key="s.id"
          :style="{ '--server-index': index }"
          class="cell server-card"
          :class="{ 'is-off': !s.enabled }"
        >
          <header class="card-head">
            <span class="kind-icon" :class="`tone-${status(s).tone}`"
              ><IconCloud
            /></span>
            <div class="card-title">
              <h2>{{ s.name }}</h2>
              <p class="mono">{{ s.publicHost }}</p>
            </div>
            <div class="card-tags">
              <UiTag v-if="s.bbr" :tone="s.bbr === 'bbr' ? 'ok' : 'warn'">
                {{ s.bbr.toUpperCase() }}
              </UiTag>
              <UiTag v-if="s.agentUpdateRequested" tone="warn">待更新</UiTag>
              <UiTag :tone="status(s).tone">{{ status(s).text }}</UiTag>
            </div>
          </header>
          <div v-if="s.quota?.limit" class="quota-line">
            <span
              >本月 {{ formatBytes(s.quota.used) }} /
              {{ formatBytes(s.quota.limit) }} ·
              {{ s.quota.resetAt.slice(5, 10) }} 重置</span
            ><span class="quota-bar"
              ><i
                :class="{ hot: s.quota.used / s.quota.limit > 0.9 }"
                :style="{
                  transform: `scaleX(${Math.min(s.quota.used / s.quota.limit, 1)})`,
                }"
              ></i
            ></span>
          </div>
          <dl class="meta">
            <div>
              <dt>Agent</dt>
              <dd
                class="mono"
                :title="
                  s.agentVersion ? `Agent ${s.agentVersion}` : '尚未上报版本'
                "
              >
                {{
                  s.agentVersion || "未上报"
                }}
              </dd>
            </div>
            <div>
              <dt>xray</dt>
              <dd class="mono">{{ s.xrayVersion || "--" }}</dd>
            </div>
            <div>
              <dt>今日</dt>
              <dd class="mono">{{ formatBytes(s.todayUp + s.todayDown) }}</dd>
            </div>
            <div>
              <dt>最近回报</dt>
              <dd>
                {{
                  s.lastSeenAt ? formatDateTime(s.lastSeenAt).slice(5) : "--"
                }}
              </dd>
            </div>
          </dl>
          <div
            v-if="visibleUnlockItems(s.unlock?.items).length"
            class="unlock-strip"
          >
            <span
              v-for="it in visibleUnlockItems(s.unlock?.items)"
              :key="it.key"
              class="unlock-pill"
              :class="it.status === 'available' ? 'is-ok' : 'is-warn'"
            >
              {{ formatUnlockBadge(it) }}
            </span>
          </div>
          <p v-if="s.lastError" class="error-line">
            <IconWarning />{{ s.lastError }}
          </p>
          <ul v-if="!s.removing" class="inbounds">
            <li
              v-for="i in s.inbounds"
              :key="i.id"
              :class="{ 'is-off': !i.enabled }"
            >
              <span class="port mono">:{{ i.port }}</span>
              <span class="in-main">
                <strong>{{ i.remark || templateLabel(i.template) }}</strong>
                <small
                  >{{ templateLabel(i.template)
                  }}<template v-if="i.portHopping">
                    · 跳跃 {{ i.portHopping }}</template
                  >
                  ·
                  {{
                    i.tlsMode === "auto"
                      ? i.tlsFingerprint
                        ? "无域名 · 证书已就绪"
                        : "无域名 · 部署中"
                      : i.sni
                  }}</small
                >
              </span>
              <el-switch
                :model-value="i.enabled"
                size="small"
                :aria-label="`${i.remark} 启用`"
                @change="toggleInbound(i)"
              />
              <RowIconButton
                :icon="IconPencil"
                :label="`编辑入站 ${i.remark}`"
                @click="openInbound(s, i)"
              />
              <RowIconButton
                :icon="IconTrash"
                :label="`删除入站 ${i.remark}`"
                @click="removeInbound(i)"
              />
            </li>
            <li v-if="!s.inbounds.length" class="empty-in">
              还没有入站，添加一个即可开始发放节点。
            </li>
          </ul>
          <footer v-if="s.removing" class="card-actions is-removing">
            <span class="removing-hint"
              >已通知 agent 卸载 xray 与自身，完成后自动移除。</span
            >
            <UiButton variant="ghost" @click="removeServer(s, true)"
              >强制删除</UiButton
            >
          </footer>
          <footer v-else class="card-actions">
            <div class="card-primary-row">
              <UiButton :icon="IconPlus" size="sm" @click="openInbound(s)"
                >添加入站</UiButton
              >
              <UiButton
                v-if="s.kind === 'agent'"
                variant="ghost"
                size="sm"
                :icon="IconWrench"
                @click="openOps(s)"
                >内核运维</UiButton
              >
            </div>
            <el-dropdown trigger="click" placement="bottom-end">
              <button
                type="button"
                class="more-trigger"
                :aria-label="`更多操作 ${s.name}`"
              >
                <IconDots />
              </button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="openServer(s)">
                    <span class="dd-item"><IconPencil />编辑服务器</span>
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-if="s.kind === 'agent' && s.online"
                    @click="checkServerUnlock(s.id)"
                  >
                    <span class="dd-item"><IconRefresh />检测原生解锁</span>
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-if="s.kind === 'agent'"
                    @click="updateAgent(s)"
                  >
                    <span class="dd-item">
                      <IconRefresh />
                      {{
                        s.agentUpdateRequested
                          ? "撤销 Agent 更新"
                          : "更新 Agent"
                      }}
                    </span>
                  </el-dropdown-item>
                  <el-dropdown-item
                    v-if="s.kind === 'agent'"
                    @click="showInstall(s)"
                  >
                    <span class="dd-item"><IconTerminal />重新生成安装命令</span>
                  </el-dropdown-item>
                  <el-dropdown-item @click="toggleServer(s)">
                    <span class="dd-item">
                      <IconPower />
                      {{ s.enabled ? "停用服务器" : "启用服务器" }}
                    </span>
                  </el-dropdown-item>
                  <el-dropdown-item divided @click="removeServer(s)">
                    <span class="dd-item is-danger"><IconTrash />删除服务器</span>
                  </el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </footer>
        </article>
      </div>
    </template>

    <div v-if="!loading && servers.length" class="lower-grid">
      <UiPanel
        title="最近 30 天流量"
        description="全部服务器合计，按服务器回报的实际字节统计。"
      >
        <TrafficBars :rows="traffic" />
      </UiPanel>
      <UiPanel
        title="账户与配额"
        :count="openAccounts"
        description="开通后账户的订阅里会出现自建节点；超额、到期或设备超限会自动停用。"
        flush
      >
        <ul class="accounts">
          <li
            v-for="a in accounts"
            :key="a.userId"
            :class="{ 'is-off': !a.enabled }"
          >
            <span class="avatar" aria-hidden="true">{{
              (a.nickname || a.username).charAt(0).toUpperCase()
            }}</span>
            <span class="acc-main">
              <strong>{{ a.nickname || a.username }}</strong>
              <small v-if="!a.enabled">未开通</small>
              <small v-else-if="a.blockedReason && !a.active" class="warn">{{
                a.blockedReason
              }}</small>
              <small v-else
                >{{ formatBytes(a.trafficUsed)
                }}{{
                  a.trafficLimit ? ` / ${formatBytes(a.trafficLimit)}` : ""
                }}
                · 今日 {{ formatBytes(a.todayBytes)
                }}<template v-if="a.onlineIps">
                  · 在线 {{ a.onlineIps }} IP</template
                ></small
              >
              <span v-if="a.enabled && a.trafficLimit" class="acc-bar"
                ><i
                  :style="{ transform: `scaleX(${usageRatio(a)})` }"
                  :class="{ hot: usageRatio(a) > 0.8 }"
                ></i
              ></span>
            </span>
            <UiButton variant="ghost" @click="openAccount(a)">{{
              a.enabled ? "设置" : "开通"
            }}</UiButton>
          </li>
        </ul>
      </UiPanel>
    </div>

    <UiDialog
      v-model="serverDialog"
      :title="serverDialogTitle"
      width="580px"
      class="node-modal"
    >
      <el-form
        class="proxy-node-form"
        label-position="top"
        @submit.prevent="submitServer"
      >
        <section class="modal-section">
          <header class="modal-section-head">
            <h3>基础连接</h3>
            <p>填写节点识别名称与客户端实际连接的公网入口地址。</p>
          </header>
          <div class="modal-grid-2">
            <el-form-item label="节点名称" required>
              <el-input
                v-model="serverForm.name"
                placeholder="例如：东京主力 01"
              />
            </el-form-item>
            <el-form-item label="代理 IP / 域名" required>
              <el-input
                v-model="serverForm.publicHost"
                class="mono-input"
                placeholder="jp.example.com 或公网 IP"
              />
            </el-form-item>
          </div>
        </section>

        <section class="modal-section">
          <header class="modal-section-head">
            <h3>月度流量配额</h3>
            <p>
              按面板统计的上下行合计；超额后自动暂停下发，并在每月重置日恢复。
            </p>
          </header>

          <div class="quota-presets" role="group" aria-label="快捷流量配额">
            <button
              v-for="preset in QUOTA_PRESETS"
              :key="preset.gb"
              type="button"
              class="quota-preset-pill mono"
              :class="{ 'is-active': serverForm.trafficLimitGb === preset.gb }"
              @click="serverForm.trafficLimitGb = preset.gb"
            >
              {{ preset.label }}
            </button>
          </div>

          <div class="quota-controls">
            <div class="quota-field">
              <span class="quota-field-label">上限（GB）</span>
              <el-input-number
                v-model="serverForm.trafficLimitGb"
                :min="0"
                :step="100"
                controls-position="right"
              />
            </div>
            <div class="quota-field">
              <span class="quota-field-label">重置日（每月）</span>
              <div class="reset-day-wrap">
                <el-input-number
                  v-model="serverForm.resetDay"
                  :min="1"
                  :max="28"
                  controls-position="right"
                />
                <span>号</span>
              </div>
            </div>
          </div>
        </section>

        <section class="modal-section is-last">
          <header class="modal-section-head">
            <h3>附加采集能力</h3>
            <p>复用同一份 Agent 进程，按需开启监控与流量排行。</p>
          </header>

          <div class="capability-toggles">
            <label
              class="capability-row"
              :class="{ 'is-on': serverForm.monitorEnabled }"
            >
              <div class="capability-copy">
                <strong>服务器硬件监控</strong>
                <small>上报 CPU、内存、磁盘与实时网速，无需额外部署监控服务。</small>
              </div>
              <el-switch
                v-model="serverForm.monitorEnabled"
                aria-label="服务器硬件监控"
              />
            </label>

            <label
              class="capability-row"
              :class="{ 'is-on': serverForm.websiteTrafficEnabled }"
            >
              <div class="capability-copy">
                <strong>网站流量统计</strong>
                <small>仅记录代理转发的域名与字节数，不采集完整 URL，默认关闭。</small>
              </div>
              <el-switch
                v-model="serverForm.websiteTrafficEnabled"
                aria-label="网站流量统计"
              />
            </label>

            <label
              class="capability-row"
              :class="{ 'is-on': serverForm.autoFirewall }"
            >
              <div class="capability-copy">
                <strong>自动放行防火墙端口</strong>
                <small>Agent 自动放行入站 TCP/UDP 与跳跃端口，删入站或卸载时自动回收。</small>
              </div>
              <el-switch
                v-model="serverForm.autoFirewall"
                aria-label="自动放行防火墙端口"
              />
            </label>
          </div>
        </section>
      </el-form>

      <template #footer>
        <div class="modal-footer-bar">
          <span class="modal-footer-tip">
            {{ serverForm.id ? "修改将在 Agent 下次心跳生效" : "保存后将生成一次性 Agent 安装命令" }}
          </span>
          <div class="modal-footer-btns">
            <UiButton variant="ghost" @click="serverDialog = false">
              取消
            </UiButton>
            <UiButton
              variant="primary"
              :loading="savingServer"
              @click="submitServer"
            >
              {{ serverForm.id ? "保存修改" : "创建并生成命令" }}
            </UiButton>
          </div>
        </div>
      </template>
    </UiDialog>

    <UiDialog
      :model-value="!!install"
      title="在代理节点上安装 Agent"
      width="620px"
      @update:model-value="install = null"
    >
      <template v-if="install">
        <p class="dialog-hint">
          用 root 登录 <b>{{ install.name }}</b
          >，执行下面的命令。命令里包含令牌，只显示这一次，请勿公开。
        </p>
        <SecretLink :value="install.command" label="安装命令" />
      </template>
      <template #footer
        ><UiButton variant="primary" @click="install = null"
          >完成</UiButton
        ></template
      >
    </UiDialog>

    <ServerOpsDrawer
      v-model="opsDrawer"
      :server="opsServer"
      @refresh="load(true)"
    />

    <UiDialog
      :model-value="!!uninstall"
      title="在服务器上手动卸载"
      width="620px"
      @update:model-value="uninstall = null"
    >
      <template v-if="uninstall">
        <p class="dialog-hint">
          面板里的 <b>{{ uninstall.name }}</b> 已删除。用 root
          登录这台服务器执行下面的命令，停止并删除 agent、xray 及其配置。
        </p>
        <pre class="cmd mono">{{ uninstall.command }}</pre>
      </template>
      <template #footer>
        <UiButton @click="copyUninstall">复制命令</UiButton>
        <UiButton variant="primary" @click="uninstall = null">完成</UiButton>
      </template>
    </UiDialog>

    <InboundDrawer
      v-model="inboundDrawer"
      :server-id="inboundServer"
      :inbound="editingInbound"
      @saved="load(true)"
    />
    <AccountDrawer
      v-model="accountDrawer"
      :account="editingAccount"
      @saved="load(true)"
    />
  </div>
</template>
<style scoped>
.servers-page {
  min-width: 0;
}
.telemetry {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 16px;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.tm {
  position: relative;
  display: grid;
  gap: 4px;
  padding: 12px 16px;
  border-left: 1px solid var(--c-line);
}
.tm:first-child {
  border-left: 0;
}
.tm-label {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--c-muted);
  font-size: 12px;
}
.tm-value {
  font-size: 23px;
  font-weight: 600;
  letter-spacing: -0.03em;
  line-height: 1;
}
.tm-value small {
  margin-left: 4px;
  color: var(--c-muted);
  font-size: 13px;
  font-weight: 400;
  letter-spacing: 0;
}
.pulse {
  position: relative;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
}
.pulse.live {
  background: var(--c-ok);
}
.pulse.live::after {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: var(--c-ok);
  content: "";
  animation: tm-ping 1.8s var(--ease-out) infinite;
}
@keyframes tm-ping {
  to {
    opacity: 0;
    transform: scale(3);
  }
}
.tm-traffic-right {
  display: inline-flex;
  align-items: center;
  gap: 12px;
}
.spark {
  position: static;
  flex: none;
  width: 56px;
  height: 20px;
  overflow: visible;
}
.spark polyline {
  fill: none;
  stroke: var(--c-signal-text);
  stroke-width: 1.6;
  stroke-linecap: round;
  stroke-linejoin: round;
  stroke-dasharray: 200;
  animation: spark-draw 1.4s var(--ease-out) both;
}
@keyframes spark-draw {
  from {
    stroke-dashoffset: 200;
  }
}
.server-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 340px), 1fr));
  gap: 12px;
  margin-bottom: 12px;
}
.server-card {
  display: grid;
  gap: 10px;
  align-content: start;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.server-card.is-off {
  opacity: 0.7;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.kind-icon {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 42px;
  height: 42px;
  border-radius: 12px;
  background: var(--c-surface-2);
  color: var(--c-muted);
}
.kind-icon svg {
  width: 22px;
  height: 22px;
}
.kind-icon.tone-ok {
  background: var(--c-signal);
  color: var(--c-on-signal);
  box-shadow: 0 0 0 6px var(--c-glow);
  animation: breathe 2.8s var(--ease-out) infinite;
}
.kind-icon.tone-bad {
  background: color-mix(in srgb, var(--c-bad) 16%, transparent);
  color: var(--c-bad);
}
@keyframes breathe {
  50% {
    box-shadow: 0 0 0 10px transparent;
  }
}
.card-title {
  flex: 1;
  min-width: 0;
}
.card-title h2 {
  font-size: 17px;
}
.card-title p {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.meta {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 120px), 1fr));
  gap: 8px 16px;
  margin: 0;
}
.meta dt {
  color: var(--c-muted);
  font-size: 12px;
}
.meta dd {
  margin: 2px 0 0;
  font-size: 13px;
}
.unlock-strip {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.unlock-pill {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
  line-height: 1.5;
  letter-spacing: 0.01em;
}
.unlock-pill.is-ok {
  background: color-mix(in srgb, var(--c-ok) 12%, transparent);
  color: var(--c-ok);
}
.unlock-pill.is-warn {
  background: color-mix(in srgb, var(--c-warn) 14%, transparent);
  color: var(--c-warn-ink);
}
.error-line {
  display: flex;
  gap: 6px;
  padding: 8px 10px;
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--c-warn) 12%, transparent);
  color: var(--c-warn-ink);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.error-line svg {
  flex: 0 0 auto;
  width: 16px;
  height: 16px;
}
.inbounds {
  display: grid;
  gap: 6px;
}
.inbounds li {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 6px 6px 10px;
  border-radius: var(--radius-control);
  background: var(--c-canvas);
}
.inbounds li.is-off {
  opacity: 0.55;
}
.port {
  min-width: 56px;
  color: var(--c-signal-text);
  font-size: 13px;
}
.in-main {
  display: grid;
  flex: 1;
  min-width: 0;
}
.in-main strong {
  font-size: 13px;
  font-weight: 600;
}
.in-main small,
.empty-in {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.empty-in {
  padding: 10px !important;
  background: transparent !important;
  white-space: normal;
}
.card-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}
.card-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 12px;
  margin-top: auto;
  border-top: 1px solid var(--c-line);
}
.card-primary-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.more-trigger {
  display: inline-grid;
  place-items: center;
  width: 32px;
  height: 32px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
  transition:
    background 150ms var(--ease-out),
    color 150ms var(--ease-out),
    border-color 150ms var(--ease-out);
}
.more-trigger:hover {
  background: var(--c-surface-2);
  border-color: var(--c-line);
  color: var(--c-ink);
}
.more-trigger svg {
  width: 18px;
  height: 18px;
}
.dd-item {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}
.dd-item svg {
  width: 15px;
  height: 15px;
}
.dd-item.is-danger {
  color: var(--c-bad);
}
.lower-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
  align-items: start;
}
.accounts li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--c-line);
}
.accounts li:last-child {
  border-bottom: 0;
}
.accounts li.is-off {
  opacity: 0.7;
}
.avatar {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  font: 600 13px var(--font-mono);
}
.acc-main {
  display: grid;
  flex: 1;
  gap: 2px;
  min-width: 0;
}
.acc-main strong {
  font-size: 14px;
  font-weight: 600;
}
.acc-main small {
  color: var(--c-muted);
  font-size: 12px;
}
.acc-main small.warn {
  color: var(--c-warn-ink);
}
.acc-bar {
  height: 4px;
  margin-top: 4px;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  overflow: hidden;
}
.acc-bar i {
  display: block;
  height: 100%;
  background: var(--c-ok);
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
.acc-bar i.hot {
  background: var(--c-bad);
}
.card-actions.is-removing {
  justify-content: space-between;
}
.removing-hint {
  color: var(--c-warn-ink);
  font-size: 13px;
}
.cmd {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface-2);
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-all;
}
.quota-fields {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  color: var(--c-muted);
  font-size: 13px;
}
.field-hint {
  margin: 6px 0 0;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.5;
}
.quota-line {
  display: grid;
  gap: 6px;
  color: var(--c-muted);
  font-size: 12px;
}
.quota-bar {
  height: 4px;
  overflow: hidden;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
}
.quota-bar i {
  display: block;
  height: 100%;
  background: var(--c-ok);
  transform-origin: left;
  transition: transform var(--duration-slow) var(--ease-out);
}
.quota-bar i.hot {
  background: var(--c-bad);
}
.monitor-toggle {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.servers-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--c-ink);
  margin-bottom: 20px;
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
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}
.inline-metrics dd.is-ok {
  color: var(--c-ok);
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

/* Refactored Proxy Node Modal */
.proxy-node-form {
  display: grid;
  gap: 20px;
}
.modal-section {
  display: grid;
  gap: 12px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--c-line);
}
.modal-section.is-last {
  padding-bottom: 0;
  border-bottom: 0;
}
.modal-section-head {
  display: grid;
  gap: 3px;
}
.modal-section-head h3 {
  margin: 0;
  font-size: 14px;
  font-weight: 500;
  letter-spacing: -0.015em;
  color: var(--c-ink);
}
.modal-section-head p {
  margin: 0;
  font-size: 12px;
  color: var(--c-muted);
  line-height: 1.5;
}
.modal-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.modal-grid-2 :deep(.el-form-item) {
  margin-bottom: 0;
}
.mono-input :deep(input) {
  font-family: var(--font-mono);
  font-size: 13px;
}
.quota-presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}
.quota-preset-pill {
  padding: 6px 13px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}
.quota-preset-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}
.quota-preset-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}
.quota-controls {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}
.quota-field {
  display: grid;
  gap: 6px;
}
.quota-field-label {
  font-size: 12px;
  color: var(--c-muted);
}
.quota-field :deep(.el-input-number) {
  width: 100%;
}
.reset-day-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--c-muted);
  font-size: 13px;
}
.capability-toggles {
  display: grid;
  gap: 10px;
}
.capability-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 13px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-canvas);
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out);
}
.capability-row:hover {
  border-color: var(--c-ink);
}
.capability-row.is-on {
  border-color: var(--c-ok);
  background: color-mix(in srgb, var(--c-ok) 5%, var(--c-surface));
}
.capability-copy {
  display: grid;
  gap: 3px;
}
.capability-copy strong {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}
.capability-copy small {
  font-size: 12px;
  color: var(--c-muted);
  line-height: 1.5;
}
.modal-footer-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.modal-footer-tip {
  font-size: 12px;
  color: var(--c-muted);
}
.modal-footer-btns {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
@media (max-width: 600px) {
  .modal-grid-2,
  .quota-controls {
    grid-template-columns: 1fr;
  }
}
.dialog-hint {
  margin-bottom: 12px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
}
.mono {
  font-family: var(--font-mono);
}
@media (max-width: 899px) {
  .telemetry {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .tm:nth-child(3) {
    border-left: 0;
  }
  .tm:nth-child(n + 3) {
    border-top: 1px solid var(--c-line);
  }
  .tm {
    padding: 14px 16px;
  }
  .tm-value {
    font-size: 22px;
  }
  .spark {
    right: 14px;
    width: 48px;
  }
  .server-grid,
  .lower-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .meta {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    row-gap: 10px;
  }
  .server-card {
    padding: 16px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .kind-icon.tone-ok,
  .pulse.live::after,
  .spark polyline {
    animation: none;
  }
}
</style>
