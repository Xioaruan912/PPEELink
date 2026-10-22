<script setup lang="ts">
import { CopyDocument, Edit, MoreFilled } from "@element-plus/icons-vue";
import { StatusDot } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import CountryFlag from "@/ui/CountryFlag.vue";
import { formatDateTime } from "@/utils/date";
import {
  useNodes,
  validState,
  validText,
  nodeEffectiveHidden,
  type OverviewItem,
} from "./useNodes";
defineProps<{ node: OverviewItem }>();
// 暂不下发标签：原因的简称 + 预计恢复日期
const probeTitle = (node: OverviewItem) =>
  (node.probes || [])
    .map(
      (probe) =>
        `${probe.region || probe.name}: ${
          probe.success ? `通 ${probe.rtt}ms` : probe.error || "失败"
        }`
    )
    .join("\n");
const policyLabel = (policy?: string) =>
  policy === "any"
    ? "任一 Agent"
    : policy === "all"
      ? "全部 Agent"
      : policy === "manual"
        ? "手动有效"
        : "";
const blockLabel = (node: OverviewItem) => {
  const b = node.blocked!;
  const short =
    b.kind === "paused"
      ? "已暂停"
      : b.kind === "server-quota"
        ? "服务器流量用完"
        : "连续检测无效";
  return b.until ? `${short}，${b.until.slice(5, 10)} 恢复` : short;
};
const {
  selectedIds,
  toggleSelect,
  nodeCommand,
  copyNode,
  fullNode,
  protocol,
  busy,
  setRowVisible,
  activeGroup,
  nodeName,
  checking,
} = useNodes();
</script>
<template>
  <article
    class="node-row"
    :class="{ 'is-selected': selectedIds.has(node.id) }"
  >
    <div class="node-identity">
      <el-checkbox
        :model-value="selectedIds.has(node.id)"
        :aria-label="`选择 ${node.name}`"
        @change="toggleSelect(node.id)"
      />
      <CountryFlag :code="node.countryCode" :label="node.country" />
      <div class="node-name">
        <button :title="node.name" @click="nodeCommand('edit', node)">
          {{ node.name }}
        </button>
        <span class="host mono" :title="node.server">{{
          node.server || "主机未知"
        }}</span>
        <span class="membership" :title="(node.groups || []).join(' / ')"
          ><span
            v-if="node.blocked"
            class="block-badge"
            :class="`block-${node.blocked.kind}`"
            :title="node.blocked.reason"
            >暂不下发 · {{ blockLabel(node) }}</span
          ><span v-if="node.relayId" class="relay-badge"
            >经 {{ nodeName(node.relayId) || "已删除节点" }} 中转</span
          ><span
            v-if="node.forceDeliver"
            class="force-badge"
            title="检测无效时也不会从订阅中排除"
            >始终下发</span
          ><span
            v-if="node.probes?.length"
            class="probe-badge"
            :class="{ 'is-bad': !node.probes.some((probe) => probe.success) }"
            :title="probeTitle(node)"
            >检测 {{ node.probes.filter((probe) => probe.success).length }}/{{
              node.probes.length
            }}</span
          ><span
            v-if="node.probePolicy && node.probePolicy !== 'panel'"
            class="policy-badge"
            :title="`检测策略：${policyLabel(node.probePolicy)}`"
            >{{ policyLabel(node.probePolicy) }}</span
          >{{ node.country || "国家未知" }} ·
          {{ (node.groups || []).join(" / ") || "未分组" }}</span
        >
      </div>
    </div>
    <div class="row-protocol mono">
      <span class="mobile-label">协议</span>{{ protocol(node) }}
    </div>
    <div class="latency" :class="`latency--${validState(node)}`">
      <span class="mobile-label">状态</span
      ><StatusDot
        :state="validState(node)"
        :running="checking"
        :title="
          node.checkedAt
            ? `检测于 ${formatDateTime(node.checkedAt)}`
            : undefined
        "
        ><span>{{ validText(node) }}</span></StatusDot
      >
    </div>
    <div class="row-visibility">
      <span class="mobile-label">可见</span
      ><el-tooltip
        :trigger="['hover', 'focus']"
        :trigger-keys="[]"
        :content="
          nodeEffectiveHidden(fullNode(node.id))
            ? '节点已全局隐藏，分组管理中可恢复'
            : '关闭后全局隐藏节点'
        "
      >
        <el-switch
          :model-value="!nodeEffectiveHidden(fullNode(node.id))"
          :disabled="
            busy || !!fullNode(node.id)?.GroupNodes?.some((g) => g.Hidden)
          "
          :aria-label="`${node.name} 可见性`"
          @change="setRowVisible(node)"
        />
      </el-tooltip>
    </div>
    <div class="row-actions">
      <RowIconButton
        :icon="Edit"
        :label="`编辑 ${node.name}`"
        @click="nodeCommand('edit', node)"
      />
      <RowIconButton
        :icon="CopyDocument"
        :label="`复制 ${node.name} 链接`"
        @click="copyNode(node)"
      />
      <el-dropdown
        trigger="click"
        @command="(command: string) => nodeCommand(command, node)"
      >
        <RowIconButton :icon="MoreFilled" :label="`${node.name} 更多操作`" />
        <template #dropdown
          ><el-dropdown-menu>
            <el-dropdown-item command="tcp">国内可达性</el-dropdown-item>
            <el-dropdown-item command="unlock">解锁检测</el-dropdown-item>
            <el-dropdown-item command="chain" divided
              >设置中转</el-dropdown-item
            >
            <el-dropdown-item command="temp">临时订阅</el-dropdown-item>
            <el-dropdown-item command="probe-policy"
              >检测策略…</el-dropdown-item
            >
            <el-dropdown-item command="deliver">{{
              node.forceDeliver
                ? "恢复按检测结果下发"
                : "始终下发（忽略检测）"
            }}</el-dropdown-item>
            <el-dropdown-item
              v-if="node.blocked?.kind === 'paused'"
              command="resume"
              divided
              >恢复下发</el-dropdown-item
            >
            <el-dropdown-item v-else command="pause" divided
              >暂停下发…</el-dropdown-item
            >
            <el-dropdown-item command="group" divided
              >加入分组</el-dropdown-item
            >
            <el-dropdown-item v-if="activeGroup !== '全部'" command="unbind"
              >移出当前分组</el-dropdown-item
            >
            <el-dropdown-item command="delete" divided :disabled="busy"
              ><span class="danger">删除节点</span></el-dropdown-item
            >
          </el-dropdown-menu></template
        >
      </el-dropdown>
    </div>
  </article>
</template>
<style scoped>
.node-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 78px 112px 52px 132px;
  align-items: center;
  gap: 10px;
  padding: 10px 0;
  min-height: 68px;
  border-bottom: 1px solid var(--c-line);
  transition: background var(--duration-fast);
}
.node-row:hover {
  background: var(--c-surface-2);
}
.is-selected {
  background: var(--c-selected);
}
.node-identity {
  display: flex;
  align-items: center;
  min-width: 0;
  gap: 8px;
}
.node-identity :deep(.el-checkbox) {
  margin: 0;
  flex-shrink: 0;
}
.node-name {
  display: flex;
  flex-direction: column;
  min-width: 0;
  gap: 0;
}
.node-name button {
  text-align: left;
  font-size: 14px;
  font-weight: 600;
}
.node-name button,
.host,
.membership {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.host,
.membership {
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.5;
}
.block-badge {
  display: inline-block;
  margin-right: 6px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--c-warn) 16%, transparent);
  color: var(--c-warn-ink);
  font-size: 12px;
}
.block-badge.block-health {
  background: color-mix(in srgb, var(--c-bad) 12%, transparent);
  color: var(--c-bad);
}
.relay-badge {
  margin-right: 6px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--c-signal) 18%, transparent);
  color: var(--c-signal-text);
  font-size: 12px;
}
.probe-badge,
.policy-badge {
  margin-right: 6px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  font-size: 12px;
}
.probe-badge {
  background: color-mix(in srgb, var(--c-ok) 14%, transparent);
  color: var(--c-ok);
}
.probe-badge.is-bad {
  background: color-mix(in srgb, var(--c-bad) 12%, transparent);
  color: var(--c-bad);
}
.policy-badge {
  background: var(--c-surface-2);
  color: var(--c-muted);
}
.force-badge {
  margin-right: 6px;
  padding: 1px 8px;
  border-radius: var(--radius-pill);
  background: color-mix(in srgb, var(--c-data) 15%, transparent);
  color: var(--c-data);
  font-size: 12px;
}
.host.mono {
  font-size: 12px;
}
.mono {
  font: 400 14px/1.6 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.row-protocol {
  color: var(--c-muted);
  overflow-wrap: anywhere;
}
.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.row-visibility {
  display: flex;
  align-items: center;
}
.latency :deep(.status-dot),
.latency :deep(.status-dot span) {
  color: inherit;
}
.latency {
  transition: color var(--duration-medium);
  color: var(--c-muted);
}
.latency--ok {
  color: var(--c-ok);
}
.latency--warn {
  color: var(--c-warn);
}
.latency--bad,
.danger {
  color: var(--c-bad);
}
.mobile-label {
  display: none;
}
@media (max-width: 1199px) {
  .node-row {
    grid-template-columns: minmax(0, 1fr) 65px 100px 44px 132px;
    gap: 8px;
  }
}
@media (max-width: 767px) {
  .node-row {
    grid-template-columns: minmax(0, 1fr) auto;
    padding: 12px 0;
    gap: 10px;
  }
  .node-identity {
    grid-column: 1 / -1;
  }
  .row-protocol {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .row-protocol .mobile-label,
  .row-visibility .mobile-label {
    display: inline;
    font: 400 14px/1.6 var(--font-ui);
    color: var(--c-muted);
  }
  .latency {
    justify-self: end;
  }
  .row-visibility {
    gap: 10px;
  }
  .row-actions {
    justify-self: end;
  }
}
</style>
