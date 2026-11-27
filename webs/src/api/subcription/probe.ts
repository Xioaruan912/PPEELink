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
  publicIp?: string;
  reportPublicIp?: boolean;
  allowClash?: boolean;
  concurrency?: number;
  clashApi?: string;
  hasClashSecret?: boolean;
  clashGroup?: string;
  clashNow?: string;
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

export interface UserUnlockEgress {
  ip: string;
  country: string;
  loc: string;
  city?: string;
  colo?: string;
  isp?: string;
  asn?: string;
  hosting?: boolean;
  rtt: number;
  directIp?: string;
  matchedNode?: string;
}

export interface UserUnlockItem {
  key: string;
  name: string;
  group: string;
  ok: boolean;
  status: "available" | "originals" | "blocked" | "unknown";
  region?: string;
  note: string;
  evidence: string;
  latencyMs: number;
  checkedAt: string;
}

export interface UserUnlockResponse {
  jobId: string;
  probeName: string;
  egress: UserUnlockEgress;
  items: UserUnlockItem[];
  error?: string;
}

export interface UserSpeedSample {
  elapsedMs: number;
  mbps: number;
}

export interface UserSpeedChannel {
  mode: "proxy" | "direct";
  target?: string;
  downloadMbps: number;
  peakMbps: number;
  uploadMbps: number;
  ttfbMs: number;
  jitterMs: number;
  bytesDown: number;
  bytesUp: number;
  durationMs: number;
  samples: UserSpeedSample[];
  error?: string;
}

export interface UserSpeedtestResponse {
  jobId: string;
  probeName: string;
  mode: "proxy" | "direct" | "both";
  proxy?: UserSpeedChannel;
  direct?: UserSpeedChannel;
  retentionPct?: number;
  error?: string;
}

export interface BidirectionalTraceSide {
  hops?: UserTraceHop[];
  line?: string;
  error?: string;
}

export interface BidirectionalTraceResponse {
  nodeName: string;
  nodeHost: string;
  probeName: string;
  probeIsp?: string;
  clientIp?: string;
  returnServer?: string;
  outbound: BidirectionalTraceSide;
  returnPath: BidirectionalTraceSide;
  verdict: string;
}

export interface LocalDiagnoseResponse {
  available: boolean;
  reason?: string;
  probeName?: string;
  probeIsp?: string;
  hasIpv6?: boolean | null;
  ipv6Mismatch?: boolean;
  host?: string;
  port?: number;
  kind?: string;
  success?: boolean;
  rtt?: number;
  error?: string;
}

export function runUserAgentUnlock(probeId?: number) {
  return request<UserUnlockResponse>({
    url: "/api/v1/probes/unlock",
    method: "post",
    data: probeId ? { probeId } : {},
    timeout: 32000,
  });
}

export function runUserAgentSpeedtest(data: {
  probeId?: number;
  mode: "proxy" | "direct" | "both";
}) {
  return request<UserSpeedtestResponse>({
    url: "/api/v1/probes/speedtest",
    method: "post",
    data,
    timeout: 36000,
  });
}

export function runBidirectionalTrace(data: {
  nodeId: number;
  probeId?: number;
}) {
  return request<BidirectionalTraceResponse>({
    url: "/api/v1/probes/trace-bidirectional",
    method: "post",
    data,
    timeout: 32000,
  });
}

export function diagnoseNodeFromUserAgent(data: {
  nodeId: number;
  probeId?: number;
}) {
  return request<LocalDiagnoseResponse>({
    url: "/api/v1/probes/diagnose-local",
    method: "post",
    data,
    timeout: 16000,
  });
}

export function applyProbeSortToSubs(probeId?: number) {
  return request<{ updatedSubscriptions: number }>({
    url: "/api/v1/probes/apply-sort",
    method: "post",
    data: probeId ? { probeId } : {},
  });
}

export interface ClashStatus {
  connected: boolean;
  controller?: string;
  version?: string;
  group?: string;
  current?: string;
  proxies?: string[];
  error?: string;
}

export interface ClashNodeTestItem {
  name: string;
  egressIp?: string;
  country?: string;
  ttfbMs: number;
  downloadMbps: number;
  chatgpt: string;
  claude: string;
  gemini: string;
  netflix: string;
  disney: string;
  youtube: string;
  error?: string;
}

export interface ClashReport {
  jobId: string;
  probeName?: string;
  action: string;
  status?: ClashStatus;
  group?: string;
  original?: string;
  switched?: string;
  items?: ClashNodeTestItem[];
  error?: string;
}

export function getProbeClashStatus(probeId?: number) {
  return request<ClashReport>({
    url: "/api/v1/probes/clash/status",
    method: "post",
    data: probeId ? { probeId } : {},
    timeout: 14000,
  });
}

export function switchProbeClashNode(data: {
  probeId?: number;
  group?: string;
  node: string;
}) {
  return request<ClashReport>({
    url: "/api/v1/probes/clash/switch",
    method: "post",
    data,
    timeout: 14000,
  });
}

export interface NetDiagCheck {
  key: string;
  label: string;
  status: "ok" | "warn" | "fail";
  detail: string;
  advice?: string;
  rttMs?: number;
}

export interface NetDiagReport {
  summary: string;
  checkedAt: string;
  checks: NetDiagCheck[];
}

export interface ClashBatchStatus extends ClashReport {
  currentNode?: string;
  total?: number;
  done?: boolean;
  savedSnapshots?: number;
}

export function runProbeClashBatchTest(data: {
  probeId?: number;
  group?: string;
  nodes?: string[];
}) {
  return request<ClashReport>({
    url: "/api/v1/probes/clash/batch-test",
    method: "post",
    data,
    timeout: 130000,
  });
}

export function startProbeClashBatch(data: {
  probeId?: number;
  group?: string;
  nodes?: string[];
}) {
  return request<{ jobId: string; probeName: string }>({
    url: "/api/v1/probes/clash/batch/start",
    method: "post",
    data,
    timeout: 12000,
  });
}

export function getProbeClashBatchStatus(jobId: string) {
  return request<ClashBatchStatus>({
    url: "/api/v1/probes/clash/batch/status",
    method: "get",
    params: { jobId },
    timeout: 8000,
  });
}

export function runProbeNetDiag(probeId?: number) {
  return request<NetDiagReport>({
    url: "/api/v1/probes/net-diag",
    method: "post",
    data: { probeId },
    timeout: 15000,
  });
}

export interface ProbeTopologyOrigin {
  id: number;
  name: string;
  isp?: string;
  publicIp?: string;
  clashNow?: string;
  lat: number;
  lon: number;
}

export interface ProbeTopologyArc {
  probeId: number;
  nodeId: number;
  success: boolean;
  rtt: number;
  kind: string;
}

export interface ProbeTopologyAnomaly {
  tone: "warn" | "bad";
  title: string;
  target: string;
}

export interface ProbeTopologyResponse {
  origins: ProbeTopologyOrigin[];
  arcs: ProbeTopologyArc[];
  anomalies: ProbeTopologyAnomaly[];
}

export function getProbeTopology() {
  return request<ProbeTopologyResponse>({
    url: "/api/v1/probes/topology",
    method: "get",
    timeout: 12000,
  });
}

export function pushProbeConfig(data: {
  probeIds?: number[];
  panelUrl?: string;
  concurrency?: number;
  reportPublicIp?: boolean;
  allowClash?: boolean;
  clashApi?: string;
  clashSecret?: string;
}) {
  return request<{
    results: {
      probeId: number;
      probeName: string;
      ok: boolean;
      message: string;
    }[];
    okCount: number;
    total: number;
  }>({
    url: "/api/v1/probes/push-config",
    method: "post",
    data,
    timeout: 24000,
  });
}
