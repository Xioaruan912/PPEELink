import { formatDateTime } from "@/utils/date";
import {
  ref,
  reactive,
  computed,
  onMounted,
  onBeforeUnmount,
  inject,
  provide,
  watch,
} from "vue";
import {
  getSubs,
  getSubPreviewNodes,
  previewSubPipeline,
  startSubscriptionBuild,
  getSubscriptionArtifacts,
  rollbackSubscriptionArtifact,
  safePublishSubscription,
  AddSub,
  DelSub,
  UpdateSub,
  ResetToken,
  SetExpire,
  getImportLinks,
  createShortLink,
} from "@/api/subcription/subs";
import { getTemp } from "@/api/subcription/temp";
import {
  getNodes,
  getNodeOverview,
  GetGroupFull,
} from "@/api/subcription/node";

export interface Sub {
  ID: number;
  Name: string;
  CreatedAt: string;
  Config: string;
  NodeOrder?: string;
  Token: string;
  ExpiresAt: string | null;
  Nodes: Node[];
  GroupRefs?: GroupRef[];
  Pipeline?: string;
  SourceURLs?: string;
  SubLogs: SubLogs[];
}
export interface Node {
  ID: number;
  Name: string;
  Link: string;
  GroupNodes?: { Name: string }[];
}
interface GroupRef {
  ID: number;
  Name: string;
  NodeCount: number;
}
interface Config {
  clash: string;
  surge: string;
  loon: string;
  udp: boolean;
  cert: boolean;
}
interface SubLogs {
  IP: string;
  Count: number;
  Addr: string;
  Date: string;
}
interface Temp {
  file: string;
}
interface OverviewItem {
  id: number;
  name: string;
  server: string;
  country: string;
  countryCode: string;
  rtt: number;
  groups: string[];
  score: number;
  availability: number;
  confidence: number;
  sampleCount: number;
}

import { ElMessage } from "element-plus";
import { extractHostPort } from "@/utils/ping";
import { copySubscriptionText } from "./clipboard";

export function createSubscriptions() {
  const tableData = ref<Sub[]>([]);
  const searchKey = ref("");
  const Clash = ref("");
  const Surge = ref("");
  const Loon = ref("");
  const clashMode = ref("1");
  const surgeMode = ref("1");
  const loonMode = ref("1");
  const SubTitle = ref("");
  const Subname = ref("");
  const oldSubname = ref("");
  const airportUrl = ref("");
  const isAirportUrl = ref(false);
  const dialogVisible = ref(false);
  const NodesList = ref<Node[]>([]);
  const value1 = ref<string[]>([]);
  const allGroups = ref<GroupRef[]>([]);
  const selectedGroups = ref<number[]>([]);
  const openedNodeGroups = ref<string[]>([]);
  const nodeQualityMap = computed(
    () => new Map(overview.value.map((item) => [item.id, item]))
  );
  const nodeByName = computed(
    () => new Map(NodesList.value.map((item) => [item.Name, item]))
  );
  const nodeQuality = (node: Node) => nodeQualityMap.value.get(node.ID);
  const nodeQualityByName = (name: string) => {
    const node = nodeByName.value.get(name);
    return node ? nodeQuality(node) : undefined;
  };
  const nodeQualityLabel = (node: Node) => {
    const q = nodeQuality(node);
    if (!q || q.sampleCount <= 0) return "暂无质量数据";
    if (q.rtt < 0) return "当前离线";
    return `质量 ${q.score}分`;
  };
  const groupedNodes = computed(() => {
    const by = new Map<string, Node[]>();
    for (const n of NodesList.value) {
      const names = (n.GroupNodes || []).map((g) => g.Name).filter(Boolean);
      for (const name of names.length ? names : ["未分组"]) {
        if (!by.has(name)) by.set(name, []);
        by.get(name)!.push(n);
      }
    }
    return [...by.entries()]

      .map(([name, nodes]) => {
        const sortedNodes = [...nodes].sort((a, b) => {
          const qa = nodeQuality(a);
          const qb = nodeQuality(b);
          const onlineDiff =
            Number((qb?.rtt ?? -1) >= 0) - Number((qa?.rtt ?? -1) >= 0);
          if (onlineDiff) return onlineDiff;
          const scoreDiff = (qb?.score || 0) - (qa?.score || 0);
          if (scoreDiff) return scoreDiff;
          const availabilityDiff =
            (qb?.availability || 0) - (qa?.availability || 0);
          return availabilityDiff || a.Name.localeCompare(b.Name, "zh-CN");
        });
        const qualitySamples = sortedNodes
          .map((node) => nodeQuality(node))
          .filter(
            (q): q is OverviewItem => !!q && q.sampleCount > 0 && q.rtt >= 0
          );
        return {
          name,
          nodes: sortedNodes,
          qualityScore: qualitySamples.length
            ? Math.max(...qualitySamples.map((q) => q.score))
            : -1,
        };
      })
      .sort(
        (a, b) =>
          b.qualityScore - a.qualityScore ||
          a.name.localeCompare(b.name, "zh-CN")
      );
  });
  const udpOn = ref(false);
  const certOn = ref(false);
  const qrcode = ref("");
  const qrTitle = ref("");
  const qrDialog = ref(false);
  const templist = ref<Temp[]>([]);
  const editingSubId = ref(0);
  const artifactDrawer = ref(false);
  const artifactLoading = ref(false);
  const artifactError = ref("");
  let artifactVersion = 0;
  const artifactSub = ref<Sub | null>(null);
  const artifactClient = ref("clash");
  const artifactItems = ref<any[]>([]);
  const artifactPointers = ref<any[]>([]);
  const publishDialog = ref(false);
  const publishSub = ref<Sub | null>(null);
  const publishForm = ref({ client: "clash", template: "" });
  const publishLoading = ref(false);
  const pipeline = reactive({
    include: "",
    exclude: "",
    renamePattern: "",
    renameReplacement: "",
    protocols: [] as string[],
    sort: "original",
    dedupe: true,
    maxNodes: 0,
    excludePlaceholder: false,
    deletePattern: "",
    regexSort: "",
    resolveDomain: false,
    emoji: false,
    emojiRemoveOld: true,
    setUdp: "" as "" | "on" | "off",
    setTfo: "" as "" | "on" | "off",
    setSkipCertVerify: "" as "" | "on" | "off",
    maxMultiplier: 0,
    script: "",
  });
  const emojiRulesText = ref("");
  const pipelinePreview = ref<any>(null);
  const pipelineLoading = ref(false);
  let pipelineVersion = 0;
  const parseEmojiRules = (text: string) =>
    text
      .split("\n")
      .map((l) => l.trim())
      .filter(Boolean)
      .map((l) => {
        const i = l.indexOf("=");
        if (i < 0) return null;
        return { match: l.slice(0, i).trim(), emoji: l.slice(i + 1).trim() };
      })
      .filter(
        (r): r is { match: string; emoji: string } =>
          !!r && !!r.match && !!r.emoji
      );
  const pipelineJSON = () => {
    const payload: any = {
      include: pipeline.include,
      exclude: pipeline.exclude,
      renamePattern: pipeline.renamePattern,
      renameReplacement: pipeline.renameReplacement,
      protocols: pipeline.protocols,
      sort: pipeline.sort,
      dedupe: pipeline.dedupe,
      maxNodes: pipeline.maxNodes,
      emoji: pipeline.emoji,
      emojiRemoveOld: pipeline.emojiRemoveOld,
      excludePlaceholder: pipeline.excludePlaceholder,
      deletePattern: pipeline.deletePattern,
      regexSort: pipeline.regexSort,
      resolveDomain: pipeline.resolveDomain,
    };
    if (pipeline.setUdp) payload.setUdp = pipeline.setUdp === "on";
    if (pipeline.setTfo) payload.setTfo = pipeline.setTfo === "on";
    if (pipeline.setSkipCertVerify)
      payload.setSkipCertVerify = pipeline.setSkipCertVerify === "on";
    if (pipeline.maxMultiplier > 0)
      payload.maxMultiplier = pipeline.maxMultiplier;
    if (pipeline.script.trim()) payload.script = pipeline.script;
    const rules = parseEmojiRules(emojiRulesText.value);
    if (rules.length) payload.emojiRules = rules;
    return JSON.stringify(payload);
  };
  const buildDialog = ref(false);
  const buildSub = ref<Sub | null>(null);
  const buildClient = ref("clash");
  const buildLoading = ref(false);
  const buildSubscription = (sub: Sub) => {
    buildSub.value = sub;
    buildClient.value = "clash";
    buildDialog.value = true;
  };
  const runBuild = async () => {
    if (!buildSub.value) return;
    buildLoading.value = true;
    try {
      const { data } = await startSubscriptionBuild({
        subscriptionId: buildSub.value.ID,
        client: buildClient.value,
      });
      ElMessage.success(
        `生成产物任务已创建 #${data?.taskId || ""}，可在任务中心查看`
      );
      buildDialog.value = false;
    } finally {
      buildLoading.value = false;
    }
  };
  const loadArtifacts = async () => {
    if (!artifactSub.value) return;
    const version = ++artifactVersion;
    artifactLoading.value = true;
    artifactError.value = "";
    artifactItems.value = [];
    artifactPointers.value = [];
    try {
      const { data } = await getSubscriptionArtifacts(
        artifactSub.value.ID,
        artifactClient.value
      );
      if (!alive || version !== artifactVersion) return;
      artifactItems.value = data?.items || [];
      artifactPointers.value = data?.pointers || [];
    } catch {
      if (alive && version === artifactVersion)
        artifactError.value = "产物版本加载失败，请重试";
    } finally {
      if (alive && version === artifactVersion) artifactLoading.value = false;
    }
  };
  const openArtifacts = async (sub: Sub) => {
    artifactSub.value = sub;
    artifactDrawer.value = true;
    await loadArtifacts();
  };
  const lkgId = computed(
    () =>
      artifactPointers.value.find((p: any) => p.client === artifactClient.value)
        ?.lastKnownGoodArtifactId || 0
  );
  const setLkg = async (artifact: any) => {
    if (
      !(await confirmAction(
        "回滚产物",
        `将产物 #${artifact.id} 设为 ${artifactClient.value} 的 Last Known Good？`,
        "确认回滚"
      ))
    )
      return;
    await rollbackSubscriptionArtifact(artifact.id);
    ElMessage.success("Last Known Good 已切换");
    await Promise.all([loadArtifacts(), getsubs()]);
  };
  const openSafePublish = (sub: Sub) => {
    publishSub.value = sub;
    publishForm.value = {
      client: "clash",
      template:
        templist.value.find((t) => /\.ya?ml$/i.test(t.file))?.file || "",
    };
    publishDialog.value = true;
  };
  const runSafePublish = async () => {
    if (!publishSub.value || !publishForm.value.template) return;
    publishLoading.value = true;
    try {
      const { data } = await safePublishSubscription({
        subscriptionId: publishSub.value.ID,
        client: publishForm.value.client,
        template: publishForm.value.template,
      });
      ElMessage.success(
        `发布验证任务已创建 #${data?.taskId || ""}，全部检查通过后才会切换 LKG`
      );
      publishDialog.value = false;
    } finally {
      publishLoading.value = false;
    }
  };
  const shortLink = ref("");
  const importDialog = ref(false);
  const importSub = ref<Sub | null>(null);
  const importData = ref<any>(null);
  const importLoading = ref(false);
  let importVersion = 0;
  const openImport = async (sub: Sub) => {
    const version = ++importVersion;
    importSub.value = sub;
    importData.value = null;
    shortLink.value = "";
    importDialog.value = true;
    importLoading.value = true;
    try {
      const { data } = await getImportLinks(sub.ID);
      if (alive && version === importVersion && importDialog.value)
        importData.value = data;
    } finally {
      if (alive && version === importVersion) importLoading.value = false;
    }
  };
  watch(importDialog, (visible) => {
    if (!visible) importVersion++;
  });
  const makeShortLink = async (target: string) => {
    const version = importVersion;
    const subscriptionId = importSub.value?.ID;
    const { data } = await createShortLink({
      target,
      remark: importSub.value?.Name || "",
    });
    if (
      !alive ||
      version !== importVersion ||
      !importDialog.value ||
      importSub.value?.ID !== subscriptionId
    )
      return;
    if (data?.url) {
      ElMessage.success("短链已生成并复制");
      shortLink.value = data.url;
      try {
        await copySubscriptionText(data.url);
      } catch {
        ElMessage.warning("短链已生成，请手动复制");
      }
    }
  };

  // 卡片「更多」菜单派发
  const onCardCommand = async (cmd: string, sub: Sub) => {
    try {
      switch (cmd) {
        case "publish":
          openSafePublish(sub);
          break;
        case "build":
          buildSubscription(sub);
          break;
        case "artifacts":
          await openArtifacts(sub);
          break;
        case "reset":
          await handleReset(sub);
          break;
        case "edit":
          handleEdit(sub);
          break;
        case "delete":
          await handleDel(sub);
          break;
      }
    } catch {
      ElMessage.error("操作失败，请检查服务后重试");
    }
  };

  const resetPipeline = (raw = "") => {
    pipelineVersion++;
    pipelineLoading.value = false;
    const defaults: any = {
      include: "",
      exclude: "",
      renamePattern: "",
      renameReplacement: "",
      protocols: [],
      sort: "original",
      dedupe: true,
      maxNodes: 0,
      excludePlaceholder: false,
      deletePattern: "",
      regexSort: "",
      resolveDomain: false,
      emoji: false,
      emojiRemoveOld: true,
      maxMultiplier: 0,
    };
    let parsed: any = {};
    try {
      const value = JSON.parse(raw || "{}");
      parsed =
        value && typeof value === "object" && !Array.isArray(value)
          ? value
          : {};
    } catch {
      /* use defaults */
    }
    const value = { ...defaults, ...parsed };
    value.setUdp =
      typeof parsed.setUdp === "boolean" ? (parsed.setUdp ? "on" : "off") : "";
    value.setTfo =
      typeof parsed.setTfo === "boolean" ? (parsed.setTfo ? "on" : "off") : "";
    value.setSkipCertVerify =
      typeof parsed.setSkipCertVerify === "boolean"
        ? parsed.setSkipCertVerify
          ? "on"
          : "off"
        : "";
    if (!value.script) value.script = "";
    emojiRulesText.value = (parsed.emojiRules || [])
      .map((r: any) => `${r.match}=${r.emoji}`)
      .join("\n");
    Object.assign(pipeline, value);
    pipelinePreview.value = null;
  };
  const runPipelinePreview = async () => {
    if (!editingSubId.value) {
      ElMessage.info("新增订阅保存后即可预览真实处理结果");
      return;
    }
    const version = ++pipelineVersion;
    pipelineLoading.value = true;
    pipelinePreview.value = null;
    try {
      const { data } = await previewSubPipeline({
        id: editingSubId.value,
        pipeline: pipelineJSON(),
      });
      if (alive && version === pipelineVersion && dialogVisible.value)
        pipelinePreview.value = data;
    } finally {
      if (alive && version === pipelineVersion) pipelineLoading.value = false;
    }
  };

  // 抽屉
  const drawerVisible = ref(false);
  const drawerLoading = ref(false);
  const drawerSub = ref<Sub | null>(null);
  const overview = ref<OverviewItem[]>([]);
  const expirePick = ref("");
  const expireClear = ref(false);

  // 每卡客户端 tab
  const activeClient = ref<Record<number, string>>({});
  const clientOptions = [
    { label: "自动识别", value: "auto" },
    { label: "V2Ray", value: "v2ray" },
    { label: "Clash", value: "clash" },
    { label: "Surge", value: "surge" },
    { label: "Loon", value: "loon" },
  ];

  const statusFilter = ref<string | number>("all");
  const filteredSubs = computed(() =>
    tableData.value.filter((sub) => {
      const matches = sub.Name.toLowerCase().includes(
        searchKey.value.trim().toLowerCase()
      );
      const expired =
        !!sub.ExpiresAt && new Date(sub.ExpiresAt).getTime() <= Date.now();
      return (
        matches &&
        (statusFilter.value === "all" ||
          (statusFilter.value === "expired" ? expired : !expired))
      );
    })
  );
  const expiredCount = computed(
    () =>
      tableData.value.filter(
        (sub) =>
          sub.ExpiresAt && new Date(sub.ExpiresAt).getTime() <= Date.now()
      ).length
  );
  const recentPublished = ref<Record<number, string>>({});
  const publicationErrors = ref<Record<number, boolean>>({});
  const publicationLoading = ref<Record<number, boolean>>({});
  const nodeCounts = ref<Record<number, number>>({});
  const serverAddress = () => {
    return (
      location.protocol +
      "//" +
      location.hostname +
      (location.port ? ":" + location.port : "")
    );
  };

  const subUrl = (sub: Sub, client: string) => {
    const base = `${serverAddress()}/c/?token=${sub.Token}`;
    return client === "auto" ? base : `${base}&client=${client}`;
  };

  const currentUrl = (sub: Sub) => {
    return subUrl(sub, activeClient.value[sub.ID] || "auto");
  };

  const loading = ref(false);

  async function getsubs() {
    const version = ++loadVersion;
    loading.value = true;
    loadError.value = "";
    try {
      const { data } = await getSubs();
      if (!alive || version !== loadVersion) return;
      tableData.value = (data || []).map((sub: Sub) => ({
        ...sub,
        Nodes: sub.Nodes || [],
        GroupRefs: sub.GroupRefs || [],
        SubLogs: sub.SubLogs || [],
      }));
      recentPublished.value = {};
      publicationErrors.value = {};
      nodeCounts.value = {};
      publicationLoading.value = Object.fromEntries(
        tableData.value.map((sub) => [sub.ID, true])
      );
      loading.value = false;
      void Promise.allSettled(
        tableData.value.map(async (sub) => {
          const [artifacts, preview] = await Promise.allSettled([
            getSubscriptionArtifacts(sub.ID),
            getSubPreviewNodes(sub.ID),
          ]);
          if (!alive || version !== loadVersion) return;
          publicationLoading.value[sub.ID] = false;
          if (artifacts.status === "fulfilled") {
            const { items = [], pointers = [] } = artifacts.value.data || {};
            const current = new Set(
              pointers.map((p: any) => p.lastKnownGoodArtifactId)
            );
            const published = items
              .filter((item: any) => current.has(item.id))
              .map((item: any) => item.createdAt)
              .sort();
            recentPublished.value[sub.ID] = published.at(-1) || "";
          } else publicationErrors.value[sub.ID] = true;
          if (preview.status === "fulfilled")
            nodeCounts.value[sub.ID] = (preview.value.data || []).length;
        })
      );
    } catch {
      if (alive && version === loadVersion)
        loadError.value = "订阅加载失败，请重试";
    } finally {
      if (alive && version === loadVersion) loading.value = false;
    }
  }
  const loadError = ref("");
  const metadataError = ref("");
  let alive = true;
  let loadVersion = 0;
  const loadMetadata = async () => {
    metadataError.value = "";
    const results = await Promise.allSettled([
      getTemp(),
      getNodes(),
      GetGroupFull(),
      getNodeOverview(),
    ]);
    if (!alive) return;
    const [temps, nodes, groups, quality] = results;
    if (temps.status === "fulfilled") templist.value = temps.value.data || [];
    if (nodes.status === "fulfilled") NodesList.value = nodes.value.data || [];
    if (groups.status === "fulfilled")
      allGroups.value = (groups.value.data || []).map((g: any) => ({
        ID: g.id,
        Name: g.name,
        NodeCount: g.node_count || 0,
      }));
    if (quality.status === "fulfilled")
      overview.value = quality.value.data || [];
    if (results.some((r) => r.status === "rejected"))
      metadataError.value = "节点、分组或模板未完整加载，请重试";
  };
  onMounted(() => {
    getsubs();
    loadMetadata();
  });
  onBeforeUnmount(() => {
    alive = false;
    loadVersion++;
    confirmationResolve?.(false);
  });
  // ---- 添加/编辑 ----
  const saving = ref(false);
  const editorSection = ref<string | number>("basic");
  const addSubs = async () => {
    if (saving.value) return;
    if (!Subname.value.trim()) {
      editorSection.value = "basic";
      ElMessage.warning("订阅名称不能为空");
      return;
    }
    if (
      !isAirportUrl.value &&
      value1.value.length === 0 &&
      selectedGroups.value.length === 0
    ) {
      editorSection.value = "sources";
      ElMessage.warning("请选择至少一个节点或分组");
      return;
    }
    if (isAirportUrl.value && !airportUrl.value.trim()) {
      editorSection.value = "sources";
      ElMessage.warning("机场订阅链接不能为空");
      return;
    }

    saving.value = true;
    try {
      const config = JSON.stringify({
        clash: Clash.value.trim(),
        surge: Surge.value.trim(),
        loon: Loon.value.trim(),
        udp: udpOn.value,
        cert: certOn.value,
      });
      const groupStr = selectedGroups.value.join(",");
      if (SubTitle.value === "添加订阅") {
        await AddSub({
          config,
          name: Subname.value.trim(),
          nodes: value1.value.join(","),
          groups: groupStr,
          airport_url: isAirportUrl.value ? airportUrl.value.trim() : "",
          pipeline: pipelineJSON(),
        });
        ElMessage.success("添加成功");
      } else {
        await UpdateSub({
          config,
          name: Subname.value.trim(),
          nodes: value1.value.join(","),
          groups: groupStr,
          oldname: oldSubname.value,
          airport_url: isAirportUrl.value ? airportUrl.value.trim() : "",
          pipeline: pipelineJSON(),
        });
        ElMessage.success("更新成功");
      }
      await getsubs();
      dialogVisible.value = false;
    } finally {
      saving.value = false;
    }
  };

  const handleAddSub = () => {
    editorSection.value = "basic";
    SubTitle.value = "添加订阅";
    Subname.value = "";
    oldSubname.value = "";
    airportUrl.value = "";
    isAirportUrl.value = false;
    udpOn.value = false;
    certOn.value = false;
    Clash.value = "./template/clash.yaml";
    Surge.value = "./template/surge.conf";
    Loon.value = "./template/loon.conf";
    clashMode.value = "1";
    surgeMode.value = "1";
    loonMode.value = "1";
    value1.value = [];
    selectedGroups.value = [];
    editingSubId.value = 0;
    resetPipeline();
    dialogVisible.value = true;
  };

  const parseConfig = (raw: string): Config => {
    try {
      const c = JSON.parse(raw || "{}");
      return {
        clash: c.clash || "",
        surge: c.surge || "",
        loon: c.loon || "",
        udp: !!c.udp,
        cert: !!c.cert,
      };
    } catch {
      return { clash: "", surge: "", loon: "", udp: false, cert: false };
    }
  };

  const handleEdit = (sub: Sub) => {
    editorSection.value = "basic";
    const config = parseConfig(sub.Config);
    SubTitle.value = "编辑订阅";
    Subname.value = sub.Name;
    oldSubname.value = sub.Name;
    airportUrl.value = sub.SourceURLs || "";
    isAirportUrl.value = !!sub.SourceURLs;
    udpOn.value = !!config.udp;
    certOn.value = !!config.cert;
    Clash.value = config.clash;
    Surge.value = config.surge;
    Loon.value = config.loon;
    clashMode.value = config.clash.startsWith("./template/") ? "1" : "2";
    surgeMode.value = config.surge.startsWith("./template/") ? "1" : "2";
    loonMode.value = config.loon.startsWith("./template/") ? "1" : "2";
    const nodeNames = new Set(sub.Nodes.map((n) => n.Name));
    const orderedNames = (sub.NodeOrder || "")
      .split(",")
      .filter((name) => nodeNames.has(name));
    value1.value = [...new Set([...orderedNames, ...nodeNames])];
    selectedGroups.value = (sub.GroupRefs || []).map((g) => g.ID);
    editingSubId.value = sub.ID;
    resetPipeline(sub.Pipeline || "");
    dialogVisible.value = true;
  };

  // 移除已选节点
  const removeNode = (name: string) => {
    value1.value = value1.value.filter((n) => n !== name);
  };

  // 全选全部节点
  const selectAllNodes = () => {
    const s = new Set(value1.value);
    for (const n of NodesList.value) s.add(n.Name);
    value1.value = [...s];
  };
  const selectGroupNodes = (nodes: Node[]) => {
    const s = new Set(value1.value);
    nodes.forEach((n) => s.add(n.Name));
    value1.value = [...s];
  };

  // 一键清空已选节点
  const clearAllNodes = () => {
    value1.value = [];
  };

  // ---- 删除 ----
  const handleDel = async (sub: Sub) => {
    if (
      !(await confirmAction(
        "删除订阅",
        `删除「${sub.Name}」？订阅链接将停止服务。`,
        "删除订阅"
      ))
    )
      return;
    await DelSub({ id: sub.ID });
    await getsubs();
    ElMessage.success("删除成功");
  };
  const handleQrcode = (url: string, title: string) => {
    qrcode.value = url;
    qrTitle.value = title;
    qrDialog.value = true;
  };

  const openUrl = (url: string) => {
    window.open(url, "_blank", "noopener,noreferrer");
  };

  // ---- 重置链接 ----
  const handleReset = async (sub: Sub) => {
    if (
      !(await confirmAction(
        "重置订阅链接",
        `重置后「${sub.Name}」的旧链接立即失效。`,
        "重置 Token"
      ))
    )
      return;
    const { data } = await ResetToken({ id: sub.ID });
    sub.Token = data?.token || sub.Token;
    ElMessage.success("订阅链接已重置");
  };
  // ---- 过期 ----
  const fmtTime = formatDateTime;

  const previewNodesList = ref<Node[]>([]);

  // ---- 抽屉 ----
  const openDrawer = async (sub: Sub) => {
    abortProbes();
    const version = probeVersion;
    detailSection.value = "nodes";
    previewError.value = "";
    drawerSub.value = sub;
    drawerVisible.value = true;
    drawerLoading.value = true;
    expireClear.value = !sub.ExpiresAt;
    expirePick.value = sub.ExpiresAt || "";
    previewNodesList.value = [];

    try {
      const [ovRes, previewRes] = await Promise.all([
        overview.value.length === 0
          ? getNodeOverview()
          : Promise.resolve({ data: overview.value }),
        getSubPreviewNodes(sub.ID),
      ]);
      if (!alive || version !== probeVersion || !drawerVisible.value) return;
      if (overview.value.length === 0) overview.value = ovRes.data || [];
      previewNodesList.value = previewRes.data || [];

      drawerLoading.value = false;
      await testPreviewNodes();
    } catch {
      if (alive && version === probeVersion)
        previewError.value = "节点详情加载失败，请重试";
    } finally {
      if (alive && version === probeVersion) drawerLoading.value = false;
    }
  };

  // 将拉取到的所有真实节点按国家分组
  const drawerNodesByCountry = computed(() => {
    if (!previewNodesList.value.length) return [];
    // 取所有的名字
    const mapped = previewNodesList.value.map(
      (node) =>
        overview.value.find((o) => o.id === node.ID) || {
          id: node.ID,
          name: node.Name,
          server: "",
          country: "未知",
          countryCode: "",
          rtt: -3,
          groups: [],
          score: 0,
          availability: 0,
          confidence: 0,
          sampleCount: 0,
        }
    );
    // 按国家聚合
    const byCountry: Record<string, OverviewItem[]> = {};
    for (const n of mapped) {
      const key = n.country || "未知";
      if (!byCountry[key]) byCountry[key] = [];
      byCountry[key].push(n);
    }
    return Object.keys(byCountry)
      .sort()
      .map((c) => ({ country: c, items: byCountry[c] }));
  });

  const drawerGroupRefs = computed(() => {
    const sub = drawerSub.value;
    if (!sub) return [];
    return (sub.GroupRefs || []).map((g) => ({ ...g }));
  });

  const rttLabel = (rtt: number) =>
    rtt === -3
      ? "未检测"
      : rtt === -2
        ? "测试中…"
        : rtt < 0
          ? "不可达"
          : `${rtt}ms`;
  const expireSaving = ref(false);
  const saveExpire = async () => {
    if (
      !expireClear.value &&
      (!expirePick.value ||
        !Number.isFinite(new Date(expirePick.value).getTime()))
    ) {
      ElMessage.warning("请选择有效过期时间");
      return;
    }
    const sub = drawerSub.value;
    if (!sub) return;
    const expire = expireClear.value
      ? ""
      : String(Math.floor(new Date(expirePick.value).getTime() / 1000));
    expireSaving.value = true;
    try {
      await SetExpire({ id: sub.ID, expire });
      sub.ExpiresAt = expireClear.value
        ? null
        : new Date(expirePick.value).toISOString();
      ElMessage.success("过期时间已更新");
    } finally {
      expireSaving.value = false;
    }
  };

  const detailSection = ref<string | number>("nodes");
  const previewError = ref("");
  const confirmation = ref({
    visible: false,
    title: "",
    message: "",
    action: "",
  });
  let confirmationResolve: ((value: boolean) => void) | undefined;
  const confirmAction = (title: string, message: string, action: string) =>
    new Promise<boolean>((resolve) => {
      confirmationResolve?.(false);
      confirmation.value = { visible: true, title, message, action };
      confirmationResolve = resolve;
    });
  const resolveConfirmation = (value: boolean) => {
    confirmation.value.visible = false;
    confirmationResolve?.(value);
    confirmationResolve = undefined;
  };
  watch(
    () => confirmation.value.visible,
    (visible) => {
      if (!visible) {
        confirmationResolve?.(false);
        confirmationResolve = undefined;
      }
    }
  );
  const probeControllers = new Map<
    AbortController,
    { quality: OverviewItem; rtt: number }
  >();
  let probeVersion = 0;
  const abortProbes = () => {
    probeVersion++;
    probeControllers.forEach(({ quality, rtt }, controller) => {
      if (quality.rtt === -2) quality.rtt = rtt;
      controller.abort();
    });
    probeControllers.clear();
  };
  watch(drawerVisible, (visible) => {
    if (!visible) abortProbes();
  });
  onBeforeUnmount(abortProbes);
  const testPreviewNodes = async () => {
    const version = probeVersion;
    const nodes = previewNodesList.value.map((node) => ({
      node,
      quality: overview.value.find((o) => o.id === node.ID),
    }));
    let cursor = 0;
    const worker = async () => {
      while (
        cursor < nodes.length &&
        alive &&
        drawerVisible.value &&
        version === probeVersion
      ) {
        const { node, quality } = nodes[cursor++];
        const target = extractHostPort(node.Link);
        if (!target || !quality || quality.rtt === -1) continue;
        const controller = new AbortController();
        probeControllers.set(controller, { quality, rtt: quality.rtt });
        quality.rtt = -2;
        const started = performance.now();
        const timer = setTimeout(() => controller.abort(), 2000);
        try {
          await fetch(`https://${target.host}:${target.port}`, {
            mode: "no-cors",
            cache: "no-cache",
            signal: controller.signal,
          });
          if (alive && version === probeVersion)
            quality.rtt = Math.round(performance.now() - started);
        } catch (error) {
          if (alive && version === probeVersion)
            quality.rtt =
              (error as { name?: string } | null)?.name === "AbortError"
                ? -1
                : Math.round(performance.now() - started);
        } finally {
          clearTimeout(timer);
          probeControllers.delete(controller);
        }
      }
    };
    await Promise.all(
      Array.from({ length: Math.min(6, nodes.length) }, worker)
    );
  };
  const moveNode = (index: number, offset: number) => {
    const target = index + offset;
    if (target < 0 || target >= value1.value.length) return;
    const next = [...value1.value];
    [next[index], next[target]] = [next[target], next[index]];
    value1.value = next;
  };
  const withFeedback =
    <T extends unknown[]>(action: (...args: T) => unknown) =>
    async (...args: T) => {
      try {
        await action(...args);
      } catch {
        ElMessage.error("操作失败，请检查服务后重试");
      }
    };

  return {
    tableData,
    searchKey,
    Clash,
    Surge,
    Loon,
    clashMode,
    surgeMode,
    loonMode,
    SubTitle,
    Subname,
    oldSubname,
    airportUrl,
    isAirportUrl,
    dialogVisible,
    NodesList,
    value1,
    allGroups,
    selectedGroups,
    openedNodeGroups,
    nodeQuality,
    nodeQualityByName,
    nodeQualityLabel,
    groupedNodes,
    udpOn,
    certOn,
    qrcode,
    qrTitle,
    qrDialog,
    templist,
    editingSubId,
    artifactDrawer,
    artifactLoading,
    artifactError,
    artifactSub,
    artifactClient,
    artifactItems,
    artifactPointers,
    publishDialog,
    publishSub,
    publishForm,
    publishLoading,
    pipeline,
    emojiRulesText,
    pipelinePreview,
    pipelineLoading,
    parseEmojiRules,
    pipelineJSON,
    buildDialog,
    buildSub,
    buildClient,
    buildLoading,
    buildSubscription,
    runBuild: withFeedback(runBuild),
    loadArtifacts: withFeedback(loadArtifacts),
    openArtifacts: withFeedback(openArtifacts),
    lkgId,
    setLkg: withFeedback(setLkg),
    openSafePublish,
    runSafePublish: withFeedback(runSafePublish),
    shortLink,
    importDialog,
    importSub,
    importData,
    importLoading,
    openImport: withFeedback(openImport),
    makeShortLink: withFeedback(makeShortLink),
    onCardCommand,
    resetPipeline,
    runPipelinePreview: withFeedback(runPipelinePreview),
    drawerVisible,
    drawerLoading,
    drawerSub,
    overview,
    expirePick,
    expireClear,
    activeClient,
    clientOptions,
    statusFilter,
    filteredSubs,
    expiredCount,
    recentPublished,
    publicationErrors,
    publicationLoading,
    nodeCounts,
    subUrl,
    currentUrl,
    loading,
    loadError,
    metadataError,
    loadMetadata,
    saving,
    editorSection,
    addSubs: withFeedback(addSubs),
    handleAddSub,
    parseConfig,
    handleEdit,
    removeNode,
    selectAllNodes,
    selectGroupNodes,
    clearAllNodes,
    handleDel: withFeedback(handleDel),
    handleQrcode,
    openUrl,
    handleReset: withFeedback(handleReset),
    fmtTime,
    previewNodesList,
    openDrawer,
    drawerNodesByCountry,
    drawerGroupRefs,
    rttLabel,
    expireSaving,
    saveExpire: withFeedback(saveExpire),
    detailSection,
    previewError,
    confirmation,
    confirmAction,
    resolveConfirmation,
    abortProbes,
    testPreviewNodes,
    moveNode,
    getsubs,
  };
}
const key = Symbol("subscriptions");
export function provideSubscriptions() {
  const state = createSubscriptions();
  provide(key, state);
  return state;
}
export function useSubscriptions(): ReturnType<typeof createSubscriptions> {
  const state = inject<ReturnType<typeof createSubscriptions>>(key);
  if (!state) throw new Error("Missing subscriptions provider");
  return state;
}
