import request from "@/utils/request";

export interface MonitorMetric {
  id: number;
  monitorId: number;
  collectedAt: string;
  cpuPercent: number;
  memUsed: number;
  memTotal: number;
  swapUsed: number;
  swapTotal: number;
  diskUsed: number;
  diskTotal: number;
  load1: number;
  load5: number;
  load15: number;
  uptime: number;
  netInSpeed: number;
  netOutSpeed: number;
  transferIn: number;
  transferOut: number;
  tcpConns: number;
  udpConns: number;
  processCount: number;
}

export interface MonitorItem {
  id: number;
  name: string;
  group: string;
  note: string;
  enabled: boolean;
  hidden: boolean;
  os: string;
  arch: string;
  version: string;
  /** 完整性校验值，仅用于升级确认，不作为展示版本号。 */
  agentHash?: string;
  agentVersion?: string;
  latestAgentVersion?: string;
  agentUpdateRequested?: boolean;
  agentUpdateError?: string;
  agentUpdateAvailable?: boolean;
  agentUpdateConfirmed?: boolean;
  agentUpgradeable?: boolean;
  ownsAgent?: boolean;
  removing?: boolean;
  lastSeenAt?: string | null;
  createdAt: string;
  online: boolean;
  sshConfigured: boolean;
  proxyServerId?: number | null;
  linked?: boolean;
  sshHost: string;
  sshPort: number;
  sshUser: string;
  latest?: MonitorMetric;
  check?: { total: number; success: number; checkedAt?: string | null };
}

export interface MonitorDetail {
  monitor: {
    id: number;
    name: string;
    group: string;
    note: string;
    os: string;
    arch: string;
    version: string;
    lastSeenAt?: string | null;
    sshHost: string;
    sshPort: number;
    sshUser: string;
  };
  online: boolean;
  sshConfigured: boolean;
  latest?: MonitorMetric;
  metrics: MonitorMetric[];
  range: string;
}

export function getMonitors() {
  return request<{
    items: MonitorItem[];
    agentVersion: string;
    summary: { total: number; online: number; offline: number };
  }>({ url: "/api/v1/monitors/list", method: "get" });
}

export function addMonitor(data: {
  name: string;
  group: string;
  note: string;
}) {
  return request<{ id: number; name: string; token: string; install: string }>({
    url: "/api/v1/monitors/add",
    method: "post",
    data,
  });
}

export function updateMonitorAgent(id: number, cancel = false) {
  return request({
    url: "/api/v1/monitors/update-agent",
    method: "post",
    data: { id, cancel },
  });
}

export function deleteMonitor(id: number) {
  return request<{ pending?: boolean }>({
    url: "/api/v1/monitors/delete",
    method: "post",
    data: { id },
  });
}

export function resetMonitorToken(id: number) {
  return request<{ token: string; install: string }>({
    url: "/api/v1/monitors/reset-token",
    method: "post",
    data: { id },
  });
}

export function getMonitorDetail(id: number, range = "24h") {
  return request<MonitorDetail>({
    url: "/api/v1/monitors/detail",
    method: "get",
    params: { id, range },
  });
}

export function saveMonitorSSH(data: {
  id: number;
  host: string;
  port: number;
  user: string;
  password: string;
}) {
  return request({
    url: "/api/v1/monitors/ssh/save",
    method: "post",
    data,
  });
}

export function getMonitorSSHTicket(id: number) {
  return request<{ ticket: string; expiresIn: number }>({
    url: "/api/v1/monitors/ssh/ticket",
    method: "post",
    data: { id },
  });
}
