<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { Delete, Plus, RefreshRight, TopRight } from "@element-plus/icons-vue";
import {
  createApiToken,
  getApiTokens,
  getAuditLogs,
  inspectSafeBackup,
  revokeApiToken,
} from "@/api/ops";
import {
  EmptyState,
  PageHeader,
  StatusDot,
  UiButton,
  UiDialog,
  UiDrawer,
  UiStat,
  UiTable,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import BackupSection from "./BackupSection.vue";
import { formatDateTime } from "@/utils/date";
defineOptions({ name: "OpsCenter" });
const tokens = ref<any[]>([]);
const backup = ref<Record<string, any>>({});
const audits = ref<any[]>([]);
const loading = ref(false);
const loadError = ref("");
const tokenDialog = ref(false);
const tokenForm = ref({
  name: "",
  scopes: ["read"] as string[],
  expiresInDays: 90,
});
const formError = ref("");
const creating = ref(false);
const createdToken = ref("");
const copying = ref(false);
const revoking = ref<number[]>([]);
const activeTokens = computed(
  () => tokens.value.filter((row) => tokenState(row) === "ok").length
);
const formatTime = (value: string | undefined) =>
  formatDateTime(value) || "暂无记录";
const prefix = (row: any) =>
  `${String(row.tokenPrefix || "").slice(0, 12) || "未记录"}••••`;
const tokenState = (row: any): "ok" | "idle" | "warn" =>
  !row.enabled
    ? "idle"
    : row.expiresAt && new Date(row.expiresAt).getTime() <= Date.now()
      ? "warn"
      : "ok";
const tokenLabel = (row: any) =>
  !row.enabled ? "已撤销" : tokenState(row) === "warn" ? "已过期" : "有效";
const scopes = (row: any) =>
  Array.isArray(row.scopes) ? row.scopes.join(", ") : row.scopes || "无";
async function load() {
  if (loading.value) return;
  loading.value = true;
  try {
    const results = await Promise.allSettled([
      getApiTokens(),
      inspectSafeBackup(),
      getAuditLogs(),
    ]);
    if (results[0].status === "fulfilled")
      tokens.value = results[0].value?.data || [];
    if (results[1].status === "fulfilled")
      backup.value = results[1].value?.data || {};
    if (results[2].status === "fulfilled")
      audits.value = results[2].value?.data || [];
    const names = ["API Token", "备份摘要", "审计日志"];
    loadError.value = results
      .flatMap((item, index) =>
        item.status === "rejected" ? names[index] : []
      )
      .join("、");
  } finally {
    loading.value = false;
  }
}
function openToken() {
  tokenForm.value = { name: "", scopes: ["read"], expiresInDays: 90 };
  formError.value = "";
  tokenDialog.value = true;
}
async function createToken() {
  const form = tokenForm.value;
  if (!form.name.trim()) {
    formError.value = "请输入 Token 名称。";
    return;
  }
  if (!form.scopes.length) {
    formError.value = "请选择至少一项权限。";
    return;
  }
  if (
    !Number.isInteger(form.expiresInDays) ||
    form.expiresInDays < 0 ||
    form.expiresInDays > 3650
  ) {
    formError.value = "有效期应为 0 至 3650 之间的整数。";
    return;
  }
  creating.value = true;
  formError.value = "";
  try {
    const res = await createApiToken({ ...form, name: form.name.trim() });
    createdToken.value = res?.data?.token || "";
    tokenDialog.value = false;
    await load();
  } catch {
    /* 保留已填写表单，API 层提示失败。 */
  } finally {
    creating.value = false;
  }
}
async function revoke(row: any) {
  try {
    await ElMessageBox.confirm(
      `撤销 API Token「${row.name}」？撤销后立即失效。`,
      "确认撤销",
      {
        type: "warning",
        confirmButtonText: "撤销 Token",
        cancelButtonText: "保留",
      }
    );
  } catch {
    return;
  }
  revoking.value.push(row.id);
  try {
    await revokeApiToken(row.id);
    ElMessage.success("已撤销");
    await load();
  } catch {
    /* API 层显示错误，列表保留以便重试。 */
  } finally {
    revoking.value = revoking.value.filter((id) => id !== row.id);
  }
}
const openStatus = () =>
  window.open("/status", "_blank", "noopener,noreferrer");
function setCreatedVisible(visible: boolean) {
  if (!visible) createdToken.value = "";
}
async function copyCreatedToken() {
  if (!createdToken.value) return;
  copying.value = true;
  try {
    await navigator.clipboard.writeText(createdToken.value);
    ElMessage.success("已复制");
  } catch {
    ElMessage.error("复制失败，请选中 Token 手动复制。");
  } finally {
    copying.value = false;
  }
}
onMounted(load);
</script>
<template>
  <div class="ops-page">
    <PageHeader
      title="运维中心"
      description="管理访问权限、安全备份与操作记录。"
      ><template #actions
        ><UiButton :icon="TopRight" @click="openStatus"
          >打开公开状态页</UiButton
        ></template
      ></PageHeader
    >
    <EmptyState
      v-if="loadError"
      size="section"
      title="部分数据读取失败"
      :description="`${loadError}未能加载，请重试。`"
      ><template #action
        ><UiButton @click="load">重新加载</UiButton></template
      ></EmptyState
    >
    <div class="ops-layout">
      <section class="token-section" data-enter>
        <header class="section-heading">
          <div>
            <h2>API Token</h2>
            <p>按需授权，明文仅在创建后显示一次。</p>
          </div>
          <UiButton variant="primary" :icon="Plus" @click="openToken"
            >创建 Token</UiButton
          >
        </header>
        <div class="token-summary">
          <UiStat
            size="large"
            label="有效 Token"
            :value="loading ? null : activeTokens"
          />
          <p>
            服务端仅保存 SHA-256 哈希。<br />列表只展示前缀，撤销后立即失效。
          </p>
        </div>
        <UiTable :rows="tokens" :loading="loading" label="API Token 列表">
          <el-table-column label="名称 / 权限" min-width="160"
            ><template #default="{ row }"
              ><div class="table-name">{{ row.name }}</div>
              <span class="muted mono">{{ scopes(row) }}</span></template
            ></el-table-column
          >
          <el-table-column label="前缀" min-width="160"
            ><template #default="{ row }"
              ><span class="mono">{{ prefix(row) }}</span></template
            ></el-table-column
          >
          <el-table-column label="到期" min-width="160"
            ><template #default="{ row }"
              ><span class="mono">{{
                row.expiresAt ? formatTime(row.expiresAt) : "永不过期"
              }}</span></template
            ></el-table-column
          >
          <el-table-column label="状态" width="104"
            ><template #default="{ row }"
              ><StatusDot
                :state="tokenState(row)"
                :label="tokenLabel(row)" /></template
          ></el-table-column>
          <el-table-column label="操作" width="64" align="right"
            ><template #default="{ row }"
              ><RowIconButton
                v-if="row.enabled"
                :icon="Delete"
                :label="`撤销 ${row.name}`"
                :loading="revoking.includes(row.id)"
                @click="revoke(row)" /></template
          ></el-table-column>
          <template #empty
            ><EmptyState
              size="section"
              title="尚无 API Token"
              description="创建仅包含所需权限的访问凭据。"
              ><template #action
                ><UiButton @click="openToken">创建 Token</UiButton></template
              ></EmptyState
            ></template
          >
          <template #mobile-card="{ row }"
            ><div class="mobile-heading">
              <h3>{{ row.name }}</h3>
              <RowIconButton
                v-if="row.enabled"
                :icon="Delete"
                :label="`撤销 ${row.name}`"
                :loading="revoking.includes(row.id)"
                @click="revoke(row)"
              />
            </div>
            <StatusDot :state="tokenState(row)" :label="tokenLabel(row)" />
            <dl class="mobile-fields">
              <dt>前缀</dt>
              <dd class="mono">{{ prefix(row) }}</dd>
              <dt>权限</dt>
              <dd class="mono">{{ scopes(row) }}</dd>
              <dt>到期</dt>
              <dd>
                {{ row.expiresAt ? formatTime(row.expiresAt) : "永不过期" }}
              </dd>
            </dl></template
          >
        </UiTable>
      </section>
      <BackupSection :summary="backup" :loading="loading" @restored="load" />
    </div>
    <section class="audit-section" data-enter>
      <header class="section-heading">
        <div>
          <h2>操作审计</h2>
          <p>记录操作者与请求信息，不记录请求体及凭据。</p>
        </div>
        <UiButton :icon="RefreshRight" :loading="loading" @click="load"
          >刷新记录</UiButton
        >
      </header>
      <UiTable
        :rows="audits"
        :loading="loading"
        label="管理操作审计"
        max-height="560"
      >
        <el-table-column label="时间" min-width="180"
          ><template #default="{ row }"
            ><span class="mono">{{ formatTime(row.createdAt) }}</span></template
          ></el-table-column
        ><el-table-column
          prop="actor"
          label="操作者"
          min-width="120"
        /><el-table-column
          prop="authType"
          label="认证"
          min-width="88"
        /><el-table-column label="IP" min-width="144"
          ><template #default="{ row }"
            ><span class="mono">{{ row.ip }}</span></template
          ></el-table-column
        ><el-table-column
          prop="method"
          label="方法"
          min-width="80"
        /><el-table-column label="路径" min-width="220"
          ><template #default="{ row }"
            ><span class="mono audit-path">{{ row.path }}</span></template
          ></el-table-column
        ><el-table-column label="状态" width="96"
          ><template #default="{ row }"
            ><StatusDot
              :state="Number(row.status) >= 400 ? 'bad' : 'idle'"
              :label="String(row.status)" /></template
        ></el-table-column>
        <template #empty
          ><EmptyState
            size="section"
            title="暂无操作记录"
            description="后续管理请求会在这里留下审计记录。"
        /></template>
        <template #mobile-card="{ row }"
          ><div class="mobile-heading">
            <span class="mono">{{ formatTime(row.createdAt) }}</span
            ><StatusDot
              :state="Number(row.status) >= 400 ? 'bad' : 'idle'"
              :label="String(row.status)"
            />
          </div>
          <p class="mono audit-path">{{ row.method }} {{ row.path }}</p>
          <dl class="mobile-fields">
            <dt>操作者</dt>
            <dd>{{ row.actor }}</dd>
            <dt>认证</dt>
            <dd>{{ row.authType }}</dd>
            <dt>IP</dt>
            <dd class="mono">{{ row.ip }}</dd>
          </dl></template
        >
      </UiTable>
    </section>
    <UiDrawer
      v-model="tokenDialog"
      title="创建 API Token"
      size="520px"
      :close-on-click-modal="!creating"
      :close-on-press-escape="!creating"
      :show-close="!creating"
      ><el-form label-position="top" @submit.prevent="createToken"
        ><el-form-item label="名称" required
          ><el-input
            v-model="tokenForm.name"
            maxlength="120"
            placeholder="例如：监控服务"
            :disabled="creating" /></el-form-item
        ><el-form-item label="权限" required
          ><el-checkbox-group v-model="tokenForm.scopes" :disabled="creating"
            ><el-checkbox value="read">read 读取</el-checkbox
            ><el-checkbox value="write">write 写入</el-checkbox
            ><el-checkbox value="admin"
              >admin 管理</el-checkbox
            ></el-checkbox-group
          ></el-form-item
        >
        <p class="muted form-note">
          write 包含读取权限；admin 可执行全部管理操作。
        </p>
        <el-form-item label="有效期（天，0 = 永久）"
          ><el-input-number
            v-model="tokenForm.expiresInDays"
            :min="0"
            :max="3650"
            :disabled="creating"
        /></el-form-item>
        <p v-if="formError" class="error-text" role="alert">
          {{ formError }}
        </p></el-form
      ><template #footer
        ><UiButton :disabled="creating" @click="tokenDialog = false"
          >取消</UiButton
        ><UiButton variant="primary" :loading="creating" @click="createToken"
          >创建 Token</UiButton
        ></template
      ></UiDrawer
    >
    <UiDialog
      :model-value="!!createdToken"
      title="保存你的 Token"
      width="620px"
      @update:model-value="setCreatedVisible"
      ><p class="token-warning">明文只显示这一次，关闭后无法再次查看。</p>
      <el-input
        :model-value="createdToken"
        readonly
        type="textarea"
        :rows="3"
        class="token-value"
        aria-label="新建 API Token，请立即保存"
      /><template #footer
        ><div class="dialog-actions">
          <UiButton @click="setCreatedVisible(false)">已保存，关闭</UiButton
          ><UiButton
            variant="primary"
            :loading="copying"
            @click="copyCreatedToken"
            >复制 Token</UiButton
          >
        </div></template
      ></UiDialog
    >
  </div>
</template>
<style scoped>
.ops-page,
.token-section {
  min-width: 0;
}
.ops-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(0, 1fr);
  gap: 40px;
  align-items: start;
}
.section-heading {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 24px;
  margin-bottom: 32px;
}
.section-heading h2 {
  margin: 0 0 8px;
  font-size: 22px;
}
.section-heading p {
  margin: 0;
  color: var(--c-muted);
  font-size: 14px;
}
.token-summary {
  display: flex;
  align-items: end;
  gap: 40px;
  margin-bottom: 24px;
}
.token-summary p {
  color: var(--c-muted);
  font-size: 14px;
  margin: 0 0 8px;
}
.audit-section {
  margin-top: 64px;
  padding-top: 48px;
  border-top: 1px solid var(--c-line);
}
.mono {
  font: 400 14px/1.6 var(--font-mono);
  overflow-wrap: anywhere;
}
.muted {
  color: var(--c-muted);
}
.table-name {
  font-weight: 600;
  overflow-wrap: anywhere;
}
.audit-path {
  overflow-wrap: anywhere;
}
.mobile-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}
.mobile-heading h3 {
  font-size: 16px;
  overflow-wrap: anywhere;
  margin: 0;
}
.mobile-fields {
  display: grid;
  grid-template-columns: 64px minmax(0, 1fr);
  gap: 12px;
  font-size: 14px;
  margin-top: 16px;
}
.mobile-fields dt {
  color: var(--c-muted);
}
.mobile-fields dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.form-note {
  font-size: 14px;
  margin: 0 0 24px;
}
.error-text {
  color: var(--c-bad);
  font-size: 14px;
}
.token-warning {
  color: var(--c-warn-ink);
}
.token-value {
  font-family: var(--font-mono);
  margin-top: 16px;
}
.dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}
@media (max-width: 1279px) {
  .ops-layout {
    grid-template-columns: minmax(0, 1fr);
    gap: 48px;
  }
}
@media (max-width: 767px) {
  .section-heading {
    flex-direction: column;
    gap: 16px;
  }
  .token-summary {
    align-items: start;
    flex-direction: column;
    gap: 16px;
  }
  .audit-section {
    margin-top: 48px;
    padding-top: 32px;
  }
}
</style>
