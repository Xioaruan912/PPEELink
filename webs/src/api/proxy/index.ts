import request from "@/utils/request";

export type InboundTemplate =
  | "vless-reality-vision"
  | "vless-xhttp-reality"
  | "vless-grpc-reality"
  | "vless-ws-tls"
  | "vmess-ws-tls"
  | "vmess-tcp"
  | "trojan-reality"
  | "trojan-tls"
  | "shadowsocks-2022"
  | "hysteria2";

export interface DomainRoute {
  domain: string;
  match: "exact" | "suffix" | "category";
  egressNodeId: number;
}
export interface ProxyInbound {
  domainRoutes?: DomainRoute[];
  tlsMode?: "auto" | "manual" | "self-signed" | "";
  listen?: string;
  fingerprint?: string;
  spiderX?: string;
  privateKey?: string;
  tlsFingerprint?: string;
  egressNodeId?: number;
  tlsCertFile?: string;
  tlsKeyFile?: string;
  id: number;
  serverId: number;
  remark: string;
  template: InboundTemplate;
  port: number;
  portHopping?: string;
  target: string;
  sni: string;
  publicKey: string;
  shortId: string;
  path: string;
  host?: string;
  method?: string;
  enabled: boolean;
}
export interface ServerUnlockItem {
  key: string;
  name: string;
  status: "available" | "originals" | "blocked" | "unknown";
  region?: string;
  note?: string;
}

export interface ServerUnlockReport {
  ip?: string;
  country?: string;
  isp?: string;
  asn?: string;
  hosting?: boolean;
  checkedAt?: string;
  items: ServerUnlockItem[];
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
  unlock?: ServerUnlockReport;
  unlockCheckedAt?: string | null;
  terminalEnabled?: boolean;
  filesEnabled?: boolean;
  dockerEnabled?: boolean;
  allowAutoUpdate?: boolean;
  /** 程序完整性摘要；Agent 的展示版本独立递增。 */
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
  autoFirewall?: boolean;
  bbr?: string;
  qdisc?: string;
  rmemMax?: number;
  wmemMax?: number;
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
  cloudflare?: boolean;
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
  autoFirewall?: boolean;
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
export const tuneProxyServer = (id: number) =>
  request<{
    ok: boolean;
    output: string;
    bbr: string;
    qdisc: string;
    rmemMax: number;
    wmemMax: number;
  }>({
    url: "/api/v1/proxy/servers/tune",
    method: "post",
    data: { id },
    timeout: 30000,
  });
export const upgradeProxyCore = (id: number, version = "") =>
  request<{ ok: boolean; output: string }>({
    url: "/api/v1/proxy/servers/upgrade-core",
    method: "post",
    data: { id, version },
    timeout: 100000,
  });
export interface ParsedLogEntry {
  id: number;
  time: string;
  fullTime: string;
  level: "error" | "warn" | "info";
  module: string;
  message: string;
  hint?: string;
  count: number;
  raw: string;
}

export interface LogDiagnosticFinding {
  level: "error" | "warn" | "ok";
  title: string;
  detail: string;
  count: number;
}

export interface ParsedLogSummary {
  total: number;
  errors: number;
  warnings: number;
  infos: number;
  findings: LogDiagnosticFinding[];
}

export const getProxyServerLogs = (params: {
  id: number;
  source?: "xray" | "agent";
  filter?: string;
  lines?: number;
}) =>
  request<{
    logs: string;
    source: string;
    entries?: ParsedLogEntry[];
    summary?: ParsedLogSummary;
  }>({
    url: "/api/v1/proxy/servers/logs",
    method: "get",
    params,
    timeout: 25000,
  });
export const checkProxyServerPort = (
  serverId: number,
  port: number,
  proto: "tcp" | "udp"
) =>
  request<{ inUse: boolean; detail: string }>({
    url: "/api/v1/proxy/servers/check-port",
    method: "post",
    data: { serverId, port, proto },
    timeout: 15000,
  });

export interface ServerSNICheckItem {
  host: string;
  target: string;
  ok: boolean;
  tls13: boolean;
  h2: boolean;
  rttMs: number;
  issuer?: string;
  validDays?: number;
  score: number;
  grade: "best" | "ok" | "bad";
  reason?: string;
}

export const checkProxyServerSNI = (serverId: number, targets?: string[]) =>
  request<{ items: ServerSNICheckItem[] }>({
    url: "/api/v1/proxy/servers/check-sni",
    method: "post",
    data: { serverId, targets: targets?.filter(Boolean) || [] },
    timeout: 24000,
  });

export interface ServerSecurityClient {
  ip: string;
  conns: number;
  ports: number[];
}

export interface ServerSecurityAlert {
  ip: string;
  hits: number;
  lastReason: string;
  banned: boolean;
}

export interface ServerSecurityAudit {
  activeConns: number;
  uniqueClientIps: number;
  clients: ServerSecurityClient[];
  alerts: ServerSecurityAlert[];
  bannedIps: string[];
}

export const getProxyServerSecurity = (id: number) =>
  request<ServerSecurityAudit>({
    url: "/api/v1/proxy/servers/security",
    method: "get",
    params: { id },
    timeout: 24000,
  });

export const banProxyServerIP = (serverId: number, ip: string) =>
  request({
    url: "/api/v1/proxy/servers/security/ban",
    method: "post",
    data: { serverId, ip },
    timeout: 18000,
  });

export const unbanProxyServerIP = (serverId: number, ip: string) =>
  request({
    url: "/api/v1/proxy/servers/security/unban",
    method: "post",
    data: { serverId, ip },
    timeout: 18000,
  });

export const runProxyServerUnlockCheck = (data: {
  serverId?: number;
  all?: boolean;
}) =>
  request<{
    results: {
      serverId: number;
      serverName: string;
      ok: boolean;
      unlock?: ServerUnlockReport;
      error?: string;
    }[];
    okCount: number;
    total: number;
  }>({
    url: "/api/v1/proxy/servers/unlock-check",
    method: "post",
    data,
    timeout: 32000,
  });

export const migrateProxyServerURL = (data: {
  panelUrl: string;
  serverIds?: number[];
}) =>
  request<{
    results: {
      serverId: number;
      serverName: string;
      ok: boolean;
      message: string;
    }[];
    okCount: number;
    total: number;
  }>({
    url: "/api/v1/proxy/servers/migrate-url",
    method: "post",
    data,
    timeout: 26000,
  });

export const saveProxyServerPolicy = (data: {
  serverIds?: number[];
  terminalEnabled?: boolean;
  filesEnabled?: boolean;
  dockerEnabled?: boolean;
  allowAutoUpdate?: boolean;
  autoFirewall?: boolean;
  websiteTrafficEnabled?: boolean;
}) =>
  request<{ updated: number; pushed: number }>({
    url: "/api/v1/proxy/servers/policy",
    method: "post",
    data,
    timeout: 20000,
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
