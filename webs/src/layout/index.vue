<template>
  <div class="workspace-shell">
    <a href="#workspace-content" class="skip-link" @click.prevent="focusMain"
      >跳到主要内容</a
    >
    <header class="workspace-header">
      <router-link
        to="/dashboard"
        class="workspace-brand"
        aria-label="PPEELink 首页"
        ><span class="brand-symbol"><i></i><i></i><i></i></span
        ><span>PPEELink<span class="brand-period">.</span></span></router-link
      >
      <nav class="primary-navigation" aria-label="主导航">
        <router-link
          v-for="item in primaryNav"
          :key="item.path"
          :to="item.path"
          :class="{ active: isActive(item.path) }"
          ><WorkIcon :name="item.icon" /><span>{{ item.label }}</span
          ><i v-if="isActive(item.path)" class="nav-active-marker"></i
        ></router-link>
      </nav>
      <div class="workspace-header-actions">
        <button
          class="search-trigger"
          aria-label="快速跳转"
          @click="openCommands"
        >
          <WorkIcon name="search" /><span>搜索</span>
        </button>
        <el-dropdown trigger="click"
          ><button class="profile-trigger" aria-label="账户菜单">
            <span>{{
              (userStore.user.username || "A").charAt(0).toUpperCase()
            }}</span></button
          ><template #dropdown
            ><el-dropdown-menu
              ><el-dropdown-item @click="router.push('/system/user/set')"
                >账户设置</el-dropdown-item
              ><el-dropdown-item divided @click="userStore.logout()"
                >退出登录</el-dropdown-item
              ></el-dropdown-menu
            ></template
          ></el-dropdown
        >
      </div>
    </header>
    <div v-if="sectionLinks.length > 1" class="workspace-subbar">
      <nav
        v-if="sectionLinks.length > 1"
        class="section-navigation"
        aria-label="页面"
      >
        <router-link
          v-for="item in sectionLinks"
          :key="item.path"
          :to="item.path"
          :class="{ active: route.path === item.path }"
          >{{ item.label }}</router-link
        >
      </nav>
    </div>
    <main id="workspace-content" class="workspace-content" tabindex="-1">
      <header v-if="route.path !== '/dashboard'" class="workspace-page-heading">
        <div>
          <h1>{{ currentInfo.label }}</h1>
          <p>{{ currentInfo.description }}</p>
        </div>
        <span class="page-heading-symbol"
          ><WorkIcon :name="currentInfo.icon"
        /></span>
      </header>
      <AppMain />
    </main>
    <nav class="command-dock" aria-label="快捷工具">
      <button @click="testVisible = true" aria-label="查看测试任务">
        <WorkIcon name="pulse" /><span>任务</span>
      </button>
      <i></i
      ><button
        class="dock-command"
        @click="openCommands"
        aria-label="打开快捷跳转"
      >
        <WorkIcon name="search" /><span>搜索</span></button
      ><i></i>
      <button
        @click="toggleTheme"
        :aria-label="isDark ? '切换浅色外观' : '切换深色外观'"
      >
        <WorkIcon :name="isDark ? 'sun' : 'moon'" /><span>外观</span>
      </button>
      <button
        @click="settingsStore.settingsVisible = true"
        aria-label="界面设置"
      >
        <WorkIcon name="settings" /><span>设置</span>
      </button>
    </nav>
    <el-dialog
      v-model="commandVisible"
      title="快速跳转"
      class="command-dialog"
      width="580px"
      :show-close="false"
      @opened="focusCommand"
    >
      <div class="command-search">
        <WorkIcon name="search" /><input
          ref="commandInput"
          v-model="query"
          placeholder="输入页面名称，或搜索功能…"
          aria-label="搜索页面"
          @keydown.down.prevent="
            selected = Math.min(selected + 1, filteredCommands.length - 1)
          "
          @keydown.up.prevent="selected = Math.max(selected - 1, 0)"
          @keydown.enter.prevent="goSelected"
        /><kbd>Esc</kbd>
      </div>
      <div class="command-results">
        <p>
          页面 <span>{{ filteredCommands.length }}</span>
        </p>
        <button
          v-for="(item, index) in filteredCommands"
          :key="item.path"
          :class="{ highlighted: selected === index }"
          @click="navigate(item.path)"
          @pointermove="selected = index"
        >
          <WorkIcon :name="item.icon" /><span
            ><strong>{{ item.label }}</strong
            ><small>{{ item.group }}</small></span
          ><WorkIcon name="arrow" />
        </button>
        <div v-if="!filteredCommands.length" class="command-empty">
          没有找到页面，试试「节点」「订阅」或「测速」。
        </div>
      </div>
      <template #footer
        ><span class="command-help">↑ ↓ 选择 <span>Enter 打开</span></span
        ><span>随时按 Ctrl K</span></template
      >
    </el-dialog>
    <TestStatusDialog v-model:visible="testVisible" />
    <Settings />
  </div>
</template>
<script setup lang="ts">
import {
  computed,
  ref,
  watch,
  onMounted,
  onBeforeUnmount,
  nextTick,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import { usePermissionStore, useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import AppMain from "./components/AppMain/index.vue";
import Settings from "./components/Settings/index.vue";
import TestStatusDialog from "@/components/TestStatusDialog.vue";
import WorkIcon from "@/components/workspace/WorkIcon.vue";
const route = useRoute(),
  router = useRouter(),
  permission = usePermissionStore(),
  settingsStore = useSettingsStore(),
  userStore = useUserStore();
const isDark = computed(() => settingsStore.theme === ThemeEnum.DARK);
function toggleTheme() {
  settingsStore.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function focusMain() {
  document.getElementById("workspace-content")?.focus();
}
const primaryNav = [
  { path: "/dashboard", label: "总览", icon: "grid" },
  { path: "/subcription/nodes", label: "节点", icon: "network" },
  { path: "/subcription/subs", label: "订阅", icon: "link" },
  { path: "/test/speed", label: "测速", icon: "pulse" },
  { path: "/template/rules", label: "规则", icon: "rules" },
];
const pageDetails: Record<
  string,
  { label: string; group: string; icon: string; description: string }
> = {
  "/dashboard": {
    label: "网络总览",
    group: "工作区",
    icon: "globe",
    description: "",
  },
  "/subcription/nodes": {
    label: "节点",
    group: "连接管理",
    icon: "network",
    description: "添加、分组和管理你的节点。",
  },
  "/subcription/subs": {
    label: "订阅",
    group: "连接管理",
    icon: "link",
    description: "选择节点，生成设备可用的订阅链接。",
  },
  "/subcription/collections": {
    label: "订阅合集",
    group: "连接管理",
    icon: "link",
    description: "把多个订阅合并为一个链接。",
  },
  "/subcription/airport": {
    label: "机场订阅",
    group: "连接管理",
    icon: "globe",
    description: "添加机场订阅，自动更新节点。",
  },
  "/subcription/clients": {
    label: "客户端",
    group: "连接管理",
    icon: "diagonal",
    description: "为你的设备找到合适的连接工具。",
  },
  "/test/speed": {
    label: "速度测试",
    group: "测速",
    icon: "pulse",
    description: "选择节点，测试实际下载速度。",
  },
  "/test/unlock": {
    label: "服务检测",
    group: "测速",
    icon: "globe",
    description: "查看节点能否访问常用网站和视频服务。",
  },
  "/test/tcp": {
    label: "连接测试",
    group: "测速",
    icon: "network",
    description: "检查不同地区能否连接到节点。",
  },
  "/template/list": {
    label: "配置模板",
    group: "规则",
    icon: "rules",
    description: "保存和修改客户端的连接设置。",
  },
  "/template/builder": {
    label: "编辑模板",
    group: "规则",
    icon: "rules",
    description: "按需修改连接设置并保存。",
  },
  "/template/rules": {
    label: "访问规则",
    group: "规则",
    icon: "rules",
    description: "定义连接的去向，管理访问规则与规则集。",
  },
  "/system/user/set": {
    label: "账户设置",
    group: "我的账户",
    icon: "settings",
    description: "修改用户名和密码。",
  },
};
const currentInfo = computed(
  () =>
    pageDetails[route.path] || {
      label: String(route.meta.title || "工作区"),
      group: "工作区",
      icon: "grid",
      description: "",
    }
);
const commands = computed(() => {
  const result = [{ path: "/dashboard", ...pageDetails["/dashboard"] }];
  for (const parent of permission.routes)
    for (const child of parent.children || []) {
      const path = child.path.startsWith("/")
        ? child.path
        : `${parent.path}/${child.path}`.replace(/\/+/g, "/");
      if (
        path === "/dashboard" ||
        (!pageDetails[path] && (parent.meta?.hidden || child.meta?.hidden))
      )
        continue;
      result.push({
        path,
        ...(pageDetails[path] || {
          label: String(child.meta?.title || child.name),
          group: String(parent.meta?.title || "工作区"),
          icon: "grid",
          description: "",
        }),
      });
    }
  return result;
});
const sectionLinks = computed(() =>
  commands.value.filter(
    (item) => item.path.split("/")[1] === route.path.split("/")[1]
  )
);
function isActive(path: string) {
  if (path === "/dashboard") return route.path === path;
  if (path === "/subcription/nodes")
    return [
      "/subcription/nodes",
      "/subcription/airport",
      "/subcription/clients",
    ].includes(route.path);
  if (path === "/subcription/subs")
    return ["/subcription/subs", "/subcription/collections"].includes(
      route.path
    );
  return route.path.split("/")[1] === path.split("/")[1];
}
const commandVisible = ref(false),
  testVisible = ref(false),
  query = ref(""),
  selected = ref(0),
  commandInput = ref<HTMLInputElement>();
const filteredCommands = computed(() =>
  commands.value.filter((item) =>
    `${item.label} ${item.group} ${item.path}`
      .toLowerCase()
      .includes(query.value.toLowerCase())
  )
);
watch(query, () => (selected.value = 0));
watch(selected, () =>
  nextTick(() => {
    document
      .querySelector(".command-results .highlighted")
      ?.scrollIntoView({ block: "nearest" });
  })
);
function openCommands() {
  query.value = "";
  selected.value = 0;
  commandVisible.value = true;
}
function focusCommand() {
  nextTick(() => commandInput.value?.focus());
}
function navigate(path: string) {
  commandVisible.value = false;
  router.push(path);
}
function goSelected() {
  const item = filteredCommands.value[selected.value];
  if (item) navigate(item.path);
}
function shortcut(event: KeyboardEvent) {
  if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === "k") {
    event.preventDefault();
    commandVisible.value ? (commandVisible.value = false) : openCommands();
  }
}
onMounted(() => window.addEventListener("keydown", shortcut));
onBeforeUnmount(() => window.removeEventListener("keydown", shortcut));
</script>
