import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  getAirports,
  getAirportDetail,
  AddAirport,
  UpdateAirport,
  DelAirport,
  SyncAirport,
  selectAirportNodes,
} from "@/api/subcription/airport";
import { getNodeOverview } from "@/api/subcription/node";
import { getTasks } from "@/api/task";
import { testLocalAll } from "@/utils/ping";
import { formatDateTime } from "@/utils/date";
import { readUnlockResponse } from "./unlockResponse";

export interface Airport {
  ID: number;
  Name: string;
  URL: string;
  AutoCleanup: boolean;
  IsDedicated: boolean;
  NodeCount: number;
  LastSync: string | null;
  TrafficUpload?: number;
  TrafficDownload?: number;
  TrafficTotal?: number;
  TrafficExpire?: string | null;
  TrafficUpdatedAt?: string | null;
}
export interface AirportNode {
  ID: number;
  Name: string;
  Link: string;
}
export interface AirportDetail {
  id: number;
  name: string;
  url: string;
  auto_cleanup: boolean;
  is_dedicated: boolean;
  node_count: number;
  last_sync: string | null;
  nodes: AirportNode[];
  selected_nodes: string[];
}
interface SyncState {
  active: boolean;
  taskId?: number;
  progress?: number;
  message: string;
  baseline: string | null;
  started: number;
  failed?: boolean;
  unknown?: boolean;
}
const messageOf = (error: any, fallback: string) =>
  error?.response?.data?.msg || error?.message || fallback;
export const displayTime = (value: string | null) =>
  formatDateTime(value) || "从未同步";
export function maskSource(value: string) {
  try {
    const url = new URL(value);
    return `${url.protocol}//${url.host}/••••••••`;
  } catch {
    return "••••••••";
  }
}

export function useAirports() {
  const tableData = ref<Airport[]>([]),
    loading = ref(true),
    loadError = ref("");
  const search = ref("");
  const filtered = computed(() =>
    tableData.value.filter((a) =>
      a.Name.toLowerCase().includes(search.value.trim().toLowerCase())
    )
  );
  const nodeCount = computed(() =>
    tableData.value.reduce((sum, a) => sum + (a.NodeCount || 0), 0)
  );
  let disposed = false,
    listGeneration = 0;
  const syncStates = ref<Record<number, SyncState>>({});
  let pollTimer: ReturnType<typeof setTimeout> | undefined;
  const activeSyncCount = computed(
    () => Object.values(syncStates.value).filter((s) => s.active).length
  );
  async function loadData() {
    const generation = ++listGeneration;
    loading.value = true;
    loadError.value = "";
    try {
      const { data } = await getAirports();
      if (!disposed && generation === listGeneration)
        tableData.value = data || [];
    } catch (error) {
      if (!disposed && generation === listGeneration)
        loadError.value = messageOf(error, "请检查网络后重试");
    } finally {
      if (!disposed && generation === listGeneration) loading.value = false;
    }
  }
  function schedulePoll() {
    if (disposed || pollTimer || !activeSyncCount.value) return;
    pollTimer = setTimeout(() => {
      pollTimer = undefined;
      void pollSync();
    }, 2500);
  }
  async function pollSync() {
    const active = Object.entries(syncStates.value).filter(([, s]) => s.active);
    if (!active.length || disposed) return;
    const listVersion = listGeneration;
    const [airports, tasks] = await Promise.allSettled([
      getAirports(),
      active.some(([, s]) => s.taskId)
        ? getTasks(300)
        : Promise.resolve({ data: [] }),
    ]);
    if (disposed) return;
    const rows: Airport[] | undefined =
      airports.status === "fulfilled" ? airports.value.data || [] : undefined;
    if (rows && listVersion === listGeneration) tableData.value = rows;
    const taskRows: any[] =
      tasks.status === "fulfilled" ? tasks.value.data || [] : [];
    for (const [id, snapshot] of active) {
      const state = syncStates.value[Number(id)];
      if (state !== snapshot || !state.active) continue;
      const row = rows?.find((a) => a.ID === Number(id));
      const task = state.taskId
        ? taskRows.find((t) => t.id === state.taskId)
        : undefined;
      if (task) {
        state.message = task.message || "后台同步中";
        if (Number.isFinite(task.progress))
          state.progress = Math.max(0, Math.min(100, task.progress));
        if (["success", "failed", "cancelled"].includes(task.status)) {
          state.active = false;
          state.failed = task.status !== "success";
          state.message =
            task.error ||
            (task.status === "success"
              ? "同步完成"
              : task.status === "cancelled"
                ? "同步已取消"
                : "同步失败");
          if (state.failed) ElMessage.error(state.message);
          else {
            ElMessage.success(`${row?.Name || "机场"}已同步`);
            // The task and list requests race: fetch again if the list preceded the commit.
            if (!row || row.LastSync === state.baseline) void loadData();
          }
          continue;
        }
      } else if (row?.LastSync && row.LastSync !== state.baseline) {
        state.active = false;
        state.message = "同步时间已更新";
        continue;
      }
      if (Date.now() - state.started > 10 * 60 * 1000) {
        state.active = false;
        state.unknown = true;
        state.message = "暂未确认结果，请刷新或查看任务中心";
      } else if (
        airports.status === "rejected" ||
        (state.taskId && tasks.status === "rejected")
      ) {
        state.message = "连接中断，正在重试查询";
      } else if (!task && state.taskId)
        state.message = "等待任务状态，正在核对同步时间";
    }
    schedulePoll();
  }
  async function handleSync(row: Airport) {
    if (syncStates.value[row.ID]?.active) return;
    const state: SyncState = {
      active: true,
      message: "正在提交同步",
      baseline: row.LastSync,
      started: Date.now(),
    };
    syncStates.value[row.ID] = state;
    try {
      const { data } = await SyncAirport({ id: row.ID });
      if (disposed) return;
      syncStates.value[row.ID].taskId = data?.taskId;
      syncStates.value[row.ID].message = "后台测活与同步中";
      schedulePoll();
    } catch (error) {
      if (disposed) return;
      syncStates.value[row.ID] = {
        ...state,
        active: false,
        failed: true,
        message: messageOf(error, "同步请求失败"),
      };
      ElMessage.error(syncStates.value[row.ID].message);
    }
  }

  const editorVisible = ref(false),
    saving = ref(false),
    formError = ref("");
  const emptyForm = () => ({
    ID: 0,
    Name: "",
    URL: "",
    AutoCleanup: true,
    IsDedicated: false,
  });
  const form = ref(emptyForm());
  function handleAdd() {
    form.value = emptyForm();
    formError.value = "";
    editorVisible.value = true;
  }
  function handleEdit(row: Airport) {
    form.value = {
      ID: row.ID,
      Name: row.Name,
      URL: row.URL,
      AutoCleanup: row.AutoCleanup,
      IsDedicated: row.IsDedicated,
    };
    formError.value = "";
    editorVisible.value = true;
  }
  async function submitForm() {
    if (saving.value) return;
    if (!form.value.Name.trim() || !form.value.URL.trim()) {
      formError.value = "请填写机场名称和订阅链接";
      return;
    }
    saving.value = true;
    formError.value = "";
    try {
      const payload = {
        ...form.value,
        Name: form.value.Name.trim(),
        URL: form.value.URL.trim(),
      };
      await (payload.ID ? UpdateAirport(payload) : AddAirport(payload));
      if (disposed) return;
      editorVisible.value = false;
      ElMessage.success(payload.ID ? "机场已更新" : "机场已添加");
      await loadData();
    } catch (error) {
      if (!disposed) formError.value = messageOf(error, "保存失败，请重试");
    } finally {
      if (!disposed) saving.value = false;
    }
  }
  const deleteTarget = ref<Airport>(),
    deleting = ref(false),
    deleteError = ref("");
  function handleDel(row: Airport) {
    deleteTarget.value = row;
    deleteError.value = "";
  }
  async function confirmDelete(deleteNodes: boolean) {
    const row = deleteTarget.value;
    if (!row || deleting.value) return;
    deleting.value = true;
    deleteError.value = "";
    try {
      await DelAirport({ id: row.ID, delete_nodes: deleteNodes });
      if (disposed) return;
      delete syncStates.value[row.ID];
      deleteTarget.value = undefined;
      if (drawerDetail.value?.id === row.ID) drawerVisible.value = false;
      ElMessage.success(
        deleteNodes ? "机场及所属节点已删除" : "机场已删除，节点已保留"
      );
      await loadData();
    } catch (error) {
      if (!disposed) deleteError.value = messageOf(error, "删除失败，请重试");
    } finally {
      if (!disposed) deleting.value = false;
    }
  }

  const drawerVisible = ref(false),
    drawerLoading = ref(false),
    detailError = ref("");
  const drawerDetail = ref<AirportDetail>(),
    detailTarget = ref<number>();
  const selectedNodeNames = ref<string[]>([]),
    savingNodes = ref(false),
    nodeSaveError = ref("");
  const rttMap = ref<Record<string, number>>({}),
    rttLoading = ref(false),
    rttError = ref("");
  let detailGeneration = 0,
    rttController: AbortController | undefined;
  function stopDetailWork() {
    ++detailGeneration;
    rttController?.abort();
  }
  watch(drawerVisible, (visible) => {
    if (!visible) stopDetailWork();
  });
  async function openDetail(row: Pick<Airport, "ID">) {
    stopDetailWork();
    const generation = detailGeneration;
    drawerVisible.value = true;
    drawerLoading.value = true;
    drawerDetail.value = undefined;
    detailTarget.value = row.ID;
    detailError.value = "";
    nodeSaveError.value = "";
    selectedNodeNames.value = [];
    rttMap.value = {};
    try {
      const { data } = await getAirportDetail(row.ID);
      if (disposed || generation !== detailGeneration) return;
      drawerDetail.value = {
        ...data,
        nodes: data?.nodes || [],
        selected_nodes: data?.selected_nodes || [],
      };
      selectedNodeNames.value = [...(data?.selected_nodes || [])];
      void loadNodeRtt();
    } catch (error) {
      if (!disposed && generation === detailGeneration)
        detailError.value = messageOf(error, "获取机场详情失败");
    } finally {
      if (!disposed && generation === detailGeneration)
        drawerLoading.value = false;
    }
  }
  async function loadNodeRtt() {
    rttController?.abort();
    const controller = new AbortController();
    rttController = controller;
    rttLoading.value = true;
    rttError.value = "";
    try {
      const { data } = await getNodeOverview();
      if (disposed || controller.signal.aborted) return;
      const names = new Set(drawerDetail.value?.nodes.map((n) => n.Name));
      const nodes = (data || [])
        .filter((n: any) => names.has(n.name))
        .map((n: any) => ({ ...n, rtt: n.rtt === -1 ? -1 : -2 }));
      rttMap.value = Object.fromEntries(nodes.map((n: any) => [n.name, n.rtt]));
      await testLocalAll(
        nodes,
        (index, rtt) => {
          if (!disposed && !controller.signal.aborted && nodes[index])
            rttMap.value[nodes[index].name] = rtt;
        },
        controller.signal
      );
    } catch (error) {
      if (!disposed && !controller.signal.aborted)
        rttError.value = messageOf(error, "本地延迟检测失败");
    } finally {
      if (!disposed && !controller.signal.aborted) rttLoading.value = false;
    }
  }
  function toggleNode(name: string, checked: boolean) {
    selectedNodeNames.value = checked
      ? [...new Set([...selectedNodeNames.value, name])]
      : selectedNodeNames.value.filter((n) => n !== name);
  }
  const allSelected = computed(
    () =>
      !!drawerDetail.value?.nodes.length &&
      drawerDetail.value.nodes.every((n) =>
        selectedNodeNames.value.includes(n.Name)
      )
  );
  function toggleAllNodes() {
    selectedNodeNames.value = allSelected.value
      ? []
      : [...new Set(drawerDetail.value?.nodes.map((n) => n.Name) || [])];
  }
  async function saveNodeSelection() {
    const detail = drawerDetail.value;
    if (!detail || savingNodes.value) return;
    savingNodes.value = true;
    nodeSaveError.value = "";
    try {
      await selectAirportNodes({
        id: detail.id,
        nodes: selectedNodeNames.value.join(","),
      });
      if (!disposed) ElMessage.success("节点选择已保存");
    } catch (error) {
      if (!disposed) nodeSaveError.value = messageOf(error, "保存失败，请重试");
    } finally {
      if (!disposed) savingNodes.value = false;
    }
  }
  onMounted(loadData);
  onBeforeUnmount(() => {
    disposed = true;
    ++listGeneration;
    stopDetailWork();
    if (pollTimer) clearTimeout(pollTimer);
  });
  return {
    tableData,
    loading,
    loadError,
    search,
    filtered,
    nodeCount,
    loadData,
    syncStates,
    activeSyncCount,
    handleSync,
    editorVisible,
    saving,
    formError,
    form,
    handleAdd,
    handleEdit,
    submitForm,
    deleteTarget,
    deleting,
    deleteError,
    handleDel,
    confirmDelete,
    drawerVisible,
    drawerLoading,
    drawerDetail,
    detailError,
    detailTarget,
    openDetail,
    selectedNodeNames,
    savingNodes,
    nodeSaveError,
    rttMap,
    rttLoading,
    rttError,
    loadNodeRtt,
    toggleNode,
    allSelected,
    toggleAllNodes,
    saveNodeSelection,
  };
}
