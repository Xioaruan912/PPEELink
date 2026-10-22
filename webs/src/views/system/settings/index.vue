<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from "vue";
import { useRouter } from "vue-router";
import { Search } from "@element-plus/icons-vue";
import IconPulse from "~icons/ph/pulse";
import IconCloudDownload from "~icons/ph/cloud-arrow-down";
import IconKey from "~icons/ph/key";
import IconListMagnifyingGlass from "~icons/ph/list-magnifying-glass";
import IconLink from "~icons/ph/link";
import IconRefresh from "~icons/ph/arrow-clockwise";
import IconWarning from "~icons/ph/warning-circle";
import IconCheck from "~icons/ph/check-circle";
import IconCopy from "~icons/ph/copy";
import IconTrash from "~icons/ph/trash";
import IconPlus from "~icons/ph/plus";
import IconDatabase from "~icons/ph/database";
import IconDownloadSimple from "~icons/ph/download-simple";
import IconUploadSimple from "~icons/ph/upload-simple";
import IconArrowSquareOut from "~icons/ph/arrow-square-out";
import IconSliders from "~icons/ph/sliders-horizontal";
import IconBroadcast from "~icons/ph/broadcast";
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
  getAlertSetting,
  getNodeHealthEvents,
  getNodeOverview,
  SetNodeForceDeliver,
  updateAlertSetting,
} from "@/api/subcription/node";
import {
  EmptyState,
  PageHeader,
  UiButton,
  UiDialog,
  UiPanel,
  UiSkeleton,
  UiTag,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import SecretLink from "@/views/subcription/subs/SecretLink.vue";
import {
  addProbe,
  deleteProbe,
  getProbes,
  type Probe,
} from "@/api/subcription/probe";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "SystemSettings" });

type Phase = "idle" | "loading" | "ready" | "error";

interface AlertSetting {
  enabled: boolean;
  webhookUrl: string;
  failureThreshold: number;
  autoBlockInvalid: boolean;
  maintenanceStart: string;
  maintenanceEnd: string;
  expiryReminderDays: number;
  autoDisableExpired: boolean;
  accessLimit: number;
  autoDisableOverLimit: boolean;
}

interface HealthEvent {
  id: number;
  nodeId: number;
  nodeName: string;
  type: string;
  message: string;
  createdAt: string;
}

interface NodeOverviewLite {
  id: number;
  name: string;
  server?: string;
  valid: boolean;
  forceDeliver?: boolean;
  checkedAt?: string;
}

const router = useRouter();
const origin = window.location.origin;
const SECTION_KEY = "ppeelink.settings.section";

const sections = [
  {
    value: "detect",
    label: "节点检测",
    hint: "自动判定与例外",
    icon: IconPulse,
  },
  {
    value: "probes",
    label: "检测探针",
    hint: "多网络视角",
    icon: IconBroadcast,
  },
  {
    value: "backup",
    label: "备份恢复",
    hint: "快照与配置迁移",
    icon: IconCloudDownload,
  },
  { value: "tokens", label: "API 令牌", hint: "脚本与自动化", icon: IconKey },
  {
    value: "audit",
    label: "操作审计",
    hint: "谁在什么时候改过什么",
    icon: IconListMagnifyingGlass,
  },
  { value: "links", label: "短链", hint: "分享与跳转", icon: IconLink },
] as const;
type SectionValue = (typeof sections)[number]["value"];

const storedSection = localStorage.getItem(SECTION_KEY);
const tab = ref<SectionValue>(
  sections.some((section) => section.value === storedSection)
    ? (storedSection as SectionValue)
    : "detect"
);

const online = ref(navigator.onLine);
const refreshing = ref(false);
const lastRefreshed = ref<Date | null>(null);
const refreshedLabel = computed(() =>
  lastRefreshed.value
    ? `最后刷新 ${lastRefreshed.value.toLocaleTimeString("zh-CN", {
        hour: "2-digit",
        minute: "2-digit",
      })}`
    : "尚未刷新"
);

// ---- 节点检测 -------------------------------------------------------------
const detection = reactive({
  phase: "idle" as Phase,
  error: "",
  partial: false,
  setting: null as AlertSetting | null,
  events: [] as HealthEvent[],
  nodes: [] as NodeOverviewLite[],
});
const autoBlockInvalid = ref(true);
const failureThreshold = ref(3);
const savingDetection = ref<"" | "auto" | "threshold">("");
const savedFlash = ref<"" | "auto" | "threshold">("");
const detecting = ref(false);
const clearingExceptionId = ref<number | null>(null);
let flashTimer: number | undefined;

const invalidNodes = computed(() =>
  detection.nodes.filter((node) => !node.valid)
);
const exceptionDialog = ref(false);
const exceptionQuery = ref("");
const exceptionNodeOptions = computed(() => {
  const query = exceptionQuery.value.trim().toLowerCase();
  if (!query) return detection.nodes;
  return detection.nodes.filter((node) =>
    `${node.name} ${node.server || ""}`.toLowerCase().includes(query)
  );
});
const forceDeliverNodes = computed(() =>
  detection.nodes.filter((node) => node.forceDeliver)
);

function flash(key: "auto" | "threshold") {
  savedFlash.value = key;
  window.clearTimeout(flashTimer);
  flashTimer = window.setTimeout(() => {
    savedFlash.value = "";
  }, 2400);
}

async function loadDetection(force = false) {
  if (!force && detection.phase === "ready") return;
  detection.phase = "loading";
  detection.error = "";
  detection.partial = false;
  try {
    const [alertResult, eventsResult, overviewResult] = await Promise.allSettled(
      [getAlertSetting(), getNodeHealthEvents(30), getNodeOverview()]
    );
    let failed = 0;
    if (alertResult.status === "fulfilled") {
      const setting = alertResult.value?.data as AlertSetting | undefined;
      detection.setting = setting ?? null;
      autoBlockInvalid.value = setting?.autoBlockInvalid !== false;
      failureThreshold.value = setting?.failureThreshold ?? 3;
    } else failed++;
    if (eventsResult.status === "fulfilled") {
      detection.events = eventsResult.value?.data || [];
    } else failed++;
    if (overviewResult.status === "fulfilled") {
      detection.nodes = overviewResult.value?.data || [];
    } else failed++;
    detection.partial = failed > 0;
    detection.phase = failed === 3 ? "error" : "ready";
    detection.error =
      failed > 0 ? "部分数据读取失败，页面显示的是能拿到的那部分。" : "";
  } catch (error: any) {
    detection.phase = "error";
    detection.error = error?.message || "读取节点检测设置失败";
  } finally {
    lastRefreshed.value = new Date();
  }
}

async function saveAutoBlock(next: boolean) {
  if (!online.value) {
    ElMessage.warning("网络已断开，恢复连接后再试");
    return;
  }
  const previous = autoBlockInvalid.value;
  autoBlockInvalid.value = next;
  savingDetection.value = "auto";
  try {
    await updateAlertSetting({
      autoBlockInvalid: next,
      failureThreshold: failureThreshold.value,
    });
    flash("auto");
    if (detection.setting) detection.setting.autoBlockInvalid = next;
  } catch {
    autoBlockInvalid.value = previous;
    ElMessage.error("保存失败，请检查连接后重试");
  } finally {
    savingDetection.value = "";
  }
}

function onAutoBlockChange(value: string | number | boolean) {
  void saveAutoBlock(Boolean(value));
}

async function saveThreshold() {
  if (!online.value) {
    ElMessage.warning("网络已断开，恢复连接后再试");
    return;
  }
  const value = Math.min(20, Math.max(1, Number(failureThreshold.value) || 3));
  failureThreshold.value = value;
  savingDetection.value = "threshold";
  try {
    await updateAlertSetting({
      autoBlockInvalid: autoBlockInvalid.value,
      failureThreshold: value,
    });
    if (detection.setting) detection.setting.failureThreshold = value;
    flash("threshold");
  } catch {
    ElMessage.error("保存失败，请检查连接后重试");
  } finally {
    savingDetection.value = "";
  }
}

async function runDetection() {
  if (detecting.value) return;
  detecting.value = true;
  try {
    const { data } = await getNodeOverview(true);
    detection.nodes = data || [];
    ElMessage.success("检测完成");
  } catch {
    ElMessage.error("检测失败，请稍后重试");
  } finally {
    detecting.value = false;
    lastRefreshed.value = new Date();
  }
}

async function toggleException(node: NodeOverviewLite) {
  if (!online.value || clearingExceptionId.value) return;
  const next = !node.forceDeliver;
  clearingExceptionId.value = node.id;
  try {
    await SetNodeForceDeliver(node.id, next);
    node.forceDeliver = next;
    ElMessage.success(
      next ? "已设为始终下发" : "已恢复按检测结果下发"
    );
  } catch {
    ElMessage.error("操作失败，请稍后重试");
  } finally {
    clearingExceptionId.value = null;
  }
}

function goNodesFromPicker() {
  exceptionDialog.value = false;
  goNodes();
}

async function clearException(node: NodeOverviewLite) {
  if (!online.value) return;
  clearingExceptionId.value = node.id;
  try {
    await SetNodeForceDeliver(node.id, false);
    detection.nodes = detection.nodes.map((item) =>
      item.id === node.id ? { ...item, forceDeliver: false } : item
    );
    ElMessage.success("已恢复按检测结果下发");
  } catch {
    ElMessage.error("操作失败，请稍后重试");
  } finally {
    clearingExceptionId.value = null;
  }
}

// ---- 检测探针 -------------------------------------------------------------
const probesState = reactive({
  phase: "idle" as Phase,
  error: "",
  items: [] as Probe[],
});
const probeDialog = ref(false);
const probeForm = reactive({ name: "", region: "" });
const probeError = ref("");
const addingProbe = ref(false);
const newProbe = ref<{
  name: string;
  token: string;
  install: string;
} | null>(null);
const deletingProbeId = ref<number | null>(null);

async function loadProbes(force = false) {
  if (!force && probesState.phase === "ready") return;
  probesState.phase = "loading";
  probesState.error = "";
  try {
    const { data } = await getProbes();
    probesState.items = data || [];
    probesState.phase = "ready";
  } catch (error: any) {
    probesState.phase = "error";
    probesState.error = error?.message || "读取探针失败";
  } finally {
    lastRefreshed.value = new Date();
  }
}

function openProbeDialog() {
  probeForm.name = "";
  probeForm.region = "";
  probeError.value = "";
  newProbe.value = null;
  probeDialog.value = true;
}

async function submitProbe() {
  if (!online.value || addingProbe.value) return;
  if (!probeForm.name.trim()) {
    probeError.value = "请填写探针名称。";
    return;
  }
  addingProbe.value = true;
  probeError.value = "";
  try {
    const { data } = await addProbe({
      name: probeForm.name.trim(),
      region: probeForm.region.trim(),
    });
    newProbe.value = {
      name: probeForm.name.trim(),
      token: data?.token || "",
      install: data?.install || "",
    };
    await loadProbes(true);
  } catch (error: any) {
    probeError.value = error?.message || "创建探针失败，请稍后重试。";
  } finally {
    addingProbe.value = false;
  }
}

async function removeProbe(probe: Probe) {
  if (!online.value || deletingProbeId.value) return;
  try {
    await ElMessageBox.confirm(
      `删除探针「${probe.name}」？它的检测结果会一起删除。`,
      "删除探针",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  deletingProbeId.value = probe.id;
  try {
    await deleteProbe(probe.id);
    ElMessage.success("探针已删除");
    await loadProbes(true);
  } catch {
    ElMessage.error("删除失败，请稍后重试");
  } finally {
    deletingProbeId.value = null;
  }
}

watch(probeDialog, (open) => {
  if (!open) newProbe.value = null;
});

// ---- 备份 ---------------------------------------------------------------
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
    summaryState.error = error?.message || "读取备份统计失败";
  } finally {
    lastRefreshed.value = new Date();
  }
}

async function fullBackup() {
  if (!online.value) return;
  backupBusy.value = "full";
  try {
    await downloadFullBackup();
    lastBackupAt.value = new Date().toISOString();
    localStorage.setItem("ppeelink.backup.lastFull", lastBackupAt.value);
    ElMessage.success("完整备份已开始下载，建议再存一份到其他设备");
  } catch (error: any) {
    ElMessage.error(error?.message || "备份失败，请稍后重试");
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
    anchor.download = `ppeelink-config-${new Date()
      .toISOString()
      .slice(0, 10)}.json`;
    anchor.click();
    URL.revokeObjectURL(url);
    ElMessage.success("配置已导出，可以在另一台面板导入");
  } catch {
    ElMessage.error("导出失败，请稍后重试");
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
      importError.value = "不是有效的 JSON 配置文件，请重新选择。";
    });
}

async function onImportFile(event: Event) {
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
      `导入「${file.name}」？将合并模板与订阅配置，节点、账户和令牌不会被修改。`,
      "导入配置备份",
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
    importError.value = error?.message || "导入失败，请检查文件后重试。";
    ElMessage.error(importError.value);
  } finally {
    backupBusy.value = "";
  }
}

// ---- API 令牌 ------------------------------------------------------------
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

const scopeLabel = (scope: string) =>
  scopeOptions.find((option) => option.value === scope)?.label || scope;
const curlExample = computed(
  () =>
    `curl -H "Authorization: Bearer <token>" \
  ${origin}/api/v1/nodes/get`
);

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
  } finally {
    lastRefreshed.value = new Date();
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
    tokenError.value = "请填写令牌名称，例如「备份脚本」。";
    return;
  }
  if (!tokenForm.scopes.length) {
    tokenError.value = "至少选择一项权限。";
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
    tokenError.value = error?.message || "创建失败，请稍后重试。";
  } finally {
    creatingToken.value = false;
  }
}

async function revoke(item: ApiToken) {
  if (!online.value || revokingId.value) return;
  try {
    await ElMessageBox.confirm(
      `吊销令牌「${item.name}」？使用它的脚本会立即失效。`,
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
  revokingId.value = item.id;
  try {
    await revokeApiToken(item.id);
    ElMessage.success("令牌已吊销");
    await loadTokens(true);
  } catch {
    ElMessage.error("吊销失败，请稍后重试");
  } finally {
    revokingId.value = null;
  }
}

function expiryLabel(item: ApiToken) {
  if (!item.expiresAt) return "永久";
  const expired = new Date(item.expiresAt).getTime() < Date.now();
  return expired ? "已过期" : formatDateTime(item.expiresAt);
}

// ---- 审计 ----------------------------------------------------------------
const auditState = reactive({
  phase: "idle" as Phase,
  error: "",
  items: [] as AuditEntry[],
});
const auditQuery = ref("");
const auditStatus = ref<"all" | "ok" | "fail">("all");
const auditPage = ref(1);
const auditPageSize = 20;
const auditFiltered = computed(() => {
  const query = auditQuery.value.trim().toLowerCase();
  return auditState.items.filter((entry) => {
    if (auditStatus.value === "ok" && entry.status >= 400) return false;
    if (auditStatus.value === "fail" && entry.status < 400) return false;
    if (!query) return true;
    return `${entry.action} ${entry.path} ${entry.actor} ${entry.method} ${entry.ip}`
      .toLowerCase()
      .includes(query);
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
const auditTone = (status: number) =>
  status >= 500 ? "bad" : status >= 400 ? "warn" : "ok";

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
    auditState.error = error?.message || "读取审计记录失败";
  } finally {
    lastRefreshed.value = new Date();
  }
}

function clearAuditFilters() {
  auditQuery.value = "";
  auditStatus.value = "all";
}

// ---- 短链 ----------------------------------------------------------------
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
  } finally {
    lastRefreshed.value = new Date();
  }
}

async function copyText(value: string, message = "已复制") {
  try {
    await navigator.clipboard.writeText(value);
    ElMessage.success(message);
  } catch {
    ElMessage.warning("复制失败，请手动选择后复制");
  }
}

function shortLinkURL(code: string) {
  return `${window.location.origin}/s/${code}`;
}

function openLink(item: ShortLinkItem) {
  if (!item.target) return;
  window.open(item.target, "_blank", "noopener,noreferrer");
}

async function removeLink(item: ShortLinkItem) {
  if (!online.value || deletingLinkId.value) return;
  try {
    await ElMessageBox.confirm(
      `删除短链 /s/${item.code}？已分享出去的地址会立即失效。`,
      "删除短链",
      { type: "warning", confirmButtonText: "删除", cancelButtonText: "取消" }
    );
  } catch {
    return;
  }
  deletingLinkId.value = item.id;
  try {
    await deleteShortLink(item.id);
    ElMessage.success("短链已删除");
    await loadLinks(true);
  } catch {
    ElMessage.error("删除失败，请稍后重试");
  } finally {
    deletingLinkId.value = null;
  }
}

// ---- 壳层 ----------------------------------------------------------------
function metaFor(value: SectionValue) {
  if (value === "detect") {
    if (detection.phase !== "ready") return "·";
    return autoBlockInvalid.value ? "自动" : "手动";
  }
  if (value === "probes") {
    if (probesState.phase !== "ready") return "·";
    return `${probesState.items.filter((probe) => probe.online).length}/${
      probesState.items.length
    }`;
  }
  if (value === "backup") {
    return summary.value ? `${summary.value.templates ?? 0} 模板` : "·";
  }
  if (value === "tokens") {
    return tokensState.phase === "ready" ? String(tokensState.items.length) : "·";
  }
  if (value === "audit") {
    return auditState.phase === "ready" ? String(auditState.items.length) : "·";
  }
  return linksState.phase === "ready" ? String(linksState.items.length) : "·";
}

function loadSection(value: SectionValue, force = false): Promise<void> {
  switch (value) {
    case "detect":
      return loadDetection(force);
    case "probes":
      return loadProbes(force);
    case "backup":
      return loadBackup(force);
    case "tokens":
      return loadTokens(force);
    case "audit":
      return loadAudit(force);
    case "links":
      return loadLinks(force);
    default:
      return Promise.resolve();
  }
}

async function refreshActive() {
  refreshing.value = true;
  try {
    await loadSection(tab.value, true);
    lastRefreshed.value = new Date();
  } finally {
    refreshing.value = false;
  }
}

function goNodes() {
  void router.push("/subcription/nodes");
}

function goSubs() {
  void router.push("/subcription/subs");
}

function syncOnline() {
  online.value = navigator.onLine;
}

watch(tab, (value) => {
  localStorage.setItem(SECTION_KEY, value);
  void loadSection(value);
});

onMounted(() => {
  window.addEventListener("online", syncOnline);
  window.addEventListener("offline", syncOnline);
  void loadSection(tab.value);
});

onBeforeUnmount(() => {
  window.removeEventListener("online", syncOnline);
  window.removeEventListener("offline", syncOnline);
  window.clearTimeout(flashTimer);
});
</script>
<template>
  <div class="settings-page">
    <PageHeader
      title="系统设置"
      description="节点检测、数据备份、访问令牌与审计，集中在一个地方。"
    >
      <template #actions>
        <span class="refresh-meta">{{ refreshedLabel }}</span>
        <UiButton
          variant="ghost"
          :icon="IconRefresh"
          :loading="refreshing"
          @click="refreshActive"
          >刷新</UiButton
        >
      </template>
    </PageHeader>

    <div v-if="!online" class="offline-banner" role="status">
      <IconWarning />
      <span>网络已断开，设置只能查看，恢复连接后才能保存。</span>
    </div>

    <div class="settings-layout">
      <nav class="settings-rail" aria-label="设置分区">
        <button
          v-for="section in sections"
          :key="section.value"
          type="button"
          class="rail-item"
          :class="{ 'is-active': tab === section.value }"
          :aria-current="tab === section.value ? 'page' : undefined"
          @click="tab = section.value"
        >
          <span class="rail-icon"><component :is="section.icon" /></span>
          <span class="rail-copy"
            ><strong>{{ section.label }}</strong
            ><small>{{ section.hint }}</small></span
          >
          <span class="rail-meta">{{ metaFor(section.value) }}</span>
        </button>
        <div class="rail-foot">
          <span class="rail-dot" :class="online ? 'is-ok' : 'is-bad'"></span>
          <span>{{ online ? "面板连接正常" : "等待网络恢复" }}</span>
        </div>
      </nav>

      <main class="settings-main">
        <Transition name="pane" mode="out-in">
          <!-- 节点检测 -->
          <div v-if="tab === 'detect'" key="detect" class="section-stack">
            <section class="detect-hero">
              <div class="detect-copy">
                <span class="kicker">NODE HEALTH</span>
                <h2>自动判定无效节点</h2>
                <p>
                  连续
                  <strong>{{ failureThreshold }}</strong>
                  次连接失败后，节点会从订阅里暂不下发，恢复后自动回来。关闭后检测结果只作展示，所有节点都会下发。
                </p>
                <div class="detect-facts">
                  <span
                    ><i
                      class="fact-dot"
                      :class="autoBlockInvalid ? 'is-ok' : 'is-off'"
                    ></i
                    >{{ autoBlockInvalid ? "自动判定已开启" : "自动判定已关闭" }}</span
                  >
                  <span>当前 {{ invalidNodes.length }} 个节点检测失败</span>
                  <span>{{ forceDeliverNodes.length }} 个例外节点</span>
                </div>
              </div>
              <div class="detect-control">
                <el-switch
                  :model-value="autoBlockInvalid"
                  size="large"
                  :loading="savingDetection === 'auto'"
                  :disabled="!online || !detection.setting"
                  :aria-label="`自动判定无效节点：${
                    autoBlockInvalid ? '开启' : '关闭'
                  }`"
                  @change="onAutoBlockChange"
                />
                <span class="switch-state">{{
                  autoBlockInvalid ? "已开启" : "已关闭"
                }}</span>
                <Transition name="fade"
                  ><small v-if="savedFlash === 'auto'">已保存</small></Transition
                >
              </div>
            </section>

            <div v-if="detection.phase === 'error'" class="state-error" role="alert">
              <IconWarning />
              <p>{{ detection.error }}</p>
              <UiButton @click="loadDetection(true)">重试</UiButton>
            </div>
            <div v-else-if="detection.partial" class="state-note" role="status">
              <IconWarning />
              <p>{{ detection.error }}</p>
              <UiButton variant="ghost" @click="loadDetection(true)"
                >重新读取</UiButton
              >
            </div>

            <div class="two-col">
              <UiPanel
                title="检测规则"
                description="失败次数达到阈值后触发一次无效事件。"
              >
                <div class="rule-row">
                  <div class="rule-label">
                    <strong>连续失败次数</strong>
                    <span>1 到 20 次，默认 3 次。</span>
                  </div>
                  <el-input-number
                    v-model="failureThreshold"
                    :min="1"
                    :max="20"
                    :disabled="!online || !detection.setting"
                    aria-label="连续失败次数"
                  />
                  <UiButton
                    variant="primary"
                    :loading="savingDetection === 'threshold'"
                    :disabled="!online || !detection.setting"
                    @click="saveThreshold"
                    >保存</UiButton
                  >
                  <Transition name="fade"
                    ><span v-if="savedFlash === 'threshold'" class="inline-ok"
                      >已保存</span
                    ></Transition
                  >
                </div>
                <p class="hint">
                  每 10 分钟自动检测一次；节点页的“立即检测”可以手动触发。阈值同时用于健康事件与告警。
                </p>
                <div class="panel-actions">
                  <UiButton
                    :icon="IconPulse"
                    :loading="detecting"
                    :disabled="!online"
                    @click="runDetection"
                    >立即检测</UiButton
                  >
                  <UiButton variant="ghost" @click="goNodes">打开节点页</UiButton>
                </div>
              </UiPanel>

              <UiPanel
                title="例外节点"
                :count="forceDeliverNodes.length"
                description="这些节点即使检测失败也照常下发。"
              >
                <template #actions>
                  <UiButton
                    :icon="IconSliders"
                    :disabled="!online || !detection.nodes.length"
                    @click="exceptionDialog = true"
                    >选择节点</UiButton
                  >
                </template>
                <UiSkeleton
                  v-if="detection.phase === 'loading'"
                  :lines="3"
                  height="34px"
                />
                <EmptyState
                  v-else-if="!forceDeliverNodes.length"
                  size="section"
                  title="没有例外节点"
                  description="点右上角“选择节点”，或到节点页的节点菜单里打开“始终下发”。"
                >
                  <template #action
                    ><UiButton @click="goNodes">去节点页设置</UiButton></template
                  >
                </EmptyState>
                <ul v-else class="exception-list">
                  <li
                    v-for="node in forceDeliverNodes.slice(0, 6)"
                    :key="node.id"
                  >
                    <span class="node-name"
                      >{{ node.name
                      }}<small>{{
                        node.valid ? "检测有效" : "检测无效，仍会下发"
                      }}</small></span
                    >
                    <UiTag :tone="node.valid ? 'ok' : 'bad'">{{
                      node.valid ? "有效" : "无效"
                    }}</UiTag>
                    <UiButton
                      variant="ghost"
                      :loading="clearingExceptionId === node.id"
                      :disabled="!online"
                      @click="clearException(node)"
                      >恢复</UiButton
                    >
                  </li>
                </ul>
                <p v-if="forceDeliverNodes.length > 6" class="hint">
                  还有 {{ forceDeliverNodes.length - 6 }} 个例外节点，去节点页查看。
                </p>
              </UiPanel>
            </div>

            <UiPanel
              title="最近健康事件"
              :count="detection.events.length"
              description="节点连续失败与恢复的时间线。"
              flush
            >
              <UiSkeleton
                v-if="detection.phase === 'loading'"
                class="panel-pad"
                :lines="4"
                height="44px"
              />
              <div
                v-else-if="detection.phase === 'error'"
                class="state-error panel-pad"
                role="alert"
              >
                <IconWarning />
                <p>{{ detection.error }}</p>
                <UiButton @click="loadDetection(true)">重试</UiButton>
              </div>
              <EmptyState
                v-else-if="!detection.events.length"
                size="section"
                title="还没有健康事件"
                description="节点连续失败达到阈值后，这里会记录一条事件。"
              />
              <ol v-else class="event-list">
                <li v-for="event in detection.events" :key="event.id">
                  <span
                    class="event-mark"
                    :class="event.type === 'down' ? 'is-bad' : 'is-ok'"
                  ></span>
                  <div class="event-main">
                    <strong>{{ event.nodeName || `节点 ${event.nodeId}` }}</strong>
                    <p>{{ event.message }}</p>
                  </div>
                  <UiTag :tone="event.type === 'down' ? 'bad' : 'ok'">{{
                    event.type === "down" ? "无效" : "恢复"
                  }}</UiTag>
                  <time class="mono">{{ formatDateTime(event.createdAt) }}</time>
                </li>
              </ol>
            </UiPanel>
          </div>

          <!-- 检测探针 -->
          <div v-else-if="tab === 'probes'" key="probes" class="section-stack">
            <UiPanel
              title="检测探针"
              :count="probesState.items.length"
              description="在另一个网络里运行探针，节点有效性可以按多视角判断。"
              flush
            >
              <template #actions>
                <UiButton
                  variant="primary"
                  :icon="IconPlus"
                  :disabled="!online"
                  @click="openProbeDialog"
                  >添加探针</UiButton
                >
              </template>
              <UiSkeleton
                v-if="probesState.phase === 'loading'"
                class="panel-pad"
                :lines="3"
                height="48px"
              />
              <div
                v-else-if="probesState.phase === 'error'"
                class="state-error panel-pad"
                role="alert"
              >
                <IconWarning />
                <p>{{ probesState.error }}</p>
                <UiButton @click="loadProbes(true)">重试</UiButton>
              </div>
              <EmptyState
                v-else-if="!probesState.items.length"
                size="section"
                title="还没有检测探针"
                description="装一台不同网络的探针，节点检测就不再只依赖面板所在网络。"
              >
                <template #icon>
                  <span class="empty-icon"><IconBroadcast /></span>
                </template>
                <template #action>
                  <UiButton :icon="IconPlus" @click="openProbeDialog"
                    >添加第一台探针</UiButton
                  >
                </template>
              </EmptyState>
              <div v-else class="probe-list">
                <div
                  v-for="probe in probesState.items"
                  :key="probe.id"
                  class="probe-row"
                >
                  <span
                    class="probe-dot"
                    :class="probe.online ? 'is-ok' : 'is-off'"
                  ></span>
                  <div class="probe-main">
                    <strong>{{ probe.name }}</strong>
                    <small
                      >{{ probe.region || "未标注地区" }} ·
                      {{
                        probe.lastSeenAt
                          ? `最近连接 ${formatDateTime(probe.lastSeenAt)}`
                          : "从未连接"
                      }}</small
                    >
                  </div>
                  <UiTag :tone="probe.online ? 'ok' : 'neutral'">{{
                    probe.online ? "在线" : "离线"
                  }}</UiTag>
                  <RowIconButton
                    :icon="IconTrash"
                    :label="`删除探针 ${probe.name}`"
                    @click="removeProbe(probe)"
                  />
                </div>
              </div>
            </UiPanel>

            <UiPanel
              title="检测策略怎么用"
              description="节点默认只看面板，可以在节点页改策略。"
            >
              <ul class="tips">
                <li>
                  部署在另一个网络里的探针，能发现面板看不到的 IPv6-only
                  或跨网节点。
                </li>
                <li>
                  节点菜单里的“检测策略…”可以选任一探针、全部探针或手动有效。
                </li>
                <li>
                  探针每 5 分钟拉取一次节点列表并回报 TCP 结果，结果超过 12
                  分钟视为过期。
                </li>
              </ul>
            </UiPanel>
          </div>

          <!-- 备份恢复 -->
          <div v-else-if="tab === 'backup'" key="backup" class="section-stack">
            <div class="two-col">
              <UiPanel
                title="完整备份"
                description="数据库快照加全部模板，打包为 tar.gz。"
              >
                <div class="backup-lead">
                  <span class="backup-icon"><IconDatabase /></span>
                  <p>
                    包含节点、订阅、账户与模板。迁移或重装时停服解压覆盖即可。
                  </p>
                </div>
                <dl class="mini-stats">
                  <div>
                    <dt>模板</dt>
                    <dd>{{ summary?.templates ?? "·" }}</dd>
                  </div>
                  <div>
                    <dt>订阅</dt>
                    <dd>{{ summary?.subscriptions ?? "·" }}</dd>
                  </div>
                  <div>
                    <dt>上次下载</dt>
                    <dd class="small">
                      {{ lastBackupAt ? formatDateTime(lastBackupAt) : "尚未记录" }}
                    </dd>
                  </div>
                </dl>
                <p v-if="summaryState.phase === 'error'" class="inline-error">
                  {{ summaryState.error }}
                  <button type="button" @click="loadBackup(true)">重试</button>
                </p>
                <template #actions>
                  <UiButton
                    variant="primary"
                    :icon="IconDownloadSimple"
                    :loading="backupBusy === 'full'"
                    :disabled="!online"
                    @click="fullBackup"
                    >下载完整备份</UiButton
                  >
                </template>
              </UiPanel>

              <UiPanel
                title="配置导入与导出"
                description="只含模板与订阅，不含节点与凭据。"
              >
                <div class="config-row">
                  <div>
                    <strong>导出配置</strong>
                    <p>JSON 文件，可在另一台面板导入。</p>
                  </div>
                  <UiButton
                    :icon="IconDownloadSimple"
                    :loading="backupBusy === 'export'"
                    :disabled="!online"
                    @click="exportConfig"
                    >导出</UiButton
                  >
                </div>
                <div
                  class="dropzone"
                  :class="{ 'is-dragging': dragging }"
                  @dragover.prevent="dragging = true"
                  @dragleave="dragging = false"
                  @drop.prevent="onDrop"
                >
                  <IconUploadSimple />
                  <div>
                    <strong>{{
                      importFile ? importFile.name : "拖入或选择备份 JSON"
                    }}</strong>
                    <small>{{
                      importFile
                        ? fileSizeLabel
                        : "合并恢复模板与订阅，不改节点与账户"
                    }}</small>
                  </div>
                  <UiButton :disabled="!online" @click="chooseImport"
                    >选择文件</UiButton
                  >
                </div>
                <p v-if="importError" class="inline-error" role="alert">
                  {{ importError }}
                </p>
                <UiButton
                  v-if="importFile"
                  variant="primary"
                  class="import-submit"
                  :loading="backupBusy === 'import'"
                  :disabled="!online"
                  @click="confirmImport"
                  >导入并合并</UiButton
                >
                <input
                  ref="importInput"
                  type="file"
                  accept="application/json,.json"
                  hidden
                  @change="onImportFile"
                />
              </UiPanel>
            </div>

            <UiPanel
              title="恢复流程"
              description="三步完成迁移，每一步都不会动原服务器的数据。"
            >
              <ol class="steps">
                <li>
                  <span class="step-no">01</span>
                  <div>
                    <strong>在新服务器停服</strong>
                    <p>解压 tar.gz，覆盖 db 与 template 目录。</p>
                  </div>
                </li>
                <li>
                  <span class="step-no">02</span>
                  <div>
                    <strong>启动面板并登录</strong>
                    <p>账户、令牌与订阅随数据库一起恢复。</p>
                  </div>
                </li>
                <li>
                  <span class="step-no">03</span>
                  <div>
                    <strong>核对节点与订阅</strong>
                    <p>打开节点页跑一次检测，确认网络环境一致。</p>
                  </div>
                </li>
              </ol>
            </UiPanel>
          </div>

          <!-- API 令牌 -->
          <div v-else-if="tab === 'tokens'" key="tokens" class="split">
            <UiPanel
              title="API 令牌"
              :count="tokensState.items.length"
              description="给脚本或自动化用的访问凭据，权限按需最小化。"
              flush
            >
              <template #actions>
                <UiButton
                  v-if="tokensState.items.length"
                  variant="primary"
                  :icon="IconPlus"
                  :disabled="!online"
                  @click="openTokenDialog"
                  >新建令牌</UiButton
                >
              </template>
              <UiSkeleton
                v-if="tokensState.phase === 'loading'"
                class="panel-pad"
                :lines="4"
                height="48px"
              />
              <div
                v-else-if="tokensState.phase === 'error'"
                class="state-error panel-pad"
                role="alert"
              >
                <IconWarning />
                <p>{{ tokensState.error }}</p>
                <UiButton @click="loadTokens(true)">重试</UiButton>
              </div>
              <div v-else-if="!tokensState.items.length" class="empty-row">
                <span class="empty-visual"><IconKey /></span>
                <div class="empty-copy">
                  <h3>还没有令牌</h3>
                  <p>给备份脚本或自动化流程建一个，权限按需最小化；随时可以吊销。</p>
                </div>
                <div class="empty-actions">
                  <UiButton
                    variant="primary"
                    :icon="IconPlus"
                    :disabled="!online"
                    @click="openTokenDialog"
                    >新建第一个令牌</UiButton
                  >
                  <small>令牌只显示一次</small>
                </div>
              </div>
              <div v-else class="data-table token-table">
                <div class="table-head">
                  <span>名称</span><span>权限</span><span>最近使用</span
                  ><span>有效期</span><span>状态</span><span></span>
                </div>
                <div
                  v-for="item in tokensState.items"
                  :key="item.id"
                  class="table-row"
                  :class="{ 'is-off': !item.enabled }"
                >
                  <div class="cell-main">
                    <strong>{{ item.name }}</strong>
                    <code>{{ item.tokenPrefix }}…</code>
                  </div>
                  <div class="cell-tags">
                    <UiTag
                      v-for="scope in item.scopes.split(',')"
                      :key="scope"
                      :tone="scope === 'admin' ? 'warn' : 'neutral'"
                      >{{ scopeLabel(scope) }}</UiTag
                    >
                  </div>
                  <span class="cell-meta">{{
                    item.lastUsedAt ? formatDateTime(item.lastUsedAt) : "从未使用"
                  }}</span>
                  <span class="cell-meta">{{ expiryLabel(item) }}</span>
                  <UiTag :tone="item.enabled ? 'ok' : 'bad'">{{
                    item.enabled ? "启用" : "已吊销"
                  }}</UiTag>
                  <RowIconButton
                    v-if="item.enabled"
                    :icon="IconTrash"
                    :label="`吊销 ${item.name}`"
                    @click="revoke(item)"
                  />
                </div>
              </div>
            </UiPanel>

            <UiPanel
              title="调用示例"
              description="把令牌放进 Authorization 头。"
            >
              <pre class="code-block">{{ curlExample }}</pre>
              <ul class="tips">
                <li>只读令牌只能调用 GET 接口，写接口会返回 403。</li>
                <li>读写令牌可以改节点与订阅，管理令牌还能访问备份与审计。</li>
                <li>令牌只在创建时显示一次，吊销后使用它的脚本立即失效。</li>
              </ul>
              <UiButton
                variant="ghost"
                class="aside-action"
                @click="copyText(curlExample, '示例已复制')"
                >复制示例</UiButton
              >
            </UiPanel>
          </div>

          <!-- 操作审计 -->
          <div v-else-if="tab === 'audit'" key="audit" class="section-stack">
            <UiPanel
              title="操作审计"
              :count="auditState.items.length"
              description="最近的管理写操作，不记录请求内容。"
              flush
            >
              <div class="filter-bar">
                <el-input
                  v-model="auditQuery"
                  clearable
                  :prefix-icon="Search"
                  placeholder="搜索操作、路径、操作者或 IP"
                  aria-label="搜索审计记录"
                />
                <el-select
                  v-model="auditStatus"
                  class="status-filter"
                  aria-label="按结果筛选"
                >
                  <el-option label="全部结果" value="all" />
                  <el-option label="成功" value="ok" />
                  <el-option label="失败" value="fail" />
                </el-select>
                <span class="muted">{{ auditFiltered.length }} 条</span>
              </div>
              <UiSkeleton
                v-if="auditState.phase === 'loading'"
                class="panel-pad"
                :lines="6"
                height="40px"
              />
              <div
                v-else-if="auditState.phase === 'error'"
                class="state-error panel-pad"
                role="alert"
              >
                <IconWarning />
                <p>{{ auditState.error }}</p>
                <UiButton @click="loadAudit(true)">重试</UiButton>
              </div>
              <EmptyState
                v-else-if="!auditState.items.length"
                size="section"
                title="还没有审计记录"
                description="创建、修改或删除资源后，这里会出现一条记录。"
              />
              <EmptyState
                v-else-if="!auditFiltered.length"
                size="section"
                title="没有匹配的记录"
                description="换一个关键词，或把结果筛选恢复为全部。"
              >
                <template #action>
                  <UiButton @click="clearAuditFilters">清除筛选</UiButton>
                </template>
              </EmptyState>
              <template v-else>
                <div class="data-table audit-table">
                  <div class="table-head">
                    <span class="col-time">时间</span
                    ><span class="col-status">结果</span
                    ><span class="col-method">方法</span
                    ><span class="col-main">操作</span
                    ><span class="col-actor">操作者</span
                    ><span class="col-ip">IP</span>
                  </div>
                  <div
                    v-for="entry in auditPageItems"
                    :key="entry.id"
                    class="table-row"
                  >
                    <time class="col-time mono">{{
                      formatDateTime(entry.createdAt)
                    }}</time>
                    <UiTag
                      class="col-status"
                      :tone="auditTone(entry.status)"
                      >{{ entry.status }}</UiTag
                    >
                    <span class="col-method mono">{{ entry.method }}</span>
                    <div class="col-main audit-main">
                      <strong>{{ entry.action || `${entry.method} ${entry.path}` }}</strong>
                      <small>{{ entry.path }}</small>
                    </div>
                    <span class="col-actor">{{ entry.actor }}</span>
                    <span class="col-ip mono">{{ entry.ip }}</span>
                  </div>
                </div>
                <div v-if="auditPages > 1" class="pager">
                  <span>第 {{ auditPage }} / {{ auditPages }} 页</span>
                  <div>
                    <UiButton
                      variant="ghost"
                      :disabled="auditPage <= 1"
                      @click="auditPage--"
                      >上一页</UiButton
                    >
                    <UiButton
                      variant="ghost"
                      :disabled="auditPage >= auditPages"
                      @click="auditPage++"
                      >下一页</UiButton
                    >
                  </div>
                </div>
              </template>
            </UiPanel>
          </div>

          <!-- 短链 -->
          <div v-else key="links" class="split">
            <UiPanel
              title="短链"
              :count="linksState.items.length"
              description="分享订阅时生成的短链接，删除后立即失效。"
              flush
            >
              <UiSkeleton
                v-if="linksState.phase === 'loading'"
                class="panel-pad"
                :lines="4"
                height="48px"
              />
              <div
                v-else-if="linksState.phase === 'error'"
                class="state-error panel-pad"
                role="alert"
              >
                <IconWarning />
                <p>{{ linksState.error }}</p>
                <UiButton @click="loadLinks(true)">重试</UiButton>
              </div>
              <div v-else-if="!linksState.items.length" class="empty-row">
                <span class="empty-visual"><IconLink /></span>
                <div class="empty-copy">
                  <h3>还没有短链</h3>
                  <p>在订阅的分享面板里生成短链，发给客户端或临时链接会出现在这里。</p>
                </div>
                <div class="empty-actions">
                  <UiButton :disabled="!online" @click="goSubs"
                    >去订阅页生成</UiButton
                  >
                  <small>删除后立即失效</small>
                </div>
              </div>
              <div v-else class="data-table links-table">
                <div class="table-head">
                  <span>短链</span><span>目标</span><span>创建时间</span
                  ><span></span>
                </div>
                <div
                  v-for="item in linksState.items"
                  :key="item.id"
                  class="table-row"
                >
                  <div class="cell-main">
                    <strong class="link-code">/s/{{ item.code }}</strong>
                    <small class="link-remark">{{ item.remark || "未命名" }}</small>
                  </div>
                  <span class="link-target" :title="item.target">{{
                    item.target || "未记录目标"
                  }}</span>
                  <span class="cell-meta">{{
                    formatDateTime(item.createdAt)
                  }}</span>
                  <div class="link-actions">
                    <RowIconButton
                      :icon="IconCopy"
                      :label="`复制短链 /s/${item.code}`"
                      @click="copyText(shortLinkURL(item.code), '短链已复制')"
                    />
                    <RowIconButton
                      :icon="IconArrowSquareOut"
                      :label="`打开 /s/${item.code}`"
                      @click="openLink(item)"
                    />
                    <RowIconButton
                      :icon="IconTrash"
                      :label="`删除短链 /s/${item.code}`"
                      @click="removeLink(item)"
                    />
                  </div>
                </div>
              </div>
            </UiPanel>

            <UiPanel title="短链说明" description="适合发给客户端或临时分享。">
              <ul class="tips">
                <li>短链指向订阅或客户端的分享地址，目标地址保存在数据库里。</li>
                <li>删除后旧地址立即失效，已经发出去的聊天记录无法撤回。</li>
                <li>迁移面板时短链会随数据库一起带走，域名变化后需要重新生成。</li>
              </ul>
            </UiPanel>
          </div>
        </Transition>
      </main>
    </div>

    <UiDialog
      v-model="probeDialog"
      :title="newProbe ? '探针已创建' : '添加检测探针'"
      width="640px"
    >
      <template v-if="newProbe">
        <p class="dialog-note">
          在探针服务器上以 root 执行下面的命令。令牌只显示这一次。
        </p>
        <pre class="code-block probe-install">{{ newProbe.install }}</pre>
        <UiButton @click="copyText(newProbe.install, '安装命令已复制')"
          >复制安装命令</UiButton
        >
      </template>
      <el-form v-else label-position="top" @submit.prevent="submitProbe">
        <el-form-item label="名称" required>
          <el-input
            v-model="probeForm.name"
            maxlength="40"
            placeholder="例如 NoSLa_JP"
          />
        </el-form-item>
        <el-form-item label="地区">
          <el-input
            v-model="probeForm.region"
            maxlength="20"
            placeholder="例如 JP"
          />
        </el-form-item>
        <p v-if="probeError" class="field-error" role="alert">
          {{ probeError }}
        </p>
      </el-form>
      <template #footer>
        <UiButton @click="probeDialog = false">{{
          newProbe ? "完成" : "取消"
        }}</UiButton>
        <UiButton
          v-if="!newProbe"
          variant="primary"
          :loading="addingProbe"
          @click="submitProbe"
          >创建</UiButton
        >
      </template>
    </UiDialog>

    <UiDialog v-model="exceptionDialog" title="选择例外节点" width="560px">
      <p class="dialog-note exception-note">
        打开开关后，节点即使检测无效也会照常下发；手动暂停、额度限制仍然生效。
      </p>
      <el-input
        v-model="exceptionQuery"
        clearable
        :prefix-icon="Search"
        placeholder="搜索节点名称或主机"
        aria-label="搜索节点"
        class="exception-search"
      />
      <div v-if="!detection.nodes.length" class="exception-empty">
        <p>还没有节点，先去节点页添加。</p>
        <UiButton @click="goNodesFromPicker">去添加节点</UiButton>
      </div>
      <div v-else-if="!exceptionNodeOptions.length" class="exception-empty">
        <p>没有匹配「{{ exceptionQuery }}」的节点。</p>
        <UiButton variant="ghost" @click="exceptionQuery = ''"
          >清除搜索</UiButton
        >
      </div>
      <ul v-else class="exception-picker">
        <li v-for="node in exceptionNodeOptions" :key="node.id">
          <div class="pick-main">
            <strong>{{ node.name }}</strong>
            <small>{{ node.server || "主机未知" }}</small>
          </div>
          <UiTag :tone="node.valid ? 'ok' : 'bad'">{{
            node.valid ? "有效" : "无效"
          }}</UiTag>
          <el-switch
            :model-value="!!node.forceDeliver"
            :loading="clearingExceptionId === node.id"
            :disabled="!online"
            :aria-label="`${node.name} 始终下发`"
            @change="toggleException(node)"
          />
        </li>
      </ul>
      <template #footer>
        <UiButton @click="exceptionDialog = false">完成</UiButton>
      </template>
    </UiDialog>

    <UiDialog
      v-model="tokenDialog"
      :title="newToken ? '令牌已创建' : '新建 API 令牌'"
      width="480px"
    >
      <template v-if="newToken">
        <div class="token-success">
          <IconCheck />
          <p>令牌只显示这一次，关闭后无法再次查看。请立即复制保存。</p>
        </div>
        <SecretLink :value="newToken" label="API 令牌" />
        <p class="dialog-note">
          权限：{{ tokenForm.scopes.map(scopeLabel).join(" / ") }}；
          有效期：{{
            tokenForm.expiresInDays === 0
              ? "永久"
              : `${tokenForm.expiresInDays} 天`
          }}
        </p>
      </template>
      <el-form v-else label-position="top" @submit.prevent="submitToken">
        <el-form-item label="名称" required>
          <el-input
            v-model="tokenForm.name"
            maxlength="40"
            placeholder="例如：备份脚本"
          />
        </el-form-item>
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
        <p v-if="tokenError" class="field-error" role="alert">
          {{ tokenError }}
        </p>
      </el-form>
      <template #footer>
        <UiButton @click="tokenDialog = false">{{
          newToken ? "完成" : "取消"
        }}</UiButton>
        <UiButton
          v-if="!newToken"
          variant="primary"
          :loading="creatingToken"
          @click="submitToken"
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
.refresh-meta {
  color: var(--c-muted);
  font-size: 12px;
}
.offline-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  padding: 10px 14px;
  border: 1px solid color-mix(in srgb, var(--c-warn) 42%, var(--c-line));
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-warn) 8%, var(--c-surface));
  color: var(--c-warn-ink);
  font-size: 13px;
}
.offline-banner svg {
  width: 18px;
  height: 18px;
  flex: none;
}
.settings-layout {
  display: grid;
  grid-template-columns: 262px minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}
.settings-rail {
  position: sticky;
  top: 88px;
  display: grid;
  gap: 4px;
  padding: 8px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.rail-item {
  position: relative;
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border-radius: 12px;
  color: var(--c-muted);
  text-align: left;
  transition: background var(--duration-fast), color var(--duration-fast);
}
.rail-item:hover {
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.rail-item.is-active {
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.rail-item.is-active::before {
  content: "";
  position: absolute;
  top: 10px;
  bottom: 10px;
  left: 0;
  width: 2px;
  border-radius: 2px;
  background: var(--c-signal);
}
.rail-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
}
.rail-icon svg {
  width: 18px;
  height: 18px;
}
.rail-item.is-active .rail-icon {
  border-color: var(--c-signal);
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.rail-copy {
  display: grid;
  min-width: 0;
  gap: 1px;
}
.rail-copy strong {
  color: var(--c-ink);
  font: 500 14px/1.3 var(--font-ui);
}
.rail-copy small {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 11.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.rail-meta {
  padding: 4px 7px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font: 500 11px/1 var(--font-mono);
}
.rail-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
  padding: 10px;
  border-top: 1px solid var(--c-line);
  color: var(--c-muted);
  font-size: 12px;
}
.rail-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
}
.rail-dot.is-ok {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--c-ok) 18%, transparent);
}
.rail-dot.is-bad {
  background: var(--c-bad);
}
.settings-main {
  min-width: 0;
}
.section-stack {
  display: grid;
  gap: 16px;
}
.split {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(280px, 0.85fr);
  gap: 16px;
  align-items: start;
}
.two-col {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.detect-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 24px;
  padding: 22px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background:
    radial-gradient(
      120% 140% at 100% 0%,
      color-mix(in srgb, var(--c-signal) 12%, transparent),
      transparent 55%
    ),
    var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.kicker {
  color: var(--c-muted);
  font: 500 10.5px/1 var(--font-mono);
  letter-spacing: 0.14em;
}
.detect-copy h2 {
  margin: 8px 0 6px;
  font-size: 22px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.detect-copy p {
  max-width: 62ch;
  color: var(--c-muted);
  font-size: 13.5px;
  line-height: 1.7;
}
.detect-copy p strong {
  color: var(--c-ink);
  font-family: var(--font-mono);
}
.detect-facts {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 18px;
  margin-top: 14px;
  color: var(--c-muted);
  font-size: 12px;
}
.detect-facts span {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.fact-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-muted);
}
.fact-dot.is-ok {
  background: var(--c-ok);
}
.fact-dot.is-off {
  background: var(--c-warn);
}
.detect-control {
  display: grid;
  min-width: 120px;
  align-content: start;
  justify-items: end;
  gap: 6px;
}
.switch-state {
  color: var(--c-muted);
  font-size: 12.5px;
}
.detect-control small {
  color: var(--c-ok);
  font-size: 12px;
}
.state-error,
.state-note {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--c-bad) 35%, var(--c-line));
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-bad) 6%, var(--c-surface));
  color: var(--c-bad);
  font-size: 13px;
}
.state-note {
  border-color: color-mix(in srgb, var(--c-warn) 38%, var(--c-line));
  background: color-mix(in srgb, var(--c-warn) 7%, var(--c-surface));
  color: var(--c-warn-ink);
}
.state-error svg,
.state-note svg {
  width: 18px;
  height: 18px;
  flex: none;
}
.state-error p,
.state-note p {
  flex: 1;
  margin: 0;
  color: var(--c-ink);
}
.panel-pad {
  padding: 16px;
}
.panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
}
.rule-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
}
.rule-label {
  display: grid;
  flex: 1 1 200px;
  gap: 2px;
}
.rule-label strong {
  font-size: 14px;
  font-weight: 500;
}
.rule-label span {
  color: var(--c-muted);
  font-size: 12px;
}
.hint {
  margin-top: 10px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.inline-ok {
  color: var(--c-ok);
  font-size: 12px;
}
.inline-error {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  margin-top: 10px;
  color: var(--c-bad);
  font-size: 12.5px;
}
.inline-error button {
  color: inherit;
  text-decoration: underline;
}
.exception-list {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}
.exception-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  padding: 9px 0;
  border-bottom: 1px solid var(--c-line);
}
.exception-list li:last-child {
  border-bottom: 0;
}
.node-name {
  display: grid;
  min-width: 0;
  gap: 1px;
  font-size: 13.5px;
  font-weight: 500;
}
.node-name small {
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 400;
}
.event-list {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}
.event-list li {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto 138px;
  align-items: center;
  gap: 12px;
  padding: 11px 16px;
  border-bottom: 1px solid var(--c-line);
}
.event-list li:last-child {
  border-bottom: 0;
}
.event-mark {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-muted);
}
.event-mark.is-bad {
  background: var(--c-bad);
}
.event-mark.is-ok {
  background: var(--c-ok);
}
.event-main {
  display: grid;
  min-width: 0;
  gap: 1px;
}
.event-main strong {
  font-size: 13px;
  font-weight: 500;
}
.event-main p {
  margin: 0;
  color: var(--c-muted);
  font-size: 12px;
}
.event-list time {
  color: var(--c-muted);
  font-size: 11.5px;
  text-align: right;
}
.backup-lead {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}
.backup-icon {
  display: grid;
  flex: none;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-signal) 22%, var(--c-surface-2));
  color: var(--c-signal-text);
}
.backup-icon svg {
  width: 20px;
  height: 20px;
}
.backup-lead p {
  margin: 0;
  color: var(--c-muted);
  font-size: 13.5px;
  line-height: 1.7;
}
.mini-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin: 16px 0 0;
  padding: 12px 0 0;
  border-top: 1px solid var(--c-line);
}
.mini-stats dt {
  color: var(--c-muted);
  font-size: 11.5px;
}
.mini-stats dd {
  margin: 2px 0 0;
  font: 600 20px/1.2 var(--font-mono);
}
.mini-stats dd.small {
  color: var(--c-muted);
  font: 500 12.5px/1.4 var(--font-mono);
}
.config-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--c-line);
}
.config-row strong {
  font-size: 14px;
  font-weight: 500;
}
.config-row p {
  margin: 2px 0 0;
  color: var(--c-muted);
  font-size: 12.5px;
}
.dropzone {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 14px;
  padding: 14px;
  border: 1px dashed var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
  transition: border-color var(--duration-fast), background var(--duration-fast);
}
.dropzone.is-dragging {
  border-color: var(--c-signal);
  background: color-mix(in srgb, var(--c-signal) 10%, var(--c-surface-2));
}
.dropzone svg {
  width: 22px;
  height: 22px;
  flex: none;
  color: var(--c-muted);
}
.dropzone > div {
  display: grid;
  flex: 1;
  min-width: 0;
  gap: 2px;
}
.dropzone strong {
  overflow: hidden;
  font-size: 13.5px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.dropzone small {
  color: var(--c-muted);
  font-size: 12px;
}
.import-submit {
  margin-top: 12px;
}
.steps {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.steps li {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 14px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
}
.step-no {
  color: var(--c-signal-text);
  font: 600 12px/1 var(--font-mono);
}
.steps strong {
  font-size: 13.5px;
  font-weight: 500;
}
.steps p {
  margin: 3px 0 0;
  color: var(--c-muted);
  font-size: 12.5px;
  line-height: 1.6;
}
.data-table {
  display: grid;
  min-width: 0;
}
.table-head,
.table-row {
  display: grid;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
}
.table-head {
  border-bottom: 1px solid var(--c-line);
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11.5px;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.table-row {
  border-bottom: 1px solid var(--c-line);
  font-size: 13px;
}
.table-row:last-child {
  border-bottom: 0;
}
.table-row:hover {
  background: var(--c-surface-2);
}
.cell-main {
  display: grid;
  min-width: 0;
  gap: 2px;
}
.cell-main strong {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cell-main code {
  color: var(--c-muted);
  font: 12px/1.4 var(--font-mono);
}
.cell-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.cell-tags :deep(.ui-tag) {
  padding: 2px 7px;
  font-size: 11px;
  line-height: 1.4;
}
.cell-meta {
  color: var(--c-muted);
  font-size: 12px;
}
.is-off {
  opacity: 0.55;
}
.token-table .table-head,
.token-table .table-row {
  grid-template-columns:
    minmax(0, 1.3fr) minmax(120px, 0.9fr) 140px 120px 74px
    44px;
}
.audit-table .table-head,
.audit-table .table-row {
  grid-template-columns: 142px 54px 60px minmax(0, 1fr) 120px 118px;
}
.audit-main {
  display: grid;
  min-width: 0;
  gap: 1px;
}
.audit-main strong {
  overflow: hidden;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.audit-main small {
  overflow: hidden;
  color: var(--c-muted);
  font: 11.5px/1.4 var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--c-line);
}
.filter-bar :deep(.el-input) {
  max-width: 320px;
}
.status-filter {
  width: 120px;
}
.muted {
  color: var(--c-muted);
  font-size: 12px;
}
.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-top: 1px solid var(--c-line);
  color: var(--c-muted);
  font-size: 12px;
}
.pager div {
  display: flex;
  gap: 6px;
}
.links-table .table-head,
.links-table .table-row {
  grid-template-columns: minmax(0, 1fr) 150px 130px 132px;
}
.link-code {
  color: var(--c-signal-text);
  font: 500 12.5px/1.4 var(--font-mono);
}
.link-remark {
  color: var(--c-muted);
  font-size: 11.5px;
}
.link-target {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.link-actions {
  display: flex;
  justify-content: flex-end;
  gap: 2px;
}
.token-success {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 14px;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--c-ok) 40%, var(--c-line));
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-ok) 8%, var(--c-surface));
  color: var(--c-ok);
}
.token-success svg {
  width: 20px;
  height: 20px;
  flex: none;
}
.token-success p {
  margin: 0;
  color: var(--c-ink);
  font-size: 13px;
  line-height: 1.6;
}
.field-error {
  color: var(--c-bad);
  font-size: 12.5px;
  margin-top: 4px;
}
.dialog-note {
  margin-top: 10px;
  color: var(--c-muted);
  font-size: 12px;
}
.empty-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 18px;
  min-height: 176px;
  padding: 22px 20px;
}
.empty-visual {
  display: grid;
  place-items: center;
  width: 56px;
  height: 56px;
  border-radius: 16px;
  background: color-mix(in srgb, var(--c-signal) 20%, var(--c-surface-2));
  color: var(--c-signal-text);
}
.empty-visual svg {
  width: 26px;
  height: 26px;
}
.empty-copy {
  min-width: 0;
}
.empty-copy h3 {
  margin: 0 0 4px;
  font: 500 17px/1.35 var(--font-ui);
}
.empty-copy p {
  max-width: 62ch;
  margin: 0;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.65;
}
.empty-actions {
  display: grid;
  justify-items: end;
  gap: 6px;
}
.empty-actions small {
  color: var(--c-muted);
  font-size: 11.5px;
}
.tips {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 0;
  list-style: none;
}
.tips li {
  position: relative;
  padding-left: 14px;
  color: var(--c-muted);
  font-size: 12.5px;
  line-height: 1.65;
}
.tips li::before {
  content: "";
  position: absolute;
  top: 0.62em;
  left: 0;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--c-line);
}
.code-block {
  margin: 0 0 12px;
  padding: 12px;
  overflow-x: auto;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font: 12px/1.7 var(--font-mono);
  white-space: pre-wrap;
  word-break: break-all;
}
.empty-icon {
  display: grid;
  place-items: center;
  width: 40px;
  height: 40px;
  margin-bottom: 12px;
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-signal) 18%, var(--c-surface-2));
  color: var(--c-signal-text);
}
.empty-icon svg {
  width: 20px;
  height: 20px;
}
.probe-list {
  display: grid;
}
.probe-row {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto 44px;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--c-line);
}
.probe-row:last-child {
  border-bottom: 0;
}
.probe-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--c-muted);
}
.probe-dot.is-ok {
  background: var(--c-ok);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--c-ok) 18%, transparent);
}
.probe-dot.is-off {
  background: var(--c-bad);
}
.probe-main {
  display: grid;
  min-width: 0;
  gap: 2px;
}
.probe-main strong {
  font-size: 13.5px;
  font-weight: 500;
}
.probe-main small {
  color: var(--c-muted);
  font-size: 11.5px;
}
.probe-install {
  max-height: 280px;
  overflow: auto;
}
.aside-action {
  margin-top: 14px;
}
.exception-note {
  margin-top: 0;
}
.exception-search {
  margin-bottom: 10px;
}
.exception-picker {
  display: grid;
  max-height: 360px;
  margin: 0;
  padding: 0;
  overflow-y: auto;
  list-style: none;
}
.exception-picker li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 10px;
  padding: 9px 2px;
  border-bottom: 1px solid var(--c-line);
}
.exception-picker li:last-child {
  border-bottom: 0;
}
.pick-main {
  display: grid;
  min-width: 0;
  gap: 1px;
}
.pick-main strong {
  overflow: hidden;
  font-size: 13.5px;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.pick-main small {
  overflow: hidden;
  color: var(--c-muted);
  font: 11.5px/1.4 var(--font-mono);
  text-overflow: ellipsis;
  white-space: nowrap;
}
.exception-empty {
  display: grid;
  justify-items: start;
  gap: 10px;
  padding: 24px 0;
  color: var(--c-muted);
  font-size: 13px;
}
.exception-empty p {
  margin: 0;
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
  transform: translateY(12px);
  filter: blur(6px);
}
.pane-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-fast);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
@media (max-width: 1180px) {
  .token-table .table-head,
  .token-table .table-row {
    grid-template-columns: minmax(0, 1.2fr) 110px 120px 100px 66px 40px;
  }
  .audit-table .table-head,
  .audit-table .table-row {
    grid-template-columns: 132px 50px 56px minmax(0, 1fr) 100px 100px;
  }
  .event-list li {
    grid-template-columns: 10px minmax(0, 1fr) auto;
  }
  .event-list time {
    grid-column: 2 / -1;
    text-align: left;
  }
}
@media (max-width: 1180px) {
  .split {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 1023px) {
  .settings-layout {
    grid-template-columns: 1fr;
  }
  .settings-rail {
    position: static;
    display: flex;
    gap: 4px;
    padding: 6px;
    overflow-x: auto;
  }
  .rail-item {
    flex: 0 0 auto;
    grid-template-columns: 32px auto;
  }
  .rail-copy small,
  .rail-meta,
  .rail-foot {
    display: none;
  }
  .rail-item.is-active::before {
    top: auto;
    right: 8px;
    bottom: 0;
    left: 8px;
    width: auto;
    height: 2px;
  }
}
@media (max-width: 767px) {
  .two-col,
  .steps {
    grid-template-columns: 1fr;
  }
  .detect-hero {
    grid-template-columns: 1fr;
    gap: 16px;
    padding: 18px;
  }
  .detect-control {
    grid-auto-columns: auto;
    grid-auto-flow: column;
    justify-items: start;
    align-items: center;
  }
  .mini-stats {
    grid-template-columns: 1fr 1fr;
  }
  .empty-row {
    grid-template-columns: auto minmax(0, 1fr);
    min-height: 0;
    padding: 18px 0;
  }
  .empty-actions {
    grid-column: 1 / -1;
    justify-items: start;
  }
  .table-head {
    display: none;
  }
  .token-table .table-row {
    grid-template-columns: minmax(0, 1fr) auto auto;
    padding: 12px 16px;
  }
  .token-table .table-row .cell-tags,
  .token-table .table-row .cell-meta {
    display: none;
  }
  .audit-table .table-row {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      "main status"
      "time time";
    padding: 12px 16px;
  }
  .audit-table .col-time {
    grid-area: time;
  }
  .audit-table .col-status {
    grid-area: status;
    justify-self: end;
  }
  .audit-table .col-main {
    grid-area: main;
  }
  .audit-table .col-method,
  .audit-table .col-actor,
  .audit-table .col-ip {
    display: none;
  }
  .links-table .table-row {
    grid-template-columns: minmax(0, 1fr) auto;
    grid-template-areas:
      "main actions"
      "target target"
      "created created";
    padding: 12px 16px;
  }
  .links-table .cell-main {
    grid-area: main;
  }
  .links-table .link-target {
    grid-area: target;
  }
  .links-table .cell-meta {
    grid-area: created;
  }
  .links-table .link-actions {
    grid-area: actions;
  }
  .event-list li {
    grid-template-columns: 8px minmax(0, 1fr) auto;
    padding: 11px 12px;
  }
  .event-list time {
    grid-column: 2 / -1;
  }
  .config-row {
    align-items: flex-start;
    flex-direction: column;
  }
  .dropzone {
    align-items: flex-start;
    flex-wrap: wrap;
  }
}
</style>
