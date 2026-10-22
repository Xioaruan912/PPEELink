import request from "@/utils/request";

export type ProbePolicy = "panel" | "any" | "all" | "manual";

export interface Probe {
  id: number;
  name: string;
  region: string;
  enabled: boolean;
  online: boolean;
  lastSeenAt?: string | null;
  version?: string;
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

export function getProbes() {
  return request<Probe[]>({ url: "/api/v1/probes/list", method: "get" });
}

export function addProbe(data: { name: string; region: string }) {
  return request<{ id: number; name: string; token: string; install: string }>({
    url: "/api/v1/probes/add",
    method: "post",
    data,
  });
}

export function deleteProbe(id: number) {
  return request({
    url: "/api/v1/probes/delete",
    method: "post",
    data: { id },
  });
}

export function resetProbeToken(id: number) {
  return request<{ token: string; install: string }>({
    url: "/api/v1/probes/reset-token",
    method: "post",
    data: { id },
  });
}

export function setNodeProbePolicy(id: number, policy: ProbePolicy) {
  return request({
    url: "/api/v1/nodes/probe-policy",
    method: "post",
    data: { id, policy },
  });
}
