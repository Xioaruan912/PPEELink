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
} from "@/api/template/temp";
export interface Temp {
  file: string;
  text: string;
  create_date: string;
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
    baselineText = ref("");
  const validating = ref(false),
    validationErrors = ref<string[]>([]),
    validationError = ref(""),
    validationSignature = ref("");
  const filteredData = computed(() =>
    tableData.value.filter((row) =>
      row.file.toLowerCase().includes(searchKey.value.trim().toLowerCase())
    )
  );
  const editorLanguage = computed(() =>
    /\.ya?ml$/i.test(Tempname.value) ? "yaml" : "ini"
  );
  const signature = () => `${Tempname.value.trim()}\u0000${TempText.value}`;
  const validationCurrent = computed(
    () =>
      !!validationSignature.value && validationSignature.value === signature()
  );
  const dirty = computed(
    () =>
      TempText.value !== baselineText.value ||
      (!!Tempoldname.value && Tempname.value !== Tempoldname.value)
  );
  // Late responses from a closed or reopened editor must not overwrite state.
  let alive = true,
    generation = 0,
    listRequest = 0;
  onBeforeUnmount(() => {
    alive = false;
    generation++;
  });
  watch([Tempname, TempText], () => {
    validationSignature.value = "";
    validationErrors.value = [];
    validationError.value = "";
  });
  async function gettemps() {
    const request = ++listRequest;
    loading.value = true;
    loadError.value = "";
    try {
      const { data } = await getTemp();
      if (alive && request === listRequest)
        tableData.value = Array.isArray(data) ? data : [];
    } catch (error) {
      if (alive && request === listRequest)
        loadError.value = errorMessage(error);
    } finally {
      if (alive && request === listRequest) loading.value = false;
    }
  }
  function open(title: string, file: string, text: string) {
    generation++;
    validating.value = false;
    saving.value = false;
    validationSignature.value = "";
    validationErrors.value = [];
    validationError.value = "";
    TempTitle.value = title;
    Tempoldname.value = file;
    Tempname.value = file;
    TempText.value = text;
    baselineText.value = text;
    dialogVisible.value = true;
  }
  const handleAddTemp = () => open("添加模板", "", "");
  function handleEdit(row: Temp) {
    open("编辑模板", row.file, row.text);
    void runValidation(true);
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
    generation++;
    dialogVisible.value = false;
  }
  /** Server-side syntax check; returns whether the current content is valid. */
  async function runValidation(silent = false) {
    const session = generation,
      sig = signature();
    validating.value = true;
    validationError.value = "";
    try {
      const { data } = await ValidateTemp({
        filename: Tempname.value,
        text: TempText.value,
      });
      if (!alive || session !== generation || sig !== signature()) return false;
      validationErrors.value = data?.errors || [];
      validationSignature.value = sig;
      const valid = data?.valid !== false && !validationErrors.value.length;
      if (valid && !silent) ElMessage.success("模板语法校验通过");
      return valid;
    } catch (error) {
      if (alive && session === generation)
        validationError.value = errorMessage(error);
      return false;
    } finally {
      if (alive && session === generation) validating.value = false;
    }
  }
  async function addtemp() {
    if (saving.value) return;
    if (!Tempname.value.trim()) {
      ElMessage.warning("请填写文件名");
      return;
    }
    const session = generation;
    saving.value = true;
    try {
      if (!(await runValidation(true))) {
        if (alive && session === generation)
          ElMessage.warning("语法校验未通过，请修正后保存");
        return;
      }
      const payload = {
        filename: Tempname.value.trim(),
        text: TempText.value.trim(),
      };
      if (TempTitle.value === "添加模板") await AddTemp(payload);
      else await UpdateTemp({ ...payload, oldname: Tempoldname.value.trim() });
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
  onMounted(gettemps);
  return {
    tableData,
    filteredData,
    searchKey,
    loading,
    loadError,
    Tempname,
    TempText,
    TempTitle,
    dialogVisible,
    saving,
    dirty,
    editorLanguage,
    validating,
    validationErrors,
    validationError,
    validationCurrent,
    gettemps,
    handleAddTemp,
    handleEdit,
    closeEditor,
    addtemp,
    runValidation,
    handleDel,
    copyText,
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
