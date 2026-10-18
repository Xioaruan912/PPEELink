<template>
  <div
    v-if="authenticated"
    class="console-shell"
    :class="{ 'is-collapsed': collapsed, 'is-drawer-open': drawerOpen }"
  >
    <a href="#console-content" class="skip-link" @click.prevent="focusMain"
      >跳到主要内容</a
    >
    <div
      class="console-scrim"
      aria-hidden="true"
      @click="drawerOpen = false"
    ></div>
    <aside
      id="console-sidebar"
      class="console-sidebar"
      aria-label="工作区"
      @keydown.esc="drawerOpen = false"
    >
      <div class="sidebar-brand">
        <router-link
          to="/dashboard"
          class="brand-link"
          aria-label="PPEELink 首页"
          ><img
            class="brand-logo"
            :src="logo"
            width="32"
            height="32"
            alt=""
          /><span class="brand-copy"
            ><strong>PPEELink</strong><small>订阅控制台</small></span
          ></router-link
        >
        <button
          class="icon-button collapse-button"
          :aria-label="collapsed ? '展开侧栏' : '收起侧栏'"
          :aria-expanded="!collapsed"
          aria-controls="console-sidebar"
          @click="toggleCollapsed"
        >
          <component :is="collapsed ? Expand : Fold" />
        </button>
      </div>
      <div class="sidebar-scroll">
        <SideNav
          :items="navigation"
          :collapsed="collapsed && !drawerOpen"
          @navigate="drawerOpen = false"
        />
      </div>
      <footer class="sidebar-footer">
        <button
          class="sidebar-row"
          :aria-label="`测试任务：${taskLabel}`"
          @click="testVisible = true"
        >
          <StatusDot
            :state="statusError ? 'warn' : testRunning ? 'ok' : 'idle'"
            :running="testRunning"
            :label="taskLabel"
          />
        </button>
        <button
          class="sidebar-row"
          :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
          @click="toggleTheme"
        >
          <component :is="isDark ? Sunny : Moon" class="row-icon" /><span
            class="row-label"
            >{{ isDark ? "浅色主题" : "深色主题" }}</span
          >
        </button>
        <AccountMenu
          :username="user.user.username"
          :compact="collapsed && !drawerOpen"
          @logout="logout"
        />
      </footer>
    </aside>
    <div class="console-main">
      <header class="console-topbar">
        <button
          class="icon-button menu-button"
          aria-label="打开导航"
          aria-controls="console-sidebar"
          :aria-expanded="drawerOpen"
          @click="drawerOpen = true"
        >
          <Menu />
        </button>
        <nav class="crumbs" aria-label="当前位置">
          <span v-if="currentGroup" class="crumbs__group">{{
            currentGroup
          }}</span>
          <span v-if="currentGroup" class="crumbs__sep" aria-hidden="true"
            >/</span
          >
          <span class="crumbs__page" aria-current="page">{{
            currentPageTitle
          }}</span>
        </nav>
        <button
          class="search-field"
          aria-label="搜索页面，Command K 或 Control K"
          @click="commandVisible = true"
        >
          <Search /><span>搜索页面或操作</span><kbd>{{ shortcutLabel }} K</kbd>
        </button>
      </header>
      <main id="console-content" class="console-content" tabindex="-1">
        <AppMain />
      </main>
    </div>
    <CommandPalette v-model="commandVisible" :items="commands" />
    <TestStatusDialog v-if="testVisible" v-model:visible="testVisible" />
  </div>
  <main v-else class="public-error-content"><AppMain /></main>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import {
  Expand,
  Fold,
  Menu,
  Moon,
  Search,
  Sunny,
} from "@element-plus/icons-vue";
import { usePermissionStore, useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import { translateRouteTitle } from "@/utils/i18n";
import { GetTestStatus } from "@/api/subcription/node";
import logo from "@/assets/logo.svg";
import TestStatusDialog from "@/components/TestStatusDialog.vue";
import { CommandPalette, StatusDot, type CommandItem } from "@/ui";
import AppMain from "./components/AppMain/index.vue";
import SideNav from "./components/SideNav.vue";
import AccountMenu from "./components/AccountMenu.vue";
import { buildNavigation, flattenNavigation } from "./navigation";
const permission = usePermissionStore(),
  settings = useSettingsStore(),
  user = useUserStore(),
  route = useRoute(),
  router = useRouter();
const authenticated = !!localStorage.getItem("accessToken");
const isDark = computed(() => settings.theme === ThemeEnum.DARK);
const navigation = computed(() => buildNavigation(permission.routes));
const currentGroup = computed(
  () =>
    navigation.value.find(
      (group) =>
        group.children.length &&
        flattenNavigation([group]).some((item) => item.path === route.path)
    )?.label
);
const currentPageTitle = computed(
  () =>
    flattenNavigation(navigation.value).find((item) => item.path === route.path)
      ?.label || String(translateRouteTitle(route.meta.title || "PPEELink"))
);
const isExternal = (path: string) => /^(https?:)?\/\//.test(path);
const commandVisible = ref(false),
  drawerOpen = ref(false),
  testVisible = ref(false),
  testRunning = ref(false),
  statusError = ref(false);
const taskLabel = computed(() =>
  testRunning.value ? "测试进行中" : statusError.value ? "状态未知" : "任务空闲"
);
const shortcutLabel = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘" : "Ctrl";
const COLLAPSE_KEY = "console-sidebar-collapsed";
const readCollapsed = () => {
  try {
    return localStorage.getItem(COLLAPSE_KEY) === "1";
  } catch {
    return false;
  }
};
const collapsed = ref(readCollapsed());
function toggleCollapsed() {
  collapsed.value = !collapsed.value;
  try {
    localStorage.setItem(COLLAPSE_KEY, collapsed.value ? "1" : "0");
  } catch {
    /* Preference is optional. */
  }
}
function toggleTheme() {
  settings.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function focusMain() {
  document.getElementById("console-content")?.focus();
}
async function logout() {
  try {
    await ElMessageBox.confirm("确定退出当前账户？", "退出登录", {
      confirmButtonText: "退出登录",
      cancelButtonText: "取消",
      type: "warning",
      lockScroll: false,
    });
  } catch {
    return;
  }
  try {
    await user.logout();
    await router.push({ path: "/login", query: { redirect: route.fullPath } });
  } catch {
    /* Request layer reports API failures. */
  }
}
const commands = computed<CommandItem[]>(() => [
  ...navigation.value.flatMap((group) =>
    flattenNavigation([group]).map((item) => ({
      id: item.path,
      label: item.label,
      group: group.label !== item.label ? group.label : undefined,
      icon: item.icon,
      keywords: item.path,
      run: () => {
        if (isExternal(item.path)) window.location.assign(item.path);
        else return router.push(item.path);
      },
    }))
  ),
  { id: "theme", label: "切换主题", run: toggleTheme },
  { id: "collapse", label: "收起或展开侧栏", run: toggleCollapsed },
  { id: "tasks", label: "查看测试任务", run: () => (testVisible.value = true) },
  {
    id: "password",
    label: "修改密码",
    run: () => router.push("/system/user/set"),
  },
  { id: "logout", label: "退出登录", run: logout },
]);
let statusTimer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;
async function pollStatus() {
  try {
    const { data } = await GetTestStatus();
    if (!disposed) {
      testRunning.value = !!data;
      statusError.value = false;
    }
  } catch {
    if (!disposed) {
      testRunning.value = false;
      statusError.value = true;
    }
  } finally {
    if (!disposed) statusTimer = setTimeout(pollStatus, 5000);
  }
}
onMounted(() => {
  if (authenticated) pollStatus();
});
watch(
  () => route.path,
  () => (drawerOpen.value = false)
);
watch(drawerOpen, (open) =>
  document.documentElement.classList.toggle("console-drawer-lock", open)
);
onBeforeUnmount(() => {
  disposed = true;
  if (statusTimer) clearTimeout(statusTimer);
  document.documentElement.classList.remove("console-drawer-lock");
});
</script>
<style scoped>
.public-error-content {
  min-height: 100dvh;
  max-width: 1440px;
  padding: 28px;
}
.console-shell {
  --sidebar-w: 236px;
  min-height: 100dvh;
}
.console-shell.is-collapsed {
  --sidebar-w: 68px;
}
.skip-link {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: calc(var(--z-tools) + 2);
  transform: translateY(-160%);
  padding: 10px 16px;
  border-radius: var(--radius-control);
  background: var(--c-surface);
  color: var(--c-ink);
}
.skip-link:focus {
  transform: none;
}

/* Sidebar */
.console-sidebar {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: var(--z-dock);
  width: var(--sidebar-w);
  display: flex;
  flex-direction: column;
  background: var(--c-surface);
  border-right: 1px solid var(--c-line);
  transition: width var(--duration-medium) var(--ease-out);
  overflow: hidden;
}
.sidebar-brand {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 60px;
  padding: 0 12px 0 16px;
  border-bottom: 1px solid var(--c-line);
}
.brand-link {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  color: var(--c-ink);
}
.brand-copy {
  display: grid;
  line-height: 1.15;
  white-space: nowrap;
}
.brand-copy strong {
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.brand-copy small {
  color: var(--c-muted);
  font: 400 12px var(--font-mono);
}
.sidebar-scroll {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: hidden;
  padding-bottom: 12px;
}
.sidebar-footer {
  display: grid;
  gap: 2px;
  padding: 10px 12px 14px;
  border-top: 1px solid var(--c-line);
}
.sidebar-row {
  display: flex;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-height: 36px;
  padding: 0 10px;
  border: 0;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--c-muted);
  font-size: 14px;
  text-align: left;
  white-space: nowrap;
  overflow: hidden;
}
.sidebar-row:hover {
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.row-icon {
  flex: 0 0 auto;
  width: 18px;
  height: 18px;
}
.is-collapsed .brand-copy,
.is-collapsed .row-label,
.is-collapsed .sidebar-row :deep(.status-dot__label) {
  display: none;
}
.is-collapsed .sidebar-brand {
  flex-direction: column;
  justify-content: center;
  height: auto;
  padding: 14px 0 10px;
}
.is-collapsed .sidebar-row {
  justify-content: center;
  padding: 0;
}
.icon-button {
  display: inline-grid;
  place-items: center;
  flex: 0 0 auto;
  width: 36px;
  height: 36px;
  padding: 9px;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--c-muted);
}
.icon-button:hover {
  color: var(--c-ink);
  background: var(--c-surface-2);
}
.icon-button svg {
  width: 18px;
  height: 18px;
}

/* Main column */
.console-main {
  margin-left: var(--sidebar-w);
  min-width: 0;
  transition: margin-left var(--duration-medium) var(--ease-out);
}
.console-topbar {
  position: sticky;
  top: 0;
  z-index: var(--z-tools);
  display: flex;
  align-items: center;
  gap: 16px;
  height: 60px;
  padding: 0 28px;
  background: color-mix(in srgb, var(--c-canvas) 86%, transparent);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid var(--c-line);
}
.menu-button {
  display: none;
}
.crumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 14px;
  white-space: nowrap;
}
.crumbs__group,
.crumbs__sep {
  color: var(--c-muted);
}
.crumbs__page {
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
}
.search-field {
  display: flex;
  align-items: center;
  gap: 10px;
  width: min(320px, 40vw);
  height: 36px;
  margin-left: auto;
  padding: 0 8px 0 12px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 14px;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    box-shadow var(--duration-fast) var(--ease-out);
}
.search-field:hover {
  border-color: var(--c-muted);
}
.search-field svg {
  width: 16px;
  height: 16px;
}
.search-field kbd {
  margin-left: auto;
  padding: 2px 6px;
  border: 1px solid var(--c-line);
  border-radius: 6px;
  background: var(--c-surface-2);
  font: 500 12px var(--font-mono);
}
.console-content {
  max-width: 1480px;
  padding: 24px 28px 40px;
  min-width: 0;
}
.console-scrim {
  display: none;
}

@media (max-width: 1023px) {
  .console-shell,
  .console-shell.is-collapsed {
    --sidebar-w: 0px;
  }
  .console-sidebar {
    width: 272px;
    transform: translateX(-100%);
    transition: transform var(--duration-medium) var(--ease-out);
    box-shadow: var(--shadow-overlay);
    z-index: calc(var(--z-tools) + 1);
  }
  .is-drawer-open .console-sidebar {
    transform: none;
  }
  /* Drawer always shows labels, even when the desktop rail is collapsed. */
  .is-collapsed .brand-copy,
  .is-collapsed .row-label,
  .is-collapsed .sidebar-row :deep(.status-dot__label) {
    display: revert;
  }
  .is-collapsed .brand-copy {
    display: grid;
  }
  .is-collapsed .sidebar-brand {
    flex-direction: row;
    justify-content: space-between;
    height: 60px;
    padding: 0 12px 0 16px;
  }
  .is-collapsed .sidebar-row {
    justify-content: flex-start;
    padding: 0 10px;
  }
  .collapse-button {
    display: none;
  }
  .console-scrim {
    position: fixed;
    inset: 0;
    z-index: var(--z-tools);
    display: block;
    background: rgb(0 0 0 / 0.32);
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--duration-medium) var(--ease-out);
  }
  .is-drawer-open .console-scrim {
    opacity: 1;
    pointer-events: auto;
  }
  .menu-button {
    display: inline-grid;
    margin-left: -8px;
  }
  .console-topbar {
    padding: 0 20px;
    gap: 8px;
  }
  .console-content {
    padding: 20px 20px 32px;
  }
}
@media (max-width: 767px) {
  .console-topbar {
    height: calc(56px + env(safe-area-inset-top));
    padding: env(safe-area-inset-top) 12px 0;
  }
  .search-field {
    width: 36px;
    padding: 0;
    justify-content: center;
    border-color: transparent;
    background: transparent;
  }
  .search-field span,
  .search-field kbd,
  .crumbs__group,
  .crumbs__sep {
    display: none;
  }
  .console-content {
    padding: 16px 12px 28px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .console-sidebar,
  .console-main,
  .console-scrim {
    transition: none;
  }
}
:global(html.console-drawer-lock) {
  overflow: hidden;
}
</style>
