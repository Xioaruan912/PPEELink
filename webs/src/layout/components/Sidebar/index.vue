<template>
  <div :class="{ 'has-logo': sidebarLogo }">
    <!--混合布局-->
    <div class="flex w-full" v-if="layout == 'mix'">
      <SidebarLogo v-if="sidebarLogo" :collapse="!appStore.sidebar.opened" />
      <SidebarMixTopMenu class="flex-1" />
      <NavbarRight />
    </div>
    <!--左侧布局 || 顶部布局 -->
    <template v-else>
      <SidebarLogo v-if="sidebarLogo" :collapse="!appStore.sidebar.opened" />
      <el-scrollbar ref="menuRoot">
        <SidebarMenu :menu-list="permissionStore.routes" base-path="" />
      </el-scrollbar>
      <SidebarFooter v-if="layout === 'left' && appStore.sidebar.opened" />
      <NavbarRight v-if="layout === 'top'" />
    </template>
  </div>
</template>

<script setup lang="ts">
import { useSettingsStore, usePermissionStore, useAppStore } from "@/store";

const appStore = useAppStore();
const settingsStore = useSettingsStore();
const permissionStore = usePermissionStore();

const sidebarLogo = computed(() => settingsStore.sidebarLogo);
const layout = computed(() => settingsStore.layout);

// 菜单逐项入场编排
const menuRoot = ref<any>();
let staggerTimer: number | undefined;

const applyStagger = () => {
  const root: HTMLElement | undefined = menuRoot.value?.$el ?? menuRoot.value;
  if (!root) return;
  const items = root.querySelectorAll<HTMLElement>(
    ".el-menu-item, .el-sub-menu__title"
  );
  items.forEach((item, index) => {
    item.style.setProperty("--nav-i", String(Math.min(index, 12)));
  });
  root.classList.remove("nav-stagger");
  void root.offsetWidth; // restart the cascade
  root.classList.add("nav-stagger");
};

watch(
  () => permissionStore.routes,
  () => {
    if (staggerTimer) window.clearTimeout(staggerTimer);
    staggerTimer = window.setTimeout(() => nextTick(applyStagger), 140);
  }
);

onMounted(() => {
  nextTick(applyStagger);
});

onBeforeUnmount(() => {
  if (staggerTimer) window.clearTimeout(staggerTimer);
});
</script>

<style lang="scss" scoped>
.has-logo {
  display: flex;
  flex-direction: column;

  .el-scrollbar {
    flex: 1;
    min-height: 0;
  }
}
</style>
