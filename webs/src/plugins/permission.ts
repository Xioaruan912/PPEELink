import router from "@/router";
import { useUserStore, usePermissionStore } from "@/store";
import NProgress, { startProgress } from "@/utils/nprogress";
import { RouteRecordRaw } from "vue-router";
import type { UserInfo } from "@/api/user/types";

const PASSWORD_PAGE = "/system/user/set";
const USER_HOME = "/my/subscriptions";

/**
 * 账户状态决定的强制落点：必须改密码时只能停在改密页；
 * 普通用户没有仪表盘，首页改为“我的订阅”。返回空串表示不拦截。
 */
function landingFor(user: UserInfo, path: string) {
  if (user.mustChangePassword && path !== PASSWORD_PAGE) return PASSWORD_PAGE;
  const admin = user.roles.includes("ADMIN");
  if (!admin && (path === "/" || path === "/dashboard")) return USER_HOME;
  return "";
}

export function setupPermission() {
  // 白名单路由
  const whiteList = ["/login", "/401", "/404"];

  router.beforeEach(async (to, _from, next) => {
    startProgress();
    const hasToken = localStorage.getItem("accessToken");
    if (hasToken) {
      if (to.path === "/login") {
        // 如果已登录，跳转首页
        next({ path: "/" });
        NProgress.done();
      } else {
        const userStore = useUserStore();
        const hasRoles =
          userStore.user.roles && userStore.user.roles.length > 0;
        if (hasRoles) {
          const forced = landingFor(userStore.user, to.path);
          if (forced) {
            next({ path: forced, replace: true });
            NProgress.done();
          } else if (to.matched.length === 0) {
            // 未匹配到任何路由，跳转404
            next("/404");
          } else {
            next();
          }
        } else {
          const permissionStore = usePermissionStore();
          try {
            const { roles } = await userStore.getUserInfo();
            const accessRoutes = await permissionStore.generateRoutes(roles);
            accessRoutes.forEach((route: RouteRecordRaw) => {
              router.addRoute(route);
            });
            const forced = landingFor(userStore.user, to.path);
            next(
              forced
                ? { path: forced, replace: true }
                : { ...to, replace: true }
            );
          } catch (error) {
            // 移除 token 并跳转登录页
            await userStore.resetToken();
            next(`/login?redirect=${to.path}`);
            NProgress.done();
          }
        }
      }
    } else {
      // 未登录可以访问白名单页面
      if (whiteList.indexOf(to.path) !== -1) {
        next();
      } else {
        next(`/login?redirect=${to.path}`);
        NProgress.done();
      }
    }
  });

  router.afterEach(() => {
    NProgress.done();
  });
}
