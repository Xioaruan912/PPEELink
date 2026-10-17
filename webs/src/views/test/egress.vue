<template>
  <div class="test-page egress-page">
    <PageHeader title="出口检测" description="核对节点出口与实际分流路径。"
      ><template #actions
        ><UiButton
          v-if="running && mode === 'local'"
          variant="danger"
          :icon="Close"
          @click="cancelLocal"
          >停止检测</UiButton
        ><UiButton :icon="Grid" :disabled="busy" @click="matrixDrawer = true"
          >质量矩阵</UiButton
        ><UiButton :icon="Setting" :disabled="busy" @click="targetDrawer = true"
          >检测目标</UiButton
        ><UiButton
          variant="primary"
          :icon="VideoPlay"
          :loading="busy"
          :disabled="
            initialLoading ||
            nodesLoading ||
            (mode !== 'local' && !selectedSubscription) ||
            (mode === 'subscription' && !selectedNode) ||
            (mode === 'local' && !results.length)
          "
          @click="runCurrent"
          >{{ busy ? "检测进行中" : "开始检测" }}</UiButton
        ></template
      ></PageHeader
    >
    <div v-if="error" class="test-notice" role="alert">
      <span>{{ error }}</span
      ><UiButton
        v-if="!targets.length || !subscriptions.length"
        variant="ghost"
        :disabled="busy"
        @click="initialize"
        >重新加载</UiButton
      >
    </div>
    <section class="egress-source" aria-label="检测来源" data-enter>
      <UiSegmented
        v-model="mode"
        label="检测模式"
        :options="modeOptions.map((option) => ({ ...option, disabled: busy }))"
      />
      <div
        v-if="mode !== 'local'"
        class="source-selectors"
        :class="{ 'has-node': mode === 'subscription' }"
      >
        <div>
          <label for="egress-subscription" class="field-label">订阅</label
          ><el-select
            id="egress-subscription"
            v-model="selectedSubscription"
            aria-label="检测订阅"
            filterable
            :loading="initialLoading"
            :disabled="busy"
            placeholder="选择订阅"
            @change="loadSubscriptionNodes"
            ><el-option
              v-for="sub in subscriptions"
              :key="sub.ID"
              :label="sub.Name"
              :value="sub.ID"
              ><span class="node-option"
                ><span>{{ sub.Name }}</span
                ><small>{{ sub.Nodes?.length || 0 }} 个固定节点</small></span
              ></el-option
            ></el-select
          >
        </div>
        <div v-if="mode === 'subscription'">
          <label for="egress-node" class="field-label"
            >具体节点 · 默认质量最优</label
          ><el-select
            id="egress-node"
            v-model="selectedNode"
            aria-label="出口检测节点"
            filterable
            :loading="nodesLoading"
            :disabled="busy || nodesLoading"
            placeholder="选择具体节点"
            ><el-option
              v-for="item in subscriptionNodes"
              :key="item.ID"
              :label="item.Name"
              :value="item.ID"
              ><span class="node-option"
                ><span>{{ item.Name }}</span
                ><small
                  >质量 {{ item.quality?.score || 0 }} ·
                  {{
                    item.quality?.averageRtt >= 0
                      ? item.quality.averageRtt + " ms"
                      : "无样本"
                  }}</small
                ></span
              ></el-option
            ></el-select
          >
        </div>
        <p v-else class="source-note">
          {{
            mode === "template"
              ? "读取订阅的 Clash / Surge / Loon 模板，按实际规则自动选择节点。"
              : "读取 Clash / Mihomo 模板，按首条命中语义解释规则。"
          }}
        </p>
      </div>
      <p v-else class="source-note local-note">
        浏览器直连目标网站，检测当前设备的代理分流。此模式不经过服务端节点，最多
        4 项并发。
      </p>
      <UiSkeleton v-if="initialLoading" :lines="2" />
      <EmptyState
        v-else-if="mode !== 'local' && !subscriptions.length"
        size="inline"
        description="暂无订阅。添加订阅后可选择节点，也可使用本机浏览器检测。"
      />
    </section>
    <section
      v-if="mode === 'explain'"
      class="explain-section"
      aria-labelledby="explain-title"
      data-enter
    >
      <header class="section-heading">
        <div>
          <h2 id="explain-title">规则路径</h2>
          <p>输入请求上下文，逐条查看命中原因。</p>
        </div>
        <UiButton
          :loading="explainLoading"
          :disabled="!selectedSubscription || nodesLoading"
          @click="runCurrent"
          >开始解释</UiButton
        >
      </header>
      <div class="explain-query">
        <label
          >目标域名<el-input
            v-model="explainTarget"
            :disabled="busy"
            placeholder="例如 gemini.google.com" /></label
        ><label
          >目标 IP · 可选<el-input
            v-model="explainIP"
            :disabled="busy"
            placeholder="IPv4 或 IPv6" /></label
        ><label
          >端口<el-input-number
            v-model="explainPort"
            :disabled="busy"
            :min="1"
            :max="65535" /></label
        ><label
          >协议<el-select
            v-model="explainProtocol"
            :disabled="busy"
            aria-label="解释协议"
            ><el-option label="TCP" value="tcp" /><el-option
              label="UDP"
              value="udp" /></el-select
        ></label>
      </div>
      <UiSkeleton v-if="explainLoading" :lines="4" /><RuleResults
        v-else-if="explainResult"
        kind="explain"
        :result="explainResult"
      /><EmptyState
        v-else
        size="section"
        title="等待规则解释"
        description="提供域名、IP 或端口后开始解释。配置不会被修改。"
      />
    </section>
    <template v-else>
      <UiSkeleton v-if="planLoading" :lines="4" /><RuleResults
        v-if="mode === 'template' && plan"
        kind="template"
        :result="plan"
      />
      <section
        class="egress-summary"
        aria-labelledby="summary-title"
        data-enter
      >
        <header class="section-heading">
          <div>
            <h2 id="summary-title">
              出口汇总
              <span class="section-count"
                >{{ uniqueEgress.length }} 个出口</span
              >
            </h2>
            <p v-if="lastTestedAt">更新于 {{ formatDateTime(lastTestedAt) }}</p>
          </div>
          <el-switch v-model="hideIP" active-text="隐藏 IP" />
        </header>
        <TransitionGroup
          name="egress-card"
          tag="div"
          class="egress-groups"
          :css="!reduced"
          @enter="enter"
          @leave="leave"
          ><article
            v-for="(item, index) in uniqueEgress"
            :key="`${item.ip}|${item.countryCode}`"
            class="egress-group"
            :data-index="index"
          >
            <CountryFlag :code="item.countryCode" />
            <div>
              <strong class="mono">{{ maskIP(item.ip) }}</strong>
              <p>
                {{ item.countryCode || "未知地区" }} ·
                {{ item.services.length }} 个目标
              </p>
              <span>{{ item.services.join("、") }}</span>
            </div>
          </article></TransitionGroup
        >
        <EmptyState
          v-if="!uniqueEgress.length"
          size="section"
          :title="busy ? '正在收集出口' : '尚无出口结果'"
          description="开始检测后，按实际出口 IP 与国家汇总。"
        />
      </section>
      <section
        class="egress-results"
        aria-labelledby="egress-results-title"
        data-enter
      >
        <header class="section-heading">
          <div>
            <h2 id="egress-results-title">目标明细</h2>
            <p>
              {{
                mode === "template"
                  ? "按订阅模板选择节点并验证"
                  : mode === "subscription"
                    ? "通过所选节点检测目标"
                    : "由当前浏览器直接访问目标"
              }}
            </p>
          </div>
          <StatusDot
            :state="busy ? 'warn' : 'idle'"
            :label="
              busy
                ? `${results.filter((item) => item.status === 'done' || item.status === 'error').length} / ${results.length} 已返回`
                : '失败不等于网站不可用'
            "
          />
        </header>
        <UiTable :rows="results" row-key="key" label="出口检测明细"
          ><el-table-column label="目标" min-width="200"
            ><template #default="{ row }"
              ><div class="target-title">
                <span class="target-mark" aria-hidden="true">{{
                  row.icon || row.name.slice(0, 1)
                }}</span>
                <div>
                  <strong>{{ row.name }}</strong
                  ><small>{{ row.domain }}</small>
                </div>
              </div></template
            ></el-table-column
          ><el-table-column label="类型" width="90"
            ><template #default="{ row }">{{
              groupLabels[row.group] || row.group
            }}</template></el-table-column
          ><el-table-column label="地区" width="100"
            ><template #default="{ row }"
              ><CountryFlag
                v-if="row.countryCode"
                :code="row.countryCode"
              /><span v-else class="muted">未知</span></template
            ></el-table-column
          ><el-table-column label="出口 IP" min-width="180"
            ><template #default="{ row }"
              ><span class="mono">{{ maskIP(row.ip) }}</span></template
            ></el-table-column
          ><el-table-column label="响应" width="110"
            ><template #default="{ row }"
              ><span class="mono">{{
                row.rtt >= 0 ? `${row.rtt} ms` : "未获取"
              }}</span></template
            ></el-table-column
          ><el-table-column label="状态" min-width="200"
            ><template #default="{ row }"
              ><StatusDot
                :state="statusState(row)"
                :label="statusText(row)"
              /><small v-if="row.note" class="status-note">{{
                row.note
              }}</small></template
            ></el-table-column
          >
          <template #mobile-card="{ row }"
            ><div class="mobile-result-heading">
              <div class="target-title">
                <span class="target-mark" aria-hidden="true">{{
                  row.icon || row.name.slice(0, 1)
                }}</span>
                <div>
                  <strong>{{ row.name }}</strong
                  ><small>{{ row.domain }}</small>
                </div>
              </div>
              <CountryFlag v-if="row.countryCode" :code="row.countryCode" />
            </div>
            <dl class="mobile-result">
              <dt>类型</dt>
              <dd>{{ groupLabels[row.group] || row.group }}</dd>
              <dt>出口 IP</dt>
              <dd class="mono">{{ maskIP(row.ip) }}</dd>
              <dt>响应</dt>
              <dd class="mono">
                {{ row.rtt >= 0 ? `${row.rtt} ms` : "未获取" }}
              </dd>
              <dt>状态</dt>
              <dd>
                <StatusDot
                  :state="statusState(row)"
                  :label="statusText(row)"
                /><small v-if="row.note" class="status-note">{{
                  row.note
                }}</small>
              </dd>
            </dl></template
          >
          <template #empty
            ><EmptyState
              size="section"
              title="没有启用的目标"
              description="在检测目标中添加并启用目标。"
              ><template #action
                ><UiButton @click="targetDrawer = true"
                  >管理检测目标</UiButton
                ></template
              ></EmptyState
            ></template
          >
        </UiTable>
        <p class="privacy-note">
          出口 IP 仅在当前页面展示，服务端不持久化出口
          IP。本机模式会由浏览器直接请求目标；目标未开放 CORS
          时可能无法检测。隐藏 IP 可遮盖页面读数。
        </p>
      </section>
    </template>
    <TargetDrawer
      v-model="targetDrawer"
      :targets="targets"
      @changed="refreshTargets"
    /><QualityMatrixDrawer v-model="matrixDrawer" />
  </div>
</template>
<script setup lang="ts">
import CountryFlag from "@/ui/CountryFlag.vue";
import { ref } from "vue";
import { Close, Grid, Setting, VideoPlay } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiSegmented,
  UiSkeleton,
  UiTable,
  EmptyState,
  StatusDot,
} from "@/ui";
import { formatDateTime } from "@/utils/date";
import { usePrefersReducedMotion } from "@/motion";
import { useResultMotion } from "@/components/node-tests/useNodeTestSession";
import { useEgressTest, groupLabels } from "./egress/useEgressTest";
import TargetDrawer from "./egress/TargetDrawer.vue";
import QualityMatrixDrawer from "./egress/QualityMatrixDrawer.vue";
import RuleResults from "./egress/RuleResults.vue";
defineOptions({ name: "EgressTest" });
const {
  targets,
  results,
  mode,
  subscriptions,
  selectedSubscription,
  subscriptionNodes,
  selectedNode,
  nodesLoading,
  initialLoading,
  running,
  busy,
  planLoading,
  explainLoading,
  lastTestedAt,
  hideIP,
  error,
  plan,
  explainResult,
  explainTarget,
  explainIP,
  explainPort,
  explainProtocol,
  uniqueEgress,
  maskIP,
  loadTargets,
  loadSubscriptionNodes,
  runCurrent,
  cancelLocal,
  initialize,
} = useEgressTest();
const targetDrawer = ref(false),
  matrixDrawer = ref(false);
const reduced = usePrefersReducedMotion();
const { enter, leave } = useResultMotion();
const modeOptions = [
  { label: "订阅节点", value: "subscription" },
  { label: "模板验证", value: "template" },
  { label: "规则解释", value: "explain" },
  { label: "本机浏览器", value: "local" },
];
const statusState = (row: Record<string, any>) =>
  row.status === "done"
    ? "ok"
    : row.status === "error"
      ? "bad"
      : row.status === "testing"
        ? "warn"
        : "idle";
const statusText = (row: Record<string, any>) =>
  row.status === "done"
    ? row.ip
      ? "已获取出口"
      : "目标可达"
    : row.status === "testing"
      ? "检测中"
      : row.status === "error"
        ? "检测失败"
        : "等待检测";
async function refreshTargets() {
  try {
    await loadTargets();
  } catch {
    error.value = "检测目标刷新失败，请重试。";
  }
}
</script>
<style lang="scss" scoped>
@use "./test-page.scss";
.egress-source {
  margin-bottom: 48px;
  min-width: 0;
}
.source-selectors {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
  gap: 32px;
  align-items: end;
  margin-top: 32px;
}
.source-selectors > div {
  min-width: 0;
}
.source-selectors.has-node {
  grid-template-columns: minmax(0, 5fr) minmax(0, 7fr);
}
.source-note {
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.7;
  margin: 0;
  padding: 8px 0;
}
.local-note {
  margin-top: 24px;
  max-width: 720px;
}
.egress-summary {
  margin: 48px 0 56px;
  border-top: 1px solid var(--c-line);
  padding-top: 32px;
}
.egress-groups {
  position: relative;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}
.egress-group {
  display: flex;
  align-items: start;
  gap: 16px;
  padding: 24px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  min-width: 0;
}
.egress-group > div {
  min-width: 0;
}
.egress-group strong {
  font-size: 20px;
  font-weight: 400;
  overflow-wrap: anywhere;
}
.egress-group p {
  color: var(--c-muted);
  font-size: 14px;
  margin: 12px 0 8px;
}
.egress-group div > span {
  color: var(--c-muted);
  font-size: 13px;
  overflow-wrap: anywhere;
}
.egress-results {
  min-width: 0;
}
.target-title {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}
.target-title > div {
  min-width: 0;
}
.target-title strong {
  font-size: 15px;
  font-weight: 500;
}
.target-title small {
  display: block;
  color: var(--c-muted);
  font: 400 13px/1.5 var(--font-mono);
  margin-top: 8px;
  overflow-wrap: anywhere;
}
.target-mark {
  width: 36px;
  min-width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  background: var(--c-surface-2);
  border-radius: 10px;
  font-size: 16px;
  overflow: hidden;
}
.muted {
  color: var(--c-muted);
  font-size: 14px;
}
.status-note {
  display: block;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
  margin-top: 8px;
}
.privacy-note {
  border-top: 1px solid var(--c-line);
  padding-top: 24px;
  margin: 32px 0 0;
  font-size: 14px;
  line-height: 1.7;
  color: var(--c-muted);
  max-width: 880px;
}
.explain-query {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(0, 1.5fr) minmax(0, 1fr) minmax(
      0,
      1fr
    );
  gap: 20px;
}
.explain-query label {
  display: flex;
  flex-direction: column;
  gap: 12px;
  color: var(--c-muted);
  font-size: 14px;
  min-width: 0;
}
.explain-query .el-input-number {
  width: 100%;
}
.mobile-result-heading {
  display: flex;
  justify-content: space-between;
  align-items: start;
  gap: 12px;
}
.mobile-result {
  display: grid;
  grid-template-columns: 80px minmax(0, 1fr);
  gap: 16px;
  margin: 24px 0 0;
}
.mobile-result dt {
  color: var(--c-muted);
  font-size: 14px;
}
.mobile-result dd {
  margin: 0;
  font-size: 14px;
  overflow-wrap: anywhere;
}
.egress-card-move {
  transition: transform var(--duration-slow) var(--ease-spring);
}
.egress-card-leave-active {
  position: absolute;
}
@media (max-width: 1023px) {
  .source-selectors,
  .source-selectors.has-node {
    gap: 24px;
  }
  .explain-query {
    grid-template-columns: 1fr 1fr;
  }
}
@media (max-width: 767px) {
  .source-selectors,
  .source-selectors.has-node {
    grid-template-columns: 1fr;
    gap: 24px;
  }
  .egress-source :deep(.ui-segmented) {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }
  .egress-source :deep(.ui-segmented__slider) {
    display: none;
  }
  .egress-source :deep(.ui-segmented button[aria-checked="true"]) {
    background: var(--c-surface);
  }
  .egress-groups {
    grid-template-columns: 1fr;
  }
  .egress-group {
    padding: 20px;
  }
  .egress-summary {
    margin: 40px 0;
  }
  .explain-query {
    grid-template-columns: 1fr;
  }
}
@media (prefers-reduced-motion: reduce) {
  .egress-card-move {
    transition: none;
  }
}
:global(html[data-motion="reduced"]) .egress-card-move {
  transition: none;
}
</style>
