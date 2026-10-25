<script setup lang="ts">
import { UiButton } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
const { appRules, NodesList } = useSubscriptions();
function move(index: number, direction: number) {
  const next = index + direction;
  if (next < 0 || next >= appRules.value.length) return;
  const [rule] = appRules.value.splice(index, 1);
  appRules.value.splice(next, 0, rule);
}
</script>
<template>
  <section class="app-rules">
    <h3>按应用代理</h3>
    <p>
      规则从上到下匹配，优先于网站规则。仅随 Mihomo / sing-box 1.11+
      完整订阅下发，需客户端具备进程识别或 Android 应用识别能力，通常配合 TUN
      使用。iPhone 不支持这里的应用匹配。
    </p>
    <p>
      目标节点必须包含在本订阅中；节点重命名、移除后需同步修改规则。应用规则不会通过普通节点链接下发。
    </p>
    <div v-for="(rule, index) in appRules" :key="index" class="app-rule">
      <el-form-item :label="`规则 ${index + 1} · 平台`"
        ><el-select v-model="rule.platform"
          ><el-option label="桌面进程" value="desktop" /><el-option
            label="Android 应用"
            value="android" /></el-select
      ></el-form-item>
      <el-form-item label="应用标识"
        ><el-input
          v-model="rule.app"
          :placeholder="
            rule.platform === 'desktop'
              ? 'chrome.exe'
              : 'org.telegram.messenger'
          "
      /></el-form-item>
      <el-form-item label="连接方式"
        ><el-select v-model="rule.target" filterable
          ><el-option label="代理（客户端选择节点）" value="PROXY" /><el-option
            label="直连"
            value="DIRECT" /><el-option label="阻断" value="REJECT" /><el-option
            v-for="node in NodesList"
            :key="node.ID"
            :label="node.Name"
            :value="node.Name" /></el-select
      ></el-form-item>
      <div class="rule-actions">
        <UiButton :disabled="index === 0" @click="move(index, -1)"
          >上移</UiButton
        ><UiButton
          :disabled="index === appRules.length - 1"
          @click="move(index, 1)"
          >下移</UiButton
        ><UiButton @click="appRules.splice(index, 1)">删除</UiButton>
      </div>
    </div>
    <UiButton
      :disabled="appRules.length >= 64"
      @click="appRules.push({ platform: 'desktop', app: '', target: 'PROXY' })"
      >添加应用规则</UiButton
    >
  </section>
</template>
<style scoped>
.app-rules p {
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.7;
}
.app-rule {
  display: grid;
  grid-template-columns: 150px 1fr 1fr;
  gap: 12px;
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
}
.rule-actions {
  grid-column: 1/-1;
  display: flex;
  gap: 8px;
}
.app-rule :deep(.el-select) {
  width: 100%;
}
@media (max-width: 640px) {
  .app-rule {
    grid-template-columns: 1fr;
    gap: 4px;
  }
  .rule-actions :deep(button) {
    min-height: 44px;
  }
}
</style>
