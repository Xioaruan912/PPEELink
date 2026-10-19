/** 国内可达性检测：经节点连接国内各省三大运营商的服务器，只看连得上与否。 */
export interface ChinaTarget {
  province: string;
  city: string;
  isp: string;
  ip: string;
  port: number;
  ok: boolean;
  lat: number;
  lng: number;
}
export interface ProvinceGroup {
  province: string;
  items: ChinaTarget[];
  okCount: number;
}

/** 兼容旧缓存：早期结果只有 rtt（-1 表示失败）。 */
export function normalizeTarget(item: any): ChinaTarget {
  return {
    ...item,
    ok: typeof item.ok === "boolean" ? item.ok : Number(item.rtt) >= 0,
  };
}

export const reachLegend = ["全部可达", "部分可达", "不可达"];
/** 0 全部可达 / 1 部分可达 / 2 不可达 */
export function reachBand(okCount: number, total: number) {
  return okCount >= total && total > 0 ? 0 : okCount > 0 ? 1 : 2;
}
export function reachColor(
  okCount: number,
  total: number,
  token: (name: string) => string
) {
  return [token("--c-ok"), token("--c-warn"), token("--c-bad")][
    reachBand(okCount, total)
  ];
}
