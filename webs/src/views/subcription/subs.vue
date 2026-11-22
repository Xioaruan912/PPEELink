<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import gsap from "gsap";
import { Plus, Refresh } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import { provideSubscriptions } from "./subs/useSubscriptions";
import SubscriptionList from "./subs/SubscriptionList.vue";
import EditorDrawer from "./subs/EditorDrawer.vue";
import DetailDrawer from "./subs/DetailDrawer.vue";
import ShareDialogs from "./subs/ShareDialogs.vue";
import ConfirmDialog from "./subs/ConfirmDialog.vue";

defineOptions({ name: "SubsPage" });

const { tableData, loading, loadError, expiredCount, getsubs, handleAddSub } =
  provideSubscriptions();

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const activeCount = computed(() =>
  Math.max(tableData.value.length - expiredCount.value, 0)
);

onMounted(() => {
  if (!rootEl.value) return;
  gsapCtx = gsap.context(() => {
    const mm = gsap.matchMedia();
    mm.add("(prefers-reduced-motion: no-preference)", () => {
      const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
      tl.from("[data-title-reveal]", {
        yPercent: 108,
        duration: 0.64,
        clearProps: "transform",
      })
        .from(
          "[data-metric-item]",
          {
            y: 12,
            opacity: 0,
            duration: 0.48,
            stagger: 0.055,
            clearProps: "transform,opacity",
          },
          "-=0.44"
        )
        .from(
          "[data-lead-reveal]",
          {
            y: 10,
            opacity: 0,
            duration: 0.46,
            clearProps: "transform,opacity",
          },
          "-=0.38"
        )
        .from(
          "[data-actions-reveal]",
          {
            y: 8,
            opacity: 0,
            duration: 0.42,
            clearProps: "transform,opacity",
          },
          "-=0.36"
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
  <div ref="rootEl" class="subs-gallery">
    <!-- Compact Editorial Masthead with Architectural Curtain Reveal -->
    <header class="gallery-masthead" aria-labelledby="subs-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <div class="title-mask">
            <h1 id="subs-title" class="gallery-title" data-title-reveal>
              订阅链接
            </h1>
          </div>
          <dl class="inline-metrics mono" aria-label="订阅概览指标">
            <div data-metric-item>
              <dt>总数</dt>
              <dd>{{ loading || loadError ? "—" : tableData.length }}</dd>
            </div>
            <div data-metric-item>
              <dt>有效</dt>
              <dd class="is-ok">
                {{ loading || loadError ? "—" : activeCount }}
              </dd>
            </div>
            <div v-if="expiredCount > 0" data-metric-item>
              <dt>已过期</dt>
              <dd class="is-bad">{{ expiredCount }}</dd>
            </div>
          </dl>
        </div>
        <p class="gallery-lead" data-lead-reveal>
          选择客户端一键复制链接、扫码导入，或点击名称调整节点与分流规则。
        </p>
      </div>

      <div class="masthead-actions" data-actions-reveal>
        <UiButton variant="primary" :icon="Plus" @click="handleAddSub">
          新建订阅
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          @click="getsubs"
        >
          {{ loading ? "同步中" : "刷新" }}
        </UiButton>
      </div>
    </header>

    <!-- Borderless Swiss Gallery Subscription Ledger -->
    <SubscriptionList />

    <EditorDrawer />
    <DetailDrawer />
    <ShareDialogs />
    <ConfirmDialog />
  </div>
</template>

<style scoped>
.subs-gallery {
  display: grid;
  gap: 20px;
  max-width: 1180px;
  min-width: 0;
}

.gallery-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
  padding-top: 2px;
}

.masthead-left {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.title-cluster {
  display: flex;
  align-items: baseline;
  gap: 20px;
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
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.inline-metrics {
  display: inline-flex;
  align-items: baseline;
  gap: 16px;
  margin: 0;
}

.inline-metrics > div {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
}

.inline-metrics dt {
  font-family: var(--font-ui);
  font-size: 12px;
  color: var(--c-muted);
}

.inline-metrics dd {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--c-ink);
}

.inline-metrics dd.is-ok {
  color: var(--c-ok);
}

.inline-metrics dd.is-bad {
  color: var(--c-bad);
}

.gallery-lead {
  margin: 0;
  font-size: 13.5px;
  line-height: 1.55;
  color: var(--c-muted);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .title-cluster {
    gap: 10px 16px;
  }
}
</style>
