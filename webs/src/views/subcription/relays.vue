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
import gsap from "gsap";
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
const mode = ref<"port" | "gateway">("port");

const modes = [
  {
    value: "port" as const,
    title: "端口转发",
    badge: "极简透传",
    summary: "像直连网线一样把入口端口原样转发到落地，开销最小。",
    flow: ["设备", "入口端口", "落地节点"],
    traits: ["延迟最低", "保留原协议", "换落地需更新订阅"],
  },
  {
    value: "gateway" as const,
    title: "智能中转网关",
    badge: "固定入口",
    summary: "客户端始终连接固定入口，后台随时切换落地，无需更新订阅。",
    flow: ["设备", "固定入口", "随时换落地"],
    traits: ["换落地免更新", "支持按网站分流", "支持账户配额"],
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

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;
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
  disposed = true;
  loadGeneration++;
  clearInterval(timer);
  gsapCtx?.revert();
  gsapCtx = null;
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
    ? { text: "运行中", tone: "ok" as const }
    : { text: "等待应用", tone: "warn" as const };
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
    ElMessage.success("已下发更新请求");
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
    formError.value = "请填写线路名称，并选择入口服务器与落地节点。";
    return;
  }
  if ((serverById(form.serverId)?.proxyFeatures || 0) < 5) {
    formError.value = "入口 Agent 版本过旧，请先升级。";
    return;
  }
  if (form.listenPort < 1024 || form.listenPort > 65535) {
    formError.value = "监听端口需在 1024 – 65535 之间。";
    return;
  }
  saving.value = true;
  try {
    await saveProxyRelay({ ...form, name: form.name.trim() });
    editor.value = false;
    ElMessage.success("规则已保存，等待 Agent 应用");
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
    ElMessage.success(rule.enabled ? "已停用转发" : "已启用转发");
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
      `删除「${rule.name}」？对应的中转订阅节点也会移除。`,
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
    ElMessage.success("已删除转发规则");
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
    ElMessage.success(inbound.enabled ? "已停用网关" : "已启用网关");
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
      `删除网关「${inbound.remark || inbound.port}」？对应的节点会一起移除。`,
      "删除智能中转网关",
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
    ElMessage.success("已删除中转网关");
    await load(true);
  } catch (e: any) {
    actionError.value = e?.message || "删除失败";
  } finally {
    busy.value = "";
  }
}

function handlePrimaryCreate() {
  if (mode.value === "port") editRule();
  else chooseGateway();
}
</script>

<template>
  <div ref="rootEl" class="relays-gallery">
    <!-- Compact Editorial Masthead -->
    <header class="relays-masthead" aria-labelledby="relays-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <div class="title-mask">
            <h1 id="relays-title" class="gallery-title" data-title-reveal>
              中转网关
            </h1>
          </div>
          <dl class="inline-metrics mono" aria-label="中转线路概览">
            <div>
              <dt>端口转发</dt>
              <dd>{{ loading ? "—" : rules.length }}</dd>
            </div>
            <div>
              <dt>智能网关</dt>
              <dd>{{ loading ? "—" : gateways.length }}</dd>
            </div>
          </dl>
        </div>
      </div>

      <div class="masthead-actions">
        <UiButton
          v-if="mode === 'port'"
          variant="ghost"
          @click="
            router.push({
              path: '/subcription/nodes',
              query: { group: 'Agent 中转' },
            })
          "
        >
          查看中转节点
        </UiButton>
        <UiButton
          variant="primary"
          :icon="IconPlus"
          :disabled="
            mode === 'port'
              ? !capableAgents.length || !targets.length
              : !agents.some((s) => s.enabled)
          "
          @click="handlePrimaryCreate"
        >
          {{ mode === "port" ? "新建端口转发" : "新建智能网关" }}
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="IconRefresh"
          :loading="loading"
          @click="load()"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Clear 2-Mode Visual Comparison Cards -->
    <nav class="mode-compare" aria-label="选择中转模式">
      <button
        v-for="item in modes"
        :key="item.value"
        type="button"
        class="mode-card"
        :class="{ 'is-active': mode === item.value }"
        :aria-pressed="mode === item.value"
        @click="mode = item.value"
      >
        <div class="mode-head">
          <div class="mode-title-group">
            <strong>{{ item.title }}</strong>
            <span class="mode-badge">{{ item.badge }}</span>
          </div>
          <span class="mode-count mono">
            {{ item.value === "port" ? rules.length : gateways.length }} 条
          </span>
        </div>

        <p class="mode-summary">{{ item.summary }}</p>

        <div class="mode-flow mono" aria-hidden="true">
          <span>{{ item.flow[0] }}</span>
          <i>→</i>
          <span>{{ item.flow[1] }}</span>
          <i>→</i>
          <span class="is-end">{{ item.flow[2] }}</span>
        </div>

        <ul class="mode-traits">
          <li v-for="trait in item.traits" :key="trait">{{ trait }}</li>
        </ul>
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
      <template #action>
        <UiButton @click="load()">重新加载</UiButton>
      </template>
    </EmptyState>

    <UiSkeleton v-else-if="loading" :lines="4" height="84px" />

    <!-- Mode 1: Port Forwarding List -->
    <section
      v-else-if="mode === 'port'"
      class="relay-ledger"
      aria-label="端口转发列表"
    >
      <div v-if="olderAgents.length" class="upgrade-strip">
        <span>以下节点需升级 Agent 后才支持端口转发：</span>
        <div class="upgrade-chips">
          <div
            v-for="server in olderAgents"
            :key="server.id"
            class="upgrade-chip"
          >
            <strong>{{ server.name }}</strong>
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="!server.online || server.agentUpdateRequested"
              :loading="busy === `upgrade-${server.id}`"
              @click="upgrade(server)"
            >
              {{ server.agentUpdateRequested ? "升级中" : "升级 Agent" }}
            </UiButton>
          </div>
        </div>
      </div>

      <EmptyState
        v-if="!agents.length"
        size="section"
        title="还没有可用的入口节点"
        description="请先在「自建实例」添加代理节点并连接 Agent。"
      >
        <template #action>
          <UiButton
            variant="primary"
            @click="router.push('/subcription/servers')"
          >
            前往自建实例
          </UiButton>
        </template>
      </EmptyState>

      <EmptyState
        v-else-if="!targets.length"
        size="section"
        title="还没有可用的落地节点"
        description="请先在「节点资产」添加有效的 Reality 或 Hysteria2 节点。"
      >
        <template #action>
          <UiButton variant="primary" @click="router.push('/subcription/nodes')">
            前往节点资产
          </UiButton>
        </template>
      </EmptyState>

      <EmptyState
        v-else-if="!rules.length"
        size="section"
        title="暂无端口转发线路"
        description="将入口端口直接透传至落地节点。"
      >
        <template #action>
          <UiButton variant="primary" :icon="IconPlus" @click="editRule()">
            新建端口转发
          </UiButton>
        </template>
      </EmptyState>

      <div v-else class="relay-list">
        <article v-for="rule in rules" :key="rule.id" class="relay-row">
          <div class="row-main">
            <div class="row-head">
              <h3>{{ rule.name }}</h3>
              <UiTag :tone="ruleStatus(rule).tone">
                {{ ruleStatus(rule).text }}
              </UiTag>
              <span class="proto-pill mono">{{
                rule.network.toUpperCase()
              }}</span>
            </div>

            <div class="route-pipe mono">
              <span>
                {{ serverById(rule.serverId)?.name || "入口已删除" }}
                <em>:{{ rule.listenPort }}</em>
              </span>
              <i aria-hidden="true">→</i>
              <strong>{{ targetName(rule.targetNodeId) }}</strong>
            </div>

            <p v-if="rule.lastError" class="row-error">{{ rule.lastError }}</p>
          </div>

          <dl class="row-stats mono">
            <div>
              <dt>上行 / 下行</dt>
              <dd>
                {{ formatBytes(rule.up || 0) }} /
                {{ formatBytes(rule.down || 0) }}
              </dd>
            </div>
            <div>
              <dt>活跃连接</dt>
              <dd>
                {{ rule.tcpConnections || 0 }} TCP ·
                {{ rule.udpSessions || 0 }} UDP
              </dd>
            </div>
          </dl>

          <div class="row-actions">
            <el-switch
              :model-value="rule.enabled"
              :disabled="busy === `relay-${rule.id}`"
              :aria-label="`${rule.name} 启用`"
              @change="toggleRule(rule)"
            />
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="!rule.nodeId"
              @click="diagnose(rule.nodeId)"
            >
              诊断
            </UiButton>
            <UiButton size="sm" variant="ghost" @click="editRule(rule)">
              编辑
            </UiButton>
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="busy === `relay-${rule.id}`"
              @click="removeRule(rule)"
            >
              <span class="danger">删除</span>
            </UiButton>
          </div>
        </article>
      </div>
    </section>

    <!-- Mode 2: Smart Fixed-Entry Gateway List -->
    <section
      v-else
      class="relay-ledger"
      aria-label="智能中转网关列表"
    >
      <EmptyState
        v-if="!gateways.length"
        size="section"
        title="暂无智能中转网关"
        description="固定客户端入口，后续切换落地或分流规则时无需更新订阅。"
      >
        <template #action>
          <UiButton
            variant="primary"
            :icon="IconPlus"
            :disabled="!agents.some((s) => s.enabled)"
            @click="chooseGateway"
          >
            新建智能网关
          </UiButton>
        </template>
      </EmptyState>

      <div v-else class="relay-list">
        <article
          v-for="{ server, inbound } in gateways"
          :key="inbound.id"
          class="relay-row"
        >
          <div class="row-main">
            <div class="row-head">
              <h3>{{ inbound.remark || "智能中转网关" }}</h3>
              <UiTag :tone="gatewayStatus(server, inbound).tone">
                {{ gatewayStatus(server, inbound).text }}
              </UiTag>
              <span class="proto-pill mono">
                {{
                  inbound.template === "hysteria2"
                    ? "HY2"
                    : inbound.template === "vless-xhttp-reality"
                      ? "XHTTP"
                      : "Reality"
                }}
              </span>
            </div>

            <div class="route-pipe mono">
              <span>
                {{ server.name }}
                <em>:{{ inbound.port }}</em>
              </span>
              <i aria-hidden="true">→</i>
              <strong>{{ targetName(inbound.egressNodeId || 0) }}</strong>
            </div>

            <p v-if="server.lastError" class="row-error">
              {{ server.lastError }}
            </p>
          </div>

          <dl class="row-stats mono">
            <div>
              <dt>网站分流</dt>
              <dd>{{ inbound.domainRoutes?.length || 0 }} 条规则</dd>
            </div>
            <div>
              <dt>最近回报</dt>
              <dd>
                {{
                  server.lastSeenAt
                    ? formatDateTime(server.lastSeenAt).slice(5)
                    : "待回报"
                }}
              </dd>
            </div>
          </dl>

          <div class="row-actions">
            <el-switch
              :model-value="inbound.enabled"
              :disabled="busy === `gateway-${inbound.id}`"
              :aria-label="`${inbound.remark} 启用`"
              @change="toggleGateway(inbound)"
            />
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="!gatewayNodeId(server, inbound)"
              @click="diagnose(gatewayNodeId(server, inbound))"
            >
              诊断
            </UiButton>
            <UiButton
              size="sm"
              variant="ghost"
              @click="openGateway(server.id, inbound)"
            >
              切换落地
            </UiButton>
            <UiButton
              size="sm"
              variant="ghost"
              :disabled="busy === `gateway-${inbound.id}`"
              @click="removeGateway(inbound)"
            >
              <span class="danger">删除</span>
            </UiButton>
          </div>
        </article>
      </div>
    </section>

    <!-- Port Forward Editor Dialog -->
    <UiDialog
      v-model="editor"
      :title="form.id ? '编辑端口转发' : '新建端口转发'"
      width="540px"
    >
      <el-form label-position="top" @submit.prevent="submitRule">
        <el-form-item label="线路名称" required>
          <el-input
            v-model="form.name"
            maxlength="80"
            placeholder="例如：香港入口 → 东京落地"
          />
        </el-form-item>

        <div class="dialog-grid-2">
          <el-form-item label="入口服务器" required>
            <el-select
              v-model="form.serverId"
              :disabled="!!form.id"
              class="full"
              placeholder="选择入口"
            >
              <el-option
                v-for="server in agents"
                :key="server.id"
                :value="server.id"
                :label="`${server.name}${(server.proxyFeatures || 0) < 5 ? '（需升级）' : !server.online ? '（离线）' : ''}`"
                :disabled="!server.enabled || (server.proxyFeatures || 0) < 5"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="入口监听端口" required>
            <el-input-number
              v-model="form.listenPort"
              :min="1024"
              :max="65535"
              controls-position="right"
              class="full"
            />
          </el-form-item>
        </div>

        <el-form-item label="目标落地节点" required>
          <el-select
            v-model="form.targetNodeId"
            filterable
            class="full"
            placeholder="选择 Reality 或 Hysteria2 节点"
          >
            <el-option
              v-for="target in targets"
              :key="target.id"
              :value="target.id"
              :label="`${target.name} · ${target.protocol} / ${target.network.toUpperCase()}`"
            />
          </el-select>
        </el-form-item>

        <label class="enabled-toggle">
          <span>启用此转发规则</span>
          <el-switch v-model="form.enabled" />
        </label>

        <p v-if="formError" class="row-error" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer>
        <UiButton variant="ghost" @click="editor = false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="submitRule">
          保存规则
        </UiButton>
      </template>
    </UiDialog>

    <!-- Smart Gateway Entry Picker -->
    <UiDialog v-model="entryDialog" title="选择智能网关入口" width="460px">
      <el-form label-position="top">
        <el-form-item label="入口服务器" required>
          <el-select
            v-model="entryServer"
            class="full"
            placeholder="选择一台在线的代理节点"
          >
            <el-option
              v-for="server in agents"
              :key="server.id"
              :value="server.id"
              :label="server.name"
              :disabled="!server.enabled"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton variant="ghost" @click="entryDialog = false">取消</UiButton>
        <UiButton
          variant="primary"
          :disabled="!entryServer"
          @click="entryServer && openGateway(entryServer)"
        >
          下一步配置落地
        </UiButton>
      </template>
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
.relays-gallery {
  display: grid;
  gap: 20px;
  max-width: 1180px;
  min-width: 0;
  padding-bottom: 56px;
}

.relays-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.masthead-left {
  display: grid;
  gap: 4px;
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

/* 2-Mode Visual Comparison Cards */
.mode-compare {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.mode-card {
  display: grid;
  gap: 12px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  color: var(--c-ink);
  text-align: left;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-fast) var(--ease-out);
}

.mode-card:hover {
  border-color: var(--c-ink);
}

.mode-card.is-active {
  border-color: var(--c-ink);
  box-shadow: inset 0 0 0 1px var(--c-ink);
}

.mode-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mode-title-group {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.mode-title-group strong {
  font-family: var(--font-display);
  font-size: 17px;
  font-weight: 500;
  letter-spacing: -0.02em;
}

.mode-badge {
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11px;
  font-weight: 500;
}

.mode-card.is-active .mode-badge {
  background: var(--c-ink);
  color: var(--c-surface);
}

.mode-count {
  font-size: 12px;
  color: var(--c-muted);
}

.mode-summary {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--c-muted);
}

.mode-flow {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--c-canvas);
  font-size: 12px;
  color: var(--c-ink);
}

.mode-flow i {
  font-style: normal;
  color: var(--c-muted);
}

.mode-flow .is-end {
  color: var(--c-ok);
  font-weight: 500;
}

.mode-traits {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.mode-traits li {
  padding: 3px 9px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.error-banner {
  margin: 0;
  padding: 12px 16px;
  border: 1px solid rgb(223 41 41 / 0.25);
  border-radius: 12px;
  background: rgb(223 41 41 / 0.06);
  color: var(--c-bad);
  font-size: 13px;
}

.upgrade-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 12px 16px;
  margin-bottom: 14px;
  border: 1px solid rgb(180 83 9 / 0.25);
  border-radius: 12px;
  background: rgb(180 83 9 / 0.05);
  font-size: 13px;
  color: var(--c-warn);
}

.upgrade-chips {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.upgrade-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.relay-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.relay-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 0.9fr) auto;
  align-items: center;
  gap: 20px;
  padding: 20px 8px;
  margin: 0 -8px;
  border-bottom: 1px solid var(--c-line);
  transition: opacity 260ms var(--ease-out);
}

.relay-row::after {
  content: "";
  position: absolute;
  left: 8px;
  right: 8px;
  bottom: -1px;
  height: 1px;
  background: var(--c-ink);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 320ms var(--ease-out);
  pointer-events: none;
}

@media (hover: hover) and (pointer: fine) {
  .relay-list:hover .relay-row:not(:hover) {
    opacity: 0.5;
  }
  .relay-row:hover::after {
    transform: scaleX(1);
  }
}

.row-main {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.row-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.row-head h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  letter-spacing: -0.02em;
  color: var(--c-ink);
}

.proto-pill {
  padding: 2px 7px;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  font-size: 11px;
  color: var(--c-muted);
}

.route-pipe {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  font-size: 12.5px;
  color: var(--c-muted);
}

.route-pipe em {
  font-style: normal;
  color: var(--c-ink);
}

.route-pipe i {
  font-style: normal;
  color: var(--c-ok);
}

.route-pipe strong {
  font-weight: 500;
  color: var(--c-ink);
}

.row-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.row-stats dt {
  font-family: var(--font-ui);
  font-size: 11px;
  color: var(--c-muted);
  margin-bottom: 3px;
}

.row-stats dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-ink);
}

.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.row-error {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-bad);
}

.danger {
  color: var(--c-bad);
}

.dialog-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.full {
  width: 100%;
}

.enabled-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-canvas);
  font-size: 13.5px;
  color: var(--c-ink);
  cursor: pointer;
}

@media (max-width: 900px) {
  .mode-compare,
  .relay-row {
    grid-template-columns: 1fr;
  }
  .dialog-grid-2 {
    grid-template-columns: 1fr;
  }
}
</style>
