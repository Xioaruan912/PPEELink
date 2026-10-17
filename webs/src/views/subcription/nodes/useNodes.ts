import {
  ref,
  computed,
  provide,
  inject,
  onMounted,
  onBeforeUnmount,
  type InjectionKey,
} from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import {
  getNodes,
  getNodeOverview,
  AddNodes,
  DelNode,
  UpdateNode,
  GetGroupFull,
  SetGroup,
  UnbindGroup,
  DelGroup,
  SetNodeVisibility,
  SetGroupVisibility,
} from "@/api/subcription/node";
import { testLocalAll } from "@/utils/ping";
import { copySubscriptionText } from "../subs/clipboard";

export interface OverviewItem {
  id: number;
  name: string;
  link: string;
  server: string;
  country: string;
  countryCode: string;
  rtt: number;
  groups: string[];
  score: number;
  availability: number;
  averageRtt: number;
  jitter: number;
  sampleCount: number;
  lastTestedAt: string;
}
export interface NodeItem {
  ID: number;
  Name: string;
  Link: string;
  Hidden?: boolean;
  GroupNodes?: { ID: number; Name: string; Hidden?: boolean }[];
}
export interface GroupDetail {
  id: number;
  name: string;
  node_count: number;
  hidden: boolean;
}
export const rttState = (rtt: number) =>
  rtt < 0
    ? rtt === -2
      ? "idle"
      : "bad"
    : rtt < 100
      ? "ok"
      : rtt < 300
        ? "warn"
        : "bad";
export const rttText = (rtt: number) =>
  rtt === -2 ? "测试中" : rtt < 0 ? "超时" : `${rtt} ms`;
export const nodeEffectiveHidden = (node?: NodeItem) =>
  !!node?.Hidden || (node?.GroupNodes || []).some((g) => !!g.Hidden);
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
  const viewMode = ref("list"),
    activeGroup = ref("全部"),
    searchText = ref(""),
    filterCountries = ref<string[]>([]);
  const selectedIds = ref<Set<number>>(new Set());
  const allGroupNames = computed(() =>
    groupDetails.value.filter((g) => !g.hidden).map((g) => g.name)
  );
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
  const cardGroups = computed(() =>
    [...new Set(filteredNodes.value.map((n) => n.country || "未知"))]
      .sort()
      .map((country) => ({
        country,
        items: filteredNodes.value.filter(
          (n) => (n.country || "未知") === country
        ),
      }))
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
  const hiddenNodeCount = computed(
    () => fullNodes.value.filter(nodeEffectiveHidden).length
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
    loadSequence = 0,
    pingController: AbortController | undefined;
  const localTesting = ref(false);
  async function loadAll() {
    if (!alive) return;
    const sequence = ++loadSequence;
    pingController?.abort();
    loading.value = true;
    loadError.value = "";
    try {
      const [ov, nd, gp] = await Promise.all([
        getNodeOverview(),
        getNodes(true),
        GetGroupFull(true),
      ]);
      if (!alive || sequence !== loadSequence) return;
      overviewList.value = (ov?.data || []).map((n: OverviewItem) => ({
        ...n,
        rtt: n.rtt === -1 ? -1 : -2,
      }));
      fullNodes.value = nd?.data || [];
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
      void triggerLocalPing();
    } catch {
      if (alive && sequence === loadSequence)
        loadError.value = "请检查连接后重试。";
    } finally {
      if (alive && sequence === loadSequence) loading.value = false;
    }
  }
  async function triggerLocalPing() {
    pingController?.abort();
    const controller = new AbortController();
    pingController = controller;
    const nodes = overviewList.value;
    localTesting.value = true;
    try {
      await testLocalAll(
        nodes,
        (index, rtt) => {
          if (!controller.signal.aborted && alive && nodes[index])
            nodes[index].rtt = rtt;
        },
        controller.signal
      );
    } finally {
      if (pingController === controller) localTesting.value = false;
    }
  }
  const nodeDialog = ref(false),
    groupDialog = ref(false),
    dialogMode = ref<"add" | "edit">("add");
  const nodeForm = ref({ ID: 0, Name: "", Link: "" }),
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
    nodeForm.value = { ID: 0, Name: "", Link: "" };
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
    };
    chosenGroups.value = (full?.GroupNodes || []).map((g) => g.Name);
    // Hidden groups remain selectable while editing existing membership.
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
          await AddNodes({ link: item, group: groups.join(",") });
      } else
        await UpdateNode({
          id: nodeForm.value.ID,
          name: nodeForm.value.Name,
          link,
          group: groups.join(","),
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
  async function toggleNodeHidden(node: NodeItem) {
    const hidden = !node.Hidden;
    await mutation(
      async () => {
        await SetNodeVisibility(node.ID, hidden);
      },
      hidden ? "节点已全局隐藏" : "节点已恢复显示"
    );
  }
  async function setRowVisible(row: OverviewItem) {
    const full = fullNodes.value.find((n) => n.ID === row.id);
    if (full) await toggleNodeHidden(full);
  }
  async function toggleGroupHidden(group: GroupDetail) {
    const hidden = !group.hidden;
    if (
      hidden &&
      !(await confirm(
        `隐藏分组「${group.name}」后，该组全部节点会全局隐藏，数据仍保留。`,
        "隐藏分组"
      ))
    )
      return;
    await mutation(
      async () => {
        await SetGroupVisibility(group.id, hidden);
      },
      hidden ? "分组已全局隐藏" : "分组已恢复显示"
    );
  }
  async function hideActiveGroup() {
    const group = groupDetails.value.find((g) => g.name === activeGroup.value);
    if (group) await toggleGroupHidden(group);
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
  async function bulk(action: "delete" | "hide" | "unbind") {
    const nodes = [...selectedNodes.value],
      group = activeGroup.value;
    if (!nodes.length || busy.value) return;
    const label =
      action === "delete"
        ? "删除"
        : action === "hide"
          ? "全局隐藏"
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
        else if (action === "hide") await SetNodeVisibility(n.id, true);
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
    qualityVisible = ref(false),
    testNode = ref<OverviewItem | null>(null),
    qualityNode = ref<OverviewItem | null>(null);
  function nodeCommand(command: string, row: OverviewItem) {
    if (command === "quality") {
      qualityNode.value = row;
      qualityVisible.value = true;
    } else if (command === "unlock" || command === "tcp") {
      testNode.value = row;
      if (command === "unlock") unlockVisible.value = true;
      else tcpVisible.value = true;
    } else if (command === "edit") handleEditNode(row);
    else if (command === "delete") void deleteNode(row);
    else if (command === "group") openGroupManager(row);
    else if (command === "unbind") void unbindNode(row);
  }
  const fullNode = (id: number) => fullNodes.value.find((n) => n.ID === id);
  const protocol = (row: OverviewItem) =>
    (linkFor(row)?.match(/^([a-z0-9+-]+):\/\//i)?.[1] || "未知").toUpperCase();
  onMounted(loadAll);
  onBeforeUnmount(() => {
    alive = false;
    ++loadSequence;
    pingController?.abort();
  });
  return {
    loading,
    loadError,
    saving,
    busy,
    overviewList,
    fullNodes,
    groupDetails,
    viewMode,
    activeGroup,
    searchText,
    filterCountries,
    selectedIds,
    allGroupNames,
    countryOptions,
    filteredNodes,
    cardGroups,
    groupTree,
    groupedCount,
    hiddenNodeCount,
    selectedNodes,
    selectedCount,
    allSelected,
    toggleNodes,
    toggleSelect,
    clearSelection,
    clearFilters,
    localTesting,
    loadAll,
    triggerLocalPing,
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
    toggleNodeHidden,
    setRowVisible,
    toggleGroupHidden,
    hideActiveGroup,
    deleteGroup,
    copyNode,
    copySelected,
    bulk,
    nodeCommand,
    fullNode,
    protocol,
    unlockVisible,
    tcpVisible,
    qualityVisible,
    testNode,
    qualityNode,
  };
}
