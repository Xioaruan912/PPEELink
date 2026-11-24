<script setup lang="ts">
import { CopyDocument, Edit, MoreFilled } from "@element-plus/icons-vue";
import { StatusDot } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import CountryFlag from "@/ui/CountryFlag.vue";
import { formatDateTime } from "@/utils/date";
import { useNodes, validState, validText, type OverviewItem } from "./useNodes";

defineProps<{ node: OverviewItem }>();
const emit = defineEmits<{ inspect: [id: number] }>();

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
        : "已关闭下发";
  return b.until ? `${short}，${b.until.slice(5, 10)} 恢复` : short;
};

const {
  selectedIds,
  toggleSelect,
  nodeCommand,
  copyNode,
  protocol,
  busy,
  setDelivery,
  activeGroup,
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
        <div class="name-line">
          <button
            type="button"
            class="name-trigger"
            :title="node.name"
            :aria-label="`查看 ${node.name} 详情`"
            @click="emit('inspect', node.id)"
          >
            {{ node.name }}
          </button>
          <span
            v-if="node.blocked"
            class="meta-pill is-warn"
            :title="node.blocked.reason"
          >
            暂不下发 · {{ blockLabel(node) }}
          </span>
          <span
            v-if="node.userCheckedAt"
            class="meta-pill mono"
            :class="{ 'is-bad': !node.userValid }"
            :title="`${node.userProbeName || '用户 Agent'} 实测${node.userError ? '：' + node.userError : ''}`"
          >
            本机 {{ node.userValid ? `${node.userRtt ?? 0} ms` : "不可达" }}
          </span>
        </div>

        <p class="node-subline">
          <span class="host mono" :title="node.server">{{
            node.server || "主机未知"
          }}</span>
          <span class="sep" aria-hidden="true">·</span>
          <span>{{ node.country || "地区未知" }}</span>
          <span class="sep" aria-hidden="true">·</span>
          <span>{{ (node.groups || []).join(" / ") || "未分组" }}</span>
        </p>
      </div>
    </div>

    <div class="row-protocol mono">
      <span class="mobile-label">协议</span>
      <span>{{ protocol(node) }}</span>
    </div>

    <div class="latency" :class="`latency--${validState(node)}`">
      <span class="mobile-label">状态</span>
      <StatusDot
        :state="validState(node)"
        :running="checking"
        :title="
          node.checkedAt
            ? `检测于 ${formatDateTime(node.checkedAt)}`
            : undefined
        "
      >
        <span>{{ validText(node) }}</span>
      </StatusDot>
    </div>

    <div class="row-visibility">
      <span class="mobile-label">下发</span>
      <el-tooltip
        :trigger="['hover', 'focus']"
        :trigger-keys="[]"
        :content="
          node.deliveryEnabled === false
            ? '已关闭订阅下发；节点仍可管理和检测'
            : node.blocked
              ? `已允许订阅下发，当前暂不下发：${node.blocked.reason}`
              : '已允许订阅下发；关闭后节点仍保留在列表中'
        "
      >
        <el-switch
          :model-value="node.deliveryEnabled !== false"
          :disabled="busy"
          :aria-label="`${node.name} 是否下发`"
          @change="(value) => setDelivery(node.id, value === true)"
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
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="tcp">国内可达性检测</el-dropdown-item>
            <el-dropdown-item command="unlock">流媒体 / AI 解锁</el-dropdown-item>
            <el-dropdown-item command="temp">生成临时订阅</el-dropdown-item>
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
            <el-dropdown-item command="delete" divided :disabled="busy">
              <span class="danger">删除节点</span>
            </el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
    </div>
  </article>
</template>

<style scoped>
.node-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1fr) 88px 110px 64px 124px;
  align-items: center;
  gap: 16px;
  padding: 18px 10px;
  margin: 0 -10px;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  transition: opacity 280ms var(--ease-out);
}

.node-row::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: rgb(10 10 10 / 0.028);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 360ms var(--ease-out);
  pointer-events: none;
  z-index: 0;
}

.node-row::after {
  content: "";
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: -1px;
  height: 1px;
  background: var(--c-ink);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 340ms var(--ease-out);
  pointer-events: none;
  z-index: 1;
}

.node-row.is-selected::before {
  transform: scaleX(1);
  background: rgb(10 10 10 / 0.045);
}

.node-row > * {
  position: relative;
  z-index: 1;
}

.node-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.node-name {
  display: grid;
  gap: 5px;
  min-width: 0;
  flex: 1;
}

.name-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  min-width: 0;
}

.name-trigger {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-family: var(--font-display);
  font-size: 16px;
  font-weight: 500;
  letter-spacing: -0.02em;
  line-height: 1.25;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
  transition: transform 240ms var(--ease-out);
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 11px;
  line-height: 1.35;
  white-space: nowrap;
}

.meta-pill.is-warn {
  border-color: rgb(180 83 9 / 0.25);
  background: rgb(180 83 9 / 0.07);
  color: var(--c-warn);
  font-weight: 500;
}

.meta-pill.is-bad {
  border-color: rgb(223 41 41 / 0.25);
  background: rgb(223 41 41 / 0.07);
  color: var(--c-bad);
}

.meta-pill.is-ink {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.node-subline {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--c-muted);
  min-width: 0;
}

.host {
  max-width: 240px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.sep {
  opacity: 0.45;
}

.row-protocol {
  font-size: 12px;
  color: var(--c-muted);
  text-transform: uppercase;
  letter-spacing: 0.03em;
}

.latency {
  font-size: 12.5px;
}

.row-visibility {
  display: flex;
  align-items: center;
}

.row-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
  transition:
    opacity 240ms var(--ease-out),
    transform 240ms var(--ease-out);
}

.mobile-label {
  display: none;
  font-size: 11px;
  color: var(--c-muted);
}

.danger {
  color: var(--c-bad);
}

@media (hover: hover) and (pointer: fine) {
  .node-row:hover::before,
  .node-row:focus-within::before,
  .node-row:hover::after,
  .node-row:focus-within::after {
    transform: scaleX(1);
  }
  .node-row:hover .name-trigger {
    transform: translateX(3px);
  }
  .row-actions {
    opacity: 0.34;
    transform: translateX(6px);
  }
  .node-row:hover .row-actions,
  .node-row:focus-within .row-actions,
  .node-row.is-selected .row-actions {
    opacity: 1;
    transform: translateX(0);
  }
}

@media (max-width: 960px) {
  .node-row {
    grid-template-columns: 1fr auto;
    gap: 12px 16px;
    padding: 16px 0;
    margin: 0;
  }
  .node-row::after {
    left: 0;
    right: 0;
  }
  .node-identity {
    grid-column: 1 / -1;
  }
  .mobile-label {
    display: inline;
    margin-right: 6px;
  }
  .row-actions {
    grid-column: 1 / -1;
    justify-content: flex-end;
  }
}
</style>
