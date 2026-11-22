<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
  type Component,
} from "vue";
import { useRoute, useRouter } from "vue-router";
import gsap from "gsap";
import IconPulse from "~icons/ph/pulse";
import IconGlobe from "~icons/ph/globe-hemisphere-east";
import TcpTest from "./tcp.vue";
import DeviceUnlock from "./DeviceUnlock.vue";

defineOptions({ name: "NodeTest" });

type TabKey = "device" | "tcp";
const tabs: { key: TabKey; label: string; icon: Component }[] = [
  { key: "device", label: "当前设备检测", icon: IconPulse },
  { key: "tcp", label: "国内 TCP Ping", icon: IconGlobe },
];

const views: Record<TabKey, Component> = {
  device: DeviceUnlock,
  tcp: TcpTest,
};

const route = useRoute();
const router = useRouter();
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const isTab = (v: unknown): v is TabKey => tabs.some((t) => t.key === v);
const active = ref<TabKey>(isTab(route.query.tab) ? route.query.tab : "device");

watch(
  () => route.query.tab,
  (tab) => {
    if (isTab(tab)) active.value = tab;
  }
);

function select(key: TabKey) {
  active.value = key;
  void router.replace({ query: { ...route.query, tab: key } });
}

function onKey(e: KeyboardEvent, index: number) {
  const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
  if (!step) return;
  e.preventDefault();
  const next = tabs[(index + step + tabs.length) % tabs.length];
  select(next.key);
  void nextTick(() => document.getElementById(`test-tab-${next.key}`)?.focus());
}

const bar = ref<HTMLElement>();
const pill = ref({ x: 0, w: 0 });
let ro: ResizeObserver | null = null;

function measure() {
  const el = bar.value?.querySelector<HTMLElement>(
    `[data-key="${active.value}"]`
  );
  if (el) pill.value = { x: el.offsetLeft, w: el.offsetWidth };
}

watch(active, () => nextTick(measure));

onMounted(() => {
  measure();
  if (bar.value && typeof ResizeObserver !== "undefined") {
    ro = new ResizeObserver(measure);
    ro.observe(bar.value);
  }
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  ro?.disconnect();
  ro = null;
  gsapCtx?.revert();
  gsapCtx = null;
});

const current = computed(() => views[active.value]);
</script>

<template>
  <div ref="rootEl" class="test-hub">
    <header class="test-masthead">
      <div class="title-mask">
        <h1 class="gallery-title" data-title-reveal>解锁与可达</h1>
      </div>

      <nav ref="bar" class="test-tabs" role="tablist" aria-label="测试模式">
        <i
          class="tab-pill"
          aria-hidden="true"
          :style="{ '--x': `${pill.x}px`, '--w': `${pill.w}px` }"
        ></i>
        <button
          v-for="(tab, i) in tabs"
          :id="`test-tab-${tab.key}`"
          :key="tab.key"
          type="button"
          role="tab"
          class="test-tab"
          :data-key="tab.key"
          :aria-selected="active === tab.key"
          :aria-controls="`test-panel-${tab.key}`"
          :tabindex="active === tab.key ? 0 : -1"
          @click="select(tab.key)"
          @keydown="onKey($event, i)"
        >
          <component :is="tab.icon" class="tab-icon" />
          <span>{{ tab.label }}</span>
        </button>
      </nav>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <div
      :id="`test-panel-${active}`"
      role="tabpanel"
      :aria-labelledby="`test-tab-${active}`"
    >
      <Transition name="tab-swap" mode="out-in">
        <KeepAlive>
          <component :is="current" :key="active" />
        </KeepAlive>
      </Transition>
    </div>
  </div>
</template>

<style scoped>
.test-hub {
  display: grid;
  gap: 20px;
  max-width: 1180px;
  min-width: 0;
}

.test-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.test-tabs {
  position: relative;
  display: inline-grid;
  grid-template-columns: repeat(2, auto);
  gap: 2px;
  padding: 4px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
}

.tab-pill {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 0;
  width: var(--w);
  border-radius: 999px;
  background: var(--c-ink);
  transform: translateX(var(--x));
  transition:
    transform 0.45s cubic-bezier(0.16, 1, 0.3, 1),
    width 0.45s cubic-bezier(0.16, 1, 0.3, 1);
}

.test-tab {
  position: relative;
  z-index: 1;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 8px 16px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-out);
}

.test-tab:hover {
  color: var(--c-ink);
}

.test-tab[aria-selected="true"] {
  color: var(--c-surface);
}

.tab-icon {
  width: 15px;
  height: 15px;
}

.tab-swap-enter-active,
.tab-swap-leave-active {
  transition:
    opacity 220ms var(--ease-out),
    transform 240ms var(--ease-out);
}

.tab-swap-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.tab-swap-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (prefers-reduced-motion: reduce) {
  .tab-pill,
  .tab-swap-enter-active,
  .tab-swap-leave-active {
    transition: none;
  }
}
</style>
