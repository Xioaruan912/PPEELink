<script setup lang="ts">
import {
  UiDialog,
  UiButton,
  UiSkeleton,
  UiTag,
  UiTable,
  EmptyState,
  StatusDot,
} from "@/ui";
import { useTemplates } from "./useTemplates";
const {
  preflightVisible,
  preflightLoading,
  preflightError,
  preflightReport,
  preflightDomains,
  preflightCurrent,
  runPreflight,
  openRegression,
  queueTemplateValidation,
  revealIssue,
} = useTemplates();
const severityLabel = (value: string) =>
  value === "error" ? "错误" : value === "warning" ? "警告" : "提示";
const severityTone = (value: string) =>
  value === "error"
    ? ("bad" as const)
    : value === "warning"
      ? ("warn" as const)
      : ("neutral" as const);
const routeLabel = (value: string) =>
  value === "matched" ? "已确认" : value === "partial" ? "需复核" : "未命中";
const routeTone = (value: string) =>
  value === "matched"
    ? ("ok" as const)
    : value === "partial"
      ? ("warn" as const)
      : ("bad" as const);
const compatibilityLabel = (value: string) =>
  value === "pass" ? "通过" : value === "error" ? "阻止发布" : "需确认";
</script>
<template>
  <UiDialog
    v-model="preflightVisible"
    title="发布前预检"
    width="min(1120px, 96vw)"
    top="4vh"
  >
    <div class="query">
      <el-form label-position="top"
        ><el-form-item label="规则解释目标"
          ><el-input
            v-model="preflightDomains"
            type="textarea"
            :rows="3"
            aria-label="预检域名，每行一个"
            placeholder="每行一个域名" /></el-form-item
      ></el-form>
      <p class="muted">只解释模板规则，不会直连目标网站。</p>
      <div class="actions">
        <UiButton @click="openRegression">回归用例</UiButton
        ><UiButton @click="queueTemplateValidation">后台验证</UiButton
        ><UiButton
          variant="primary"
          :loading="preflightLoading"
          @click="runPreflight(true)"
          >重新预检</UiButton
        >
      </div>
    </div>
    <p v-if="preflightError" class="danger-text" role="alert">
      {{ preflightError }}
    </p>
    <UiSkeleton v-if="preflightLoading" :lines="5" height="56px" />
    <template v-else-if="preflightReport">
      <p v-if="!preflightCurrent" class="stale-note" role="status">
        内容或域名已改变，以下为上次报告，请重新预检。
      </p>
      <div class="report-summary">
        <div>
          <StatusDot
            :state="
              !preflightCurrent ? 'idle' : preflightReport.valid ? 'ok' : 'bad'
            "
            :label="
              !preflightCurrent
                ? '报告待更新'
                : preflightReport.valid
                  ? '预检通过'
                  : '发现阻止保存的问题'
            "
          />
          <p class="mono muted">
            {{ String(preflightReport.format || "").toUpperCase() }} /
            {{
              preflightReport.coverage === "full"
                ? "完整语义检查"
                : "基础结构检查"
            }}
          </p>
        </div>
        <dl>
          <dt>错误</dt>
          <dd>{{ preflightReport.summary?.errors || 0 }}</dd>
          <dt>警告</dt>
          <dd>{{ preflightReport.summary?.warnings || 0 }}</dd>
          <dt>策略组</dt>
          <dd>{{ preflightReport.summary?.groups || 0 }}</dd>
          <dt>规则</dt>
          <dd>{{ preflightReport.summary?.rules || 0 }}</dd>
        </dl>
      </div>
      <section class="report-section">
        <header>
          <h3>客户端与测试兼容性</h3>
          <span class="muted mono"
            >协议 {{ preflightReport.protocols?.length || 0 }} 类</span
          >
        </header>
        <div class="compat-list">
          <article
            v-for="item in preflightReport.compatibility"
            :key="item.target"
          >
            <div>
              <strong>{{ item.target }}</strong
              ><UiTag :tone="severityTone(item.status)">{{
                compatibilityLabel(item.status)
              }}</UiTag>
            </div>
            <p class="muted">{{ item.detail }}</p>
          </article>
        </div>
        <div v-if="preflightReport.protocols?.length" class="protocols">
          <UiTag
            v-for="item in preflightReport.protocols"
            :key="`${item.type}-${item.network}`"
            >{{ item.type || "未知"
            }}{{ item.network ? ` / ${item.network}` : "" }} ×
            {{ item.count }}</UiTag
          >
        </div>
        <UiTable
          v-if="preflightReport.capabilityMatrix?.length"
          :rows="preflightReport.capabilityMatrix"
          row-key="client"
          label="客户端能力矩阵"
        >
          <el-table-column
            prop="client"
            label="目标客户端"
            min-width="120"
          /><el-table-column label="结论" min-width="100"
            ><template #default="{ row }"
              ><UiTag :tone="severityTone(row.status)">{{
                compatibilityLabel(row.status)
              }}</UiTag></template
            ></el-table-column
          >
          <el-table-column label="原生支持" min-width="150"
            ><template #default="{ row }">{{
              row.native?.join("、") || "无"
            }}</template></el-table-column
          ><el-table-column label="需要转换" min-width="150"
            ><template #default="{ row }">{{
              row.converted?.join("、") || "无"
            }}</template></el-table-column
          ><el-table-column label="不支持" min-width="150"
            ><template #default="{ row }">{{
              row.unsupported?.join("、") || "无"
            }}</template></el-table-column
          ><el-table-column label="可能丢失字段" min-width="150"
            ><template #default="{ row }">{{
              row.lostFields?.join("、") || "无"
            }}</template></el-table-column
          >
          <template #mobile-card="{ row }"
            ><div class="mobile-title">
              <strong>{{ row.client }}</strong
              ><UiTag :tone="severityTone(row.status)">{{
                compatibilityLabel(row.status)
              }}</UiTag>
            </div>
            <dl class="mobile-facts">
              <dt>原生支持</dt>
              <dd>{{ row.native?.join("、") || "无" }}</dd>
              <dt>需要转换</dt>
              <dd>{{ row.converted?.join("、") || "无" }}</dd>
              <dt>不支持</dt>
              <dd>{{ row.unsupported?.join("、") || "无" }}</dd>
              <dt>可能丢失字段</dt>
              <dd>{{ row.lostFields?.join("、") || "无" }}</dd>
            </dl></template
          >
        </UiTable>
      </section>
      <section class="report-section">
        <header>
          <h3>问题清单</h3>
          <span class="mono muted"
            >{{ preflightReport.issues?.length || 0 }} 项</span
          >
        </header>
        <EmptyState
          v-if="!preflightReport.issues?.length"
          size="section"
          title="未发现结构或引用问题"
        /><button
          v-for="(item, index) in preflightReport.issues"
          :key="`${item.code}-${index}`"
          class="issue-row"
          :disabled="!item.line"
          @click="revealIssue(item.line)"
        >
          <UiTag :tone="severityTone(item.severity)">{{
            severityLabel(item.severity)
          }}</UiTag
          ><span>{{ item.message }}</span
          ><small v-if="item.line">第 {{ item.line }} 行</small>
        </button>
      </section>
      <section class="report-section">
        <header>
          <h3>规则命中解释</h3>
          <span class="muted">按模板顺序首次命中</span>
        </header>
        <UiTable
          :rows="preflightReport.routes || []"
          row-key="domain"
          label="预检规则命中解释"
        >
          <el-table-column label="目标" min-width="160"
            ><template #default="{ row }"
              ><span class="mono">{{ row.domain }}</span></template
            ></el-table-column
          ><el-table-column label="状态" width="100"
            ><template #default="{ row }"
              ><UiTag :tone="routeTone(row.status)">{{
                routeLabel(row.status)
              }}</UiTag></template
            ></el-table-column
          ><el-table-column label="命中规则" min-width="240"
            ><template #default="{ row }"
              ><div class="stack">
                <span class="mono">{{
                  row.matchedRule || "没有可确认的匹配"
                }}</span
                ><small v-if="row.ruleIndex" class="muted mono"
                  >规则 #{{ row.ruleIndex }}</small
                >
              </div></template
            ></el-table-column
          ><el-table-column label="策略链" min-width="240"
            ><template #default="{ row }"
              ><div class="stack">
                <span>{{ row.chain?.join(" → ") || "未提供" }}</span
                ><small v-if="row.candidates?.length" class="muted"
                  >候选：{{ row.candidates.join("、") }}</small
                ><small v-if="row.notes?.length" class="muted">{{
                  row.notes.join("；")
                }}</small>
              </div></template
            ></el-table-column
          >
          <template #mobile-card="{ row }"
            ><div class="mobile-title">
              <strong class="mono">{{ row.domain }}</strong
              ><UiTag :tone="routeTone(row.status)">{{
                routeLabel(row.status)
              }}</UiTag>
            </div>
            <dl class="mobile-facts">
              <dt>命中规则</dt>
              <dd class="mono">
                {{ row.matchedRule || "没有可确认的匹配"
                }}<small v-if="row.ruleIndex"> / #{{ row.ruleIndex }}</small>
              </dd>
              <dt>策略链</dt>
              <dd>{{ row.chain?.join(" → ") || "未提供" }}</dd>
              <template v-if="row.candidates?.length"
                ><dt>候选</dt>
                <dd>{{ row.candidates.join("、") }}</dd></template
              ><template v-if="row.notes?.length"
                ><dt>说明</dt>
                <dd>{{ row.notes.join("；") }}</dd></template
              >
            </dl></template
          >
        </UiTable>
      </section>
    </template>
    <EmptyState
      v-else-if="!preflightLoading"
      size="section"
      title="等待发布前预检"
      description="填写域名后，点击重新预检。"
    />
    <template #footer
      ><UiButton @click="preflightVisible = false">关闭</UiButton></template
    >
  </UiDialog>
</template>
<style scoped>
.query {
  margin-bottom: 20px;
}
.query :deep(.el-form-item) {
  margin-bottom: 8px;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.muted {
  color: var(--c-muted);
}
.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
p,
small {
  font-size: 14px;
}
.danger-text {
  color: var(--c-bad);
}
.stale-note {
  padding: 12px;
  border-left: 2px solid var(--c-warn);
  background: var(--c-surface-2);
}
.report-summary {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 16px 0;
  border-top: 1px solid var(--c-line);
}
.report-summary dl {
  display: grid;
  grid-template-columns: auto auto;
  gap: 8px 16px;
  margin: 0;
}
.report-summary dt {
  color: var(--c-muted);
  font-size: 14px;
}
.report-summary dd {
  text-align: right;
  margin: 0;
  font: 500 16px var(--font-mono);
}
.report-section {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--c-line);
  min-width: 0;
}
.report-section > header,
.mobile-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.report-section h3 {
  font-size: 16px;
  margin: 0;
}
.report-section > header {
  margin-bottom: 16px;
  font-size: 14px;
}
.compat-list article {
  padding: 12px 0;
  border-bottom: 1px solid var(--c-line);
}
.compat-list article > div {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}
.compat-list p {
  margin: 8px 0 0;
}
.protocols {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 16px 0;
}
.issue-row {
  display: flex;
  width: 100%;
  gap: 12px;
  align-items: center;
  padding: 12px 0;
  min-height: 40px;
  border: 0;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  text-align: left;
}
.issue-row:disabled {
  cursor: default;
  opacity: 1;
}
.issue-row span:nth-child(2) {
  flex: 1;
  overflow-wrap: anywhere;
}
.issue-row small {
  white-space: nowrap;
  color: var(--c-data);
}
.stack {
  display: grid;
  gap: 4px;
  overflow-wrap: anywhere;
}
.mobile-facts {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr);
  gap: 10px;
  margin: 12px 0 0;
  font-size: 14px;
}
.mobile-facts dt {
  color: var(--c-muted);
}
.mobile-facts dd {
  margin: 0;
  overflow-wrap: anywhere;
}
@media (max-width: 767px) {
  .report-summary {
    align-items: stretch;
    flex-direction: column;
  }
  .issue-row {
    flex-wrap: wrap;
  }
  .mobile-title {
    align-items: start;
  }
}
</style>
