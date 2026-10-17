<script setup lang="ts">
import { Plus } from "@element-plus/icons-vue";
import { PageHeader, UiButton } from "@/ui";
import { provideSubscriptions } from "./subs/useSubscriptions";
import SubscriptionList from "./subs/SubscriptionList.vue";
import EditorDrawer from "./subs/EditorDrawer.vue";
import DetailDrawer from "./subs/DetailDrawer.vue";
import ArtifactDrawer from "./subs/ArtifactDrawer.vue";
import ShareDialogs from "./subs/ShareDialogs.vue";
import PublishDialogs from "./subs/PublishDialogs.vue";
defineOptions({ name: "SubsPage" });
const { handleAddSub } = provideSubscriptions();
</script>
<template>
  <div class="subscriptions-page">
    <PageHeader title="订阅"
      ><template #actions
        ><UiButton variant="primary" :icon="Plus" @click="handleAddSub"
          >新建订阅</UiButton
        ></template
      ></PageHeader
    >
    <SubscriptionList />
    <EditorDrawer /><DetailDrawer /><ArtifactDrawer /><ShareDialogs /><PublishDialogs />
  </div>
</template>
<style scoped>
.subscriptions-page {
  min-width: 0;
}
</style>
