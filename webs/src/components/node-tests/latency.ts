export interface ChinaTarget {
  province: string;
  city: string;
  isp: string;
  ip: string;
  port: number;
  rtt: number;
  lat: number;
  lng: number;
}
export interface ProvinceGroup {
  province: string;
  items: ChinaTarget[];
  okCount: number;
  avg: number;
}
export const latencyLegend = [
  "< 50 ms",
  "50–100 ms",
  "100–200 ms",
  "200–300 ms",
  "≥ 300 ms",
  "超时",
];
export function latencyBand(rtt: number) {
  return rtt < 0
    ? 5
    : rtt < 50
      ? 0
      : rtt < 100
        ? 1
        : rtt < 200
          ? 2
          : rtt < 300
            ? 3
            : 4;
}
export function latencyColor(rtt: number, token: (name: string) => string) {
  const colors = [
    token("--c-ok"),
    token("--c-data"),
    token("--c-warn"),
    token("--c-signal"),
    token("--c-bad"),
    token("--c-muted"),
  ];
  return colors[latencyBand(rtt)];
}
