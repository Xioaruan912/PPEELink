<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Download, Refresh } from "@element-plus/icons-vue";
import { UiButton, UiDrawer } from "@/ui";
import {
  getProxyServerLogs,
  tuneProxyServer,
  upgradeProxyCore,
  type ProxyServer,
} from "@/api/proxy";

const props = defineProps<{
  modelValue: boolean;
  server: ProxyServer | null;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "refresh"): void;
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const tuning = ref(false);
const upgrading = ref(false);
const loadingLogs = ref(false);
const logSource = ref<"xray" | "agent">("xray");
const logFilter = ref("");
const logText = ref("");

const rmemMB = computed(() => {
  const b = props.server?.rmemMax || 0;
  return b > 0 ? (b / (1024 * 1024)).toFixed(1) : "—";
});

const isBBR = computed(
  () => (props.server?.bbr || "").toLowerCase() === "bbr"
);

async function runTune() {
  if (!props.server) return;
  tuning.value = true;
  try {
    const res = (await tuneProxyServer(props.server.id)) as any;
    ElMessage.success(res?.msg || "BBR 与 UDP 缓冲区调优完成");
    emit("refresh");
  } catch (err: any) {
    ElMessage.error(err?.message || "调优失败");
  } finally {
    tuning.value = false;
  }
}

async function runUpgradeCore() {
  if (!props.server) return;
  try {
    await ElMessageBox.confirm(
      `将在线下载最新官方 Xray-core 并平滑重启「${props.server.name}」（若启动失败将自动回滚至旧版）。确认继续？`,
      "热升级 Xray 内核",
      {
        type: "warning",
        confirmButtonText: "立即升级",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  upgrading.value = true;
  try {
    const res = (await upgradeProxyCore(props.server.id, "latest")) as any;
    ElMessage.success(res?.msg || "内核升级成功");
    emit("refresh");
  } catch (err: any) {
    ElMessage.error(err?.message || "升级失败");
  } finally {
    upgrading.value = false;
  }
}

async function fetchLogs() {
  if (!props.server) return;
  loadingLogs.value = true;
  try {
    const { data } = await getProxyServerLogs({
      id: props.server.id,
      source: logSource.value,
      filter: logFilter.value.trim(),
      lines: 180,
    });
    logText.value = data?.logs || "(暂无日志)";
  } catch (err: any) {
    logText.value = "";
    ElMessage.error(err?.message || "拉取日志失败");
  } finally {
    loadingLogs.value = false;
  }
}

watch(
  () => [props.modelValue, props.server?.id] as const,
  ([open]) => {
    if (open && props.server) {
      logText.value = "";
      void fetchLogs();
    }
  }
);
</script>

<template>
  <UiDrawer
    v-model="visible"
    :title="server ? `内核运维 · ${server.name}` : '内核运维'"
    size="min(720px, 100vw)"
    append-to-body
  >
    <div v-if="server" class="ops-root">
      <!-- 1. BBR + UDP Buffer Tuning -->
      <section class="ops-card">
        <header class="ops-head">
          <div>
            <h3>网络内核调优（BBR + UDP 缓冲）</h3>
            <p>优化 Hysteria2 QUIC 与 Reality 晚高峰吞吐，持久化至 /etc/sysctl.d/99-ppeelink.conf</p>
          </div>
          <UiButton
            variant="primary"
            size="sm"
            :loading="tuning"
            :disabled="!server.online"
            @click="runTune"
          >
            {{ isBBR ? "重新应用调优" : "一键开启 BBR" }}
          </UiButton>
        </header>

        <div class="ops-metrics">
          <div class="ops-kv">
            <span>拥塞控制</span>
            <strong class="mono" :class="isBBR ? 'is-ok' : 'is-warn'">
              {{ server.bbr || "未上报" }}
            </strong>
          </div>
          <div class="ops-kv">
            <span>队列调度</span>
            <strong class="mono">{{ server.qdisc || "—" }}</strong>
          </div>
          <div class="ops-kv">
            <span>UDP 接收缓冲</span>
            <strong class="mono">{{ rmemMB }} MB</strong>
          </div>
          <div class="ops-kv">
            <span>自动防火墙</span>
            <strong :class="server.autoFirewall ? 'is-ok' : ''">
              {{ server.autoFirewall ? "已开启" : "未开启" }}
            </strong>
          </div>
        </div>
      </section>

      <!-- 2. Xray Core Hot Upgrade -->
      <section class="ops-card">
        <header class="ops-head">
          <div>
            <h3>Xray 内核热升级</h3>
            <p>
              当前版本：<strong class="mono">{{ server.xrayVersion || "未知" }}</strong>
              · 升级前校验可执行性，启动失败自动回滚旧版，配置修改同样支持自动回滚
            </p>
          </div>
          <UiButton
            variant="secondary"
            size="sm"
            :icon="Download"
            :loading="upgrading"
            :disabled="!server.online"
            @click="runUpgradeCore"
          >
            升级至最新版
          </UiButton>
        </header>
      </section>

      <!-- 3. Live Kernel / Agent Logs -->
      <section class="ops-card">
        <header class="ops-head">
          <div>
            <h3>内核实时日志</h3>
            <p>按需拉取最近 180 行运行日志，免开 SSH 终端快速排查握手与证书问题</p>
          </div>
        </header>

        <div class="log-bar">
          <div class="log-tabs">
            <button
              type="button"
              :class="{ 'is-active': logSource === 'xray' }"
              @click="
                logSource = 'xray';
                fetchLogs();
              "
            >
              Xray 内核
            </button>
            <button
              type="button"
              :class="{ 'is-active': logSource === 'agent' }"
              @click="
                logSource = 'agent';
                fetchLogs();
              "
            >
              Agent 服务
            </button>
          </div>

          <input
            v-model="logFilter"
            type="search"
            class="log-filter"
            placeholder="关键字过滤（如 error / tls / hy2）…"
            @keydown.enter="fetchLogs"
          />

          <UiButton
            variant="ghost"
            size="sm"
            :icon="Refresh"
            :loading="loadingLogs"
            :disabled="!server.online"
            @click="fetchLogs"
          >
            拉取
          </UiButton>
        </div>

        <pre class="log-box mono">{{
          loadingLogs ? "正在从服务器拉取日志…" : logText || "(暂无日志)"
        }}</pre>
      </section>
    </div>
  </UiDrawer>
</template>

<style scoped>
.ops-root {
  display: grid;
  gap: 16px;
}

.ops-card {
  display: grid;
  gap: 14px;
  padding: 18px 20px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.ops-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  flex-wrap: wrap;
}

.ops-head h3 {
  margin: 0;
  font-size: 14.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.ops-head p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--c-muted);
}

.ops-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid var(--c-line);
}

.ops-kv {
  display: grid;
  gap: 3px;
}

.ops-kv span {
  font-size: 11px;
  color: var(--c-muted);
}

.ops-kv strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.ops-kv strong.is-ok {
  color: var(--c-ok);
}

.ops-kv strong.is-warn {
  color: var(--c-warn, #d97706);
}

.log-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.log-tabs {
  display: inline-flex;
  padding: 3px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
}

.log-tabs button {
  padding: 4px 11px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  cursor: pointer;
}

.log-tabs button.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.log-filter {
  flex: 1;
  min-width: 160px;
  height: 32px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  outline: none;
}

.log-filter:focus {
  border-color: var(--c-ink);
}

.log-box {
  margin: 0;
  padding: 14px 16px;
  max-height: 320px;
  overflow: auto;
  border-radius: 14px;
  background: #111317;
  color: #e5e7eb;
  font-size: 11.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
}

@media (max-width: 640px) {
  .ops-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
