<template>
  <div v-if="authenticated" class="ion-shell">
    <a href="#ion-content" class="skip-link" @click.prevent="focusMain"
      >跳到主要内容</a
    >
    <header class="ion-header">
      <div class="header-inner">
        <router-link to="/dashboard" class="brand" aria-label="PPEELink 首页"
          ><img class="brand-logo" :src="logo" width="28" height="28" alt="" />
          <span class="brand-name">PPEELink</span></router-link
        >
        <TopNav class="desktop-nav" :items="navigation" />
        <div class="header-tools">
          <button
            class="search-pill"
            aria-label="搜索页面，Command K 或 Control K"
            @click="commandVisible = true"
          >
            <IconSearch /><span>搜索</span><kbd>{{ shortcutLabel }}K</kbd>
          </button>
          <el-tooltip
            :content="`测试任务：${taskLabel}`"
            :trigger-keys="[]"
            placement="bottom"
          >
            <button
              class="tool-button"
              :class="{ 'is-running': testRunning, 'is-warn': statusError }"
              :aria-label="`测试任务：${taskLabel}`"
              @click="testVisible = true"
            >
              <IconPulse />
            </button>
          </el-tooltip>
          <el-tooltip
            :content="isDark ? '切换浅色主题' : '切换深色主题'"
            :trigger-keys="[]"
            placement="bottom"
          >
            <button
              class="tool-button"
              :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
              @click="toggleTheme"
            >
              <Transition name="swap" mode="out-in"
                ><component
                  :is="isDark ? IconSun : IconMoon"
                  :key="String(isDark)"
              /></Transition>
            </button>
          </el-tooltip>
          <AccountMenu :username="user.user.username" @logout="logout" />
          <button
            class="tool-button menu-button"
            :aria-label="sheetOpen ? '关闭导航' : '打开导航'"
            :aria-expanded="sheetOpen"
            aria-controls="ion-sheet"
            @click="sheetOpen = !sheetOpen"
          >
            <component :is="sheetOpen ? IconClose : IconList" />
          </button>
        </div>
      </div>
    </header>
    <Transition name="sheet">
      <nav
        v-if="sheetOpen"
        id="ion-sheet"
        class="mobile-sheet"
        aria-label="全部页面"
        @keydown.esc="sheetOpen = false"
      >
        <section v-for="group in navigation" :key="group.path">
          <h2 v-if="group.children.length">{{ group.label }}</h2>
          <router-link
            v-for="item in flattenNavigation([group])"
            :key="item.path"
            :to="item.path"
            class="sheet-link"
            :aria-current="route.path === item.path ? 'page' : undefined"
            @click="sheetOpen = false"
            ><span class="sheet-icon"><component :is="item.icon" /></span
            ><span
              ><strong>{{ item.label }}</strong
              ><small v-if="item.hint">{{ item.hint }}</small></span
            ></router-link
          >
        </section>
        <div class="sheet-actions">
          <UiButton @click="toggleTheme">{{
            isDark ? "浅色主题" : "深色主题"
          }}</UiButton>
          <UiButton
            @click="
              testVisible = true;
              sheetOpen = false;
            "
            >测试任务：{{ taskLabel }}</UiButton
          >
          <UiButton @click="router.push('/system/user/set')">修改密码</UiButton
          ><UiButton variant="ghost" @click="logout">退出登录</UiButton>
        </div>
      </nav>
    </Transition>
    <main id="ion-content" class="ion-content" tabindex="-1">
      <AppMain />
    </main>
    <CommandPalette v-model="commandVisible" :items="commands" />
    <TestStatusDialog v-if="testVisible" v-model:visible="testVisible" />
  </div>
  <main v-else class="public-error-content"><AppMain /></main>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { ElMessageBox } from "element-plus";
import IconSearch from "~icons/ph/magnifying-glass";
import IconPulse from "~icons/ph/pulse";
import IconSun from "~icons/ph/sun";
import IconMoon from "~icons/ph/moon-stars";
import IconList from "~icons/ph/list";
import IconClose from "~icons/ph/x";
import { usePermissionStore, useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import { GetTestStatus } from "@/api/subcription/node";
import logo from "@/assets/logo.svg";
import TestStatusDialog from "@/components/TestStatusDialog.vue";
import { CommandPalette, UiButton, type CommandItem } from "@/ui";
import AppMain from "./components/AppMain/index.vue";
import TopNav from "./components/TopNav.vue";
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
const isExternal = (path: string) => /^(https?:)?\/\//.test(path);
const commandVisible = ref(false),
  sheetOpen = ref(false),
  testVisible = ref(false),
  testRunning = ref(false),
  statusError = ref(false);
const taskLabel = computed(() =>
  testRunning.value ? "进行中" : statusError.value ? "状态未知" : "空闲"
);
const shortcutLabel = /Mac|iPhone|iPad/.test(navigator.platform)
  ? "⌘"
  : "Ctrl ";
function toggleTheme() {
  settings.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function focusMain() {
  document.getElementById("ion-content")?.focus();
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
      keywords: `${item.path} ${item.hint || ""}`,
      run: () => {
        if (isExternal(item.path)) window.location.assign(item.path);
        else return router.push(item.path);
      },
    }))
  ),
  { id: "theme", label: "切换主题", run: toggleTheme },
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
  () => (sheetOpen.value = false)
);
watch(sheetOpen, (open) =>
  document.documentElement.classList.toggle("ion-sheet-lock", open)
);
onBeforeUnmount(() => {
  disposed = true;
  if (statusTimer) clearTimeout(statusTimer);
  document.documentElement.classList.remove("ion-sheet-lock");
});
</script>
<style scoped>
.public-error-content {
  min-height: 100dvh;
  max-width: 1440px;
  padding: 28px;
}
.ion-shell {
  min-height: 100dvh;
  /* One faint accent bloom anchors the top of every page. */
  background:
    radial-gradient(60% 420px at 85% -120px, var(--c-glow), transparent 70%),
    var(--c-canvas);
}
.skip-link {
  position: fixed;
  top: 12px;
  left: 12px;
  z-index: calc(var(--z-tools) + 2);
  transform: translateY(-160%);
  padding: 10px 16px;
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  color: var(--c-ink);
}
.skip-link:focus {
  transform: none;
}
.ion-header {
  position: sticky;
  top: 0;
  z-index: var(--z-tools);
  backdrop-filter: blur(14px) saturate(1.4);
  background: color-mix(in srgb, var(--c-canvas) 72%, transparent);
  border-bottom: 1px solid color-mix(in srgb, var(--c-line) 70%, transparent);
}
.header-inner {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  align-items: center;
  gap: 16px;
  max-width: 1400px;
  height: 64px;
  margin: 0 auto;
  padding: 0 32px;
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  justify-self: start;
  color: var(--c-ink);
}
.brand-name {
  font-size: 16px;
  font-weight: 600;
  letter-spacing: -0.02em;
}
.header-tools {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}
.search-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 6px 0 12px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 13px;
  transition: border-color var(--duration-fast) var(--ease-out);
}
.search-pill:hover {
  border-color: var(--c-muted);
  color: var(--c-ink);
}
.search-pill svg {
  width: 16px;
  height: 16px;
}
.search-pill kbd {
  margin-left: 18px;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  font: 500 12px var(--font-mono);
}
.tool-button {
  position: relative;
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border-radius: var(--radius-pill);
  color: var(--c-muted);
  transition:
    color var(--duration-fast),
    background-color var(--duration-fast);
}
.tool-button:hover {
  background: var(--c-surface-2);
  color: var(--c-ink);
}
.tool-button svg {
  width: 18px;
  height: 18px;
}
/* A running task is real state, so it earns the one live indicator. */
.tool-button.is-running::after,
.tool-button.is-warn::after {
  content: "";
  position: absolute;
  top: 7px;
  right: 7px;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-ok);
  box-shadow: 0 0 0 2px var(--c-canvas);
  animation: ion-ping 1.6s var(--ease-out) infinite;
}
.tool-button.is-warn::after {
  background: var(--c-warn);
  animation: none;
}
@keyframes ion-ping {
  50% {
    transform: scale(1.5);
    opacity: 0.6;
  }
}
.swap-enter-active,
.swap-leave-active {
  transition:
    transform var(--duration-medium) var(--ease-spring),
    opacity var(--duration-fast);
}
.swap-enter-from {
  transform: rotate(-90deg) scale(0.6);
  opacity: 0;
}
.swap-leave-to {
  transform: rotate(90deg) scale(0.6);
  opacity: 0;
}
.menu-button {
  display: none;
}
.ion-content {
  max-width: 1400px;
  margin: 0 auto;
  padding: 32px 32px 56px;
  min-width: 0;
}
.mobile-sheet {
  position: fixed;
  inset: 64px 0 0;
  z-index: calc(var(--z-tools) - 1);
  display: grid;
  align-content: start;
  gap: 20px;
  padding: 20px 16px calc(24px + env(safe-area-inset-bottom));
  overflow-y: auto;
  background: var(--c-canvas);
}
.mobile-sheet h2 {
  margin: 0 0 6px;
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 500;
}
.sheet-link {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 8px;
  border-radius: 14px;
  color: var(--c-ink);
}
.sheet-link[aria-current="page"] {
  background: var(--c-surface);
  box-shadow: var(--edge-highlight);
}
.sheet-link strong {
  display: block;
  font-size: 15px;
  font-weight: 500;
}
.sheet-link small {
  color: var(--c-muted);
  font-size: 12px;
}
.sheet-icon {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
  color: var(--c-muted);
}
.sheet-link[aria-current="page"] .sheet-icon {
  border-color: transparent;
  background: var(--c-signal);
  color: var(--c-on-signal);
}
.sheet-icon svg {
  width: 18px;
  height: 18px;
}
.sheet-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.sheet-enter-active,
.sheet-leave-active {
  transition:
    opacity var(--duration-medium) var(--ease-out),
    transform var(--duration-medium) var(--ease-out);
}
.sheet-enter-from,
.sheet-leave-to {
  opacity: 0;
  transform: translateY(-12px);
}
@media (max-width: 1099px) {
  .header-inner {
    grid-template-columns: auto 1fr;
    padding: 0 20px;
  }
  .desktop-nav {
    display: none;
  }
  .menu-button {
    display: grid;
  }
  .ion-content {
    padding: 24px 20px 40px;
  }
}
@media (max-width: 767px) {
  .header-inner {
    padding: 0 12px;
    gap: 8px;
  }
  .search-pill {
    width: 36px;
    padding: 0;
    justify-content: center;
  }
  .search-pill span,
  .search-pill kbd,
  .header-tools > :deep(.account-dropdown) {
    display: none;
  }
  .ion-content {
    padding: 20px 12px 32px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .tool-button.is-running::after {
    animation: none;
  }
  .sheet-enter-active,
  .sheet-leave-active,
  .swap-enter-active,
  .swap-leave-active {
    transition: none;
  }
}
:global(html.ion-sheet-lock) {
  overflow: hidden;
}
</style>
