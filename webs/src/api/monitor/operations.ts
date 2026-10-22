import axios from "axios";

export interface RemoteFile {
  name: string; path: string; size: number; mode: string; modified: number;
  kind: "file" | "directory" | "symlink" | "special"; target?: string;
}
export interface FileListing { path: string; parent: string; items: RemoteFile[]; truncated: boolean }
export interface DockerContainer { ID: string; Names: string; Image: string; State: string; Status: string; Ports: string }
export interface DockerStats { ID: string; Container: string; Name: string; CPUPerc: string; MemUsage: string; MemPerc: string; NetIO: string; BlockIO: string; PIDs: string }
export type DockerAction = "start" | "stop" | "restart";

// Isolated from global toast interceptors: workspace errors remain inline and actionable.
const client = axios.create({ baseURL: import.meta.env.VITE_APP_BASE_API, timeout: 50000 });
client.interceptors.request.use(config => {
  const token = localStorage.getItem("accessToken");
  if (token) config.headers.Authorization = token;
  return config;
});
client.interceptors.response.use(response => response, async error => {
  let body = error.response?.data;
  if (body instanceof Blob) { try { body = JSON.parse(await body.text()); } catch { body = null; } }
  return Promise.reject(new Error(body?.msg || (error.code === "ECONNABORTED" ? "操作超时，请检查服务器连接后重试。" : "无法连接服务器，请检查网络并重试。")));
});
const base = "/api/v1/monitors";
async function json<T>(operation: string, id: number, params: Record<string, unknown> = {}, data?: unknown, method = "get"): Promise<T> {
  const response = await client.request({ url: `${base}/${operation}`, method, params: { id, ...params }, data });
  if (response.data.code !== "00000") throw new Error(response.data.msg || "操作失败，请重试。");
  return response.data.data;
}
export const listFiles = (id: number, path: string) => json<FileListing>("files/list", id, { path });
export const readFile = (id: number, path: string) => json<{ path: string; content: string }>("files/read", id, { path });
export const mutateFile = (id: number, operation: "mkdir" | "rename" | "delete", path: string, destination?: string) => json(`files/${operation}`, id, { path }, { destination }, "post");
export async function writeFile(id: number, path: string, data: string | File, upload = false) {
  const response = await client.post(`${base}/files/${upload ? "upload" : "save"}`, data, { params: { id, path }, headers: { "Content-Type": "application/octet-stream" }, transformRequest: [value => value] });
  if (response.data.code !== "00000") throw new Error(response.data.msg || "保存失败。");
}
export async function downloadFile(id: number, path: string) {
  const response = await client.get(`${base}/files/download`, { params: { id, path }, responseType: "blob" });
  const url = URL.createObjectURL(response.data);
  const anchor = document.createElement("a"); anchor.href = url; anchor.download = path.split("/").pop() || "download";
  document.body.appendChild(anchor); anchor.click(); anchor.remove(); window.setTimeout(() => URL.revokeObjectURL(url), 1000);
}
export const listContainers = (id: number) => json<{ items: DockerContainer[] }>("docker/list", id);
export const containerStats = (id: number) => json<{ items: DockerStats[] }>("docker/stats", id);
export const containerLogs = (id: number, containerId: string) => json<{ content: string; truncated: boolean; lines: number }>("docker/logs", id, { containerId });
export const actOnContainer = (id: number, containerId: string, action: DockerAction) => json(`docker/${action}`, id, { containerId }, undefined, "post");
