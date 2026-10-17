import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { ElMessage } from "element-plus";
import {
  getSubs,
  getSubPreviewNodes,
  subscriptionEgressPlan,
  subscriptionRuleExplain,
} from "@/api/subcription/subs";
import {
  EgressTest,
  getEgressTargets,
  getNodeOverview,
} from "@/api/subcription/node";
export type Target = {
  id?: number;
  key: string;
  name: string;
  domain: string;
  group: string;
  icon?: string;
  path?: string;
  method?: string;
  expectedStatus?: string;
  responseContains?: string;
  requireEgressIp?: boolean;
  timeoutSeconds?: number;
  retries?: number;
  enabled?: boolean;
  sortOrder?: number;
};
export type Result = Target & {
  status: "pending" | "testing" | "done" | "error";
  ip: string;
  countryCode: string;
  rtt: number;
  note: string;
};
export const groupLabels: Record<string, string> = {
  network: "网络",
  ai: "AI",
  social: "社交",
  content: "内容",
  finance: "金融",
  tools: "工具",
  developer: "开发",
  media: "媒体",
};
export function useEgressTest() {
  const targets = ref<Target[]>([]),
    results = ref<Result[]>([]);
  const mode = ref("subscription"),
    subscriptions = ref<any[]>([]),
    selectedSubscription = ref<number | null>(null);
  const subscriptionNodes = ref<any[]>([]),
    selectedNode = ref<number | null>(null);
  const nodesLoading = ref(false),
    initialLoading = ref(false),
    running = ref(false),
    planLoading = ref(false),
    explainLoading = ref(false);
  const lastTestedAt = ref<number>(),
    hideIP = ref(false),
    error = ref("");
  const plan = ref<any>(null),
    explainResult = ref<any>(null);
  const explainTarget = ref("gemini.google.com"),
    explainIP = ref(""),
    explainPort = ref<number | undefined>(443),
    explainProtocol = ref("tcp");
  const busy = computed(
    () => running.value || planLoading.value || explainLoading.value
  );
  let disposed = false,
    localVersion = 0,
    nodesVersion = 0;
  const controllers = new Set<AbortController>();
  const timers = new Set<number>();
  const resetResults = () => {
    results.value = targets.value
      .filter((item) => item.enabled !== false)
      .map((item) => ({
        ...item,
        status: "pending",
        ip: "",
        countryCode: "",
        rtt: -1,
        note: "",
      }));
  };
  async function loadTargets() {
    const { data } = await getEgressTargets();
    if (disposed) return;
    targets.value = data || [];
    resetResults();
    lastTestedAt.value = undefined;
  }
  async function loadSubscriptionNodes() {
    const version = ++nodesVersion,
      id = selectedSubscription.value;
    subscriptionNodes.value = [];
    selectedNode.value = null;
    error.value = "";
    plan.value = null;
    explainResult.value = null;
    resetResults();
    lastTestedAt.value = undefined;
    if (!id) {
      nodesLoading.value = false;
      return;
    }
    nodesLoading.value = true;
    try {
      const [preview, overview] = await Promise.all([
        getSubPreviewNodes(id),
        getNodeOverview(),
      ]);
      if (disposed || version !== nodesVersion) return;
      const quality = new Map(
        (overview.data || []).map((item: any) => [item.id, item])
      );
      subscriptionNodes.value = (preview.data || [])
        .map((item: any) => ({ ...item, quality: quality.get(item.ID) || {} }))
        .sort(
          (a: any, b: any) => (b.quality.score || 0) - (a.quality.score || 0)
        );
      selectedNode.value = subscriptionNodes.value[0]?.ID || null;
    } catch (cause: any) {
      if (!disposed && version === nodesVersion)
        error.value = cause.message || "订阅节点读取失败";
    } finally {
      if (!disposed && version === nodesVersion) nodesLoading.value = false;
    }
  }
  function maskIP(ip: string) {
    if (!hideIP.value || !ip) return ip || "未获取";
    return ip.includes(":")
      ? `${ip.split(":").slice(0, 2).join(":")}:****:****`
      : ip.replace(/\.\d+\.\d+$/, ".*.*");
  }
  const uniqueEgress = computed(() => {
    const groups = new Map<
      string,
      { ip: string; countryCode: string; services: string[] }
    >();
    results.value
      .filter((item) => item.status === "done" && item.ip)
      .forEach((item) => {
        const key = `${item.ip}|${item.countryCode}`;
        const found = groups.get(key);
        if (found) found.services.push(item.name);
        else
          groups.set(key, {
            ip: item.ip,
            countryCode: item.countryCode,
            services: [item.name],
          });
      });
    return [...groups.values()];
  });
  function statusAllowed(code: number, spec: string) {
    return (
      !spec.trim() ||
      spec.split(",").some((raw) => {
        const token = raw.trim();
        if (!token) return false;
        if (!token.includes("-")) return code === Number(token);
        const [low, high] = token.split("-", 2).map(Number);
        return (
          Number.isFinite(low) &&
          Number.isFinite(high) &&
          code >= low &&
          code <= high
        );
      })
    );
  }
  const current = (version: number) => !disposed && version === localVersion;
  async function detect(item: Result, version: number) {
    if (!current(version)) return;
    item.status = "testing";
    item.rtt = -1;
    item.note = "";
    const attempts = Math.max(1, Math.min(6, (item.retries || 0) + 1));
    for (let attempt = 0; attempt < attempts && current(version); attempt++) {
      const controller = new AbortController();
      controllers.add(controller);
      const timer = window.setTimeout(
        () => controller.abort(),
        Math.max(1, item.timeoutSeconds || 7) * 1000
      );
      timers.add(timer);
      const started = performance.now();
      try {
        const response = await fetch(
          `https://${item.domain}${item.path || "/cdn-cgi/trace"}?_=${Date.now()}`,
          {
            cache: "no-store",
            signal: controller.signal,
            method: item.method || "GET",
          }
        );
        if (!statusAllowed(response.status, item.expectedStatus || "200-399"))
          throw new Error(`HTTP ${response.status}`);
        const text = item.method === "HEAD" ? "" : await response.text();
        if (controller.signal.aborted)
          throw new DOMException("超时", "AbortError");
        if (
          item.responseContains &&
          !text.toLowerCase().includes(item.responseContains.toLowerCase())
        )
          throw new Error("响应未包含预期特征");
        const trace = Object.fromEntries(
          text
            .trim()
            .split("\n")
            .map((line) => {
              const index = line.indexOf("=");
              return index > 0
                ? [line.slice(0, index), line.slice(index + 1).trim()]
                : [line, ""];
            })
        );
        if (!trace.ip && item.requireEgressIp !== false)
          throw new Error("目标未返回出口 IP");
        if (!current(version)) return;
        item.ip = trace.ip || "";
        item.countryCode = (trace.loc || "").toUpperCase();
        item.rtt = Math.max(1, Math.round(performance.now() - started));
        item.note = item.ip ? "" : "站点可达，未提供出口 IP";
        item.status = "done";
        return;
      } catch (cause: any) {
        if (!current(version)) return;
        item.status = "error";
        item.note =
          cause?.name === "AbortError"
            ? "超时"
            : cause instanceof TypeError || cause?.name === "TypeError"
              ? "未开放跨域检测或网络不可达"
              : cause?.message || "目标未开放跨域检测";
      } finally {
        window.clearTimeout(timer);
        timers.delete(timer);
        controllers.delete(controller);
      }
    }
  }
  function cancelLocal() {
    localVersion++;
    controllers.forEach((controller) => controller.abort());
    controllers.clear();
    timers.forEach((timer) => window.clearTimeout(timer));
    timers.clear();
    running.value = false;
    results.value.forEach((item) => {
      if (item.status === "testing") {
        item.status = "error";
        item.note = "已停止";
      }
    });
  }
  async function runLocal() {
    if (busy.value) return;
    resetResults();
    running.value = true;
    const version = ++localVersion;
    let cursor = 0;
    try {
      const worker = async () => {
        while (current(version) && cursor < results.value.length)
          await detect(results.value[cursor++], version);
      };
      await Promise.all(Array.from({ length: 4 }, worker));
      if (current(version)) lastTestedAt.value = Date.now();
    } finally {
      if (current(version)) running.value = false;
    }
  }
  function mapResult(item: any, check: any, note = ""): Result {
    const target = targets.value.find(
      (target) => target.domain === item.domain
    );
    return {
      ...target,
      key: item.key || target?.key || item.domain,
      name: item.name,
      domain: item.domain,
      group: item.group || target?.group || "",
      status:
        check.status === "available" || check.status === "reachable"
          ? "done"
          : check.status
            ? "error"
            : "pending",
      ip: check.ip || "",
      countryCode: check.countryCode || "",
      rtt: check.rtt ?? -1,
      note: check.note || note,
    };
  }
  async function runSelected() {
    if (!selectedSubscription.value) {
      ElMessage.warning("请先选择订阅");
      return;
    }
    if (!selectedNode.value) {
      ElMessage.warning("该订阅没有可检测节点");
      return;
    }
    running.value = true;
    resetResults();
    results.value.forEach((item) => (item.status = "testing"));
    try {
      const { data } = await EgressTest({ id: selectedNode.value });
      if (!disposed) {
        results.value = (data?.results || []).map((item: any) =>
          mapResult(item, item, item.ip ? "" : "未获取出口 IP")
        );
        lastTestedAt.value = Date.now();
      }
    } catch (cause) {
      if (!disposed)
        results.value.forEach((item) => {
          item.status = "error";
          item.note = "检测请求失败";
        });
      throw cause;
    } finally {
      if (!disposed) running.value = false;
    }
  }
  async function runPlan() {
    if (!selectedSubscription.value) {
      ElMessage.warning("请先选择订阅");
      return;
    }
    planLoading.value = true;
    try {
      const { data } = await subscriptionEgressPlan(selectedSubscription.value);
      if (!disposed) {
        plan.value = data;
        if (data?.items)
          results.value = data.items.map((item: any) =>
            mapResult(
              item,
              item.result || {},
              item.fallback
                ? `未找到 ${item.expectedCountry}，已使用质量最优节点`
                : ""
            )
          );
        lastTestedAt.value = Date.now();
      }
    } finally {
      if (!disposed) planLoading.value = false;
    }
  }
  async function runExplain() {
    if (!selectedSubscription.value) {
      ElMessage.warning("请先选择订阅");
      return;
    }
    if (
      !explainTarget.value.trim() &&
      !explainIP.value.trim() &&
      !explainPort.value
    ) {
      ElMessage.warning("至少输入域名、IP 或端口之一");
      return;
    }
    explainLoading.value = true;
    explainResult.value = null;
    try {
      const { data } = await subscriptionRuleExplain({
        subscriptionId: selectedSubscription.value,
        target: explainTarget.value.trim(),
        ip: explainIP.value.trim(),
        port: explainPort.value || 0,
        protocol: explainProtocol.value,
      });
      if (!disposed) explainResult.value = data;
    } finally {
      if (!disposed) explainLoading.value = false;
    }
  }
  async function runCurrent() {
    if (busy.value || initialLoading.value || nodesLoading.value) return;
    error.value = "";
    try {
      if (mode.value === "subscription") await runSelected();
      else if (mode.value === "template") await runPlan();
      else if (mode.value === "explain") await runExplain();
      else await runLocal();
    } catch (cause: any) {
      if (!disposed) error.value = cause.message || "检测失败，请重试";
    }
  }
  watch(mode, () => {
    error.value = "";
    resetResults();
    lastTestedAt.value = undefined;
    plan.value = null;
    explainResult.value = null;
  });
  watch(selectedNode, () => {
    if (!busy.value) {
      resetResults();
      lastTestedAt.value = undefined;
    }
  });
  async function initialize() {
    initialLoading.value = true;
    error.value = "";
    const loaded = await Promise.allSettled([loadTargets(), getSubs()]);
    if (disposed) return;
    if (loaded[1].status === "fulfilled")
      subscriptions.value = loaded[1].value.data || [];
    else error.value = "订阅读取失败，请重试。";
    if (subscriptions.value.length) {
      selectedSubscription.value = subscriptions.value[0].ID;
      await loadSubscriptionNodes();
    }
    if (loaded[0].status === "rejected")
      error.value = "检测目标读取失败，请重试。";
    initialLoading.value = false;
  }
  onMounted(initialize);
  onBeforeUnmount(() => {
    disposed = true;
    nodesVersion++;
    cancelLocal();
  });
  return {
    targets,
    results,
    mode,
    subscriptions,
    selectedSubscription,
    subscriptionNodes,
    selectedNode,
    nodesLoading,
    initialLoading,
    running,
    busy,
    planLoading,
    explainLoading,
    lastTestedAt,
    hideIP,
    error,
    plan,
    explainResult,
    explainTarget,
    explainIP,
    explainPort,
    explainProtocol,
    uniqueEgress,
    maskIP,
    loadTargets,
    loadSubscriptionNodes,
    runCurrent,
    cancelLocal,
    initialize,
  };
}
