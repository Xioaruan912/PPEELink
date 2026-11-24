import type { Component } from "vue";
import type { RouteRecordRaw } from "vue-router";
import { translateRouteTitle } from "@/utils/i18n";
import IconHouse from "~icons/ph/house-simple";
import IconLink from "~icons/ph/link-simple";
import IconNodes from "~icons/ph/graph";
import IconGauge from "~icons/ph/gauge";
import IconFiles from "~icons/ph/files";
import IconSliders from "~icons/ph/sliders-horizontal";
import IconBooks from "~icons/ph/books";
import IconSubs from "~icons/ph/broadcast";
import IconFlask from "~icons/ph/flask";
import IconCode from "~icons/ph/brackets-curly";
import IconCircle from "~icons/ph/circle-dashed";
import IconUserCircle from "~icons/ph/user-circle";
import IconUsers from "~icons/ph/users-three";
import IconGear from "~icons/ph/gear-six";
import IconServer from "~icons/ph/hard-drives";
import IconChartLine from "~icons/ph/chart-line";

export interface NavigationItem {
  path: string;
  label: string;
  icon: Component;
  hint?: string;
  children: NavigationItem[];
}

// Per-route icon, refined display label, and search hint; routes come from backend permissions.
const META: Record<string, { icon: Component; hint?: string; label?: string }> =
  {
    "/dashboard": {
      icon: IconHouse,
      label: "运行态势",
      hint: "节点健康度与订阅到期概览",
    },
    "/monitor/index": {
      icon: IconChartLine,
      label: "主机与 Agent",
      hint: "服务器遥测与 Agent 集中管理",
    },
    "/monitor/website": {
      icon: IconChartLine,
      label: "流量排行",
      hint: "最近 24 小时的网站流量排行",
    },
    "/monitor/workspace": {
      icon: IconFiles,
      label: "服务器管理",
      hint: "WebShell 终端、远程文件与 Docker 容器管理",
    },
    "/monitor/user-agent": {
      icon: IconGauge,
      label: "用户 Agent",
      hint: "从你的本机按需实测到各节点服务器的连通性与延迟",
    },
    "/network": { icon: IconNodes },
    "/network/traceroute": {
      icon: IconGauge,
      label: "路由追踪",
      hint: "比较各台 VPS 到落地的逐跳路径",
    },
    "/network/throughput": {
      icon: IconGauge,
      label: "吞吐互测",
      hint: "双向测速与同一落地的中转对比",
    },
    "/subcription": { icon: IconSubs },
    "/subcription/subs": {
      icon: IconLink,
      label: "订阅链接",
      hint: "选择客户端、复制订阅链接或扫码导入",
    },
    "/subcription/diagnostic": {
      icon: IconFlask,
      label: "故障诊断",
      hint: "一键定位 Reality / HY2 连接异常",
    },
    "/subcription/nodes": {
      icon: IconNodes,
      label: "节点资产",
      hint: "分组整理、可用性检测与下发策略",
    },
    "/subcription/servers": {
      icon: IconServer,
      label: "自建实例",
      hint: "在自有服务器上部署与管理 xray-core",
    },
    "/subcription/relays": {
      icon: IconLink,
      label: "中转网关",
      hint: "纯端口转发与固定入口中转",
    },
    "/test": { icon: IconFlask },
    "/test/index": {
      icon: IconFlask,
      label: "解锁与可达",
      hint: "流媒体/AI 解锁检测与国内三网可达性",
    },
    "/template": { icon: IconCode },
    "/template/list": {
      icon: IconFiles,
      label: "配置模板",
      hint: "编辑客户端配置模板",
    },
    "/template/builder": {
      icon: IconSliders,
      label: "分流编排",
      hint: "勾选规则分类、拖拽策略分组",
    },
    "/template/rules": {
      icon: IconBooks,
      label: "规则仓库",
      hint: "规则源与规则目录管理",
    },
    "/my": { icon: IconUserCircle },
    "/my/subscriptions": {
      icon: IconLink,
      label: "我的订阅",
      hint: "分配给我的订阅与客户端导入",
    },
    "/system": { icon: IconGear },
    "/system/users": {
      icon: IconUsers,
      label: "成员与权限",
      hint: "账户、角色与订阅分配",
    },
    "/system/settings": {
      icon: IconGear,
      label: "系统偏好",
      hint: "备份、API 令牌、审计日志与短链",
    },
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
    if (route.children?.length && !children.length) return [];
    if (route.children?.length && children.length === 1) return children;
    const meta = META[path];
    return [
      {
        path,
        label: meta?.label || String(translateRouteTitle(route.meta.title)),
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

// 7-domain IA: 订阅 → 代理 → Agent → 网络测试 → 概览 → 模板分组 → 系统设置.
const GROUPS = [
  {
    label: "订阅",
    icon: IconSubs,
    paths: ["/subcription/subs", "/my/subscriptions", "/subcription/nodes"],
  },
  {
    label: "代理",
    icon: IconNodes,
    paths: ["/subcription/servers", "/subcription/relays"],
  },
  {
    label: "Agent",
    icon: IconServer,
    paths: ["/monitor/index", "/monitor/workspace", "/monitor/user-agent"],
  },
  {
    label: "网络测试",
    icon: IconFlask,
    paths: [
      "/subcription/diagnostic",
      "/test/index",
      "/network/traceroute",
      "/network/throughput",
    ],
  },
  {
    label: "概览",
    icon: IconChartLine,
    paths: ["/dashboard", "/monitor/website"],
  },
  {
    label: "模板分组",
    icon: IconCode,
    paths: ["/template/builder", "/template/list", "/template/rules"],
  },
  {
    label: "系统设置",
    icon: IconGear,
    paths: ["/system/users", "/system/settings"],
  },
];

export function organizeNavigation(items: NavigationItem[]): NavigationItem[] {
  const leaves = flattenNavigation(items);
  const used = new Set<string>();
  const result: NavigationItem[] = [];
  for (const group of GROUPS) {
    const children = group.paths.flatMap((path) => {
      const item = leaves.find((leaf) => leaf.path === path);
      if (!item || used.has(path)) return [];
      used.add(path);
      return [item];
    });
    if (children.length)
      result.push({
        path: children[0].path,
        label: group.label,
        icon: group.icon,
        children,
      });
  }
  // Preserve future backend routes and ordinary users' subscription entry.
  result.push(...leaves.filter((item) => !used.has(item.path)));
  return result;
}
