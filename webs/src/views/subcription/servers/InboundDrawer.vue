<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import DomainRouteFields from "./DomainRouteFields.vue";
import IconShuffle from "~icons/ph/shuffle";
import IconCheck from "~icons/ph/seal-check";
import {
  checkRealityTarget,
  getProxyEgressOptions,
  saveProxyInbound,
  type InboundTemplate,
  type ProxyInbound,
  type DomainRoute,
  type TargetCheck,
} from "@/api/proxy";
import { UiButton, UiDrawer } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";

const props = defineProps<{
  serverId: number;
  inbound: ProxyInbound | null;
  requireEgress?: boolean;
}>();
const visible = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();

// 精选非 Cloudflare CDN 伪装目标（TLS 1.3 + h2，已排除 CF CDN 防止被用于攻击）
const targets = [
  { host: "www.amd.com", tag: "推荐" },
  { host: "www.apple.com", tag: "" },
  { host: "dl.google.com", tag: "" },
  { host: "www.amazon.com", tag: "" },
  { host: "www.tesla.com", tag: "" },
  { host: "addons.mozilla.org", tag: "" },
];

type ProtoFamily = "vless" | "vmess" | "trojan" | "ss2022" | "hy2";

const protocolFamilies: { key: ProtoFamily; label: string }[] = [
  { key: "vless", label: "VLESS" },
  { key: "vmess", label: "VMess" },
  { key: "trojan", label: "Trojan" },
  { key: "ss2022", label: "SS-2022" },
  { key: "hy2", label: "Hysteria2" },
];

const familyTemplates: Record<
  ProtoFamily,
  { value: InboundTemplate; title: string; desc: string }[]
> = {
  vless: [
    { value: "vless-reality-vision", title: "Reality Vision", desc: "兼容性最佳，主流客户端通用" },
    { value: "vless-xhttp-reality", title: "XHTTP Reality", desc: "直连 XHTTP 伪装" },
    { value: "vless-grpc-reality", title: "gRPC Reality", desc: "多路复用 gRPC 伪装" },
    { value: "vless-ws-tls", title: "WS + TLS", desc: "WebSocket 加密传输" },
  ],
  vmess: [
    { value: "vmess-ws-tls", title: "WS + TLS", desc: "WebSocket + TLS 加密" },
    { value: "vmess-tcp", title: "TCP 直连", desc: "轻量无 TLS，适合内网/中转" },
  ],
  trojan: [
    { value: "trojan-reality", title: "Trojan Reality", desc: "Trojan 协议 + Reality 伪装" },
    { value: "trojan-tls", title: "Trojan TLS", desc: "经典 Trojan + TLS" },
  ],
  ss2022: [
    { value: "shadowsocks-2022", title: "Shadowsocks 2022", desc: "AEAD-2022 现代加密，TCP+UDP" },
  ],
  hy2: [
    { value: "hysteria2", title: "Hysteria2", desc: "UDP QUIC 高速传输，免域名签发" },
  ],
};

function familyForTemplate(t: InboundTemplate): ProtoFamily {
  if (t.startsWith("vless-")) return "vless";
  if (t.startsWith("vmess-")) return "vmess";
  if (t.startsWith("trojan-")) return "trojan";
  if (t === "shadowsocks-2022") return "ss2022";
  return "hy2";
}

const ssMethods = [
  "2022-blake3-aes-128-gcm",
  "2022-blake3-aes-256-gcm",
  "2022-blake3-chacha20-poly1305",
];

const listenOptions = [
  { label: "双栈监听", value: "" },
  { label: "仅 IPv4", value: "0.0.0.0" },
  { label: "仅 IPv6", value: "::" },
];

const fingerprints = [
  "chrome",
  "firefox",
  "safari",
  "ios",
  "android",
  "edge",
  "randomized",
];

const tlsModes: { value: "self-signed" | "auto" | "manual"; label: string }[] = [
  { value: "self-signed", label: "免证书自签" },
  { value: "auto", label: "指纹校验" },
  { value: "manual", label: "已有证书" },
];

const randomPort = () => 20000 + Math.floor(Math.random() * 30000);
const blank = () => ({
  id: 0,
  egressNodeId: 0,
  domainRoutes: [] as DomainRoute[],
  remark: "Reality",
  template: "vless-reality-vision" as InboundTemplate,
  port: randomPort(),
  target: "www.amd.com:443",
  listen: "",
  fingerprint: "chrome",
  spiderX: "/",
  publicKey: "",
  privateKey: "",
  shortId: undefined as string | undefined,
  tlsMode: "self-signed" as "auto" | "manual" | "self-signed",
  tlsCertFile: "",
  tlsKeyFile: "",
  sni: "www.amd.com",
  path: "",
  host: "",
  method: "2022-blake3-aes-128-gcm",
  enabled: true,
  regenerateKey: false,
});

const form = reactive(blank());
const activeFamily = ref<ProtoFamily>("vless");
const importKey = ref(false);
const identityLocked = computed(() => !!props.requireEgress && form.id > 0);

const usesReality = computed(() =>
  ["vless-reality-vision", "vless-xhttp-reality", "vless-grpc-reality", "trojan-reality"].includes(form.template)
);
const usesTLS = computed(() =>
  ["hysteria2", "vless-ws-tls", "vmess-ws-tls", "trojan-tls"].includes(form.template)
);
const usesWS = computed(() =>
  ["vless-ws-tls", "vmess-ws-tls"].includes(form.template)
);

function selectFamily(f: ProtoFamily) {
  if (identityLocked.value) return;
  activeFamily.value = f;
  const list = familyTemplates[f];
  if (!list.some((x) => x.value === form.template)) {
    form.template = list[0].value;
  }
}

watch(
  () => form.template,
  (template) => {
    activeFamily.value = familyForTemplate(template);
    if (!props.inbound) {
      const defaults: Record<InboundTemplate, string> = {
        "vless-reality-vision": "Reality",
        "vless-xhttp-reality": "XHTTP",
        "vless-grpc-reality": "gRPC",
        "vless-ws-tls": "VLESS-WS",
        "vmess-ws-tls": "VMess-WS",
        "vmess-tcp": "VMess",
        "trojan-reality": "Trojan-Reality",
        "trojan-tls": "Trojan",
        "shadowsocks-2022": "SS-2022",
        hysteria2: "Hysteria2",
      };
      if (Object.values(defaults).includes(form.remark) || !form.remark) {
        form.remark = defaults[template];
      }
    }
    if (usesTLS.value && (form.sni === "www.amd.com" || form.sni === "www.apple.com")) {
      form.sni = "";
    }
    if (usesReality.value && !form.target) {
      form.target = "www.amd.com:443";
      form.sni = "www.amd.com";
    }
    if (template === "vless-ws-tls" && !form.path) form.path = "/ws";
    if (template === "vmess-ws-tls" && !form.path) form.path = "/vmess";
    if (template === "vless-grpc-reality" && !form.path) form.path = "grpc";
  }
);

const saving = ref(false);
const exitOptions = ref<{ id: number; name: string }[]>([]);
const exitError = ref("");
let exitRequest = 0;

async function loadExitOptions() {
  const request = ++exitRequest;
  exitError.value = "";
  try {
    const response = await getProxyEgressOptions();
    if (request === exitRequest && visible.value) {
      exitOptions.value = response.data;
    }
  } catch {
    if (request === exitRequest) exitError.value = "出口节点加载失败";
  }
}

const error = ref("");
const check = ref<TargetCheck | null>(null);
const checking = ref(false);

watch(visible, (open) => {
  if (!open) {
    exitRequest++;
    return;
  }
  loadExitOptions();
  error.value = "";
  check.value = null;
  importKey.value = false;

  Object.assign(
    form,
    blank(),
    props.inbound
      ? {
          ...props.inbound,
          domainRoutes: (props.inbound.domainRoutes || []).map((rule) => ({
            ...rule,
          })),
          tlsMode:
            props.inbound.tlsMode === "auto"
              ? "auto"
              : props.inbound.tlsMode === "self-signed"
                ? "self-signed"
                : "manual",
          privateKey: "",
          fingerprint: props.inbound.fingerprint || "chrome",
          spiderX: props.inbound.spiderX || "/",
          host: props.inbound.host || "",
          method: props.inbound.method || "2022-blake3-aes-128-gcm",
          regenerateKey: false,
        }
      : {}
  );
  activeFamily.value = familyForTemplate(form.template);
  if (props.requireEgress && !props.inbound) form.remark = "";
});

function pickTarget(host: string) {
  form.target = `${host}:443`;
  form.sni = host;
  check.value = null;
  error.value = "";
}

async function runCheck() {
  checking.value = true;
  error.value = "";
  try {
    check.value = (await checkRealityTarget(form.target, form.sni)).data;
  } catch (e: any) {
    error.value = e?.message || "检测失败";
  } finally {
    checking.value = false;
  }
}

async function save() {
  if (props.requireEgress && !form.remark.trim()) {
    error.value = "请填写网关节点名称。";
    return;
  }
  if (props.requireEgress && !form.egressNodeId) {
    error.value = "请选择落地出口，固定入口网关不能使用当前服务器直连。";
    return;
  }
  if (usesReality.value && check.value?.cloudflare) {
    error.value = "当前伪装目标使用 Cloudflare CDN，已排除，请更换非 CF 站点（如 www.amd.com）。";
    return;
  }
  saving.value = true;
  error.value = "";
  try {
    await saveProxyInbound({
      ...form,
      regenerateKey: props.requireEgress ? false : form.regenerateKey,
      privateKey:
        !identityLocked.value && importKey.value ? form.privateKey : undefined,
      tlsMode: usesTLS.value ? form.tlsMode : "manual",
      ...(identityLocked.value && props.inbound
        ? {
            template: props.inbound.template,
            port: props.inbound.port,
            target: props.inbound.target,
            sni: props.inbound.sni,
            path: props.inbound.path,
            host: props.inbound.host,
            method: props.inbound.method,
            listen: props.inbound.listen,
            fingerprint: props.inbound.fingerprint,
            spiderX: props.inbound.spiderX,
            shortId: props.inbound.shortId,
            tlsMode: props.inbound.tlsMode,
            tlsCertFile: props.inbound.tlsCertFile,
            tlsKeyFile: props.inbound.tlsKeyFile,
          }
        : {}),
      id: form.id || undefined,
      serverId: props.serverId,
    });
    ElMessage.success("已保存，等待 Agent 应用，以运行状态和最近回报为准");
    visible.value = false;
    emit("saved");
  } catch (e: any) {
    error.value = e?.message || "保存失败";
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <UiDrawer
    v-model="visible"
    :title="
      props.requireEgress
        ? form.id
          ? '修改落地与规则'
          : '添加固定入口网关'
        : form.id
          ? '编辑入站'
          : '添加入站'
    "
    size="min(700px, 100vw)"
  >
    <div class="inbound-editor">
      <p v-if="identityLocked" class="lock-banner">
        入口地址与认证参数已锁定；如需更换端口或协议，请新建网关。
      </p>

      <KineticNotice :raw="error" />

      <el-form label-position="top" class="inbound-form" @submit.prevent="save">
        <!-- 1. 3x-ui Full Protocol Family & Mode Selector -->
        <section class="form-block">
          <header class="block-head">
            <h3 class="block-title">协议与传输（3x-ui 全量）</h3>
          </header>

          <div class="pill-rail" role="tablist" aria-label="协议族">
            <button
              v-for="fam in protocolFamilies"
              :key="fam.key"
              type="button"
              role="tab"
              class="pill-btn"
              :class="{ 'is-active': activeFamily === fam.key }"
              :aria-selected="activeFamily === fam.key"
              :disabled="identityLocked"
              @click="selectFamily(fam.key)"
            >
              {{ fam.label }}
            </button>
          </div>

          <div class="proto-grid" role="radiogroup" aria-label="协议模板">
            <button
              v-for="p in familyTemplates[activeFamily]"
              :key="p.value"
              type="button"
              role="radio"
              class="proto-card"
              :class="{ 'is-active': form.template === p.value }"
              :aria-checked="form.template === p.value"
              :disabled="identityLocked"
              @click="form.template = p.value"
            >
              <strong class="proto-title">{{ p.title }}</strong>
              <span class="proto-desc">{{ p.desc }}</span>
            </button>
          </div>
        </section>

        <!-- 2. Basic Info (Name & Port) -->
        <section class="form-block">
          <div class="two-col">
            <el-form-item
              :label="props.requireEgress ? '节点名称' : '备注名称'"
              :required="props.requireEgress"
            >
              <el-input
                v-model="form.remark"
                :placeholder="
                  props.requireEgress ? '输入网关节点名称' : '显示在节点列表中'
                "
              />
            </el-form-item>

            <el-form-item label="监听端口">
              <div class="port-row">
                <el-input-number
                  v-model="form.port"
                  :disabled="identityLocked"
                  :min="1024"
                  :max="65535"
                  controls-position="right"
                />
                <UiButton
                  v-if="!identityLocked"
                  variant="ghost"
                  :icon="IconShuffle"
                  aria-label="随机端口"
                  @click="form.port = randomPort()"
                />
              </div>
            </el-form-item>
          </div>
        </section>

        <!-- 3A. Reality Camouflage -->
        <section v-if="usesReality" class="form-block">
          <header class="block-head">
            <div>
              <h3 class="block-title">伪装目标（Reality）</h3>
              <p class="block-sub">
                仅选用非 Cloudflare CDN 的 TLS 1.3 站点，避免被用于 CDN 反射攻击
              </p>
            </div>
            <span class="shield-pill">已排除 CF CDN</span>
          </header>

          <div v-if="!identityLocked" class="target-chips">
            <button
              v-for="item in targets"
              :key="item.host"
              type="button"
              class="target-chip"
              :class="{ 'is-active': form.target === `${item.host}:443` }"
              @click="pickTarget(item.host)"
            >
              <span class="mono">{{ item.host }}</span>
              <em v-if="item.tag" class="chip-tag">{{ item.tag }}</em>
            </button>
          </div>

          <div class="two-col">
            <el-form-item label="目标地址（host:443）">
              <el-input
                v-model="form.target"
                :readonly="identityLocked"
                placeholder="www.amd.com:443"
                @input="check = null"
              />
            </el-form-item>

            <el-form-item label="SNI 握手域名">
              <el-input
                v-model="form.sni"
                :readonly="identityLocked"
                placeholder="默认与伪装目标相同"
                @input="check = null"
              />
            </el-form-item>
          </div>

          <el-form-item
            v-if="form.template === 'vless-xhttp-reality'"
            label="XHTTP 路径"
          >
            <el-input
              v-model="form.path"
              :readonly="identityLocked"
              placeholder="留空自动生成随机路径"
            />
          </el-form-item>

          <el-form-item
            v-if="form.template === 'vless-grpc-reality'"
            label="gRPC ServiceName"
          >
            <el-input
              v-model="form.path"
              :readonly="identityLocked"
              placeholder="默认 grpc"
            />
          </el-form-item>

          <div class="check-bar">
            <UiButton
              variant="ghost"
              :icon="IconCheck"
              :loading="checking"
              @click="runCheck"
            >
              检测目标可用性 & CF 校验
            </UiButton>
            <Transition name="pop">
              <span
                v-if="check"
                class="check-pill"
                :class="
                  check.cloudflare
                    ? 'is-bad'
                    : check.ok
                      ? check.h2
                        ? 'is-ok'
                        : 'is-warn'
                      : 'is-bad'
                "
              >
                {{ check.message }}
              </span>
            </Transition>
          </div>
        </section>

        <!-- 3B. TLS Protocols (Hysteria2 / VLESS-WS-TLS / VMess-WS-TLS / Trojan-TLS) -->
        <section v-else-if="usesTLS" class="form-block">
          <header class="block-head">
            <h3 class="block-title">TLS 证书与传输参数</h3>
          </header>

          <div v-if="usesWS" class="two-col">
            <el-form-item label="WebSocket 路径">
              <el-input
                v-model="form.path"
                :readonly="identityLocked"
                placeholder="/ws"
              />
            </el-form-item>
            <el-form-item label="WS Host（可选）">
              <el-input
                v-model="form.host"
                :readonly="identityLocked"
                placeholder="留空默认与 SNI 一致"
              />
            </el-form-item>
          </div>

          <div class="pill-rail" role="radiogroup" aria-label="TLS 部署方式">
            <button
              v-for="m in tlsModes"
              :key="m.value"
              type="button"
              role="radio"
              class="pill-btn"
              :class="{ 'is-active': form.tlsMode === m.value }"
              :aria-checked="form.tlsMode === m.value"
              :disabled="identityLocked"
              @click="form.tlsMode = m.value"
            >
              {{ m.label }}
            </button>
          </div>

          <p v-if="form.tlsMode === 'self-signed'" class="mode-note">
            Agent 自动生成并托管自签证书，无需域名；客户端自动携带跳过证书校验。
          </p>

          <div v-else-if="form.tlsMode === 'auto'" class="mode-note-box">
            <p class="mode-note">
              IP 直连并携带 SHA-256 证书指纹，无需域名或 Cloudflare。
            </p>
            <code
              v-if="
                props.inbound?.tlsMode === 'auto' &&
                props.inbound?.tlsFingerprint
              "
              class="fp-code mono"
            >
              SHA-256: {{ props.inbound.tlsFingerprint }}
            </code>
          </div>

          <template v-else>
            <div class="two-col">
              <el-form-item label="证书文件路径" required>
                <el-input
                  v-model="form.tlsCertFile"
                  :readonly="identityLocked"
                  placeholder="/etc/ssl/proxy/fullchain.pem"
                />
              </el-form-item>
              <el-form-item label="私钥文件路径" required>
                <el-input
                  v-model="form.tlsKeyFile"
                  :readonly="identityLocked"
                  placeholder="/etc/ssl/proxy/privkey.pem"
                />
              </el-form-item>
            </div>
            <el-form-item label="SNI 域名">
              <el-input
                v-model="form.sni"
                :readonly="identityLocked"
                placeholder="证书对应域名，如 proxy.example.com"
              />
            </el-form-item>
          </template>
        </section>

        <!-- 3C. Shadowsocks 2022 -->
        <section v-else-if="form.template === 'shadowsocks-2022'" class="form-block">
          <header class="block-head">
            <h3 class="block-title">Shadowsocks 2022 加密算法</h3>
          </header>
          <div class="pill-rail" role="radiogroup" aria-label="SS-2022 加密算法">
            <button
              v-for="m in ssMethods"
              :key="m"
              type="button"
              role="radio"
              class="pill-btn mono"
              :class="{ 'is-active': form.method === m }"
              :aria-checked="form.method === m"
              :disabled="identityLocked"
              @click="form.method = m"
            >
              {{ m }}
            </button>
          </div>
          <p class="mode-note">
            服务端主密钥与各账户独立密钥均由面板自动派生管理，同时支持 TCP 与 UDP 转发。
          </p>
        </section>

        <!-- 3D. VMess TCP -->
        <section v-else class="form-block">
          <p class="mode-note">
            VMess TCP 直连模式无需证书或伪装目标，适合作为中转落地或专线入口。
          </p>
        </section>

        <!-- 4. Egress & Routing -->
        <section class="form-block has-divider">
          <header class="block-head">
            <h3 class="block-title">出口与分流</h3>
          </header>

          <el-form-item label="默认服务端出口">
            <el-select v-model="form.egressNodeId" filterable class="full">
              <el-option
                v-if="!props.requireEgress"
                :value="0"
                label="当前服务器直接访问"
              />
              <el-option
                v-for="item in exitOptions"
                :key="item.id"
                :value="item.id"
                :label="item.name"
              />
            </el-select>
            <p v-if="exitError" class="err-inline">
              {{ exitError }}
              <button type="button" class="retry-link" @click="loadExitOptions">
                重试
              </button>
            </p>
          </el-form-item>

          <DomainRouteFields
            v-model="form.domainRoutes"
            :exits="exitOptions"
            :default-exit="form.egressNodeId"
          />
        </section>

        <!-- 5. Collapsible Advanced & Listen Settings -->
        <details class="adv-disclosure">
          <summary class="adv-summary">
            <span>高级与监听参数</span>
            <span class="adv-hint mono">
              {{
                form.listen === "0.0.0.0"
                  ? "IPv4"
                  : form.listen === "::"
                    ? "IPv6"
                    : "双栈"
              }}
              <template v-if="usesReality">
                · {{ form.fingerprint }}
              </template>
            </span>
          </summary>

          <div class="adv-body">
            <div class="adv-field">
              <span class="adv-label">监听地址</span>
              <div class="pill-rail">
                <button
                  v-for="opt in listenOptions"
                  :key="opt.value"
                  type="button"
                  class="pill-btn"
                  :class="{ 'is-active': form.listen === opt.value }"
                  :disabled="identityLocked"
                  @click="form.listen = opt.value"
                >
                  {{ opt.label }}
                </button>
              </div>
            </div>

            <template v-if="usesReality">
              <div class="adv-field">
                <span class="adv-label">客户端指纹（uTLS）</span>
                <div class="pill-rail">
                  <button
                    v-for="fp in fingerprints"
                    :key="fp"
                    type="button"
                    class="pill-btn mono"
                    :class="{ 'is-active': form.fingerprint === fp }"
                    :disabled="identityLocked"
                    @click="form.fingerprint = fp"
                  >
                    {{ fp }}
                  </button>
                </div>
              </div>

              <div class="two-col">
                <el-form-item label="Short ID">
                  <el-input
                    v-model="form.shortId"
                    :readonly="identityLocked"
                    placeholder="留空自动生成（逗号分隔多值）"
                  />
                </el-form-item>
                <el-form-item label="SpiderX">
                  <el-input
                    v-model="form.spiderX"
                    :readonly="identityLocked"
                    placeholder="/"
                  />
                </el-form-item>
              </div>

              <el-form-item v-if="form.publicKey" label="Reality 公钥">
                <el-input :model-value="form.publicKey" readonly />
              </el-form-item>

              <template v-if="!identityLocked">
                <label class="switch-inline">
                  <el-switch v-model="importKey" />
                  <span>导入已有 x-ui X25519 私钥</span>
                </label>
                <el-form-item v-if="importKey" label="x-ui Private Key">
                  <el-input
                    v-model="form.privateKey"
                    type="password"
                    show-password
                    autocomplete="new-password"
                    placeholder="粘贴 X25519 私钥（公钥自动推导）"
                  />
                </el-form-item>
              </template>
            </template>
          </div>
        </details>

        <!-- 6. Switches -->
        <div class="bottom-switches">
          <label class="switch-inline">
            <el-switch v-model="form.enabled" />
            <span>启用此入站</span>
          </label>
          <label
            v-if="
              form.id &&
              (usesReality || form.template === 'shadowsocks-2022') &&
              !props.requireEgress &&
              !importKey
            "
            class="switch-inline"
          >
            <el-switch v-model="form.regenerateKey" />
            <span>重新生成密钥（需更新订阅）</span>
          </label>
        </div>
      </el-form>
    </div>

    <template #footer>
      <div class="drawer-footer">
        <UiButton :disabled="saving" @click="visible = false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="save">
          保存入站
        </UiButton>
      </div>
    </template>
  </UiDrawer>
</template>

<style scoped>
.inbound-editor {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-bottom: 16px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

.full,
.inbound-form :deep(.el-select) {
  width: 100%;
}

.lock-banner {
  margin: 0;
  padding: 11px 14px;
  border-radius: 10px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 13px;
  color: var(--c-muted);
}

.inbound-form {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.form-block {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-block.has-divider {
  padding-top: 20px;
  border-top: 1px solid var(--c-line);
}

.block-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.block-title {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.block-sub {
  margin: 3px 0 0;
  font-size: 12.5px;
  color: var(--c-muted);
}

.shield-pill {
  flex: none;
  padding: 3px 9px;
  border-radius: 999px;
  background: color-mix(in srgb, #10b981 12%, transparent);
  color: #059669;
  font-size: 11.5px;
  font-weight: 600;
}

.proto-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 10px;
}

.proto-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 14px;
  border-radius: 12px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition: border-color 0.16s ease, background 0.16s ease;
}

.proto-card:hover:not(:disabled) {
  border-color: color-mix(in srgb, var(--c-ink) 32%, transparent);
}

.proto-card.is-active {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ink) 4%, var(--c-surface));
}

.proto-card:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.proto-title {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.proto-desc {
  font-size: 11.5px;
  line-height: 1.45;
  color: var(--c-muted);
}

.two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}

.port-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.port-row :deep(.el-input-number) {
  flex: 1;
}

.target-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.target-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.16s ease;
}

.target-chip:hover {
  color: var(--c-ink);
  border-color: color-mix(in srgb, var(--c-ink) 32%, transparent);
}

.target-chip.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.chip-tag {
  font-style: normal;
  padding: 1px 6px;
  border-radius: 999px;
  background: color-mix(in srgb, #10b981 18%, transparent);
  color: #059669;
  font-size: 10.5px;
  font-weight: 600;
}

.target-chip.is-active .chip-tag {
  background: rgba(255, 255, 255, 0.2);
  color: var(--c-surface);
}

.check-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.check-pill {
  padding: 5px 11px;
  border-radius: 999px;
  font-size: 12.5px;
  font-weight: 500;
}

.check-pill.is-ok {
  background: color-mix(in srgb, #10b981 14%, transparent);
  color: #059669;
}

.check-pill.is-warn {
  background: color-mix(in srgb, #f59e0b 16%, transparent);
  color: #b45309;
}

.check-pill.is-bad {
  background: color-mix(in srgb, #ef4444 14%, transparent);
  color: #dc2626;
}

.pill-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.pill-btn {
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.16s ease;
}

.pill-btn:hover:not(:disabled) {
  color: var(--c-ink);
  border-color: color-mix(in srgb, var(--c-ink) 30%, transparent);
}

.pill-btn.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.pill-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.mode-note {
  margin: 0;
  font-size: 12.5px;
  line-height: 1.55;
  color: var(--c-muted);
}

.mode-note-box {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.fp-code {
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  font-size: 11.5px;
  word-break: break-all;
  color: var(--c-ink);
}

.adv-disclosure {
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
  padding: 14px 0;
}

.adv-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
  cursor: pointer;
  list-style: none;
}

.adv-summary::-webkit-details-marker {
  display: none;
}

.adv-hint {
  font-size: 12px;
  font-weight: 400;
  color: var(--c-muted);
}

.adv-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-top: 16px;
}

.adv-field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.adv-label {
  font-size: 13px;
  color: var(--c-muted);
}

.bottom-switches {
  display: flex;
  flex-wrap: wrap;
  gap: 12px 24px;
}

.switch-inline {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-size: 13.5px;
  color: var(--c-ink);
}

.err-inline {
  margin: 6px 0 0;
  font-size: 12.5px;
  color: #dc2626;
}

.retry-link {
  margin-left: 8px;
  border: none;
  background: transparent;
  color: var(--c-ink);
  text-decoration: underline;
  cursor: pointer;
}

.drawer-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

.pop-enter-active {
  transition: opacity 0.16s ease, transform 0.22s cubic-bezier(0.22, 1, 0.36, 1);
}

.pop-enter-from {
  opacity: 0;
  transform: scale(0.92);
}

@media (max-width: 640px) {
  .two-col {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
