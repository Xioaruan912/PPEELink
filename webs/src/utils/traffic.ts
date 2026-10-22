/** Human-readable byte size using binary units, e.g. 1.5 GB. */
export function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(
    Math.floor(Math.log(bytes) / Math.log(1024)),
    units.length - 1
  );
  const value = bytes / 1024 ** i;
  return `${value >= 100 || i === 0 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}
export interface TrafficInfo {
  TrafficUpload?: number;
  TrafficDownload?: number;
  TrafficTotal?: number;
  TrafficExpire?: string | null;
  TrafficUpdatedAt?: string | null;
}
/** Usage summary derived from a Subscription-Userinfo snapshot; null when the source never reported one. */
export function trafficOf(row: TrafficInfo) {
  if (!row.TrafficUpdatedAt) return null;
  const used = (row.TrafficUpload || 0) + (row.TrafficDownload || 0);
  const total = row.TrafficTotal || 0;
  const ratio = total > 0 ? Math.min(used / total, 1) : 0;
  const expire = row.TrafficExpire ? new Date(row.TrafficExpire) : null;
  const daysLeft = expire
    ? Math.ceil((expire.getTime() - Date.now()) / 86400000)
    : null;
  const state: "ok" | "warn" | "bad" =
    ratio >= 0.95 || (daysLeft !== null && daysLeft <= 0)
      ? "bad"
      : ratio >= 0.8 || (daysLeft !== null && daysLeft <= 7)
        ? "warn"
        : "ok";
  return {
    used,
    total,
    remaining: Math.max(total - used, 0),
    ratio,
    expire,
    daysLeft,
    state,
  };
}
/** 分享链接里的流量与到期快照。 */
export interface SourceTraffic {
  sourceUpload?: number;
  sourceDownload?: number;
  sourceTotal?: number;
  sourceExpire?: string | null;
  sourceTrafficAt?: string | null;
}
export function sourceTrafficOf(sub: SourceTraffic) {
  return trafficOf({
    TrafficUpload: sub.sourceUpload,
    TrafficDownload: sub.sourceDownload,
    TrafficTotal: sub.sourceTotal,
    TrafficExpire: sub.sourceExpire,
    TrafficUpdatedAt: sub.sourceTrafficAt,
  });
}
