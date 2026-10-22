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
  lastSeenAt?: string | null;
  createdAt: string;
  online: boolean;
  latest?: MonitorMetric;
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
  };
  online: boolean;
  latest?: MonitorMetric;
  metrics: MonitorMetric[];
  range: string;
}

export function getMonitors() {
  return request<{
    items: MonitorItem[];
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

export function deleteMonitor(id: number) {
  return request({
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
