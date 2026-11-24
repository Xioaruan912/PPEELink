<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import gsap from "gsap";
import { ElMessage, ElMessageBox } from "element-plus";
import { CopyDocument, Download, Refresh } from "@element-plus/icons-vue";
import { UiButton, UiDrawer } from "@/ui";
import {
  banProxyServerIP,
  getProxyServerLogs,
  getProxyServerSecurity,
  tuneProxyServer,
  unbanProxyServerIP,
  upgradeProxyCore,
  type ParsedLogEntry,
  type ParsedLogSummary,
  type ProxyServer,
  type ServerSecurityAudit,
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

const activeTab = ref<"logs" | "tune" | "security">("logs");
const tuning = ref(false);
const upgrading = ref(false);
const loadingLogs = ref(false);
const loadingSec = ref(false);
const actingIP = ref("");
const securityAudit = ref<ServerSecurityAudit | null>(null);
const logSource = ref<"xray" | "agent">("xray");
const logFilter = ref("");
const levelFilter = ref<"all" | "error" | "warn" | "info">("all");
const viewMode = ref<"structured" | "raw">("structured");
const expandedIds = ref<Set<number>>(new Set());

const rawLogs = ref("");
const entries = ref<ParsedLogEntry[]>([]);
const summary = ref<ParsedLogSummary>({
  total: 0,
  errors: 0,
  warnings: 0,
  infos: 0,
  findings: [],
});

const segTrackEl = ref<HTMLElement>();
const segThumbEl = ref<HTMLElement>();
const logListEl = ref<HTMLElement>();

const rmemMB = computed(() => {
  const b = props.server?.rmemMax || 0;
  return b > 0 ? (b / (1024 * 1024)).toFixed(1) : "—";
});

const rmemRatio = computed(() => {
  const b = props.server?.rmemMax || 0;
  if (b <= 0) return 0;
  return Math.min(100, Math.max(8, Math.round((b / (16 * 1024 * 1024)) * 100)));
});

const isBBR = computed(
  () => (props.server?.bbr || "").toLowerCase() === "bbr"
);

const visibleEntries = computed(() => {
  const kw = logFilter.value.trim().toLowerCase();
  return entries.value.filter((e) => {
    if (levelFilter.value !== "all" && e.level !== levelFilter.value) {
      return false;
    }
    if (!kw) return true;
    return `${e.module} ${e.message} ${e.hint || ""}`.toLowerCase().includes(kw);
  });
});

function toggleExpand(id: number) {
  const next = new Set(expandedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  expandedIds.value = next;
}

function moveSegThumb(animate = true) {
  if (!segTrackEl.value || !segThumbEl.value) return;
  const btn = segTrackEl.value.querySelector<HTMLElement>(
    "[data-tab-active='true']"
  );
  if (!btn) return;
  const x = btn.offsetLeft;
  const width = btn.offsetWidth;
  if (!animate || window.matchMedia("(prefers-reduced-motion: reduce)").matches) {
    gsap.set(segThumbEl.value, { x, width });
    return;
  }
  gsap.to(segThumbEl.value, { x, width, duration: 0.34, ease: "expo.out" });
}

function animateLogRows() {
  if (!logListEl.value) return;
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const rows = logListEl.value.querySelectorAll("[data-log-row]");
  if (!rows.length) return;
  gsap.from(Array.from(rows).slice(0, 24), {
    y: 7,
    opacity: 0,
    duration: 0.28,
    stagger: 0.016,
    ease: "expo.out",
    clearProps: "transform,opacity",
  });
}

function selectTab(tab: "logs" | "tune" | "security") {
  if (activeTab.value === tab) return;
  activeTab.value = tab;
  void nextTick(() => moveSegThumb(true));
  if (tab === "security" && !securityAudit.value && props.server?.online) {
    void fetchSecurity();
  }
}

async function fetchSecurity() {
  if (!props.server) return;
  loadingSec.value = true;
  try {
    const res = await getProxyServerSecurity(props.server.id);
    securityAudit.value = res.data || null;
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.msg || err?.message || "读取安全状态失败（需服务器 Agent v1.21+）");
  } finally {
    loadingSec.value = false;
  }
}

async function handleBanIP(ip: string) {
  if (!props.server || actingIP.value) return;
  actingIP.value = ip;
  try {
    const res = (await banProxyServerIP(props.server.id, ip)) as any;
    ElMessage.success(res?.msg || `已封禁 ${ip}`);
    await fetchSecurity();
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.msg || err?.message || "封禁失败");
  } finally {
    actingIP.value = "";
  }
}

async function handleUnbanIP(ip: string) {
  if (!props.server || actingIP.value) return;
  actingIP.value = ip;
  try {
    const res = (await unbanProxyServerIP(props.server.id, ip)) as any;
    ElMessage.success(res?.msg || `已解封 ${ip}`);
    await fetchSecurity();
  } catch (err: any) {
    ElMessage.error(err?.response?.data?.msg || err?.message || "解封失败");
  } finally {
    actingIP.value = "";
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
    ElMessage.warning("请手动复制");
  }
}

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
  expandedIds.value = new Set();
  try {
    const { data } = await getProxyServerLogs({
      id: props.server.id,
      source: logSource.value,
      lines: 200,
    });
    rawLogs.value = data?.logs || "";
    entries.value = data?.entries || [];
    summary.value = data?.summary || {
      total: 0,
      errors: 0,
      warnings: 0,
      infos: 0,
      findings: [],
    };
    await nextTick();
    animateLogRows();
  } catch (err: any) {
    rawLogs.value = "";
    entries.value = [];
    ElMessage.error(err?.message || "拉取日志失败");
  } finally {
    loadingLogs.value = false;
  }
}

function moduleTone(mod: string) {
  const m = mod.toLowerCase();
  if (m.includes("reality") || m.includes("hysteria")) return "is-accent";
  if (m.includes("tls") || m.includes("证书")) return "is-warn";
  if (m.includes("agent") || m.includes("webshell")) return "is-info";
  return "is-neutral";
}

watch(
  () => [props.modelValue, props.server?.id] as const,
  async ([open]) => {
    if (open && props.server) {
      levelFilter.value = "all";
      logFilter.value = "";
      await nextTick();
      moveSegThumb(false);
      void fetchLogs();
    }
  }
);
</script>

<template>
  <UiDrawer
    v-model="visible"
    :title="server ? `内核运维 · ${server.name}` : '内核运维'"
    size="min(780px, 100vw)"
    append-to-body
  >
    <div v-if="server" class="ops-shell">
      <!-- Top Segmented Switch + Live Host Pill -->
      <header class="ops-topbar">
        <div ref="segTrackEl" class="ops-seg" role="tablist">
          <span ref="segThumbEl" class="ops-seg-thumb" aria-hidden="true"></span>
          <button
            type="button"
            role="tab"
            class="ops-seg-btn"
            :data-tab-active="activeTab === 'logs'"
            @click="selectTab('logs')"
          >
            实时日志与排查
            <span v-if="summary.errors > 0" class="seg-dot is-bad mono">
              {{ summary.errors }}
            </span>
          </button>
          <button
            type="button"
            role="tab"
            class="ops-seg-btn"
            :data-tab-active="activeTab === 'tune'"
            @click="selectTab('tune')"
          >
            BBR 调优与内核升级
          </button>
          <button
            type="button"
            role="tab"
            class="ops-seg-btn"
            :data-tab-active="activeTab === 'security'"
            @click="selectTab('security')"
          >
            安全防护
          </button>
        </div>

        <div class="ops-meta-pills mono">
          <span class="pill-chip" :class="isBBR ? 'is-ok' : 'is-warn'">
            {{ (server.bbr || "未知").toUpperCase() }}
          </span>
          <span class="pill-chip">{{ server.xrayVersion || "xray" }}</span>
        </div>
      </header>

      <!-- TAB 1: Structured Apple Console Logs -->
      <div v-if="activeTab === 'logs'" class="console-pane">
        <!-- Auto-Diagnosis Banner -->
        <section v-if="!loadingLogs && summary.total > 0" class="diag-area">
          <div
            v-if="summary.findings.length === 0 && summary.errors === 0"
            class="diag-ok"
          >
            <span class="diag-dot is-ok"></span>
            <strong>运行平稳</strong>
            <span>最近 {{ summary.total }} 条日志未发现严重异常</span>
          </div>

          <div v-else class="diag-cards">
            <article
              v-for="f in summary.findings"
              :key="f.title"
              class="diag-card"
              :class="`is-${f.level}`"
            >
              <div class="diag-card-head">
                <span class="diag-dot" :class="`is-${f.level}`"></span>
                <strong>{{ f.title }}</strong>
                <span class="diag-cnt mono">×{{ f.count }}</span>
              </div>
              <p>{{ f.detail }}</p>
            </article>
          </div>
        </section>

        <!-- Precision Filter Bar -->
        <div class="console-toolbar">
          <div class="source-switch">
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

          <div class="level-pills" role="group" aria-label="日志级别">
            <button
              type="button"
              class="lv-pill"
              :class="{ 'is-active': levelFilter === 'all' }"
              @click="levelFilter = 'all'"
            >
              全部 <span class="mono">{{ summary.total }}</span>
            </button>
            <button
              type="button"
              class="lv-pill is-err"
              :class="{ 'is-active': levelFilter === 'error' }"
              @click="levelFilter = 'error'"
            >
              错误 <span class="mono">{{ summary.errors }}</span>
            </button>
            <button
              type="button"
              class="lv-pill is-warn"
              :class="{ 'is-active': levelFilter === 'warn' }"
              @click="levelFilter = 'warn'"
            >
              警告 <span class="mono">{{ summary.warnings }}</span>
            </button>
            <button
              type="button"
              class="lv-pill"
              :class="{ 'is-active': levelFilter === 'info' }"
              @click="levelFilter = 'info'"
            >
              正常 <span class="mono">{{ summary.infos }}</span>
            </button>
          </div>

          <div class="toolbar-right">
            <input
              v-model="logFilter"
              type="search"
              class="console-search"
              placeholder="筛选关键字…"
            />
            <button
              type="button"
              class="mode-chip"
              @click="
                viewMode = viewMode === 'structured' ? 'raw' : 'structured'
              "
            >
              {{ viewMode === "structured" ? "原文" : "结构化" }}
            </button>
            <button
              type="button"
              class="icon-chip"
              title="复制全部日志"
              :disabled="!rawLogs"
              @click="copyText(rawLogs, '已复制原始日志')"
            >
              <el-icon><CopyDocument /></el-icon>
            </button>
            <UiButton
              variant="ghost"
              size="sm"
              :icon="Refresh"
              :loading="loadingLogs"
              :disabled="!server.online"
              @click="fetchLogs"
            >
              刷新
            </UiButton>
          </div>
        </div>

        <!-- Structured Apple Console List -->
        <div v-if="loadingLogs" class="console-loading">
          <span class="pulse-dot"></span>
          <span>正在从「{{ server.name }}」拉取并解析日志…</span>
        </div>

        <div
          v-else-if="viewMode === 'structured' && visibleEntries.length"
          ref="logListEl"
          class="log-stream"
        >
          <article
            v-for="item in visibleEntries"
            :key="item.id"
            class="log-row"
            :class="[
              `is-${item.level}`,
              { 'is-expanded': expandedIds.has(item.id) },
            ]"
            data-log-row
            @click="toggleExpand(item.id)"
          >
            <div class="log-main">
              <span class="log-time mono" :title="item.fullTime">
                {{ item.time || "—" }}
              </span>
              <span class="log-mod" :class="moduleTone(item.module)">
                {{ item.module }}
              </span>
              <span class="log-msg">{{ item.message }}</span>
              <span
                v-if="item.count > 1"
                class="log-dup mono"
                :title="`连续出现 ${item.count} 次`"
              >
                ×{{ item.count }}
              </span>
            </div>

            <div v-if="item.hint" class="log-hint">
              <span class="hint-tag">排查建议</span>
              <span>{{ item.hint }}</span>
            </div>

            <div
              v-if="expandedIds.has(item.id)"
              class="log-raw-detail"
              @click.stop
            >
              <div class="raw-detail-head">
                <span class="mono">{{ item.fullTime || "原始记录" }}</span>
                <button
                  type="button"
                  class="raw-copy-btn"
                  @click="copyText(item.raw, '已复制该条日志')"
                >
                  复制原行
                </button>
              </div>
              <code class="mono">{{ item.raw }}</code>
            </div>
          </article>
        </div>

        <pre
          v-else-if="viewMode === 'raw' && rawLogs"
          class="raw-pre mono"
        >{{ rawLogs }}</pre>

        <div v-else class="console-empty">
          <strong>暂无匹配的日志记录</strong>
          <small>
            {{
              entries.length > 0
                ? "尝试切换到「全部」或清空筛选关键字"
                : "当前服务运行安静，或尚未产生日志输出"
            }}
          </small>
        </div>
      </div>

      <!-- TAB 2: BBR Tuning + Core Hot Upgrade -->
      <div v-else-if="activeTab === 'tune'" class="tune-pane">
        <!-- 1. BBR + UDP Buffer Tuning -->
        <section class="ops-card">
          <header class="ops-head">
            <div>
              <h3>网络内核调优（BBR + 16MB UDP 缓冲）</h3>
              <p>
                针对 Hysteria2 QUIC 与 Reality 晚高峰吞吐优化，持久化写入
                <code>/etc/sysctl.d/99-ppeelink.conf</code>
              </p>
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
                {{ (server.bbr || "未上报").toUpperCase() }}
              </strong>
            </div>
            <div class="ops-kv">
              <span>队列调度</span>
              <strong class="mono">{{ server.qdisc || "—" }}</strong>
            </div>
            <div class="ops-kv">
              <span>UDP 接收缓冲</span>
              <strong class="mono">{{ rmemMB }} MB</strong>
              <div class="kv-bar" aria-hidden="true">
                <i
                  :class="rmemRatio >= 80 ? 'is-ok' : 'is-warn'"
                  :style="{ width: `${rmemRatio}%` }"
                ></i>
              </div>
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
              <h3>Xray 内核热升级与自动回滚保护</h3>
              <p>
                当前版本：<strong class="mono">{{
                  server.xrayVersion || "未知"
                }}</strong>
                · 在线下载最新官方发布版并校验可执行性；若启动失败自动回滚旧版内核，入站配置变更亦自带回滚快照。
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
              热升级至最新版
            </UiButton>
          </header>
        </section>
      </div>

      <!-- TAB 3: Inbound Security Guard (Active IPs / Probe Alerts / Firewall Ban) -->
      <div v-else-if="activeTab === 'security'" class="sec-pane">
        <div class="sec-bar">
          <div class="sec-kpis mono">
            <span class="sec-kpi">活跃连接 <strong>{{ securityAudit?.activeConns ?? 0 }}</strong></span>
            <span class="sec-kpi">在线 IP <strong>{{ securityAudit?.uniqueClientIps ?? 0 }}</strong></span>
            <span class="sec-kpi">已封禁 <strong>{{ securityAudit?.bannedIps?.length ?? 0 }}</strong></span>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            :icon="Refresh"
            :loading="loadingSec"
            :disabled="!server.online"
            @click="fetchSecurity"
          >
            刷新
          </UiButton>
        </div>

        <section class="ops-card">
          <header class="ops-head">
            <h3>异常探测来源 IP</h3>
          </header>
          <div v-if="securityAudit?.alerts?.length" class="sec-list">
            <div v-for="a in securityAudit.alerts" :key="a.ip" class="sec-row">
              <div class="sec-row-left">
                <strong class="mono">{{ a.ip }}</strong>
                <span class="sec-reason">{{ a.lastReason }}</span>
                <span class="sec-cnt mono">×{{ a.hits }}</span>
              </div>
              <button
                v-if="!a.banned"
                type="button"
                class="ban-btn"
                :disabled="actingIP === a.ip"
                @click="handleBanIP(a.ip)"
              >
                封禁
              </button>
              <button
                v-else
                type="button"
                class="unban-btn"
                :disabled="actingIP === a.ip"
                @click="handleUnbanIP(a.ip)"
              >
                解封
              </button>
            </div>
          </div>
          <div v-else class="sec-empty">暂无异常探测记录</div>
        </section>

        <section class="ops-card">
          <header class="ops-head">
            <h3>当前活跃来源 IP</h3>
          </header>
          <div v-if="securityAudit?.clients?.length" class="sec-list">
            <div v-for="c in securityAudit.clients" :key="c.ip" class="sec-row">
              <div class="sec-row-left">
                <strong class="mono">{{ c.ip }}</strong>
                <span class="sec-cnt mono">{{ c.conns }} 连接</span>
                <span class="sec-reason mono">:{{ c.ports.join(", :") }}</span>
              </div>
              <button
                type="button"
                class="ban-btn"
                :disabled="actingIP === c.ip"
                @click="handleBanIP(c.ip)"
              >
                封禁
              </button>
            </div>
          </div>
          <div v-else class="sec-empty">当前无活跃入站连接</div>
        </section>

        <section v-if="securityAudit?.bannedIps?.length" class="ops-card">
          <header class="ops-head">
            <h3>防火墙封禁列表 (PPEELINK_BAN)</h3>
          </header>
          <div class="sec-list">
            <div v-for="ip in securityAudit.bannedIps" :key="ip" class="sec-row">
              <strong class="mono">{{ ip }}</strong>
              <button
                type="button"
                class="unban-btn"
                :disabled="actingIP === ip"
                @click="handleUnbanIP(ip)"
              >
                解封
              </button>
            </div>
          </div>
        </section>
      </div>
    </div>
  </UiDrawer>
</template>

<style scoped>
.ops-shell {
  display: grid;
  gap: 16px;
}

.ops-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

/* Apple Segmented Control */
.ops-seg {
  position: relative;
  display: inline-flex;
  align-items: center;
  padding: 3px;
  border-radius: 999px;
  background: var(--c-surface-2);
  border: 1px solid var(--c-line);
}

.ops-seg-thumb {
  position: absolute;
  top: 3px;
  left: 0;
  height: calc(100% - 6px);
  border-radius: 999px;
  background: var(--c-surface);
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.08);
  pointer-events: none;
}

.ops-seg-btn {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 0;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition: color 150ms ease;
}

.ops-seg-btn[data-tab-active="true"] {
  color: var(--c-ink);
  font-weight: 600;
}

.seg-dot {
  display: inline-grid;
  place-items: center;
  min-width: 17px;
  height: 17px;
  padding: 0 5px;
  border-radius: 999px;
  font-size: 10px;
  color: #fff;
  background: var(--c-bad);
}

.ops-meta-pills {
  display: flex;
  align-items: center;
  gap: 6px;
}

.pill-chip {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  font-size: 11px;
  color: var(--c-muted);
}

.pill-chip.is-ok {
  color: var(--c-ok);
  border-color: color-mix(in srgb, var(--c-ok) 35%, var(--c-line));
  background: color-mix(in srgb, var(--c-ok) 8%, var(--c-surface));
}

.pill-chip.is-warn {
  color: var(--c-warn, #d97706);
  border-color: color-mix(in srgb, var(--c-warn, #d97706) 35%, var(--c-line));
  background: color-mix(in srgb, var(--c-warn, #d97706) 8%, var(--c-surface));
}

/* Console Pane */
.console-pane {
  display: grid;
  gap: 14px;
}

.diag-ok {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border: 1px solid color-mix(in srgb, var(--c-ok) 28%, var(--c-line));
  border-radius: 14px;
  background: color-mix(in srgb, var(--c-ok) 7%, var(--c-surface));
  font-size: 12.5px;
  color: var(--c-ink);
}

.diag-ok span:last-child {
  color: var(--c-muted);
}

.diag-cards {
  display: grid;
  gap: 8px;
}

.diag-card {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-left-width: 3px;
  border-radius: 14px;
  background: var(--c-surface);
}

.diag-card.is-error {
  border-left-color: var(--c-bad);
  background: color-mix(in srgb, var(--c-bad) 5%, var(--c-surface));
}

.diag-card.is-warn {
  border-left-color: var(--c-warn, #d97706);
  background: color-mix(in srgb, var(--c-warn, #d97706) 5%, var(--c-surface));
}

.diag-card-head {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 13px;
  color: var(--c-ink);
}

.diag-cnt {
  margin-left: auto;
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--c-surface-2);
  font-size: 11px;
  color: var(--c-muted);
}

.diag-card p {
  margin: 0;
  font-size: 12px;
  color: var(--c-muted);
  line-height: 1.5;
}

.diag-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.diag-dot.is-ok {
  background: var(--c-ok);
}

.diag-dot.is-warn {
  background: var(--c-warn, #d97706);
}

.diag-dot.is-error {
  background: var(--c-bad);
}

/* Toolbar */
.console-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.source-switch {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
}

.source-switch button {
  padding: 4px 10px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 500;
  cursor: pointer;
}

.source-switch button.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.level-pills {
  display: flex;
  align-items: center;
  gap: 4px;
}

.lv-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px 9px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11.5px;
  cursor: pointer;
  transition: border-color 140ms ease, color 140ms ease, background 140ms ease;
}

.lv-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.lv-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.lv-pill.is-err:not(.is-active) {
  color: var(--c-bad);
}

.lv-pill.is-warn:not(.is-active) {
  color: var(--c-warn, #d97706);
}

.toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
}

.console-search {
  width: 148px;
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  outline: none;
}

.console-search:focus {
  border-color: var(--c-ink);
}

.mode-chip {
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11.5px;
  cursor: pointer;
}

.mode-chip:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.icon-chip {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-muted);
  cursor: pointer;
}

.icon-chip:hover:not(:disabled) {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.icon-chip:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* Structured Log Stream */
.log-stream {
  display: grid;
  gap: 6px;
  max-height: calc(100vh - 280px);
  overflow-y: auto;
  padding-right: 4px;
}

.log-row {
  display: grid;
  gap: 6px;
  padding: 10px 13px;
  border: 1px solid var(--c-line);
  border-left-width: 3px;
  border-radius: 12px;
  background: var(--c-surface);
  cursor: pointer;
  transition: border-color 140ms ease, background 140ms ease;
}

.log-row:hover {
  border-color: color-mix(in srgb, var(--c-ink) 35%, var(--c-line));
}

.log-row.is-error {
  border-left-color: var(--c-bad);
}

.log-row.is-warn {
  border-left-color: var(--c-warn, #d97706);
}

.log-row.is-info {
  border-left-color: color-mix(in srgb, var(--c-ink) 20%, var(--c-line));
}

.log-main {
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-width: 0;
}

.log-time {
  flex-shrink: 0;
  font-size: 11px;
  color: var(--c-muted);
}

.log-mod {
  flex-shrink: 0;
  padding: 1px 7px;
  border-radius: 6px;
  font-size: 10.5px;
  font-weight: 600;
}

.log-mod.is-accent {
  background: color-mix(in srgb, var(--c-ok) 12%, var(--c-surface));
  color: var(--c-ok);
}

.log-mod.is-warn {
  background: color-mix(in srgb, var(--c-warn, #d97706) 13%, var(--c-surface));
  color: var(--c-warn, #d97706);
}

.log-mod.is-info {
  background: rgba(37, 99, 235, 0.1);
  color: #2563eb;
}

.log-mod.is-neutral {
  background: var(--c-surface-2);
  color: var(--c-muted);
}

.log-msg {
  flex: 1;
  min-width: 0;
  font-size: 12.5px;
  line-height: 1.45;
  color: var(--c-ink);
  word-break: break-word;
}

.log-dup {
  flex-shrink: 0;
  padding: 1px 6px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 10.5px;
  font-weight: 600;
}

.log-hint {
  display: flex;
  align-items: baseline;
  gap: 7px;
  padding: 6px 10px;
  border-radius: 8px;
  background: var(--c-surface-2);
  font-size: 11.5px;
  color: var(--c-muted);
  line-height: 1.45;
}

.hint-tag {
  flex-shrink: 0;
  font-weight: 600;
  color: var(--c-ink);
}

.log-raw-detail {
  display: grid;
  gap: 6px;
  padding: 10px 12px;
  border-radius: 10px;
  background: #14161b;
  color: #d1d5db;
  font-size: 11px;
  line-height: 1.5;
}

.raw-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: #9ca3af;
  font-size: 10.5px;
}

.raw-copy-btn {
  padding: 2px 8px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 999px;
  background: transparent;
  color: #e5e7eb;
  font-size: 10.5px;
  cursor: pointer;
}

.raw-copy-btn:hover {
  border-color: rgba(255, 255, 255, 0.45);
}

.log-raw-detail code {
  word-break: break-all;
  white-space: pre-wrap;
}

.raw-pre {
  margin: 0;
  padding: 14px 16px;
  max-height: calc(100vh - 280px);
  overflow: auto;
  border-radius: 14px;
  background: #14161b;
  color: #e5e7eb;
  font-size: 11.5px;
  line-height: 1.55;
  white-space: pre-wrap;
  word-break: break-all;
}

.console-loading,
.console-empty {
  display: grid;
  place-items: center;
  gap: 6px;
  padding: 48px 20px;
  border: 1px dashed var(--c-line);
  border-radius: 16px;
  color: var(--c-muted);
  font-size: 13px;
}

.console-empty strong {
  color: var(--c-ink);
  font-weight: 600;
}

.console-empty small {
  font-size: 12px;
}

.pulse-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-ok);
  animation: ops-pulse 1.6s ease-in-out infinite;
}

@keyframes ops-pulse {
  0%,
  100% {
    opacity: 0.35;
    transform: scale(0.85);
  }
  50% {
    opacity: 1;
    transform: scale(1.2);
  }
}

/* Tune Pane */
.tune-pane {
  display: grid;
  gap: 16px;
}

.ops-card {
  display: grid;
  gap: 16px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
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
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}

.ops-head p {
  margin: 5px 0 0;
  font-size: 12.5px;
  color: var(--c-muted);
  line-height: 1.5;
}

.ops-head code {
  padding: 1px 5px;
  border-radius: 5px;
  background: var(--c-surface-2);
  font-size: 11.5px;
}

.ops-metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  padding-top: 14px;
  border-top: 1px solid var(--c-line);
}

.ops-kv {
  display: grid;
  gap: 4px;
}

.ops-kv span {
  font-size: 11px;
  color: var(--c-muted);
}

.ops-kv strong {
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}

.ops-kv strong.is-ok {
  color: var(--c-ok);
}

.ops-kv strong.is-warn {
  color: var(--c-warn, #d97706);
}

.kv-bar {
  height: 4px;
  border-radius: 999px;
  background: var(--c-surface-2);
  overflow: hidden;
}

.kv-bar i {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: var(--c-ok);
}

.kv-bar i.is-warn {
  background: var(--c-warn, #d97706);
}

.sec-pane {
  display: grid;
  gap: 14px;
}

.sec-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}

.sec-kpis {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.sec-kpi {
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.sec-kpi strong {
  color: var(--c-ink);
  margin-left: 4px;
}

.sec-list {
  display: grid;
  gap: 6px;
}

.sec-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 10px;
  background: var(--c-surface-muted);
  font-size: 12.5px;
}

.sec-row-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  min-width: 0;
}

.sec-reason {
  font-size: 11.5px;
  color: var(--c-muted);
}

.sec-cnt {
  padding: 1px 6px;
  border-radius: 5px;
  background: var(--c-surface);
  font-size: 11px;
  color: var(--c-ink);
}

.ban-btn,
.unban-btn {
  padding: 4px 10px;
  border-radius: 8px;
  font-size: 11.5px;
  cursor: pointer;
  flex-shrink: 0;
}

.ban-btn {
  border: 1px solid color-mix(in srgb, var(--c-bad) 36%, transparent);
  background: color-mix(in srgb, var(--c-bad) 10%, transparent);
  color: var(--c-bad);
}

.unban-btn {
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-ink);
}

.sec-empty {
  padding: 18px 8px;
  text-align: center;
  font-size: 12.5px;
  color: var(--c-muted);
}

@media (max-width: 640px) {
  .ops-metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .toolbar-right {
    margin-left: 0;
    width: 100%;
  }
  .console-search {
    flex: 1;
  }
}
</style>
