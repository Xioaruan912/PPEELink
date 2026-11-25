<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Delete, Download, Key, Refresh } from "@element-plus/icons-vue";
import { StatusDot, UiButton, UiDrawer } from "@/ui";
import {
  deleteProbe,
  pushProbeConfig,
  toggleProbeAuto,
  updateProbe,
  type Probe,
} from "@/api/subcription/probe";

const props = defineProps<{
  modelValue: boolean;
  probes: Probe[];
}>();

const emit = defineEmits<{
  (e: "update:modelValue", v: boolean): void;
  (e: "refresh"): void;
  (e: "reset-token", probe: Probe): void;
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit("update:modelValue", v),
});

const selectedIds = ref<Set<number>>(new Set());
const batchBusy = ref(false);
const rowStatus = ref<
  Record<number, { state: "running" | "ok" | "err"; text: string }>
>({});

// Card 2: Panel URL Migration
const migrateUrl = ref("");
const migratingUrl = ref(false);
const urlResults = ref<
  { probeId: number; probeName: string; ok: boolean; message: string }[]
>([]);

// Card 3: Privacy & Runtime Parameters
const runtimeForm = reactive({
  autoProbe: false,
  reportPublicIp: true,
  allowClash: true,
  concurrency: 4,
});
const savingRuntime = ref(false);

// Card 4: Clash External Control
const clashForm = reactive({
  clashApi: "http://127.0.0.1:9090",
  clashSecret: "",
});
const savingClash = ref(false);

const latestVersion = computed(
  () => props.probes.find((p) => p.latestVersion)?.latestVersion || "1.11"
);

const onlineProbes = computed(() => props.probes.filter((p) => p.online));
const updatableOnline = computed(() =>
  props.probes.filter((p) => p.online && p.updateAvailable)
);

const selectedProbes = computed(() =>
  props.probes.filter((p) => selectedIds.value.has(p.id))
);
const selectedOnline = computed(() =>
  selectedProbes.value.filter((p) => p.online)
);

const targetProbeIds = computed(() => {
  if (selectedIds.value.size > 0) {
    return Array.from(selectedIds.value);
  }
  return onlineProbes.value.map((p) => p.id);
});

const targetScopeLabel = computed(() => {
  if (selectedIds.value.size > 0) {
    return `已选 ${selectedIds.value.size} 台`;
  }
  return `全部在线 (${onlineProbes.value.length})`;
});

function toggleOne(id: number) {
  const next = new Set(selectedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  selectedIds.value = next;
}

function hydrateFromProbes() {
  const sample =
    selectedProbes.value[0] || onlineProbes.value[0] || props.probes[0];
  if (!sample) return;
  runtimeForm.autoProbe = Boolean(sample.autoProbe);
  runtimeForm.reportPublicIp = sample.reportPublicIp ?? true;
  runtimeForm.allowClash = sample.allowClash ?? true;
  runtimeForm.concurrency = sample.concurrency || 4;
  clashForm.clashApi = sample.clashApi || "http://127.0.0.1:9090";
}

watch(
  () => props.modelValue,
  (open) => {
    if (open) {
      urlResults.value = [];
      if (!migrateUrl.value) {
        migrateUrl.value = window.location.origin;
      }
      hydrateFromProbes();
    } else {
      selectedIds.value = new Set();
      rowStatus.value = {};
    }
  }
);

async function runBatchUpgrade(targets: Probe[]) {
  const onlineTargets = targets.filter((p) => p.online);
  if (!onlineTargets.length) {
    ElMessage.warning("没有可升级的在线设备");
    return;
  }
  batchBusy.value = true;
  for (const p of onlineTargets) {
    rowStatus.value[p.id] = { state: "running", text: "正在更新…" };
  }
  let okCount = 0;
  await Promise.allSettled(
    onlineTargets.map(async (p) => {
      try {
        await updateProbe(p.id);
        okCount++;
        rowStatus.value[p.id] = {
          state: "ok",
          text: `已升级至 v${latestVersion.value}`,
        };
      } catch (err: any) {
        rowStatus.value[p.id] = {
          state: "err",
          text: err?.message || "更新失败",
        };
      }
    })
  );
  batchBusy.value = false;
  if (okCount === onlineTargets.length) {
    ElMessage.success(`已向 ${okCount} 台设备下发静默升级`);
  } else {
    ElMessage.warning(`升级下发完成：${okCount}/${onlineTargets.length} 成功`);
  }
  setTimeout(() => {
    emit("refresh");
  }, 4000);
}

async function runBatchDelete() {
  if (!selectedProbes.value.length) return;
  const onlineCnt = selectedOnline.value.length;
  const tip =
    onlineCnt > 0
      ? `确定删除选中的 ${selectedProbes.value.length} 台设备？其中 ${onlineCnt} 台在线设备将执行无痕卸载。`
      : `确定移除选中的 ${selectedProbes.value.length} 台离线设备？`;
  try {
    await ElMessageBox.confirm(tip, "批量无痕卸载", {
      type: "warning",
      confirmButtonText: "确认删除",
      cancelButtonText: "取消",
    });
  } catch {
    return;
  }
  batchBusy.value = true;
  let okCount = 0;
  await Promise.allSettled(
    selectedProbes.value.map(async (p) => {
      try {
        await deleteProbe(p.id);
        okCount++;
      } catch {
        // ignore individual failure
      }
    })
  );
  batchBusy.value = false;
  selectedIds.value = new Set();
  ElMessage.success(`已完成 ${okCount} 台设备删除`);
  emit("refresh");
}

async function handleSingleDelete(p: Probe) {
  const tip = p.online
    ? `将通知「${p.name}」自动无痕卸载并从面板移除。`
    : `确定删除离线设备「${p.name}」？`;
  try {
    await ElMessageBox.confirm(tip, "无痕卸载并删除", {
      type: "warning",
      confirmButtonText: p.online ? "无痕卸载" : "删除",
      cancelButtonText: "取消",
    });
    const res = (await deleteProbe(p.id)) as any;
    ElMessage.success(res?.msg || "已删除");
    selectedIds.value.delete(p.id);
    emit("refresh");
  } catch (err: any) {
    if (err?.message) ElMessage.error(err.message);
  }
}

async function handleMigrateURL() {
  const url = migrateUrl.value.trim().replace(/\/+$/, "");
  if (!url.startsWith("http://") && !url.startsWith("https://")) {
    ElMessage.warning("请输入 http:// 或 https:// 开头的面板地址");
    return;
  }
  if (!targetProbeIds.value.length) {
    ElMessage.warning("暂无在线设备可下发");
    return;
  }
  try {
    await ElMessageBox.confirm(
      `将向 ${targetProbeIds.value.length} 台设备下发新面板地址「${url}」，设备会在连通预检通过后热切换。`,
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
  migratingUrl.value = true;
  urlResults.value = [];
  try {
    const { data } = await pushProbeConfig({
      probeIds:
        selectedIds.value.size > 0 ? targetProbeIds.value : undefined,
      panelUrl: url,
    });
    urlResults.value = data?.results || [];
    if (data?.okCount === data?.total && (data?.total || 0) > 0) {
      ElMessage.success(`已向 ${data.okCount} 台设备完成地址切换`);
    } else {
      ElMessage.warning(
        `切换完成：${data?.okCount || 0}/${data?.total || 0} 成功`
      );
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "地址下发失败");
  } finally {
    migratingUrl.value = false;
  }
}

async function handleSaveRuntime() {
  const targets =
    selectedIds.value.size > 0 ? selectedProbes.value : props.probes;
  if (!targets.length) return;
  savingRuntime.value = true;
  try {
    await Promise.allSettled(
      targets.map((p) => toggleProbeAuto(p.id, runtimeForm.autoProbe))
    );
    const { data } = await pushProbeConfig({
      probeIds:
        selectedIds.value.size > 0 ? targetProbeIds.value : undefined,
      reportPublicIp: runtimeForm.reportPublicIp,
      allowClash: runtimeForm.allowClash,
      concurrency: runtimeForm.concurrency,
    });
    ElMessage.success(
      `参数已保存，实时生效 ${data?.okCount || 0}/${data?.total || 0} 台`
    );
    emit("refresh");
  } catch (err: any) {
    ElMessage.error(err?.message || "参数下发失败");
  } finally {
    savingRuntime.value = false;
  }
}

async function handleSaveClash() {
  const api = clashForm.clashApi.trim();
  if (!api.startsWith("http://") && !api.startsWith("https://")) {
    ElMessage.warning("请输入有效的 Clash API 地址");
    return;
  }
  savingClash.value = true;
  try {
    const { data } = await pushProbeConfig({
      probeIds:
        selectedIds.value.size > 0 ? targetProbeIds.value : undefined,
      clashApi: api,
      clashSecret: clashForm.clashSecret,
    });
    ElMessage.success(
      `Clash 设置已下发 (${data?.okCount || 0}/${data?.total || 0})`
    );
    emit("refresh");
  } catch (err: any) {
    ElMessage.error(err?.message || "下发失败");
  } finally {
    savingClash.value = false;
  }
}
</script>

<template>
  <UiDrawer
    v-model="visible"
    title="用户 Agent 管理"
    size="min(680px, 100vw)"
    append-to-body
  >
    <div class="fleet-stack">
      <!-- Card 1: Fleet Version & Devices -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>版本与设备</h3>
            <span class="fleet-pill mono">v{{ latestVersion }}</span>
            <span class="fleet-pill is-ok mono">
              在线 {{ onlineProbes.length }}/{{ probes.length }}
            </span>
          </div>
          <div class="fleet-head-actions">
            <UiButton
              variant="ghost"
              size="sm"
              :icon="Refresh"
              :disabled="batchBusy"
              @click="emit('refresh')"
            >
              刷新
            </UiButton>
            <UiButton
              v-if="selectedIds.size > 0"
              variant="danger"
              size="sm"
              :disabled="batchBusy"
              @click="runBatchDelete"
            >
              卸载 ({{ selectedIds.size }})
            </UiButton>
            <UiButton
              v-if="updatableOnline.length"
              variant="primary"
              size="sm"
              :icon="Download"
              :loading="batchBusy"
              @click="runBatchUpgrade(updatableOnline)"
            >
              升级全部 ({{ updatableOnline.length }})
            </UiButton>
          </div>
        </header>

        <div v-if="probes.length" class="fleet-node-list">
          <article
            v-for="p in probes"
            :key="p.id"
            class="fleet-node-row"
            :class="{ 'is-selected': selectedIds.has(p.id) }"
          >
            <div class="fleet-node-left">
              <el-checkbox
                :model-value="selectedIds.has(p.id)"
                @change="toggleOne(p.id)"
              />
              <StatusDot :state="p.online ? 'ok' : 'idle'" />
              <div class="fleet-node-meta">
                <div class="fleet-node-title">
                  <strong>{{ p.name }}</strong>
                  <span v-if="p.region" class="fleet-mini-tag">{{ p.region }}</span>
                  <span v-if="p.isp" class="fleet-mini-tag">{{ p.isp }}</span>
                </div>
                <span class="mono">
                  {{ p.version ? `v${p.version}` : "未知" }}
                  <template v-if="p.updateAvailable">
                    → v{{ latestVersion }}
                  </template>
                  <template v-if="rowStatus[p.id]">
                    · {{ rowStatus[p.id].text }}
                  </template>
                </span>
              </div>
            </div>

            <div class="fleet-node-actions">
              <button
                v-if="p.online"
                type="button"
                class="fleet-pill-btn"
                :class="{ 'is-primary': p.updateAvailable }"
                :disabled="batchBusy || rowStatus[p.id]?.state === 'running'"
                @click="runBatchUpgrade([p])"
              >
                {{ p.updateAvailable ? "升级" : "重装" }}
              </button>
              <button
                type="button"
                class="fleet-icon-btn"
                title="重置令牌"
                @click="emit('reset-token', p)"
              >
                <el-icon><Key /></el-icon>
              </button>
              <button
                type="button"
                class="fleet-icon-btn is-danger"
                :title="p.online ? '无痕卸载' : '删除设备'"
                @click="handleSingleDelete(p)"
              >
                <el-icon><Delete /></el-icon>
              </button>
            </div>
          </article>
        </div>

        <div v-else class="fleet-empty">暂无已配置的用户 Agent 设备</div>
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
            :loading="migratingUrl"
            :disabled="!onlineProbes.length"
            @click="handleMigrateURL"
          >
            一键下发 URL
          </UiButton>
        </div>

        <div v-if="urlResults.length" class="fleet-result-list">
          <div
            v-for="r in urlResults"
            :key="r.probeId"
            class="fleet-result-chip"
            :class="r.ok ? 'is-ok' : 'is-err'"
          >
            <strong>{{ r.probeName }}</strong>
            <span>{{ r.message }}</span>
          </div>
        </div>
      </section>

      <!-- Card 3: Privacy & Runtime Parameters -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>隐私与运行参数</h3>
            <span class="fleet-pill">{{ targetScopeLabel }}</span>
          </div>
          <UiButton
            variant="primary"
            size="sm"
            :loading="savingRuntime"
            :disabled="!probes.length"
            @click="handleSaveRuntime"
          >
            应用参数
          </UiButton>
        </header>

        <div class="fleet-settings">
          <div class="fleet-setting-row">
            <span>1 分钟自动采样</span>
            <el-switch v-model="runtimeForm.autoProbe" />
          </div>
          <div class="fleet-setting-row">
            <span>上报本机公网 IP 与运营商</span>
            <el-switch v-model="runtimeForm.reportPublicIp" />
          </div>
          <div class="fleet-setting-row">
            <span>允许 Clash / Mihomo 联动</span>
            <el-switch v-model="runtimeForm.allowClash" />
          </div>
          <div class="fleet-setting-row">
            <span>并发探测线程数</span>
            <div class="fleet-seg">
              <button
                v-for="n in [2, 4, 6, 8]"
                :key="n"
                type="button"
                class="fleet-seg-btn mono"
                :class="{ 'is-active': runtimeForm.concurrency === n }"
                @click="runtimeForm.concurrency = n"
              >
                {{ n }}
              </button>
            </div>
          </div>
        </div>
      </section>

      <!-- Card 4: Clash External Control -->
      <section class="fleet-card">
        <header class="fleet-card-head">
          <div class="fleet-title-group">
            <h3>Clash 外部控制</h3>
            <span class="fleet-pill">{{ targetScopeLabel }}</span>
          </div>
          <UiButton
            variant="ghost"
            size="sm"
            :loading="savingClash"
            :disabled="!onlineProbes.length"
            @click="handleSaveClash"
          >
            下发设置
          </UiButton>
        </header>

        <div class="fleet-clash-grid">
          <el-input
            v-model="clashForm.clashApi"
            placeholder="http://127.0.0.1:9090"
            clearable
          />
          <el-input
            v-model="clashForm.clashSecret"
            type="password"
            show-password
            placeholder="API Secret（留空不变）"
            clearable
          />
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
  flex-wrap: wrap;
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
  gap: 3px;
  min-width: 0;
}

.fleet-node-title {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.fleet-node-title strong {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.fleet-mini-tag {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 10.5px;
}

.fleet-node-meta > span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.fleet-node-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.fleet-pill-btn {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
  transition: border-color 140ms ease, transform 140ms ease;
}

.fleet-pill-btn:hover:not(:disabled) {
  border-color: var(--c-ink);
}

.fleet-pill-btn:active:not(:disabled) {
  transform: scale(0.96);
}

.fleet-pill-btn.is-primary {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.fleet-pill-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.fleet-icon-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-muted);
  cursor: pointer;
  transition: color 140ms ease, border-color 140ms ease, transform 140ms ease;
}

.fleet-icon-btn:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.fleet-icon-btn:active {
  transform: scale(0.95);
}

.fleet-icon-btn.is-danger:hover {
  color: var(--c-bad);
  border-color: var(--c-bad);
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

.fleet-seg {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px;
  border-radius: 999px;
  background: var(--c-surface-2);
}

.fleet-seg-btn {
  min-width: 34px;
  height: 26px;
  border: none;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background 140ms ease, color 140ms ease;
}

.fleet-seg-btn.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.fleet-clash-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.fleet-empty {
  padding: 28px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--c-muted);
  border: 1px dashed var(--c-line);
  border-radius: 14px;
}

@media (max-width: 560px) {
  .fleet-url-row,
  .fleet-clash-grid {
    grid-template-columns: 1fr;
  }
}
</style>
