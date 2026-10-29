<script setup lang="ts">
import {
  computed,
  onBeforeUnmount,
  onMounted,
  reactive,
  ref,
  watch,
} from "vue";
import { useRouter } from "vue-router";
import ServerHeader from "@/components/servers/ServerHeader.vue";
import InboundDrawer from "./servers/InboundDrawer.vue";
import { getNodes } from "@/api/subcription/node";
import {
  listProxyServers,
  listProxyRelays,
  listRelayTargets,
  saveProxyRelay,
  deleteProxyRelay,
  deleteProxyInbound,
  saveProxyInbound,
  requestAgentUpdate,
  getProxyEgressOptions,
  type ProxyServer,
  type ProxyInbound,
  type ProxyRelay,
  type RelayTarget,
  type RelaySave,
} from "@/api/proxy";
import { UiButton, UiDialog, UiTag, UiSkeleton, EmptyState } from "@/ui";
import { formatBytes } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";
import IconPlus from "~icons/ph/plus";
import IconRefresh from "~icons/ph/arrows-clockwise";

defineOptions({ name: "ProxyRelays" });
const router = useRouter();
const mode = ref("port");
const modes = [
  {
    value: "port",
    title: "纯端口转发",
    text: "入口只转发 TCP / UDP，保留落地的协议与凭据。",
  },
  {
    value: "gateway",
    title: "固定入口网关",
    text: "入口凭据保持不变，在面板切换落地出口。",
  },
];
const servers = ref<ProxyServer[]>([]);
const rules = ref<ProxyRelay[]>([]);
const targets = ref<RelayTarget[]>([]);
const egressOptions = ref<{ id: number; name: string }[]>([]);
const nodes = ref<{ ID: number; Name: string; Link: string }[]>([]);
const loading = ref(true);
const loadError = ref("");
const actionError = ref("");
const busy = ref("");
let timer: ReturnType<typeof setInterval> | undefined;
let loadGeneration = 0;
let disposed = false;
let loadingRequest = false;
async function load(quiet = false) {
  if (quiet && loadingRequest) return;
  const generation = ++loadGeneration;
  loadingRequest = true;
  if (!quiet) loading.value = true;
  try {
    const [s, r, t] = await Promise.all([
      listProxyServers(),
      listProxyRelays(),
      listRelayTargets(),
    ]);
    if (disposed || generation !== loadGeneration) return;
    servers.value = s.data || [];
    rules.value = r.data || [];
    targets.value = t.data || [];
    loadError.value = "";
    const extra = await Promise.allSettled([
      getNodes(true),
      getProxyEgressOptions(),
    ]);
    if (disposed || generation !== loadGeneration) return;
    if (extra[0].status === "fulfilled")
      nodes.value = extra[0].value.data || [];
    if (extra[1].status === "fulfilled")
      egressOptions.value = extra[1].value.data || [];
  } catch (e: any) {
    if (!disposed && generation === loadGeneration)
      loadError.value = e?.message || "无法加载中转设置，请重试";
  } finally {
    if (!disposed && generation === loadGeneration) {
      loading.value = false;
      loadingRequest = false;
    }
  }
}
onMounted(() => {
  void load();
  timer = setInterval(() => {
    if (!document.hidden) void load(true);
  }, 20000);
});
onBeforeUnmount(() => {
  disposed = true;
  loadGeneration++;
  clearInterval(timer);
});
const agents = computed(() =>
  servers.value.filter((s) => s.kind === "agent" && !s.removing)
);
const olderAgents = computed(() =>
  agents.value.filter((s) => (s.proxyFeatures || 0) < 5)
);
const capableAgents = computed(() =>
  agents.value.filter((s) => s.enabled && (s.proxyFeatures || 0) >= 5)
);
const gateways = computed(() =>
  servers.value.flatMap((server) =>
    (server.inbounds || [])
      .filter((inbound) => (inbound.egressNodeId || 0) > 0)
      .map((inbound) => ({ server, inbound }))
  )
);
const serverById = (id: number) => servers.value.find((s) => s.id === id);
const targetName = (id: number) =>
  targets.value.find((t) => t.id === id)?.name ||
  egressOptions.value.find((t) => t.id === id)?.name ||
  "落地节点已删除";
function ruleStatus(rule: ProxyRelay) {
  const server = serverById(rule.serverId);
  if (!server?.online) return { text: "入口离线", tone: "neutral" as const };
  if (!rule.enabled)
    return rule.running
      ? { text: "等待停止", tone: "warn" as const }
      : { text: "已停用", tone: "neutral" as const };
  if (rule.lastError) return { text: "应用异常", tone: "bad" as const };
  const confirmed = !!rule.lastSeenAt && rule.revision === rule.appliedRevision;
  if (!confirmed) return { text: "等待应用", tone: "warn" as const };
  return rule.running
    ? { text: "运行中", tone: "ok" as const }
    : { text: "等待运行", tone: "warn" as const };
}
function gatewayStatus(server: ProxyServer, inbound: ProxyInbound) {
  if (!server.online && server.kind === "agent")
    return { text: "入口离线", tone: "neutral" as const };
  if (server.lastError) return { text: "运行异常", tone: "bad" as const };
  if (!inbound.enabled || !server.enabled)
    return { text: "已停用", tone: "neutral" as const };
  return server.running
    ? { text: "服务器运行中", tone: "ok" as const }
    : { text: "等待服务器应用", tone: "warn" as const };
}
function diagnose(id: number) {
  if (id)
    void router.push({
      path: "/subcription/diagnostic",
      query: { id: String(id) },
    });
}
function gatewayNodeId(server: ProxyServer, inbound: ProxyInbound) {
  const host = server.publicHost.replace(/^\[|\]$/g, "").toLowerCase();
  return (
    nodes.value.find((node) => {
      try {
        const link = new URL(node.Link);
        return (
          link.hostname.replace(/^\[|\]$/g, "").toLowerCase() === host &&
          Number(link.port) === inbound.port
        );
      } catch {
        return false;
      }
    })?.ID || 0
  );
}
async function upgrade(server: ProxyServer) {
  try {
    await ElMessageBox.confirm(
      `请求 ${server.name} 更新 Agent，更新期间已有连接可能短暂重连。`,
      "更新 Agent",
      {
        confirmButtonText: "下发更新",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
  } catch {
    return;
  }
  busy.value = `upgrade-${server.id}`;
  try {
    await requestAgentUpdate(server.id);
    ElMessage.success("已下发更新请求，等待 Agent 心跳与能力回报");
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "更新请求失败";
  } finally {
    busy.value = "";
  }
}

const editor = ref(false);
const formError = ref("");
const saving = ref(false);
const blank = (): RelaySave => ({
  serverId: 0,
  name: "",
  listenPort: 20000,
  network: "tcp",
  targetNodeId: 0,
  enabled: true,
});
const form = reactive<RelaySave>(blank());
const selectedTarget = computed(() =>
  targets.value.find((t) => t.id === form.targetNodeId)
);
watch(
  () => form.targetNodeId,
  () => {
    if (selectedTarget.value) form.network = selectedTarget.value.network;
  }
);
function editRule(rule?: ProxyRelay) {
  Object.assign(
    form,
    blank(),
    rule
      ? {
          id: rule.id,
          serverId: rule.serverId,
          name: rule.name,
          listenPort: rule.listenPort,
          network: rule.network,
          targetNodeId: rule.targetNodeId,
          enabled: rule.enabled,
        }
      : { id: undefined, serverId: capableAgents.value[0]?.id || 0 }
  );
  formError.value = "";
  editor.value = true;
}
async function submitRule() {
  formError.value = "";
  if (!form.name.trim() || !form.serverId || !form.targetNodeId) {
    formError.value = "请填写线路名称，并选择入口 Agent 和物理落地节点。";
    return;
  }
  if ((serverById(form.serverId)?.proxyFeatures || 0) < 5) {
    formError.value = "入口 Agent 尚未具备端口转发能力，请先升级并等待回报。";
    return;
  }
  if (form.listenPort < 1024 || form.listenPort > 65535) {
    formError.value = "监听端口需在 1024 到 65535 之间。";
    return;
  }
  saving.value = true;
  try {
    await saveProxyRelay({ ...form, name: form.name.trim() });
    editor.value = false;
    ElMessage.success("规则已保存，等待 Agent 应用与回报");
    await load(true);
  } catch (e: any) {
    formError.value = e?.message || "保存失败";
  } finally {
    saving.value = false;
  }
}
async function toggleRule(rule: ProxyRelay) {
  busy.value = `relay-${rule.id}`;
  actionError.value = "";
  try {
    await saveProxyRelay({
      id: rule.id,
      serverId: rule.serverId,
      name: rule.name,
      listenPort: rule.listenPort,
      network: rule.network,
      targetNodeId: rule.targetNodeId,
      enabled: !rule.enabled,
    });
    ElMessage.success("已保存，等待 Agent 应用与回报");
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "保存失败";
  } finally {
    busy.value = "";
  }
}
async function removeRule(rule: ProxyRelay) {
  try {
    await ElMessageBox.confirm(
      `删除「${rule.name}」？该入口转发和对应订阅节点将移除。`,
      "删除端口转发",
      {
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
  } catch {
    return;
  }
  busy.value = `relay-${rule.id}`;
  try {
    await deleteProxyRelay(rule.id);
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "删除失败";
  } finally {
    busy.value = "";
  }
}
const entryDialog = ref(false);
const entryServer = ref<number>();
const inboundDrawer = ref(false);
const inboundServer = ref(0);
const editingInbound = ref<ProxyInbound | null>(null);
function chooseGateway() {
  entryServer.value = agents.value.find((s) => s.enabled)?.id;
  entryDialog.value = true;
}
function openGateway(serverId: number, inbound: ProxyInbound | null = null) {
  inboundServer.value = serverId;
  editingInbound.value = inbound;
  entryDialog.value = false;
  inboundDrawer.value = true;
}
async function toggleGateway(inbound: ProxyInbound) {
  busy.value = `gateway-${inbound.id}`;
  try {
    await saveProxyInbound({
      ...inbound,
      enabled: !inbound.enabled,
      regenerateKey: false,
    });
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "保存失败";
  } finally {
    busy.value = "";
  }
}
async function removeGateway(inbound: ProxyInbound) {
  try {
    await ElMessageBox.confirm(
      `删除网关「${inbound.remark || inbound.port}」？对应的自建节点会一起移除。`,
      "删除固定入口网关",
      {
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
  } catch {
    return;
  }
  busy.value = `gateway-${inbound.id}`;
  try {
    await deleteProxyInbound(inbound.id);
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "删除失败";
  } finally {
    busy.value = "";
  }
}
</script>

<template>
  <div class="relays-page server-experience">
    <ServerHeader
      section="relay"
      title="中转设置"
      description="按需选择连接方式，设备连接入口，网站看到落地出口。"
    >
      <template #actions
        ><UiButton :icon="IconRefresh" :loading="loading" @click="load()"
          >刷新状态</UiButton
        ></template
      >
    </ServerHeader>
    <nav class="mode-picker" aria-label="中转方式">
      <button
        v-for="item in modes"
        :key="item.value"
        type="button"
        class="mode-option"
        :class="{ active: mode === item.value }"
        :aria-pressed="mode === item.value"
        @click="mode = item.value"
      >
        <strong>{{ item.title }}</strong
        ><span>{{ item.text }}</span>
      </button>
    </nav>
    <p v-if="actionError" class="error-banner" role="alert">
      {{ actionError }}
    </p>
    <EmptyState
      v-if="loadError"
      title="无法加载中转设置"
      :description="loadError"
    >
      <template #action><UiButton @click="load()">重新加载</UiButton></template>
    </EmptyState>
    <UiSkeleton v-else-if="loading" :lines="4" height="100px" />
    <template v-else>
      <section
        v-if="mode === 'port'"
        class="relay-section"
        aria-labelledby="port-title"
      >
        <header class="section-heading">
          <div>
            <h2 id="port-title">
              纯端口转发 <small>{{ rules.length }} 条线路</small>
            </h2>
            <p>
              设备 → 入口 Agent 固定端口 → 物理落地。客户端无需设置链式代理。
            </p>
          </div>
          <UiButton
            @click="router.push({ path: '/subcription/nodes', query: { group: 'Agent 中转' } })"
            >查看中转节点</UiButton
          >
          <UiButton
            variant="primary"
            :icon="IconPlus"
            :disabled="!capableAgents.length || !targets.length"
            @click="editRule()"
            >添加转发</UiButton
          >
        </header>
        <div class="explain">
          <p>
            订阅使用入口地址，并保留落地的协议、认证和证书校验。更换凭据不同的落地后，需要更新客户端订阅。
          </p>
          <p>
            线路流量为原始转发字节，无法区分账户或网站。
          </p>
        </div>
        <div v-if="olderAgents.length" class="upgrade-list">
          <p>部分 Agent 需要升级后才能添加端口转发；以新能力回报为准。</p>
          <div
            v-for="server in olderAgents"
            :key="server.id"
            class="upgrade-row"
          >
            <span
              >{{ server.name }} · {{ server.online ? "在线" : "离线" }}</span
            >
            <UiButton
              :disabled="!server.online || server.agentUpdateRequested"
              :loading="busy === `upgrade-${server.id}`"
              @click="upgrade(server)"
              >{{
                server.agentUpdateRequested ? "等待更新回报" : "更新 Agent"
              }}</UiButton
            >
          </div>
        </div>
        <p v-if="!agents.length" class="empty-note">
          先在「自建节点」添加入口服务器并连接 Agent，即可配置服务端转发。
        </p>
        <p v-else-if="!targets.length" class="empty-note">
          暂无可用物理落地。请先在节点列表添加有效的 Reality 或 HY2 节点。
        </p>
        <p v-else-if="!rules.length" class="empty-note">
          暂无转发线路。选择入口与落地后，保存的规则将等待 Agent 应用。
        </p>
        <div class="line-grid">
          <article v-for="rule in rules" :key="rule.id" class="line-card">
            <header class="line-heading">
              <h3>{{ rule.name }}</h3>
              <UiTag :tone="ruleStatus(rule).tone">{{
                ruleStatus(rule).text
              }}</UiTag>
            </header>
            <div class="path">
              <div>
                <span>入口 · {{ rule.network.toUpperCase() }}</span
                ><strong>{{
                  serverById(rule.serverId)?.name || "入口已删除"
                }}</strong
                ><small
                  >{{ serverById(rule.serverId)?.publicHost }}:{{
                    rule.listenPort
                  }}</small
                >
              </div>
              <b aria-hidden="true">→</b>
              <div>
                <span>物理落地</span
                ><strong>{{ targetName(rule.targetNodeId) }}</strong>
              </div>
            </div>
            <dl class="line-stats">
              <div>
                <dt>上行</dt>
                <dd>{{ formatBytes(rule.up || 0) }}</dd>
              </div>
              <div>
                <dt>下行</dt>
                <dd>{{ formatBytes(rule.down || 0) }}</dd>
              </div>
              <div>
                <dt>TCP 连接 / UDP 会话</dt>
                <dd>
                  {{ rule.tcpConnections || 0 }} / {{ rule.udpSessions || 0 }}
                </dd>
              </div>
            </dl>
            <p class="last-report">
              最近回报：{{
                rule.lastSeenAt ? formatDateTime(rule.lastSeenAt) : "尚未回报"
              }}
            </p>
            <p v-if="rule.lastError" class="line-error">{{ rule.lastError }}</p>
            <footer class="line-actions">
              <el-switch
                :model-value="rule.enabled"
                :disabled="busy === `relay-${rule.id}`"
                :aria-label="`${rule.name} 启用`"
                @change="toggleRule(rule)"
              /><UiButton
                :disabled="!rule.nodeId"
                @click="diagnose(rule.nodeId)"
                >诊断入口</UiButton
              ><UiButton @click="editRule(rule)">修改</UiButton
              ><UiButton
                :disabled="busy === `relay-${rule.id}`"
                @click="removeRule(rule)"
                >删除</UiButton
              >
            </footer>
          </article>
        </div>
        <p class="footnote">
          保存不代表已运行。等待 Agent
          的运行与版本回报；修改落地仅影响新连接，已有连接按原路径继续。诊断沿生成的入口节点实际访问目标。
        </p>
      </section>
      <section
        v-else-if="mode === 'gateway'"
        class="relay-section"
        aria-labelledby="gateway-title"
      >
        <header class="section-heading">
          <div>
            <h2 id="gateway-title">
              固定入口网关 <small>{{ gateways.length }} 条线路</small>
            </h2>
            <p>
              设备 → 固定入口入站 → Reality / HY2
              落地。沿用自建节点的账户、配额与网站分流。
            </p>
          </div>
          <UiButton
            variant="primary"
            :icon="IconPlus"
            :disabled="!agents.some((s) => s.enabled)"
            @click="chooseGateway"
            >添加网关</UiButton
          >
        </header>
        <div class="explain">
          <p>
            首次需要把香港等入口节点导入客户端一次；之后仅在面板修改默认服务端出口，入口的密钥、短标识、账户凭据和自动证书身份保持不变。
          </p>
          <p>
            账户用量沿用「自建节点」统计；此处不把服务器总流量当作单条网关流量。
          </p>
        </div>
        <p v-if="!gateways.length" class="empty-note">
          暂无固定入口网关。先选择入口服务器，再配置入站协议和落地出口。
        </p>
        <div class="line-grid">
          <article
            v-for="{ server, inbound } in gateways"
            :key="inbound.id"
            class="line-card"
          >
            <header class="line-heading">
              <h3>{{ inbound.remark || "固定入口网关" }}</h3>
              <UiTag :tone="gatewayStatus(server, inbound).tone">{{
                gatewayStatus(server, inbound).text
              }}</UiTag>
            </header>
            <div class="path">
              <div>
                <span>固定入口</span><strong>{{ server.name }}</strong
                ><small>{{ server.publicHost }}:{{ inbound.port }}</small>
              </div>
              <b aria-hidden="true">→</b>
              <div>
                <span>默认落地</span
                ><strong>{{ targetName(inbound.egressNodeId || 0) }}</strong>
              </div>
            </div>
            <p class="last-report">
              {{
                inbound.template === "hysteria2"
                  ? "Hysteria2"
                  : inbound.template === "vless-xhttp-reality"
                    ? "XHTTP Reality"
                    : "Reality Vision"
              }}
              · {{ inbound.domainRoutes?.length || 0 }} 条网站分流 · 最近回报
              {{
                server.lastSeenAt
                  ? formatDateTime(server.lastSeenAt)
                  : "尚未回报"
              }}
            </p>
            <p v-if="server.lastError" class="line-error">
              {{ server.lastError }}
            </p>
            <p v-if="!gatewayNodeId(server, inbound)" class="last-report">
              入口节点尚未生成；请确认已开通账户并等待 Agent 部署。
            </p>
            <footer class="line-actions">
              <el-switch
                :model-value="inbound.enabled"
                :disabled="busy === `gateway-${inbound.id}`"
                :aria-label="`${inbound.remark} 启用`"
                @change="toggleGateway(inbound)"
              /><UiButton
                :disabled="!gatewayNodeId(server, inbound)"
                @click="diagnose(gatewayNodeId(server, inbound))"
                >诊断入口</UiButton
              ><UiButton @click="openGateway(server.id, inbound)"
                >修改落地与规则</UiButton
              ><UiButton
                :disabled="busy === `gateway-${inbound.id}`"
                @click="removeGateway(inbound)"
                >删除</UiButton
              >
            </footer>
          </article>
        </div>
        <p class="footnote">
          修改落地时保留入口身份。节点是否可用请运行入口诊断，服务器运行状态不等同于线路连通性通过。
        </p>
      </section>
    </template>

    <UiDialog
      v-model="editor"
      :title="form.id ? '修改端口转发' : '添加端口转发'"
      width="540px"
    >
      <el-form label-position="top" @submit.prevent="submitRule">
        <el-form-item label="线路名称" required
          ><el-input
            v-model="form.name"
            maxlength="80"
            placeholder="例如 香港入口 → 东京落地"
        /></el-form-item>
        <el-form-item label="入口 Agent" required
          ><el-select
            v-model="form.serverId"
            :disabled="!!form.id"
            class="full"
            placeholder="选择入口服务器"
            ><el-option
              v-for="server in agents"
              :key="server.id"
              :value="server.id"
              :label="`${server.name}${(server.proxyFeatures || 0) < 5 ? ' · 需要升级 Agent' : !server.enabled ? ' · 已停用' : !server.online ? ' · 离线' : ''}`"
              :disabled="!server.enabled || (server.proxyFeatures || 0) < 5"
          /></el-select>
          <p v-if="form.id" class="field-note">更换入口请新建线路</p>
        </el-form-item>
        <el-form-item label="入口监听端口" required
          ><el-input-number
            v-model="form.listenPort"
            :min="1024"
            :max="65535"
            controls-position="right"
            class="full"
          />
          <p class="field-note">
            请先放行对应端口，不能与入口已有入站冲突。
          </p></el-form-item
        >
        <el-form-item label="物理落地节点" required
          ><el-select
            v-model="form.targetNodeId"
            filterable
            class="full"
            placeholder="选择 Reality 或 HY2 节点"
            ><el-option
              v-for="target in targets"
              :key="target.id"
              :value="target.id"
              :label="`${target.name} · ${target.protocol} / ${target.network.toUpperCase()}`" /></el-select
        ></el-form-item>
        <p class="field-note">
          传输：{{
            selectedTarget
              ? selectedTarget.network.toUpperCase()
              : "由落地协议确定"
          }}。只使用该节点的原始物理连接；不会沿用客户端中转关系。
        </p>
        <label class="enabled-label"
          ><el-switch v-model="form.enabled" />启用转发</label
        >
        <p v-if="formError" class="line-error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer
        ><UiButton @click="editor = false">取消</UiButton
        ><UiButton variant="primary" :loading="saving" @click="submitRule"
          >保存，等待应用</UiButton
        ></template
      >
    </UiDialog>
    <UiDialog v-model="entryDialog" title="选择固定入口服务器" width="480px">
      <el-form label-position="top"
        ><el-form-item label="入口 Agent"
          ><el-select
            v-model="entryServer"
            class="full"
            placeholder="选择入口服务器"
            ><el-option
              v-for="server in agents"
              :key="server.id"
              :value="server.id"
              :label="server.name"
              :disabled="!server.enabled" /></el-select></el-form-item
      ></el-form>
      <p class="field-note">
        下一步配置入口入站与落地出口。后续修改落地时，客户端继续使用同一入口。
      </p>
      <template #footer
        ><UiButton @click="entryDialog = false">取消</UiButton
        ><UiButton
          variant="primary"
          :disabled="!entryServer"
          @click="entryServer && openGateway(entryServer)"
          >配置网关</UiButton
        ></template
      >
    </UiDialog>
    <InboundDrawer
      v-model="inboundDrawer"
      :server-id="inboundServer"
      :inbound="editingInbound"
      require-egress
      @saved="load(true)"
    />
  </div>
</template>

<style scoped>
.relays-page {
  min-width: 0;
}
.mode-picker {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 22px 0;
}
.mode-option {
  min-width: 0;
  text-align: left;
  padding: 18px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
  display: grid;
  gap: 8px;
}
.mode-option strong {
  color: var(--c-ink);
  font-size: 15px;
}
.mode-option span {
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.7;
}
.mode-option.active {
  border-color: var(--c-focus);
  background: var(--c-selected);
  box-shadow: inset 0 0 0 1px var(--c-focus);
}
.mode-option.active strong {
  color: var(--c-signal-text);
}
.mode-option:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 3px;
}
.relay-section {
  min-width: 0;
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}
.section-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 20px;
}
.section-heading h2 {
  margin: 0 0 8px;
  font-size: 21px;
  font-weight: 600;
}
.section-heading h2 small {
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 400;
  margin-left: 8px;
}
.section-heading p,
.explain,
.footnote,
.empty-note,
.field-note {
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.8;
}
.section-heading p {
  margin: 0;
}
.explain {
  padding: 14px 16px;
  background: var(--c-canvas);
  border-radius: 10px;
  margin-bottom: 20px;
}
.explain p {
  margin: 0;
}
.explain p + p {
  margin-top: 5px;
}
.empty-note {
  padding: 24px 0;
  margin: 0;
}
.upgrade-list {
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  margin: 16px 0;
}
.upgrade-list > p {
  margin: 0 0 8px;
  color: var(--c-muted);
  font-size: 12px;
}
.upgrade-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-block: 5px;
  font-size: 13px;
}
.line-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 390px), 1fr));
  gap: 14px;
}
.line-card {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}
.line-heading {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  justify-content: space-between;
  margin-bottom: 18px;
}
.line-heading h3 {
  margin: 0;
  font-size: 16px;
  overflow-wrap: anywhere;
}
.path {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 18px minmax(0, 1fr);
  gap: 12px;
  align-items: center;
}
.path > div {
  min-width: 0;
  display: grid;
  gap: 5px;
}
.path span,
.path small {
  color: var(--c-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.path strong {
  font-size: 14px;
  overflow-wrap: anywhere;
  font-weight: 600;
}
.path b {
  color: var(--c-muted);
  font-weight: 400;
}
.line-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 24px;
  margin: 18px 0 12px;
  padding-top: 14px;
  border-top: 1px solid var(--c-line);
}
.line-stats dt {
  color: var(--c-muted);
  font-size: 11px;
}
.line-stats dd {
  margin: 4px 0 0;
  font-size: 13px;
  font-family: var(--font-mono);
}
.last-report {
  color: var(--c-muted);
  font-size: 11px;
  margin-top: 12px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.line-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--c-line);
  padding-top: 14px;
  margin-top: 16px;
}
.line-actions :deep(.el-switch) {
  margin-right: auto;
}
.line-error,
.error-banner {
  color: var(--c-bad);
  font-size: 13px;
  line-height: 1.7;
  overflow-wrap: anywhere;
}
.error-banner {
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
}
.footnote {
  margin: 18px 0 0;
  font-size: 12px;
}
.full {
  width: 100%;
}
.field-note {
  margin: 8px 0 0;
  font-size: 12px;
}
.enabled-label {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 18px;
  font-size: 13px;
}
@media (max-width: 767px) {
  .mode-picker {
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
    margin: 16px 0;
  }
  .mode-option {
    padding: 14px 16px;
    gap: 4px;
  }
  .relay-section {
    padding: 18px;
  }
  .section-heading {
    flex-direction: column;
  }
  .section-heading > :deep(button) {
    width: 100%;
  }
  .line-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .line-card {
    padding: 14px;
  }
  .path {
    grid-template-columns: minmax(0, 1fr);
    gap: 8px;
  }
  .path b {
    transform: rotate(90deg);
    justify-self: start;
  }
  .upgrade-row {
    flex-wrap: wrap;
  }
}
</style>
