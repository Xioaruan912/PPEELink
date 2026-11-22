<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { UiDrawer, UiButton, UiSegmented, StatusDot, EmptyState } from "@/ui";
import CountryFlag from "@/ui/CountryFlag.vue";
import IconCopy from "~icons/ph/copy";
import IconPencil from "~icons/ph/pencil-simple";
import IconPulse from "~icons/ph/pulse";
import IconFlask from "~icons/ph/flask";
import IconSliders from "~icons/ph/sliders-horizontal";
import IconTrash from "~icons/ph/trash";
import { formatDateTime } from "@/utils/date";
import { useNodes, validState, validText, type OverviewItem } from "./useNodes";

const id = defineModel<number | null>({ required: true });
const {
  overviewList,
  protocol,
  copyNode,
  nodeCommand,
  setDelivery,
  busy,
  activeGroup,
} = useNodes();
const node = computed(() =>
  overviewList.value.find((item) => item.id === id.value)
);
const visible = computed({
  get: () => !!node.value,
  set: (open) => {
    if (!open) id.value = null;
  },
});
const section = ref<string | number>("connection");
const sections = [
  { label: "连接", value: "connection" },
  { label: "检测", value: "checks" },
  { label: "订阅下发", value: "delivery" },
];
const policies = {
  panel: "面板检测",
  any: "任一 Agent 通过",
  all: "全部 Agent 通过",
  manual: "手动标记有效",
};
const probeCount = computed(
  () => node.value?.probes?.filter((probe) => probe.success).length ?? 0
);
const capabilityTiles = computed(() => [
  {
    label: "UDP 转发",
    active: !!node.value?.udp,
    status: node.value?.udp ? "已启用" : "未开启",
  },
  {
    label: "TCP Fast Open",
    active: !!node.value?.tfo,
    status: node.value?.tfo ? "已启用" : "未开启",
  },
  {
    label: "TLS 证书校验",
    active: !node.value?.skipCertVerify,
    status: node.value?.skipCertVerify ? "已跳过" : "严格校验",
  },
]);
const rttPercent = (rtt?: number, ok?: boolean) => {
  if (!ok || !rtt) return 0;
  return Math.max(14, Math.min(100, Math.round((1 - Math.min(rtt, 350) / 400) * 100)));
};
watch(id, () => {
  section.value = "connection";
});
function openAction(command: string, selected: OverviewItem) {
  visible.value = false;
  nodeCommand(command, selected);
}
</script>

<template>
  <UiDrawer
    v-model="visible"
    title="节点详情"
    size="min(660px, 100vw)"
    append-to-body
    class="node-inspector"
  >
    <div v-if="node" class="inspection">
      <header class="dossier-hero">
        <div class="dossier-top">
          <div class="inspection-symbol">
            <CountryFlag :code="node.countryCode" :label="node.country" />
          </div>
          <div class="inspection-title">
            <h3>{{ node.name }}</h3>
            <p class="inspection-host mono">
              {{ node.server || "未指定主机地址" }}
            </p>
          </div>
          <div class="hero-status-pill">
            <StatusDot :state="validState(node)" :label="validText(node)" />
          </div>
        </div>
        <div class="dossier-metrics">
          <div>
            <span>传输协议</span>
            <strong>{{ protocol(node) }}</strong>
          </div>
          <div>
            <span>探针通过</span>
            <strong class="tabular">{{
              node.probes?.length
                ? `${probeCount} / ${node.probes.length}`
                : "未检测"
            }}</strong>
          </div>
          <div>
            <span>分发状态</span>
            <strong>{{
              node.blocked
                ? "暂不下发"
                : node.deliveryEnabled === false
                  ? "已关闭"
                  : "正常分发"
            }}</strong>
          </div>
        </div>
      </header>

      <div class="inspection-controls">
        <UiSegmented
          v-model="section"
          :options="sections"
          label="节点详情分区"
        />
      </div>

      <Transition name="inspector-pane" mode="out-in">
        <section
          v-if="section === 'connection'"
          key="connection"
          class="inspection-section"
          aria-label="连接信息"
        >
          <div class="spec-tray">
            <header class="spec-tray__head">
              <h4>连接规格</h4>
            </header>
            <dl class="bento-specs">
              <div class="bento-spec-cell is-wide">
                <dt>服务器地址</dt>
                <dd class="mono">{{ node.server || "未知" }}</dd>
              </div>
              <div class="bento-spec-cell">
                <dt>协议类型</dt>
                <dd>{{ protocol(node) }}</dd>
              </div>
              <div class="bento-spec-cell">
                <dt>国家或地区</dt>
                <dd>{{ node.country || "未知" }}</dd>
              </div>
              <div class="bento-spec-cell is-wide">
                <dt>所属分组</dt>
                <dd>{{ node.groups?.join(" · ") || "未分组" }}</dd>
              </div>
            </dl>

            <div class="capability-grid" aria-label="传输特性">
              <div
                v-for="cap in capabilityTiles"
                :key="cap.label"
                class="capability-tile"
                :class="{ 'is-on': cap.active }"
              >
                <span class="cap-dot" aria-hidden="true"></span>
                <div>
                  <strong>{{ cap.label }}</strong>
                  <small>{{ cap.status }}</small>
                </div>
              </div>
            </div>

            <div class="inspection-actions">
              <UiButton @click="openAction('group', node)">加入分组</UiButton>
              <UiButton
                v-if="
                  activeGroup !== '全部' && node.groups?.includes(activeGroup)
                "
                :disabled="busy"
                @click="nodeCommand('unbind', node)"
                >移出当前分组</UiButton
              >
            </div>
          </div>

          <div class="danger-zone">
            <div>
              <strong>移除此节点</strong>
              <p>从节点库与关联订阅中彻底移除。</p>
            </div>
            <UiButton
              variant="danger"
              :icon="IconTrash"
              :disabled="busy"
              @click="nodeCommand('delete', node)"
              >删除节点</UiButton
            >
          </div>
        </section>

        <section
          v-else-if="section === 'checks'"
          key="checks"
          class="inspection-section"
          aria-label="检测状态"
        >
          <div class="spec-tray">
            <header class="spec-tray__head">
              <h4>可用性策略与即时诊断</h4>
            </header>
            <dl class="bento-specs">
              <div class="bento-spec-cell">
                <dt>判定策略</dt>
                <dd>{{ policies[node.probePolicy || "panel"] }}</dd>
              </div>
              <div class="bento-spec-cell">
                <dt>最近检测</dt>
                <dd class="tabular">
                  {{
                    node.checkedAt ? formatDateTime(node.checkedAt) : "暂无记录"
                  }}
                </dd>
              </div>
            </dl>
            <div class="inspection-actions">
              <UiButton :icon="IconPulse" @click="openAction('tcp', node)"
                >国内可达性</UiButton
              >
              <UiButton :icon="IconFlask" @click="openAction('unlock', node)"
                >解锁检测</UiButton
              >
              <UiButton
                variant="ghost"
                :icon="IconSliders"
                @click="openAction('probe-policy', node)"
                >修改检测策略</UiButton
              >
            </div>
          </div>

          <div class="spec-tray">
            <header class="spec-tray__head">
              <h4>探针遥测矩阵</h4>
              <span v-if="node.probes?.length" class="probe-summary tabular"
                >{{ probeCount }} / {{ node.probes.length }} 节点可达</span
              >
            </header>
            <ul v-if="node.probes?.length" class="probe-cards">
              <li
                v-for="probe in node.probes"
                :key="probe.id"
                class="probe-card"
              >
                <div class="probe-card__top">
                  <div class="probe-info">
                    <strong>{{ probe.name }}</strong>
                    <small
                      >{{ probe.region
                      }}{{
                        !probe.success && probe.error ? ` · ${probe.error}` : ""
                      }}</small
                    >
                  </div>
                  <StatusDot
                    :state="probe.success ? 'ok' : 'bad'"
                    :label="probe.success ? `${probe.rtt} ms` : '连接失败'"
                  />
                </div>
                <div class="probe-meter" aria-hidden="true">
                  <span
                    class="probe-meter__fill"
                    :class="{ 'is-bad': !probe.success }"
                    :style="{
                      width: `${probe.success ? rttPercent(probe.rtt, true) : 100}%`,
                    }"
                  ></span>
                </div>
              </li>
            </ul>
            <EmptyState
              v-else
              size="inline"
              description="该节点暂无探针记录。"
            />
          </div>
        </section>

        <section
          v-else
          key="delivery"
          class="inspection-section"
          aria-label="订阅下发"
        >
          <div class="spec-tray">
            <header class="spec-tray__head">
              <h4>分发控制</h4>
            </header>
            <div class="delivery-banner">
              <div>
                <strong>允许订阅下发</strong>
                <p>
                  {{
                    node.blocked?.reason ||
                    (node.deliveryEnabled === false
                      ? "节点已从订阅输出排除。"
                      : "节点可随订阅正常分发至客户端。")
                  }}
                </p>
                <p v-if="node.blocked?.until" class="resume-hint tabular">
                  预计恢复 {{ formatDateTime(node.blocked.until) }}
                </p>
              </div>
              <el-switch
                :model-value="node.deliveryEnabled !== false"
                :disabled="busy"
                aria-label="允许订阅下发"
                @change="setDelivery(node.id, $event === true)"
              />
            </div>
            <div class="inspection-actions">
              <UiButton
                v-if="node.blocked?.kind === 'paused'"
                :disabled="busy"
                @click="nodeCommand('resume', node)"
                >恢复下发</UiButton
              >
              <UiButton
                v-else
                :disabled="busy"
                @click="openAction('pause', node)"
                >暂停下发</UiButton
              >
              <UiButton :disabled="busy" @click="nodeCommand('deliver', node)">{{
                node.forceDeliver ? "恢复按检测结果下发" : "设置始终下发"
              }}</UiButton>
              <UiButton @click="openAction('temp', node)">临时订阅</UiButton>
            </div>
          </div>
        </section>
      </Transition>
    </div>
    <template v-if="node" #footer>
      <UiButton :icon="IconPencil" @click="openAction('edit', node)"
        >编辑连接</UiButton
      >
      <UiButton variant="primary" :icon="IconCopy" @click="copyNode(node)"
        >复制节点链接</UiButton
      >
    </template>
  </UiDrawer>
</template>

<style scoped>
.inspection {
  display: grid;
  gap: 18px;
}
.dossier-hero {
  display: grid;
  gap: 18px;
  padding: 22px;
  border-radius: var(--radius-panel);
  background: var(--rail-bg);
  color: var(--rail-text);
}
.dossier-top {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 14px;
}
.inspection-symbol {
  display: grid;
  place-items: center;
  width: 46px;
  height: 46px;
  background: var(--rail-hover);
  border-radius: var(--radius-control);
}
.inspection-title {
  min-width: 0;
}
.dossier-hero h3 {
  font-size: clamp(19px, 3vw, 24px);
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: -0.025em;
  color: var(--rail-active);
  overflow-wrap: anywhere;
}
.inspection-host {
  margin-top: 4px;
  font-size: 12.5px;
  color: var(--rail-muted);
}
.hero-status-pill {
  padding: 4px 11px;
  border-radius: var(--radius-pill);
  background: var(--rail-hover);
}
.dossier-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  padding-top: 14px;
  border-top: 1px solid rgb(255 255 255 / 0.12);
}
.dossier-metrics > div {
  display: grid;
  gap: 4px;
  padding: 6px 14px;
  border-right: 1px solid rgb(255 255 255 / 0.12);
}
.dossier-metrics > div:first-child {
  padding-left: 0;
}
.dossier-metrics > div:last-child {
  border-right: 0;
}
.dossier-metrics span {
  font-size: 11.5px;
  color: var(--rail-muted);
}
.dossier-metrics strong {
  font-size: 14.5px;
  font-weight: 660;
  color: var(--rail-active);
}

.inspection-controls {
  display: flex;
}
.inspection-controls :deep(.ui-segmented) {
  width: 100%;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
}
.inspection-section {
  display: grid;
  gap: 16px;
}
.spec-tray {
  padding: 18px 20px;
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  border: 1px solid var(--c-line);
}
.spec-tray__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}
.spec-tray__head h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 630;
  color: var(--c-ink);
}
.probe-summary {
  font-size: 12.5px;
  color: var(--c-muted);
}
.tabular {
  font-variant-numeric: tabular-nums;
}

.bento-specs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin: 0;
}
.bento-spec-cell {
  display: grid;
  gap: 4px;
  padding: 12px 14px;
  border-radius: var(--radius-control);
  background: var(--c-canvas);
  border: 1px solid var(--c-line);
}
.bento-spec-cell.is-wide {
  grid-column: 1 / -1;
}
.bento-spec-cell dt {
  font-size: 11.5px;
  color: var(--c-muted);
}
.bento-spec-cell dd {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
  overflow-wrap: anywhere;
}

.capability-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  margin-top: 10px;
}
.capability-tile {
  display: flex;
  align-items: center;
  gap: 9px;
  padding: 10px 12px;
  border-radius: var(--radius-control);
  background: var(--c-canvas);
  border: 1px solid var(--c-line);
  opacity: 0.65;
}
.capability-tile.is-on {
  opacity: 1;
  background: color-mix(in srgb, var(--c-ok) 6%, var(--c-surface));
  border-color: color-mix(in srgb, var(--c-ok) 24%, var(--c-line));
}
.cap-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--c-muted);
  flex: none;
}
.capability-tile.is-on .cap-dot {
  background: var(--c-ok);
}
.capability-tile strong {
  display: block;
  font-size: 12.5px;
  font-weight: 600;
  color: var(--c-ink);
}
.capability-tile small {
  display: block;
  font-size: 11.5px;
  color: var(--c-muted);
}

.inspection-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 16px;
}

.probe-cards {
  display: grid;
  gap: 10px;
}
.probe-card {
  display: grid;
  gap: 10px;
  padding: 14px 16px;
  border-radius: var(--radius-control);
  background: var(--c-canvas);
  border: 1px solid var(--c-line);
}
.probe-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.probe-info {
  min-width: 0;
}
.probe-info strong,
.delivery-banner strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}
.probe-info small,
.delivery-banner p {
  display: block;
  margin-top: 2px;
  color: var(--c-muted);
  font-size: 12.5px;
  overflow-wrap: anywhere;
}
.probe-meter {
  height: 5px;
  border-radius: 999px;
  background: var(--c-surface-2);
  overflow: hidden;
}
.probe-meter__fill {
  display: block;
  height: 100%;
  border-radius: 999px;
  background: var(--c-ok);
  transition: width var(--duration-slow) var(--ease-out);
}
.probe-meter__fill.is-bad {
  background: var(--c-bad);
}

.delivery-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border-radius: 14px;
  background: var(--c-canvas);
  border: 1px solid var(--c-line);
}
.resume-hint {
  color: var(--c-warn-ink) !important;
  font-weight: 560;
}
.danger-zone {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-radius: 18px;
  background: color-mix(in srgb, var(--c-bad) 6%, var(--c-surface));
  border: 1px solid color-mix(in srgb, var(--c-bad) 18%, var(--c-line));
}
.danger-zone strong {
  font-size: 14px;
  font-weight: 620;
  color: var(--c-ink);
}
.danger-zone p {
  margin-top: 2px;
  font-size: 12.5px;
  color: var(--c-muted);
}

.inspector-pane-enter-active,
.inspector-pane-leave-active {
  transition:
    opacity var(--duration-fast) var(--ease-out),
    transform var(--duration-fast) var(--ease-out);
}
.inspector-pane-enter-from {
  opacity: 0;
  transform: translateY(6px);
}
.inspector-pane-leave-to {
  opacity: 0;
  transform: translateY(-4px);
}

@media (max-width: 520px) {
  .dossier-top {
    grid-template-columns: auto minmax(0, 1fr);
  }
  .hero-status-pill {
    grid-column: 2;
    width: fit-content;
  }
  .bento-specs,
  .capability-grid {
    grid-template-columns: 1fr;
  }
  .danger-zone {
    flex-direction: column;
    align-items: stretch;
  }
}
@media (prefers-reduced-motion: reduce) {
  .inspector-pane-enter-active,
  .inspector-pane-leave-active {
    transition: none;
  }
}
</style>
