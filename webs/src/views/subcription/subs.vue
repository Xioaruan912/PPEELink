<script setup lang="ts">
import { Plus } from "@element-plus/icons-vue";
import SubscriptionHeader from "@/components/subscriptions/SubscriptionHeader.vue";
import { UiButton } from "@/ui";
import { provideSubscriptions } from "./subs/useSubscriptions";
import SubscriptionList from "./subs/SubscriptionList.vue";
import EditorDrawer from "./subs/EditorDrawer.vue";
import DetailDrawer from "./subs/DetailDrawer.vue";
import ShareDialogs from "./subs/ShareDialogs.vue";
import ConfirmDialog from "./subs/ConfirmDialog.vue";
defineOptions({ name: "SubsPage" });
const { handleAddSub } = provideSubscriptions();
</script>
<template>
  <div class="subscriptions-page">
    <SubscriptionHeader
      section="subscriptions"
      title="订阅管理"
      description="管理节点来源，为设备生成对应客户端的订阅链接。"
      ><template #actions
        ><UiButton variant="primary" :icon="Plus" @click="handleAddSub"
          >新建订阅</UiButton
        ></template
      ></SubscriptionHeader
    >
    <SubscriptionList />
    <EditorDrawer /><DetailDrawer /><ShareDialogs /><ConfirmDialog />
  </div>
</template>
<style scoped>
.subscriptions-page {
  min-width: 0;
}
</style>
