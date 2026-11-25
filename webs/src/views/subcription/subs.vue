<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import gsap from "gsap";
import { MagicStick, Plus, Refresh } from "@element-plus/icons-vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { UiButton } from "@/ui";
import {
  getUnlockRoutingPreview,
  type UnlockRoutingPreview,
} from "@/api/proxy";
import { generateSmartClashTemplate } from "@/api/template/temp";
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

const routingPreview = ref<UnlockRoutingPreview | null>(null);
const smartTplBusy = ref(false);

async function loadRoutingPreview() {
  try {
    const { data } = await getUnlockRoutingPreview();
    routingPreview.value = data || null;
  } catch {
    // Non-fatal if user is non-admin or no servers configured
  }
}

async function handleSyncSmartTemplate(applyAllSubs = false) {
  if (smartTplBusy.value) return;
  if (applyAllSubs) {
    try {
      await ElMessageBox.confirm(
        "将生成最新智能分流模板 clash-smart.yaml，并把全部订阅的 Clash 模板切换为该模板（保留 UDP / proxy-provider 等原有设置）。是否继续？",
        "应用智能分流 Clash 模板",
        {
          confirmButtonText: "确认应用",
          cancelButtonText: "取消",
          type: "info",
        }
      );
    } catch {
      return;
    }
  }
  smartTplBusy.value = true;
  try {
    const res = await generateSmartClashTemplate({ applyAllSubs });
    ElMessage.success(res?.msg || "已同步智能分流 Clash 模板");
    if (applyAllSubs) {
      await getsubs();
    }
  } catch (err: any) {
    ElMessage.error(err?.message || "同步智能分流模板失败");
  } finally {
    smartTplBusy.value = false;
  }
}

function refreshAll() {
  void getsubs();
  void loadRoutingPreview();
}

onMounted(() => {
  void loadRoutingPreview();
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
          @click="refreshAll"
        >
          {{ loading ? "同步中" : "刷新" }}
        </UiButton>
      </div>
    </header>

    <!-- Borderless Swiss Gallery Subscription Ledger -->
    <SubscriptionList />

    <!-- Smart Unlock Subscription Routing Preview -->
    <section
      v-if="routingPreview && routingPreview.groups.length"
      class="smart-route-card"
      aria-label="原生解锁智能分流"
    >
      <header class="smart-route-head">
        <div class="smart-route-title">
          <h2>订阅智能分流</h2>
          <span class="sr-pill is-ok">自动生效中</span>
          <span class="sr-pill mono">
            已验 {{ routingPreview.checkedServers }}/{{
              routingPreview.totalServers
            }}
            台
          </span>
        </div>
        <div class="smart-route-actions">
          <UiButton
            size="sm"
            variant="ghost"
            :icon="MagicStick"
            :loading="smartTplBusy"
            @click="handleSyncSmartTemplate(false)"
          >
            同步智能模板
          </UiButton>
          <UiButton
            size="sm"
            variant="primary"
            :loading="smartTplBusy"
            @click="handleSyncSmartTemplate(true)"
          >
            应用至全部订阅
          </UiButton>
        </div>
      </header>

      <div class="smart-route-grid">
        <div
          v-for="g in routingPreview.groups"
          :key="g.key"
          class="smart-route-row"
        >
          <span class="sr-service">{{ g.service }}</span>
          <div class="sr-nodes">
            <span
              v-for="name in g.included"
              :key="`in-${name}`"
              class="sr-node is-ok"
            >
              {{ name }}
            </span>
            <span
              v-for="name in g.excluded"
              :key="`ex-${name}`"
              class="sr-node is-off"
            >
              {{ name }}
            </span>
          </div>
        </div>
      </div>
    </section>

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

.smart-route-card {
  display: grid;
  gap: 16px;
  padding: 20px 22px;
  border: 1px solid var(--c-line);
  border-radius: 20px;
  background: var(--c-surface);
}

.smart-route-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.smart-route-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.smart-route-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.smart-route-title h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--c-ink);
}

.sr-pill {
  padding: 2px 9px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11.5px;
  font-weight: 500;
}

.sr-pill.is-ok {
  background: color-mix(in srgb, var(--c-ok) 12%, transparent);
  color: var(--c-ok);
}

.smart-route-grid {
  display: grid;
}

.smart-route-row {
  display: grid;
  grid-template-columns: 140px 1fr;
  align-items: center;
  gap: 12px;
  padding: 12px 2px;
  border-bottom: 1px solid var(--c-line);
}

.smart-route-row:first-child {
  padding-top: 2px;
}

.smart-route-row:last-child {
  padding-bottom: 2px;
  border-bottom: none;
}

.sr-service {
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
}

.sr-nodes {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.sr-node {
  padding: 2px 9px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 500;
}

.sr-node.is-ok {
  background: color-mix(in srgb, var(--c-ok) 12%, transparent);
  color: var(--c-ok);
}

.sr-node.is-off {
  background: var(--c-surface-2);
  color: var(--c-muted);
  text-decoration: line-through;
  opacity: 0.72;
}

@media (max-width: 640px) {
  .smart-route-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
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
