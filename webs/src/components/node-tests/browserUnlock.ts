import type { UnlockService, UnlockCheckResult, UnlockResult } from "./unlock";

// Probe definitions/readers adapted from yt1988/unlock-check (MIT), e941ebd.
// See THIRD_PARTY_NOTICES.md. Each region belongs to its own target; no majority guess.
const evidence = (kind: string) =>
  ["openai", "dazn", "tvb"].includes(kind)
    ? "api"
    : kind === "trace"
      ? "region"
      : "reach";
const base = (s: UnlockService): UnlockCheckResult => ({
  key: s.key,
  name: s.name,
  group: s.group,
  ok: false,
  status: "unknown",
  evidence: evidence(s.kind),
  note: "响应不符合检测特征，无法判断",
  checkedAt: new Date().toISOString(),
});
const countryCode = (cc: unknown) =>
  typeof cc === "string" &&
  /^[A-Z]{2}$/.test(cc.toUpperCase()) &&
  !["XX", "ZZ"].includes(cc.toUpperCase())
    ? cc.toUpperCase()
    : undefined;

export function classifyBrowserResponse(
  s: UnlockService,
  code: number,
  text: string
): UnlockCheckResult {
  const r = base(s),
    lower = text.toLowerCase();
  let data: any;
  try {
    data = JSON.parse(text);
  } catch {
    /* An unreadable response stays unknown. */
  }
  if (s.kind === "openai" && data && lower.includes("unsupported_country"))
    return { ...r, status: "blocked", note: "服务接口明确拒绝此地区" };
  if (
    s.kind === "dazn" &&
    code === 403 &&
    lower.includes("security policy has been breached")
  )
    return { ...r, status: "blocked", note: "服务接口拒绝此出口" };
  if (code < 200 || code >= 300)
    return { ...r, note: `HTTP ${code}，无法判断` };
  if (
    [
      "just a moment",
      "cf-chl-",
      "challenge-platform",
      "verify you are human",
      "checking your browser",
    ].some((x) => lower.includes(x))
  )
    return { ...r, note: "目标要求浏览器验证，无法判断" };
  if (s.kind === "trace") {
    const cc = countryCode(/^loc=([^\r\n]+)$/m.exec(text)?.[1]?.trim());
    if (!cc) return r;
    const has = (list: string | undefined) => list?.split(/\s+/).includes(cc);
    if (!s.allow && !s.deny)
      return {
        ...r,
        status: "reachable",
        region: cc,
        note: "已取得目标地区，未配置地区可用性规则",
      };
    const allowed = (!s.allow || has(s.allow)) && !has(s.deny);
    return {
      ...r,
      region: cc,
      status: allowed ? "inferred" : "region_blocked",
      note: "根据该服务返回的地区及地区规则推测；未验证账号、风控或实际功能",
    };
  }
  let allowed: boolean | undefined, region: string | undefined;
  if (s.kind === "openai" && typeof data?.required === "boolean" && !data.error)
    allowed = true;
  if (s.kind === "tvb" && typeof data?.allow_in_this_country === "boolean") {
    allowed = data.allow_in_this_country;
    region = countryCode(data.country);
  }
  if (s.kind === "dazn" && data) {
    const m = /"isAllowed"\s*:\s*(true|false)/.exec(text);
    if (m) allowed = m[1] === "true";
    region = countryCode(
      /"GeolocatedCountry"\s*:\s*"([a-z]{2})"/i.exec(text)?.[1]
    );
  }
  if (allowed !== undefined)
    return {
      ...r,
      ok: allowed,
      status: allowed ? "available" : "blocked",
      region,
      note: "地区接口判定；账号、订阅与实际功能需另行验证",
    };
  // Browser cross-origin page loads do not prove login, streaming or AI generation.
  if (["reach", "netflix", "disney", "gemini"].includes(s.kind))
    return {
      ...r,
      status: "reachable",
      evidence: "reach",
      note: "站点可访问，未确认服务解锁或实际功能",
    };
  return r;
}

async function readBody(response: Response): Promise<string> {
  if (!response.body) return "";
  const reader = response.body.getReader(),
    decoder = new TextDecoder();
  let text = "",
    size = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) return text + decoder.decode();
      size += value.byteLength;
      if (size > 2 * 1024 * 1024) throw new Error("响应超过大小限制");
      text += decoder.decode(value, { stream: true });
    }
  } finally {
    await reader.cancel().catch(() => undefined);
    reader.releaseLock();
  }
}

async function ask(
  s: UnlockService,
  signal: AbortSignal,
  fetcher: typeof fetch
): Promise<UnlockCheckResult> {
  const r = base(s);
  const init: RequestInit = {
    signal,
    cache: "no-store",
    credentials: "omit",
    referrerPolicy: "no-referrer",
  };
  let response: Response;
  try {
    response = await fetcher(s.url, {
      ...init,
      mode: "cors",
      method: s.method || "GET",
      ...(s.body
        ? { body: s.body, headers: { "Content-Type": "application/json" } }
        : {}),
    });
  } catch {
    if (signal.aborted) return { ...r, note: "请求已取消或超时" };
    try {
      const reachable = await fetcher(s.reachUrl || s.url, {
        ...init,
        mode: "no-cors",
        method: "GET",
      });
      await reachable.body?.cancel();
      if (!signal.aborted && (reachable.type === "opaque" || reachable.ok))
        return {
          ...r,
          status: "reachable",
          evidence: "reach",
          note: "浏览器仅确认连接到目标；HTTP 状态不可见，未确认解锁",
        };
    } catch {
      /* CORS, blocking and network errors cannot establish a service verdict. */
    }
    return {
      ...r,
      note: signal.aborted
        ? "请求已取消或超时"
        : "连接失败或浏览器限制，无法判断",
    };
  }
  try {
    return classifyBrowserResponse(
      s,
      response.status,
      await readBody(response)
    );
  } catch {
    return {
      ...r,
      note: signal.aborted ? "请求已取消或超时" : "响应读取失败，无法判断",
    };
  }
}

export async function runBrowserUnlock(
  services: UnlockService[],
  signal: AbortSignal,
  onResult: (result: UnlockCheckResult) => void,
  fetcher: typeof fetch = fetch
): Promise<UnlockResult> {
  const controller = new AbortController();
  let expired = false,
    next = 0;
  const stop = () => controller.abort();
  signal.addEventListener("abort", stop, { once: true });
  if (signal.aborted) stop();
  const timer = setTimeout(() => {
    expired = true;
    stop();
  }, 45000);
  const results = services.map((s) => ({
    ...base(s),
    note: "检测已停止或超时，未完成",
  }));
  try {
    await Promise.all(
      Array.from({ length: Math.min(4, services.length) }, async () => {
        while (next < services.length && !controller.signal.aborted) {
          const i = next++,
            own = new AbortController();
          const abort = () => own.abort();
          controller.signal.addEventListener("abort", abort, { once: true });
          if (controller.signal.aborted) abort();
          const timeout = setTimeout(abort, 8000);
          try {
            const r = await ask(services[i], own.signal, fetcher);
            results[i] = r;
            if (!controller.signal.aborted) onResult(r);
          } finally {
            clearTimeout(timeout);
            own.abort();
            controller.signal.removeEventListener("abort", abort);
          }
        }
      })
    );
    return {
      source: "browser",
      nodeName: "当前设备",
      results,
      ok: results.some((r) => r.ok),
      complete: !controller.signal.aborted,
      ...(controller.signal.aborted
        ? {
            error: expired
              ? "检测已超时，未完成项目无法判断"
              : "检测已停止，未完成项目无法判断",
          }
        : {}),
    };
  } finally {
    clearTimeout(timer);
    signal.removeEventListener("abort", stop);
  }
}

export function unlockLabel(
  item: Pick<UnlockCheckResult, "status" | "ok" | "evidence">
): string {
  if (!item.evidence) return "历史结果，请重新检测";
  switch (item.status) {
    case "available":
      return item.evidence === "page" ? "页面可用" : "接口通过";
    case "blocked":
      return "明确受限";
    case "inferred":
      return "推测可用";
    case "region_blocked":
      return "推测受限";
    case "reachable":
      return "仅可访问";
    default:
      return "无法判断";
  }
}
export function unlockState(
  item: Pick<UnlockCheckResult, "status" | "ok" | "evidence">
): "ok" | "warn" | "bad" {
  if (!item.evidence) return "warn";
  return item.status === "available"
    ? "ok"
    : item.status === "blocked"
      ? "bad"
      : "warn";
}
export const unlockEvidenceLabel: Record<string, string> = {
  api: "官方接口",
  page: "页面特征",
  region: "目标服务地区推测",
  reach: "连接可达性",
};
