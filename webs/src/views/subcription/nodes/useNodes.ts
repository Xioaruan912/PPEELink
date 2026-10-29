import {
  ref,
  computed,
  provide,
  inject,
  onMounted,
  onActivated,
  onDeactivated,
  onBeforeUnmount,
  type InjectionKey,
} from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getNodes,
  getNodeOverview,
  pauseNodes,
  AddNodes,
  DelNode,
  UpdateNode,
  GetGroupFull,
  SetGroup,
  UnbindGroup,
  DelGroup,
  SetNodeDelivery,
  SetNodeForceDeliver,
} from "@/api/subcription/node";
import { setNodeProbePolicy } from "@/api/subcription/probe";

import { copySubscriptionText } from "../subs/clipboard";

export interface OverviewItem {
  id: number;
  name: string;
  link: string;
  server: string;
  country: string;
  countryCode: string;
  /** 面板服务器能连上节点端口即有效 */
  valid: boolean;
  /** 暂不下发的原因；为空表示正常下发 */
  blocked?: { kind: string; reason: string; until?: string };
  /** 即使检测无效也照常下发 */
  forceDeliver?: boolean;
  deliveryEnabled: boolean;
  /** 检测策略 */
  probePolicy?: "panel" | "any" | "all" | "manual";
  /** 最近一次各探针结果 */
  probes?: {
    id: number;
    name: string;
    region: string;
    success: boolean;
    rtt: number;
    error?: string;
    checkedAt?: string;
  }[];
  checkedAt: string;
  groups: string[];
  udp?: boolean;
  tfo?: boolean;
  skipCertVerify?: boolean;
}
export type NodeToolKind = "rename" | "flags" | "dedupe" | "temp";
export interface NodeItem {
  ID: number;
  Name: string;
  Link: string;
  deliveryDisabled?: boolean;
  GroupNodes?: { ID: number; Name: string }[];
}
export interface GroupDetail {
  id: number;
  name: string;
  node_count: number;
}
export const validState = (node: OverviewItem) => (node.valid ? "ok" : "bad");
export const validText = (node: OverviewItem) => (node.valid ? "有效" : "无效");
const key: InjectionKey<ReturnType<typeof createNodes>> = Symbol("nodes");
export function provideNodes() {
  const state = createNodes();
  provide(key, state);
  return state;
}
export function useNodes() {
  const state = inject(key);
  if (!state) throw new Error("节点工作台尚未初始化");
  return state;
}
function createNodes() {
  const loading = ref(false),
    loadError = ref(""),
    saving = ref(false),
    busy = ref(false);
  const overviewList = ref<OverviewItem[]>([]),
    fullNodes = ref<NodeItem[]>([]),
    groupDetails = ref<GroupDetail[]>([]);
  const activeGroup = ref("全部"),
    searchText = ref(""),
    filterCountries = ref<string[]>([]);
  const selectedIds = ref<Set<number>>(new Set());
  const allGroupNames = computed(() => groupDetails.value.map((g) => g.name));
  const countryOptions = computed(() =>
    [
      ...new Set(overviewList.value.map((n) => n.country).filter(Boolean)),
    ].sort()
  );
  const filteredNodes = computed(() =>
    overviewList.value.filter(
      (n) =>
        (activeGroup.value === "全部" ||
          (n.groups || []).includes(activeGroup.value)) &&
        (!searchText.value.trim() ||
          `${n.name} ${n.server}`
            .toLowerCase()
            .includes(searchText.value.trim().toLowerCase())) &&
        (!filterCountries.value.length ||
          filterCountries.value.includes(n.country))
    )
  );
  const groupTree = computed(() =>
    allGroupNames.value.map((name) => ({
      name,
      count: overviewList.value.filter((n) => (n.groups || []).includes(name))
        .length,
    }))
  );
  const groupedCount = computed(
    () => overviewList.value.filter((n) => (n.groups || []).length).length
  );
  const selectedNodes = computed(() =>
    overviewList.value.filter((n) => selectedIds.value.has(n.id))
  );
  const selectedCount = computed(() => selectedIds.value.size);
  const allSelected = (nodes: OverviewItem[]) =>
    nodes.length > 0 && nodes.every((n) => selectedIds.value.has(n.id));
  const toggleNodes = (nodes: OverviewItem[]) => {
    const next = new Set(selectedIds.value),
      remove = allSelected(nodes);
    nodes.forEach((n) => (remove ? next.delete(n.id) : next.add(n.id)));
    selectedIds.value = next;
  };
  const toggleSelect = (id: number) => {
    const next = new Set(selectedIds.value);
    if (next.has(id)) next.delete(id);
    else next.add(id);
    selectedIds.value = next;
  };
  const clearSelection = () => {
    selectedIds.value = new Set();
  };
  const clearFilters = () => {
    activeGroup.value = "全部";
    searchText.value = "";
    filterCountries.value = [];
  };
  let alive = true,
    loadSequence = 0;
  let active = true;
  let loadingRequest = false;
  // 有效性检测：面板服务器 TCP 连接节点端口，连得上即有效
  const checking = ref(false);
  async function loadAll(background: boolean | Event = false) {
    const quiet = background === true;
    if (!alive) return;
    if (quiet && loadingRequest) return;
    const sequence = ++loadSequence;
    loadingRequest = true;
    if (!quiet) {
      loading.value = true;
      loadError.value = "";
    }
    try {
      const [ov, nd, gp] = await Promise.all([
        getNodeOverview(),
        getNodes(true),
        GetGroupFull(true),
      ]);
      if (!alive || sequence !== loadSequence) return;
      overviewList.value = (ov?.data || []).map((n: OverviewItem) => ({
        ...n,
      }));
      fullNodes.value = nd?.data || [];
      loadError.value = "";
      groupDetails.value = Array.isArray(gp?.data) ? gp.data : [];
      selectedIds.value = new Set(
        [...selectedIds.value].filter((id) =>
          overviewList.value.some((n) => n.id === id)
        )
      );
      if (
        activeGroup.value !== "全部" &&
        !allGroupNames.value.includes(activeGroup.value)
      )
        activeGroup.value = "全部";
      if (
        !manageGroupId.value ||
        !groupDetails.value.some((g) => g.id === manageGroupId.value)
      )
        manageGroupId.value = groupDetails.value[0]?.id;
    } catch {
      if (!quiet && alive && sequence === loadSequence)
        loadError.value = "请检查连接后重试。";
    } finally {
      if (alive && sequence === loadSequence) {
        loading.value = false;
        loadingRequest = false;
      }
    }
  }
  // 立即重新检测全部节点是否有效
  async function checkValidity() {
    if (checking.value) return;
    checking.value = true;
    try {
      const { data } = await getNodeOverview(true);
      if (!alive) return;
      const byId = new Map<number, OverviewItem>(
        (data || []).map((n: OverviewItem) => [n.id, n])
      );
      for (const n of overviewList.value) {
        const fresh = byId.get(n.id);
        if (fresh)
          Object.assign(n, { valid: fresh.valid, checkedAt: fresh.checkedAt });
      }
      const bad = overviewList.value.filter((n) => !n.valid).length;
      ElMessage.success(
        bad ? `检测完成，${bad} 个节点无效` : "检测完成，全部有效"
      );
    } finally {
      checking.value = false;
    }
  }
  const probePolicyDialog = ref(false),
    probePolicyNode = ref<OverviewItem | null>(null),
    probePolicy = ref<"panel" | "any" | "all" | "manual">("panel"),
    savingProbePolicy = ref(false);
  function openProbePolicy(row: OverviewItem) {
    probePolicyNode.value = row;
    probePolicy.value = row.probePolicy || "panel";
    probePolicyDialog.value = true;
  }
  async function saveProbePolicy() {
    if (!probePolicyNode.value || savingProbePolicy.value) return;
    savingProbePolicy.value = true;
    try {
      await setNodeProbePolicy(probePolicyNode.value.id, probePolicy.value);
      probePolicyNode.value.probePolicy = probePolicy.value;
      ElMessage.success("检测策略已保存");
      probePolicyDialog.value = false;
      await loadAll();
    } catch {
      ElMessage.error("保存失败，请稍后重试");
    } finally {
      savingProbePolicy.value = false;
    }
  }
  const nodeDialog = ref(false),
    groupDialog = ref(false),
    dialogMode = ref<"add" | "edit">("add");
  const nodeForm = ref({ ID: 0, Name: "", Link: "", Country: "auto" }),
    groupMode = ref("existing"),
    chosenGroups = ref<string[]>([]),
    newGroups = ref("");
  const manageGroupId = ref<number>(),
    selectionNode = ref(""),
    delGroupName = ref("");
  const manageGroupNodes = computed(() =>
    fullNodes.value.filter((n) =>
      (n.GroupNodes || []).some((g) => g.ID === manageGroupId.value)
    )
  );
  const chosenGroupNames = () =>
    groupMode.value === "existing"
      ? chosenGroups.value
      : newGroups.value
          .split(",")
          .map((s) => s.trim())
          .filter(Boolean);
  const resetGroups = () => {
    groupMode.value = allGroupNames.value.length ? "existing" : "new";
    chosenGroups.value = [];
    newGroups.value = "";
  };
  function handleAddNode() {
    dialogMode.value = "add";
    nodeForm.value = { ID: 0, Name: "", Link: "", Country: "auto" };
    resetGroups();
    nodeDialog.value = true;
  }
  function handleEditNode(row: OverviewItem) {
    const full = fullNodes.value.find((n) => n.ID === row.id);
    dialogMode.value = "edit";
    resetGroups();
    nodeForm.value = {
      ID: row.id,
      Name: row.name,
      Link: full?.Link || row.link,
      Country: "auto",
    };
    chosenGroups.value = (full?.GroupNodes || []).map((g) => g.Name);
    groupMode.value = "existing";
    nodeDialog.value = true;
  }
  async function submitNode() {
    if (saving.value) return;
    const groups = chosenGroupNames(),
      link = nodeForm.value.Link.trim();
    const links = link
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean);
    if (!groups.length) {
      ElMessage.warning("节点必须至少选择一个分组");
      return;
    }
    if (!link || (dialogMode.value === "add" && !links.length)) {
      ElMessage.warning("节点链接不能为空");
      return;
    }
    saving.value = true;
    try {
      if (dialogMode.value === "add") {
        for (const item of links)
          await AddNodes({
            link: item,
            group: groups.join(","),
            country: nodeForm.value.Country,
          });
      } else
        await UpdateNode({
          id: nodeForm.value.ID,
          name: nodeForm.value.Name,
          link,
          group: groups.join(","),
          country: nodeForm.value.Country,
        });
      ElMessage.success(
        dialogMode.value === "add" ? "节点添加成功" : "节点更新成功"
      );
      nodeDialog.value = false;
    } catch {
      ElMessage.error("保存失败，请检查后重试");
    } finally {
      saving.value = false;
      await loadAll();
    }
  }
  function openGroupManager(node?: OverviewItem) {
    resetGroups();
    selectionNode.value = node?.name || "";
    groupDialog.value = true;
  }
  async function bindGroup() {
    if (busy.value) return;
    const groups = chosenGroupNames();
    if (!selectionNode.value) {
      ElMessage.warning("请选择节点");
      return;
    }
    if (!groups.length) {
      ElMessage.warning(
        groupMode.value === "new" ? "分组名不能为空" : "请选择分组"
      );
      return;
    }
    busy.value = true;
    try {
      const existing =
        fullNodes.value.find((node) => node.Name === selectionNode.value)
          ?.GroupNodes || [];
      // The backend replaces membership; joining must send the complete union.
      const membership = [
        ...new Set([...existing.map((group) => group.Name), ...groups]),
      ];
      await SetGroup({
        name: selectionNode.value,
        group: membership.join(","),
      });
      ElMessage.success("分组绑定成功");
      await loadAll();
    } catch {
      ElMessage.error("绑定失败");
    } finally {
      busy.value = false;
    }
  }
  async function confirm(message: string, title = "确认操作") {
    if (!alive) return false;
    try {
      await ElMessageBox.confirm(message, title, {
        type: "warning",
        confirmButtonText: "确认",
        cancelButtonText: "取消",
      });
      return alive;
    } catch {
      return false;
    }
  }
  async function mutation(work: () => Promise<void>, success: string) {
    if (busy.value || !alive) return;
    busy.value = true;
    try {
      await work();
      ElMessage.success(success);
    } catch {
      ElMessage.error("操作失败，请重试");
    } finally {
      busy.value = false;
      await loadAll();
    }
  }
  async function setDelivery(id: number, enabled: boolean) {
    await mutation(
      async () => {
        await SetNodeDelivery(id, enabled);
      },
      enabled ? "已开启订阅下发" : "已关闭订阅下发，节点仍保留在列表中"
    );
  }
  async function deleteGroup() {
    if (!delGroupName.value) {
      ElMessage.warning("请选择分组");
      return;
    }
    const name = delGroupName.value;
    if (
      !(await confirm(
        `删除分组「${name}」？仅解除节点绑定，节点仍保留。`,
        "删除分组"
      ))
    )
      return;
    await mutation(async () => {
      await DelGroup({ name });
      delGroupName.value = "";
    }, "分组已删除");
  }
  const linkFor = (row: OverviewItem) =>
    fullNodes.value.find((n) => n.ID === row.id)?.Link || row.link;
  async function copyText(value: string) {
    try {
      await copySubscriptionText(value);
      ElMessage.success("链接已复制");
    } catch {
      ElMessage.error("复制失败，请重试");
    }
  }
  const copyNode = (row: OverviewItem) => copyText(linkFor(row));
  const copySelected = () =>
    copyText(selectedNodes.value.map(linkFor).join("\n"));
  async function deleteNode(row: OverviewItem) {
    if (!(await confirm(`删除节点「${row.name}」？`, "删除节点"))) return;
    await mutation(async () => {
      await DelNode({ id: row.id });
    }, "节点已删除");
  }
  async function bulk(
    action: "delete" | "enable-delivery" | "disable-delivery" | "unbind"
  ) {
    const nodes = [...selectedNodes.value],
      group = activeGroup.value;
    if (!nodes.length || busy.value) return;
    const label =
      action === "delete"
        ? "删除"
        : action === "disable-delivery"
          ? "关闭下发"
          : action === "enable-delivery"
            ? "开启下发"
            : `移出分组「${group}」`;
    if (
      !(await confirm(
        `${label}选中的 ${nodes.length} 个节点？${action === "delete" ? "" : "节点数据仍保留。"}`,
        "批量操作"
      ))
    )
      return;
    await mutation(async () => {
      for (const n of nodes) {
        if (action === "delete") await DelNode({ id: n.id });
        else if (action === "enable-delivery" || action === "disable-delivery")
          await SetNodeDelivery(n.id, action === "enable-delivery");
        else if ((n.groups || []).includes(group))
          await UnbindGroup({ name: n.name, group });
      }
      clearSelection();
    }, "批量操作完成");
  }
  async function unbindNode(row: OverviewItem) {
    const group = activeGroup.value;
    if (
      group === "全部" ||
      !(await confirm(`将「${row.name}」移出分组「${group}」？节点仍保留。`))
    )
      return;
    await mutation(async () => {
      await UnbindGroup({ name: row.name, group });
    }, "已移出分组");
  }
  const unlockVisible = ref(false),
    tcpVisible = ref(false),
    testNode = ref<OverviewItem | null>(null);
  // 批量工具对话框：kind 决定内容，targets 为作用节点（去重作用于全部）
  const tool = ref<{ kind: NodeToolKind; targets: OverviewItem[] } | null>(
    null
  );
  function openTool(kind: NodeToolKind, targets?: OverviewItem[]) {
    tool.value = { kind, targets: targets ?? selectedNodes.value };
  }
  const nodeName = (id?: number) =>
    id ? overviewList.value.find((n) => n.id === id)?.name : undefined;
  function nodeCommand(command: string, row: OverviewItem) {
    if (command === "probe-policy") {
      openProbePolicy(row);
      return;
    }
    if (command === "deliver") {
      void toggleForceDeliver(row);
      return;
    }
    if (command === "temp") {
      openTool(command, [row]);
      return;
    }
    if (command === "pause") {
      pauseTargets.value = [row];
      pauseDialog.value = true;
      return;
    }
    if (command === "resume") {
      void resumeNodes([row.id]);
      return;
    }
    if (command === "unlock" || command === "tcp") {
      testNode.value = row;
      if (command === "unlock") unlockVisible.value = true;
      else tcpVisible.value = true;
    } else if (command === "edit") handleEditNode(row);
    else if (command === "delete") void deleteNode(row);
    else if (command === "group") openGroupManager(row);
    else if (command === "unbind") void unbindNode(row);
  }
  // 暂停下发：节点流量用完等情况，暂停到恢复日，期间不出现在任何订阅里
  const pauseDialog = ref(false);
  const pauseTargets = ref<OverviewItem[]>([]);
  function openPause(rows: OverviewItem[]) {
    if (!rows.length) {
      ElMessage.warning("请选择节点");
      return;
    }
    pauseTargets.value = rows;
    pauseDialog.value = true;
  }
  async function toggleForceDeliver(row: OverviewItem) {
    const next = !row.forceDeliver;
    await mutation(
      async () => {
        await SetNodeForceDeliver(row.id, next);
      },
      next ? "已设为始终下发" : "已恢复按检测结果下发"
    );
  }
  async function resumeNodes(ids: number[]) {
    const res: any = await pauseNodes(ids, "");
    ElMessage.success(res?.msg || "已恢复下发");
    await loadAll();
  }
  const fullNode = (id: number) => fullNodes.value.find((n) => n.ID === id);
  const protocol = (row: OverviewItem) =>
    (linkFor(row)?.match(/^([a-z0-9+-]+):\/\//i)?.[1] || "未知").toUpperCase();
  let refreshTimer: ReturnType<typeof setInterval> | undefined;
  function refreshVisible() {
    if (active && !document.hidden) void loadAll(true);
  }
  function startRefresh() {
    if (!refreshTimer) refreshTimer = setInterval(refreshVisible, 15000);
  }
  function stopRefresh() {
    clearInterval(refreshTimer);
    refreshTimer = undefined;
  }
  onMounted(() => {
    void loadAll();
    startRefresh();
    document.addEventListener("visibilitychange", refreshVisible);
  });
  onActivated(() => {
    active = true;
    refreshVisible();
    startRefresh();
  });
  onDeactivated(() => {
    active = false;
    stopRefresh();
    ++loadSequence;
    loadingRequest = false;
    loading.value = false;
  });
  onBeforeUnmount(() => {
    alive = false;
    ++loadSequence;
    stopRefresh();
    document.removeEventListener("visibilitychange", refreshVisible);
  });
  return {
    loading,
    loadError,
    saving,
    busy,
    overviewList,
    fullNodes,
    groupDetails,
    activeGroup,
    searchText,
    filterCountries,
    selectedIds,
    allGroupNames,
    countryOptions,
    filteredNodes,
    groupTree,
    groupedCount,
    selectedNodes,
    selectedCount,
    allSelected,
    toggleNodes,
    toggleSelect,
    clearSelection,
    clearFilters,
    checking,
    checkValidity,
    pauseDialog,
    pauseTargets,
    openPause,
    resumeNodes,
    toggleForceDeliver,
    loadAll,
    nodeDialog,
    groupDialog,
    dialogMode,
    nodeForm,
    groupMode,
    chosenGroups,
    newGroups,
    manageGroupId,
    selectionNode,
    delGroupName,
    manageGroupNodes,
    handleAddNode,
    handleEditNode,
    submitNode,
    openGroupManager,
    bindGroup,
    setDelivery,
    deleteGroup,
    copyNode,
    copySelected,
    bulk,
    nodeCommand,
    fullNode,
    protocol,
    unlockVisible,
    tcpVisible,
    testNode,
    tool,
    openTool,
    nodeName,
    probePolicyDialog,
    probePolicyNode,
    probePolicy,
    savingProbePolicy,
    openProbePolicy,
    saveProbePolicy,
  };
}
