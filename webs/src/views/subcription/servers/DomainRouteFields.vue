<script setup lang="ts">
import { computed } from "vue";
import { UiButton } from "@/ui";
import type { DomainRoute } from "@/api/proxy";
const rules = defineModel<DomainRoute[]>({ required: true });
const props = defineProps<{
  exits: { id: number; name: string }[];
  defaultExit: number;
}>();
const fallback = computed(() =>
  props.defaultExit
    ? props.exits.find((x) => x.id === props.defaultExit)?.name ||
      "已选出口（请确认节点仍可用）"
    : "当前服务器直接访问"
);
function move(index: number, direction: number) {
  const next = index + direction;
  if (next < 0 || next >= rules.value.length) return;
  const copy = [...rules.value];
  [copy[index], copy[next]] = [copy[next], copy[index]];
  rules.value = copy;
}
function add() {
  rules.value = [
    ...rules.value,
    { domain: "", match: "suffix", egressNodeId: props.defaultExit },
  ];
}
</script>
<template>
  <section class="domain-routes" aria-labelledby="domain-routes-title">
    <div class="routes-heading">
      <h3 id="domain-routes-title">按网站分流</h3>
      <span>{{ rules.length }} / 32</span>
    </div>
    <p class="routes-help">
      规则从上往下匹配，命中第一条后使用指定出口。出口不可用时停止该路线，不改为直连。
    </p>
    <div v-for="(rule, index) in rules" :key="index" class="domain-rule">
      <div class="rule-heading">
        <strong>规则 {{ index + 1 }}</strong>
        <div class="rule-actions">
          <UiButton
            :aria-label="`上移规则 ${index + 1}`"
            :disabled="index === 0"
            @click="move(index, -1)"
            >上移</UiButton
          >
          <UiButton
            :aria-label="`下移规则 ${index + 1}`"
            :disabled="index === rules.length - 1"
            @click="move(index, 1)"
            >下移</UiButton
          >
          <UiButton
            :aria-label="`删除规则 ${index + 1}`"
            @click="rules = rules.filter((_, i) => i !== index)"
            >删除</UiButton
          >
        </div>
      </div>
      <el-form-item label="网站域名"
        ><el-input
          v-model="rule.domain"
          placeholder="example.com，不含 https:// 或路径"
          maxlength="253"
      /></el-form-item>
      <div class="rule-fields">
        <el-form-item label="匹配范围"
          ><el-select v-model="rule.match"
            ><el-option value="suffix" label="此域名及所有子域名" /><el-option
              value="exact"
              label="仅此域名" /></el-select
        ></el-form-item>
        <el-form-item label="使用出口"
          ><el-select v-model="rule.egressNodeId" filterable
            ><el-option :value="0" label="当前服务器直接访问" /><el-option
              v-if="
                rule.egressNodeId &&
                !exits.some((x) => x.id === rule.egressNodeId)
              "
              :value="rule.egressNodeId"
              label="出口不可用，请重新选择"
              disabled /><el-option
              v-for="item in exits"
              :key="item.id"
              :value="item.id"
              :label="item.name" /></el-select
        ></el-form-item>
      </div>
    </div>
    <UiButton :disabled="rules.length >= 32" @click="add"
      >添加网站规则</UiButton
    >
    <p class="route-fallback">
      未匹配的网站 <strong>{{ fallback }}</strong>
    </p>
    <p class="routes-help">
      在服务器上生效，无需修改客户端规则。仅能按连接中可识别的域名分流；纯 IP
      或无法识别域名的连接沿用默认出口。需升级 Agent。
    </p>
  </section>
</template>
<style scoped>
.domain-routes {
  margin: 8px 0 22px;
  border-top: 1px solid var(--c-line);
  padding-top: 18px;
}
.routes-heading,
.rule-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 10px;
}
h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 650;
}
.routes-heading > span,
.routes-help {
  font-size: 12px;
  color: var(--c-muted);
  line-height: 1.65;
}
.routes-help {
  margin: 8px 0 14px;
}
.domain-rule {
  padding: 14px 0 4px;
  border-top: 1px solid var(--c-line);
}
.rule-heading {
  margin-bottom: 12px;
  font-size: 13px;
}
.rule-actions {
  display: flex;
  gap: 4px;
}
.rule-fields {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 12px;
}
.domain-rule :deep(.el-select) {
  width: 100%;
}
.route-fallback {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 12px;
  font-size: 12px;
  color: var(--c-muted);
  padding: 12px 0;
  margin: 10px 0 0;
}
.route-fallback strong {
  color: var(--c-ink);
  overflow-wrap: anywhere;
}
@media (max-width: 540px) {
  .rule-fields {
    grid-template-columns: minmax(0, 1fr);
    gap: 0;
  }
  .rule-actions :deep(button) {
    min-height: 44px;
    min-width: 44px;
  }
}
</style>
