<template>
  <main ref="page" class="login-page">
    <section class="login-story" aria-labelledby="brand-title">
      <div class="aurora" aria-hidden="true">
        <i class="aurora-a"></i><i class="aurora-b"></i><i class="aurora-c"></i>
      </div>
      <a href="#/login" class="login-brand" aria-label="PPEELink"
        ><img
          class="brand-logo"
          :src="logo"
          width="36"
          height="36"
          alt="PPEELink"
        /><span>PPEELink</span></a
      >
      <div class="login-statement">
        <h1 id="brand-title" data-enter>
          订阅、节点与规则，<br /><em>一处掌控。</em>
        </h1>
        <p data-enter>自托管的代理订阅控制台，给需要掌握自己网络的人。</p>
      </div>
    </section>
    <section class="login-form-side" aria-labelledby="login-title">
      <header class="login-topline">
        <span>个人网络工作台</span
        ><el-tooltip
          :content="isDark ? '切换浅色主题' : '切换深色主题'"
          :trigger="['hover', 'focus']"
          :trigger-keys="[]"
          ><button
            class="theme-toggle"
            :aria-label="isDark ? '切换浅色主题' : '切换深色主题'"
            @click="toggleTheme"
          >
            <component :is="isDark ? Sunny : Moon" /></button
        ></el-tooltip>
      </header>
      <div class="login-form-wrap" data-enter>
        <h2 id="login-title">欢迎回来</h2>
        <p class="login-intro">登录你的 PPEELink。</p>
        <el-form
          ref="loginFormRef"
          :model="loginData"
          :rules="loginRules"
          label-position="top"
          @submit.prevent="handleLogin"
        >
          <el-form-item label="用户名" prop="username" for="login-username"
            ><el-input
              id="login-username"
              v-model="loginData.username"
              name="username"
              placeholder="输入用户名"
              autocomplete="username"
              size="large"
          /></el-form-item>
          <el-form-item label="密码" prop="password" for="login-password"
            ><el-input
              id="login-password"
              v-model="loginData.password"
              name="password"
              placeholder="输入密码"
              autocomplete="current-password"
              type="password"
              show-password
              size="large"
              @keyup="checkCapslock"
            /><span v-if="isCapslock" class="capslock-hint" role="status"
              >大写锁定已开启</span
            ></el-form-item
          >
          <button type="button" class="default-account" @click="quickFill">
            使用默认账户
          </button>
          <p v-if="loginError" class="login-error" role="alert">
            {{ loginError }}
          </p>
          <UiButton
            class="login-submit"
            type="submit"
            variant="primary"
            :loading="loading"
            ><span>{{ loading ? "正在登录…" : "登录" }}</span
            ><ArrowRight v-if="!loading" aria-hidden="true"
          /></UiButton>
        </el-form>
      </div>
    </section>
  </main>
</template>
<script setup lang="ts">
import { computed, ref, onMounted } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import type { FormInstance, FormRules } from "element-plus";
const userStore = useUserStore(),
  settingsStore = useSettingsStore(),
  router = useRouter(),
  route = useRoute();
const loading = ref(false),
  loginError = ref(""),
  isCapslock = ref(false),
  loginFormRef = ref<FormInstance>();
const loginData = ref({ username: "", password: "" });
const loginRules: FormRules = {
  username: [{ required: true, message: "请输入用户名", trigger: "blur" }],
  password: [
    { required: true, message: "请输入密码", trigger: "blur" },
    { min: 6, message: "密码至少需要 6 个字符", trigger: "blur" },
  ],
};
const isDark = computed(() => settingsStore.theme === ThemeEnum.DARK);
function toggleTheme() {
  settingsStore.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}
function quickFill() {
  loginData.value = { username: "admin", password: "123456" };
  loginFormRef.value?.clearValidate();
}
function checkCapslock(event: KeyboardEvent) {
  isCapslock.value = event.getModifierState("CapsLock");
}
async function handleLogin() {
  if (
    loading.value ||
    !(await loginFormRef.value?.validate().catch(() => false))
  )
    return;
  loading.value = true;
  loginError.value = "";
  try {
    await userStore.login(loginData.value);
    const redirect =
      typeof route.query.redirect === "string" &&
      route.query.redirect.startsWith("/") &&
      !route.query.redirect.startsWith("//")
        ? route.query.redirect
        : "/dashboard";
    await router.push(redirect);
  } catch {
    loginError.value = "登录未成功，请检查账户、密码及服务连接后重试。";
  } finally {
    loading.value = false;
  }
}
import logo from "@/assets/logo.svg";
import ArrowRight from "~icons/ph/arrow-right";
import Moon from "~icons/ph/moon-stars";
import Sunny from "~icons/ph/sun";
import { UiButton } from "@/ui";
import { useMotion } from "@/motion";
const page = ref<HTMLElement>();
const { animate } = useMotion();
onMounted(() => {
  const elements = page.value?.querySelectorAll("[data-enter]");
  if (elements)
    animate(
      Array.from(elements),
      { opacity: 0, y: 32, filter: "blur(10px)" },
      {
        opacity: 1,
        y: 0,
        filter: "blur(0px)",
        duration: 0.9,
        stagger: 0.08,
        ease: "expo.out",
        clearProps: "opacity,transform,filter",
      }
    );
});
</script>
<style scoped>
.login-page {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(420px, 1fr);
  min-height: 100dvh;
  background: var(--c-canvas);
}
.login-story {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  margin: 12px 0 12px 12px;
  padding: 32px;
  overflow: hidden;
  border-radius: 24px;
  background: #0b0c0e;
  color: #eceef2;
}
/* Three slow-moving colour fields; frozen for reduced motion. */
.aurora {
  position: absolute;
  inset: 0;
  filter: blur(60px) saturate(1.2);
  opacity: 0.9;
}
.aurora i {
  position: absolute;
  border-radius: 50%;
}
.aurora-a {
  width: 60%;
  height: 60%;
  right: -10%;
  top: -15%;
  background: #c8f03a;
  opacity: 0.55;
  animation: drift-a 18s ease-in-out infinite alternate;
}
.aurora-b {
  width: 55%;
  height: 55%;
  left: -15%;
  bottom: -20%;
  background: #2f6bff;
  opacity: 0.55;
  animation: drift-b 22s ease-in-out infinite alternate;
}
.aurora-c {
  width: 35%;
  height: 35%;
  left: 40%;
  top: 40%;
  background: #3fd18f;
  opacity: 0.35;
  animation: drift-a 26s ease-in-out infinite alternate-reverse;
}
@keyframes drift-a {
  to {
    transform: translate(-18%, 22%) scale(1.15);
  }
}
@keyframes drift-b {
  to {
    transform: translate(24%, -14%) scale(1.1);
  }
}
.login-story::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(180deg, transparent 40%, rgb(11 12 14 / 0.75));
}
.login-brand {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 12px;
  width: fit-content;
  color: inherit;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
}
.login-statement {
  position: relative;
  z-index: 1;
}
.login-statement h1 {
  margin-bottom: 16px;
  font-size: clamp(40px, 4.6vw, 68px);
  line-height: 1.12;
  letter-spacing: -0.02em;
}
.login-statement em {
  font-style: normal;
  color: #c8f03a;
}
.login-statement p {
  max-width: 32ch;
  color: rgb(236 238 242 / 0.72);
  font-size: 15px;
}
.login-form-side {
  display: grid;
  grid-template-rows: auto minmax(min-content, 1fr) 44px;
  padding: 24px 40px;
  min-width: 0;
}
.login-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--c-muted);
  font-size: 13px;
}
.theme-toggle {
  display: grid;
  place-items: center;
  width: 36px;
  height: 36px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  color: var(--c-ink);
  background: var(--c-surface);
}
.theme-toggle svg {
  width: 18px;
  height: 18px;
}
.theme-toggle:hover {
  background: var(--c-surface-2);
}
.login-form-wrap {
  width: 100%;
  max-width: 380px;
  align-self: center;
  margin: 20px auto;
}
.login-form-wrap h2 {
  margin-bottom: 8px;
  font-size: 32px;
  font-weight: 600;
  letter-spacing: -0.02em;
}
.login-intro {
  color: var(--c-muted);
  margin-bottom: 28px;
}
.login-form-wrap :deep(.el-form-item) {
  margin-bottom: 18px;
}
.login-form-wrap :deep(.el-form-item__label) {
  padding-bottom: 6px;
}
.login-form-wrap :deep(.el-input__wrapper) {
  min-height: 44px;
}
.default-account {
  min-height: 32px;
  padding: 0;
  color: var(--c-muted);
  font-size: 13px;
  text-decoration: underline;
  text-underline-offset: 4px;
}
.default-account:hover {
  color: var(--c-ink);
}
.login-submit {
  position: relative;
  width: 100%;
  min-height: 46px;
  margin-top: 16px;
}
.login-submit svg {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 30px;
  height: 30px;
  padding: 7px;
  border-radius: 50%;
  background: var(--c-on-signal);
  color: var(--c-signal);
  transition: transform var(--duration-medium) var(--ease-spring);
}
.login-submit:hover svg {
  transform: translateX(-3px);
}
.login-error {
  padding: 10px 12px;
  margin-top: 12px;
  border: 1px solid color-mix(in srgb, var(--c-bad) 40%, var(--c-line));
  border-radius: var(--radius-control);
  color: var(--c-bad);
  background: color-mix(in srgb, var(--c-bad) 8%, var(--c-surface));
  font-size: 14px;
}
.capslock-hint {
  color: var(--c-muted);
  font-size: 13px;
}
@media (prefers-reduced-motion: reduce) {
  .aurora i {
    animation: none;
  }
}
@media (max-width: 1023px) {
  .login-page {
    grid-template-columns: minmax(0, 1fr);
  }
  .login-story {
    min-height: 320px;
    margin: 12px 12px 0;
    padding: 24px;
  }
  .login-statement h1 {
    font-size: 36px;
  }
  .login-form-side {
    padding: 20px 16px 24px;
  }
  .login-form-wrap {
    margin: 24px auto;
  }
}
</style>
