<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import gsap from "gsap";
import { Search } from "@element-plus/icons-vue";
import IconRefresh from "~icons/ph/arrow-clockwise";
import IconCopy from "~icons/ph/copy";
import IconTrash from "~icons/ph/trash";
import IconPlus from "~icons/ph/plus";
import IconDownloadSimple from "~icons/ph/download-simple";
import IconUploadSimple from "~icons/ph/upload-simple";
import IconArrowSquareOut from "~icons/ph/arrow-square-out";
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
import { EmptyState, UiButton, UiDialog, UiSkeleton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import SecretLink from "@/views/subcription/subs/SecretLink.vue";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "SystemSettings" });

type Phase = "idle" | "loading" | "ready" | "error";

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const SECTION_KEY = "ppeelink.settings.section.v2";
const sections = [
  { value: "backup", label: "备份与恢复" },
  { value: "tokens", label: "API 令牌" },
  { value: "audit", label: "操作审计" },
  { value: "links", label: "短链" },
] as const;
type SectionValue = (typeof sections)[number]["value"];

const storedSection = localStorage.getItem(SECTION_KEY);
const tab = ref<SectionValue>(
  sections.some((s) => s.value === storedSection)
    ? (storedSection as SectionValue)
    : "backup"
);

const online = ref(navigator.onLine);
const refreshing = ref(false);

// ---- 1. 备份与恢复 ----------------------------------------------------------
const summaryState = reactive({
  phase: "idle" as Phase,
  error: "",
  summary: null as Record<string, number> | null,
});
const summary = computed(() => summaryState.summary);
const backupBusy = ref<"" | "full" | "export" | "import">("");
const lastBackupAt = ref(localStorage.getItem("ppeelink.backup.lastFull") || "");
const importInput = ref<HTMLInputElement>();
const importFile = ref<File | null>(null);
const importError = ref("");
const dragging = ref(false);

const fileSizeLabel = computed(() => {
  const size = importFile.value?.size ?? 0;
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
});

async function loadBackup(force = false) {
  if (!force && summaryState.phase === "ready") return;
  summaryState.phase = "loading";
  summaryState.error = "";
  try {
    const { data } = await inspectSafeBackup();
    summaryState.summary = data || null;
    summaryState.phase = "ready";
  } catch (error: any) {
    summaryState.phase = "error";
    summaryState.error = error?.message || "读取备份概览失败";
  }
}

async function fullBackup() {
  if (!online.value) return;
  backupBusy.value = "full";
  try {
    await downloadFullBackup();
    lastBackupAt.value = new Date().toISOString();
    localStorage.setItem("ppeelink.backup.lastFull", lastBackupAt.value);
    ElMessage.success("完整备份已开始下载");
  } catch (error: any) {
    ElMessage.error(error?.message || "备份失败");
  } finally {
    backupBusy.value = "";
  }
}

async function exportConfig() {
  if (!online.value) return;
  backupBusy.value = "export";
  try {
    const { data } = await exportSafeBackup();
    const blob = new Blob([JSON.stringify(data, null, 2)], {
      type: "application/json",
    });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `ppeelink-config-${new Date().toISOString().slice(0, 10)}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    ElMessage.success("配置文件已导出");
  } catch {
    ElMessage.error("导出失败");
  } finally {
    backupBusy.value = "";
  }
}

function chooseImport() {
  importInput.value?.click();
}

function acceptImportFile(file: File) {
  importFile.value = file;
  importError.value = "";
  file
    .text()
    .then((text) => {
      JSON.parse(text);
    })
    .catch(() => {
      importFile.value = null;
      importError.value = "无效的 JSON 配置文件";
    });
}

function onImportFile(event: Event) {
  const input = event.target as HTMLInputElement;
  const file = input.files?.[0];
  input.value = "";
  if (file) acceptImportFile(file);
}

function onDrop(event: DragEvent) {
  dragging.value = false;
  const file = event.dataTransfer?.files?.[0];
  if (file) acceptImportFile(file);
}

async function confirmImport() {
  if (!importFile.value || !online.value) return;
  const file = importFile.value;
  try {
    await ElMessageBox.confirm(
      `合并导入「${file.name}」？节点、成员与令牌不会被覆盖。`,
      "导入配置",
      { type: "warning", confirmButtonText: "导入", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  backupBusy.value = "import";
  try {
    const payload = JSON.parse(await file.text());
    await importSafeBackup(payload);
    ElMessage.success("配置已合并恢复");
    importFile.value = null;
    await loadBackup(true);
  } catch (error: any) {
    importError.value = error?.message || "导入失败";
    ElMessage.error(importError.value);
  } finally {
    backupBusy.value = "";
  }
}

// ---- 2. API 令牌 -----------------------------------------------------------
const tokensState = reactive({
  phase: "idle" as Phase,
  error: "",
  items: [] as ApiToken[],
});
const tokenDialog = ref(false);
const tokenForm = reactive({
  name: "",
  scopes: ["read"] as string[],
  expiresInDays: 90,
});
const tokenError = ref("");
const creatingToken = ref(false);
const newToken = ref("");
const revokingId = ref<number | null>(null);

const scopeOptions = [
  { value: "read", label: "只读" },
  { value: "write", label: "读写" },
  { value: "admin", label: "管理" },
];
const expiryPresets = [
  { label: "30 天", value: 30 },
  { label: "90 天", value: 90 },
  { label: "1 年", value: 365 },
  { label: "永久", value: 0 },
];

const scopeLabel = (scope: string) =>
  scopeOptions.find((o) => o.value === scope)?.label || scope;

function toggleScope(val: string) {
  const idx = tokenForm.scopes.indexOf(val);
  if (idx >= 0) tokenForm.scopes.splice(idx, 1);
  else tokenForm.scopes.push(val);
}

async function loadTokens(force = false) {
  if (!force && tokensState.phase === "ready") return;
  tokensState.phase = "loading";
  tokensState.error = "";
  try {
    const { data } = await getApiTokens();
    tokensState.items = data || [];
    tokensState.phase = "ready";
  } catch (error: any) {
    tokensState.phase = "error";
    tokensState.error = error?.message || "读取令牌失败";
  }
}

function openTokenDialog() {
  tokenForm.name = "";
  tokenForm.scopes = ["read"];
  tokenForm.expiresInDays = 90;
  tokenError.value = "";
  newToken.value = "";
  tokenDialog.value = true;
}

async function submitToken() {
  if (!online.value) return;
  const name = tokenForm.name.trim();
  if (!name) {
    tokenError.value = "请填写令牌名称";
    return;
  }
  if (!tokenForm.scopes.length) {
    tokenError.value = "至少选择一项权限";
    return;
  }
  creatingToken.value = true;
  tokenError.value = "";
  try {
    const { data } = await createApiToken({
      name,
      scopes: tokenForm.scopes,
      expiresInDays: tokenForm.expiresInDays,
    });
    newToken.value = data?.token || "";
    await loadTokens(true);
    ElMessage.success("令牌已创建");
  } catch (error: any) {
    tokenError.value = error?.message || "创建失败";
  } finally {
    creatingToken.value = false;
  }
}

async function revoke(item: ApiToken) {
  if (!online.value || revokingId.value) return;
  try {
    await ElMessageBox.confirm(`吊销令牌「${item.name}」？`, "吊销令牌", {
      type: "warning",
      confirmButtonText: "吊销",
      cancelButtonText: "取消",
    });
  } catch {
    return;
  }
  revokingId.value = item.id;
  try {
    await revokeApiToken(item.id);
    ElMessage.success("令牌已吊销");
    await loadTokens(true);
  } catch {
    ElMessage.error("吊销失败");
  } finally {
    revokingId.value = null;
  }
}

function expiryLabel(item: ApiToken) {
  if (!item.expiresAt) return "永久";
  return new Date(item.expiresAt).getTime() < Date.now()
    ? "已过期"
    : formatDateTime(item.expiresAt);
}

// ---- 3. 操作审计（去噪精简 + 点击展开详情） ----------------------------------
const auditState = reactive({
  phase: "idle" as Phase,
  error: "",
  items: [] as AuditEntry[],
});
const auditQuery = ref("");
const auditStatus = ref<"all" | "ok" | "fail">("all");
const auditPage = ref(1);
const auditPageSize = 15;
const expandedAuditId = ref<number | null>(null);

const auditFiltered = computed(() => {
  const q = auditQuery.value.trim().toLowerCase();
  return auditState.items.filter((entry) => {
    if (auditStatus.value === "ok" && entry.status >= 400) return false;
    if (auditStatus.value === "fail" && entry.status < 400) return false;
    if (!q) return true;
    return `${entry.action} ${entry.path} ${entry.actor} ${entry.method}`
      .toLowerCase()
      .includes(q);
  });
});

const auditPages = computed(() =>
  Math.max(1, Math.ceil(auditFiltered.value.length / auditPageSize))
);

const auditPageItems = computed(() =>
  auditFiltered.value.slice(
    (auditPage.value - 1) * auditPageSize,
    auditPage.value * auditPageSize
  )
);

watch([auditQuery, auditStatus], () => {
  auditPage.value = 1;
});
watch(auditPages, (pages) => {
  if (auditPage.value > pages) auditPage.value = pages;
});

async function loadAudit(force = false) {
  if (!force && auditState.phase === "ready") return;
  auditState.phase = "loading";
  auditState.error = "";
  try {
    const { data } = await getAuditLogs();
    auditState.items = data || [];
    auditState.phase = "ready";
  } catch (error: any) {
    auditState.phase = "error";
    auditState.error = error?.message || "读取审计日志失败";
  }
}

function toggleAuditRow(id: number) {
  expandedAuditId.value = expandedAuditId.value === id ? null : id;
}

// ---- 4. 短链 ---------------------------------------------------------------
const linksState = reactive({
  phase: "idle" as Phase,
  error: "",
  items: [] as ShortLinkItem[],
});
const deletingLinkId = ref<number | null>(null);

async function loadLinks(force = false) {
  if (!force && linksState.phase === "ready") return;
  linksState.phase = "loading";
  linksState.error = "";
  try {
    const { data } = await getShortLinks();
    linksState.items = data || [];
    linksState.phase = "ready";
  } catch (error: any) {
    linksState.phase = "error";
    linksState.error = error?.message || "读取短链失败";
  }
}

function shortLinkURL(code: string) {
  return `${window.location.origin}/s/${code}`;
}

async function copyText(value: string, message = "已复制") {
  try {
    await navigator.clipboard.writeText(value);
    ElMessage.success(message);
  } catch {
    ElMessage.warning("复制失败");
  }
}

function openLink(item: ShortLinkItem) {
  if (!item.target) return;
  window.open(item.target, "_blank", "noopener,noreferrer");
}

async function removeLink(item: ShortLinkItem) {
  if (!online.value || deletingLinkId.value) return;
  try {
    await ElMessageBox.confirm(`删除短链 /s/${item.code}？`, "删除短链", {
      type: "warning",
      confirmButtonText: "删除",
      cancelButtonText: "取消",
    });
  } catch {
    return;
  }
  deletingLinkId.value = item.id;
  try {
    await deleteShortLink(item.id);
    ElMessage.success("短链已删除");
    await loadLinks(true);
  } catch {
    ElMessage.error("删除失败");
  } finally {
    deletingLinkId.value = null;
  }
}

// ---- Shell -----------------------------------------------------------------
function badgeCount(value: SectionValue): string {
  if (value === "tokens" && tokensState.phase === "ready")
    return String(tokensState.items.length);
  if (value === "audit" && auditState.phase === "ready")
    return String(auditState.items.length);
  if (value === "links" && linksState.phase === "ready")
    return String(linksState.items.length);
  return "";
}

function loadSection(value: SectionValue, force = false): Promise<void> {
  if (value === "backup") return loadBackup(force);
  if (value === "tokens") return loadTokens(force);
  if (value === "audit") return loadAudit(force);
  return loadLinks(force);
}

async function refreshActive() {
  refreshing.value = true;
  try {
    await loadSection(tab.value, true);
  } finally {
    refreshing.value = false;
  }
}

function syncOnline() {
  online.value = navigator.onLine;
}

watch(tab, (val) => {
  localStorage.setItem(SECTION_KEY, val);
  void loadSection(val);
});

onMounted(() => {
  window.addEventListener("online", syncOnline);
  window.addEventListener("offline", syncOnline);
  void loadSection(tab.value);
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("online", syncOnline);
  window.removeEventListener("offline", syncOnline);
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="settings-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="settings-masthead">
      <div class="title-mask">
        <h1 class="gallery-title" data-title-reveal>系统设置</h1>
      </div>

      <div class="masthead-actions">
        <UiButton
          v-if="tab === 'tokens'"
          variant="primary"
          :icon="IconPlus"
          @click="openTokenDialog"
        >
          新建令牌
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="IconRefresh"
          :loading="refreshing"
          @click="refreshActive"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Clean 4-Tab Horizontal Pill Bar -->
    <nav class="tab-bar" aria-label="系统设置分区">
      <button
        v-for="s in sections"
        :key="s.value"
        type="button"
        class="tab-pill"
        :class="{ 'is-active': tab === s.value }"
        :aria-current="tab === s.value ? 'page' : undefined"
        @click="tab = s.value"
      >
        <span>{{ s.label }}</span>
        <small v-if="badgeCount(s.value)" class="mono">
          {{ badgeCount(s.value) }}
        </small>
      </button>
    </nav>

    <!-- Tab Panels -->
    <Transition name="pane" mode="out-in">
      <!-- 1. 备份与恢复 -->
      <section v-if="tab === 'backup'" key="backup" class="pane-stack">
        <UiSkeleton v-if="summaryState.phase === 'loading'" :lines="3" />
        <dl v-else-if="summary" class="kpi-strip">
          <div class="kpi-cell">
            <dt>配置模板</dt>
            <dd class="mono">{{ summary.templates ?? 0 }}</dd>
          </div>
          <div class="kpi-cell">
            <dt>订阅链接</dt>
            <dd class="mono">{{ summary.subscriptions ?? 0 }}</dd>
          </div>
          <div class="kpi-cell">
            <dt>节点资产</dt>
            <dd class="mono">{{ summary.nodes ?? 0 }}</dd>
          </div>
        </dl>

        <div class="action-list">
          <div class="action-row">
            <div class="action-copy">
              <strong>完整数据库备份</strong>
              <span v-if="lastBackupAt" class="mono">
                上次备份 {{ formatDateTime(lastBackupAt) }}
              </span>
              <span v-else>下载完整 SQLite 快照文件</span>
            </div>
            <UiButton
              variant="primary"
              :icon="IconDownloadSimple"
              :loading="backupBusy === 'full'"
              :disabled="!online"
              @click="fullBackup"
            >
              下载快照
            </UiButton>
          </div>

          <div class="action-row">
            <div class="action-copy">
              <strong>导出 JSON 配置</strong>
              <span>仅导出模板与订阅结构，不含密钥</span>
            </div>
            <UiButton
              variant="ghost"
              :icon="IconDownloadSimple"
              :loading="backupBusy === 'export'"
              :disabled="!online"
              @click="exportConfig"
            >
              导出 JSON
            </UiButton>
          </div>
        </div>

        <!-- Clean Dropzone for Config Import -->
        <div
          class="import-drop"
          :class="{ 'is-dragging': dragging }"
          @dragover.prevent="dragging = true"
          @dragleave="dragging = false"
          @drop.prevent="onDrop"
        >
          <input
            ref="importInput"
            type="file"
            accept=".json,application/json"
            class="sr-only"
            @change="onImportFile"
          />
          <div class="drop-copy">
            <strong>{{ importFile ? importFile.name : "拖入 JSON 配置文件恢复" }}</strong>
            <span v-if="importFile" class="mono">{{ fileSizeLabel }}</span>
            <span v-else>合并导入模板与订阅，保留现有节点与成员</span>
          </div>
          <div class="drop-actions">
            <UiButton variant="ghost" :icon="IconUploadSimple" @click="chooseImport">
              选择文件
            </UiButton>
            <UiButton
              v-if="importFile"
              variant="primary"
              :loading="backupBusy === 'import'"
              @click="confirmImport"
            >
              确认导入
            </UiButton>
          </div>
        </div>
        <p v-if="importError" class="inline-error" role="alert">{{ importError }}</p>
      </section>

      <!-- 2. API 令牌 -->
      <section v-else-if="tab === 'tokens'" key="tokens" class="pane-stack">
        <UiSkeleton v-if="tokensState.phase === 'loading'" :lines="4" />
        <EmptyState
          v-else-if="tokensState.phase === 'error'"
          size="section"
          title="读取令牌失败"
          :description="tokensState.error"
        >
          <template #action>
            <UiButton @click="loadTokens(true)">重试</UiButton>
          </template>
        </EmptyState>
        <EmptyState
          v-else-if="!tokensState.items.length"
          size="section"
          title="暂无 API 令牌"
        >
          <template #action>
            <UiButton variant="primary" :icon="IconPlus" @click="openTokenDialog">
              新建令牌
            </UiButton>
          </template>
        </EmptyState>
        <div v-else class="gallery-list">
          <div v-for="item in tokensState.items" :key="item.id" class="gallery-row">
            <div class="row-primary">
              <strong>{{ item.name }}</strong>
              <span v-if="item.tokenPrefix" class="mono muted">{{ item.tokenPrefix }}…</span>
            </div>
            <div class="row-chips">
              <span
                v-for="sc in String(item.scopes || '').split(',').map((s) => s.trim()).filter(Boolean)"
                :key="sc"
                class="chip"
              >
                {{ scopeLabel(sc) }}
              </span>
            </div>
            <span class="row-meta mono">{{ expiryLabel(item) }}</span>
            <RowIconButton
              :icon="IconTrash"
              :label="`吊销 ${item.name}`"
              :disabled="revokingId === item.id"
              @click="revoke(item)"
            />
          </div>
        </div>
      </section>

      <!-- 3. 操作审计（低密度、单行精简、点击展开） -->
      <section v-else-if="tab === 'audit'" key="audit" class="pane-stack">
        <div class="audit-bar">
          <div class="pill-group" role="group" aria-label="状态筛选">
            <button
              type="button"
              class="seg-pill"
              :class="{ 'is-active': auditStatus === 'all' }"
              @click="auditStatus = 'all'"
            >
              全部
            </button>
            <button
              type="button"
              class="seg-pill"
              :class="{ 'is-active': auditStatus === 'ok' }"
              @click="auditStatus = 'ok'"
            >
              成功
            </button>
            <button
              type="button"
              class="seg-pill"
              :class="{ 'is-active': auditStatus === 'fail' }"
              @click="auditStatus = 'fail'"
            >
              异常
            </button>
          </div>

          <label class="search-wrap">
            <el-icon class="search-icon"><Search /></el-icon>
            <input
              v-model="auditQuery"
              type="search"
              class="search-input"
              placeholder="搜索操作或成员..."
              aria-label="搜索审计日志"
            />
          </label>
        </div>

        <UiSkeleton v-if="auditState.phase === 'loading'" :lines="5" />
        <EmptyState
          v-else-if="!auditPageItems.length"
          size="section"
          title="暂无匹配的操作记录"
        />
        <div v-else class="audit-list">
          <div
            v-for="entry in auditPageItems"
            :key="entry.id"
            class="audit-item"
          >
            <button
              type="button"
              class="audit-row"
              @click="toggleAuditRow(entry.id)"
            >
              <span
                class="audit-dot"
                :class="entry.status >= 400 ? 'is-bad' : 'is-ok'"
                aria-hidden="true"
              ></span>
              <span class="audit-action">{{ entry.action || entry.path }}</span>
              <span class="audit-actor">{{ entry.actor || "system" }}</span>
              <span class="audit-time mono">{{ formatDateTime(entry.createdAt) }}</span>
            </button>
            <div v-if="expandedAuditId === entry.id" class="audit-detail mono">
              <span>{{ entry.method }} {{ entry.path }}</span>
              <span>HTTP {{ entry.status }}</span>
              <span v-if="entry.ip">IP {{ entry.ip }}</span>
            </div>
          </div>
        </div>

        <div v-if="auditPages > 1" class="pager">
          <UiButton
            size="sm"
            variant="ghost"
            :disabled="auditPage <= 1"
            @click="auditPage--"
          >
            上一页
          </UiButton>
          <span class="mono muted">{{ auditPage }} / {{ auditPages }}</span>
          <UiButton
            size="sm"
            variant="ghost"
            :disabled="auditPage >= auditPages"
            @click="auditPage++"
          >
            下一页
          </UiButton>
        </div>
      </section>

      <!-- 4. 短链 -->
      <section v-else key="links" class="pane-stack">
        <UiSkeleton v-if="linksState.phase === 'loading'" :lines="4" />
        <EmptyState
          v-else-if="!linksState.items.length"
          size="section"
          title="暂无短链记录"
        />
        <div v-else class="gallery-list">
          <div v-for="item in linksState.items" :key="item.id" class="gallery-row is-link">
            <div class="row-primary">
              <strong class="mono">/s/{{ item.code }}</strong>
              <span class="muted target-url">{{ item.target }}</span>
            </div>
            <span class="row-meta mono">{{ formatDateTime(item.createdAt) }}</span>
            <div class="row-actions">
              <RowIconButton
                :icon="IconCopy"
                label="复制短链"
                @click="copyText(shortLinkURL(item.code), '短链已复制')"
              />
              <RowIconButton
                :icon="IconArrowSquareOut"
                label="打开目标"
                @click="openLink(item)"
              />
              <RowIconButton
                :icon="IconTrash"
                label="删除短链"
                :disabled="deletingLinkId === item.id"
                @click="removeLink(item)"
              />
            </div>
          </div>
        </div>
      </section>
    </Transition>

    <!-- Create API Token Dialog -->
    <UiDialog
      v-model="tokenDialog"
      :title="newToken ? '令牌已生成' : '新建 API 令牌'"
      width="480px"
    >
      <div v-if="newToken" class="token-created">
        <p class="muted">此令牌仅展示一次，请立即复制保存。</p>
        <SecretLink :value="newToken" label="API 令牌" />
      </div>
      <el-form v-else label-position="top" @submit.prevent="submitToken">
        <el-form-item label="名称" required>
          <el-input v-model="tokenForm.name" placeholder="例如：自动备份脚本" />
        </el-form-item>
        <el-form-item label="权限范围">
          <div class="pill-group">
            <button
              v-for="opt in scopeOptions"
              :key="opt.value"
              type="button"
              class="seg-pill"
              :class="{ 'is-active': tokenForm.scopes.includes(opt.value) }"
              @click="toggleScope(opt.value)"
            >
              {{ opt.label }}
            </button>
          </div>
        </el-form-item>
        <el-form-item label="有效期">
          <div class="pill-group">
            <button
              v-for="p in expiryPresets"
              :key="p.value"
              type="button"
              class="seg-pill"
              :class="{ 'is-active': tokenForm.expiresInDays === p.value }"
              @click="tokenForm.expiresInDays = p.value"
            >
              {{ p.label }}
            </button>
          </div>
        </el-form-item>
        <p v-if="tokenError" class="inline-error" role="alert">{{ tokenError }}</p>
      </el-form>
      <template #footer>
        <UiButton variant="ghost" @click="tokenDialog = false">
          {{ newToken ? "完成" : "取消" }}
        </UiButton>
        <UiButton
          v-if="!newToken"
          variant="primary"
          :loading="creatingToken"
          @click="submitToken"
        >
          生成令牌
        </UiButton>
      </template>
    </UiDialog>
  </div>
</template>

<style scoped>
.settings-gallery {
  display: grid;
  gap: 20px;
  max-width: 1120px;
  min-width: 0;
}

.settings-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

/* Horizontal 4-Tab Pill Bar */
.tab-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.tab-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 8px 16px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.tab-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.tab-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.tab-pill small {
  font-size: 11px;
  opacity: 0.7;
}

.pane-stack {
  display: grid;
  gap: 20px;
}

/* Hairline KPI Strip */
.kpi-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--c-line);
}

.kpi-cell {
  display: grid;
  gap: 6px;
  padding: 0 24px;
  border-left: 1px solid var(--c-line);
}

.kpi-cell:first-child {
  padding-left: 0;
  border-left: 0;
}

.kpi-cell dt {
  font-size: 12px;
  color: var(--c-muted);
}

.kpi-cell dd {
  margin: 0;
  font-size: 26px;
  font-weight: 500;
  color: var(--c-ink);
}

/* Backup Action Rows */
.action-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.action-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 4px;
  border-bottom: 1px solid var(--c-line);
  flex-wrap: wrap;
}

.action-copy {
  display: grid;
  gap: 4px;
}

.action-copy strong {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.action-copy span {
  font-size: 12.5px;
  color: var(--c-muted);
}

.import-drop {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 22px;
  border: 1px dashed var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  flex-wrap: wrap;
  transition: border-color var(--duration-fast) var(--ease-out);
}

.import-drop.is-dragging {
  border-color: var(--c-ok);
}

.drop-copy {
  display: grid;
  gap: 4px;
}

.drop-copy strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.drop-copy span {
  font-size: 12px;
  color: var(--c-muted);
}

.drop-actions {
  display: flex;
  gap: 8px;
}

/* Generic Borderless Gallery List (Tokens & Links) */
.gallery-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.gallery-row {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) auto auto auto;
  align-items: center;
  gap: 16px;
  padding: 16px 8px;
  border-bottom: 1px solid var(--c-line);
}

.gallery-row.is-link {
  grid-template-columns: minmax(0, 1.6fr) auto auto;
}

.row-primary {
  display: grid;
  gap: 3px;
  min-width: 0;
}

.row-primary strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.target-url {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-chips {
  display: flex;
  gap: 6px;
}

.chip {
  padding: 3px 9px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--c-ink);
  background: var(--c-surface);
}

.row-meta {
  font-size: 12px;
  color: var(--c-muted);
}

.row-actions {
  display: flex;
  gap: 4px;
}

/* Audit (Low Density) */
.audit-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.seg-pill {
  padding: 6px 12px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
}

.seg-pill:hover {
  border-color: var(--c-line);
  color: var(--c-ink);
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.search-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: min(260px, 100%);
  padding: 6px 0;
  border-bottom: 1px solid var(--c-line);
}

.search-wrap:focus-within {
  border-bottom-color: var(--c-ink);
}

.search-icon {
  color: var(--c-muted);
}

.search-input {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 13px;
  outline: none !important;
}

.audit-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.audit-item {
  border-bottom: 1px solid var(--c-line);
}

.audit-row {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 13px 6px;
  border: 0;
  background: transparent;
  text-align: left;
  cursor: pointer;
}

.audit-row:hover {
  background: var(--c-surface);
}

.audit-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.audit-dot.is-ok {
  background: var(--c-ok);
}

.audit-dot.is-bad {
  background: var(--c-bad);
}

.audit-action {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.audit-actor {
  font-size: 12px;
  color: var(--c-muted);
}

.audit-time {
  font-size: 12px;
  color: var(--c-muted);
}

.audit-detail {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  padding: 8px 24px 12px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.pager {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
}

.token-created {
  display: grid;
  gap: 12px;
}

.muted {
  font-size: 12px;
  color: var(--c-muted);
}

.inline-error {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-bad);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
}

.pane-enter-active,
.pane-leave-active {
  transition: opacity 160ms var(--ease-out);
}

.pane-enter-from,
.pane-leave-to {
  opacity: 0;
}

@media (max-width: 780px) {
  .kpi-strip {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .kpi-cell {
    padding: 0;
    border-left: 0;
  }
  .gallery-row,
  .gallery-row.is-link {
    grid-template-columns: minmax(0, 1fr) auto;
  }
  .audit-row {
    grid-template-columns: 10px minmax(0, 1fr);
  }
  .audit-actor,
  .audit-time {
    grid-column: 2;
  }
}
</style>
