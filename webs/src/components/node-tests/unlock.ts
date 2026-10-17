export interface UnlockCheckResult {
  key: string;
  name: string;
  group: string;
  ok: boolean;
  rtt: number;
  note: string;
  region?: string;
  status?: string;
}
export interface UnlockResult {
  nodeName: string;
  ok: boolean;
  results: UnlockCheckResult[];
  error?: string;
}
