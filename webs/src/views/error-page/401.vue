<template>
  <section class="error-page">
    <p class="error-code" data-enter>401</p>
    <PageHeader
      class="error-header"
      eyebrow=""
      title="无权访问"
      description="当前账户没有访问此页面的权限。"
    />
    <UiButton
      variant="primary"
      @click="router.push(authenticated ? '/dashboard' : '/login')"
      >{{ authenticated ? "返回首页" : "前往登录" }}</UiButton
    >
  </section>
</template>
<script setup lang="ts">
import { useRouter } from "vue-router";
import { PageHeader, UiButton } from "@/ui";
const router = useRouter();
const authenticated = !!localStorage.getItem("accessToken");
</script>
<style scoped>
.error-page {
  min-height: 65dvh;
  padding-top: 16px;
  max-width: 720px;
}
.error-code {
  color: var(--c-signal-text);
  font: 400 clamp(112px, 22vw, 240px)/0.9 var(--font-display);
  margin: 0 0 28px;
}
.error-header {
  padding: 0;
  border: 0;
  margin-bottom: 24px;
}
</style>
