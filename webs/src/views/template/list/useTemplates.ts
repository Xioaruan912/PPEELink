import {
  ref,
  computed,
  onMounted,
  onBeforeUnmount,
  provide,
  inject,
  watch,
  type InjectionKey,
} from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getTemp,
  AddTemp,
  UpdateTemp,
  DelTemp,
  ValidateTemp,
  PreflightTemp,
  GetTempVersions,
  GetTempVersion,
  RollbackTemp,
} from "@/api/template/temp";
import {
  compareRegression,
  deleteRegressionCase,
  getRegressionCases,
  saveRegressionCase,
} from "@/api/regression";
import { startTemplateValidationTask } from "@/api/task";
export interface Temp {
  file: string;
  text: string;
  create_date: string;
}
export interface OutlineItem {
  key: string;
  line: number;
  level: number;
}
export const tempType = (file: string) =>
  /\.ya?ml$/i.test(file)
    ? "Clash / Mihomo"
    : /\.conf$/i.test(file)
      ? "Loon"
      : "通用";
export const fileSize = (text: string) => {
  const bytes = new Blob([text || ""]).size;
  return bytes < 1024
    ? `${bytes} B`
    : bytes < 1024 * 1024
      ? `${(bytes / 1024).toFixed(1)} KB`
      : `${(bytes / 1024 / 1024).toFixed(2)} MB`;
};
const errorMessage = (error: any) =>
  typeof error === "string" ? error : error?.message || "请求失败，请重试";
function createTemplates() {
  const tableData = ref<Temp[]>([]),
    searchKey = ref(""),
    loading = ref(false),
    loadError = ref("");
  const Tempoldname = ref(""),
    Tempname = ref(""),
    TempText = ref(""),
    TempTitle = ref("");
  const dialogVisible = ref(false),
    saving = ref(false),
    editorRef = ref<{ revealLine: (line: number) => void }>();
  const outline = ref<OutlineItem[]>([]),
    outlineExpanded = ref(false),
    validationErrors = ref<string[]>([]);
  const validating = ref(false),
    validationSignature = ref(""),
    validationError = ref("");
  const versionsVisible = ref(false),
    versions = ref<any[]>([]),
    versionLoading = ref(false),
    versionError = ref("");
  const latestVersions = ref<Record<string, number | null | undefined>>({});
  const versionFailures = ref<Record<string, boolean>>({});
  const checkedTemplates = ref<
    Record<string, { text: string; valid: boolean }>
  >({});
  const diffVisible = ref(false),
    diffLoading = ref(false),
    diffError = ref(""),
    oldVersionText = ref(""),
    oldVersionTitle = ref("");
  const preflightVisible = ref(false),
    preflightLoading = ref(false),
    preflightReport = ref<any>(null),
    preflightError = ref("");
  const preflightDomains = ref(
      "gemini.google.com\nchatgpt.com\nopenai.com\nclaude.ai"
    ),
    lastPreflightSignature = ref("");
  const baselineText = ref(""),
    inspectTab = ref<string | number>("checks");
  const regressionVisible = ref(false),
    regressionLoading = ref(false),
    regressionSaving = ref(false),
    regressionError = ref("");
  const regressionCases = ref<any[]>([]),
    regressionDiff = ref<any[]>([]),
    regressionResults = ref<any[]>([]);
  const emptyRegression = () => ({
    name: "",
    domain: "",
    expectedPolicy: "",
    expectedCountry: "",
    forbiddenPolicy: "DIRECT",
    protocol: "tcp",
    port: 443,
    enabled: true,
  });
  const regressionForm = ref<any>(emptyRegression());
  const filteredData = computed(() =>
    tableData.value.filter((row) =>
      row.file.toLowerCase().includes(searchKey.value.trim().toLowerCase())
    )
  );
  const visibleOutline = computed(() =>
    outlineExpanded.value
      ? outline.value
      : outline.value.filter((item) => item.level === 0)
  );
  const editorLanguage = computed(() =>
    /\.ya?ml$/i.test(Tempname.value) ? "yaml" : "ini"
  );
  const contentSignature = () =>
    `${Tempname.value.trim()}\u0000${TempText.value}`;
  const preflightSignature = () =>
    `${contentSignature()}\u0000${preflightDomains.value}`;
  const validationCurrent = computed(
    () =>
      !!validationSignature.value &&
      validationSignature.value === contentSignature()
  );
  const preflightCurrent = computed(
    () =>
      !!lastPreflightSignature.value &&
      lastPreflightSignature.value === preflightSignature()
  );
  const dirty = computed(
    () =>
      TempText.value !== baselineText.value ||
      (!!Tempoldname.value && Tempname.value !== Tempoldname.value)
  );
  let alive = true,
    generation = 0,
    listRequest = 0,
    validationRequest = 0,
    preflightRequest = 0,
    versionsRequest = 0,
    diffRequest = 0,
    regressionRequest = 0;
  onBeforeUnmount(() => {
    alive = false;
    generation++;
    listRequest++;
  });
  watch([Tempname, TempText], () => {
    validationSignature.value = "";
    outline.value = [];
    validationErrors.value = [];
    validationError.value = "";
  });
  watch(dialogVisible, (visible) => {
    if (!visible) {
      generation++;
      validating.value = false;
      preflightLoading.value = false;
    }
  });
  async function gettemps() {
    const request = ++listRequest;
    loading.value = true;
    loadError.value = "";
    try {
      const { data } = await getTemp();
      if (!alive || request !== listRequest) return;
      tableData.value = Array.isArray(data) ? data : [];
      latestVersions.value = {};
      versionFailures.value = {};
      // Bounded parallel reads: the list endpoint has no version metadata.
      const queue = [...tableData.value];
      await Promise.all(
        Array.from({ length: Math.min(4, queue.length) }, async () => {
          while (queue.length && alive && request === listRequest) {
            const row = queue.shift()!;
            try {
              const { data: history } = await GetTempVersions(row.file);
              if (alive && request === listRequest)
                latestVersions.value[row.file] = history?.[0]?.id ?? null;
            } catch {
              if (alive && request === listRequest)
                versionFailures.value[row.file] = true;
            }
          }
        })
      );
    } catch (error) {
      if (alive && request === listRequest)
        loadError.value = errorMessage(error);
    } finally {
      if (alive && request === listRequest) loading.value = false;
    }
  }
  function resetEditor() {
    generation++;
    validating.value = false;
    preflightLoading.value = false;
    saving.value = false;
    validationSignature.value = "";
    validationError.value = "";
    validationErrors.value = [];
    outline.value = [];
    outlineExpanded.value = false;
    preflightReport.value = null;
    lastPreflightSignature.value = "";
    preflightError.value = "";
    versions.value = [];
    versionError.value = "";
    inspectTab.value = "checks";
    regressionDiff.value = [];
    regressionResults.value = [];
    regressionError.value = "";
    regressionForm.value = emptyRegression();
  }
  function handleAddTemp() {
    resetEditor();
    TempTitle.value = "添加模板";
    Tempoldname.value = "";
    Tempname.value = "";
    TempText.value = "";
    baselineText.value = "";
    dialogVisible.value = true;
  }
  function handleEdit(row: Temp) {
    resetEditor();
    TempTitle.value = "编辑模板";
    Tempname.value = row.file;
    Tempoldname.value = row.file;
    TempText.value = row.text;
    baselineText.value = row.text;
    dialogVisible.value = true;
    void runValidation();
    void loadVersions();
  }
  async function closeEditor() {
    if (saving.value) return;
    if (dirty.value) {
      try {
        await ElMessageBox.confirm(
          "尚有未保存内容，确认退出编辑？",
          "退出编辑",
          {
            type: "warning",
            confirmButtonText: "放弃修改",
            cancelButtonText: "继续编辑",
          }
        );
      } catch {
        return;
      }
    }
    dialogVisible.value = false;
  }
  function loonOutline(text: string) {
    const items: OutlineItem[] = [];
    let inSection = false;
    text.split(/\r?\n/).forEach((raw, index) => {
      const line = raw.trim();
      if (!line || line.startsWith("#") || line.startsWith(";")) return;
      const section = line.match(/^\[([^\]]+)\]$/);
      if (section) {
        items.push({
          key: `[${section[1].trim()}]`,
          line: index + 1,
          level: 0,
        });
        inSection = true;
        return;
      }
      if (!inSection) return;
      const eq = line.indexOf("="),
        parts = line.split(",").map((value) => value.trim());
      const key =
        eq > 0
          ? line.slice(0, eq).trim()
          : parts.length >= 2 && parts[0] && parts[1]
            ? `${parts[0]} · ${parts[1]}`
            : line.split(/\s+/)[0] || "";
      if (key)
        items.push({
          key: key.length > 72 ? `${key.slice(0, 69)}...` : key,
          line: index + 1,
          level: 1,
        });
    });
    return items;
  }
  async function runValidation() {
    const request = ++validationRequest,
      session = generation,
      signature = contentSignature();
    const filename = Tempname.value,
      text = TempText.value;
    validating.value = true;
    validationError.value = "";
    try {
      const { data } = await ValidateTemp({ filename, text });
      if (
        !alive ||
        request !== validationRequest ||
        session !== generation ||
        signature !== contentSignature()
      )
        return;
      outline.value = /\.conf$/i.test(filename)
        ? loonOutline(text)
        : data?.outline || [];
      validationErrors.value = data?.errors || [];
      validationSignature.value = signature;
      checkedTemplates.value[filename] = {
        text,
        valid: data?.valid !== false && !validationErrors.value.length,
      };
      if (!validationErrors.value.length) ElMessage.success("模板语法校验通过");
    } catch (error) {
      if (alive && session === generation && request === validationRequest)
        validationError.value = errorMessage(error);
    } finally {
      if (alive && session === generation && request === validationRequest)
        validating.value = false;
    }
  }
  async function runPreflight(show = true) {
    if (!Tempname.value.trim() || !TempText.value.trim()) {
      ElMessage.warning("请先填写模板文件名和内容");
      return false;
    }
    const signature = preflightSignature(),
      session = generation;
    if (
      !show &&
      signature === lastPreflightSignature.value &&
      preflightReport.value?.valid
    )
      return true;
    const request = ++preflightRequest;
    if (show) preflightVisible.value = true;
    preflightLoading.value = true;
    preflightError.value = "";
    try {
      const { data } = await PreflightTemp({
        filename: Tempname.value.trim(),
        text: TempText.value,
        domains: preflightDomains.value,
      });
      if (
        !alive ||
        session !== generation ||
        request !== preflightRequest ||
        signature !== preflightSignature()
      )
        return false;
      preflightReport.value = data;
      lastPreflightSignature.value = signature;
      if (data?.valid && show)
        ElMessage.success(
          data?.summary?.warnings ? "预检通过，请确认警告" : "发布前预检通过"
        );
      return !!data?.valid;
    } catch (error) {
      if (alive && session === generation && request === preflightRequest)
        preflightError.value = errorMessage(error);
      return false;
    } finally {
      if (alive && session === generation && request === preflightRequest)
        preflightLoading.value = false;
    }
  }
  async function addtemp() {
    if (saving.value) return;
    if (!Tempname.value.trim()) {
      ElMessage.warning("请填写文件名");
      return;
    }
    const signature = preflightSignature(),
      session = generation;
    saving.value = true;
    try {
      const canSave = await runPreflight(false);
      if (!alive || session !== generation) return;
      if (!canSave || signature !== preflightSignature()) {
        preflightVisible.value = true;
        ElMessage.warning("预检未通过或内容已改变，请检查后保存");
        return;
      }
      if (TempTitle.value === "添加模板")
        await AddTemp({
          filename: Tempname.value.trim(),
          text: TempText.value.trim(),
        });
      else
        await UpdateTemp({
          filename: Tempname.value.trim(),
          oldname: Tempoldname.value.trim(),
          text: TempText.value.trim(),
        });
      ElMessage.success(
        TempTitle.value === "添加模板" ? "添加成功" : "更新成功"
      );
      dialogVisible.value = false;
      void gettemps();
    } catch (error) {
      ElMessage.error(errorMessage(error));
    } finally {
      if (alive && session === generation) saving.value = false;
    }
  }
  async function loadVersions() {
    if (!Tempoldname.value) return;
    const request = ++versionsRequest,
      filename = Tempoldname.value,
      session = generation;
    versionLoading.value = true;
    versionError.value = "";
    try {
      const { data } = await GetTempVersions(filename);
      if (!alive || session !== generation || request !== versionsRequest)
        return;
      versions.value = data || [];
      latestVersions.value[filename] = versions.value[0]?.id ?? null;
    } catch (error) {
      if (alive && session === generation && request === versionsRequest)
        versionError.value = errorMessage(error);
    } finally {
      if (alive && session === generation && request === versionsRequest)
        versionLoading.value = false;
    }
  }
  function openVersions() {
    inspectTab.value = "versions";
    versionsVisible.value = true;
    void loadVersions();
  }
  async function compareVersion(item: any) {
    const request = ++diffRequest,
      session = generation;
    diffVisible.value = true;
    diffLoading.value = true;
    diffError.value = "";
    oldVersionText.value = "";
    oldVersionTitle.value = `版本 #${item.id} · ${item.action}`;
    try {
      const { data } = await GetTempVersion(item.id);
      if (alive && session === generation && request === diffRequest)
        oldVersionText.value = data?.content || "";
    } catch (error) {
      if (alive && session === generation && request === diffRequest)
        diffError.value = errorMessage(error);
    } finally {
      if (alive && session === generation && request === diffRequest)
        diffLoading.value = false;
    }
  }
  async function rollbackVersion(item: any) {
    try {
      await ElMessageBox.confirm(
        `确定回滚到版本 #${item.id}？已保存版本会保留为历史；未保存的编辑将丢弃。`,
        "模板回滚",
        {
          type: "warning",
          confirmButtonText: "确认回滚",
          cancelButtonText: "取消",
          confirmButtonClass: "el-button--danger",
        }
      );
    } catch {
      return;
    }
    try {
      await RollbackTemp({ id: item.id });
      ElMessage.success("回滚成功");
      versionsVisible.value = false;
      diffVisible.value = false;
      dialogVisible.value = false;
      void gettemps();
    } catch (error) {
      ElMessage.error(errorMessage(error));
    }
  }
  async function handleDel(row: Temp) {
    try {
      await ElMessageBox.confirm(`确定删除 ${row.file}？`, "删除模板", {
        type: "warning",
        confirmButtonText: "删除模板",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      });
    } catch {
      return;
    }
    try {
      await DelTemp({ filename: row.file });
      ElMessage.success("删除成功");
      await gettemps();
    } catch (error) {
      ElMessage.error(errorMessage(error));
    }
  }
  async function copyText(row: Temp) {
    try {
      await navigator.clipboard.writeText(row.text);
      ElMessage.success("内容已复制");
    } catch {
      ElMessage.warning("复制失败");
    }
  }
  function revealIssue(line?: number) {
    if (!line) return;
    preflightVisible.value = false;
    editorRef.value?.revealLine(line);
  }
  async function openRegression() {
    regressionVisible.value = true;
    regressionLoading.value = true;
    regressionError.value = "";
    regressionDiff.value = [];
    regressionResults.value = [];
    const request = ++regressionRequest,
      session = generation;
    const filename = Tempname.value,
      before = baselineText.value,
      after = TempText.value;
    try {
      const { data } = await getRegressionCases();
      if (!alive || request !== regressionRequest || session !== generation)
        return;
      regressionCases.value = data || [];
      if (before && after) {
        const { data: comparison } = await compareRegression({
          filename,
          before,
          after,
        });
        if (
          !alive ||
          request !== regressionRequest ||
          session !== generation ||
          after !== TempText.value
        )
          return;
        regressionDiff.value = comparison?.diff || [];
        regressionResults.value = comparison?.after || [];
      }
    } catch (error) {
      if (alive && session === generation && request === regressionRequest)
        regressionError.value = errorMessage(error);
    } finally {
      if (alive && session === generation && request === regressionRequest)
        regressionLoading.value = false;
    }
  }
  async function persistRegression() {
    if (
      !regressionForm.value.name.trim() ||
      !regressionForm.value.domain.trim()
    ) {
      ElMessage.warning("请填写名称和域名");
      return;
    }
    if (
      !regressionForm.value.port ||
      regressionForm.value.port < 1 ||
      regressionForm.value.port > 65535
    ) {
      ElMessage.warning("端口范围为 1–65535");
      return;
    }
    regressionSaving.value = true;
    try {
      await saveRegressionCase({
        ...regressionForm.value,
        name: regressionForm.value.name.trim(),
        domain: regressionForm.value.domain.trim(),
      });
      regressionForm.value = emptyRegression();
      await openRegression();
    } catch (error) {
      regressionError.value = errorMessage(error);
    } finally {
      regressionSaving.value = false;
    }
  }
  async function removeRegression(row: any) {
    try {
      await ElMessageBox.confirm(`删除回归用例「${row.name}」？`, "删除用例", {
        type: "warning",
        confirmButtonText: "删除",
        cancelButtonText: "取消",
        confirmButtonClass: "el-button--danger",
      });
    } catch {
      return;
    }
    try {
      await deleteRegressionCase(row.id);
      await openRegression();
    } catch (error) {
      regressionError.value = errorMessage(error);
    }
  }
  function editRegression(row: any) {
    regressionForm.value = { ...row };
  }
  async function queueTemplateValidation() {
    if (!Tempoldname.value) {
      ElMessage.info("请先保存模板，再加入后台验证任务");
      return;
    }
    try {
      const { data } = await startTemplateValidationTask(Tempoldname.value);
      ElMessage.success(`模板验证任务已创建 #${data?.taskId || ""}`);
    } catch (error) {
      ElMessage.error(errorMessage(error));
    }
  }
  onMounted(gettemps);
  return {
    tableData,
    searchKey,
    loading,
    loadError,
    Tempoldname,
    Tempname,
    TempText,
    TempTitle,
    dialogVisible,
    saving,
    editorRef,
    outline,
    visibleOutline,
    outlineExpanded,
    validationErrors,
    validating,
    validationCurrent,
    validationError,
    checkedTemplates,
    versionsVisible,
    versions,
    versionLoading,
    versionError,
    latestVersions,
    versionFailures,
    diffVisible,
    diffLoading,
    diffError,
    oldVersionText,
    oldVersionTitle,
    preflightVisible,
    preflightLoading,
    preflightReport,
    preflightDomains,
    preflightError,
    preflightCurrent,
    inspectTab,
    dirty,
    regressionVisible,
    regressionLoading,
    regressionSaving,
    regressionError,
    regressionCases,
    regressionDiff,
    regressionResults,
    regressionForm,
    filteredData,
    editorLanguage,
    gettemps,
    handleAddTemp,
    handleEdit,
    closeEditor,
    addtemp,
    runValidation,
    runPreflight,
    loadVersions,
    openVersions,
    compareVersion,
    rollbackVersion,
    handleDel,
    copyText,
    revealIssue,
    openRegression,
    persistRegression,
    removeRegression,
    editRegression,
    queueTemplateValidation,
  };
}
const key: InjectionKey<ReturnType<typeof createTemplates>> =
  Symbol("templates");
export function provideTemplates() {
  const state = createTemplates();
  provide(key, state);
  return state;
}
export function useTemplates() {
  const state = inject(key);
  if (!state) throw new Error("模板工作区尚未初始化");
  return state;
}
