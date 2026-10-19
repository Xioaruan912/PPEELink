<template>
  <el-config-provider
    :locale="appStore.locale"
    :size="size"
    :message="messageConfig"
    ><router-view
  /></el-config-provider>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { SizeEnum } from "@/enums/SizeEnum";
import { useAppStore, useSettingsStore } from "@/store";
const appStore = useAppStore();
useSettingsStore();
const size = computed(() => appStore.size as SizeEnum);
// 提示出现在顶部导航下方：Element Plus 在鼠标悬停时会暂停倒计时，
// 盖在导航上时鼠标常停在那里，看起来就像提示一直不消失。另给一个关闭按钮。
const messageConfig = { offset: 76, showClose: true, grouping: true };
</script>
