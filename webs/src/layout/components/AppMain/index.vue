<template>
  <section class="app-main">
    <router-view v-slot="{ Component, route }"
      ><transition name="workspace-page" mode="out-in"
        ><keep-alive :include="cachedViews"
          ><component
            :is="Component"
            :key="route.path" /></keep-alive></transition
    ></router-view>
  </section>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { useTagsViewStore } from "@/store";
const cachedViews = computed(() => useTagsViewStore().cachedViews);
</script>
<style scoped>
.app-main {
  position: relative;
  min-width: 0;
  width: 100%;
}
.workspace-page-enter-active {
  transition:
    opacity 0.3s,
    transform 0.45s cubic-bezier(0.16, 1, 0.3, 1);
}
.workspace-page-leave-active {
  transition:
    opacity 0.12s,
    transform 0.12s;
}
.workspace-page-enter-from {
  opacity: 0;
  transform: translateY(14px);
}
.workspace-page-leave-to {
  opacity: 0;
  transform: translateY(-5px);
}
@media (prefers-reduced-motion: reduce) {
  .workspace-page-enter-active,
  .workspace-page-leave-active {
    transition: none;
  }
}
</style>
