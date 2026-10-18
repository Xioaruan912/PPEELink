import {
  computed,
  inject,
  onBeforeUnmount,
  onMounted,
  provide,
  reactive,
  ref,
} from "vue";
import { ElMessage } from "element-plus";
import {
  getRuleSources,
  getRuleCatalog,
  getRulePreview,
  getRuleTemplateGroups,
  syncRuleCatalog,
  applyRulesToTemplate,
} from "@/api/rules";
import { getTemp } from "@/api/template/temp";
import { formatDateTime } from "@/utils/date";

export type RuleItem = {
  externalId: string;
  sourceKey: string;
  name: string;
  category: string;
  platform: string;
  format: string;
  ruleCount: number;
  checksum?: string;
  remoteUpdate?: string;
  metadataJson?: string;
};
export type NormalizedRule = {
  type: string;
  value: string;
  options?: string[];
};
export const ruleText = (rule: NormalizedRule) =>
  [rule.type, rule.value, ...(rule.options || [])].join(",");
export const sourceLabel = (key: string) =>
  (
    ({
      shunt_rules: "ShuntRules",
      ios_rule_script: "ios_rule_script",
    }) as Record<string, string>
  )[key] || key;
export const fmtTime = (value?: string) => formatDateTime(value) || "尚未同步";
export function metadataSummary(row: RuleItem) {
  try {
    return Object.entries(JSON.parse(row.metadataJson || "{}"))
      .slice(0, 3)
      .map(([key, value]) => `${key} ${String(value)}`)
      .join(" · ");
  } catch {
    return "";
  }
}
const message = (error: unknown) =>
  typeof error === "string"
    ? error
    : error instanceof Error
      ? error.message
      : "未知错误";
function createRules() {
  const loading = ref(true),
    sourcesLoading = ref(true),
    syncing = ref(false),
    errorText = ref(""),
    sourcesError = ref("");
  const rules = ref<RuleItem[]>([]),
    sources = ref<any[]>([]),
    total = ref(0);
  const loadingMore = ref(false),
    loadMoreError = ref("");
  const catalogPage = ref(0),
    catalogExhausted = ref(false);
  const pageSize = 24;
  const hasMore = computed(
    () => !catalogExhausted.value && rules.value.length < total.value
  );
  let catalogFilters = {
    source: "",
    platform: "Clash",
    category: "",
    keyword: "",
  };
  const keyword = ref(""),
    source = ref(""),
    platform = ref("Clash"),
    category = ref("");
  const categories = [
    "AI",
    "流媒体",
    "社交",
    "Apple",
    "Google",
    "Microsoft",
    "开发者",
    "广告/隐私",
    "其他",
  ];
  const selectedIds = ref<string[]>([]);
  const selectedRules = computed(() =>
    rules.value.filter((row) => selectedIds.value.includes(row.externalId))
  );
  const focusedRule = ref<RuleItem | null>(null),
    detailOpen = ref(false);
  const previewLoading = ref(false),
    preview = ref<any>(null);
  let alive = true;
  let importRequest = 0;
  let previewRequest = 0,
    catalogRequest = 0;
  const confirmation = reactive({
    visible: false,
    title: "",
    message: "",
    action: "",
  });
  let answer: ((accepted: boolean) => void) | undefined;
  function resolveConfirmation(accepted: boolean) {
    confirmation.visible = false;
    answer?.(accepted);
    answer = undefined;
  }
  function confirm(title: string, text: string, action: string) {
    answer?.(false);
    Object.assign(confirmation, {
      visible: true,
      title,
      message: text,
      action,
    });
    return new Promise<boolean>((resolve) => {
      answer = resolve;
    });
  }
  onBeforeUnmount(() => {
    alive = false;
    importRequest++;
    groupsRequest++;
    answer?.(false);
    previewRequest++;
    catalogRequest++;
  });
  async function loadSources() {
    sourcesLoading.value = true;
    sourcesError.value = "";
    try {
      const res = await getRuleSources();
      if (alive) sources.value = Array.isArray(res?.data) ? res.data : [];
    } catch (error) {
      if (!alive) return;
      sourcesError.value = `规则来源读取失败：${message(error)}`;
    } finally {
      if (alive) sourcesLoading.value = false;
    }
  }
  async function loadCatalog() {
    if (!alive) return;
    const request = ++catalogRequest;
    catalogFilters = {
      source: source.value,
      platform: platform.value,
      category: category.value,
      keyword: keyword.value,
    };
    loading.value = true;
    loadingMore.value = false;
    errorText.value = "";
    loadMoreError.value = "";
    catalogPage.value = 0;
    catalogExhausted.value = false;
    rules.value = [];
    total.value = 0;
    try {
      const res = await getRuleCatalog({
        ...catalogFilters,
        page: 1,
        pageSize,
      });
      if (request !== catalogRequest) return;
      rules.value = Array.isArray(res?.data?.items)
        ? res.data.items.filter((row: RuleItem) => row?.externalId)
        : [];
      total.value = Number(res?.data?.total || 0);
      catalogPage.value = 1;
      catalogExhausted.value = !rules.value.length;
      selectedIds.value = selectedIds.value.filter((id) =>
        rules.value.some((row) => row.externalId === id)
      );
    } catch (error) {
      if (request === catalogRequest) {
        errorText.value = `规则目录加载失败：${message(error)}`;
      }
    } finally {
      if (request === catalogRequest) loading.value = false;
    }
  }
  async function loadMoreCatalog() {
    if (!alive || loading.value || loadingMore.value || !hasMore.value) return;
    const request = catalogRequest,
      nextPage = catalogPage.value + 1;
    loadingMore.value = true;
    loadMoreError.value = "";
    try {
      const res = await getRuleCatalog({
        ...catalogFilters,
        page: nextPage,
        pageSize,
      });
      if (!alive || request !== catalogRequest) return;
      const items: RuleItem[] = Array.isArray(res?.data?.items)
        ? res.data.items.filter((row: RuleItem) => row?.externalId)
        : [];
      const known = new Set(rules.value.map((row) => row.externalId));
      const next = items.filter((row) => {
        if (known.has(row.externalId)) return false;
        known.add(row.externalId);
        return true;
      });
      rules.value = [...rules.value, ...next];
      total.value = Number(res?.data?.total || 0);
      catalogPage.value = nextPage;
      catalogExhausted.value = !next.length;
    } catch (error) {
      if (alive && request === catalogRequest)
        loadMoreError.value = `加载更多失败：${message(error)}`;
    } finally {
      if (alive && request === catalogRequest) loadingMore.value = false;
    }
  }
  function resetFilters() {
    keyword.value = "";
    source.value = "";
    category.value = "";
    platform.value = "Clash";
    void loadCatalog();
  }
  function toggleCategory(value: string) {
    category.value = category.value === value ? "" : value;
    void loadCatalog();
  }
  function toggleSelected(id: string) {
    selectedIds.value = selectedIds.value.includes(id)
      ? selectedIds.value.filter((value) => value !== id)
      : [...selectedIds.value, id];
  }
  async function doSync() {
    syncing.value = true;
    errorText.value = "";
    try {
      await syncRuleCatalog();
      await Promise.all([loadSources(), loadCatalog()]);
      ElMessage.success("规则目录已同步");
    } catch (error) {
      errorText.value = `同步失败：${message(error)}`;
    } finally {
      syncing.value = false;
    }
  }
  function focus(row: RuleItem) {
    if (focusedRule.value?.externalId !== row.externalId) {
      previewRequest++;
      preview.value = null;
      previewLoading.value = false;
    }
    focusedRule.value = row;
  }
  async function openPreview(row: RuleItem, expanded = detailOpen.value) {
    focus(row);
    detailOpen.value = expanded;
    const request = ++previewRequest;
    previewLoading.value = true;
    preview.value = null;
    try {
      const res = await getRulePreview(row.externalId);
      if (request === previewRequest) preview.value = res?.data || null;
    } catch (error) {
      if (request === previewRequest) preview.value = { error: message(error) };
    } finally {
      if (request === previewRequest) previewLoading.value = false;
    }
  }
  const importOpen = ref(false),
    importLoading = ref(false),
    templateLoading = ref(false),
    groupsLoading = ref(false),
    importError = ref("");
  const importItems = ref<RuleItem[]>([]),
    templates = ref<{ file: string }[]>([]),
    proxyGroups = ref<string[]>([]);
  const policyGroups = computed(() => [
    ...new Set([...proxyGroups.value, "DIRECT"]),
  ]);
  const proxyOptions = computed(() => [
    ...new Set(["DIRECT", ...proxyGroups.value]),
  ]);
  const importForm = reactive({
    template: "",
    policy: "",
    proxy: "",
    conflictPolicy: "keep",
  });
  let groupsRequest = 0;
  async function loadTemplateGroups() {
    if (!alive) return;
    const request = ++groupsRequest;
    proxyGroups.value = [];
    importForm.proxy = "";
    importForm.policy = "";
    importError.value = "";
    if (!importForm.template) return;
    groupsLoading.value = true;
    try {
      const res = await getRuleTemplateGroups(importForm.template);
      if (request === groupsRequest) {
        proxyGroups.value = Array.isArray(res?.data) ? res.data : [];
        importForm.policy = proxyGroups.value[0] || "DIRECT";
      }
    } catch (error) {
      if (request === groupsRequest)
        importError.value = `读取模板节点组失败：${message(error)}`;
    } finally {
      if (request === groupsRequest) groupsLoading.value = false;
    }
  }
  async function openImport(items: RuleItem[]) {
    if (!items.length) return;
    if (items.some((row) => row.platform !== "Clash")) {
      errorText.value =
        "V1 当前仅支持将 Clash 规则写入模板；Surge / Loon 可浏览和预览。";
      ElMessage.warning(errorText.value);
      return;
    }
    const request = ++importRequest;
    importItems.value = [...items];
    importOpen.value = true;
    templateLoading.value = true;
    importError.value = "";
    try {
      const res = await getTemp();
      if (!alive || request !== importRequest) return;
      templates.value = Array.isArray(res?.data)
        ? res.data.filter((row: { file: string }) => /\.ya?ml$/i.test(row.file))
        : [];
      if (!templates.value.some((row) => row.file === importForm.template))
        importForm.template = templates.value[0]?.file || "";
      await loadTemplateGroups();
    } catch (error) {
      if (!alive || request !== importRequest) return;
      templates.value = [];
      importError.value = `模板读取失败：${message(error)}`;
    } finally {
      if (alive && request === importRequest) templateLoading.value = false;
    }
  }
  async function submitImport() {
    if (
      !importForm.template ||
      !importForm.policy.trim() ||
      importLoading.value
    )
      return;
    if (
      !(await confirm(
        "写入目标模板",
        `将 ${importItems.value.length} 项规则导入「${importForm.template}」，使用策略「${importForm.policy}」。写入前会创建模板版本。`,
        "确认导入"
      ))
    )
      return;
    importLoading.value = true;
    importError.value = "";
    try {
      const res: any = await applyRulesToTemplate({
        ruleIds: importItems.value.map((row) => row.externalId),
        ...importForm,
        mode: "provider",
        position: "before-match",
      });
      const count = Array.isArray(res?.data?.results)
        ? res.data.results.length
        : importItems.value.length;
      ElMessage.success(
        `${res?.msg || "规则导入完成"}：${count} 条规则已写入 ${importForm.template}`
      );
      const warnings = Array.isArray(res?.data?.warnings)
        ? res.data.warnings
        : [];
      if (warnings.length)
        ElMessage.warning(
          `导入完成，但有 ${warnings.length} 条警告：${warnings.join("；")}`
        );
      importOpen.value = false;
      selectedIds.value = [];
    } catch (error) {
      importError.value = `导入失败：${message(error)}`;
    } finally {
      importLoading.value = false;
    }
  }
  onMounted(() => {
    void Promise.all([loadSources(), loadCatalog()]);
  });
  return {
    loading,
    sourcesLoading,
    syncing,
    errorText,
    sourcesError,
    rules,
    sources,
    total,
    keyword,
    source,
    platform,
    category,
    categories,
    selectedIds,
    selectedRules,
    focusedRule,
    detailOpen,
    previewLoading,
    preview,
    confirmation,
    resolveConfirmation,
    loadSources,
    loadCatalog,
    loadMoreCatalog,
    loadingMore,
    loadMoreError,
    hasMore,
    resetFilters,
    toggleCategory,
    toggleSelected,
    doSync,
    openPreview,
    importOpen,
    importLoading,
    templateLoading,
    groupsLoading,
    importError,
    importItems,
    templates,
    policyGroups,
    proxyOptions,
    importForm,
    loadTemplateGroups,
    openImport,
    submitImport,
  };
}
type RulesContext = ReturnType<typeof createRules>;
const contextKey: InjectionKey<RulesContext> = Symbol("rules");
export function provideRules() {
  const context = createRules();
  provide(contextKey, context);
  return context;
}
export function useRules() {
  const context = inject(contextKey);
  if (!context) throw new Error("Rules context missing");
  return context;
}
