<script setup lang="ts">
import { UiButton, UiDialog } from "@/ui";
import { useNodes } from "./useNodes";

const {
  probePolicyDialog,
  probePolicyNode,
  probePolicy,
  savingProbePolicy,
  saveProbePolicy,
} = useNodes();

const options = [
  {
    value: "panel",
    label: "只看面板",
    hint: "和以前一样，只由面板服务器连接节点端口判断。",
  },
  {
    value: "any",
    label: "任一视角通过",
    hint: "面板或任意一个自建服务器 agent 连上即有效，适合 IPv6-only 或跨网络节点。",
  },
  {
    value: "all",
    label: "所有视角通过",
    hint: "面板和所有在线 agent 都通过才算有效，最严格。",
  },
  {
    value: "manual",
    label: "手动有效",
    hint: "不自动判无效，节点始终下发；检测结果只展示。",
  },
] as const;
</script>
<template>
  <UiDialog v-model="probePolicyDialog" title="检测策略" width="540px">
    <p class="policy-intro">
      节点「{{ probePolicyNode?.name }}」用哪个视角判断有效性。每台自建服务器 agent
      会自动参与检测，结果可在监控页查看。
    </p>
    <div class="policy-options" role="radiogroup" aria-label="节点检测策略">
      <label
        v-for="option in options"
        :key="option.value"
        class="policy-option"
        :class="{ 'is-active': probePolicy === option.value }"
      >
        <input v-model="probePolicy" type="radio" :value="option.value" />
        <span class="policy-copy">
          <strong>{{ option.label }}</strong>
          <small>{{ option.hint }}</small>
        </span>
      </label>
    </div>
    <template #footer>
      <UiButton @click="probePolicyDialog = false">取消</UiButton>
      <UiButton
        variant="primary"
        :loading="savingProbePolicy"
        @click="saveProbePolicy"
        >保存策略</UiButton
      >
    </template>
  </UiDialog>
</template>
<style scoped>
.policy-intro {
  margin: 0 0 14px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.7;
}
.policy-options {
  display: grid;
  gap: 8px;
}
.policy-option {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
  cursor: pointer;
  transition:
    border-color var(--duration-fast),
    background var(--duration-fast);
}
.policy-option:hover {
  background: var(--c-surface-2);
}
.policy-option.is-active {
  border-color: var(--c-focus);
  background: color-mix(in srgb, var(--c-signal) 7%, var(--c-surface));
}
.policy-option input {
  margin-top: 4px;
  accent-color: var(--c-focus);
}
.policy-copy {
  display: grid;
  gap: 2px;
}
.policy-copy strong {
  font-size: 13.5px;
  font-weight: 500;
}
.policy-copy small {
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
</style>
