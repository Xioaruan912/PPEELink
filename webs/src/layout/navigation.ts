import type { RouteRecordRaw } from "vue-router";
import { translateRouteTitle } from "@/utils/i18n";
export interface NavigationItem {
  path: string;
  label: string;
  icon: string;
  children: NavigationItem[];
}
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
    return [
      {
        path,
        label: String(translateRouteTitle(route.meta.title)),
        icon: String(route.meta.icon || "menu"),
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
