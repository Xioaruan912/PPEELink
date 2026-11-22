<template>
  <main class="union-login">
    <header class="union-login__header">
      <a href="#/login" class="union-logo" aria-label="PPEELink">
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
            y="8"
            width="36"
            height="26"
            rx="13"
            stroke="currentColor"
            stroke-width="4.5"
          />
          <circle
            class="union-logo__dot"
            cx="122"
            cy="21"
            r="4"
            fill="var(--c-ok)"
          />
          <text
            x="150"
            y="33"
            fill="currentColor"
            class="union-logo__word union-logo__trail"
          >
            LINK
          </text>
        </svg>
      </a>

      <div class="theme-switch-pair">
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
    </header>

    <div class="union-login__grid" data-enter>
      <section class="union-login__hero" aria-labelledby="brand-title">
        <div class="hero-copy">
          <h1 id="brand-title" class="hero-headline">
            <span class="headline-mask"
              ><span class="headline-line" style="--i: 0"
                >Network</span
              ></span
            >
            <span class="headline-mask"
              ><span class="headline-line" style="--i: 1"
                >Architecture.</span
              ></span
            >
          </h1>
          <p class="hero-statement">
            多客户端订阅分发、全球边缘节点资产与实时链路遥测的统一空间。
          </p>
        </div>

        <dl class="hero-index" aria-label="系统架构能力">
          <div>
            <dt>DISPATCH</dt>
            <dd>订阅链接与多端自适应同步</dd>
          </div>
          <div>
            <dt>ASSETS</dt>
            <dd>节点探测、分组与下发控制</dd>
          </div>
          <div>
            <dt>TELEMETRY</dt>
            <dd>流媒体解锁与三网逐跳追踪</dd>
          </div>
        </dl>
      </section>

      <section class="union-login__panel" aria-labelledby="login-title">
        <div class="login-form-wrap">
          <h2 id="login-title">登录工作区</h2>
          <p class="login-form-hint">输入管理员或成员凭据以进入控制台。</p>
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
            <p v-if="loginError" class="login-error" role="alert">
              {{ loginError }}
            </p>
            <UiButton
              class="login-submit"
              type="submit"
              variant="primary"
              :loading="loading"
              ><span>{{ loading ? "正在验证…" : "进入工作区" }}</span
              ><ArrowRight v-if="!loading" aria-hidden="true"
            /></UiButton>
          </el-form>
        </div>
      </section>
    </div>
  </main>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import type { FormInstance, FormRules } from "element-plus";
import ArrowRight from "~icons/ph/arrow-right";
import { UiButton } from "@/ui";

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
    const { mustChangePassword } = await userStore.login(loginData.value);
    if (mustChangePassword) {
      await router.push("/system/user/set");
      return;
    }
    const redirect =
      typeof route.query.redirect === "string" &&
      route.query.redirect.startsWith("/") &&
      !route.query.redirect.startsWith("//")
        ? route.query.redirect
        : "/";
    await router.push(redirect);
  } catch {
    loginError.value = "登录未成功，请检查账户、密码及服务连接后重试。";
  } finally {
    loading.value = false;
  }
}
</script>

<style scoped>
.union-login {
  min-height: 100dvh;
  display: grid;
  grid-template-rows: auto 1fr;
  background-color: var(--c-canvas);
}
.union-login__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 24px clamp(20px, 4vw, 56px);
  border-bottom: 1px solid var(--c-line);
  background: color-mix(in srgb, var(--c-canvas) 90%, transparent);
}
.union-logo {
  display: inline-block;
  color: var(--c-ink);
}
.union-logo__svg {
  height: 26px;
  width: auto;
  overflow: visible;
}
.union-logo__word {
  font: 800 32px/1 var(--font-display);
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
    width: 58px;
  }
  .union-logo:hover .union-logo__trail {
    transform: translateX(22px);
  }
  .union-logo:hover .union-logo__dot {
    transform: translateX(11px) scale(1.2);
  }
}

.theme-switch-pair {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font: 700 13.5px/1 var(--font-display);
}
.theme-word {
  color: var(--c-ink);
  opacity: 0.28;
  transition: opacity var(--duration-medium) var(--ease-out);
}
.theme-word.is-on {
  opacity: 1;
}
.theme-slash {
  color: var(--c-muted);
  opacity: 0.35;
}

.union-login__grid {
  max-width: 1180px;
  width: calc(100% - 40px);
  margin: auto;
  padding: clamp(32px, 5vw, 64px);
  display: grid;
  grid-template-columns: 1.2fr 0.95fr;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
}
.union-login__hero {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 56px;
  padding-right: clamp(24px, 4vw, 56px);
  border-right: 1px solid var(--c-line);
}
.hero-headline {
  font: 760 clamp(44px, 6.2vw, 84px)/0.94 var(--font-display);
  letter-spacing: -0.055em;
  color: var(--c-ink);
}
.headline-mask {
  display: block;
  overflow: hidden;
  padding-bottom: 0.08em;
  margin-bottom: -0.08em;
}
.headline-line {
  display: block;
}
@media (prefers-reduced-motion: no-preference) {
  .headline-line {
    animation: mask-rise var(--duration-reveal) var(--ease-out) both;
    animation-delay: calc(var(--i, 0) * 120ms + 80ms);
  }
}
.hero-statement {
  margin-top: 24px;
  max-width: 38ch;
  font-size: 16px;
  line-height: 1.65;
  color: var(--c-muted);
}
.hero-index {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
  padding-top: 24px;
  border-top: 1px solid var(--c-line);
}
.hero-index dt {
  font: 700 12px/1.2 var(--font-display);
  letter-spacing: 0.04em;
  color: var(--c-ink);
}
.hero-index dd {
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--c-muted);
}

.union-login__panel {
  display: grid;
  place-items: center;
  padding-left: clamp(24px, 4vw, 56px);
}
.login-form-wrap {
  width: min(100%, 360px);
}
.login-form-wrap h2 {
  font-size: 28px;
  font-weight: 700;
  letter-spacing: -0.035em;
  color: var(--c-ink);
}
.login-form-hint {
  margin: 8px 0 28px;
  font-size: 13.5px;
  color: var(--c-muted);
}
.login-submit {
  width: 100%;
  min-height: 44px;
  margin-top: 12px;
}
.login-submit :deep(span) {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
.login-error,
.capslock-hint {
  font-size: 13px;
  color: var(--c-bad);
}

@media (max-width: 860px) {
  .union-login__grid {
    grid-template-columns: 1fr;
    gap: 32px;
  }
  .union-login__hero {
    padding-right: 0;
    padding-bottom: 28px;
    border-right: 0;
    border-bottom: 1px solid var(--c-line);
  }
  .union-login__panel {
    padding-left: 0;
  }
}
</style>
