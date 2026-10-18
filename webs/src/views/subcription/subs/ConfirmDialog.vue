<script setup lang="ts">
import { UiButton, UiDialog } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
const { confirmation, resolveConfirmation } = useSubscriptions();
</script>
<template>
  <UiDialog
    v-model="confirmation.visible"
    :title="confirmation.title"
    width="480px"
    ><p>{{ confirmation.message }}</p>
    <template #footer
      ><UiButton @click="resolveConfirmation(false)">取消</UiButton
      ><UiButton variant="danger" @click="resolveConfirmation(true)">{{
        confirmation.action
      }}</UiButton></template
    ></UiDialog
  >
</template>
