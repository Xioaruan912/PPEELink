<script setup lang="ts">
import { computed, ref } from "vue";
import { Download, Upload } from "@element-plus/icons-vue";
import { exportSafeBackup, importSafeBackup } from "@/api/ops";
import { UiButton, UiDialog, UiDrawer, UiStat } from "@/ui";
import { formatDateTime } from "@/utils/date";
const props = defineProps<{ summary: Record<string, any>; loading: boolean }>();
const emit = defineEmits<{ restored: [] }>();
const fileInput = ref<HTMLInputElement>();
const preview = ref<Record<string, any> | null>(null);
const previewVisible = ref(false);
const confirmVisible = ref(false);
const acknowledged = ref(false);
const filename = ref("");
const fileError = ref("");
const busy = ref(false);
const exporting = ref(false);
const metrics = [
  { key: "templates", label: "模板" },
  { key: "egressTargets", label: "检测目标" },
  { key: "subscriptions", label: "订阅配置" },
  { key: "airports", label: "机场设置" },
];
const previewCounts = computed<Record<string, number>>(() => ({
  templates: Object.keys(preview.value?.templates || {}).length,
  egressTargets: preview.value?.egressTargets?.length || 0,
  subscriptions: preview.value?.subscriptions?.length || 0,
  airports: preview.value?.airports?.length || 0,
}));
const pretty = computed(() => JSON.stringify(preview.value, null, 2));
const object = (value: unknown): value is Record<string, any> =>
  !!value && typeof value === "object" && !Array.isArray(value);
function validateBackup(value: unknown): asserts value is Record<string, any> {
  if (!object(value) || value.version !== 1)
    throw new Error("仅支持 version 为 1 的安全配置备份。");
  if (
    !object(value.templates) ||
    !Object.values(value.templates).every((item) => typeof item === "string")
  )
    throw new Error("模板应为文件名与文本内容的映射。");
  for (const key of ["egressTargets", "subscriptions", "airports"]) {
    if (!Array.isArray(value[key]) || !value[key].every(object))
      throw new Error(`${key} 格式无效，应为配置数组。`);
  }
  if (
    !value.egressTargets.every(
      (item: any) =>
        typeof item.key === "string" &&
        typeof item.name === "string" &&
        typeof item.domain === "string"
    )
  )
    throw new Error("检测目标缺少 key、name 或 domain。");
  if (
    !value.subscriptions.every(
      (item: any) =>
        typeof item.name === "string" &&
        typeof item.config === "string" &&
        typeof item.pipeline === "string"
    )
  )
    throw new Error("订阅配置缺少名称、配置或流水线文本。");
  if (
    !value.airports.every(
      (item: any) =>
        typeof item.name === "string" &&
        typeof item.autoCleanup === "boolean" &&
        typeof item.isDedicated === "boolean"
    )
  )
    throw new Error("机场设置缺少名称或开关配置。");
  if (
    !object(value.alert) ||
    typeof value.alert.enabled !== "boolean" ||
    !Number.isInteger(value.alert.failureThreshold) ||
    typeof value.alert.maintenanceStart !== "string" ||
    typeof value.alert.maintenanceEnd !== "string"
  )
    throw new Error("告警配置格式无效，请选择完整的安全备份。");
}
async function selectFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (!file) return;
  fileError.value = "";
  try {
    const parsed: unknown = JSON.parse(await file.text());
    validateBackup(parsed);
    preview.value = parsed;
    filename.value = file.name;
    acknowledged.value = false;
    previewVisible.value = true;
  } catch (error) {
    preview.value = null;
    fileError.value =
      error instanceof Error
        ? error.message
        : "文件读取失败，请重新选择 JSON 备份。";
  }
}
async function downloadBackup() {
  exporting.value = true;
  try {
    const data = await exportSafeBackup();
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    try {
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = `sublinkx-safe-backup-${new Date().toISOString().slice(0, 10)}.json`;
      document.body.append(anchor);
      anchor.click();
      anchor.remove();
    } finally {
      URL.revokeObjectURL(url);
    }
  } catch {
    ElMessage.error("备份导出失败，请重试。");
  } finally {
    exporting.value = false;
  }
}
async function restore() {
  if (
    !preview.value ||
    !confirmVisible.value ||
    !acknowledged.value ||
    busy.value
  )
    return;
  busy.value = true;
  try {
    const res: any = await importSafeBackup(preview.value);
    ElMessage.success(res?.msg || res?.data?.msg || "安全配置已恢复");
    confirmVisible.value = false;
    previewVisible.value = false;
    preview.value = null;
    emit("restored");
  } catch {
    /* 保留预览，便于确认后重试。 */
  } finally {
    busy.value = false;
  }
}
function requestRestore() {
  if (!preview.value) return;
  acknowledged.value = false;
  confirmVisible.value = true;
}
function cancelRestore() {
  if (!busy.value) confirmVisible.value = false;
}
function clearPreview() {
  if (!confirmVisible.value && !busy.value) preview.value = null;
}
</script>
<template>
  <section class="backup-section" data-enter>
    <header class="section-heading">
      <h2>安全备份</h2>
      <p>导出安全配置，或预览文件后合并恢复。</p>
    </header>
    <div class="backup-summary">
      <UiStat
        v-for="metric in metrics"
        :key="metric.key"
        size="small"
        :label="metric.label"
        :value="loading ? null : Number(props.summary[metric.key] || 0)"
      />
    </div>
    <p class="backup-note">
      备份排除密码、Token、节点链接、机场源 URL 与 Webhook。
    </p>
    <p class="backup-note">
      恢复会覆盖同名模板、同 key
      检测目标及同名订阅与机场的安全设置，并更新告警配置。
    </p>
    <div class="backup-actions">
      <UiButton :icon="Download" :loading="exporting" @click="downloadBackup"
        >导出安全备份</UiButton
      ><UiButton :icon="Upload" @click="fileInput?.click()"
        >选择备份恢复</UiButton
      >
    </div>
    <input
      ref="fileInput"
      class="file-input"
      type="file"
      accept=".json,application/json"
      aria-label="选择安全备份文件"
      tabindex="-1"
      @change="selectFile"
    />
    <p v-if="fileError" class="error-text" role="alert">{{ fileError }}</p>
    <UiDrawer
      v-model="previewVisible"
      title="备份预览"
      size="760px"
      @closed="clearPreview"
      ><template v-if="preview"
        ><p class="file-name">{{ filename }}</p>
        <p class="backup-note">
          版本 {{ preview.version }} · 生成于
          {{ formatDateTime(preview.generatedAt) || "未记录" }}
        </p>
        <div class="backup-summary preview-summary">
          <UiStat
            v-for="metric in metrics"
            :key="metric.key"
            size="small"
            :label="metric.label"
            :value="previewCounts[metric.key]"
          />
        </div>
        <h3>拟导入内容</h3>
        <dl class="preview-details">
          <dt>模板文件</dt>
          <dd>{{ Object.keys(preview.templates).join("、") || "无" }}</dd>
          <dt>检测目标</dt>
          <dd>
            {{
              preview.egressTargets
                .map((item: any) => `${item.name} (${item.key})`)
                .join("、") || "无"
            }}
          </dd>
          <dt>订阅</dt>
          <dd>
            {{
              preview.subscriptions.map((item: any) => item.name).join("、") ||
              "无"
            }}
          </dd>
          <dt>机场</dt>
          <dd>
            {{
              preview.airports.map((item: any) => item.name).join("、") || "无"
            }}
          </dd>
          <dt>告警</dt>
          <dd>
            {{ preview.alert.enabled ? "启用" : "停用" }} · 连续失败阈值
            {{ preview.alert.failureThreshold }} · 维护
            {{ preview.alert.maintenanceStart || "未设置" }} 至
            {{ preview.alert.maintenanceEnd || "未设置" }}
          </dd>
        </dl>
        <h3>完整配置</h3>
        <pre class="backup-json">{{ pretty }}</pre></template
      ><template #footer
        ><UiButton :disabled="busy" @click="previewVisible = false"
          >关闭预览</UiButton
        ><UiButton variant="danger" :disabled="!preview" @click="requestRestore"
          >合并恢复</UiButton
        ></template
      ></UiDrawer
    >
    <UiDialog
      v-model="confirmVisible"
      title="确认恢复配置"
      width="560px"
      :close-on-click-modal="!busy"
      :close-on-press-escape="!busy"
      :show-close="!busy"
      ><p>
        将从「{{ filename }}」恢复 {{ previewCounts.templates }} 个模板、{{
          previewCounts.egressTargets
        }}
        个检测目标、{{ previewCounts.subscriptions }} 份订阅配置和
        {{ previewCounts.airports }} 项机场设置。
      </p>
      <p class="error-text">
        同名配置会被覆盖，告警设置也会更新。此操作会改变现有运行配置，请先导出当前备份。
      </p>
      <p class="backup-note">
        合并不会删除节点、订阅关系或敏感凭据；无同名订阅和机场时不会新建。
      </p>
      <el-checkbox v-model="acknowledged" :disabled="busy"
        >我已核对预览，确认覆盖上述安全配置</el-checkbox
      ><template #footer
        ><div class="confirm-actions">
          <UiButton :disabled="busy" @click="cancelRestore">返回预览</UiButton
          ><UiButton
            variant="danger"
            :disabled="!acknowledged"
            :loading="busy"
            @click="restore"
            >确认恢复</UiButton
          >
        </div></template
      ></UiDialog
    >
  </section>
</template>
<style scoped>
.backup-section {
  min-width: 0;
  padding: 32px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}
.section-heading h2 {
  font-size: 22px;
  margin: 0 0 8px;
}
.section-heading p,
.backup-note {
  color: var(--c-muted);
  font-size: 14px;
}
.section-heading p {
  margin: 0 0 24px;
}
.backup-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 24px;
  margin-bottom: 24px;
}
.backup-actions,
.confirm-actions {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}
.backup-actions {
  margin-top: 24px;
}
.confirm-actions {
  justify-content: flex-end;
}
.file-input {
  display: none;
}
.error-text {
  color: var(--c-bad);
  font-size: 14px;
  overflow-wrap: anywhere;
}
.file-name {
  font-family: var(--font-mono);
  overflow-wrap: anywhere;
}
.preview-summary {
  margin-top: 32px;
  grid-template-columns: repeat(4, minmax(0, 1fr));
}
.preview-details {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 16px;
  font-size: 14px;
}
.preview-details dt {
  color: var(--c-muted);
}
.preview-details dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.backup-json {
  max-height: 45vh;
  overflow: auto;
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface-2);
  color: var(--c-data);
  font: 400 14px/1.6 var(--font-mono);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
:deep(.el-checkbox) {
  height: auto;
  min-height: 44px;
  white-space: normal;
}
:deep(.el-checkbox__label) {
  white-space: normal;
}
@media (max-width: 767px) {
  .backup-section {
    padding: 24px 16px;
  }
  .preview-summary {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .backup-actions {
    flex-direction: column;
  }
}
</style>
