/**
 * 用户电脑上运行的 Clash（mihomo 内核）的外部控制接口。
 *
 * 延迟由用户本机的 Clash 经节点访问测试地址得出，与 Clash 界面里看到的数字一致。
 * 浏览器只和 127.0.0.1 上的控制接口通信；地址与密钥只存在当前浏览器，不会发给面板。
 */
export interface ClashController {
  url: string;
  secret: string;
  testUrl: string;
}

const STORAGE_KEY = "clash-controller";
export const DEFAULT_TEST_URL = "https://www.gstatic.com/generate_204";
export const CONTROLLER_PRESETS = [
  { label: "Clash Verge", url: "http://127.0.0.1:9097" },
  { label: "mihomo / FlClash", url: "http://127.0.0.1:9090" },
];

/** 延迟结果里表示“这个节点不在 Clash 当前配置中”。 */
export const RTT_NOT_IN_CLASH = -3;

export function loadController(): ClashController | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (typeof parsed?.url !== "string" || !parsed.url) return null;
    return {
      url: parsed.url,
      secret: typeof parsed.secret === "string" ? parsed.secret : "",
      testUrl: parsed.testUrl || DEFAULT_TEST_URL,
    };
  } catch {
    return null;
  }
}

export function saveController(c: ClashController) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(c));
  } catch {
    /* 隐私模式等情况下存不下，本次会话仍可使用 */
  }
}

export class ClashError extends Error {
  constructor(
    message: string,
    readonly kind: "unreachable" | "auth" | "http"
  ) {
    super(message);
  }
}

function normalizeUrl(url: string) {
  const trimmed = url.trim().replace(/\/+$/, "");
  return /^https?:\/\//.test(trimmed) ? trimmed : `http://${trimmed}`;
}

async function call(
  c: ClashController,
  path: string,
  { signal, timeout = 8000 }: { signal?: AbortSignal; timeout?: number } = {}
) {
  const controller = new AbortController();
  const abort = () => controller.abort();
  signal?.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, timeout);
  try {
    const res = await fetch(normalizeUrl(c.url) + path, {
      headers: c.secret ? { Authorization: `Bearer ${c.secret}` } : {},
      cache: "no-store",
      signal: controller.signal,
    });
    if (res.status === 401 || res.status === 403)
      throw new ClashError(
        "密钥不正确，请到 Clash 的外部控制设置里查看",
        "auth"
      );
    return res;
  } catch (e) {
    if (e instanceof ClashError) throw e;
    if (signal?.aborted) throw e;
    throw new ClashError(
      "连不上 Clash 的控制接口。请确认 Clash 正在运行、已开启外部控制且地址端口正确；浏览器询问是否允许访问本地网络时请选择允许。",
      "unreachable"
    );
  } finally {
    clearTimeout(timer);
    signal?.removeEventListener("abort", abort);
  }
}

/** 连接测试：返回内核版本。 */
export async function clashVersion(c: ClashController) {
  const res = await call(c, "/version");
  if (!res.ok) throw new ClashError(`控制接口返回 HTTP ${res.status}`, "http");
  const data = await res.json();
  return `${data.meta ? "mihomo " : ""}${data.version || "未知版本"}`;
}

/** Clash 当前配置里的全部代理名。 */
export async function clashProxyNames(c: ClashController) {
  const res = await call(c, "/proxies");
  if (!res.ok) throw new ClashError(`控制接口返回 HTTP ${res.status}`, "http");
  const data = await res.json();
  return new Set(Object.keys(data?.proxies || {}));
}

/** 让 Clash 经指定节点访问测试地址，返回毫秒；超时或失败返回 -1。 */
export async function clashDelay(
  c: ClashController,
  name: string,
  signal?: AbortSignal,
  timeoutMs = 5000
) {
  const query = new URLSearchParams({
    url: c.testUrl || DEFAULT_TEST_URL,
    timeout: String(timeoutMs),
  });
  const res = await call(
    c,
    `/proxies/${encodeURIComponent(name)}/delay?${query}`,
    { signal, timeout: timeoutMs + 3000 }
  );
  if (res.status === 404) return RTT_NOT_IN_CLASH;
  if (!res.ok) return -1;
  const data = await res.json();
  return typeof data?.delay === "number" && data.delay > 0 ? data.delay : -1;
}

/**
 * 依次测试多个节点（并发 6），onUpdate 回报每个结果。
 * 节点名不在 Clash 配置中的直接回报 RTT_NOT_IN_CLASH，不发起测试。
 */
export async function clashDelayAll(
  c: ClashController,
  names: string[],
  onUpdate: (index: number, rtt: number) => void,
  signal?: AbortSignal
) {
  const available = await clashProxyNames(c);
  let next = 0;
  const worker = async () => {
    while (!signal?.aborted && next < names.length) {
      const index = next++;
      if (!available.has(names[index])) {
        onUpdate(index, RTT_NOT_IN_CLASH);
        continue;
      }
      onUpdate(index, -2);
      let rtt = -1;
      try {
        rtt = await clashDelay(c, names[index], signal);
      } catch (e) {
        if (signal?.aborted) return;
        if (e instanceof ClashError && e.kind !== "http") throw e;
      }
      if (!signal?.aborted) onUpdate(index, rtt);
    }
  };
  await Promise.all(Array.from({ length: Math.min(6, names.length) }, worker));
}
