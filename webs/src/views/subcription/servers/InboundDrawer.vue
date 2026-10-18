<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import IconShuffle from "~icons/ph/shuffle";
import IconCheck from "~icons/ph/seal-check";
import {
  checkRealityTarget,
  saveProxyInbound,
  type ProxyInbound,
  type TargetCheck,
} from "@/api/proxy";
import { UiButton, UiDrawer, UiSegmented } from "@/ui";

const props = defineProps<{ serverId: number; inbound: ProxyInbound | null }>();
const visible = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();

// 常用伪装目标：需支持 TLS 1.3 与 h2，建议选与服务器同地区、延迟低的站点
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
  remark: "Reality",
  template: "vless-reality-vision" as ProxyInbound["template"],
  port: randomPort(),
  target: "www.apple.com:443",
  sni: "www.apple.com",
  path: "",
  enabled: true,
  regenerateKey: false,
});
const form = reactive(blank());
const saving = ref(false);
const error = ref("");
const check = ref<TargetCheck | null>(null);
const checking = ref(false);
watch(visible, (open) => {
  if (!open) return;
  error.value = "";
  check.value = null;
  Object.assign(
    form,
    blank(),
    props.inbound ? { ...props.inbound, regenerateKey: false } : {}
  );
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
  saving.value = true;
  error.value = "";
  try {
    await saveProxyInbound({
      ...form,
      id: form.id || undefined,
      serverId: props.serverId,
    });
    ElMessage.success("已保存，服务器将在一分钟内应用");
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
    :title="form.id ? '编辑入站' : '添加入站'"
    size="min(560px, 100vw)"
  >
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item label="协议模板">
        <UiSegmented
          v-model="form.template"
          label="协议模板"
          :options="[
            { label: 'VLESS Reality Vision', value: 'vless-reality-vision' },
            { label: 'VLESS XHTTP Reality', value: 'vless-xhttp-reality' },
          ]"
        />
        <p class="hint">
          {{
            form.template === "vless-reality-vision"
              ? "兼容性最好，Clash Meta、Shadowrocket、sing-box 等主流客户端都支持。"
              : "XHTTP 可走 CDN 与多路复用，需要较新的客户端（Xray 内核 / mihomo 新版）。"
          }}
        </p>
      </el-form-item>
      <div class="grid">
        <el-form-item label="备注">
          <el-input v-model="form.remark" placeholder="显示在节点名里" />
        </el-form-item>
        <el-form-item label="端口">
          <div class="port-row">
            <el-input-number
              v-model="form.port"
              :min="1024"
              :max="65535"
              controls-position="right"
            />
            <UiButton
              variant="ghost"
              :icon="IconShuffle"
              aria-label="随机端口"
              @click="form.port = randomPort()"
            />
          </div>
        </el-form-item>
      </div>
      <el-form-item label="伪装目标（Reality target）">
        <el-input
          v-model="form.target"
          placeholder="www.apple.com:443"
          @input="check = null"
        />
        <div class="chips">
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
      <el-form-item label="SNI（客户端握手用的域名）">
        <el-input
          v-model="form.sni"
          placeholder="默认与伪装目标相同"
          @input="check = null"
        />
      </el-form-item>
      <div class="check-row">
        <UiButton :icon="IconCheck" :loading="checking" @click="runCheck"
          >检测伪装目标</UiButton
        >
        <Transition name="pop">
          <span
            v-if="check"
            class="check-result"
            :class="check.ok ? (check.h2 ? 'is-ok' : 'is-warn') : 'is-bad'"
          >
            {{ check.message
            }}<template v-if="check.ok"> · {{ check.latency }} ms</template>
          </span>
        </Transition>
      </div>
      <p class="hint">
        检测在面板所在机器上进行；同一站点在不同地区表现可能不同，保存后若节点不通，换一个目标再试。
      </p>
      <el-form-item
        v-if="form.template === 'vless-xhttp-reality'"
        label="XHTTP 路径"
      >
        <el-input v-model="form.path" placeholder="留空自动生成随机路径" />
      </el-form-item>
      <div class="switches">
        <label><el-switch v-model="form.enabled" /> 启用</label>
        <label v-if="form.id"
          ><el-switch v-model="form.regenerateKey" />
          重新生成密钥（所有客户端需更新订阅）</label
        >
      </div>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
    </el-form>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" @click="save"
        >保存</UiButton
      >
    </template>
  </UiDrawer>
</template>
<style scoped>
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
