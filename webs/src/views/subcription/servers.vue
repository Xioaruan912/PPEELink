<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import IconPlus from "~icons/ph/plus";
import IconServer from "~icons/ph/hard-drives";
import IconCloud from "~icons/ph/cloud-arrow-up";
import IconTerminal from "~icons/ph/terminal-window";
import IconPencil from "~icons/ph/pencil-simple";
import IconTrash from "~icons/ph/trash";
import IconWarning from "~icons/ph/warning";
import IconPower from "~icons/ph/power";
import {
  deleteProxyInbound,
  deleteProxyServer,
  getProxyTraffic,
  listProxyAccounts,
  listProxyServers,
  resetServerToken,
  saveProxyInbound,
  saveProxyServer,
  type DailyTraffic,
  type ProxyAccount,
  type ProxyInbound,
  type ProxyServer,
} from "@/api/proxy";
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
import RowIconButton from "@/ui/RowIconButton.vue";
import { useStagger } from "@/motion";
import SecretLink from "./subs/SecretLink.vue";
import InboundDrawer from "./servers/InboundDrawer.vue";
import AccountDrawer from "./servers/AccountDrawer.vue";
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
const stagger = useStagger();

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
      if (root.value) stagger(root.value.querySelectorAll(".server-card"));
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
  servers.value.reduce((s, x) => s + x.todayUp + x.todayDown, 0)
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
const templateLabel = (t: string) =>
  t === "vless-xhttp-reality" ? "XHTTP Reality" : "Reality Vision";

// ---- 服务器编辑 ----
const serverDialog = ref(false);
const serverForm = reactive({
  id: 0,
  name: "",
  kind: "agent" as "local" | "agent",
  publicHost: "",
  enabled: true,
  trafficLimitGb: 0,
  resetDay: 1,
  monitorEnabled: false,
});
const GB = 1024 ** 3;
const savingServer = ref(false);
const install = ref<{ name: string; command: string } | null>(null);
// 从引导卡片选过类型、或面板服务器已添加过时，对话框不再重复询问类型
const kindLocked = ref(false);
const hasLocal = computed(() => servers.value.some((x) => x.kind === "local"));
const serverDialogTitle = computed(() =>
  serverForm.id
    ? "编辑服务器"
    : !kindLocked.value
      ? "添加服务器"
      : serverForm.kind === "local"
        ? "添加面板服务器"
        : "添加远程服务器"
);
function openServer(s?: ProxyServer, kind?: "local" | "agent") {
  kindLocked.value = !!kind || hasLocal.value;
  Object.assign(
    serverForm,
    s
      ? {
          id: s.id,
          name: s.name,
          kind: s.kind,
          publicHost: s.publicHost,
          enabled: s.enabled,
          trafficLimitGb: Math.round((s.trafficLimit / GB) * 100) / 100,
          resetDay: s.resetDay || 1,
          monitorEnabled: s.monitorEnabled,
        }
      : {
          id: 0,
          name: "",
          kind: kind || (hasLocal.value ? "agent" : "local"),
          // 面板服务器的连接地址通常就是当前访问面板的域名
          publicHost:
            (kind || (hasLocal.value ? "agent" : "local")) === "local"
              ? window.location.hostname
              : "",
          enabled: true,
          trafficLimitGb: 0,
          resetDay: 1,
          monitorEnabled: false,
        }
  );
  serverDialog.value = true;
}
watch(
  () => serverForm.kind,
  (kind) => {
    if (kind === "local" && !serverForm.id && !serverForm.publicHost)
      serverForm.publicHost = window.location.hostname;
  }
);
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
  });
  ElMessage.success(s.enabled ? "已停用，xray 将在一分钟内停止" : "已启用");
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
  const common = `入站与“自建·${s.name}”节点分组会一起删除。`;
  if (s.kind === "local")
    return `将停止并卸载面板服务器上的 xray（systemd 服务、程序与配置）。${common}`;
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
</script>
<template>
  <div class="servers-page">
    <PageHeader
      title="自建节点"
      description="在自己的服务器上运行 xray-core，按账户发放节点、统计流量。"
    >
      <template v-if="servers.length" #actions>
        <UiButton variant="primary" :icon="IconPlus" @click="openServer()"
          >添加服务器</UiButton
        >
      </template>
    </PageHeader>

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
    <Onboarding
      v-else-if="!servers.length"
      @add="openServer(undefined, $event)"
    />
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
          <span class="tm-value mono"
            >{{ todayParts[0] }}<small>{{ todayParts[1] }}</small></span
          >
          <svg
            class="spark"
            viewBox="0 0 70 24"
            preserveAspectRatio="none"
            aria-hidden="true"
          >
            <polyline :points="spark" />
          </svg>
        </div>
      </section>
      <div ref="root" class="server-grid">
        <article
          v-for="s in servers"
          :key="s.id"
          class="cell server-card"
          :class="{ 'is-off': !s.enabled }"
        >
          <header class="card-head">
            <span class="kind-icon" :class="`tone-${status(s).tone}`"
              ><component :is="s.kind === 'local' ? IconServer : IconCloud"
            /></span>
            <div class="card-title">
              <h2>{{ s.name }}</h2>
              <p class="mono">{{ s.publicHost }}</p>
            </div>
            <UiTag :tone="status(s).tone">{{ status(s).text }}</UiTag>
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
              <dt>类型</dt>
              <dd>{{ s.kind === "local" ? "面板服务器" : "远程服务器" }}</dd>
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
                <small>{{ templateLabel(i.template) }} · {{ i.sni }}</small>
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
            <UiButton :icon="IconPlus" @click="openInbound(s)"
              >添加入站</UiButton
            >
            <span class="act-icons">
              <RowIconButton
                :icon="IconPencil"
                :label="`编辑服务器 ${s.name}`"
                @click="openServer(s)"
              />
              <RowIconButton
                v-if="s.kind === 'agent'"
                :icon="IconTerminal"
                :label="`重新生成 ${s.name} 安装命令`"
                @click="showInstall(s)"
              />
              <RowIconButton
                :icon="IconPower"
                :label="`${s.enabled ? '停用' : '启用'}服务器 ${s.name}`"
                :class="{ 'is-on': s.enabled }"
                @click="toggleServer(s)"
              />
              <RowIconButton
                :icon="IconTrash"
                :label="`删除服务器 ${s.name}`"
                @click="removeServer(s)"
              />
            </span>
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

    <UiDialog v-model="serverDialog" :title="serverDialogTitle" width="480px">
      <el-form label-position="top" @submit.prevent="submitServer">
        <el-form-item v-if="!serverForm.id && !kindLocked" label="类型">
          <UiSegmented
            v-model="serverForm.kind"
            label="服务器类型"
            :options="[
              {
                label: '面板服务器',
                value: 'local',
                disabled: servers.some((x) => x.kind === 'local'),
              },
              { label: '远程服务器', value: 'agent' },
            ]"
          />
        </el-form-item>
        <el-form-item label="名称"
          ><el-input v-model="serverForm.name" placeholder="例如 东京 1"
        /></el-form-item>
        <el-form-item label="客户端连接地址（域名或 IP）">
          <el-input
            v-model="serverForm.publicHost"
            placeholder="例如 216.23.84.93"
          />
        </el-form-item>
        <el-form-item label="月流量上限（GB，0 为不限）">
          <div class="quota-fields">
            <el-input-number
              v-model="serverForm.trafficLimitGb"
              :min="0"
              :step="100"
              controls-position="right"
            />
            <span>每月</span>
            <el-input-number
              v-model="serverForm.resetDay"
              :min="1"
              :max="28"
              controls-position="right"
            />
            <span>号重置</span>
          </div>
          <p class="field-hint">
            按面板统计的上下行合计。用完后停止
            xray、节点暂不下发，到重置日自动恢复。
          </p>
        </el-form-item>
        <el-form-item label="服务器监控">
          <div class="monitor-toggle">
            <el-switch v-model="serverForm.monitorEnabled" />
            <span>复用现有 agent 上报 CPU、内存、磁盘与网络，无需再装监控服务。</span>
          </div>
        </el-form-item>
        <p class="dialog-hint">
          {{
            serverForm.kind === "local"
              ? "xray 装在运行面板的这台机器上：面板下载官方 xray-core（校验 SHA-256 后运行），并以 systemd 服务托管，面板重启不影响连接。"
              : "保存后会给出一条安装命令，在远程服务器上以 root 执行即可。agent 主动连回面板，无需开放额外端口或提供 SSH 密码。"
          }}
        </p>
      </el-form>
      <template #footer>
        <UiButton @click="serverDialog = false">取消</UiButton>
        <UiButton
          variant="primary"
          :loading="savingServer"
          @click="submitServer"
          >保存</UiButton
        >
      </template>
    </UiDialog>

    <UiDialog
      :model-value="!!install"
      title="在远程服务器上安装 agent"
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
  gap: 6px;
  padding: 16px 20px;
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
  font-size: 26px;
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
.spark {
  position: absolute;
  right: 18px;
  bottom: 16px;
  width: 70px;
  height: 24px;
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
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 380px), 1fr));
  gap: 14px;
  margin-bottom: 16px;
}
.server-card {
  display: grid;
  gap: 14px;
  align-content: start;
  padding: 18px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background:
    radial-gradient(
      120% 80% at 100% 0%,
      color-mix(in srgb, var(--c-signal) 8%, transparent),
      transparent 55%
    ),
    var(--c-surface);
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
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
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
.card-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  padding-top: 12px;
  border-top: 1px solid var(--c-line);
}
.act-icons {
  display: flex;
  gap: 2px;
  margin-left: auto;
}
.act-icons .is-on {
  color: var(--c-ok);
}
.lower-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 14px;
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
