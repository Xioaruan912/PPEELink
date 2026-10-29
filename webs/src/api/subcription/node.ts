import request from "@/utils/request";
export function getNodes(includeHidden = false) {
  return request({
    url: "/api/v1/nodes/get",
    method: "get",
    params: includeHidden ? { include_hidden: 1 } : undefined,
  });
}

// 节点概览（国家、分组与有效性）；refresh 为 true 时立即重新检测
export function getNodeOverview(refresh = false) {
  return request({
    url: "/api/v1/nodes/overview",
    method: "get",
    params: refresh ? { refresh: 1 } : undefined,
  });
}

// 有效 / 无效节点数
export function getNodeQualitySummary() {
  return request({
    url: "/api/v1/nodes/quality/summary",
    method: "get",
  });
}

export interface CountryOption {
  code: string;
  name: string;
  flag: string;
  aliases: string[];
}
// 国家/地区列表：命名时选择国旗
export function getCountries() {
  return request({ url: "/api/v1/nodes/countries", method: "get" });
}
export function getNodeHealthEvents(limit = 30) {
  return request({
    url: "/api/v1/nodes/health/events",
    method: "get",
    params: { limit },
  });
}
export function getAlertSetting() {
  return request({ url: "/api/v1/nodes/alerts", method: "get" });
}
export function updateAlertSetting(
  data: Record<string, string | number | boolean>
) {
  const form = new FormData();
  Object.entries(data).forEach(([key, value]) => {
    if (value !== undefined && value !== null) form.append(key, String(value));
  });
  return request({
    url: "/api/v1/nodes/alerts",
    method: "post",
    data: form,
  });
}

// 节点解锁测试
export function UnlockTest(data: any) {
  return request({
    url: "/api/v1/nodes/unlock",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}
export function getEgressTargets() {
  return request({ url: "/api/v1/nodes/egress-targets", method: "get" });
}
export function saveEgressTarget(data: any) {
  return request({ url: "/api/v1/nodes/egress-targets", method: "post", data });
}
export function deleteEgressTarget(id: number) {
  return request({
    url: "/api/v1/nodes/egress-targets",
    method: "delete",
    params: { id },
  });
}

// 中国各地延迟测试
export function ChinaPingTest(data: any) {
  return request({
    url: "/api/v1/nodes/chinaping",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}

// 当前测试状态
export function GetTestStatus() {
  return request({
    url: "/api/v1/nodes/test/status",
    method: "get",
  });
}
// 停止当前测试
export function CancelTest() {
  return request({
    url: "/api/v1/nodes/test/cancel",
    method: "post",
  });
}

export function AddNodes(data: any) {
  return request({
    url: "/api/v1/nodes/add",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}
export function UpdateNode(data: any) {
  return request({
    url: "/api/v1/nodes/update",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}
export function DelNode(data: any) {
  return request({
    url: "/api/v1/nodes/delete",
    method: "delete",
    params: data,
  });
}
// 获取全部分组
export function GetGroup() {
  return request({
    url: "/api/v1/nodes/group/get",
    method: "get",
  });
}
// 获取分组完整信息（含 ID/节点数）
export function GetGroupFull(includeHidden = false) {
  return request({
    url: "/api/v1/nodes/group/full",
    method: "get",
    params: includeHidden ? { include_hidden: 1 } : undefined,
  });
}
export function SetNodeDelivery(id: number, enabled: boolean) {
  return request({
    url: "/api/v1/nodes/delivery",
    method: "post",
    data: { id, enabled },
  });
}
// 即使检测无效也始终下发
export function SetNodeForceDeliver(id: number, forceDeliver: boolean) {
  return request({
    url: "/api/v1/nodes/deliver",
    method: "post",
    data: { id, forceDeliver },
  });
}
// 设置关联分组
export function SetGroup(data: any) {
  return request({
    url: "/api/v1/nodes/group/set",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}
// 解除节点与分组绑定
export function UnbindGroup(data: any) {
  return request({
    url: "/api/v1/nodes/group/unbind",
    method: "post",
    data,
    headers: {
      "Content-Type": "multipart/form-data",
    },
  });
}
// 删除分组

export function DelGroup(data: any) {
  return request({
    url: "/api/v1/nodes/group/delete",
    method: "delete",
    params: data,
  });
}

export type NodeBatchAction = "rename" | "emoji-add" | "emoji-remove" | "flags";
export interface NodeChange {
  id: number;
  before: string;
  after: string;
}
/** 批量改名 / 加减国旗 / 开关；preview 为真时只返回改动预览。 */
export function nodeBatch(data: {
  ids: number[];
  action: NodeBatchAction;
  pattern?: string;
  replacement?: string;
  flags?: { udp: boolean; tfo: boolean; skipCertVerify: boolean };
  preview?: boolean;
}) {
  return request<{ changes: NodeChange[] }>({
    url: "/api/v1/nodes/batch",
    method: "post",
    data,
  });
}
export interface DedupeGroup {
  keep: { ID: number; Name: string };
  removed: { ID: number; Name: string }[];
}
/** 按“协议 + 服务器 + 端口”去重，保留最早的节点并转移分组与订阅引用。 */
export function nodeDedupe(preview: boolean) {
  return request<{ groups: DedupeGroup[]; removed: number }>({
    url: "/api/v1/nodes/dedupe",
    method: "post",
    data: { preview },
  });
}
/** 为单个节点生成限时订阅链接。 */
export function nodeTempLink(nodeId: number, hours: number) {
  return request<{
    links: Record<string, string>;
    expiresAt: string;
    name: string;
  }>({
    url: "/api/v1/nodes/temp-link",
    method: "post",
    data: { nodeId, hours },
  });
}

/** 暂停下发到 until（YYYY-MM-DD，当天恢复）；until 为空表示立即恢复下发。 */
export function pauseNodes(ids: number[], until: string, note = "") {
  return request({
    url: "/api/v1/nodes/pause",
    method: "post",
    data: { ids, until, note },
    headers: { "Content-Type": "application/json" },
  });
}
