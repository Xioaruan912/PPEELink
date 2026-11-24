import request from "@/utils/request";

export type ProbePolicy = "panel" | "any" | "all" | "manual";

export interface Probe {
  id: number;
  name: string;
  region: string;
  enabled: boolean;
  autoProbe?: boolean;
  hasIpv6?: boolean | null;
  isp?: string;
  iface?: string;
  online: boolean;
  lastSeenAt?: string | null;
  version?: string;
  latestVersion?: string;
  updateAvailable?: boolean;
}

export interface HistoryPoint {
  t: number;
  rtt: number;
  min?: number;
  max?: number;
  ok: boolean;
  loss: number;
}

export interface HistoryStats {
  current?: number | null;
  avg: number;
  min: number;
  max: number;
  jitter?: number;
  peakAvg?: number;
  offPeakAvg?: number;
  peakUptime?: number;
  uptime: number;
  samples: number;
}

export interface BestPeakSummary {
  nodeId: number;
  nodeName: string;
  avg: number;
  peakAvg?: number;
  jitter: number;
  uptime: number;
}

export interface PeakBand {
  start: number;
  end: number;
}

export interface SparklineSeries {
  points: HistoryPoint[];
  stats: HistoryStats;
}

export interface SparklinesResponse {
  from: number;
  to: number;
  peakBands?: PeakBand[];
  bestPeak?: BestPeakSummary | null;
  nodes: Record<string, SparklineSeries>;
}

export interface UserTraceHop {
  ttl: number;
  ip: string;
  rtt: number;
  timeout: boolean;
  asn?: string;
  isp?: string;
  location?: string;
  lineTag?: string;
  lineTone?: "ok" | "info" | "warn" | "muted" | "";
}

export interface UserTraceResponse {
  probeName: string;
  host: string;
  hops: UserTraceHop[];
  error?: string;
}

export interface ProbeHistorySeries {
  probeId: number;
  probeName: string;
  region: string;
  stats: HistoryStats;
  points: HistoryPoint[];
}

export interface NodeHistoryResponse {
  nodeId: number;
  range: "1h" | "6h" | "24h" | "7d";
  from: number;
  to: number;
  peakBands?: PeakBand[];
  series: ProbeHistorySeries[];
}

export interface ProbeStatus {
  id: number;
  name: string;
  region: string;
  success: boolean;
  rtt: number;
  error?: string;
  checkedAt?: string;
}

export interface ProbeCommands {
  windows: string;
  macos: string;
  linux: string;
}

export function getProbes() {
  return request<Probe[]>({ url: "/api/v1/probes/list", method: "get" });
}

export function addProbe(data: { name: string; region: string }) {
  return request<{
    id: number;
    name: string;
    token: string;
    install: string;
    commands?: ProbeCommands;
  }>({
    url: "/api/v1/probes/add",
    method: "post",
    data,
  });
}

export function deleteProbe(id: number) {
  return request<{ uninstalled?: boolean }>({
    url: "/api/v1/probes/delete",
    method: "post",
    data: { id },
    timeout: 15000,
  });
}

export function updateProbe(id: number) {
  return request({
    url: "/api/v1/probes/update",
    method: "post",
    data: { id },
    timeout: 20000,
  });
}

export function resetProbeToken(id: number) {
  return request<{
    token: string;
    install: string;
    commands?: ProbeCommands;
  }>({
    url: "/api/v1/probes/reset-token",
    method: "post",
    data: { id },
  });
}

export function triggerUserProbe(data?: {
  probeId?: number;
  nodeIds?: number[];
}) {
  return request<{
    jobId: string;
    completed: boolean;
    probeName: string;
    count: number;
  }>({
    url: "/api/v1/probes/trigger",
    method: "post",
    data: data || {},
    timeout: 25000,
  });
}

export function setNodeProbePolicy(id: number, policy: ProbePolicy) {
  return request({
    url: "/api/v1/nodes/probe-policy",
    method: "post",
    data: { id, policy },
  });
}

export function toggleProbeAuto(id: number, autoProbe: boolean) {
  return request<{ id: number; autoProbe: boolean }>({
    url: "/api/v1/probes/auto-toggle",
    method: "post",
    data: { id, autoProbe },
  });
}

export function getProbeSparklines(params?: {
  hours?: number;
  probeId?: number;
}) {
  return request<SparklinesResponse>({
    url: "/api/v1/probes/sparklines",
    method: "get",
    params,
  });
}

export function getProbeHistory(params: {
  nodeId: number;
  range?: "1h" | "6h" | "24h" | "7d";
  probeId?: number;
}) {
  return request<NodeHistoryResponse>({
    url: "/api/v1/probes/history",
    method: "get",
    params,
  });
}

export function traceNodeFromUserAgent(data: {
  nodeId: number;
  probeId?: number;
}) {
  return request<UserTraceResponse>({
    url: "/api/v1/probes/trace",
    method: "post",
    data,
    timeout: 30000,
  });
}

export function applyProbeSortToSubs(probeId?: number) {
  return request<{ updatedSubscriptions: number }>({
    url: "/api/v1/probes/apply-sort",
    method: "post",
    data: probeId ? { probeId } : {},
  });
}
