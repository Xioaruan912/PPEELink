<template>
  <div ref="rootEl" class="dash-cinema" :aria-busy="loading">
    <!-- Compact Single-Screen Top Bar -->
    <header class="dash-masthead" aria-labelledby="dash-title">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 id="dash-title" class="gallery-title" data-title-reveal>
            运行态势
          </h1>
        </div>
        <span
          v-if="!loading"
          class="live-status mono"
          :class="{ 'is-warn': summary.invalid > 0 }"
        >
          <i aria-hidden="true"></i>
          {{ summary.valid }}/{{ summary.total }} 在线 · {{ validRate }}%
        </span>
        <button
          v-for="item in topologyAnomalies"
          :key="item.title"
          type="button"
          class="anomaly-pill"
          :class="`is-${item.tone}`"
          @click="router.push(item.target)"
        >
          <i aria-hidden="true"></i>
          <span>{{ item.title }}</span>
        </button>
      </div>

      <div class="masthead-actions">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="loading"
          @click="loadOverview"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <div v-if="loadError" class="overview-notice" role="alert">
      <StatusDot state="warn" :label="loadError" />
      <UiButton variant="ghost" :disabled="loading" @click="loadOverview">
        重新加载
      </UiButton>
    </div>

    <!-- Flex-Filling Interactive Global Topology Canvas -->
    <section class="map-hero-wrap" aria-label="全球节点交互地图">
      <WorldMap
        class="dash-hero-map"
        :points="mapPoints"
        :origins="topologyOrigins"
        :live-arcs="liveFlightArcs"
        :loading="loading"
        :selected-name="selectedPoint?.name"
        @select="onMapSelect"
        @refresh="loadOverview"
      />

      <!-- Subtle Hint When No Node Is Selected -->
      <Transition name="inspector-pop">
        <div
          v-if="!selectedPoint && mapPoints.length && !loading"
          class="map-hint"
        >
          <i aria-hidden="true"></i>
          <span>点击地图圆点查看节点卡片</span>
        </div>
      </Transition>

      <!-- Floating Frosted Glass Node Card (In-Place, No Route Jump) -->
      <Transition name="inspector-pop" mode="out-in">
        <aside
          v-if="selectedPoint && !loading"
          :key="selectedPoint.name"
          class="map-inspector"
          aria-live="polite"
        >
          <div class="inspector-top">
            <div class="inspector-badges">
              <span class="inspector-region">
                {{ selectedPoint.country || "未识别地区" }}
              </span>
              <span v-if="activeProtocol" class="inspector-proto mono">
                {{ activeProtocol }}
              </span>
            </div>

            <div class="inspector-right">
              <span
                class="inspector-state"
                :class="{
                  'is-blocked': activeNode?.blocked,
                  'is-bad': activeNode && !activeNode.blocked && !activeNode.valid,
                }"
              >
                <i aria-hidden="true"></i>
                {{
                  activeNode?.blocked
                    ? "暂不下发"
                    : activeNode && !activeNode.valid
                      ? "无效"
                      : "运行正常"
                }}
              </span>
              <button
                type="button"
                class="inspector-close"
                aria-label="关闭节点卡片"
                @click="selectedPoint = undefined"
              >
                <el-icon><Close /></el-icon>
              </button>
            </div>
          </div>

          <h2 class="inspector-name">{{ selectedPoint.name }}</h2>

          <p
            v-if="activeNode?.server || selectedPoint.server"
            class="inspector-host mono"
          >
            {{ activeNode?.server || selectedPoint.server }}
          </p>

          <dl v-if="activeNode" class="inspector-meta">
            <div>
              <dt>所属分组</dt>
              <dd>{{ activeNode.groups?.join(" · ") || "未分组" }}</dd>
            </div>
            <div>
              <dt>探针检测</dt>
              <dd class="mono">
                {{
                  activeNode.probes?.length
                    ? `${activeNode.probes.filter((p) => p.success).length}/${activeNode.probes.length} 通过`
                    : "未测"
                }}
              </dd>
            </div>
            <div>
              <dt>UDP 转发</dt>
              <dd>{{ activeNode.udp ? "已启用" : "未开启" }}</dd>
            </div>
            <div>
              <dt>TCP Fast Open</dt>
              <dd>{{ activeNode.tfo ? "已启用" : "未开启" }}</dd>
            </div>
          </dl>

          <div v-if="activeNode?.link" class="inspector-foot">
            <button
              type="button"
              class="inspector-copy"
              :class="{ 'is-done': copiedName === selectedPoint.name }"
              @click="copyActiveLink"
            >
              <el-icon><Check v-if="copiedName === selectedPoint.name" /><CopyDocument v-else /></el-icon>
              <span>{{ copiedName === selectedPoint.name ? "已复制链接" : "复制节点链接" }}</span>
            </button>
          </div>
        </aside>
      </Transition>
    </section>

    <!-- Always-Visible Node Quick Selector Strip -->
    <nav
      v-if="mapPoints.length && !loading"
      class="node-pill-rail"
      aria-label="快速定位地图节点"
    >
      <button
        v-for="pt in mapPoints"
        :key="pt.name"
        type="button"
        class="map-node-pill"
        :class="{ 'is-active': selectedPoint?.name === pt.name }"
        @click="togglePoint(pt)"
      >
        <i
          class="pill-dot"
          :class="{
            'is-blocked': nodeByName(pt.name)?.blocked,
            'is-bad':
              nodeByName(pt.name) &&
              !nodeByName(pt.name)?.blocked &&
              !nodeByName(pt.name)?.valid,
          }"
          aria-hidden="true"
        ></i>
        <span>{{ pt.name }}</span>
        <small>{{ pt.country || "未知" }}</small>
      </button>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { useRouter } from "vue-router";
import { ElMessage } from "element-plus";
import gsap from "gsap";
import { getNodeMap } from "@/api/total";
import { getNodeOverview, getNodeQualitySummary } from "@/api/subcription/node";
import {
  getProbeTopology,
  type ProbeTopologyAnomaly,
  type ProbeTopologyArc,
  type ProbeTopologyOrigin,
} from "@/api/subcription/probe";
import type { OverviewItem } from "@/views/subcription/nodes/useNodes";
import { Check, Close, CopyDocument, Refresh } from "@element-plus/icons-vue";
import { UiButton, StatusDot } from "@/ui";
import WorldMap from "./components/WorldMap.vue";

defineOptions({ name: "Dashboard", inheritAttrs: false });

interface MapPoint {
  lat: number;
  lng: number;
  name: string;
  country: string;
  server?: string;
}

const router = useRouter();
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const loading = ref(true);
const loadError = ref("");
const summary = ref({ total: 0, valid: 0, invalid: 0 });
const mapPoints = ref<MapPoint[]>([]);
const nodes = ref<OverviewItem[]>([]);
const topologyOrigins = ref<ProbeTopologyOrigin[]>([]);
const topologyRawArcs = ref<ProbeTopologyArc[]>([]);
const topologyAnomalies = ref<ProbeTopologyAnomaly[]>([]);

const liveFlightArcs = computed(() => {
  if (!topologyOrigins.value.length || !topologyRawArcs.value.length) return [];
  const originMap = new Map(topologyOrigins.value.map((o) => [o.id, o]));
  const nodeNameById = new Map(nodes.value.map((n) => [n.id, n.name]));
  const ptByName = new Map(mapPoints.value.map((p) => [p.name, p]));

  const out: {
    key: string;
    fromLat: number;
    fromLng: number;
    toLat: number;
    toLng: number;
    rtt: number;
    success: boolean;
    title: string;
  }[] = [];

  for (const arc of topologyRawArcs.value) {
    const orig = originMap.get(arc.probeId);
    const name = nodeNameById.get(arc.nodeId);
    const pt = name ? ptByName.get(name) : undefined;
    if (!orig || !pt) continue;
    out.push({
      key: `${arc.probeId}-${arc.nodeId}`,
      fromLat: orig.lat,
      fromLng: orig.lon,
      toLat: pt.lat,
      toLng: pt.lng,
      rtt: arc.rtt,
      success: arc.success,
      title: `${orig.name} → ${pt.name} · ${arc.success ? `${arc.rtt} ms` : "不可达"}`,
    });
  }
  return out;
});
const selectedPoint = ref<MapPoint | undefined>(undefined);
const copiedName = ref("");
let copyTimer: ReturnType<typeof setTimeout> | null = null;

const validRate = computed(() => {
  const { total, invalid: offline } = summary.value;
  return total
    ? Math.round((Math.max(total - offline, 0) / total) * 1000) / 10
    : 0;
});

function nodeByName(name: string) {
  return nodes.value.find((n) => n.name === name);
}

const activeNode = computed(() =>
  selectedPoint.value ? nodeByName(selectedPoint.value.name) : undefined
);

const activeProtocol = computed(() => {
  const link = activeNode.value?.link || "";
  const match = link.match(/^([a-zA-Z0-9+.-]+):\/\//);
  return match ? match[1].toUpperCase() : "";
});

function onMapSelect(pt: MapPoint | undefined) {
  if (!pt) return;
  selectedPoint.value = pt;
}

function togglePoint(pt: MapPoint) {
  if (selectedPoint.value?.name === pt.name) {
    selectedPoint.value = undefined;
  } else {
    selectedPoint.value = pt;
  }
}

async function copyTextSafe(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      /* fall through to execCommand */
    }
  }
  const ta = document.createElement("textarea");
  ta.value = text;
  ta.setAttribute("readonly", "");
  ta.style.position = "fixed";
  ta.style.top = "-9999px";
  ta.style.left = "-9999px";
  document.body.appendChild(ta);
  ta.focus();
  ta.select();
  let ok = false;
  try {
    ok = document.execCommand("copy");
  } catch {
    ok = false;
  } finally {
    document.body.removeChild(ta);
  }
  return ok;
}

async function copyActiveLink() {
  const link = activeNode.value?.link;
  const name = selectedPoint.value?.name;
  if (!link || !name) return;
  const ok = await copyTextSafe(link);
  if (ok) {
    copiedName.value = name;
    ElMessage.success("已复制节点链接");
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => {
      copiedName.value = "";
    }, 1800);
  } else {
    ElMessage.warning("复制失败，请检查浏览器剪贴板权限");
  }
}

let inFlight = false;
async function loadOverview() {
  if (inFlight) return;
  inFlight = true;
  loading.value = true;
  loadError.value = "";
  const results = await Promise.allSettled([
    getNodeQualitySummary(),
    getNodeMap(),
    getNodeOverview(),
    getProbeTopology(),
  ]);
  const [quality, map, overview, topo] = results;
  if (quality.status === "fulfilled")
    summary.value = { ...summary.value, ...quality.value.data };
  if (map.status === "fulfilled") {
    mapPoints.value = Array.isArray(map.value.data) ? map.value.data : [];
    if (
      selectedPoint.value &&
      !mapPoints.value.some((p) => p.name === selectedPoint.value?.name)
    ) {
      selectedPoint.value = undefined;
    }
  }
  if (overview.status === "fulfilled")
    nodes.value = Array.isArray(overview.value.data) ? overview.value.data : [];
  if (topo.status === "fulfilled" && topo.value.data) {
    topologyOrigins.value = topo.value.data.origins || [];
    topologyRawArcs.value = topo.value.data.arcs || [];
    topologyAnomalies.value = topo.value.data.anomalies || [];
  }
  if (results.slice(0, 3).some((r) => r.status === "rejected"))
    loadError.value = "部分网络数据暂时无法加载，请稍后重试。";
  loading.value = false;
  inFlight = false;
}

onMounted(() => {
  void loadOverview();
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  if (copyTimer) clearTimeout(copyTimer);
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<style scoped>
.dash-cinema {
  display: flex;
  flex-direction: column;
  gap: 14px;
  height: calc(100dvh - 176px);
  min-height: 460px;
  max-height: 820px;
  max-width: 1280px;
  min-width: 0;
}

.dash-masthead {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.6vw, 34px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.live-status {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-ink);
}

.live-status i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
  box-shadow: 0 0 0 3px rgb(21 184 90 / 0.16);
}

.live-status.is-warn i {
  background: var(--c-warn);
  box-shadow: 0 0 0 3px rgb(180 83 9 / 0.16);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  flex: none;
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.overview-notice {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}

/* Flex-Filling Hero Map Canvas */
.map-hero-wrap {
  position: relative;
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.dash-hero-map {
  height: 100%;
  border-radius: 20px;
  overflow: hidden;
}

.dash-hero-map :deep(.ui-panel__header) {
  display: none;
}

.dash-hero-map :deep(.ui-panel__body) {
  height: 100%;
  padding: 0;
}

.dash-hero-map :deep(.map-stage) {
  height: 100%;
  min-height: 0;
}

.dash-hero-map :deep(.world-map) {
  width: 100%;
  height: 100%;
  min-height: 0;
}

.dash-hero-map :deep(.map-selector),
.dash-hero-map :deep(.map-location) {
  display: none;
}

.dash-hero-map :deep(.node-hit) {
  cursor: pointer;
  fill: transparent;
}

.dash-hero-map :deep(.map-node.is-selected .node-mark) {
  fill: var(--c-ok);
}

.dash-hero-map :deep(.map-node.is-selected .node-halo) {
  stroke: var(--c-ok);
}

/* Subtle Hint Pill When Closed */
.map-hint {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 8;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 88%, transparent);
  backdrop-filter: blur(16px);
  font-size: 12px;
  color: var(--c-muted);
  pointer-events: none;
}

.map-hint i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

/* Floating Frosted Glass Node Card */
.map-inspector {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 12;
  width: min(336px, calc(100% - 40px));
  max-height: calc(100% - 40px);
  overflow-y: auto;
  display: grid;
  gap: 10px;
  padding: 18px 20px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: color-mix(in srgb, var(--c-surface) 92%, transparent);
  backdrop-filter: blur(22px) saturate(160%);
  box-shadow: 0 24px 56px -18px rgb(0 0 0 / 0.18);
}

.inspector-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.inspector-badges,
.inspector-right {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.inspector-region {
  font-size: 11.5px;
  font-weight: 500;
  color: var(--c-muted);
}

.inspector-proto {
  padding: 2px 7px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 10.5px;
  font-weight: 600;
  letter-spacing: 0.04em;
}

.inspector-state {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  font-weight: 500;
  color: var(--c-ok);
}

.inspector-state i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.inspector-state.is-blocked {
  color: var(--c-warn);
}

.inspector-state.is-bad {
  color: var(--c-bad);
}

.inspector-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.inspector-close:hover {
  background: var(--c-surface-muted);
  color: var(--c-ink);
}

.inspector-name {
  margin: 0;
  font-family: var(--font-display);
  font-size: 17px;
  font-weight: 500;
  letter-spacing: -0.025em;
  color: var(--c-ink);
}

.inspector-host {
  margin: -4px 0 0;
  font-size: 11.5px;
  color: var(--c-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px 12px;
  margin: 0;
  padding: 12px 0;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
}

.inspector-meta dt {
  font-size: 11px;
  color: var(--c-muted);
  margin-bottom: 2px;
}

.inspector-meta dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.inspector-foot {
  display: flex;
  justify-content: flex-end;
}

.inspector-copy {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.inspector-copy:hover {
  border-color: var(--c-ink);
  background: var(--c-ink);
  color: var(--c-surface);
}

.inspector-copy.is-done {
  border-color: var(--c-ok);
  color: var(--c-ok);
}

.inspector-pop-enter-active,
.inspector-pop-leave-active {
  transition:
    opacity 220ms var(--ease-out),
    transform 220ms var(--ease-out);
}

.inspector-pop-enter-from,
.inspector-pop-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

/* Always-Visible Node Quick Selector Strip */
.node-pill-rail {
  flex: none;
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: thin;
}

.map-node-pill {
  flex: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.map-node-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.map-node-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.map-node-pill small {
  font-size: 11px;
  opacity: 0.65;
}

.pill-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
  flex: none;
}

.pill-dot.is-blocked {
  background: var(--c-warn);
}

.pill-dot.is-bad {
  background: var(--c-bad);
}

.anomaly-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 11px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: transform 120ms ease, border-color 150ms ease;
}

.anomaly-pill:active {
  transform: scale(0.97);
}

.anomaly-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.anomaly-pill.is-warn {
  border-color: color-mix(in srgb, var(--c-warn) 40%, transparent);
  color: var(--c-warn);
}

.anomaly-pill.is-bad {
  border-color: color-mix(in srgb, var(--c-bad) 40%, transparent);
  color: var(--c-bad);
}

@media (max-width: 780px) {
  .dash-cinema {
    height: auto;
    min-height: 0;
    max-height: none;
  }
  .map-hero-wrap {
    flex: none;
  }
  .dash-hero-map {
    height: 360px;
  }
  .map-hint {
    display: none;
  }
  .map-inspector {
    position: static;
    width: 100%;
    max-height: none;
    margin-top: 12px;
  }
}
</style>
