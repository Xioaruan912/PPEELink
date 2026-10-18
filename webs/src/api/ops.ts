import request from "@/utils/request";

export interface ApiToken {
  id: number;
  name: string;
  tokenPrefix: string;
  scopes: string;
  expiresAt?: string;
  lastUsedAt?: string;
  enabled: boolean;
  createdAt: string;
}
export interface AuditEntry {
  id: number;
  actor: string;
  authType: string;
  ip: string;
  method: string;
  path: string;
  status: number;
  action: string;
  createdAt: string;
}
export interface ShortLinkItem {
  id: number;
  code: string;
  target: string;
  remark: string;
  createdAt: string;
}

export const getApiTokens = () =>
  request<ApiToken[]>({ url: "/api/v1/tokens/list", method: "get" });
export const createApiToken = (data: {
  name: string;
  scopes: string[];
  expiresInDays: number;
}) => request({ url: "/api/v1/tokens/create", method: "post", data });
export const revokeApiToken = (id: number) =>
  request({ url: "/api/v1/tokens/revoke", method: "post", params: { id } });
export const inspectSafeBackup = () =>
  request({ url: "/api/v1/ops/backup/inspect", method: "get" });
export const exportSafeBackup = () =>
  request({ url: "/api/v1/ops/backup/export", method: "get" });
export const importSafeBackup = (data: unknown) =>
  request({ url: "/api/v1/ops/backup/import", method: "post", data });
export const getAuditLogs = () =>
  request<AuditEntry[]>({
    url: "/api/v1/audit/list",
    method: "get",
    params: { limit: 150 },
  });
export const getShortLinks = () =>
  request<ShortLinkItem[]>({ url: "/api/v1/shortlinks/list", method: "get" });
export const deleteShortLink = (id: number) =>
  request({
    url: "/api/v1/shortlinks/delete",
    method: "delete",
    params: { id },
  });

/** 下载完整备份（数据库快照 + 模板）。二进制流不经过 JSON 响应拦截器。 */
export async function downloadFullBackup() {
  const response = await fetch(
    `${import.meta.env.VITE_APP_BASE_API}/api/v1/ops/backup/full`,
    { headers: { Authorization: localStorage.getItem("accessToken") || "" } }
  );
  if (!response.ok) throw new Error(`备份失败 HTTP ${response.status}`);
  const disposition = response.headers.get("Content-Disposition") || "";
  const name =
    /filename=([^;]+)/.exec(disposition)?.[1] || "ppeelink-backup.tar.gz";
  const url = URL.createObjectURL(await response.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  URL.revokeObjectURL(url);
}
