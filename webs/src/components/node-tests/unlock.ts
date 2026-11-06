export interface UnlockService {
  key: string;
  name: string;
  group: string;
  kind: string;
  url: string;
  reachUrl?: string;
  method?: string;
  body?: string;
  allow?: string;
  deny?: string;
}
export interface UnlockCheckResult {
  key: string;
  name: string;
  group: string;
  ok: boolean;
  note: string;
  region?: string;
  status?: string;
  evidence?: string;
  checkedAt?: string;
}
export interface UnlockResult {
  nodeName: string;
  source?: "node" | "browser";
  complete?: boolean;
  ok: boolean;
  results: UnlockCheckResult[];
  error?: string;
}
export const unlockGroups = [
  { key: "all", label: "全部常用服务" },
  { key: "ai", label: "AI 服务" },
  { key: "video", label: "影视流媒体" },
  { key: "local", label: "地区电视" },
  { key: "forum", label: "社交 / 其它" },
];
