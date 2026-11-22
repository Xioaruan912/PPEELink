<script setup lang="ts">
import { Plus } from "@element-plus/icons-vue";
import PageHeader from "@/ui/PageHeader.vue";
import { UiButton } from "@/ui";
import { provideSubscriptions } from "./subs/useSubscriptions";
import SubscriptionList from "./subs/SubscriptionList.vue";
import EditorDrawer from "./subs/EditorDrawer.vue";
import DetailDrawer from "./subs/DetailDrawer.vue";
import ShareDialogs from "./subs/ShareDialogs.vue";
import ConfirmDialog from "./subs/ConfirmDialog.vue";
defineOptions({ name: "SubsPage" });
const { tableData, loading, loadError, expiredCount, handleAddSub } =
  provideSubscriptions();
</script>
<template>
  <div class="subscriptions-page">
    <aside class="editorial-masthead" data-enter>
      <PageHeader
        title="订阅链接"
        description="选择客户端，一键复制订阅链接，或扫码导入你的设备。"
        ><template #actions
          ><UiButton variant="primary" :icon="Plus" @click="handleAddSub"
            >新建订阅</UiButton
          ></template
        ></PageHeader
      >
      <div class="masthead-ledger" aria-label="订阅概览">
        <div>
          <span>订阅总数</span>
          <strong>{{ loading || loadError ? "—" : tableData.length }}</strong>
        </div>
        <div>
          <span>有效</span>
          <strong class="ok">{{
            loading || loadError ? "—" : tableData.length - expiredCount
          }}</strong>
        </div>
        <div>
          <span>已过期</span>
          <strong :class="{ bad: expiredCount }">{{
            loading || loadError ? "—" : expiredCount
          }}</strong>
        </div>
      </div>
    </aside>

    <div class="editorial-stage">
      <SubscriptionList />
    </div>

    <EditorDrawer /><DetailDrawer /><ShareDialogs /><ConfirmDialog />
  </div>
</template>
<style scoped>
.subscriptions-page {
  display: grid;
  grid-template-columns: 300px minmax(0, 1fr);
  gap: 40px;
  align-items: start;
  min-width: 0;
}
.editorial-masthead {
  position: sticky;
  top: 28px;
  display: grid;
  gap: 28px;
}
.editorial-masthead :deep(.page-header) {
  margin-bottom: 0;
}
.editorial-masthead :deep(.page-header__grid) {
  flex-direction: column;
  align-items: stretch;
  gap: 18px;
}
.editorial-masthead :deep(.page-header__actions button) {
  width: 100%;
}
.masthead-ledger {
  display: grid;
  border-top: 1px solid var(--c-ink);
}
.masthead-ledger > div {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 2px;
  border-bottom: 1px solid var(--c-line);
}
.masthead-ledger span {
  font-size: 13px;
  font-weight: 540;
  color: var(--c-muted);
}
.masthead-ledger strong {
  font: 700 28px/1 var(--font-display);
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}
.masthead-ledger .ok {
  color: var(--c-ok);
}
.masthead-ledger .bad {
  color: var(--c-bad);
}
.editorial-stage {
  min-width: 0;
}
@media (max-width: 1024px) {
  .subscriptions-page {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .editorial-masthead {
    position: static;
    gap: 20px;
  }
  .masthead-ledger {
    grid-template-columns: repeat(3, minmax(0, 1fr));
    border: 1px solid var(--c-line);
    border-radius: var(--radius-panel);
    background: var(--c-surface);
  }
  .masthead-ledger > div {
    padding: 14px 16px;
    border-bottom: 0;
    border-right: 1px solid var(--c-line);
  }
  .masthead-ledger > div:last-child {
    border-right: 0;
  }
}
</style>
