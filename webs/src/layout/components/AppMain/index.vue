<template>
  <section class="app-main">
    <router-view v-slot="{ Component, route }"
      ><Transition
        mode="out-in"
        :css="false"
        appear
        @enter="enter"
        @leave="leaveFrame"
        @enter-cancelled="cancel"
        @leave-cancelled="cancel"
        ><KeepAlive :include="cachedViews"
          ><component :is="routeFrame(route).component" :key="route.path"
            ><component :is="Component" /></component></KeepAlive></Transition
    ></router-view>
  </section>
</template>
<script setup lang="ts">
import {
  computed,
  defineComponent,
  h,
  nextTick,
  type Component as VueComponent,
} from "vue";
import { useRouter } from "vue-router";
import { useRouteTransition } from "@/motion";
const router = useRouter();
const { enter, leave, cancel } = useRouteTransition();
function leaveFrame(element: Element, done: () => void) {
  // Reduced motion and failed animations can finish synchronously. Let Vue
  // finish patching KeepAlive's placeholder before afterLeave updates it again.
  leave(element, () => void nextTick(done));
}
const frames = new Map<
  string | symbol,
  { name: string; component: VueComponent }
>();
function routeFrame(route: { name?: string | symbol | null; path: string }) {
  const key = route.name ?? route.path;
  let frame = frames.get(key);
  if (!frame) {
    const name = `AtlasRouteFrame${frames.size}`;
    frame = {
      name,
      component: defineComponent({
        name,
        setup(_, { slots }) {
          // Transition always owns an element, even for Fragment/comment views.
          return () => h("div", { class: "route-frame" }, slots.default?.());
        },
      }),
    };
    frames.set(key, frame);
  }
  return frame;
}
// Cache the stable frame and its view together; a keyed div around KeepAlive
// would destroy the cache on every navigation.
const cachedViews = computed(() =>
  router
    .getRoutes()
    .filter((route) => route.meta.keepAlive && route.name)
    .map((route) => routeFrame(route).name)
);
</script>
<style scoped>
.app-main {
  position: relative;
  min-width: 0;
  width: 100%;
}
.app-main :deep(> *),
.app-main :deep(.route-frame > *) {
  min-width: 0;
  max-width: 100%;
}
</style>
