<template>
  <div class="login-stage">
    <!-- animated node network backdrop -->
    <canvas ref="canvasRef" class="stage-canvas" aria-hidden="true"></canvas>
    <div class="stage-aurora" aria-hidden="true"></div>
    <div class="stage-grid" aria-hidden="true"></div>

    <!-- top controls -->
    <div class="stage-topbar">
      <div class="stage-live" ref="liveRef">
        <span class="live-dot"></span>
        <span>CONTROL PLANE ONLINE</span>
      </div>
      <div class="stage-tools">
        <el-switch
          v-model="isDark"
          inline-prompt
          :active-icon="Moon"
          :inactive-icon="Sunny"
          @change="toggleTheme"
        />
        <lang-select class="ml-3 cursor-pointer" />
      </div>
    </div>

    <!-- brand stage -->
    <section class="brand-stage" ref="brandRef">
      <p class="brand-eyebrow" data-hero>
        <span>PPEELINK</span><i>/</i><span>NODE &amp; SUBSCRIPTION</span>
      </p>
      <h1 class="brand-title">
        <span class="title-line" data-hero>网络资产，</span>
        <span class="title-line" data-hero>一处<span class="title-em">掌控</span>。</span>
      </h1>
      <p class="brand-copy" data-hero>
        订阅编排、节点测活、规则中心与客户端分发，收敛在一块安静的控制面板里。
      </p>
      <ul class="brand-features" data-hero>
        <li><b>01</b><span>多客户端订阅管线</span></li>
        <li><b>02</b><span>节点质量与解锁观测</span></li>
        <li><b>03</b><span>规则快照与回滚</span></li>
      </ul>
      <div class="brand-meta" data-hero>
        <span class="meta-chip"><i></i>服务运行中</span>
        <span class="meta-sep"></span>
        <span class="meta-version">v{{ version || "—" }}</span>
      </div>
    </section>

    <!-- auth panel -->
    <section class="auth-stage" ref="cardRef">
      <div class="auth-card">
        <header class="auth-head">
          <span class="auth-logo">
            <img :src="logo" alt="PPEELink" />
            <span class="auth-logo-ring" aria-hidden="true"></span>
          </span>
          <div class="auth-heading">
            <h2>{{ defaultSettings.title }}</h2>
            <p>节点与订阅控制台</p>
          </div>
          <el-tag class="auth-version" effect="plain">v{{ version || "—" }}</el-tag>
        </header>

        <p class="auth-intro">
          <span class="intro-dot"></span>
          欢迎回来，请登录以继续管理服务
        </p>

        <el-form
          ref="loginFormRef"
          :model="loginData"
          :rules="loginRules"
          class="auth-form"
        >
          <el-form-item prop="username" data-field>
            <div class="field-shell">
              <svg-icon icon-class="user" class="field-icon" />
              <el-input
                ref="username"
                v-model="loginData.username"
                :placeholder="$t('login.username')"
                name="username"
                size="large"
                class="field-input"
              />
            </div>
          </el-form-item>

          <el-tooltip
            :visible="isCapslock"
            :content="$t('login.capsLock')"
            placement="right"
          >
            <el-form-item prop="password" data-field>
              <div class="field-shell">
                <svg-icon icon-class="lock" class="field-icon" />
                <el-input
                  v-model="loginData.password"
                  :placeholder="$t('login.password')"
                  type="password"
                  name="password"
                  size="large"
                  class="field-input"
                  show-password
                  @keyup="checkCapslock"
                  @keyup.enter="handleLogin"
                />
              </div>
            </el-form-item>
          </el-tooltip>

          <el-button
            v-magnetic="0.16"
            :loading="loading"
            type="primary"
            size="large"
            class="auth-submit"
            data-field
            @click.prevent="handleLogin"
          >
            <span class="submit-label">{{ $t("login.login") }}</span>
            <span class="submit-arrow" aria-hidden="true">→</span>
          </el-button>
        </el-form>

        <footer class="auth-foot">
          <span class="foot-dot"></span>
          <span>端到端加密会话 · 登录行为将写入审计日志</span>
        </footer>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { useSettingsStore, useUserStore } from "@/store";
import { GetVersion } from "@/api/auth";
import { LoginData } from "@/api/auth/types";
import { Sunny, Moon } from "@element-plus/icons-vue";
import { LocationQuery, LocationQueryValue, useRoute } from "vue-router";
import router from "@/router";
import defaultSettings from "@/settings";
import { ThemeEnum } from "@/enums/ThemeEnum";
import { gsap } from "gsap";

const logo = new URL("../../assets/logo.png", import.meta.url).href;

const reduceMotion =
  typeof window !== "undefined" &&
  window.matchMedia("(prefers-reduced-motion: reduce)").matches;

// ---------- version ----------
const version = ref("");
GetVersion()
  .then((res) => {
    version.value = res.data;
  })
  .catch(() => undefined);

// ---------- stores ----------
const userStore = useUserStore();
const settingsStore = useSettingsStore();
const { t } = useI18n();

// ---------- state ----------
const isDark = ref(settingsStore.theme === ThemeEnum.DARK);
const loading = ref(false);
const isCapslock = ref(false);
const loginFormRef = ref(ElForm);
const loginData = ref<LoginData>({ username: "", password: "" });

const loginRules = computed(() => ({
  username: [
    {
      required: true,
      trigger: "blur",
      message: t("login.message.username.required"),
    },
  ],
  password: [
    {
      required: true,
      trigger: "blur",
      message: t("login.message.password.required"),
    },
    { min: 6, message: t("login.message.password.min"), trigger: "blur" },
  ],
}));

// ---------- refs ----------
const canvasRef = ref<HTMLCanvasElement>();
const cardRef = ref<HTMLElement>();
const brandRef = ref<HTMLElement>();
const liveRef = ref<HTMLElement>();

// ---------- login ----------
const route = useRoute();
function shakeCard() {
  if (reduceMotion || !cardRef.value) return;
  gsap.fromTo(
    cardRef.value,
    { x: -9 },
    { x: 0, duration: 0.6, ease: "elastic.out(1, 0.32)" }
  );
}

function handleLogin() {
  loginFormRef.value.validate((valid: boolean) => {
    if (!valid) return;
    loading.value = true;
    userStore
      .login(loginData.value)
      .then(() => {
        const query: LocationQuery = route.query;
        const redirect = (query.redirect as LocationQueryValue) ?? "/";
        const otherQueryParams = Object.keys(query).reduce(
          (acc: any, cur: string) => {
            if (cur !== "redirect") acc[cur] = query[cur];
            return acc;
          },
          {}
        );
        router.push({ path: redirect, query: otherQueryParams });
      })
      .catch(() => {
        shakeCard();
      })
      .finally(() => {
        loading.value = false;
      });
  });
}

// ---------- theme ----------
const toggleTheme = () => {
  const next =
    settingsStore.theme === ThemeEnum.DARK ? ThemeEnum.LIGHT : ThemeEnum.DARK;
  settingsStore.changeTheme(next);
};

function checkCapslock(event: KeyboardEvent) {
  if (event instanceof KeyboardEvent) {
    isCapslock.value = event.getModifierState("CapsLock");
  }
}

// ---------- node network canvas ----------
let cleanupCanvas: (() => void) | undefined;
function initNetwork(canvas: HTMLCanvasElement) {
  const ctx = canvas.getContext("2d");
  if (!ctx) return;
  const dpr = Math.min(window.devicePixelRatio || 1, 2);
  let width = 0;
  let height = 0;
  let raf = 0;
  let alive = true;
  const pointer = { x: -9999, y: -9999 };

  type Node = { x: number; y: number; vx: number; vy: number; r: number };
  let nodes: Node[] = [];

  const accent = () =>
    getComputedStyle(document.documentElement)
      .getPropertyValue("--ui-accent")
      .trim() || "#3ddba2";

  const resize = () => {
    width = canvas.clientWidth;
    height = canvas.clientHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    const count = Math.min(Math.floor((width * height) / 26000), 72);
    nodes = Array.from({ length: count }, () => ({
      x: Math.random() * width,
      y: Math.random() * height,
      vx: (Math.random() - 0.5) * 0.32,
      vy: (Math.random() - 0.5) * 0.32,
      r: Math.random() * 1.6 + 0.8,
    }));
  };

  const draw = () => {
    if (!alive) return;
    ctx.clearRect(0, 0, width, height);
    const color = accent();
    const linkDist = Math.min(Math.max(width, height) * 0.09, 150);

    for (const n of nodes) {
      n.x += n.vx;
      n.y += n.vy;
      const dxp = n.x - pointer.x;
      const dyp = n.y - pointer.y;
      const dp = Math.hypot(dxp, dyp);
      if (dp < 130) {
        n.x += (dxp / (dp || 1)) * 0.7;
        n.y += (dyp / (dp || 1)) * 0.7;
      }
      if (n.x < -20) n.x = width + 20;
      if (n.x > width + 20) n.x = -20;
      if (n.y < -20) n.y = height + 20;
      if (n.y > height + 20) n.y = -20;
    }

    for (let i = 0; i < nodes.length; i++) {
      const a = nodes[i];
      for (let j = i + 1; j < nodes.length; j++) {
        const b = nodes[j];
        const d = Math.hypot(a.x - b.x, a.y - b.y);
        if (d < linkDist) {
          const alpha = (1 - d / linkDist) * 0.2;
          ctx.strokeStyle = color;
          ctx.globalAlpha = alpha;
          ctx.lineWidth = 0.7;
          ctx.beginPath();
          ctx.moveTo(a.x, a.y);
          ctx.lineTo(b.x, b.y);
          ctx.stroke();
        }
      }
    }

    ctx.globalAlpha = 1;
    for (const n of nodes) {
      ctx.fillStyle = color;
      ctx.globalAlpha = 0.52;
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.r, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 0.13;
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.r * 4.2, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;

    raf = requestAnimationFrame(draw);
  };

  const onPointer = (event: PointerEvent) => {
    const rect = canvas.getBoundingClientRect();
    pointer.x = event.clientX - rect.left;
    pointer.y = event.clientY - rect.top;
  };
  const onLeave = () => {
    pointer.x = -9999;
    pointer.y = -9999;
  };
  const onVisibility = () => {
    if (document.hidden) {
      cancelAnimationFrame(raf);
      raf = 0;
    } else if (!raf && alive) {
      raf = requestAnimationFrame(draw);
    }
  };

  resize();
  window.addEventListener("resize", resize);
  window.addEventListener("pointermove", onPointer, { passive: true });
  window.addEventListener("pointerleave", onLeave);
  document.addEventListener("visibilitychange", onVisibility);

  if (reduceMotion) {
    // static snapshot, no loop
    ctx.clearRect(0, 0, width, height);
    const color = accent();
    ctx.fillStyle = color;
    for (const n of nodes) {
      ctx.globalAlpha = 0.35;
      ctx.beginPath();
      ctx.arc(n.x, n.y, n.r, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;
  } else {
    raf = requestAnimationFrame(draw);
  }

  cleanupCanvas = () => {
    alive = false;
    cancelAnimationFrame(raf);
    window.removeEventListener("resize", resize);
    window.removeEventListener("pointermove", onPointer);
    window.removeEventListener("pointerleave", onLeave);
    document.removeEventListener("visibilitychange", onVisibility);
  };
}

// ---------- entrance choreography ----------
let timeline: gsap.core.Timeline | null = null;

onMounted(() => {
  if (canvasRef.value) initNetwork(canvasRef.value);

  if (reduceMotion) return;
  const liveEl = liveRef.value;
  const cardEl = cardRef.value;
  const heroEls = brandRef.value?.querySelectorAll("[data-hero]") ?? [];
  const fields = cardRef.value?.querySelectorAll("[data-field]") ?? [];

  timeline = gsap.timeline({ defaults: { ease: "power3.out" } });
  timeline.fromTo(
    ".stage-canvas",
    { opacity: 0, scale: 1.04 },
    { opacity: 1, scale: 1, duration: 1.4, ease: "power2.out" }
  );
  if (liveEl) {
    timeline.fromTo(
      liveEl,
      { opacity: 0, y: -10 },
      { opacity: 1, y: 0, duration: 0.5 },
      "-=1.0"
    );
  }
  if (heroEls.length) {
    timeline.fromTo(
      heroEls,
      { opacity: 0, y: 34, filter: "blur(10px)" },
      { opacity: 1, y: 0, filter: "blur(0px)", duration: 0.9, stagger: 0.09 },
      "-=0.7"
    );
  }
  if (cardEl) {
    timeline.fromTo(
      cardEl,
      { opacity: 0, y: 30, scale: 0.97, filter: "blur(12px)" },
      { opacity: 1, y: 0, scale: 1, filter: "blur(0px)", duration: 1.0 },
      "-=0.75"
    );
  }
  if (fields.length) {
    timeline.fromTo(
      fields,
      { opacity: 0, y: 18 },
      { opacity: 1, y: 0, duration: 0.6, stagger: 0.08 },
      "-=0.5"
    );
  }
});

onBeforeUnmount(() => {
  timeline?.kill();
  cleanupCanvas?.();
});
</script>

<style lang="scss" scoped>
/* ------------------------------------------------------------------ */
/* stage                                                              */
/* ------------------------------------------------------------------ */

.login-stage {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.15fr) minmax(420px, 0.85fr);
  align-items: center;
  gap: clamp(32px, 5vw, 80px);
  width: 100%;
  min-height: 100dvh;
  padding: clamp(24px, 5vw, 72px);
  overflow: hidden;
  background:
    radial-gradient(
      ellipse 62% 48% at 74% 14%,
      color-mix(in srgb, var(--ui-accent) 12%, transparent),
      transparent 68%
    ),
    radial-gradient(
      ellipse 46% 40% at 10% 90%,
      color-mix(in srgb, var(--gold-300) 10%, transparent),
      transparent 70%
    ),
    var(--ui-canvas);
}

.stage-canvas {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  opacity: 0.85;
}

.stage-aurora {
  position: absolute;
  inset: -20%;
  pointer-events: none;
  background:
    radial-gradient(
      38% 32% at 24% 28%,
      color-mix(in srgb, var(--ui-accent) 10%, transparent),
      transparent 70%
    ),
    radial-gradient(
      30% 26% at 78% 74%,
      color-mix(in srgb, var(--gold-300) 7%, transparent),
      transparent 72%
    );
  animation: aurora-drift 22s ease-in-out infinite;
  filter: blur(2px);
}

.stage-grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  opacity: 0.5;
  background-image:
    linear-gradient(
      color-mix(in srgb, var(--ui-border) 55%, transparent) 1px,
      transparent 1px
    ),
    linear-gradient(
      90deg,
      color-mix(in srgb, var(--ui-border) 55%, transparent) 1px,
      transparent 1px
    );
  background-size: 44px 44px;
  mask-image: radial-gradient(circle at 38% 42%, #000 0%, transparent 74%);
}

.stage-topbar {
  position: absolute;
  z-index: 3;
  top: 0;
  right: 0;
  left: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 22px 28px;
}

.stage-live {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  font-family: var(--font-mono);
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.22em;
  color: var(--ui-text-muted);
}

.live-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ui-accent);
  animation: jade-pulse 2.6s ease-out infinite;
}

/* ------------------------------------------------------------------ */
/* brand stage                                                        */
/* ------------------------------------------------------------------ */

.brand-stage {
  position: relative;
  z-index: 2;
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  max-width: 620px;
}

.brand-eyebrow {
  display: inline-flex;
  gap: 10px;
  margin: 0 0 22px;
  font-family: var(--font-mono);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.24em;
  color: var(--ui-accent-strong);

  i {
    font-style: normal;
    color: var(--ui-text-muted);
  }
}

.brand-title {
  margin: 0;
  font-size: clamp(38px, 4.6vw, 64px);
  font-weight: 700;
  line-height: 1.08;
  letter-spacing: -0.035em;
  color: var(--ui-text);
}

.title-line {
  display: block;
}

.title-em {
  position: relative;
  color: var(--ui-accent);
  text-shadow: 0 0 34px color-mix(in srgb, var(--ui-accent) 40%, transparent);
}

.title-em::after {
  position: absolute;
  right: 0;
  bottom: 4px;
  left: 0;
  height: 2px;
  border-radius: var(--r-pill);
  background: linear-gradient(
    90deg,
    transparent,
    var(--ui-accent),
    transparent
  );
  opacity: 0.7;
  content: "";
}

.brand-copy {
  max-width: 46ch;
  margin: 24px 0 0;
  font-size: 14.5px;
  line-height: 1.8;
  color: var(--ui-text-secondary);
}

.brand-features {
  display: flex;
  flex-direction: column;
  gap: 10px;
  margin: 30px 0 0;
  padding: 0;
  list-style: none;

  li {
    display: flex;
    align-items: center;
    gap: 14px;
    font-size: 13px;
    color: var(--ui-text-secondary);
  }

  b {
    font-family: var(--font-mono);
    font-size: 11px;
    font-weight: 600;
    color: var(--ui-accent-strong);
    background: var(--ui-accent-soft);
    border: 1px solid color-mix(in srgb, var(--ui-accent) 24%, transparent);
    border-radius: var(--r-xs);
    padding: 3px 8px;
  }
}

.brand-meta {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  margin-top: 40px;
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--ui-text-muted);
}

.meta-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;

  i {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--ui-accent);
  }
}

.meta-sep {
  width: 34px;
  height: 1px;
  background: var(--ui-border);
}

/* ------------------------------------------------------------------ */
/* auth panel                                                         */
/* ------------------------------------------------------------------ */

.auth-stage {
  position: relative;
  z-index: 2;
  display: flex;
  justify-content: center;
}

.auth-card {
  width: min(440px, 100%);
  padding: 30px 30px 22px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-xl);
  background: color-mix(in srgb, var(--ui-surface-strong) 88%, transparent);
  box-shadow:
    inset 0 1px 0 color-mix(in srgb, #fff 6%, transparent),
    0 34px 90px rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(20px) saturate(1.15);
}

html:not(.dark) .auth-card {
  box-shadow:
    inset 0 1px 0 rgba(255, 255, 255, 0.8),
    0 34px 90px rgba(18, 42, 31, 0.16);
}

.auth-head {
  display: grid;
  grid-template-columns: 52px minmax(0, 1fr) auto;
  align-items: center;
  gap: 13px;
}

.auth-logo {
  position: relative;
  display: grid;
  place-items: center;
  width: 52px;
  height: 52px;

  img {
    width: 46px;
    height: 46px;
    border-radius: 14px;
    object-fit: cover;
    box-shadow: 0 10px 24px var(--ui-accent-shadow);
  }
}

.auth-logo-ring {
  position: absolute;
  inset: -4px;
  border: 1px solid color-mix(in srgb, var(--ui-accent) 40%, transparent);
  border-radius: 18px;
  animation: logo-ring 3.4s ease-in-out infinite;
  pointer-events: none;
}

@keyframes logo-ring {
  0%,
  100% {
    transform: scale(1);
    opacity: 0.55;
  }
  50% {
    transform: scale(1.07);
    opacity: 1;
  }
}

.auth-heading {
  min-width: 0;

  h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.01em;
    color: var(--ui-text);
  }

  p {
    margin: 4px 0 0;
    font-size: 11px;
    color: var(--ui-text-muted);
  }
}

.auth-version {
  font-family: var(--font-mono);
}

.auth-intro {
  display: flex;
  align-items: center;
  gap: 9px;
  margin: 24px 0 6px;
  padding: 11px 13px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-sm);
  background: var(--ui-surface);
  font-size: 12px;
  color: var(--ui-text-secondary);
}

.intro-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--ui-accent);
  animation: jade-pulse 2.6s ease-out infinite;
}

.auth-form {
  margin-top: 14px;

  :deep(.el-form-item) {
    margin-bottom: 14px;
  }

  :deep(.el-input__wrapper) {
    padding: 0;
    background: transparent;
    box-shadow: none !important;
  }

  :deep(.el-input__wrapper input:-webkit-autofill) {
    transition: background-color 1000s ease-in-out 0s;
  }
}

.field-shell {
  display: flex;
  align-items: center;
  width: 100%;
  padding: 0 10px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-md);
  background: var(--ui-surface-strong);
  transition:
    border-color var(--dur-mid) ease,
    box-shadow var(--dur-mid) ease,
    transform var(--dur-mid) var(--ease-out);
}

.field-shell:focus-within {
  border-color: var(--ui-accent);
  box-shadow: 0 0 0 3px var(--ui-focus-ring);
  transform: translateY(-1px);
}

.field-icon {
  flex-shrink: 0;
  margin-right: 9px;
  color: var(--ui-text-muted);
  transition: color var(--dur-mid) ease;
}

.field-shell:focus-within .field-icon {
  color: var(--ui-accent);
}

.field-input {
  flex: 1;

  :deep(.el-input__wrapper) {
    height: 46px;
  }
}

.auth-submit {
  width: 100%;
  height: 48px;
  margin-top: 4px;
  border-radius: var(--r-md) !important;
  font-size: 15px;
  letter-spacing: 0.14em;

  .submit-label {
    font-weight: 700;
  }

  .submit-arrow {
    display: inline-block;
    margin-left: 10px;
    transition: transform 260ms var(--ease-out);
  }

  &:hover .submit-arrow {
    transform: translateX(5px);
  }
}

.auth-foot {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 20px;
  padding-top: 14px;
  border-top: 1px dashed color-mix(in srgb, var(--ui-border) 80%, transparent);
  font-size: 11px;
  color: var(--ui-text-muted);
}

.foot-dot {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--ui-accent);
  opacity: 0.7;
}

/* ------------------------------------------------------------------ */
/* responsive                                                         */
/* ------------------------------------------------------------------ */

@media (max-width: 1024px) {
  .login-stage {
    grid-template-columns: 1fr;
    align-items: center;
    justify-items: center;
    padding-top: 96px;
  }

  .brand-stage {
    align-items: center;
    max-width: 640px;
    text-align: center;
  }

  .brand-features {
    display: none;
  }

  .brand-copy {
    margin-top: 16px;
  }

  .brand-meta {
    margin-top: 22px;
  }
}

@media (max-width: 520px) {
  .login-stage {
    padding: 84px 16px 40px;
  }

  .brand-title {
    font-size: clamp(30px, 9.4vw, 40px);
  }

  .auth-card {
    padding: 24px 20px 18px;
  }

  .stage-topbar {
    padding: 16px;
  }

  .stage-live span:last-child {
    display: none;
  }
}

@media (prefers-reduced-motion: reduce) {
  .stage-aurora,
  .live-dot,
  .auth-logo-ring {
    animation: none;
  }
}
</style>
