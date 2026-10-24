<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import gsap from "gsap";
import { usePrefersReducedMotion } from "@/motion";
import IconMonitor from "~icons/ph/chart-line-up";
import IconServer from "~icons/ph/hard-drives";
import IconFolder from "~icons/ph/folders";
import IconGlobe from "~icons/ph/globe-hemisphere-west";
const props = defineProps<{ title: string; description: string; section: "monitor" | "proxy" | "workspace" | "traffic" }>();
const links = [
  { key: "monitor", path: "/monitor/index", label: "监控与 Agent", hint: "运行状态与远程终端", icon: IconMonitor },
  { key: "proxy", path: "/subcription/servers", label: "自建节点", hint: "入站配置与账户管理", icon: IconServer },
  { key: "workspace", path: "/monitor/workspace", label: "文件与容器", hint: "服务器文件与 Docker", icon: IconFolder },
  { key: "traffic", path: "/monitor/website", label: "网站流量", hint: "流量去向与使用占比", icon: IconGlobe },
];
const current = computed(() => links.find(item => item.key === props.section)!);
const root = ref<HTMLElement>();
const reduced = usePrefersReducedMotion();
let animation: gsap.Context | undefined;
let moveX: gsap.QuickToFunc | undefined;
let moveY: gsap.QuickToFunc | undefined;
function pointerMove(event: PointerEvent) {
  if (reduced.value || event.pointerType !== "mouse" || !root.value) return;
  const rect = root.value.getBoundingClientRect();
  moveX?.((event.clientX - rect.left - rect.width / 2) * .035);
  moveY?.((event.clientY - rect.top - rect.height / 2) * .06);
}
function resetPointer() { moveX?.(0); moveY?.(0); }
function stop() { animation?.revert(); animation = undefined; moveX = undefined; moveY = undefined; }
function start() {
  stop();
  if (!root.value || reduced.value) return;
  animation = gsap.context(() => {
    gsap.from(".server-heading-copy > *", { y: 20, opacity: 0, duration: .8, stagger: .09, ease: "expo.out", clearProps: "all" });
    gsap.from(".server-module", { y: 18, opacity: 0, duration: .65, stagger: .07, delay: .12, ease: "power3.out", clearProps: "all" });
    moveX = gsap.quickTo(".server-emblem", "x", { duration: .8, ease: "power3.out" });
    moveY = gsap.quickTo(".server-emblem", "y", { duration: .8, ease: "power3.out" });
  }, root.value);
}
onMounted(start);
watch(reduced, start);
onBeforeUnmount(stop);
</script>

<template>
  <header ref="root" class="server-heading" @pointermove="pointerMove" @pointerleave="resetPointer">
    <div class="server-heading-main">
      <div class="server-heading-copy">
        <span class="server-section-label">服务器空间</span>
        <h1>{{ title }}</h1>
        <p>{{ description }}</p>
        <div v-if="$slots.actions" class="server-heading-actions"><slot name="actions" /></div>
      </div>
      <div class="server-emblem" aria-hidden="true"><component :is="current.icon" /></div>
    </div>
    <nav class="server-modules" aria-label="服务器功能">
      <RouterLink v-for="item in links" :key="item.key" :to="item.path"
        class="server-module" :class="{ 'is-active': item.key === section }"
        :aria-current="item.key === section ? 'page' : undefined">
        <span class="server-module-icon"><component :is="item.icon" /></span>
        <span class="server-module-copy"><strong>{{ item.label }}</strong><small>{{ item.hint }}</small></span>
      </RouterLink>
    </nav>
  </header>
</template>

<style scoped>
.server-heading { position: relative; isolation: isolate; min-width: 0; border: 1px solid var(--c-line); border-radius: 26px; background: linear-gradient(120deg, var(--c-surface) 12%, color-mix(in srgb, var(--c-signal) 22%, var(--c-surface)) 100%); box-shadow: 0 16px 46px -34px var(--c-focus); overflow: hidden; }
.server-heading::before { content: ""; position: absolute; z-index: -1; width: 45%; height: 240px; right: -8%; top: -120px; border: 44px solid color-mix(in srgb, var(--c-signal) 14%, transparent); border-radius: 50%; transform: rotate(-24deg); pointer-events: none; }
.server-heading-main { display: flex; align-items: center; justify-content: space-between; gap: 24px; padding: 26px 30px 22px; }
.server-heading-copy { min-width: 0; }
.server-section-label { color: var(--c-signal-text); font-size: 12px; font-weight: 600; }
.server-heading h1 { margin: 8px 0 8px; font: 600 clamp(26px, 3vw, 40px)/1.16 var(--font-display); letter-spacing: -.035em; color: var(--c-ink); }
.server-heading p { margin: 0; font-size: 13px; color: var(--c-muted); line-height: 1.7; max-width: 60ch; }
.server-heading-actions { display: flex; gap: 8px; flex-wrap: wrap; margin-top: 18px; }
.server-emblem { flex: 0 0 112px; height: 112px; display: grid; place-items: center; border: 1px solid color-mix(in srgb, var(--c-surface) 75%, var(--c-signal)); border-radius: 32px; background: linear-gradient(135deg, var(--c-surface), color-mix(in srgb, var(--c-signal) 35%, var(--c-surface))); box-shadow: 0 18px 36px -22px var(--c-focus), inset 0 1px 0 var(--c-surface); color: var(--c-signal-text); transform: rotate(-8deg); margin-right: 16px; }
.server-emblem svg { width: 56px; height: 56px; }
.server-modules { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; padding: 8px; border-top: 1px solid var(--c-line); background: color-mix(in srgb, var(--c-surface) 70%, transparent); }
.server-module { display: flex; align-items: center; gap: 10px; min-width: 0; padding: 12px; border-radius: 17px; color: var(--c-muted); transition: transform .35s var(--ease-spring), background .2s, color .2s; }
.server-module.is-active { background: var(--c-signal); color: var(--c-on-signal); box-shadow: 0 7px 18px -12px var(--c-focus); }
.server-module-icon { display: grid; place-items: center; flex: 0 0 34px; height: 34px; border-radius: 11px; background: color-mix(in srgb, var(--c-surface) 65%, transparent); }
.server-module-icon svg { width: 20px; height: 20px; }
.server-module-copy { min-width: 0; display: grid; gap: 3px; }
.server-module strong { font-size: 13px; font-weight: 600; white-space: nowrap; }
.server-module small { font-size: 11px; opacity: .85; white-space: nowrap; }
.server-module:focus-visible { outline: 2px solid var(--c-focus); outline-offset: -2px; }
@media (hover: hover) and (prefers-reduced-motion: no-preference) {
  .server-module:hover { transform: translateY(-3px); background: var(--c-surface-2); color: var(--c-ink); }
  .server-module.is-active:hover { background: var(--c-signal); }
}
@media(max-width: 1000px) { .server-module small { display: none; } }
@media(max-width: 767px) {
  .server-heading { border-radius: 20px; }
  .server-heading-main { padding: 20px 18px; gap: 10px; }
  .server-emblem { flex-basis: 56px; height: 64px; border-radius: 20px; margin-right: 0; }
  .server-emblem svg { width: 32px; height: 32px; }
  .server-heading-actions :deep(button) { min-height: 44px; }
  .server-modules { grid-template-columns: repeat(2, minmax(0, 1fr)); padding: 6px; gap: 4px; }
  .server-module { min-height: 52px; padding: 8px; gap: 7px; border-radius: 13px; }
  .server-module strong { font-size: 12px; }
  .server-module-icon { flex-basis: 28px; height: 30px; }
}
@media(prefers-reduced-motion: reduce) { .server-module { transition: none; } }
:global(html[data-motion="reduced"]) .server-module { transition: none; transform: none; }
</style>

<style scoped>
.server-heading{background:var(--c-surface);box-shadow:none;border-radius:16px}.server-heading::before{display:none}.server-emblem{background:var(--c-surface-2);box-shadow:none;border:1px solid var(--c-line);color:var(--c-muted)}.server-modules{background:var(--c-canvas)}.server-module{border-radius:9px}.server-module.is-active{background:var(--c-surface);color:var(--c-signal-text);box-shadow:0 1px 4px #0000000d}.server-module-icon{background:transparent}.server-module.is-active:hover{background:var(--c-surface)}
</style>
