export function estimatedTraffic(
  rateMbps: number,
  duration: number,
  sources: number,
  direction: string
): number {
  return (
    ((Math.max(0, rateMbps) * 1e6) / 8) *
    Math.max(0, duration) *
    Math.max(0, sources) *
    (direction === "both" ? 2 : 1)
  );
}
export function curvePoints(
  intervals: { end: number; bitsPerSecond: number }[]
): string {
  if (!intervals.length) return "";
  const peak = Math.max(1, ...intervals.map((v) => v.bitsPerSecond));
  return intervals
    .map(
      (v, i) =>
        `${intervals.length === 1 ? 140 : Math.round((i / (intervals.length - 1)) * 280)},${Math.round(70 - (Math.max(0, v.bitsPerSecond) / peak) * 70)}`
    )
    .join(" ");
}
export function formatRate(value: number): string {
  if (!Number.isFinite(value)) return "—";
  return value >= 1e9
    ? `${(value / 1e9).toFixed(2)} Gbps`
    : `${(value / 1e6).toFixed(2)} Mbps`;
}
export function formatBytes(value: number): string {
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)} GB`;
  return `${(Math.max(0, value) / 1e6).toFixed(1)} MB`;
}
