<template>
  <section class="error-page">
    <p class="error-code" data-enter>401</p>
    <h1 data-enter>无权访问</h1>
    <p class="error-description" data-enter>当前账户没有访问此页面的权限。</p>
    <UiButton variant="primary" @click="router.push('/dashboard')"
      >返回首页</UiButton
    >
  </section>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { UiButton } from "@/ui";
const router = useRouter();
</script>
<style scoped>
.error-page {
  min-height: 65dvh;
  padding-top: 24px;
  max-width: 720px;
}
.error-code {
  color: var(--c-signal);
  font: 400 clamp(112px, 22vw, 240px)/0.9 var(--font-display);
  margin: 0 0 48px;
}
.error-page h1 {
  font-size: clamp(40px, 5vw, 72px);
  margin-bottom: 24px;
}
.error-description {
  color: var(--c-muted);
  margin-bottom: 40px;
}
</style>
