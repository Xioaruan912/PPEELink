<script setup lang="ts">
import { onMounted, reactive, ref, watch } from "vue";
import IconDownload from "~icons/ph/download-simple";
import IconUpload from "~icons/ph/upload-simple";
import IconDatabase from "~icons/ph/database";
import IconKey from "~icons/ph/key";
import IconTrash from "~icons/ph/trash";
import {
  createApiToken,
  deleteShortLink,
  downloadFullBackup,
  exportSafeBackup,
  getApiTokens,
  getAuditLogs,
  getShortLinks,
  importSafeBackup,
  inspectSafeBackup,
  revokeApiToken,
  type ApiToken,
  type AuditEntry,
  type ShortLinkItem,
} from "@/api/ops";
import {
  EmptyState,
  PageHeader,
  UiButton,
  UiDialog,
  UiPanel,
  UiSegmented,
  UiSkeleton,
  UiTag,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import SecretLink from "@/views/subcription/subs/SecretLink.vue";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "SystemSettings" });
const tabs = [
  { label: "备份恢复", value: "backup" },
  { label: "API 令牌", value: "tokens" },
  { label: "操作审计", value: "audit" },
  { label: "短链", value: "links" },
];
const tab = ref<string | number>("backup");

// ---- 备份 ---------------------------------------------------------------
const summary = ref<Record<string, number> | null>(null);
const busy = ref("");
async function loadSummary() {
  try {
    summary.value = (await inspectSafeBackup()).data;
  } catch {
    summary.value = null;
  }
}
async function fullBackup() {
  busy.value = "full";
  try {
    await downloadFullBackup();
    ElMessage.success("完整备份已开始下载");
  } catch (error: any) {
    ElMessage.error(error?.message || "备份失败");
  } finally {
    busy.value = "";
  }
}
async function exportConfig() {
  busy.value = "export";
  try {
    const { data } = await exportSafeBackup();
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json",
    });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = "ppeelink-config.json";
    a.click();
    URL.revokeObjectURL(a.href);
  } finally {
    busy.value = "";
  }
}
const fileInput = ref<HTMLInputElement>();
async function importConfig(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  (event.target as HTMLInputElement).value = "";
  if (!file) return;
  let payload: unknown;
  try {
    payload = JSON.parse(await file.text());
  } catch {
    ElMessage.error("不是有效的配置备份文件");
    return;
  }
  try {
    await ElMessageBox.confirm(
      "将合并恢复模板、订阅配置与机场设置；节点、账户和令牌不会被修改。",
      "导入配置备份",
      { type: "warning", confirmButtonText: "导入", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  busy.value = "import";
  try {
    await importSafeBackup(payload);
    ElMessage.success("配置已合并恢复");
    await loadSummary();
  } finally {
    busy.value = "";
  }
}

// ---- API 令牌 ------------------------------------------------------------
const tokens = ref<ApiToken[]>([]);
const tokensLoading = ref(false);
const tokenDialog = ref(false);
const tokenForm = reactive({ name: "", scopes: ["read"], expiresInDays: 90 });
const newToken = ref("");
async function loadTokens() {
  tokensLoading.value = true;
  try {
    tokens.value = (await getApiTokens()).data || [];
  } finally {
    tokensLoading.value = false;
  }
}
async function submitToken() {
  if (!tokenForm.name.trim()) {
    ElMessage.warning("请填写名称");
    return;
  }
  const { data } = await createApiToken({
    ...tokenForm,
    name: tokenForm.name.trim(),
  });
  newToken.value = data?.token || "";
  await loadTokens();
}
async function revoke(item: ApiToken) {
  try {
    await ElMessageBox.confirm(
      `吊销令牌 ${item.name}？使用它的脚本将立即失效。`,
      "吊销令牌",
      {
        type: "warning",
        confirmButtonText: "吊销",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  await revokeApiToken(item.id);
  ElMessage.success("已吊销");
  await loadTokens();
}

// ---- 审计 ----------------------------------------------------------------
const audit = ref<AuditEntry[]>([]);
const auditLoading = ref(false);
async function loadAudit() {
  auditLoading.value = true;
  try {
    audit.value = (await getAuditLogs()).data || [];
  } finally {
    auditLoading.value = false;
  }
}

// ---- 短链 ----------------------------------------------------------------
const links = ref<ShortLinkItem[]>([]);
const linksLoading = ref(false);
async function loadLinks() {
  linksLoading.value = true;
  try {
    links.value = (await getShortLinks()).data || [];
  } finally {
    linksLoading.value = false;
  }
}
async function removeLink(item: ShortLinkItem) {
  await deleteShortLink(item.id);
  ElMessage.success("已删除");
  await loadLinks();
}

const loaders: Record<string, () => Promise<void>> = {
  backup: loadSummary,
  tokens: loadTokens,
  audit: loadAudit,
  links: loadLinks,
};
watch(tab, (value) => void loaders[String(value)]?.());
onMounted(loadSummary);
</script>
<template>
  <div class="settings-page">
    <PageHeader
      title="系统设置"
      description="备份数据、管理 API 令牌，查看谁改过什么。"
    />
    <UiSegmented
      v-model="tab"
      class="settings-tabs"
      :options="tabs"
      label="设置分区"
    />
    <Transition name="pane" mode="out-in">
      <div v-if="tab === 'backup'" key="backup" class="backup-grid">
        <section class="backup-hero" data-enter>
          <span class="hero-icon"><IconDatabase /></span>
          <h2>完整备份</h2>
          <p>
            数据库快照加全部模板，打包为
            tar.gz。包含节点、订阅、账户，迁移或重装时停服解压覆盖即可。
          </p>
          <UiButton
            variant="primary"
            :icon="IconDownload"
            :loading="busy === 'full'"
            @click="fullBackup"
            >下载完整备份</UiButton
          >
        </section>
        <UiPanel
          title="配置导出与导入"
          description="只含模板、订阅配置与机场设置，不含节点与凭据，适合在两台服务之间同步配置。"
          data-enter
        >
          <dl v-if="summary" class="summary">
            <div>
              <dt>模板</dt>
              <dd class="mono">{{ summary.templates }}</dd>
            </div>
            <div>
              <dt>订阅</dt>
              <dd class="mono">{{ summary.subscriptions }}</dd>
            </div>
            <div>
              <dt>机场</dt>
              <dd class="mono">{{ summary.airports }}</dd>
            </div>
          </dl>
          <div class="row-actions">
            <UiButton
              :icon="IconDownload"
              :loading="busy === 'export'"
              @click="exportConfig"
              >导出配置</UiButton
            >
            <UiButton
              :icon="IconUpload"
              :loading="busy === 'import'"
              @click="fileInput?.click()"
              >导入配置</UiButton
            >
            <input
              ref="fileInput"
              type="file"
              accept="application/json"
              hidden
              @change="importConfig"
            />
          </div>
        </UiPanel>
      </div>

      <UiPanel
        v-else-if="tab === 'tokens'"
        key="tokens"
        title="API 令牌"
        :count="tokens.length"
        description="给脚本或自动化用的访问凭据，可限定只读或读写。"
        flush
      >
        <template #actions>
          <UiButton
            :icon="IconKey"
            @click="
              newToken = '';
              tokenDialog = true;
            "
            >新建令牌</UiButton
          >
        </template>
        <UiSkeleton v-if="tokensLoading" :lines="3" height="52px" />
        <EmptyState
          v-else-if="!tokens.length"
          size="section"
          title="还没有令牌"
          description="新建令牌后即可在脚本里调用接口。"
        />
        <ul v-else class="plain-list">
          <li
            v-for="item in tokens"
            :key="item.id"
            :class="{ 'is-off': !item.enabled }"
          >
            <div class="item-main">
              <strong>{{ item.name }}</strong>
              <span class="mono muted">{{ item.tokenPrefix }}…</span>
            </div>
            <div class="item-meta">
              <UiTag
                v-for="s in item.scopes.split(',')"
                :key="s"
                :tone="s === 'admin' ? 'warn' : 'neutral'"
                >{{ s }}</UiTag
              >
              <UiTag v-if="!item.enabled" tone="bad">已吊销</UiTag>
              <span class="muted">{{
                item.lastUsedAt
                  ? `最近使用 ${formatDateTime(item.lastUsedAt)}`
                  : "从未使用"
              }}</span>
            </div>
            <RowIconButton
              v-if="item.enabled"
              :icon="IconTrash"
              :label="`吊销 ${item.name}`"
              @click="revoke(item)"
            />
          </li>
        </ul>
      </UiPanel>

      <UiPanel
        v-else-if="tab === 'audit'"
        key="audit"
        title="操作审计"
        :count="audit.length"
        description="最近的管理写操作（不记录请求内容）。"
        flush
      >
        <UiSkeleton v-if="auditLoading" :lines="5" height="40px" />
        <EmptyState v-else-if="!audit.length" size="section" title="暂无记录" />
        <ol v-else class="audit-list">
          <li v-for="item in audit" :key="item.id">
            <time class="mono muted">{{ formatDateTime(item.createdAt) }}</time>
            <span class="audit-action">{{
              item.action || `${item.method} ${item.path}`
            }}</span>
            <span class="muted">{{ item.actor }}</span>
            <UiTag :tone="item.status >= 400 ? 'bad' : 'ok'">{{
              item.status
            }}</UiTag>
          </li>
        </ol>
      </UiPanel>

      <UiPanel
        v-else
        key="links"
        title="短链"
        :count="links.length"
        description="分享订阅时生成的短链接，删除后立即失效。"
        flush
      >
        <UiSkeleton v-if="linksLoading" :lines="3" height="52px" />
        <EmptyState
          v-else-if="!links.length"
          size="section"
          title="还没有短链"
          description="在订阅的分享面板里可以生成。"
        />
        <ul v-else class="plain-list">
          <li v-for="item in links" :key="item.id">
            <div class="item-main">
              <strong>{{ item.remark || "未命名" }}</strong>
              <span class="mono muted">/s/{{ item.code }}</span>
            </div>
            <span class="muted">{{ formatDateTime(item.createdAt) }}</span>
            <RowIconButton
              :icon="IconTrash"
              :label="`删除短链 ${item.code}`"
              @click="removeLink(item)"
            />
          </li>
        </ul>
      </UiPanel>
    </Transition>

    <UiDialog v-model="tokenDialog" title="新建 API 令牌" width="460px">
      <template v-if="newToken">
        <p class="token-note">令牌只显示这一次，请立即复制保存。</p>
        <SecretLink :value="newToken" label="API 令牌" />
      </template>
      <el-form v-else label-position="top" @submit.prevent="submitToken">
        <el-form-item label="名称" required
          ><el-input v-model="tokenForm.name" placeholder="例如：备份脚本"
        /></el-form-item>
        <el-form-item label="权限">
          <el-checkbox-group v-model="tokenForm.scopes">
            <el-checkbox value="read">只读</el-checkbox>
            <el-checkbox value="write">读写</el-checkbox>
            <el-checkbox value="admin">管理</el-checkbox>
          </el-checkbox-group>
        </el-form-item>
        <el-form-item label="有效期（天，0 为永久）">
          <el-input-number
            v-model="tokenForm.expiresInDays"
            :min="0"
            :max="3650"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <UiButton @click="tokenDialog = false">{{
          newToken ? "完成" : "取消"
        }}</UiButton>
        <UiButton v-if="!newToken" variant="primary" @click="submitToken"
          >创建</UiButton
        >
      </template>
    </UiDialog>
  </div>
</template>
<style scoped>
.settings-page {
  min-width: 0;
}
.settings-tabs {
  margin-bottom: 16px;
}
.backup-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.2fr);
  gap: 16px;
  align-items: stretch;
}
.backup-hero {
  position: relative;
  display: grid;
  align-content: start;
  justify-items: start;
  gap: 10px;
  padding: 24px;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background:
    radial-gradient(
      80% 90% at 100% 0%,
      color-mix(in srgb, var(--c-signal) 22%, transparent),
      transparent 65%
    ),
    var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.hero-icon {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  border-radius: 14px;
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.hero-icon svg {
  width: 22px;
  height: 22px;
}
.backup-hero h2 {
  font-size: 20px;
}
.backup-hero p {
  max-width: 44ch;
  color: var(--c-muted);
  font-size: 14px;
}
.summary {
  display: flex;
  gap: 28px;
  margin-bottom: 16px;
}
.summary dt {
  color: var(--c-muted);
  font-size: 12px;
}
.summary dd {
  margin: 0;
  font-size: 22px;
  font-weight: 600;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.plain-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--c-line);
}
.plain-list li:last-child,
.audit-list li:last-child {
  border-bottom: 0;
}
.plain-list li.is-off {
  opacity: 0.6;
}
.item-main {
  display: grid;
  min-width: 0;
}
.item-main strong {
  font-size: 14px;
  font-weight: 600;
}
.item-meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
}
.muted {
  color: var(--c-muted);
  font-size: 12px;
}
.mono {
  font-family: var(--font-mono);
}
.audit-list li {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr) 120px auto;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  border-bottom: 1px solid var(--c-line);
  font-size: 13px;
}
.audit-action {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.token-note {
  margin-bottom: 12px;
  color: var(--c-warn-ink);
  font-size: 13px;
}
.pane-enter-active,
.pane-leave-active {
  transition:
    opacity var(--duration-medium) var(--ease-out),
    transform var(--duration-medium) var(--ease-out),
    filter var(--duration-medium) var(--ease-out);
}
.pane-enter-from {
  opacity: 0;
  transform: translateY(14px);
  filter: blur(6px);
}
.pane-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
@media (max-width: 899px) {
  .backup-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .audit-list li {
    grid-template-columns: minmax(0, 1fr) auto;
  }
  .audit-list time,
  .audit-list li > span.muted {
    grid-column: 1 / -1;
  }
}
</style>
