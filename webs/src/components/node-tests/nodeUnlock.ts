import type { UnlockResult, UnlockCheckResult } from "./unlock";
import { readEvents, token } from "./useNodeTestSession";

export async function requestNodeUnlock(
  id: number,
  group: string,
  signal: AbortSignal,
  onResult: (row: UnlockCheckResult) => void
): Promise<UnlockResult> {
  const own = new AbortController();
  const abort = () => own.abort();
  signal.addEventListener("abort", abort, { once: true });
  if (signal.aborted) abort();
  const timer = setTimeout(abort, 50000);
  try {
    const response = await fetch(
      import.meta.env.VITE_APP_BASE_API + "/api/v1/nodes/unlock",
      {
        method: "POST",
        signal: own.signal,
        headers: {
          Accept: "text/event-stream",
          "Content-Type": "application/x-www-form-urlencoded",
          Authorization: token(),
        },
        body: new URLSearchParams({ id: String(id), group }),
      }
    );
    if (!response.ok) {
      const data = await response.json().catch(() => null);
      throw new Error(data?.msg || "节点检测失败");
    }
    if (!response.headers.get("content-type")?.includes("text/event-stream")) {
      const data = await response.json();
      if (data?.code !== "00000" || !Array.isArray(data?.data?.results))
        throw new Error(data?.msg || "节点检测响应无效");
      return data.data;
    }
    let result: UnlockResult | undefined;
    await readEvents(
      response,
      (type, data) => {
        if (type === "error") throw new Error(data?.msg || "节点检测失败");
        if (type === "item" && typeof data?.key === "string") onResult(data);
        if (type === "done" && Array.isArray(data?.results)) result = data;
      },
      () => !own.signal.aborted,
      () => undefined
    );
    if (!result)
      throw new Error(
        own.signal.aborted
          ? "检测已停止或超时"
          : "连接中断，检测未完成；已收到的结果仍保留"
      );
    return result;
  } catch (cause) {
    if (own.signal.aborted && !signal.aborted)
      throw new Error("检测连接已超时，请重试。");
    throw cause;
  } finally {
    clearTimeout(timer);
    own.abort();
    signal.removeEventListener("abort", abort);
  }
}
