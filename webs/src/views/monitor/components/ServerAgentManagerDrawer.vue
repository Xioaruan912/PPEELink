<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Refresh } from "@element-plus/icons-vue";
import { EmptyState, StatusDot, UiButton, UiDrawer, UiTag } from "@/ui";
import { updateMonitorAgent, type MonitorItem } from "@/api/monitor";
import {
  listProxyServers,
  migrateProxyServerURL,
  runProxyServerUnlockCheck,
  saveProxyServerPolicy,
  type ProxyServer,
  type ServerUnlockItem,
} from "@/api/proxy";

const props = defineProps<{
  modelValue: boolean;
  items: MonitorItem[];
  latestAgentVersion: string;
  checkingAgents?: boolean;
  upgradingId?: number | null;
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "refresh"): void;
  (e: "upgrade", item: MonitorItem, cancel?: boolean): void;
  (e: "open-proxy"): void;
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const proxyServers = ref<ProxyServer[]>([]);
const selectedServerIds = ref<Set<number>>(new Set());

// Card 2: Panel URL Migration
const migrateUrl = ref("");
const migrating = ref(false);
const migrateResults = ref<
  { serverId: number; serverName: string; ok: boolean; message: string }[]
>([]);

// Card 3: Privacy & Security Policy
const policy = reactive({
  terminalEnabled: true,
  filesEnabled: true,
  dockerEnabled: true,
  autoFirewall: false,
  websiteTrafficEnabled: false,
  allowAutoUpdate: true,
});
const savingPolicy = ref(false);

// Card 4: Native Unlock Matrix
const checkingUnlock = ref(false);
const batchUpgrading = ref(false);

const onlineItems = computed(() => props.items.filter((i) => i.online));
const updatableItems = computed(() =>
  props.items.filter(
    (i) =>
      i.online &&
      i.agentUpgradeable !== false &&
      i.agentUpdateAvailable &&
      !i.agentUpdateRequested &&
      !i.removing
  )
);

const linkedOnlineServerIds = computed(() => {
  const fromServers = proxyServers.value
    .filter((s) => s.kind === "agent" && s.online)
    .map((s) => s.id);
  if (fromServers.length) return fromServers;
  return props.items
    .filter((i) => i.online && i.proxyServerId)
    .map((i) => Number(i.proxyServerId));
});

const targetServerIds = computed(() => {
  if (selectedServerIds.value.size > 0) {
    return Array.from(selectedServerIds.value);
  }
  return linkedOnlineServerIds.value;
});

const targetScopeLabel = computed(() => {
  if (selectedServerIds.value.size > 0) {
    return `已选 ${selectedServerIds.value.size} 台`;
  }
  return `全部在线 (${linkedOnlineServerIds.value.length})`;
});

function toggleServerSelect(serverId?: number | null) {
  if (!serverId) return;
  const next = new Set(selectedServerIds.value);
  if (next.has(serverId)) next.delete(serverId);
  else next.add(serverId);
  selectedServerIds.value = next;
}

async function loadProxyState() {
  try {
    const { data } = await listProxyServers();
    proxyServers.value = data || [];
    const sample = proxyServers.value.find((s) => s.kind === "agent");
    if (sample) {
      policy.terminalEnabled = sample.terminalEnabled ?? true;
      policy.filesEnabled = sample.filesEnabled ?? true;
      policy.dockerEnabled = sample.dockerEnabled ?? true;
      policy.allowAutoUpdate = sample.allowAutoUpdate ?? true;
      policy.autoFirewall = sample.autoFirewall ?? false;
      policy.websiteTrafficEnabled = sample.websiteTrafficEnabled ?? false;
    }
  } catch {
    // Non-fatal: monitor items already carry summary fields
  }
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      migrateResults.value = [];
      selectedServerIds.value = new Set();
      if (!migrateUrl.value) {
        migrateUrl.value = window.location.origin;
      }
      void loadProxyState();
    }
  }
);

function agentLabel(item: MonitorItem) {
  return item.agentVersion
    ? `v${item.agentVersion}`
    : item.version || item.agentHash
      ? "旧版本"
      : "等待上报";
}

function agentState(item: MonitorItem) {
  if (item.removing) return "等待卸载";
  if (item.agentUpdateError) return "升级失败";
  if (item.agentUpdateRequested)
    return item.online ? "升级中" : "待上线";
  if (item.agentUpgradeable === false) return "随面板更新";
  if (item.agentUpdateConfirmed) return "最新";
  if (item.agentUpdateAvailable) return "可升级";
  return "待上报";
}

async function upgradeAll() {
  const targets = [...updatableItems.value];
  if (!targets.length || batchUpgrading.value) return;
  batchUpgrading.value = true;
  try {
    let okCount = 0;
    let firstErr = "";
    await Promise.allSettled(
      targets.map(async (item) => {
        try {
          await updateMonitorAgent(item.id, false);
          okCount++;
        } catch (err: any) {
          if (!firstErr) firstErr = err?.message || "请求失败";
        }
      })
    );
    if (okCount === targets.length) {
      ElMessage.success(`已向 ${okCount} 台服务器下发 Agent 更新`);
    } else if (okCount > 0) {
      ElMessage.warning(`已下发 ${okCount}/${targets.length} 台（${firstErr}）`);
    } else {
      ElMessage.error(firstErr || "批量升级失败");
    }
    emit("refresh");
  } finally {
    batchUpgrading.value = false;
  }
}

async function handleMigrateURL() {
  const url = migrateUrl.value.trim().replace(/\/+$/, "");
  if (!url.startsWith("http://") && !url.startsWith("https://")) {
    ElMessage.warning("请输入 http:// 或 https:// 开头的面板地址");
    return;
  }
  if (!targetServerIds.value.length) {
    ElMessage.warning("暂无可下发的在线服务器");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `将向 ${targetServerIds.value.length} 台服务器下发新面板地址「${url}」，节点会在预检通过后切换并重启。`,
      "确认迁移面板地址",
      {
        type: "warning",
        confirmButtonText: "立即下发",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  migrating.value = true;
  migrateResults.value = [];
  try {
    const { data } = await migrateProxyServerURL({
      panelUrl: url,
      serverIds:
        selectedServerIds.value.size > 0 ? targetServerIds.value : undefined,
    });
    migrateResults.value = data?.results || [];
    if (data?.okCount === data?.total && (data?.total || 0) > 0) {
      ElMessage.success(`已成功迁移 ${data.okCount} 台服务器`);
    } else {
      ElMessage.warning(
        `迁移完成：${data?.okCount || 0}/${data?.total || 0} 成功`
      );
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "地址迁移失败");
  } finally {
    migrating.value = false;
  }
}

async function handleSavePolicy() {
  savingPolicy.value = true;
  try {
    const { data } = await saveProxyServerPolicy({
      serverIds:
        selectedServerIds.value.size > 0 ? targetServerIds.value : undefined,
      terminalEnabled: policy.terminalEnabled,
      filesEnabled: policy.filesEnabled,
      dockerEnabled: policy.dockerEnabled,
      allowAutoUpdate: policy.allowAutoUpdate,
      autoFirewall: policy.autoFirewall,
      websiteTrafficEnabled: policy.websiteTrafficEnabled,
    });
    ElMessage.success(
      `已更新 ${data?.updated || 0} 台策略，实时推送 ${data?.pushed || 0} 台`
    );
    emit("refresh");
    await loadProxyState();
  } catch (err: any) {
    ElMessage.error(err?.message || "策略保存失败");
  } finally {
    savingPolicy.value = false;
  }
}

async function handleUnlockCheck(serverId?: number) {
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
        "请先升级服务器 Agent 至最新版本";
      ElMessage.warning(`0/${total} 台检测成功：${firstErr}`);
    } else if (okCount < total) {
      ElMessage.warning(`解锁检测完成：${okCount}/${total} 台成功`);
    } else {
      ElMessage.success(`解锁检测完成：${okCount}/${total} 台已更新`);
    }
    await loadProxyState();
  } catch (err: any) {
    ElMessage.error(err?.message || "解锁检测失败");
  } finally {
    checkingUnlock.value = false;
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

function formatPillText(it: ServerUnlockItem) {
  const base = shortUnlockLabel[it.key] || it.name;
  if (it.status === "originals") return `${base}自制`;
  return base;
}
</script>

<template>
  <UiDrawer
    v-model="visible"
    title="Server Agent 管理"
    size="min(680px, 100vw)"
    append-to-body
  >
    <div class="fleet-stack">
      <!-- Card 1: Fleet Version & Upgrade -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>版本与节点</h3>
            <span class="fleet-pill mono">
              {{ latestAgentVersion ? `v${latestAgentVersion}` : "—" }}
            </span>
            <span class="fleet-pill is-ok mono">
              在线 {{ onlineItems.length }}/{{ items.length }}
            </span>
          </div>
          <div class="fleet-head-actions">
            <UiButton
              variant="ghost"
              size="sm"
              :icon="Refresh"
              :loading="checkingAgents"
              @click="emit('refresh')"
            >
              刷新
            </UiButton>
            <UiButton
              v-if="updatableItems.length"
              variant="primary"
              size="sm"
              :loading="batchUpgrading"
              @click="upgradeAll"
            >
              升级全部 ({{ updatableItems.length }})
            </UiButton>
          </div>
        </header>

        <EmptyState
          v-if="!items.length"
          size="inline"
          description="暂无已接入的服务器"
        />

        <div v-else class="fleet-node-list">
          <div
            v-for="item in items"
            :key="item.id"
            class="fleet-node-row"
            :class="{
              'is-selected':
                item.proxyServerId &&
                selectedServerIds.has(Number(item.proxyServerId)),
            }"
          >
            <div class="fleet-node-left">
              <el-checkbox
                v-if="item.proxyServerId"
                :model-value="selectedServerIds.has(Number(item.proxyServerId))"
                @change="toggleServerSelect(item.proxyServerId)"
              />
              <StatusDot :state="item.online ? 'ok' : 'idle'" />
              <div class="fleet-node-meta">
                <strong>{{ item.name }}</strong>
                <span class="mono">
                  {{ agentLabel(item) }} · {{ agentState(item) }}
                </span>
              </div>
            </div>

            <div class="fleet-node-actions">
              <UiButton
                v-if="
                  item.agentUpgradeable !== false &&
                  (item.agentUpdateAvailable ||
                    item.agentUpdateRequested ||
                    item.agentUpdateError)
                "
                size="sm"
                :variant="item.agentUpdateRequested ? 'ghost' : 'primary'"
                :loading="upgradingId === item.id"
                :disabled="upgradingId !== null || item.removing"
                @click="
                  emit(
                    'upgrade',
                    item,
                    !!item.agentUpdateRequested && !item.agentUpdateError
                  )
                "
              >
                {{
                  item.agentUpdateError
                    ? "重试"
                    : item.agentUpdateRequested
                      ? "撤销"
                      : "升级"
                }}
              </UiButton>
              <UiButton
                v-if="item.proxyServerId"
                size="sm"
                variant="ghost"
                @click="emit('open-proxy')"
              >
                节点
              </UiButton>
            </div>
          </div>
        </div>
      </section>

      <!-- Card 2: Panel URL Migration -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>面板地址迁移</h3>
            <span class="fleet-pill">{{ targetScopeLabel }}</span>
          </div>
        </header>

        <div class="fleet-url-row">
          <el-input
            v-model="migrateUrl"
            placeholder="https://new-panel.example.com"
            clearable
          />
          <UiButton
            variant="primary"
            size="sm"
            :loading="migrating"
            :disabled="!linkedOnlineServerIds.length"
            @click="handleMigrateURL"
          >
            一键下发 URL
          </UiButton>
        </div>

        <div v-if="migrateResults.length" class="fleet-result-list">
          <div
            v-for="r in migrateResults"
            :key="r.serverId"
            class="fleet-result-chip"
            :class="r.ok ? 'is-ok' : 'is-err'"
          >
            <strong>{{ r.serverName }}</strong>
            <span>{{ r.message }}</span>
          </div>
        </div>
      </section>

      <!-- Card 3: Security & Privacy Policy -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>安全与隐私权限</h3>
            <span class="fleet-pill">{{ targetScopeLabel }}</span>
          </div>
          <UiButton
            variant="primary"
            size="sm"
            :loading="savingPolicy"
            :disabled="!proxyServers.length"
            @click="handleSavePolicy"
          >
            应用策略
          </UiButton>
        </header>

        <div class="fleet-settings">
          <div class="fleet-setting-row">
            <span>WebShell 远程终端</span>
            <el-switch v-model="policy.terminalEnabled" />
          </div>
          <div class="fleet-setting-row">
            <span>文件浏览与上传</span>
            <el-switch v-model="policy.filesEnabled" />
          </div>
          <div class="fleet-setting-row">
            <span>Docker 容器管理</span>
            <el-switch v-model="policy.dockerEnabled" />
          </div>
          <div class="fleet-setting-row">
            <span>自动防火墙管理</span>
            <el-switch v-model="policy.autoFirewall" />
          </div>
          <div class="fleet-setting-row">
            <span>站点流量统计</span>
            <el-switch v-model="policy.websiteTrafficEnabled" />
          </div>
          <div class="fleet-setting-row">
            <span>允许远程自更新</span>
            <el-switch v-model="policy.allowAutoUpdate" />
          </div>
        </div>
      </section>

      <!-- Card 4: Native Unlock Matrix -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>原生解锁矩阵</h3>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            :loading="checkingUnlock"
            :disabled="!linkedOnlineServerIds.length"
            @click="handleUnlockCheck()"
          >
            测全服解锁
          </UiButton>
        </header>

        <div v-if="proxyServers.length" class="fleet-unlock-list">
          <div
            v-for="s in proxyServers.filter((x) => x.kind === 'agent')"
            :key="s.id"
            class="fleet-unlock-row"
          >
            <div class="fleet-unlock-head">
              <div class="fleet-node-left">
                <StatusDot :state="s.online ? 'ok' : 'idle'" />
                <strong>{{ s.name }}</strong>
                <span v-if="s.unlock?.country" class="fleet-pill mono">
                  {{ s.unlock.country }}
                </span>
              </div>
              <button
                v-if="s.online"
                type="button"
                class="fleet-link-btn"
                :disabled="checkingUnlock"
                @click="handleUnlockCheck(s.id)"
              >
                检测
              </button>
            </div>

            <div
              v-if="visibleUnlockItems(s.unlock?.items).length"
              class="fleet-unlock-pills"
            >
              <UiTag
                v-for="it in visibleUnlockItems(s.unlock?.items)"
                :key="it.key"
                :tone="it.status === 'available' ? 'ok' : 'warn'"
              >
                {{ formatPillText(it) }}
              </UiTag>
            </div>
            <div v-else class="fleet-unlock-empty">未检测</div>
          </div>
        </div>
      </section>
    </div>
  </UiDrawer>
</template>

<style scoped>
.fleet-stack {
  display: grid;
  gap: 20px;
  padding-bottom: 16px;
}

.fleet-card {
  display: grid;
  gap: 16px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
}

.fleet-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.fleet-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.fleet-title-group h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--c-ink);
}

.fleet-head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.fleet-pill {
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 500;
}

.fleet-pill.is-ok {
  color: var(--c-ok);
}

.fleet-node-list {
  display: grid;
  gap: 10px;
}

.fleet-node-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
  transition: border-color 160ms ease;
}

.fleet-node-row.is-selected {
  border-color: var(--c-ink);
}

.fleet-node-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.fleet-node-meta {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.fleet-node-meta strong {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.fleet-node-meta span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.fleet-node-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.fleet-url-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  align-items: center;
}

.fleet-result-list {
  display: grid;
  gap: 8px;
}

.fleet-result-chip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 12px;
  border-radius: 12px;
  font-size: 12px;
  background: var(--c-surface-2);
}

.fleet-result-chip.is-ok span {
  color: var(--c-ok);
}

.fleet-result-chip.is-err span {
  color: var(--c-bad);
}

.fleet-settings {
  display: grid;
}

.fleet-setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 13px 2px;
  border-bottom: 1px solid var(--c-line);
  font-size: 13.5px;
  color: var(--c-ink);
}

.fleet-setting-row:first-child {
  padding-top: 4px;
}

.fleet-setting-row:last-child {
  padding-bottom: 2px;
  border-bottom: none;
}

.fleet-unlock-list {
  display: grid;
  gap: 14px;
}

.fleet-unlock-row {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
}

.fleet-unlock-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.fleet-unlock-head strong {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.fleet-link-btn {
  border: none;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 8px;
  transition: opacity 140ms ease, transform 140ms ease;
}

.fleet-link-btn:hover:not(:disabled) {
  opacity: 0.7;
}

.fleet-link-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.fleet-link-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.fleet-unlock-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.fleet-unlock-empty {
  font-size: 12px;
  color: var(--c-muted);
}

@media (max-width: 560px) {
  .fleet-url-row {
    grid-template-columns: 1fr;
  }
}
</style>
