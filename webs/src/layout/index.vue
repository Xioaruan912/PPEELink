<template>
  <div v-if="authenticated" class="atlas-shell">
    <a href="#atlas-content" class="skip-link" @click.prevent="focusMain"
      >跳到主要内容</a
    >
    <aside class="atlas-dock" aria-label="工作区导航">
      <router-link class="dock-brand" to="/dashboard" aria-label="PPEELink 首页"
        ><img
          class="brand-logo"
          :src="logo"
          width="40"
          height="40"
          alt="PPEELink"
      /></router-link>
      <nav
        ref="dockNav"
        class="dock-navigation"
        aria-label="主导航"
        @keydown="onDockKeydown"
      >
        <i ref="indicator" class="dock-indicator" aria-hidden="true"></i>
        <div
          v-for="(item, index) in navigation"
          :key="item.path"
          class="dock-entry"
          :data-active="isActive(item)"
          :data-index="index"
        >
          <DockGroup :item="item" :active="isActive(item)" />
        </div>
      </nav>
      <div class="dock-account">
        <el-tooltip
          :content="isDark ? '切换浅色主题' : '切换深色主题'"
          :trigger="['hover', 'focus']"
          :trigger-keys="[]"
          ><button
            class="icon-button"
            :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
            @click="toggleTheme"
          >
            <component :is="isDark ? Sunny : Moon" /></button
        ></el-tooltip>
        <AccountMenu
          :username="user.user.username"
          label="账户菜单"
          @logout="logout"
        />
      </div>
    </aside>
    <header class="atlas-topbar">
      <router-link
        class="mobile-brand"
        to="/dashboard"
        aria-label="PPEELink 首页"
      >
        <img class="brand-logo" :src="logo" width="32" height="32" alt="" />
        <span>{{ currentPageTitle }}</span>
      </router-link>
      <div class="atlas-tools" role="group" aria-label="快捷工具">
        <el-tooltip
          content="搜索页面，Command K 或 Control K"
          :trigger="['hover', 'focus']"
          :trigger-keys="[]"
          ><button
            class="search-button"
            aria-label="搜索页面，Command K 或 Control K"
            @click="commandVisible = true"
          >
            <Search /><kbd>{{ shortcutLabel }} K</kbd>
          </button></el-tooltip
        >
        <button
          class="task-button"
          :aria-label="`测试任务：${testRunning ? '进行中' : statusError ? '状态获取失败' : '空闲'}`"
          @click="testVisible = true"
        >
          <StatusDot
            :state="statusError ? 'warn' : testRunning ? 'ok' : 'idle'"
            :running="testRunning"
            :label="
              testRunning ? '测试中' : statusError ? '状态未知' : '任务空闲'
            "
          />
        </button>
        <el-tooltip
          :content="isDark ? '切换浅色主题' : '切换深色主题'"
          :trigger="['hover', 'focus']"
          :trigger-keys="[]"
          ><button
            class="icon-button"
            :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
            @click="toggleTheme"
          >
            <component :is="isDark ? Sunny : Moon" /></button
        ></el-tooltip>
        <AccountMenu :username="user.user.username" @logout="logout" />
      </div>
    </header>
    <main id="atlas-content" class="atlas-content" tabindex="-1">
      <AppMain />
    </main>
    <nav class="mobile-dock" aria-label="移动主导航" @keydown="onDockKeydown">
      <DockGroup
        v-for="item in navigation.slice(0, 4)"
        :key="item.path"
        :item="item"
        :active="isActive(item)"
      /><button
        class="mobile-more"
        aria-label="打开更多菜单"
        @click="moreVisible = true"
      >
        <MoreFilled /><span>更多</span>
      </button>
    </nav>
    <UiDrawer
      v-model="moreVisible"
      title="所有页面"
      direction="btt"
      size="75dvh"
      ><nav class="more-navigation" aria-label="所有页面">
        <section v-for="group in navigation" :key="group.path">
          <h2>{{ group.label }}</h2>
          <a
            v-for="item in flattenNavigation([group])"
            :key="item.path"
            :href="isExternal(item.path) ? item.path : `#${item.path}`"
            :aria-current="route.path === item.path ? 'page' : undefined"
            @click="moreVisible = false"
            ><SvgIcon :icon-class="item.icon" />{{ item.label }}</a
          >
        </section>
      </nav>
      <template #footer>
        <div class="more-actions">
          <UiButton variant="ghost" @click="toggleTheme">
            {{ isDark ? "切换浅色主题" : "切换深色主题" }}
          </UiButton>
          <UiButton
            variant="ghost"
            @click="
              testVisible = true;
              moreVisible = false;
            "
          >
            测试任务
          </UiButton>
          <UiButton
            @click="
              router.push('/system/user/set');
              moreVisible = false;
            "
            >修改密码</UiButton
          ><UiButton variant="ghost" @click="logout">退出登录</UiButton>
        </div>
      </template>
    </UiDrawer>
    <CommandPalette v-model="commandVisible" :items="commands" />
    <TestStatusDialog v-if="testVisible" v-model:visible="testVisible" />
  </div>
  <main v-else class="public-error-content"><AppMain /></main>
</template>
<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import { Moon, MoreFilled, Search, Sunny } from "@element-plus/icons-vue";
import { usePermissionStore, useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import { translateRouteTitle } from "@/utils/i18n";
import { GetTestStatus } from "@/api/subcription/node";
import logo from "@/assets/logo.svg";
import SvgIcon from "@/components/SvgIcon/index.vue";
import TestStatusDialog from "@/components/TestStatusDialog.vue";
import {
  CommandPalette,
  StatusDot,
  UiButton,
  UiDrawer,
  type CommandItem,
} from "@/ui";
import { useMotion } from "@/motion";
import AppMain from "./components/AppMain/index.vue";
import DockGroup from "./components/DockGroup.vue";
import AccountMenu from "./components/AccountMenu.vue";
import {
  buildNavigation,
  flattenNavigation,
  type NavigationItem,
} from "./navigation";
const permission = usePermissionStore(),
  settings = useSettingsStore(),
  user = useUserStore(),
  route = useRoute(),
  router = useRouter();
const authenticated = !!localStorage.getItem("accessToken");
const isDark = computed(() => settings.theme === ThemeEnum.DARK);
const navigation = computed(() => buildNavigation(permission.routes));
const currentPageTitle = computed(
  () =>
    flattenNavigation(navigation.value).find((item) => item.path === route.path)
      ?.label || String(translateRouteTitle(route.meta.title || "PPEELink"))
);
const isExternal = (path: string) => /^(https?:)?\/\//.test(path);
const isActive = (item: NavigationItem) =>
  flattenNavigation([item]).some((child) => child.path === route.path);
const commandVisible = ref(false),
  moreVisible = ref(false),
  testVisible = ref(false),
  testRunning = ref(false),
  statusError = ref(false);
const shortcutLabel = /Mac|iPhone|iPad/.test(navigator.platform) ? "⌘" : "Ctrl";
function toggleTheme() {
  settings.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function onDockKeydown(event: KeyboardEvent) {
  if (event.defaultPrevented) return;
  const controls = Array.from(
    (event.currentTarget as HTMLElement).querySelectorAll<HTMLElement>(
      ".dock-trigger, .mobile-more"
    )
  );
  const index = controls.indexOf(event.target as HTMLElement);
  if (index < 0) return;
  let next: number;
  switch (event.key) {
    case "ArrowDown":
    case "ArrowRight":
      next = (index + 1) % controls.length;
      break;
    case "ArrowUp":
    case "ArrowLeft":
      next = (index - 1 + controls.length) % controls.length;
      break;
    case "Home":
      next = 0;
      break;
    case "End":
      next = controls.length - 1;
      break;
    default:
      return;
  }
  event.preventDefault();
  controls[next]?.focus();
}
function focusMain() {
  document.getElementById("atlas-content")?.focus();
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
const dockNav = ref<HTMLElement>(),
  indicator = ref<HTMLElement>();
const { animate, stop } = useMotion();
let previousY = 0;
async function moveIndicator() {
  await nextTick();
  const entry = dockNav.value?.querySelector<HTMLElement>(
    '[data-active="true"]'
  );
  if (!entry || !indicator.value) {
    if (indicator.value) indicator.value.style.opacity = "0";
    return;
  }
  indicator.value.style.opacity = "1";
  const nextY = entry.offsetTop + (entry.offsetHeight - 24) / 2;
  stop();
  animate(
    indicator.value,
    { y: previousY },
    { y: nextY, duration: 0.56, ease: "elastic.out(1,.6)" }
  );
  previousY = nextY;
}
let observer: ResizeObserver | undefined;
onMounted(() => {
  if (authenticated) pollStatus();
  moveIndicator();
  observer = new ResizeObserver(moveIndicator);
  if (dockNav.value) observer.observe(dockNav.value);
});
watch(
  () => [route.path, navigation.value],
  () => {
    moreVisible.value = false;
    moveIndicator();
  }
);
onBeforeUnmount(() => {
  disposed = true;
  if (statusTimer) clearTimeout(statusTimer);
  observer?.disconnect();
});
</script>
<style scoped>
.public-error-content {
  min-height: 100dvh;
  max-width: 1440px;
  padding: 28px;
}
@media (min-width: 768px) and (max-width: 1023px) {
  .public-error-content {
    padding: 20px;
  }
}
@media (max-width: 767px) {
  .public-error-content {
    padding: 20px 12px;
  }
}

.atlas-shell {
  min-height: 100dvh;
}
.skip-link {
  position: fixed;
  top: 16px;
  left: 96px;
  z-index: var(--z-tools);
  transform: translateY(-160%);
  padding: 10px 16px;
  border-radius: var(--radius-control);
  background: var(--c-surface);
  color: var(--c-ink);
}
.skip-link:focus {
  transform: none;
}
.atlas-dock {
  position: fixed;
  inset: 0 auto 0 0;
  z-index: var(--z-dock);
  width: 60px;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0 12px;
  background: var(--c-surface);
  border-right: 1px solid var(--c-line);
}
.dock-brand {
  display: grid;
  place-items: center;
  width: 40px;
  min-height: 40px;
  margin-bottom: 16px;
}
.dock-navigation {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  padding: 0 10px;
  min-height: 0;
  flex: 1;
}
.dock-indicator {
  position: absolute;
  top: 0;
  left: 0;
  width: 3px;
  height: 24px;
  background: var(--c-signal);
  border-radius: 999px;
  pointer-events: none;
}
.dock-account {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-top: 12px;
  border-top: 1px solid var(--c-line);
}
.icon-button {
  display: inline-grid;
  place-items: center;
  flex: 0 0 auto;
  width: 44px;
  height: 36px;
  padding: 10px;
  border: 1px solid transparent;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--c-ink);
}
.icon-button svg {
  width: 20px;
  height: 20px;
}
.icon-button:hover {
  border-color: var(--c-line);
  background: var(--c-surface-2);
}

.atlas-topbar {
  display: contents;
}
.mobile-brand {
  display: none;
}
.atlas-tools {
  position: fixed;
  z-index: var(--z-tools);
  top: 24px;
  right: 48px;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  box-shadow: var(--shadow-panel);
}
.search-button,
.task-button {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  min-height: 36px;
  padding: 8px 10px;
  background: transparent;
  border: 0;
  border-radius: var(--radius-control);
}
.search-button:hover,
.task-button:hover {
  background: var(--c-surface-2);
}
.search-button svg {
  width: 18px;
  height: 18px;
}
.search-button kbd {
  font-size: 13px;
  color: var(--c-muted);
}
.task-button {
  border-left: 1px solid var(--c-line);
}
.atlas-content {
  margin-left: 48px;
  padding: 112px 28px 32px;
  max-width: 1536px;
  min-width: 0;
}
.mobile-dock {
  display: none;
}
.more-navigation {
  display: grid;
  gap: 20px;
}
.more-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.more-navigation h2 {
  color: var(--c-muted);
  font: 400 14px var(--font-mono);
}
.more-navigation a {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 10px;
  min-height: 36px;
  border-radius: var(--radius-control);
  color: var(--c-ink);
}
.more-navigation a:hover,
.more-navigation a[aria-current] {
  background: var(--c-selected);
}
@media (min-width: 768px) and (max-width: 1023px) {
  .atlas-content {
    padding-left: 20px;
    padding-right: 20px;
  }
  .atlas-tools {
    right: 32px;
  }
}
@media (max-width: 767px) {
  .atlas-dock {
    display: none;
  }
  .atlas-content {
    margin-left: 0;
    min-height: 100dvh;
    padding: calc(56px + env(safe-area-inset-top)) 12px 0;
  }
  /* Native focus scrolling must stop above the fixed dock too. */
  :global(html) {
    scroll-padding-bottom: calc(
      var(--mobile-dock-height) + env(safe-area-inset-bottom) + 20px
    );
  }
  .atlas-topbar {
    position: fixed;
    inset: 0 0 auto;
    z-index: var(--z-tools);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    padding: calc(10px + env(safe-area-inset-top)) 12px 10px;
    border-bottom: 1px solid var(--c-line);
    background: var(--c-canvas);
  }
  .mobile-brand {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    min-height: 36px;
    color: var(--c-ink);
    font-size: 14px;
  }
  .mobile-brand span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .atlas-tools {
    position: static;
    flex-shrink: 0;
    gap: 4px;
    padding: 0;
    border: 0;
    background: transparent;
    box-shadow: none;
  }
  .search-button {
    justify-content: center;
    width: 44px;
    padding: 10px;
  }
  .search-button kbd,
  .atlas-tools .task-button,
  .atlas-tools > .icon-button {
    display: none;
  }
  .mobile-dock {
    position: fixed;
    inset: auto 0 0;
    z-index: var(--z-dock);
    display: grid;
    grid-auto-flow: column;
    grid-auto-columns: minmax(0, 1fr);
    gap: 4px;
    height: calc(var(--mobile-dock-height) + env(safe-area-inset-bottom));
    padding: 8px 10px calc(8px + env(safe-area-inset-bottom));
    background: var(--c-surface);
    border-top: 1px solid var(--c-line);
  }
  .mobile-more {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-height: 48px;
    border: 0;
    background: transparent;
    border-radius: var(--radius-control);
    color: var(--c-muted);
    font-size: 14px;
  }
  .mobile-more svg {
    width: 22px;
    height: 22px;
  }
  .skip-link {
    left: 16px;
  }
}
</style>
