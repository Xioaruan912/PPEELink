import request from "@/utils/request";
export interface ThroughputSource {
  id: number;
  name: string;
  publicHost: string;
  available: boolean;
  version: string;
  error: string;
  seenAt?: string;
  prepareRequested: boolean;
  bidirSupported: boolean;
}
export type Direction = "forward" | "reverse" | "both" | "bidir";
export interface ThroughputSettings {
  landingId: number;
  sourceIds: number[];
  protocol: "tcp" | "udp";
  direction: Direction;
  family: 4 | 6;
  port: number;
  duration: number;
  streams: number;
  rateMbps: number;
}
export interface ThroughputRun {
  id: number;
  landingId: number;
  landingName: string;
  target: string;
  protocol: "tcp" | "udp";
  direction: Direction;
  family: 4 | 6;
  port: number;
  duration: number;
  streams: number;
  rateMbps: number;
  status: "running" | "completed" | "failed" | "cancelled";
  createdAt: string;
  finishedAt?: string;
}
export interface ThroughputInterval {
  start: number;
  end: number;
  bitsPerSecond: number;
  bytes: number;
  jitterMs: number;
  lossPercent: number;
}
export interface DirectionResult {
  bitsPerSecond: number;
  bytes: number;
  retransmits: number;
  rttMs?: number;
  jitterMs: number;
  lostPackets: number;
  packets: number;
  lossPercent: number;
  intervals: ThroughputInterval[];
}
export interface ThroughputResult {
  protocol: string;
  direction: string;
  duration: number;
  streams: number;
  rateMbps: number;
  capReached: boolean;
  progressIntervals?: ThroughputInterval[];
  progressDirection?: string;
  progressObservation?: string;
  forward?: DirectionResult;
  reverse?: DirectionResult;
  cpuHost: number;
  cpuRemote: number;
  warnings?: string[];
}
export interface ThroughputJob {
  id: number;
  runId: number;
  sourceId: number;
  sourceName: string;
  sequence: number;
  direction: "forward" | "reverse" | "bidir";
  status:
    | "queued"
    | "starting"
    | "running"
    | "completed"
    | "failed"
    | "cancelled"
    | "timeout";
  error: string;
  result?: ThroughputResult;
  startedAt?: string;
  finishedAt?: string;
}
export interface ThroughputDetail {
  run: ThroughputRun;
  jobs: ThroughputJob[];
}
const base = "/api/v1/network/throughput";
export const getThroughputSources = () =>
  request<ThroughputSource[]>({ url: base + "/sources", method: "get" });
export const prepareThroughput = (serverIds: number[]) =>
  request({ url: base + "/prepare", method: "post", data: { serverIds } });
export const getThroughputHistory = () =>
  request<ThroughputRun[]>({ url: base + "/list", method: "get" });
export const getThroughputDetail = (id: number) =>
  request<ThroughputDetail>({
    url: base + "/detail",
    method: "get",
    params: { id },
  });
export const createThroughput = (data: ThroughputSettings) =>
  request<ThroughputRun>({ url: base + "/create", method: "post", data });
export const cancelThroughput = (id: number) =>
  request({ url: base + "/cancel", method: "post", data: { id } });
