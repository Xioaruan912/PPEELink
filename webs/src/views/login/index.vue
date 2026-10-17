<template>
  <main class="access-page">
    <section class="access-story">
      <router-link to="/" class="workspace-brand access-brand"
        ><span class="brand-symbol"><i></i><i></i><i></i></span
        ><span>PPEELink<span class="brand-period">.</span></span></router-link
      >
      <div class="access-story-copy">
        <h1><span>把连接，</span><span>掌握在手中。</span></h1>
        <p>在一个地方，管理节点和订阅。</p>
      </div>
      <div class="access-globe"><NetworkGlobe /></div>
    </section>
    <section class="access-form-side">
      <header class="access-topline">
        <span>账户登录</span
        ><button
          class="access-appearance"
          @click="toggleTheme"
          :aria-label="isDark ? '切换浅色外观' : '切换深色外观'"
        >
          <WorkIcon :name="isDark ? 'sun' : 'moon'" />
        </button>
      </header>
      <div class="access-form-wrap">
        <span class="access-entry-mark"><WorkIcon name="diagonal" /></span>
        <h2>登录</h2>
        <el-form
          ref="loginFormRef"
          :model="loginData"
          :rules="loginRules"
          label-position="top"
          class="access-form"
          @submit.prevent="handleLogin"
        >
          <el-form-item label="用户名" prop="username"
            ><el-input
              id="login-username"
              v-model="loginData.username"
              placeholder="输入用户名"
              name="username"
              autocomplete="username"
              size="large"
          /></el-form-item>
          <el-form-item label="密码" prop="password"
            ><el-input
              id="login-password"
              v-model="loginData.password"
              placeholder="输入密码"
              name="password"
              autocomplete="current-password"
              type="password"
              show-password
              size="large"
              @keyup="checkCapslock"
            /><span v-if="isCapslock" class="capslock-hint"
              >大写锁定已开启</span
            ></el-form-item
          >
          <div class="access-helper">
            <button type="button" @click="quickFill">使用默认账户</button>
          </div>
          <p v-if="loginError" class="access-error" role="alert">
            {{ loginError }}
          </p>
          <button
            v-magnetic
            class="access-submit"
            type="submit"
            :disabled="loading"
          >
            <span>{{ loading ? "正在登录…" : "登录" }}</span
            ><WorkIcon v-if="!loading" name="arrow" /><span
              v-else
              class="access-progress"
              ><i></i><i></i><i></i
            ></span>
          </button>
        </el-form>
      </div>
    </section>
  </main>
</template>
<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import type { FormInstance, FormRules } from "element-plus";
import WorkIcon from "@/components/workspace/WorkIcon.vue";
import NetworkGlobe from "@/components/workspace/NetworkGlobe.vue";
import { vMagnetic } from "@/utils/magnetic";
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
</script>
<style scoped>
.access-page {
  display: grid;
  grid-template-columns: minmax(0, 1.35fr) minmax(420px, 1fr);
  min-height: 100dvh;
}
.access-story {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: 100dvh;
  padding: 36px 48px;
  background: var(--ui-surface-strong);
  overflow: hidden;
}
.access-brand {
  position: relative;
  z-index: 2;
  width: max-content;
}
.access-story-copy {
  position: relative;
  z-index: 2;
  margin-top: clamp(40px, 10vh, 110px);
  pointer-events: none;
}
.access-context {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  color: var(--ui-text-secondary);
}
.access-context i {
  height: 5px;
  width: 5px;
  border-radius: 50%;
  background: var(--ui-accent);
}
.access-story h1 {
  font-size: clamp(44px, 4.3vw, 72px);
  line-height: 1.16;
  letter-spacing: -3px;
  font-weight: 550;
  margin: 22px 0;
}
.access-story h1 span {
  display: block;
  animation: access-line-in 1s var(--ease-workspace) both;
}
.access-story h1 span:last-child {
  animation-delay: 0.1s;
}
.access-story-copy p {
  font-size: 16px;
  line-height: 1.9;
  color: var(--ui-text-secondary);
  margin: 0;
  animation: access-copy-in 1s 0.2s var(--ease-workspace) both;
}
.access-globe {
  position: relative;
  align-self: flex-end;
  width: 85%;
  height: clamp(260px, 38vh, 420px);
  margin-top: -70px;
  margin-right: -60px;
  flex: 1;
  min-height: 300px;
}
.access-globe:deep(.globe-caption),
.access-globe:deep(.globe-hint),
.access-globe:deep(.globe-play) {
  display: none;
}
.access-capabilities {
  display: flex;
  align-items: center;
  gap: 30px;
  position: relative;
  z-index: 2;
  margin-top: 20px;
}
.access-capabilities span {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  color: var(--ui-text-secondary);
}
.access-capabilities svg {
  height: 16px;
  width: 16px;
}
.access-story-footer {
  display: flex;
  justify-content: space-between;
  position: relative;
  z-index: 2;
  color: var(--ui-text-muted);
  font-size: 14px;
  padding-top: 28px;
  margin-top: 25px;
  border-top: 1px solid var(--ui-border-strong);
}
.access-form-side {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: 32px 40px;
  background: var(--ui-surface);
  min-width: 0;
}
.access-topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  color: var(--ui-text-muted);
  font-size: 14px;
}
.access-appearance {
  height: 30px;
  width: 30px;
  border: 1px solid var(--ui-border);
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: none;
}
.access-appearance:hover {
  background: var(--ui-hover);
}
.access-appearance svg {
  width: 14px;
  height: 14px;
}
.access-form-wrap {
  width: 100%;
  max-width: 320px;
  margin: 50px auto 65px;
  animation: access-form-in 0.8s 0.15s var(--ease-workspace) both;
}
.access-entry-mark {
  display: grid;
  place-items: center;
  height: 40px;
  width: 40px;
  border: 1px solid var(--ui-border-strong);
  border-radius: 11px;
  transform: rotate(-7deg);
  margin-bottom: 25px;
}
.access-entry-mark svg {
  width: 20px;
  height: 20px;
}
.access-form-wrap h2 {
  font-size: 32px;
  letter-spacing: -1.5px;
  font-weight: 600;
  margin: 0 0 12px;
}
.access-form-intro {
  font-size: 16px;
  color: var(--ui-text-secondary);
  margin: 0 0 35px;
}
.access-form:deep(.el-form-item) {
  margin-bottom: 22px;
}
.access-form:deep(.el-form-item__label) {
  font-size: 15px;
  padding-bottom: 7px;
  line-height: 20px;
  height: auto;
}
.access-form:deep(.el-form-item__label:before) {
  display: none;
}
.access-form:deep(.el-input__wrapper) {
  min-height: 45px;
  background: var(--ui-canvas);
  padding: 0 14px;
  box-shadow: 0 0 0 1px var(--ui-border) inset;
}
.access-form:deep(.el-input__inner) {
  font-size: 16px;
}
.access-helper {
  display: flex;
  gap: 6px;
  align-items: center;
  font-size: 14px;
  margin-top: 3px;
}
.access-helper > span {
  color: var(--ui-text-muted);
}
.access-helper button {
  color: var(--ui-accent);
  background: none;
  font-size: 14px;
  padding: 0;
  text-decoration: underline;
  text-underline-offset: 3px;
}
.access-submit {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  height: 45px;
  padding: 0 17px;
  margin-top: 25px;
  border-radius: 6px;
  background: var(--ui-accent);
  color: var(--ui-surface);
  font-size: 16px;
  font-weight: 550;
  transition:
    background 0.2s,
    transform 0.35s var(--ease-spring);
}
.access-submit:hover {
  background: var(--ui-accent-strong);
  transform: translateY(-2px);
}
.access-submit:disabled {
  opacity: 0.7;
  transform: none;
}
.access-submit svg {
  width: 18px;
  height: 18px;
  transition: transform 0.3s var(--ease-spring);
}
.access-submit:hover svg {
  transform: translateX(3px);
}
.access-progress {
  display: flex;
  gap: 3px;
  align-items: center;
}
.access-progress i {
  height: 4px;
  width: 4px;
  background: currentColor;
  border-radius: 50%;
  animation: access-dot 0.8s infinite alternate;
}
.access-progress i:nth-child(2) {
  animation-delay: 0.2s;
}
.access-progress i:nth-child(3) {
  animation-delay: 0.4s;
}
.access-local-note {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 30px;
  color: var(--ui-text-muted);
  font-size: 14px;
}
.access-note-line {
  height: 1px;
  width: 25px;
  background: var(--ui-border-strong);
}
.access-form-footer {
  display: flex;
  justify-content: space-between;
  font-size: 14px;
  color: var(--ui-text-muted);
}
.capslock-hint {
  font-size: 14px;
  color: var(--ui-warning);
  margin-top: 4px;
}
.access-error {
  font-size: 15px;
  color: var(--ui-danger);
  background: var(--ui-danger-soft);
  padding: 10px 12px;
  line-height: 1.7;
  margin: 15px 0 0;
  border-radius: 5px;
}
@keyframes access-line-in {
  from {
    opacity: 0;
    transform: translateY(35px);
    clip-path: inset(0 0 100% 0);
  }
  to {
    opacity: 1;
    transform: none;
    clip-path: inset(0);
  }
}
@keyframes access-copy-in {
  from {
    opacity: 0;
    transform: translateY(14px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes access-form-in {
  from {
    opacity: 0;
    transform: translateX(20px);
  }
  to {
    opacity: 1;
    transform: none;
  }
}
@keyframes access-dot {
  to {
    opacity: 0.3;
    transform: translateY(-3px);
  }
}
@media (min-width: 1600px) {
  .access-story {
    padding: 45px 65px;
  }
  .access-form-side {
    padding: 40px 55px;
  }
  .access-form-wrap {
    max-width: 350px;
  }
  .access-globe {
    margin-top: -120px;
    width: 90%;
    height: 440px;
    max-height: 480px;
  }
}
@media (max-width: 1100px) {
  .access-page {
    grid-template-columns: minmax(0, 1.1fr) minmax(360px, 1fr);
  }
  .access-story {
    padding: 30px;
  }
  .access-story h1 {
    font-size: 46px;
    letter-spacing: -2px;
  }
  .access-form-side {
    padding: 28px;
  }
  .access-globe {
    width: 110%;
    margin-top: -30px;
    margin-right: -70px;
    min-height: 270px;
  }
  .access-capabilities {
    gap: 18px;
  }
  .access-capabilities span {
    font-size: 14px;
  }
}
@media (max-width: 760px) {
  .access-page {
    grid-template-columns: 1fr;
  }
  .access-story {
    min-height: 0;
    padding: 24px 26px 28px;
  }
  .access-brand {
    font-size: 22px;
  }
  .access-story-copy {
    margin-top: 33px;
  }
  .access-context {
    font-size: 14px;
  }
  .access-story h1 {
    font-size: 37px;
    letter-spacing: -1.8px;
    margin: 16px 0;
  }
  .access-story h1 span {
    display: inline;
  }
  .access-story h1 span:first-child {
    display: block;
  }
  .access-story-copy p {
    font-size: 14px;
    line-height: 1.8;
    max-width: 165px;
  }
  .access-globe {
    position: absolute;
    right: -60px;
    bottom: -10px;
    width: 260px;
    height: 260px;
    min-height: 0;
    margin: 0;
    opacity: 0.7;
  }
  .access-capabilities,
  .access-story-footer {
    display: none;
  }
  .access-form-side {
    padding: 23px 26px;
    min-height: 500px;
  }
  .access-topline {
    font-size: 14px;
  }
  .access-form-wrap {
    max-width: 380px;
    margin: 15px auto 50px;
  }
  .access-entry-mark {
    display: none;
  }
  .access-form-wrap h2 {
    font-size: 26px;
    margin-top: 0;
  }
  .access-form-intro {
    font-size: 15px;
    margin-bottom: 26px;
  }
  .access-form:deep(.el-form-item) {
    margin-bottom: 20px;
  }
  .access-form-footer {
    font-size: 14px;
  }
  .access-local-note {
    margin-top: 22px;
  }
}
</style>
