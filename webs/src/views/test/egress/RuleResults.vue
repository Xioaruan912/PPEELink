<template>
  <section
    v-if="kind === 'explain' && result"
    class="rule-results"
    aria-label="规则解释结果"
    data-enter
  >
    <div class="rule-flow">
      <span>{{ result.target || result.ip || "请求" }}</span
      ><b aria-hidden="true">→</b
      ><span>{{ result.matchedRule || "未命中" }}</span
      ><template v-for="(item, index) in result.chain || []" :key="index"
        ><b aria-hidden="true">→</b><span>{{ item }}</span></template
      ><template v-if="result.selectedNode"
        ><b aria-hidden="true">→</b
        ><span class="selected-node">{{
          result.selectedNode.name
        }}</span></template
      >
    </div>
    <dl class="rule-meta">
      <div>
        <dt>模板</dt>
        <dd>{{ result.template || "未提供" }}</dd>
      </div>
      <div>
        <dt>命中位置</dt>
        <dd>第 {{ result.ruleIndex || "未知" }} 条</dd>
      </div>
      <div>
        <dt>候选节点</dt>
        <dd>{{ result.candidateCount || 0 }}</dd>
      </div>
      <div>
        <dt>策略</dt>
        <dd>{{ result.policy || "未提供" }}</dd>
      </div>
      <div>
        <dt>期望地区</dt>
        <dd>{{ result.expectedCountry || "不限" }}</dd>
      </div>
      <div>
        <dt>实际节点</dt>
        <dd>{{ result.selectedNode?.name || "未选择" }}</dd>
      </div>
    </dl>
    <header class="rule-heading">
      <h3>命中前的规则</h3>
      <span>共评估 {{ result.evaluatedCount || 0 }} 条</span>
    </header>
    <UiTable :rows="result.previous || []" row-key="index" label="未命中规则"
      ><el-table-column prop="index" label="序号" width="72" /><el-table-column
        prop="rule"
        label="规则"
        min-width="260" /><el-table-column
        prop="reason"
        label="未命中原因"
        min-width="200" /><el-table-column
        prop="source"
        label="来源"
        min-width="140" /><template #mobile-card="{ row }"
        ><p class="rule-code">{{ row.index }} · {{ row.rule }}</p>
        <p>{{ row.reason }}</p>
        <small class="rule-source"
          >来源：{{ row.source || "未提供" }}</small
        ></template
      ><template #empty
        ><EmptyState
          size="inline"
          description="没有需要列出的前置规则。" /></template
    ></UiTable>
    <p v-if="result.warnings?.length" class="rule-warning" role="status">
      {{ result.warnings.join("；") }}
    </p>
  </section>
  <section
    v-else-if="kind === 'template' && result"
    class="rule-results"
    aria-label="模板验证结果"
    data-enter
  >
    <header class="rule-heading">
      <div>
        <h2>模板分流</h2>
        <p>{{ result.template || "未读取到本地模板" }}</p>
      </div>
      <span>{{ result.items?.length || 0 }} 个目标</span>
    </header>
    <UiTable :rows="result.items || []" row-key="domain" label="模板规则验证"
      ><el-table-column
        prop="name"
        label="网站"
        min-width="140" /><el-table-column label="命中规则" min-width="220"
        ><template #default="{ row }"
          ><span class="rule-code">{{
            row.matchedRule || "默认策略"
          }}</span></template
        ></el-table-column
      ><el-table-column label="期望地区" width="100"
        ><template #default="{ row }">{{
          row.expectedCountry || "不限"
        }}</template></el-table-column
      ><el-table-column label="实际节点" min-width="220"
        ><template #default="{ row }"
          ><div class="plan-node">
            {{ row.selectedNode?.name || "无可用节点"
            }}<small v-if="row.selectedNode"
              >{{ row.selectedNode.countryCode || "未知地区" }} · 质量
              {{ row.selectedNode.score || 0 }}</small
            >
          </div></template
        ></el-table-column
      ><el-table-column label="结果" width="110"
        ><template #default="{ row }"
          ><span :class="succeeded(row) ? 'plan-ok' : 'plan-bad'">{{
            succeeded(row) ? "分流成功" : "失败"
          }}</span></template
        ></el-table-column
      ><template #mobile-card="{ row }"
        ><div class="plan-mobile-head">
          <strong>{{ row.name }}</strong
          ><span :class="succeeded(row) ? 'plan-ok' : 'plan-bad'">{{
            succeeded(row) ? "分流成功" : "失败"
          }}</span>
        </div>
        <dl class="plan-mobile">
          <dt>命中规则</dt>
          <dd class="rule-code">{{ row.matchedRule || "默认策略" }}</dd>
          <dt>期望地区</dt>
          <dd>{{ row.expectedCountry || "不限" }}</dd>
          <dt>实际节点</dt>
          <dd>
            {{ row.selectedNode?.name || "无可用节点"
            }}<small v-if="row.selectedNode"
              >{{ row.selectedNode.countryCode || "未知地区" }} · 质量
              {{ row.selectedNode.score || 0 }}</small
            >
          </dd>
        </dl></template
      ><template #empty
        ><EmptyState
          size="section"
          title="暂无模板目标"
          description="请检查订阅绑定的模板与启用目标。" /></template
    ></UiTable>
    <p v-if="result.warnings?.length" class="rule-warning" role="status">
      {{ result.warnings.join("；") }}
    </p>
  </section>
</template>
<script setup lang="ts">
import { UiTable, EmptyState } from "@/ui";
defineProps<{ kind: "explain" | "template"; result: any }>();
const succeeded = (row: any) =>
  row.selectedNode?.id > 0 &&
  ["available", "reachable"].includes(row.result?.status);
</script>
<style scoped>
.rule-results {
  margin-top: 32px;
  min-width: 0;
}
.rule-flow {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  margin-bottom: 32px;
  overflow-wrap: anywhere;
  background: var(--c-surface);
}
.rule-flow span {
  font-size: 15px;
  min-width: 0;
  overflow-wrap: anywhere;
}
.rule-flow b {
  font-weight: 400;
  color: var(--c-muted);
}
.rule-flow .selected-node {
  color: var(--c-data);
}
.rule-meta {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 24px;
  margin: 0 0 40px;
}
.rule-meta dt {
  font-size: 13px;
  color: var(--c-muted);
  margin-bottom: 8px;
}
.rule-meta dd {
  font-size: 15px;
  margin: 0;
  overflow-wrap: anywhere;
}
.rule-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin: 24px 0;
}
.rule-heading h2 {
  font-size: 22px;
  margin: 0;
}
.rule-heading h3 {
  font-size: 18px;
  margin: 0;
}
.rule-heading p,
.rule-heading > span {
  font-size: 14px;
  color: var(--c-muted);
  margin: 8px 0 0;
}
.rule-code {
  font: 400 14px/1.6 var(--font-mono);
  overflow-wrap: anywhere;
}
.rule-warning {
  padding: 16px 0;
  color: var(--c-warn-ink);
  font-size: 14px;
  line-height: 1.6;
}
.rule-source,
.plan-node small,
.plan-mobile small {
  display: block;
  font-size: 13px;
  color: var(--c-muted);
  margin-top: 8px;
}
.plan-ok {
  color: var(--c-ok);
}
.plan-bad {
  color: var(--c-bad);
}
.plan-mobile-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  font-size: 15px;
}
.plan-mobile {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr);
  gap: 16px;
  margin: 24px 0 0;
}
.plan-mobile dt {
  font-size: 14px;
  color: var(--c-muted);
}
.plan-mobile dd {
  font-size: 14px;
  margin: 0;
}
@media (max-width: 767px) {
  .rule-meta {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .rule-flow {
    padding: 16px;
  }
  .rule-heading {
    align-items: start;
    flex-direction: column;
  }
}
</style>
