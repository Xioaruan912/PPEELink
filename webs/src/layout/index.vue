<template>
  <div
    v-if="authenticated"
    class="workspace-shell"
    :class="{ 'workspace-locked': locked }"
  >
    <canvas
      ref="gridCanvas"
      class="union-grid-canvas"
      aria-hidden="true"
    ></canvas>

    <a href="#workspace-content" class="skip-link" @click.prevent="focusMain"
      >跳到主要内容</a
    >

    <!-- Open-Sky Minimalist Corner Header (No full-width top navbar!) -->
    <header class="sky-header">
      <div class="sky-header__left">
        <button
          v-if="!locked"
          class="workspace-icon mobile-trigger"
          aria-label="打开导航"
          :aria-expanded="sheetOpen"
          @click="sheetOpen = true"
        >
          <IconList />
        </button>

        <RouterLink
          to="/"
          class="union-logo"
          aria-label="PPEELink 首页"
          data-hover="logo-morph"
        >
          <span class="sr-only">PPEELink</span>
          <svg
            class="union-logo__svg"
            viewBox="0 0 310 44"
            fill="none"
            aria-hidden="true"
          >
            <text x="0" y="33" fill="currentColor" class="union-logo__word">
              PPEE
            </text>
            <rect
              class="union-logo__capsule"
              x="104"
              y="9"
              width="34"
              height="24"
              rx="12"
              stroke="currentColor"
              stroke-width="4"
            />
            <circle
              class="union-logo__dot"
              cx="121"
              cy="21"
              r="3.8"
              fill="var(--c-ok)"
            />
            <text
              x="148"
              y="33"
              fill="currentColor"
              class="union-logo__word union-logo__trail"
            >
              LINK
            </text>
          </svg>
        </RouterLink>

        <div v-if="!locked && activeGroup" class="sky-coord" aria-hidden="true">
          <span class="coord-slash">/</span>
          <span>{{ activeGroup.label }}</span>
          <span v-if="activePage" class="coord-page"
            >· {{ activePage.label }}</span
          >
        </div>
      </div>

      <div class="sky-header__right">
        <button
          v-if="!locked"
          class="union-search-btn"
          aria-label="搜索页面，Command K 或 Control K"
          @click="commandVisible = true"
        >
          <IconSearch /><span>搜索</span><kbd>{{ shortcutLabel }}K</kbd>
        </button>
        <button
          v-if="!locked && isAdmin"
          class="workspace-icon"
          :class="{ 'task-live': testRunning }"
          :aria-label="`测试任务：${taskLabel}`"
          :title="`测试任务：${taskLabel}`"
          @click="testVisible = true"
        >
          <IconPulse />
        </button>
        <div class="theme-switch-pair" role="group" aria-label="主题模式">
          <button
            type="button"
            class="theme-word"
            :class="{ 'is-on': !isDark }"
            :aria-label="isDark ? '切换浅色主题' : '当前浅色主题'"
            @click="isDark && toggleTheme()"
          >
            Light
          </button>
          <span class="theme-slash" aria-hidden="true">/</span>
          <button
            type="button"
            class="theme-word"
            :class="{ 'is-on': isDark }"
            :aria-label="!isDark ? '切换深色主题' : '当前深色主题'"
            @click="!isDark && toggleTheme()"
          >
            Dark
          </button>
        </div>
        <AccountMenu :username="user.user.username" @logout="logout" />
      </div>
    </header>

    <main id="workspace-content" class="workspace-content" tabindex="-1">
      <AppMain />
    </main>

    <!-- Floating Bottom Machined Obsidian Command Dock -->
    <div v-if="!locked" class="floating-dock-anchor">
      <div class="floating-dock">
        <nav
          ref="domainNavEl"
          class="dock-domains"
          aria-label="功能域切换"
        >
          <span
            class="dock-glider"
            :style="domainGliderStyle"
            aria-hidden="true"
          ></span>
          <RouterLink
            v-for="group in navigation"
            :key="group.label"
            :to="group.path"
            class="dock-domain-pill"
            :class="{ 'is-active': activeGroup?.label === group.label }"
          >
            <span>{{ group.label }}</span>
          </RouterLink>
        </nav>

        <span
          v-if="activeGroup && activeGroup.children.length > 0"
          class="dock-divider"
          aria-hidden="true"
        ></span>

        <nav
          v-if="activeGroup && activeGroup.children.length > 0"
          class="dock-subpages"
          :aria-label="`${activeGroup.label}页面`"
        >
          <template
            v-for="item in flattenNavigation([activeGroup])"
            :key="item.path"
          >
            <a
              v-if="isExternal(item.path)"
              :href="item.path"
              class="dock-sub-link"
            >
              <span class="union-nav-dot" aria-hidden="true"></span>
              <span class="union-nav-label">{{ item.label }}</span>
            </a>
            <RouterLink
              v-else
              :to="item.path"
              class="dock-sub-link"
              :aria-current="route.path === item.path ? 'page' : undefined"
            >
              <span class="union-nav-dot" aria-hidden="true"></span>
              <span class="union-nav-label">{{ item.label }}</span>
            </RouterLink>
          </template>
        </nav>
      </div>
    </div>

    <el-drawer
      v-if="!locked"
      v-model="sheetOpen"
      size="min(320px, 88vw)"
      direction="ltr"
      title="PPEELink"
      append-to-body
      destroy-on-close
      class="workspace-mobile"
    >
      <WorkspaceNav :items="navigation" @navigate="sheetOpen = false" />
    </el-drawer>
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
import IconSearch from "~icons/ph/magnifying-glass";
import IconPulse from "~icons/ph/pulse";
import IconList from "~icons/ph/list";

import { usePermissionStore, useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import { GetTestStatus } from "@/api/subcription/node";
import TestStatusDialog from "@/components/TestStatusDialog.vue";
import { CommandPalette, type CommandItem } from "@/ui";
import { prefersReducedMotion } from "@/motion";
import AppMain from "./components/AppMain/index.vue";
import WorkspaceNav from "./components/WorkspaceNav.vue";
import AccountMenu from "./components/AccountMenu.vue";
import {
  buildNavigation,
  flattenNavigation,
  organizeNavigation,
} from "./navigation";

const permission = usePermissionStore(),
  settings = useSettingsStore(),
  user = useUserStore(),
  route = useRoute(),
  router = useRouter();
const authenticated = !!localStorage.getItem("accessToken");
const isDark = computed(() => settings.theme === ThemeEnum.DARK);
const isAdmin = computed(() => user.user.roles.includes("ADMIN"));
const locked = computed(() => !!user.user.mustChangePassword);
const navigation = computed(() =>
  organizeNavigation(
    buildNavigation(permission.routes).filter(
      (item) => isAdmin.value || item.path !== "/dashboard"
    )
  )
);
const activeGroup = computed(
  () =>
    navigation.value.find(
      (group) =>
        group.children.length &&
        group.children.some((item) => item.path === route.path)
    ) || navigation.value[0]
);
const activePage = computed(() =>
  flattenNavigation(navigation.value).find((item) => item.path === route.path)
);
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

const domainNavEl = ref<HTMLElement>();
const domainGliderStyle = ref({
  transform: "translate3d(0,0,0)",
  width: "0px",
  opacity: 0,
});

function syncDomainGlider() {
  const el = domainNavEl.value;
  if (!el) return;
  const active = el.querySelector<HTMLElement>(".dock-domain-pill.is-active");
  if (!active) {
    domainGliderStyle.value = { ...domainGliderStyle.value, opacity: 0 };
    return;
  }
  domainGliderStyle.value = {
    transform: `translate3d(${active.offsetLeft}px, 0, 0)`,
    width: `${active.offsetWidth}px`,
    opacity: 1,
  };
}

function toggleTheme() {
  settings.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function focusMain() {
  document.getElementById("workspace-content")?.focus();
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
  ...(isAdmin.value
    ? [
        {
          id: "tasks",
          label: "查看测试任务",
          run: () => {
            testVisible.value = true;
          },
        },
      ]
    : []),
  {
    id: "password",
    label: "修改密码",
    run: () => router.push("/system/user/set"),
  },
  { id: "logout", label: "退出登录", run: logout },
]);

/* Sparse UNION-TEC Cursor-Reactive Micro-Crosshairs */
const gridCanvas = ref<HTMLCanvasElement>();
let rafId = 0;
let cleanupCanvas: (() => void) | undefined;

function mountBlueprintCanvas() {
  const canvas = gridCanvas.value;
  if (!canvas) return;
  const ctx = canvas.getContext("2d");
  if (!ctx) return;

  let width = 0;
  let height = 0;
  let targetX = -1000;
  let targetY = -1000;
  let curX = -1000;
  let curY = -1000;
  let settled = false;

  const resize = () => {
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = width * dpr;
    canvas.height = height * dpr;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    draw();
  };

  const draw = () => {
    ctx.clearRect(0, 0, width, height);
    const dark = document.documentElement.classList.contains("dark");
    const step = 136;
    const baseAlpha = dark ? 0.1 : 0.075;

    for (let x = step; x < width; x += step) {
      for (let y = step; y < height; y += step) {
        const dx = curX - x;
        const dy = curY - y;
        const dist = Math.hypot(dx, dy);
        const pull = dist < 200 ? (1 - dist / 200) * 5 : 0;
        const ox = dist > 1 ? (dx / dist) * pull : 0;
        const oy = dist > 1 ? (dy / dist) * pull : 0;
        const arm = dist < 200 ? 3 + (1 - dist / 200) * 2.5 : 2.5;
        const alpha =
          dist < 200 ? baseAlpha + (1 - dist / 200) * 0.24 : baseAlpha;

        ctx.strokeStyle = dark
          ? `rgba(255,255,255,${alpha})`
          : `rgba(10,10,10,${alpha})`;
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.moveTo(x + ox - arm, y + oy + 0.5);
        ctx.lineTo(x + ox + arm, y + oy + 0.5);
        ctx.moveTo(x + ox + 0.5, y + oy - arm);
        ctx.lineTo(x + ox + 0.5, y + oy + arm);
        ctx.stroke();
      }
    }
  };

  const tick = () => {
    curX += (targetX - curX) * 0.1;
    curY += (targetY - curY) * 0.1;
    draw();
    if (Math.hypot(targetX - curX, targetY - curY) > 0.4) {
      rafId = requestAnimationFrame(tick);
    } else {
      settled = true;
    }
  };

  const onMove = (e: PointerEvent) => {
    if (prefersReducedMotion()) return;
    targetX = e.clientX;
    targetY = e.clientY;
    if (settled) {
      settled = false;
      rafId = requestAnimationFrame(tick);
    }
  };

  resize();
  settled = true;
  window.addEventListener("resize", resize);
  window.addEventListener("pointermove", onMove, { passive: true });
  cleanupCanvas = () => {
    cancelAnimationFrame(rafId);
    window.removeEventListener("resize", resize);
    window.removeEventListener("pointermove", onMove);
  };
}

let statusTimer: ReturnType<typeof setTimeout> | undefined;
let disposed = false;
async function pollStatus() {
  if (locked.value || !isAdmin.value) {
    if (!disposed) statusTimer = setTimeout(pollStatus, 5000);
    return;
  }
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

let resizeObs: ResizeObserver | undefined;
onMounted(() => {
  if (authenticated) pollStatus();
  mountBlueprintCanvas();
  nextTick(syncDomainGlider);
  if (typeof ResizeObserver !== "undefined" && domainNavEl.value) {
    resizeObs = new ResizeObserver(syncDomainGlider);
    resizeObs.observe(domainNavEl.value);
  }
});
watch(isDark, () => {
  window.dispatchEvent(new Event("resize"));
});
watch(
  () => [route.path, navigation.value.length],
  () => {
    sheetOpen.value = false;
    nextTick(syncDomainGlider);
  }
);
onBeforeUnmount(() => {
  disposed = true;
  if (statusTimer) clearTimeout(statusTimer);
  cleanupCanvas?.();
  resizeObs?.disconnect();
});
</script>

<style scoped>
.workspace-shell {
  --sidebar-w: 0px;
  position: relative;
  min-height: 100dvh;
  background: var(--c-canvas);
}
.union-grid-canvas {
  position: fixed;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 0;
}

/* Open-Sky Corner Bar (Zero full-width border, pure open space) */
.sky-header {
  position: relative;
  z-index: var(--z-tools);
  max-width: 1400px;
  margin: 0 auto;
  padding: 26px clamp(20px, 4vw, 48px) 12px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
}
.sky-header__left {
  display: flex;
  align-items: center;
  gap: 14px;
}
.union-logo {
  display: inline-flex;
  align-items: center;
  color: var(--c-ink);
}
.union-logo__svg {
  height: 22px;
  width: auto;
  overflow: visible;
}
.union-logo__word {
  font: 800 31px/1 var(--font-display);
  letter-spacing: -0.04em;
}
.union-logo__capsule,
.union-logo__trail,
.union-logo__dot {
  transition:
    transform var(--duration-slow) var(--ease-out),
    width var(--duration-slow) var(--ease-out);
}
@media (hover: hover) and (pointer: fine) {
  .union-logo:hover .union-logo__capsule {
    width: 54px;
  }
  .union-logo:hover .union-logo__trail {
    transform: translateX(20px);
  }
  .union-logo:hover .union-logo__dot {
    transform: translateX(10px) scale(1.18);
  }
}

.sky-coord {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 12.5px;
  color: var(--c-muted);
  white-space: nowrap;
}
.coord-slash {
  opacity: 0.35;
}
.coord-page {
  color: var(--c-ink);
  font-weight: 600;
}

.sky-header__right {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 82%, transparent);
  backdrop-filter: blur(12px);
  border: 1px solid var(--c-line);
  white-space: nowrap;
}
.theme-switch-pair {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0 6px;
  font: 660 12px/1 var(--font-display);
}
.theme-word {
  color: var(--c-ink);
  opacity: 0.3;
  transition: opacity var(--duration-medium) var(--ease-out);
}
.theme-word.is-on {
  opacity: 1;
}
@media (hover: hover) and (pointer: fine) {
  .theme-word:hover {
    opacity: 0.75;
  }
}
.theme-slash {
  color: var(--c-muted);
  opacity: 0.35;
  font-size: 11px;
}

.union-search-btn,
.workspace-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 30px;
  padding: 0 11px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font: 560 12px/1 var(--font-ui);
  white-space: nowrap;
  flex-shrink: 0;
  transition:
    background-color var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}
.union-search-btn svg,
.workspace-icon svg {
  width: 14px;
  height: 14px;
}
.union-search-btn kbd {
  color: var(--c-muted);
  font: 500 10.5px/1 var(--font-mono);
  margin-left: 2px;
}
@media (hover: hover) and (pointer: fine) {
  .union-search-btn:hover,
  .workspace-icon:hover {
    background: var(--c-ink);
    color: var(--c-surface);
  }
  .union-search-btn:hover kbd {
    color: var(--c-surface);
  }
}
.workspace-icon.task-live {
  color: var(--c-ok);
}

/* Floating Bottom Machined Obsidian Command Dock */
.floating-dock-anchor {
  position: fixed;
  z-index: 1200;
  left: 16px;
  right: 16px;
  bottom: 20px;
  display: flex;
  justify-content: center;
  pointer-events: none;
}
.floating-dock {
  pointer-events: auto;
  max-width: 100%;
  overflow-x: auto;
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 6px 8px;
  border-radius: 999px;
  background: rgb(12 12 14 / 0.9);
  backdrop-filter: blur(22px) saturate(160%);
  border: 1px solid rgb(255 255 255 / 0.12);
  box-shadow:
    0 24px 60px -12px rgb(0 0 0 / 0.35),
    inset 0 1px 0 rgb(255 255 255 / 0.14);
}
.dock-domains {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 2px;
}
.dock-glider {
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  border-radius: 999px;
  background: #ffffff;
  pointer-events: none;
  transition:
    transform var(--duration-medium) var(--ease-out),
    width var(--duration-medium) var(--ease-out),
    opacity var(--duration-fast) ease;
}
.dock-domain-pill {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  min-height: 34px;
  padding: 0 14px;
  border-radius: 999px;
  color: rgb(255 255 255 / 0.65);
  font-size: 13px;
  font-weight: 540;
  white-space: nowrap;
  transition:
    color var(--duration-medium) var(--ease-out),
    opacity var(--duration-medium) var(--ease-out);
}
.dock-domain-pill.is-active {
  color: #0a0a0a;
  font-weight: 660;
}
@media (hover: hover) and (pointer: fine) {
  .dock-domain-pill:not(.is-active):hover {
    color: #ffffff;
  }
}

.dock-divider {
  width: 1px;
  height: 20px;
  background: rgb(255 255 255 / 0.16);
  flex: none;
}

.dock-subpages {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding-right: 6px;
}
.dock-sub-link {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 34px;
  padding: 0 11px;
  border-radius: 999px;
  color: rgb(255 255 255 / 0.65);
  font-size: 13px;
  font-weight: 520;
  white-space: nowrap;
  transition:
    color var(--duration-medium) var(--ease-out),
    background-color var(--duration-medium) var(--ease-out),
    opacity var(--duration-medium) var(--ease-out);
}
.union-nav-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: rgb(255 255 255 / 0.3);
  flex: none;
  transition:
    transform var(--duration-medium) var(--ease-out),
    background-color var(--duration-medium) var(--ease-out),
    width var(--duration-medium) var(--ease-out),
    border-radius var(--duration-medium) var(--ease-out);
}
.union-nav-label {
  transition: transform var(--duration-medium) var(--ease-out);
}
.dock-sub-link[aria-current="page"] {
  color: #ffffff;
  background: rgb(255 255 255 / 0.11);
  font-weight: 640;
}
.dock-sub-link[aria-current="page"] .union-nav-dot {
  width: 10px;
  border-radius: 3px;
  background: #29df7e;
}
@media (hover: hover) and (pointer: fine) {
  .dock-subpages:hover .dock-sub-link:not(:hover) {
    opacity: 0.45;
  }
  .dock-sub-link:hover {
    color: #ffffff;
  }
  .dock-sub-link:hover .union-nav-dot {
    background: #29df7e;
    transform: scale(1.25);
  }
}

.workspace-content {
  position: relative;
  z-index: 1;
  max-width: 1400px;
  margin: 0 auto;
  padding: 24px clamp(20px, 4vw, 48px) 120px;
  min-width: 0;
}
.mobile-trigger {
  display: none;
}
.public-error-content {
  min-height: 100dvh;
  padding: 28px;
}
.skip-link {
  position: fixed;
  z-index: 2000;
  top: 10px;
  left: 10px;
  transform: translateY(-160%);
  padding: 8px 14px;
  background: var(--c-surface);
}
.skip-link:focus {
  transform: none;
}

@media (max-width: 980px) {
  .floating-dock-anchor,
  .sky-coord {
    display: none;
  }
  .mobile-trigger {
    display: inline-flex;
  }
  .workspace-content {
    padding-top: 16px;
    padding-bottom: 64px;
  }
}
@media (max-width: 560px) {
  .theme-switch-pair,
  .union-search-btn span,
  .union-search-btn kbd {
    display: none;
  }
}
</style>
<style>
.el-drawer.workspace-mobile {
  background: var(--rail-bg);
  color: var(--rail-text);
  border-radius: 0;
}
.workspace-mobile .el-drawer__header,
.workspace-mobile .el-drawer__title,
.workspace-mobile .el-drawer__close-btn {
  color: var(--rail-text);
}
.workspace-mobile .el-drawer__body {
  padding-inline: 0;
}
</style>
