<template>
  <main ref="page" class="login-page">
    <section class="login-story" aria-labelledby="brand-title">
      <a href="#/login" class="login-brand" aria-label="PPEELink"
        ><img
          class="brand-logo"
          :src="logo"
          width="44"
          height="44"
          alt="PPEELink"
        /><span>PPEELink</span></a
      >
      <div class="login-statement">
        <h1 id="brand-title" data-enter>连接，自有方向。</h1>
        <p data-enter>在一处，掌握节点、订阅与网络。</p>
      </div>
      <svg
        class="signal-atlas"
        viewBox="0 0 800 480"
        fill="none"
        aria-hidden="true"
      >
        <g class="signal-path signal-path--muted">
          <path d="M-40 400C160 400 180 80 460 80S740 170 850 10" />
          <path d="M-40 430C200 430 210 150 470 150S680 300 850 80" />
          <path d="M-40 460C260 460 240 220 490 220S720 400 850 150" />
        </g>
        <g class="signal-path signal-path--signal">
          <path d="M-40 360C140 360 170 30 460 30S720 160 850 -20" />
          <circle cx="460" cy="30" r="6" />
          <circle cx="170" cy="230" r="4" />
        </g>
        <g class="signal-path signal-path--data">
          <path d="M-40 480C230 480 290 290 520 290S730 460 850 250" />
          <circle cx="520" cy="290" r="5" />
        </g>
      </svg>
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
        <p class="login-group">账户 / 登录</p>
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
import { ArrowRight, Moon, Sunny } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import { useMotion } from "@/motion";
const page = ref<HTMLElement>();
const { animate } = useMotion();
onMounted(() => {
  const elements = page.value?.querySelectorAll("[data-enter]");
  if (elements)
    animate(
      Array.from(elements),
      { opacity: 0, y: 24, clipPath: "inset(0 0 15% 0)" },
      {
        opacity: 1,
        y: 0,
        clipPath: "inset(0)",
        duration: 0.48,
        stagger: 0.04,
        ease: "power3.out",
        clearProps: "opacity,transform,clipPath",
      }
    );
  const lines = page.value?.querySelectorAll(".signal-path");
  if (lines)
    animate(
      Array.from(lines),
      { clipPath: "inset(0 100% 0 0)", opacity: 0.3 },
      {
        clipPath: "inset(0)",
        opacity: 1,
        duration: 0.48,
        stagger: 0.04,
        ease: "power3.inOut",
        clearProps: "clipPath,opacity",
      }
    );
});
</script>
<style scoped>
.login-page {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(0, 1fr);
  min-height: 100dvh;
}
.login-story {
  position: relative;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 24px 28px;
  background: var(--c-canvas);
  border-right: 1px solid var(--c-line);
}
.login-brand {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  width: fit-content;
  color: var(--c-ink);
  font: 400 24px/1 var(--font-display);
  position: relative;
  z-index: 1;
}
.login-statement {
  position: relative;
  z-index: 1;
  margin-top: clamp(32px, 12vh, 112px);
}
.login-statement h1 {
  font-size: clamp(40px, 4.8vw, 72px);
  line-height: 1.1;
  white-space: nowrap;
  margin-bottom: 16px;
  letter-spacing: -0.035em;
}
.login-statement p {
  font-size: 14px;
  color: var(--c-muted);
  max-width: 25ch;
}
.signal-atlas {
  width: calc(100% + 96px);
  height: auto;
  max-width: none;
  margin-left: -28px;
  min-height: 240px;
  margin-top: 24px;
}
.signal-path {
  stroke-width: 1.2;
}
.signal-path--muted {
  stroke: var(--c-line);
}
.signal-path--signal {
  stroke: var(--c-signal);
  stroke-width: 1.8;
}
.signal-path--data {
  stroke: var(--c-data);
}
.signal-path circle {
  fill: var(--c-canvas);
}
.login-form-side {
  display: grid;
  grid-template-rows: auto minmax(min-content, 1fr) 44px;
  padding: 24px 28px;
  background: var(--c-surface);
  min-width: 0;
}
.login-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  color: var(--c-muted);
  font-size: 14px;
}
.theme-toggle {
  display: grid;
  place-items: center;
  width: 44px;
  height: 36px;
  padding: 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  color: var(--c-ink);
  background: transparent;
}
.theme-toggle svg {
  width: 20px;
  height: 20px;
}
.theme-toggle:hover {
  background: var(--c-surface-2);
}
.login-form-wrap {
  width: 100%;
  max-width: 360px;
  align-self: center;
  margin: 20px auto;
}
.login-group {
  font: 400 13px/1.4 var(--font-mono);
  letter-spacing: 0.08em;
  color: var(--c-muted);
  margin-bottom: 16px;
}
.login-form-wrap h2 {
  font: 400 28px/1.1 var(--font-display);
  margin-bottom: 12px;
}
.login-intro {
  color: var(--c-muted);
  margin-bottom: 24px;
}
.login-form-wrap :deep(.el-form-item) {
  margin-bottom: 20px;
}
.login-form-wrap :deep(.el-form-item__label) {
  padding-bottom: 8px;
}
.default-account {
  min-height: 36px;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-data);
  font-size: 14px;
  text-decoration: underline;
  text-underline-offset: 4px;
}
.login-submit {
  width: 100%;
  margin-top: 16px;
  justify-content: space-between;
}
.login-submit svg {
  width: 20px;
  height: 20px;
  margin-left: 12px;
  vertical-align: middle;
}
.login-error {
  padding: 10px 12px;
  color: var(--c-bad);
  background: var(--c-surface-2);
  border-left: 2px solid var(--c-bad);
  font-size: 14px;
  margin-top: 12px;
}
.capslock-hint {
  color: var(--c-muted);
  font-size: 14px;
}
@media (min-width: 768px) and (max-width: 1023px) {
  .login-story,
  .login-form-side {
    display: flex;
    flex-direction: column;
    padding: 20px;
  }
  .login-statement h1 {
    font-size: 28px;
  }
  .login-page {
    grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  }
}
@media (max-width: 767px) {
  .login-page {
    grid-template-columns: minmax(0, 1fr);
  }
  .login-story {
    padding: 16px 12px 20px;
    min-height: 296px;
    border-right: 0;
    border-bottom: 1px solid var(--c-line);
  }
  .login-brand {
    font-size: 20px;
    gap: 8px;
  }
  .login-brand img {
    width: 36px;
    height: 36px;
  }
  .login-statement {
    margin-top: 24px;
  }
  .login-statement h1 {
    font-size: 28px;
    margin-bottom: 12px;
  }
  .login-statement p {
    max-width: 23ch;
    margin: 0;
  }
  .signal-atlas {
    position: absolute;
    width: 360px;
    min-height: 0;
    height: 216px;
    right: -120px;
    bottom: -80px;
    margin: 0;
    opacity: 0.65;
  }
  .login-form-side {
    padding: 16px 12px;
  }
  .login-form-wrap {
    margin: 24px 0 20px;
    max-width: none;
  }
  .login-intro {
    margin-bottom: 20px;
  }
}
</style>
