<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from "vue";
import IconRefresh from "~icons/ph/arrows-clockwise";
import { getMySubscriptions } from "@/api/user";
import { getMySelfHosted } from "@/api/proxy";
import type { MySubscription } from "@/api/user/types";
import { EmptyState, PageHeader, UiButton, UiSkeleton } from "@/ui";
import { useStagger } from "@/motion";
import { useUserStore } from "@/store";
import ClientShare from "@/components/share/ClientShare.vue";
import TrafficMeter from "@/components/share/TrafficMeter.vue";
import { trafficOf } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";

defineOptions({ name: "MySubscriptions" });
const user = useUserStore();
const items = ref<MySubscription[]>([]);
const loading = ref(true);
const loadError = ref("");
const activeId = ref<number>();
const list = ref<HTMLElement>();
const stagger = useStagger();

const active = computed(
  () => items.value.find((s) => s.id === activeId.value) || items.value[0]
);
const expired = (s: MySubscription) =>
  !!s.expiresAt && new Date(s.expiresAt).getTime() <= Date.now();
// 复用流量组件：把汇总流量转换成它需要的字段
const traffic = (s: MySubscription) =>
  s.traffic.reported
    ? trafficOf({
        TrafficUpload: s.traffic.upload,
        TrafficDownload: s.traffic.download,
        TrafficTotal: s.traffic.total,
        TrafficExpire: s.traffic.expire,
        TrafficUpdatedAt: "reported",
      })
    : null;

async function load() {
  loading.value = true;
  loadError.value = "";
  try {
    const [{ data }, self] = await Promise.all([
      getMySubscriptions(),
      getMySelfHosted().catch(() => ({ data: { enabled: false } as any })),
    ]);
    items.value = Array.isArray(data) ? data : [];
    // 管理员为账户开通的自建节点，作为一份独立订阅排在最前面
    const sh = self.data;
    if (sh?.enabled && sh.links) {
      items.value.unshift({
        id: -1,
        name: sh.active
          ? "自建节点"
          : `自建节点（${sh.blockedReason || "已停用"}）`,
        expiresAt: sh.expireAt || null,
        links: sh.links,
        imports: sh.imports || {},
        compatibility: sh.compatibility,
        traffic: {
          reported: !!sh.limit,
          upload: 0,
          download: sh.used || 0,
          total: sh.limit || 0,
          expire: sh.expireAt || null,
        },
      });
    }
    if (!items.value.some((s) => s.id === activeId.value))
      activeId.value = items.value[0]?.id;
  } catch (error: any) {
    loadError.value = error?.message || "请检查网络后重试";
  } finally {
    loading.value = false;
    await nextTick();
    if (list.value) stagger(list.value.querySelectorAll(".sub-card"));
  }
}
onMounted(load);
</script>
<template>
  <div class="my-page">
    <PageHeader
      title="我的订阅"
      :description="`${user.user.nickname || user.user.username || ''}，选择客户端即可导入。`"
    >
      <template #actions>
        <UiButton :icon="IconRefresh" :loading="loading" @click="load"
          >刷新</UiButton
        >
      </template>
    </PageHeader>
    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载订阅"
      :description="loadError"
      ><template #action
        ><UiButton @click="load">重新加载</UiButton></template
      ></EmptyState
    >
    <UiSkeleton v-else-if="loading" :lines="3" height="96px" />
    <EmptyState
      v-else-if="!items.length"
      size="page"
      title="还没有分配订阅"
      description="请联系管理员为你的账户分配订阅。"
    />
    <div v-else class="my-layout">
      <div ref="list" class="sub-list" role="tablist" aria-label="我的订阅">
        <button
          v-for="sub in items"
          :key="sub.id"
          class="sub-card"
          role="tab"
          :aria-selected="active?.id === sub.id"
          @click="activeId = sub.id"
        >
          <span class="sub-head">
            <strong>{{ sub.name }}</strong>
            <span class="sub-state" :class="{ 'is-bad': expired(sub) }">{{
              expired(sub) ? "已过期" : "可用"
            }}</span>
          </span>
          <TrafficMeter v-if="traffic(sub)" :traffic="traffic(sub)" />
          <span v-else class="sub-expire">{{
            sub.expiresAt ? `到期 ${formatDateTime(sub.expiresAt)}` : "长期有效"
          }}</span>
        </button>
      </div>
      <section v-if="active" class="share-panel" aria-live="polite">
        <h2>{{ active.name }}</h2>
        <ClientShare
          :links="active.links"
          :imports="active.imports"
          :reset-key="active.id"
          :compatibility="active.compatibility"
        />
      </section>
    </div>
  </div>
</template>
<style scoped>
.my-page {
  min-width: 0;
}
.my-layout {
  display: grid;
  grid-template-columns: minmax(240px, 320px) minmax(0, 1fr);
  gap: 20px;
  align-items: start;
}
.sub-list {
  display: grid;
  gap: 8px;
}
.sub-card {
  display: grid;
  gap: 10px;
  padding: 16px 18px;
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  text-align: left;
  transition:
    background-color var(--duration-fast) ease,
    transform var(--duration-press) var(--ease-out);
}
@media (hover: hover) and (pointer: fine) {
  .sub-card:hover {
    background: var(--c-surface-2);
  }
}
.sub-card:active {
  transform: scale(0.99);
}
.sub-card[aria-selected="true"] {
  background: var(--c-ink);
  color: var(--c-surface);
}
.sub-card[aria-selected="true"] .sub-expire {
  color: color-mix(in srgb, var(--c-surface) 70%, transparent);
}
.sub-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.sub-head strong {
  overflow: hidden;
  font-size: 15px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.sub-state {
  flex: 0 0 auto;
  padding: 2px 8px;
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--c-ok) 15%, transparent);
  color: var(--c-ok);
  font-size: 12px;
}
.sub-state.is-bad {
  background: color-mix(in srgb, var(--c-bad) 15%, transparent);
  color: var(--c-bad);
}
.sub-expire {
  color: var(--c-muted);
  font-size: 13px;
}
.share-panel {
  min-width: 0;
  padding: 24px;
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.share-panel h2 {
  margin-bottom: 18px;
  font-size: 18px;
  font-weight: 600;
  letter-spacing: -0.02em;
}
@media (max-width: 899px) {
  .my-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .share-panel {
    padding: 14px;
  }
}
</style>
