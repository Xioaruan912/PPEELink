<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { Delete, Download, Key, Refresh } from "@element-plus/icons-vue";
import { StatusDot, UiButton, UiDrawer } from "@/ui";
import {
  deleteProbe,
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
const rowStatus = ref<Record<number, { state: "running" | "ok" | "err"; text: string }>>({});

const latestVersion = computed(
  () => props.probes.find((p) => p.latestVersion)?.latestVersion || "1.3"
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

const allSelected = computed(
  () => props.probes.length > 0 && selectedIds.value.size === props.probes.length
);

function toggleAll() {
  if (allSelected.value) {
    selectedIds.value = new Set();
  } else {
    selectedIds.value = new Set(props.probes.map((p) => p.id));
  }
}

function toggleOne(id: number) {
  const next = new Set(selectedIds.value);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  selectedIds.value = next;
}

async function runBatchUpgrade(targets: Probe[]) {
  const onlineTargets = targets.filter((p) => p.online);
  if (!onlineTargets.length) {
    ElMessage.warning("没有可升级的在线设备");
    return;
  }
  batchBusy.value = true;
  for (const p of onlineTargets) {
    rowStatus.value[p.id] = { state: "running", text: "正在下发更新…" };
  }
  let okCount = 0;
  await Promise.allSettled(
    onlineTargets.map(async (p) => {
      try {
        await updateProbe(p.id);
        okCount++;
        rowStatus.value[p.id] = {
          state: "ok",
          text: `已下发，正在重启至 v${latestVersion.value}`,
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

async function runBatchAutoToggle(enable: boolean) {
  if (!selectedProbes.value.length) return;
  batchBusy.value = true;
  let okCount = 0;
  await Promise.allSettled(
    selectedProbes.value.map(async (p) => {
      try {
        await toggleProbeAuto(p.id, enable);
        okCount++;
      } catch {
        // ignore individual failure
      }
    })
  );
  batchBusy.value = false;
  ElMessage.success(
    enable
      ? `已为 ${okCount} 台设备开启 1 分钟曲线记录`
      : `已为 ${okCount} 台设备关闭定时记录`
  );
  emit("refresh");
}

async function runBatchDelete() {
  if (!selectedProbes.value.length) return;
  const onlineCnt = selectedOnline.value.length;
  const tip =
    onlineCnt > 0
      ? `确定删除选中的 ${selectedProbes.value.length} 台设备？其中 ${onlineCnt} 台在线设备将自动执行本机无痕卸载（清除所有文件与开机自启）。`
      : `确定从面板移除选中的 ${selectedProbes.value.length} 台离线设备？`;
  try {
    await ElMessageBox.confirm(tip, "批量无痕卸载并删除", {
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
    ? `将通知本机「${p.name}」自动退出并无痕删除所有本地文件与开机自启项，同时从面板移除。`
    : `删除「${p.name}」？（当前离线，仅从面板移除）`;
  try {
    await ElMessageBox.confirm(tip, "无痕卸载并删除", {
      type: "warning",
      confirmButtonText: p.online ? "无痕卸载并删除" : "删除",
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

watch(
  () => props.modelValue,
  (open) => {
    if (!open) {
      selectedIds.value = new Set();
      rowStatus.value = {};
    }
  }
);
</script>

<template>
  <UiDrawer
    v-model="visible"
    title="用户 Agent 管理"
    size="min(680px, 100vw)"
    append-to-body
  >
    <div class="am-root">
      <!-- Top Summary + One-Click Batch Upgrade -->
      <header class="am-summary">
        <div class="am-stats">
          <div class="am-stat">
            <span>全部设备</span>
            <strong class="mono">{{ probes.length }}</strong>
          </div>
          <div class="am-stat">
            <span>在线</span>
            <strong class="mono is-ok">{{ onlineProbes.length }}</strong>
          </div>
          <div class="am-stat">
            <span>可升级</span>
            <strong
              class="mono"
              :class="updatableOnline.length ? 'is-warn' : ''"
            >
              {{ updatableOnline.length }}
            </strong>
          </div>
          <div class="am-stat">
            <span>最新版本</span>
            <strong class="mono">v{{ latestVersion }}</strong>
          </div>
        </div>

        <div class="am-top-actions">
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
            variant="primary"
            size="sm"
            :icon="Download"
            :loading="batchBusy"
            :disabled="!updatableOnline.length"
            @click="runBatchUpgrade(updatableOnline)"
          >
            {{
              updatableOnline.length
                ? `一键批量升级 (${updatableOnline.length})`
                : "全部已是最新"
            }}
          </UiButton>
        </div>
      </header>

      <!-- Selection & Contextual Batch Bar -->
      <div v-if="probes.length" class="am-batch-bar">
        <label class="am-check-all">
          <el-checkbox
            :model-value="allSelected"
            :indeterminate="selectedIds.size > 0 && !allSelected"
            @change="toggleAll"
          />
          <span>
            {{
              selectedIds.size > 0 ? `已选 ${selectedIds.size} 台` : "全选设备"
            }}
          </span>
        </label>

        <div v-if="selectedIds.size > 0" class="am-batch-btns">
          <button
            type="button"
            class="am-mini-btn is-primary"
            :disabled="batchBusy || !selectedOnline.length"
            @click="runBatchUpgrade(selectedOnline)"
          >
            升级选中 ({{ selectedOnline.length }})
          </button>
          <button
            type="button"
            class="am-mini-btn"
            :disabled="batchBusy"
            @click="runBatchAutoToggle(true)"
          >
            开启 1m
          </button>
          <button
            type="button"
            class="am-mini-btn"
            :disabled="batchBusy"
            @click="runBatchAutoToggle(false)"
          >
            关闭 1m
          </button>
          <button
            type="button"
            class="am-mini-btn is-danger"
            :disabled="batchBusy"
            @click="runBatchDelete"
          >
            无痕卸载
          </button>
        </div>
      </div>

      <!-- Device Rows -->
      <div v-if="probes.length" class="am-list">
        <article
          v-for="p in probes"
          :key="p.id"
          class="am-row"
          :class="{ 'is-selected': selectedIds.has(p.id) }"
        >
          <div class="am-row-left">
            <el-checkbox
              :model-value="selectedIds.has(p.id)"
              @change="toggleOne(p.id)"
            />
            <div class="am-row-info">
              <div class="am-row-title">
                <StatusDot :state="p.online ? 'ok' : 'idle'" />
                <strong>{{ p.name }}</strong>
                <span v-if="p.region" class="am-chip">{{ p.region }}</span>
                <span v-if="p.isp" class="am-chip">{{ p.isp }}</span>
                <span
                  v-if="p.hasIpv6 !== null && p.hasIpv6 !== undefined"
                  class="am-chip mono"
                >
                  {{ p.hasIpv6 ? "v4+v6" : "仅v4" }}
                </span>
              </div>

              <div class="am-row-meta">
                <span
                  class="am-ver mono"
                  :class="{
                    'is-outdated': p.updateAvailable,
                    'is-latest': p.version && !p.updateAvailable,
                  }"
                >
                  {{ p.version ? `v${p.version}` : "未知版本" }}
                  <template v-if="p.updateAvailable">
                    → v{{ latestVersion }}
                  </template>
                </span>
                <span class="am-sep">·</span>
                <span class="am-sub">{{
                  p.autoProbe ? "1m 曲线记录中" : "按需模式"
                }}</span>
                <template v-if="rowStatus[p.id]">
                  <span class="am-sep">·</span>
                  <span
                    class="am-feedback"
                    :class="`is-${rowStatus[p.id].state}`"
                  >
                    {{ rowStatus[p.id].text }}
                  </span>
                </template>
              </div>
            </div>
          </div>

          <div class="am-row-actions">
            <button
              v-if="p.online"
              type="button"
              class="am-action-btn"
              :class="{ 'is-highlight': p.updateAvailable }"
              :disabled="batchBusy || rowStatus[p.id]?.state === 'running'"
              @click="runBatchUpgrade([p])"
            >
              <el-icon><Download /></el-icon>
              <span>{{ p.updateAvailable ? "升级" : "重装" }}</span>
            </button>
            <button
              type="button"
              class="am-icon-btn"
              title="重置令牌"
              @click="emit('reset-token', p)"
            >
              <el-icon><Key /></el-icon>
            </button>
            <button
              type="button"
              class="am-icon-btn is-danger"
              :title="p.online ? '无痕卸载并删除' : '删除设备'"
              @click="handleSingleDelete(p)"
            >
              <el-icon><Delete /></el-icon>
            </button>
          </div>
        </article>
      </div>

      <div v-else class="am-empty">暂无已配置的用户 Agent 设备。</div>
    </div>
  </UiDrawer>
</template>

<style scoped>
.am-root {
  display: grid;
  gap: 16px;
}

.am-summary {
  display: grid;
  gap: 14px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface-2);
}

.am-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.am-stat {
  display: grid;
  gap: 2px;
}

.am-stat span {
  font-size: 11px;
  color: var(--c-muted);
}

.am-stat strong {
  font-size: 18px;
  font-weight: 600;
  color: var(--c-ink);
}

.am-stat strong.is-ok {
  color: var(--c-ok);
}

.am-stat strong.is-warn {
  color: var(--c-warn, #d97706);
}

.am-top-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--c-line);
}

.am-batch-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
  padding: 0 4px;
}

.am-check-all {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
}

.am-batch-btns {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.am-mini-btn {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 11.5px;
  cursor: pointer;
  transition: border-color 140ms ease, opacity 140ms ease;
}

.am-mini-btn:hover:not(:disabled) {
  border-color: var(--c-ink);
}

.am-mini-btn.is-primary {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.am-mini-btn.is-danger {
  color: var(--c-bad);
}

.am-mini-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.am-list {
  display: grid;
  gap: 10px;
}

.am-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  transition: border-color 150ms ease;
}

.am-row.is-selected {
  border-color: var(--c-ink);
}

.am-row-left {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
}

.am-row-info {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.am-row-title {
  display: flex;
  align-items: center;
  gap: 7px;
  flex-wrap: wrap;
}

.am-row-title strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.am-chip {
  padding: 1px 7px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 10.5px;
}

.am-row-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 11.5px;
  color: var(--c-muted);
}

.am-ver.is-outdated {
  color: var(--c-warn, #d97706);
  font-weight: 600;
}

.am-ver.is-latest {
  color: var(--c-ok);
}

.am-feedback.is-running {
  color: var(--c-ink);
}

.am-feedback.is-ok {
  color: var(--c-ok);
}

.am-feedback.is-err {
  color: var(--c-bad);
}

.am-row-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-shrink: 0;
}

.am-action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 5px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.am-action-btn.is-highlight {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.am-action-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.am-icon-btn {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 1px solid var(--c-line);
  border-radius: 50%;
  background: var(--c-surface);
  color: var(--c-muted);
  cursor: pointer;
}

.am-icon-btn:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.am-icon-btn.is-danger:hover {
  color: var(--c-bad);
  border-color: var(--c-bad);
}

.am-empty {
  padding: 36px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--c-muted);
  border: 1px dashed var(--c-line);
  border-radius: 16px;
}
</style>
