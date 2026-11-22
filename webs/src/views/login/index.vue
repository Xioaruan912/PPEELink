<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import gsap from "gsap";
import { useSettingsStore, useUserStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import ArrowRight from "~icons/ph/arrow-right";
import IconEye from "~icons/ph/eye";
import IconEyeSlash from "~icons/ph/eye-slash";

const userStore = useUserStore();
const settingsStore = useSettingsStore();
const router = useRouter();
const route = useRoute();

const rootEl = ref<HTMLElement>();
const cardEl = ref<HTMLElement>();
const loading = ref(false);
const loginError = ref("");
const isCapslock = ref(false);
const showPassword = ref(false);
const shakeForm = ref(false);
const focusedField = ref<"" | "username" | "password">("");

const loginData = ref({ username: "", password: "" });

const isDark = computed(() => settingsStore.theme === ThemeEnum.DARK);
function toggleTheme() {
  settingsStore.changeTheme(isDark.value ? ThemeEnum.LIGHT : ThemeEnum.DARK);
}

function checkCapslock(event: KeyboardEvent) {
  isCapslock.value = event.getModifierState("CapsLock");
}

function triggerShake() {
  shakeForm.value = false;
  requestAnimationFrame(() => {
    shakeForm.value = true;
    setTimeout(() => {
      shakeForm.value = false;
    }, 420);
  });
}

async function handleLogin() {
  if (loading.value) return;
  loginError.value = "";
  const u = loginData.value.username.trim();
  const p = loginData.value.password;
  if (!u) {
    loginError.value = "请输入用户名";
    triggerShake();
    return;
  }
  if (!p || p.length < 6) {
    loginError.value = "密码至少需要 6 个字符";
    triggerShake();
    return;
  }

  loading.value = true;
  try {
    const { mustChangePassword } = await userStore.login({
      username: u,
      password: p,
    });
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
    loginError.value = "登录未成功，请检查账户、密码及服务连接。";
    triggerShake();
  } finally {
    loading.value = false;
  }
}

// Cursor-reactive ambient light & subtle Apple glass card tilt
const pointer = ref({ x: 50, y: 40 });
const cardTilt = ref({ rx: 0, ry: 0, gx: 50, gy: 50 });
const reduceMotion =
  typeof window !== "undefined" &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches;

function onStageMove(e: PointerEvent) {
  if (reduceMotion || e.pointerType !== "mouse") return;
  pointer.value = {
    x: (e.clientX / window.innerWidth) * 100,
    y: (e.clientY / window.innerHeight) * 100,
  };
}

function onCardMove(e: PointerEvent) {
  if (reduceMotion || e.pointerType !== "mouse" || !cardEl.value) return;
  const rect = cardEl.value.getBoundingClientRect();
  const px = (e.clientX - rect.left) / rect.width;
  const py = (e.clientY - rect.top) / rect.height;
  cardTilt.value = {
    rx: (0.5 - py) * 5,
    ry: (px - 0.5) * 6,
    gx: px * 100,
    gy: py * 100,
  };
}

function onCardLeave() {
  cardTilt.value = { rx: 0, ry: 0, gx: 50, gy: 50 };
}

let gsapCtx: gsap.Context | null = null;
onMounted(() => {
  if (!rootEl.value) return;
  gsapCtx = gsap.context(() => {
    const mm = gsap.matchMedia();
    mm.add("(prefers-reduced-motion: no-preference)", () => {
      const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
      tl.from("[data-logo-reveal]", {
        y: -12,
        opacity: 0,
        duration: 0.6,
        clearProps: "transform,opacity",
      })
        .from(
          "[data-headline-line]",
          {
            yPercent: 112,
            duration: 0.82,
            stagger: 0.09,
            clearProps: "transform",
          },
          "-=0.4"
        )
        .from(
          "[data-hero-fade]",
          {
            y: 16,
            opacity: 0,
            duration: 0.62,
            stagger: 0.08,
            clearProps: "transform,opacity",
          },
          "-=0.52"
        )
        .from(
          "[data-vessel-reveal]",
          {
            y: 28,
            scale: 0.96,
            opacity: 0,
            duration: 0.78,
            clearProps: "transform,opacity",
          },
          "-=0.62"
        )
        .from(
          "[data-orbit-ring]",
          {
            scale: 0.82,
            opacity: 0,
            duration: 1.0,
            stagger: 0.1,
            clearProps: "transform,opacity",
          },
          "-=0.7"
        );
    });
  }, rootEl.value);
});

onBeforeUnmount(() => {
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <main
    ref="rootEl"
    class="apple-login"
    :style="{
      '--mx': `${pointer.x}%`,
      '--my': `${pointer.y}%`,
    }"
    @pointermove="onStageMove"
  >
    <!-- Ambient Studio Spotlight -->
    <div class="ambient-glow" aria-hidden="true"></div>

    <!-- Minimalist Floating Top Bar with Kinetic PPEE ○ LINK Logo -->
    <header class="login-header" data-logo-reveal>
      <a href="#/login" class="brand-mark" aria-label="PPEELink">
        <span class="sr-only">PPEELink</span>
        <svg
          class="brand-svg"
          viewBox="0 0 320 44"
          fill="none"
          aria-hidden="true"
        >
          <text x="0" y="33" fill="currentColor" class="brand-word">PPEE</text>
          <rect
            class="brand-capsule"
            x="104"
            y="8"
            width="42"
            height="26"
            rx="13"
            stroke="currentColor"
            stroke-width="4"
          />
          <circle
            class="brand-pulse-ring"
            cx="125"
            cy="21"
            r="8"
            stroke="var(--c-ok)"
            stroke-width="1.5"
          />
          <circle
            class="brand-core"
            cx="125"
            cy="21"
            r="4.2"
            fill="var(--c-ok)"
          />
          <text
            x="156"
            y="33"
            fill="currentColor"
            class="brand-word brand-trail"
          >
            LINK
          </text>
        </svg>
      </a>

      <div class="theme-switch-pair" role="group" aria-label="主题模式">
        <button
          type="button"
          class="theme-word"
          :class="{ 'is-on': !isDark }"
          @click="isDark && toggleTheme()"
        >
          Light
        </button>
        <span class="theme-slash" aria-hidden="true">/</span>
        <button
          type="button"
          class="theme-word"
          :class="{ 'is-on': isDark }"
          @click="!isDark && toggleTheme()"
        >
          Dark
        </button>
      </div>
    </header>

    <!-- Apple Keynote Asymmetric Stage -->
    <div class="login-stage">
      <!-- Left Editorial + Kinetic Orbital Hardware Emblem -->
      <section class="hero-column" aria-labelledby="brand-title">
        <div class="orbital-stage" aria-hidden="true">
          <svg class="orbital-svg" viewBox="0 0 360 120" fill="none">
            <rect
              data-orbit-ring
              x="4"
              y="16"
              width="144"
              height="88"
              rx="44"
              stroke="var(--c-line)"
              stroke-width="1"
            />
            <rect
              data-orbit-ring
              x="22"
              y="30"
              width="108"
              height="60"
              rx="30"
              stroke="var(--c-ink)"
              stroke-width="1.5"
              stroke-opacity="0.24"
            />
            <circle
              class="sat sat-a"
              cx="76"
              cy="60"
              r="4.5"
              fill="var(--c-ok)"
            />
            <circle
              class="sat-halo"
              cx="76"
              cy="60"
              r="12"
              stroke="var(--c-ok)"
              stroke-width="1"
            />
            <line
              x1="148"
              y1="60"
              x2="340"
              y2="60"
              stroke="var(--c-line)"
              stroke-dasharray="3 5"
            />
            <circle cx="212" cy="60" r="2.5" fill="var(--c-ink)" opacity="0.35" />
            <circle cx="284" cy="60" r="2.5" fill="var(--c-ok)" />
          </svg>
        </div>

        <h1 id="brand-title" class="hero-headline">
          <span class="line-mask">
            <span class="line-inner" data-headline-line>Network</span>
          </span>
          <span class="line-mask">
            <span class="line-inner" data-headline-line>Architecture.</span>
          </span>
        </h1>

        <p class="hero-statement" data-hero-fade>
          多客户端订阅分发、全球边缘节点资产与实时链路遥测的静谧控制空间。
        </p>

        <dl class="hero-specs" data-hero-fade aria-label="核心能力">
          <div>
            <dt>订阅分发</dt>
            <dd>多端自适应配置与一键扫码</dd>
          </div>
          <div>
            <dt>节点资产</dt>
            <dd>自动探针、分组与下发策略</dd>
          </div>
          <div>
            <dt>服务器运维</dt>
            <dd>WebShell、文件与容器一体化</dd>
          </div>
        </dl>
      </section>

      <!-- Right Kinetic Frosted Glass Login Vessel -->
      <section class="vessel-column" aria-labelledby="login-title">
        <div
          ref="cardEl"
          class="login-vessel"
          :class="{ 'is-shaking': shakeForm }"
          data-vessel-reveal
          :style="{
            transform: `perspective(1100px) rotateX(${cardTilt.rx}deg) rotateY(${cardTilt.ry}deg)`,
            '--gx': `${cardTilt.gx}%`,
            '--gy': `${cardTilt.gy}%`,
          }"
          @pointermove="onCardMove"
          @pointerleave="onCardLeave"
        >
          <div class="vessel-sheen" aria-hidden="true"></div>

          <header class="vessel-head">
            <div class="vessel-badge">
              <i aria-hidden="true"></i>
              <span>PPEELink Console</span>
            </div>
            <h2 id="login-title">欢迎回来</h2>
            <p>输入您的账户凭据以进入工作台。</p>
          </header>

          <form class="kinetic-form" novalidate @submit.prevent="handleLogin">
            <!-- Floating Label Kinetic Input: Username -->
            <div
              class="k-field"
              :class="{
                'is-focused': focusedField === 'username',
                'is-filled': loginData.username.length > 0,
              }"
            >
              <label for="login-username" class="k-label">用户名</label>
              <input
                id="login-username"
                v-model="loginData.username"
                name="username"
                type="text"
                autocomplete="username"
                spellcheck="false"
                @focus="focusedField = 'username'"
                @blur="focusedField = ''"
              />
              <span class="k-bar" aria-hidden="true"></span>
            </div>

            <!-- Floating Label Kinetic Input: Password -->
            <div
              class="k-field"
              :class="{
                'is-focused': focusedField === 'password',
                'is-filled': loginData.password.length > 0,
              }"
            >
              <label for="login-password" class="k-label">密码</label>
              <input
                id="login-password"
                v-model="loginData.password"
                name="password"
                :type="showPassword ? 'text' : 'password'"
                autocomplete="current-password"
                @focus="focusedField = 'password'"
                @blur="focusedField = ''"
                @keyup="checkCapslock"
              />
              <button
                type="button"
                class="pwd-toggle"
                :aria-label="showPassword ? '隐藏密码' : '显示密码'"
                @click="showPassword = !showPassword"
              >
                <IconEyeSlash v-if="showPassword" aria-hidden="true" />
                <IconEye v-else aria-hidden="true" />
              </button>
              <span class="k-bar" aria-hidden="true"></span>
            </div>

            <p v-if="isCapslock" class="capslock-hint" role="status">
              大写锁定已开启
            </p>
            <p v-if="loginError" class="login-error" role="alert">
              {{ loginError }}
            </p>

            <button
              type="submit"
              class="kinetic-submit"
              :class="{ 'is-loading': loading }"
              :disabled="loading"
            >
              <span class="submit-curtain" aria-hidden="true"></span>
              <span class="submit-content">
                <span v-if="loading" class="submit-spinner" aria-hidden="true"></span>
                <span>{{ loading ? "正在验证凭据…" : "进入工作区" }}</span>
                <ArrowRight v-if="!loading" class="submit-arrow" aria-hidden="true" />
              </span>
            </button>
          </form>
        </div>
      </section>
    </div>
  </main>
</template>

<style scoped>
.apple-login {
  position: relative;
  min-height: 100dvh;
  display: grid;
  grid-template-rows: auto 1fr;
  background: var(--c-canvas);
  overflow: hidden;
}

.ambient-glow {
  position: fixed;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(
    640px circle at var(--mx, 50%) var(--my, 40%),
    color-mix(in srgb, var(--c-ok) 7%, transparent),
    transparent 65%
  );
  transition: background 180ms linear;
}

.login-header {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  max-width: 1360px;
  width: 100%;
  margin: 0 auto;
  padding: 28px clamp(20px, 4vw, 56px);
}

/* Kinetic PPEE ○ LINK Brand Mark */
.brand-mark {
  display: inline-flex;
  align-items: center;
  color: var(--c-ink);
  text-decoration: none;
}

.brand-svg {
  height: 28px;
  width: auto;
  overflow: visible;
}

.brand-word {
  font-family: var(--font-display);
  font-size: 31px;
  font-weight: 700;
  letter-spacing: -0.04em;
}

.brand-capsule {
  transform-box: fill-box;
  transform-origin: center;
  transition: transform 380ms var(--ease-out);
}

.brand-core {
  transform-box: fill-box;
  transform-origin: center;
  transition: transform 380ms var(--ease-out);
}

.brand-pulse-ring {
  transform-box: fill-box;
  transform-origin: center;
  opacity: 0;
  animation: brand-pulse 3.2s var(--ease-out) infinite;
}

.brand-mark:hover .brand-capsule {
  transform: scaleX(1.12);
}

.brand-mark:hover .brand-core {
  transform: scale(1.25);
}

@keyframes brand-pulse {
  0% {
    transform: scale(0.6);
    opacity: 0.7;
  }
  70% {
    transform: scale(1.6);
    opacity: 0;
  }
  100% {
    transform: scale(1.6);
    opacity: 0;
  }
}

.theme-switch-pair {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 84%, transparent);
  backdrop-filter: blur(12px);
}

.theme-word {
  border: 0;
  background: transparent;
  padding: 0;
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.theme-word.is-on {
  color: var(--c-ink);
  font-weight: 600;
}

.theme-slash {
  color: var(--c-line);
  font-size: 12px;
}

/* Main Stage */
.login-stage {
  position: relative;
  z-index: 2;
  max-width: 1360px;
  width: 100%;
  margin: 0 auto;
  padding: 20px clamp(20px, 4vw, 56px) 56px;
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(360px, 440px);
  gap: clamp(32px, 6vw, 96px);
  align-items: center;
}

.hero-column {
  display: grid;
  gap: 24px;
  min-width: 0;
}

.orbital-stage {
  width: min(360px, 100%);
}

.orbital-svg {
  width: 100%;
  height: auto;
  overflow: visible;
}

.sat-halo {
  transform-box: fill-box;
  transform-origin: center;
  animation: brand-pulse 2.8s var(--ease-out) infinite;
}

.hero-headline {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(46px, 6.2vw, 84px);
  font-weight: 600;
  line-height: 0.98;
  letter-spacing: -0.055em;
  color: var(--c-ink);
}

.line-mask {
  display: block;
  overflow: hidden;
  padding-bottom: 6px;
}

.line-inner {
  display: block;
}

.hero-statement {
  margin: 0;
  max-width: 44ch;
  font-size: 16px;
  line-height: 1.65;
  color: var(--c-muted);
}

.hero-specs {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 20px;
  margin: 12px 0 0;
  padding-top: 22px;
  border-top: 1px solid var(--c-line);
}

.hero-specs dt {
  font-size: 12px;
  font-weight: 600;
  color: var(--c-ink);
  margin-bottom: 4px;
}

.hero-specs dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-muted);
  line-height: 1.5;
}

/* Frosted Glass Login Vessel */
.vessel-column {
  min-width: 0;
}

.login-vessel {
  position: relative;
  overflow: hidden;
  display: grid;
  gap: 26px;
  padding: 36px 34px;
  border-radius: 26px;
  border: 1px solid var(--c-line);
  background: color-mix(in srgb, var(--c-surface) 88%, transparent);
  backdrop-filter: blur(26px) saturate(160%);
  box-shadow:
    0 32px 72px -24px rgb(0 0 0 / 0.14),
    inset 0 1px 0 rgb(255 255 255 / 0.65);
  transition: transform 240ms var(--ease-out);
  will-change: transform;
}

.login-vessel.is-shaking {
  animation: vessel-shake 380ms cubic-bezier(0.36, 0.07, 0.19, 0.97) both;
}

@keyframes vessel-shake {
  0%, 100% { transform: translateX(0); }
  20% { transform: translateX(-8px); }
  40% { transform: translateX(7px); }
  60% { transform: translateX(-5px); }
  80% { transform: translateX(3px); }
}

.vessel-sheen {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: radial-gradient(
    260px circle at var(--gx, 50%) var(--gy, 50%),
    rgb(255 255 255 / 0.16),
    transparent 70%
  );
}

.vessel-head {
  position: relative;
  display: grid;
  gap: 8px;
}

.vessel-badge {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  width: fit-content;
  padding: 4px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-canvas);
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--c-muted);
}

.vessel-badge i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.vessel-head h2 {
  margin: 4px 0 0;
  font-family: var(--font-display);
  font-size: 26px;
  font-weight: 500;
  letter-spacing: -0.03em;
  color: var(--c-ink);
}

.vessel-head p {
  margin: 0;
  font-size: 13.5px;
  color: var(--c-muted);
}

/* Custom Kinetic Apple Floating-Label Inputs (Zero Double-Box Bug!) */
.kinetic-form {
  position: relative;
  display: grid;
  gap: 16px;
}

.k-field {
  position: relative;
  display: flex;
  align-items: center;
  height: 58px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-canvas);
  transition:
    border-color 220ms var(--ease-out),
    background 220ms var(--ease-out),
    box-shadow 220ms var(--ease-out);
}

.k-field.is-focused {
  border-color: var(--c-ink);
  background: var(--c-surface);
  box-shadow: 0 0 0 4px color-mix(in srgb, var(--c-ink) 7%, transparent);
}

.k-label {
  position: absolute;
  left: 16px;
  top: 50%;
  transform: translateY(-50%);
  transform-origin: left center;
  font-size: 14px;
  color: var(--c-muted);
  pointer-events: none;
  transition:
    transform 240ms var(--ease-out),
    color 240ms var(--ease-out);
}

.k-field.is-focused .k-label,
.k-field.is-filled .k-label {
  transform: translateY(-135%) scale(0.78);
  color: var(--c-muted);
}

.k-field.is-focused .k-label {
  color: var(--c-ink);
  font-weight: 500;
}

.k-field input {
  width: 100%;
  height: 100%;
  padding: 20px 44px 6px 16px;
  border: 0;
  outline: none !important;
  background: transparent;
  color: var(--c-ink);
  font-size: 15px;
  font-weight: 450;
}

.k-bar {
  position: absolute;
  left: 16px;
  right: 16px;
  bottom: -1px;
  height: 2px;
  border-radius: 2px;
  background: var(--c-ok);
  transform: scaleX(0);
  transform-origin: center;
  transition: transform 320ms var(--ease-out);
}

.k-field.is-focused .k-bar {
  transform: scaleX(1);
}

.pwd-toggle {
  position: absolute;
  right: 10px;
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
  transition: color 180ms ease, background 180ms ease;
}

.pwd-toggle:hover {
  color: var(--c-ink);
  background: var(--c-surface-2);
}

.pwd-toggle svg {
  width: 18px;
  height: 18px;
}

.capslock-hint {
  margin: -4px 0 0;
  font-size: 12px;
  color: var(--c-warn);
}

.login-error {
  margin: -2px 0 0;
  padding: 10px 14px;
  border-radius: 10px;
  background: rgb(223 41 41 / 0.08);
  color: var(--c-bad);
  font-size: 13px;
}

/* Magnetic Curtain-Fill Submit Button */
.kinetic-submit {
  position: relative;
  overflow: hidden;
  margin-top: 6px;
  height: 52px;
  width: 100%;
  border: 0;
  border-radius: 14px;
  background: var(--c-ink);
  color: var(--c-surface);
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  transition: transform 130ms var(--ease-out), box-shadow 260ms var(--ease-out);
}

.submit-curtain {
  position: absolute;
  inset: 0;
  background: var(--c-ok);
  transform: translateY(102%);
  transition: transform 360ms var(--ease-out);
}

.submit-content {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.submit-arrow {
  width: 17px;
  height: 17px;
  transition: transform 280ms var(--ease-out);
}

.kinetic-submit:hover:not(:disabled) {
  box-shadow: 0 14px 32px -10px rgb(21 184 90 / 0.42);
}

.kinetic-submit:hover:not(:disabled) .submit-curtain {
  transform: translateY(0);
}

.kinetic-submit:hover:not(:disabled) {
  color: #ffffff;
}

.kinetic-submit:hover:not(:disabled) .submit-arrow {
  transform: translateX(4px);
}

.kinetic-submit:active:not(:disabled) {
  transform: scale(0.975);
}

.kinetic-submit:disabled {
  opacity: 0.7;
  cursor: wait;
}

.submit-spinner {
  width: 15px;
  height: 15px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 680ms linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

@media (max-width: 960px) {
  .login-stage {
    grid-template-columns: 1fr;
    gap: 32px;
    padding-top: 8px;
  }
  .orbital-stage {
    display: none;
  }
  .hero-specs {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .login-vessel {
    padding: 28px 22px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .brand-pulse-ring,
  .sat-halo,
  .login-vessel.is-shaking {
    animation: none;
  }
  .login-vessel {
    transform: none !important;
  }
}
</style>
