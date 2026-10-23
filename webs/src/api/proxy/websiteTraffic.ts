import request from "@/utils/request";
import { fromInputValue, hourFloor } from "@/utils/datetime";

export interface WebsiteTrafficQuery {
  serverId?: number;
  domain?: string;
  start: string;
  end: string;
  order?: "total" | "up" | "down";
  limit?: number;
}
export interface WebsiteTrafficData {
  items: {
    serverId: number;
    serverName: string;
    domain: string;
    up: number;
    down: number;
    total: number;
    rank: number;
  }[];
  summary: { up: number; down: number; total: number };
  start: string;
  end: string;
  granularity: "hour";
  scope: "proxy";
  unknownDomain: string;
}

/**
 * 起止时间一律按东八区（UTC+8）解释后再转 UTC，
 * 浏览器时区不再影响查询窗口。
 */
export function websiteTrafficParams(
  query: WebsiteTrafficQuery
): WebsiteTrafficQuery {
  const start = fromInputValue(query.start);
  const end = fromInputValue(query.end);
  if (start === null || end === null || start >= end)
    throw new Error("请选择有效时间范围，结束时间必须晚于开始时间。");
  if (start !== hourFloor(start) || end !== hourFloor(end))
    throw new Error(
      "起止时间必须对应整点（分钟、秒及毫秒为 0），东八区整点即 UTC 整点。"
    );
  if (end - start > 366 * 86400000)
    throw new Error("查询时间范围不能超过 366 天。");
  const limit = query.limit ?? 50;
  const order = query.order ?? "total";
  if (!Number.isInteger(limit) || limit < 1 || limit > 100)
    throw new Error("排行条数必须为 1 至 100。");
  if (!["total", "up", "down"].includes(order)) throw new Error("排序无效。");
  if (
    query.serverId !== undefined &&
    (!Number.isInteger(query.serverId) || query.serverId <= 0)
  )
    throw new Error("请选择有效节点。");
  return {
    ...(query.serverId === undefined ? {} : { serverId: query.serverId }),
    ...(query.domain?.trim() ? { domain: query.domain.trim() } : {}),
    start: new Date(start).toISOString(),
    end: new Date(end).toISOString(),
    order,
    limit,
  };
}

export const getWebsiteTraffic = (query: WebsiteTrafficQuery) =>
  request<WebsiteTrafficData>({
    url: "/api/v1/proxy/website-traffic",
    method: "get",
    params: websiteTrafficParams(query),
  });

/** Session-only snapshot: never persist website browsing statistics to browser storage. */
export class WebsiteTrafficState {
  status: "idle" | "loading" | "success" | "error" = "idle";
  data: WebsiteTrafficData | null = null;
  query: WebsiteTrafficQuery | null = null;
  updatedAt = "";
  private generation = 0;

  async load(query: WebsiteTrafficQuery, fetcher = getWebsiteTraffic) {
    const generation = ++this.generation;
    this.status = "loading";
    try {
      const params = websiteTrafficParams(query);
      const response = await fetcher(params);
      if (generation !== this.generation) return;
      this.data = response.data;
      this.query = params;
      this.updatedAt = new Date().toISOString();
      this.status = "success";
    } catch {
      if (generation === this.generation) this.status = "error";
    }
  }

  cancel() {
    this.generation++;
    if (this.status === "loading") this.status = this.data ? "success" : "idle";
  }
}
