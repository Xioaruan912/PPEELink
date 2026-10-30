<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import DomainRouteFields from "./DomainRouteFields.vue";
import IconShuffle from "~icons/ph/shuffle";
import IconCheck from "~icons/ph/seal-check";
import {
  checkRealityTarget,
  getProxyEgressOptions,
  saveProxyInbound,
  type ProxyInbound,
  type DomainRoute,
  type TargetCheck,
} from "@/api/proxy";
import { UiButton, UiDrawer, UiSegmented } from "@/ui";

const props = defineProps<{
  serverId: number;
  inbound: ProxyInbound | null;
  requireEgress?: boolean;
}>();
const visible = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();

// 常用伪装目标：需支持 TLS 1.3 与 h2，建议选与服务器同地区的站点
const targets = [
  "www.apple.com",
  "dl.google.com",
  "www.amazon.com",
  "www.tesla.com",
  "addons.mozilla.org",
];
const randomPort = () => 20000 + Math.floor(Math.random() * 30000);
const blank = () => ({
  id: 0,
  egressNodeId: 0,
  domainRoutes: [] as DomainRoute[],
  remark: "Reality",
  template: "vless-reality-vision" as ProxyInbound["template"],
  port: randomPort(),
  target: "www.apple.com:443",
  listen: "",
  fingerprint: "chrome",
  spiderX: "/",
  publicKey: "",
  privateKey: "",
  shortId: undefined as string | undefined,
  tlsMode: "self-signed" as "auto" | "manual" | "self-signed",
  tlsCertFile: "",
  tlsKeyFile: "",
  sni: "www.apple.com",
  path: "",
  enabled: true,
  regenerateKey: false,
});
const form = reactive(blank());
const importKey = ref(false);

const identityLocked = computed(() => !!props.requireEgress && form.id > 0);
watch(
  () => form.template,
  (template) => {
    if (template === "hysteria2" && props.inbound?.template !== "hysteria2") {
      if (form.sni === "www.apple.com") form.sni = "";
      if (form.remark === "Reality") form.remark = "Hysteria2";
    }
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
    if (request === exitRequest && visible.value)
      exitOptions.value = response.data;
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
          regenerateKey: false,
        }
      : {}
  );
  if (props.requireEgress && !props.inbound) form.remark = "";
});
function pickTarget(host: string) {
  form.target = `${host}:443`;
  form.sni = host;
  check.value = null;
}
async function runCheck() {
  checking.value = true;
  try {
    check.value = (await checkRealityTarget(form.target, form.sni)).data;
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
  saving.value = true;
  error.value = "";
  try {
    await saveProxyInbound({
      ...form,
      regenerateKey: props.requireEgress ? false : form.regenerateKey,
      privateKey:
        !identityLocked.value && importKey.value ? form.privateKey : undefined,
      tlsMode: form.template === "hysteria2" ? form.tlsMode : "manual",
      ...(identityLocked.value && props.inbound
        ? {
            template: props.inbound.template,
            port: props.inbound.port,
            target: props.inbound.target,
            sni: props.inbound.sni,
            path: props.inbound.path,
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
    size="min(560px, 100vw)"
  >
    <div class="deployment-baseline">
      <strong>x-ui 节点部署</strong>
      <p class="hint">
        Reality 参数与 x-ui 一致。HY2 可由 Agent
        自动生成证书，无需申请域名或上传证书。
      </p>
    </div>
    <p v-if="identityLocked" class="hint identity-hint">
      入口地址与认证保持不变，需要更改入口请新建网关
    </p>
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item label="协议模板">
        <UiSegmented
          v-model="form.template"
          label="协议模板"
          :options="[
            {
              label: 'VLESS Reality / Vision',
              value: 'vless-reality-vision',
              disabled: identityLocked,
            },
            {
              label: 'XHTTP Reality',
              value: 'vless-xhttp-reality',
              disabled: identityLocked,
            },
            {
              label: 'Hysteria2',
              value: 'hysteria2',
              disabled: identityLocked,
            },
          ]"
        />
        <p class="hint">
          {{
            form.template === "hysteria2"
              ? "使用 UDP，支持 x-ui 自签名方式、证书指纹或已有证书。"
              : form.template === "vless-reality-vision"
                ? "兼容性最好，Clash Meta、Shadowrocket、sing-box 等主流客户端都支持。"
                : "XHTTP Reality 使用直连 TLS 伪装，需要支持 XHTTP 的客户端；不用于 CDN TLS 终止。"
          }}
        </p>
      </el-form-item>
      <div class="grid">
        <el-form-item
          :label="props.requireEgress ? '节点名称' : '备注'"
          :required="props.requireEgress"
        >
          <el-input
            v-model="form.remark"
            :placeholder="
              props.requireEgress ? '请输入节点名称' : '显示在节点名里'
            "
          />
        </el-form-item>
        <el-form-item label="端口">
          <div class="port-row">
            <el-input-number
              v-model="form.port"
              :disabled="identityLocked"
              :min="1024"
              :max="65535"
              controls-position="right"
            />
            <UiButton
              variant="ghost"
              :icon="IconShuffle"
              v-if="!identityLocked"
              aria-label="随机端口"
              @click="form.port = randomPort()"
            />
          </div>
        </el-form-item>
      </div>
      <el-form-item
        v-if="form.template !== 'hysteria2'"
        label="伪装目标（Reality target）"
      >
        <el-input
          v-model="form.target"
          :readonly="identityLocked"
          placeholder="www.apple.com:443"
          @input="check = null"
        />
        <div v-if="!identityLocked" class="chips">
          <button
            v-for="h in targets"
            :key="h"
            type="button"
            class="chip"
            :class="{ on: form.target === `${h}:443` }"
            @click="pickTarget(h)"
          >
            {{ h }}
          </button>
        </div>
      </el-form-item>
      <el-form-item v-if="form.template === 'hysteria2'" label="部署方式">
        <UiSegmented
          v-model="form.tlsMode"
          label="部署方式"
          :options="[
            {
              label: '无需证书配置',
              value: 'self-signed',
              disabled: identityLocked,
            },
            {
              label: '证书指纹校验',
              value: 'auto',
              disabled: identityLocked,
            },
            {
              label: '使用已有证书',
              value: 'manual',
              disabled: identityLocked,
            },
          ]"
        />
      </el-form-item>
      <div
        v-if="form.template === 'hysteria2' && form.tlsMode === 'self-signed'"
        class="auto-tls-info"
      >
        <strong>按 x-ui 方式部署 HY2</strong>
        <p class="hint">
          无需域名、无需填写证书路径。Agent
          自动生成并保管证书，订阅和分享链接会明确携带“跳过证书验证”。
        </p>
        <p class="hint">
          可用于支持 HY2 的 Mihomo、sing-box、Loon 等客户端。服务器必须放行所选
          UDP 端口；TLS 仍然启用。
        </p>
      </div>
      <div
        v-if="form.template === 'hysteria2' && form.tlsMode === 'auto'"
        class="auto-tls-info"
      >
        <strong>服务器 IP 直连，无需域名或 Cloudflare</strong>
        <p class="hint">
          Agent 自动生成并保管证书，订阅携带证书指纹。服务器连接地址需填写
          IP，并放行所选 UDP 端口。
        </p>
        <p class="hint">
          推荐 Mihomo / sing-box 完整配置。官方 HY2 分享链接需使用支持 pinSHA256
          的客户端；Surge / Loon
          格式不下发此节点，通用链接导入需确认客户端支持指纹校验。
        </p>
        <p
          v-if="
            props.inbound?.tlsMode === 'auto' && props.inbound?.tlsFingerprint
          "
          class="certificate-state"
        >
          证书已就绪 · SHA-256<br /><code>{{
            props.inbound.tlsFingerprint
          }}</code>
        </p>
        <p v-else class="hint">
          {{
            form.id
              ? "等待 Agent 部署并回传证书，完成后自动加入订阅。"
              : "点击部署后自动加入订阅，无需填写证书路径。"
          }}
        </p>
      </div>
      <el-form-item
        v-if="form.template !== 'hysteria2' || form.tlsMode === 'manual'"
        label="SNI（客户端握手用的域名）"
      >
        <el-input
          v-model="form.sni"
          :readonly="identityLocked"
          :placeholder="
            form.template === 'hysteria2'
              ? '证书对应域名，如 proxy.example.com'
              : '默认与伪装目标相同'
          "
          @input="check = null"
        />
      </el-form-item>
      <div v-if="form.template !== 'hysteria2'" class="check-row">
        <UiButton :icon="IconCheck" :loading="checking" @click="runCheck"
          >检测伪装目标</UiButton
        >
        <Transition name="pop">
          <span
            v-if="check"
            class="check-result"
            :class="check.ok ? (check.h2 ? 'is-ok' : 'is-warn') : 'is-bad'"
          >
            {{ check.message }}
          </span>
        </Transition>
      </div>
      <p v-if="form.template !== 'hysteria2' && !identityLocked" class="hint">
        检测在面板所在机器上进行；同一站点在不同地区表现可能不同，保存后若节点不通，换一个目标再试。
      </p>
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
      <template
        v-if="form.template === 'hysteria2' && form.tlsMode === 'manual'"
      >
        <el-form-item label="证书文件路径" required
          ><el-input
            v-model="form.tlsCertFile"
            :readonly="identityLocked"
            placeholder="/etc/ssl/proxy/fullchain.pem"
        /></el-form-item>
        <el-form-item label="私钥文件路径" required
          ><el-input
            v-model="form.tlsKeyFile"
            :readonly="identityLocked"
            placeholder="/etc/ssl/proxy/privkey.pem"
        /></el-form-item>
        <p class="hint">
          SNI 填写证书对应域名。请先放行所选 UDP 端口；证书内容不会上传到面板。
        </p>
      </template>
      <el-form-item label="监听地址"
        ><el-select
          v-model="form.listen"
          :disabled="identityLocked"
          style="width: 100%"
          ><el-option label="双栈监听（x-ui 默认）" value="" /><el-option
            label="仅 IPv4"
            value="0.0.0.0" /><el-option label="仅 IPv6" value="::" /><el-option
            v-if="form.listen && !['0.0.0.0', '::'].includes(form.listen)"
            :label="form.listen"
            :value="form.listen" /></el-select
      ></el-form-item>
      <details v-if="form.template !== 'hysteria2'" class="reality-advanced">
        <summary>Reality 参数（x-ui）</summary>
        <el-form-item label="客户端指纹"
          ><el-select
            v-model="form.fingerprint"
            :disabled="identityLocked"
            style="width: 100%"
            ><el-option
              v-for="fp in [
                'chrome',
                'firefox',
                'safari',
                'ios',
                'android',
                'edge',
                'randomized',
              ]"
              :key="fp"
              :label="fp"
              :value="fp" /></el-select></el-form-item
        ><el-form-item label="Short ID"
          ><el-input
            v-model="form.shortId"
            :readonly="identityLocked"
            placeholder="新建时自动生成；多个值用逗号分隔"
          />
          <p class="hint">
            每项为偶数位十六进制，最长16位。分享链接使用第一个值；修改后需更新订阅。
          </p></el-form-item
        ><el-form-item label="SpiderX"
          ><el-input
            v-model="form.spiderX"
            :readonly="identityLocked"
            placeholder="/" /></el-form-item
        ><el-form-item v-if="form.publicKey" label="Reality 公钥"
          ><el-input :model-value="form.publicKey" readonly /></el-form-item
        ><template v-if="!identityLocked"
          ><label class="import-key-toggle"
            ><el-switch v-model="importKey" /> 使用已有 x-ui 私钥</label
          ><el-form-item v-if="importKey" label="x-ui Private Key"
            ><el-input
              v-model="form.privateKey"
              type="password"
              show-password
              autocomplete="new-password"
              placeholder="粘贴 x-ui 的 X25519 私钥"
            />
            <p class="hint">
              公钥由私钥自动推导；不会把私钥写入分享链接。
            </p></el-form-item
          ></template
        >
      </details>
      <el-form-item label="默认服务端出口">
        <el-select v-model="form.egressNodeId" filterable style="width: 100%"
          ><el-option
            v-if="!props.requireEgress"
            :value="0"
            label="当前服务器直接访问" /><el-option
            v-for="item in exitOptions"
            :key="item.id"
            :value="item.id"
            :label="item.name"
        /></el-select>
        <p class="hint">
          未命中网站规则的 TCP / UDP 流量使用此出口。支持 Reality TCP / XHTTP 与
          HY2；出口失效时不会改为直连。保存前需升级 Agent。
        </p>
        <p v-if="exitError" class="error">
          {{ exitError }} <UiButton @click="loadExitOptions">重试</UiButton>
        </p>
      </el-form-item>
      <DomainRouteFields
        v-model="form.domainRoutes"
        :exits="exitOptions"
        :default-exit="form.egressNodeId"
      />
      <div class="switches">
        <label><el-switch v-model="form.enabled" /> 启用</label>
        <label
          v-if="
            form.id &&
            form.template !== 'hysteria2' &&
            !props.requireEgress &&
            !importKey
          "
          ><el-switch v-model="form.regenerateKey" />
          重新生成密钥（所有客户端需更新订阅）</label
        >
      </div>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </el-form>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" @click="save">{{
        form.template === "hysteria2" && form.tlsMode !== "manual" && !form.id
          ? "部署 HY2"
          : "保存"
      }}</UiButton>
    </template>
  </UiDrawer>
</template>
<style scoped>
.deployment-baseline {
  padding: 12px 14px;
  margin-bottom: 18px;
  border-radius: 12px;
  background: var(--c-surface-2);
}
.deployment-baseline strong {
  font-size: 14px;
}
.reality-advanced {
  padding: 12px 0;
  margin-bottom: 14px;
  border-block: 1px solid var(--c-line);
}
.reality-advanced summary {
  font-size: 13px;
  cursor: pointer;
  margin-bottom: 14px;
}
.import-key-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  margin-bottom: 12px;
}

.identity-hint {
  padding: 12px 14px;
  margin-bottom: 18px;
  border-radius: 10px;
  background: var(--c-surface-2);
}
.auto-tls-info {
  padding: 14px;
  margin-bottom: 18px;
  background: var(--c-surface-2);
  border-radius: 12px;
}
.auto-tls-info strong {
  font-size: 14px;
}
.certificate-state {
  font-size: 12px;
  margin-top: 10px;
}
.certificate-state code {
  word-break: break-all;
  color: var(--c-muted);
}
.grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
}
.port-row {
  display: flex;
  gap: 6px;
  width: 100%;
}
.port-row :deep(.el-input-number) {
  flex: 1;
}
.hint {
  margin-top: 6px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.chip {
  padding: 3px 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  font-size: 12px;
  color: var(--c-muted);
  transition:
    border-color var(--duration-fast),
    color var(--duration-fast),
    background-color var(--duration-fast);
}
.chip:hover,
.chip.on {
  border-color: var(--c-focus);
  color: var(--c-ink);
  background: var(--c-selected);
}
.check-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}
.check-result {
  padding: 4px 10px;
  border-radius: var(--radius-pill);
  font-size: 13px;
}
.is-ok {
  background: color-mix(in srgb, var(--c-ok) 14%, transparent);
  color: var(--c-ok);
}
.is-warn {
  background: color-mix(in srgb, var(--c-warn) 16%, transparent);
  color: var(--c-warn-ink);
}
.is-bad {
  background: color-mix(in srgb, var(--c-bad) 14%, transparent);
  color: var(--c-bad);
}
.pop-enter-active {
  transition:
    opacity var(--duration-fast),
    transform var(--duration-medium) var(--ease-spring);
}
.pop-enter-from {
  opacity: 0;
  transform: scale(0.85);
}
.switches {
  display: grid;
  gap: 6px;
  margin-top: 14px;
}
.switches label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}
.error {
  margin-top: 12px;
  color: var(--c-bad);
  font-size: 13px;
}
@media (max-width: 599px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
