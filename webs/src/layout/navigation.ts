import type { Component } from "vue";
import type { RouteRecordRaw } from "vue-router";
import { translateRouteTitle } from "@/utils/i18n";
import IconHouse from "~icons/ph/house-simple";
import IconLink from "~icons/ph/link-simple";
import IconStack from "~icons/ph/stack";
import IconNodes from "~icons/ph/graph";
import IconAirplane from "~icons/ph/airplane-tilt";
import IconDevices from "~icons/ph/devices";
import IconGauge from "~icons/ph/gauge";
import IconLock from "~icons/ph/lock-key-open";
import IconPulse from "~icons/ph/pulse";
import IconFiles from "~icons/ph/files";
import IconSliders from "~icons/ph/sliders-horizontal";
import IconBooks from "~icons/ph/books";
import IconSubs from "~icons/ph/broadcast";
import IconFlask from "~icons/ph/flask";
import IconCode from "~icons/ph/brackets-curly";
import IconCircle from "~icons/ph/circle-dashed";

export interface NavigationItem {
  path: string;
  label: string;
  icon: Component;
  hint?: string;
  children: NavigationItem[];
}
// Per-route icon and one-line hint for menus; routes themselves come from the server.
const META: Record<string, { icon: Component; hint?: string }> = {
  "/dashboard": { icon: IconHouse },
  "/subcription": { icon: IconSubs },
  "/subcription/subs": { icon: IconLink, hint: "生成与分发订阅链接" },
  "/subcription/collections": { icon: IconStack, hint: "多份订阅合成一个链接" },
  "/subcription/nodes": { icon: IconNodes, hint: "分组、编辑与检测节点" },
  "/subcription/airport": { icon: IconAirplane, hint: "机场来源、同步与流量" },
  "/subcription/clients": { icon: IconDevices, hint: "各平台客户端下载" },
  "/test": { icon: IconFlask },
  "/test/speed": { icon: IconGauge, hint: "经节点测量真实吞吐" },
  "/test/unlock": { icon: IconLock, hint: "AI 与流媒体可用性" },
  "/test/tcp": { icon: IconPulse, hint: "各省运营商连通性" },
  "/template": { icon: IconCode },
  "/template/list": { icon: IconFiles, hint: "编辑客户端配置模板" },
  "/template/builder": { icon: IconSliders, hint: "勾选规则、拖拽分组" },
  "/template/rules": { icon: IconBooks, hint: "规则源与规则目录" },
};
export function resolvePath(parent: string, child: string) {
  return /^(https?:)?\/\//.test(child) || child.startsWith("/")
    ? child
    : `${parent}/${child}`.replace(/\/+/g, "/");
}
export function buildNavigation(
  routes: RouteRecordRaw[],
  parent = ""
): NavigationItem[] {
  return routes.flatMap((route) => {
    if (route.meta?.hidden) return [];
    const path = resolvePath(parent, route.path);
    const children = buildNavigation(route.children || [], path);
    if (!route.meta?.title) return children;
    const meta = META[path];
    return [
      {
        path,
        label: String(translateRouteTitle(route.meta.title)),
        icon: meta?.icon || IconCircle,
        hint: meta?.hint,
        children,
      },
    ];
  });
}
export function flattenNavigation(items: NavigationItem[]): NavigationItem[] {
  return items.flatMap((item) =>
    item.children.length ? flattenNavigation(item.children) : [item]
  );
}
