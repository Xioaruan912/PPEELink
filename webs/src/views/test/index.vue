<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch, type Component } from "vue";
import { useRoute, useRouter } from "vue-router";
import IconLock from "~icons/ph/lock-key-open";
import IconPulse from "~icons/ph/pulse";
import UnlockTest from "./unlock.vue";
import TcpTest from "./tcp.vue";

/** 节点测试：解锁检测与国内可达性在一个页面，用顶部标签切换。 */
defineOptions({ name: "NodeTest" });

type TabKey = "unlock" | "tcp";
const tabs: { key: TabKey; label: string; hint: string; icon: Component }[] = [
  {
    key: "unlock",
    label: "解锁检测",
    hint: "AI 与流媒体可用性",
    icon: IconLock,
  },
  {
    key: "tcp",
    label: "国内可达性",
    hint: "节点 IP 是否被墙",
    icon: IconPulse,
  },
];
const views: Record<TabKey, Component> = {
  unlock: UnlockTest,
  tcp: TcpTest,
};

const route = useRoute();
const router = useRouter();
const isTab = (v: unknown): v is TabKey => tabs.some((t) => t.key === v);
const active = ref<TabKey>(isTab(route.query.tab) ? route.query.tab : "unlock");
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
// 左右方向键在标签间移动
function onKey(e: KeyboardEvent, index: number) {
  const step = e.key === "ArrowRight" ? 1 : e.key === "ArrowLeft" ? -1 : 0;
  if (!step) return;
  e.preventDefault();
  const next = tabs[(index + step + tabs.length) % tabs.length];
  select(next.key);
  void nextTick(() => document.getElementById(`test-tab-${next.key}`)?.focus());
}

// 指示块跟随当前标签
const bar = ref<HTMLElement>();
const pill = ref({ x: 0, w: 0 });
function measure() {
  const el = bar.value?.querySelector<HTMLElement>(
    `[data-key="${active.value}"]`
  );
  if (el) pill.value = { x: el.offsetLeft, w: el.offsetWidth };
}
watch(active, () => nextTick(measure));
onMounted(() => {
  measure();
  if (bar.value) new ResizeObserver(measure).observe(bar.value);
});
const current = computed(() => views[active.value]);
</script>
<template>
  <div class="node-test">
    <nav
      ref="bar"
      class="test-tabs"
      role="tablist"
      aria-label="测试类型"
      data-enter
    >
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
        <span class="tab-text">
          <strong>{{ tab.label }}</strong>
          <small>{{ tab.hint }}</small>
        </span>
      </button>
    </nav>
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
.test-tabs {
  position: relative;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px;
  margin-bottom: 28px;
  padding: 4px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  box-shadow: var(--shadow-panel), var(--edge-highlight);
}
.tab-pill {
  position: absolute;
  top: 4px;
  bottom: 4px;
  left: 0;
  width: var(--w);
  border-radius: 14px;
  background: var(--c-ink);
  transform: translateX(var(--x));
  transition:
    transform 0.5s var(--ease-spring),
    width 0.5s var(--ease-spring);
}
.test-tab {
  position: relative;
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
  padding: 12px 16px;
  border: 0;
  border-radius: 14px;
  background: transparent;
  color: var(--c-muted);
  font: inherit;
  text-align: left;
  cursor: pointer;
  transition: color var(--duration-medium) var(--ease-out);
}
.test-tab:hover {
  color: var(--c-ink);
}
.test-tab[aria-selected="true"] {
  color: var(--c-surface);
}
.test-tab:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.tab-icon {
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  transition: transform 0.5s var(--ease-spring);
}
.test-tab[aria-selected="true"] .tab-icon {
  transform: rotate(-8deg) scale(1.08);
}
.tab-text {
  display: grid;
  min-width: 0;
}
.tab-text strong {
  font-size: 14px;
  font-weight: 600;
}
.tab-text small {
  overflow: hidden;
  font-size: 12px;
  opacity: 0.75;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.tab-swap-enter-active,
.tab-swap-leave-active {
  transition:
    opacity 0.28s var(--ease-out),
    transform 0.36s var(--ease-out),
    filter 0.28s var(--ease-out);
}
.tab-swap-enter-from {
  opacity: 0;
  transform: translateY(14px);
  filter: blur(6px);
}
.tab-swap-leave-to {
  opacity: 0;
  transform: translateY(-8px);
  filter: blur(4px);
}
@media (max-width: 767px) {
  .test-tabs {
    margin-bottom: 20px;
  }
  .test-tab {
    flex-direction: column;
    gap: 4px;
    padding: 10px 4px;
    text-align: center;
  }
  .tab-text small {
    display: none;
  }
  .tab-text strong {
    font-size: 13px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .tab-pill,
  .tab-icon,
  .tab-swap-enter-active,
  .tab-swap-leave-active {
    transition: none;
  }
}
</style>
