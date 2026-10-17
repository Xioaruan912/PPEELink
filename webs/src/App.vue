<template>
  <el-config-provider :locale="appStore.locale" :size="size"
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
</script>
