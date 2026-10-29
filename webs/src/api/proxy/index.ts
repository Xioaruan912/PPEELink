import request from "@/utils/request";

export type InboundTemplate =
  "vless-reality-vision" | "vless-xhttp-reality" | "hysteria2";

export interface DomainRoute {
  domain: string;
  match: "exact" | "suffix" | "category";
  egressNodeId: number;
}
export interface ProxyInbound {
  domainRoutes?: DomainRoute[];
  tlsMode?: "auto" | "manual" | "";
  tlsFingerprint?: string;
  egressNodeId?: number;
  tlsCertFile?: string;
  tlsKeyFile?: string;
  id: number;
  serverId: number;
  remark: string;
  template: InboundTemplate;
  port: number;
  target: string;
  sni: string;
  publicKey: string;
  shortId: string;
  path: string;
  enabled: boolean;
}
export interface ProxyServer {
  id: number;
  name: string;
  kind: "local" | "agent";
  publicHost: string;
  enabled: boolean;
  xrayVersion: string;
  agentVersion: string;
  proxyFeatures?: number;
  /** 统一 Agent 上报的程序摘要（有值时以它为准，语义化版本号已废弃）。 */
  agentHash?: string;
  arch: string;
  running: boolean;
  lastError: string;
  lastSeenAt: string | null;
  appliedRev: string;
  removing: boolean;
  trafficLimit: number;
  resetDay: number;
  monitorEnabled: boolean;
  websiteTrafficEnabled?: boolean;
  agentUpdateRequested: boolean;
  /** 本计费周期流量；over 为 true 时 xray 已停止、节点暂不下发，resetAt 恢复 */
  quota: { used: number; limit: number; over: boolean; resetAt: string };
  inbounds: ProxyInbound[];
  online: boolean;
  todayUp: number;
  todayDown: number;
  users: number;
}
export interface ProxyAccount {
  userId: number;
  username: string;
  nickname: string;
  enabled: boolean;
  uuid: string;
  trafficLimit: number;
  trafficUsed: number;
  resetDay: number;
  expireAt: string | null;
  deviceLimit: number;
  blockedUntil: string | null;
  blockedReason: string;
  onlineIps: number;
  todayBytes: number;
  active: boolean;
}
export interface TargetCheck {
  ok: boolean;
  tls13: boolean;
  h2: boolean;
  latency: number;
  message: string;
}
export interface DailyTraffic {
  day: string;
  serverId: number;
  up: number;
  down: number;
}
export interface SelfHosted {
  enabled: boolean;
  active?: boolean;
  links?: Record<string, string>;
  imports?: Record<string, string>;
  used?: number;
  limit?: number;
  expireAt?: string | null;
  blockedReason?: string;
  nodes?: number;
}

export const listProxyServers = () =>
  request<ProxyServer[]>({ url: "/api/v1/proxy/servers", method: "get" });
export const saveProxyServer = (data: {
  id?: number;
  name: string;
  kind?: "local" | "agent";
  publicHost: string;
  enabled: boolean;
  trafficLimitGb?: number;
  resetDay?: number;
  monitorEnabled?: boolean;
  websiteTrafficEnabled?: boolean;
}) =>
  request<{ id: number; token?: string; install?: string }>({
    url: "/api/v1/proxy/servers/save",
    method: "post",
    data,
  });
export const resetServerToken = (id: number) =>
  request<{ token: string; install: string }>({
    url: "/api/v1/proxy/servers/reset-token",
    method: "post",
    params: { id },
  });
/** 删除服务器；force 跳过等待 agent 卸载。返回 data.pending（等待卸载）或 data.uninstall（需手动执行的卸载命令）。 */
export const requestAgentUpdate = (id: number, cancel = false) =>
  request<{ requested: boolean }>({
    url: "/api/v1/proxy/servers/update-agent",
    method: "post",
    params: cancel ? { id, cancel: 1 } : { id },
  });
export const deleteProxyServer = (id: number, force = false) =>
  request({
    url: "/api/v1/proxy/servers/delete",
    method: "delete",
    params: force ? { id, force: 1 } : { id },
  });
export const saveProxyInbound = (
  data: Partial<ProxyInbound> & { regenerateKey?: boolean }
) =>
  request<ProxyInbound>({
    url: "/api/v1/proxy/inbounds/save",
    method: "post",
    data,
  });
export const deleteProxyInbound = (id: number) =>
  request({
    url: "/api/v1/proxy/inbounds/delete",
    method: "delete",
    params: { id },
  });
export const checkRealityTarget = (target: string, sni: string) =>
  request<TargetCheck>({
    url: "/api/v1/proxy/check-target",
    method: "post",
    data: { target, sni },
  });
export const listProxyAccounts = () =>
  request<ProxyAccount[]>({ url: "/api/v1/proxy/accounts", method: "get" });
export const saveProxyAccount = (data: {
  userId: number;
  enabled: boolean;
  trafficLimit: number;
  resetDay: number;
  expireAt: string | null;
  deviceLimit: number;
  resetUsage?: boolean;
  regenerate?: boolean;
  unblock?: boolean;
}) => request({ url: "/api/v1/proxy/accounts/save", method: "post", data });
export const getProxyTraffic = (days = 30) =>
  request<DailyTraffic[]>({
    url: "/api/v1/proxy/traffic",
    method: "get",
    params: { days },
  });
export const getMySelfHosted = () =>
  request<SelfHosted>({ url: "/api/v1/my/self-hosted", method: "get" });

export const getProxyEgressOptions = () =>
  request<any, { data: { id: number; name: string }[] }>({
    url: "/api/v1/proxy/egress-options",
    method: "get",
  });

export type RelayNetwork = "tcp" | "udp" | "both";
export interface ProxyRelay {
  id: number;
  serverId: number;
  name: string;
  listenPort: number;
  network: RelayNetwork;
  targetNodeId: number;
  enabled: boolean;
  nodeId: number;
  revision: string;
  appliedRevision: string;
  running: boolean;
  lastError: string;
  lastSeenAt: string | null;
  up: number;
  down: number;
  tcpConnections: number;
  udpSessions: number;
}
export interface RelayTarget {
  id: number;
  name: string;
  protocol: "Reality" | "HY2";
  network: "tcp" | "udp";
}
export type RelaySave = Pick<
  ProxyRelay,
  "serverId" | "name" | "listenPort" | "network" | "targetNodeId" | "enabled"
> & { id?: number };
export const listProxyRelays = () =>
  request<ProxyRelay[]>({ url: "/api/v1/proxy/relays", method: "get" });
export const saveProxyRelay = (data: RelaySave) =>
  request<ProxyRelay>({
    url: "/api/v1/proxy/relays/save",
    method: "post",
    data,
  });
export const deleteProxyRelay = (id: number) =>
  request({
    url: "/api/v1/proxy/relays/delete",
    method: "delete",
    params: { id },
  });
export const listRelayTargets = () =>
  request<RelayTarget[]>({ url: "/api/v1/proxy/relay-targets", method: "get" });
