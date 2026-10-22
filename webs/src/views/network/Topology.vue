<template>
  <div class="topology-page">
    <header class="page-heading">
      <div><p class="eyebrow">NETWORK TOPOLOGY · 只读</p><h1>网络拓扑</h1><p>沿明确的配置关系，查看订阅、节点与服务器之间的路径。</p></div>
      <UiButton :loading="loading" :title="loading ? '正在读取配置快照，请稍候' : '重新读取配置，不执行网络检测'" @click="reload">{{ loading ? '读取中' : '重新加载' }}</UiButton>
    </header>

    <div class="snapshot-state" role="status" aria-live="polite">
      <span v-if="!online">离线 · 保留当前快照，恢复连接后可重新加载。</span>
      <span v-else-if="loading">正在读取拓扑{{ snapshot ? '，当前仍显示上一份快照' : '' }}…</span>
      <span v-else-if="snapshot">快照 {{ formatTime(snapshot.generatedAt) }} · {{ graph.total }} 个对象 · 当前显示 {{ graph.shown }} 个</span>
    </div>
    <div v-if="error" class="notice error" role="alert">{{ error }} <span v-if="snapshot">下方为上次成功读取的快照。</span>请检查连接或权限后，点击“重新加载”。</div>
    <div v-if="snapshot?.warnings.length" class="notice" role="status">部分数据不可用：{{ snapshot.warnings.join('；') }}。可继续查看已读取的数据，或重新加载。</div>

    <UiPanel title="配置关系图" description="实线表示明确引用；虚线表示中转。点击卡片查看关系详情。" flush>
      <div class="toolbar">
        <label class="search-label">查找对象<input v-model="search" type="search" placeholder="名称、类型或 ID" autocomplete="off" /></label>
        <label class="focus-toggle" :title="selected ? '仅保留所选对象的上游和下游' : '请先选择一张对象卡片'"><input v-model="focused" type="checkbox" :disabled="!selected" />仅关联路径</label>
        <span v-if="!selected" class="muted">选择卡片后可聚焦路径</span>
        <div class="zoom-controls" aria-label="图形缩放"><UiButton :disabled="zoom <= 0.6" title="缩小，最小 60%" @click="zoom = Math.max(0.6, +(zoom - 0.1).toFixed(1))">−</UiButton><UiButton title="重置为 100% 并回到图形起点" @click="resetView">{{ Math.round(zoom * 100) }}% · 重置</UiButton><UiButton :disabled="zoom >= 1.4" title="放大，最大 140%" @click="zoom = Math.min(1.4, +(zoom + 0.1).toFixed(1))">＋</UiButton></div>
      </div>
      <div class="legend" aria-label="图例"><span v-for="(label, kind) in kindLabels" :key="kind"><i :class="`kind-${kind}`"></i>{{ label }}</span><span>● 在线 / 运行中</span><span>○ 未检测不等于故障</span></div>
      <div v-if="loading && !snapshot" class="loading-grid" aria-label="正在加载拓扑"><div v-for="i in 5" :key="i" class="skeleton-column"><div v-for="j in 3" :key="j" class="skeleton-card"></div></div></div>
      <div v-else-if="!snapshot" class="empty"><h3>尚未读取到拓扑</h3><p>重新加载以获取当前配置。此页面不会修改配置或执行网络检测。</p><UiButton @click="reload">重试</UiButton></div>
      <div v-else-if="!graph.total" class="empty"><h3>暂无网络配置</h3><p>配置订阅、节点或自建服务器后，明确的引用关系会显示在这里。</p><RouterLink to="/subcription/nodes">查看节点配置 →</RouterLink></div>
      <div v-else-if="!graph.shown" class="empty"><h3>没有符合条件的对象</h3><p>请缩短搜索词，或退出关联路径模式。</p><UiButton @click="search = ''; focused = false">清除筛选</UiButton></div>
      <div v-else ref="viewport" class="graph-scroll" tabindex="0" aria-label="网络拓扑画布，可横向和纵向滚动">
        <div :style="{ width: `${WIDTH * zoom}px`, height: `${height * zoom}px` }">
          <div class="graph-canvas" :style="{ width: `${WIDTH}px`, height: `${height}px`, transform: `scale(${zoom})` }">
            <div v-for="(title, col) in columnTitles" :key="title" class="column-title" :style="{ left: `${col * COLUMN + 20}px` }">{{ title }} <span>{{ graph.columns[col].length }}</span></div>
            <svg class="connections" :width="WIDTH" :height="height" aria-hidden="true"><defs><marker id="topology-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="5" markerHeight="5" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="currentColor" /></marker></defs><path v-for="edge in graph.edges" :key="`${edge.source}-${edge.target}-${edge.relation}`" :d="edgePath(edge)" :class="{ relay: edge.relation === 'relay', highlighted: edge.source === selectedId || edge.target === selectedId }" marker-end="url(#topology-arrow)" /></svg>
            <button v-for="item in positioned" :key="item.id" type="button" class="node-card" :class="[`kind-${item.kind}`, { selected: selectedId === item.id }]" :style="{ left: `${item.x}px`, top: `${item.y}px` }" :aria-pressed="selectedId === item.id" :aria-label="`${kindLabels[item.kind]}：${item.name}，${statusLabels[item.status] || item.status}`" @click="selectedId = item.id"><span class="card-meta">{{ kindLabels[item.kind] }} <span :class="{ good: ['online', 'running'].includes(item.status) }">{{ statusLabels[item.status] || item.status }}</span></span><strong :title="item.name">{{ item.name || item.id }}</strong><small>{{ item.id }}</small></button>
          </div>
        </div>
      </div>
      <div v-if="snapshot" class="graph-footer">
        <p v-if="graph.shown < graph.filtered" class="notice">为保持可读性，每列最多显示 36 个对象；当前 {{ graph.shown }} / {{ graph.filtered }} 个。请搜索或选择对象聚焦路径。未显示对象之间的连线也已隐藏。</p>
        <p v-if="graph.dangling.length" class="notice">{{ graph.dangling.length }} 条引用的端点缺失（已删除或未能读取），未绘制这些连线。<span v-for="edge in graph.dangling.slice(0, 5)" :key="`${edge.source}-${edge.target}`"> {{ edge.source }} → {{ edge.target }}；</span></p>
        <p>自建入站通过 serverId 关联服务器；监控通过 proxyServerId 关联。现有配置节点没有服务器外键，因此不按名称、分组名或 IP 猜测服务器归属。节点“未检测”仅表示本页不执行健康检测。</p>
      </div>
    </UiPanel>

    <UiPanel title="对象详情" :description="selected ? '查看原始 ID、状态与明确的直接关系' : '选择图中的卡片，查看详情和管理入口'">
      <template v-if="selected"><div class="detail-heading"><div><h3>{{ selected.name }}</h3><p>{{ kindLabels[selected.kind] }} · {{ selected.id }} · {{ statusLabels[selected.status] || selected.status }}</p></div><RouterLink :to="selected.route">前往{{ kindLabels[selected.kind] }}管理 →</RouterLink></div><p v-if="selected.detail" class="muted">{{ selected.detail }}</p><p v-if="selected.checkedAt" class="muted">最近上报：{{ formatTime(selected.checkedAt) }}（不是实时状态）</p><p v-if="!connections.length" class="muted">没有已记录的直接引用关系；不代表网络不可达。</p><ul v-else class="relation-list"><li v-for="(connection, index) in connections.slice(0, 60)" :key="index"><span>{{ connection.incoming ? '上游' : '下游' }} · {{ relationLabels[connection.edge.relation] || connection.edge.relation }}</span><button v-if="connection.node" type="button" @click="selectRelated(connection.node.id)">{{ connection.node.name }} <small>{{ connection.node.id }}</small></button><span v-else>端点缺失：{{ connection.other }}</span></li></ul><p v-if="connections.length > 60" class="muted">显示前 60 / {{ connections.length }} 条直接关系，请搜索目标对象。</p></template>
      <p v-else class="muted">只读视图不会展示订阅令牌、节点连接凭据或安装链接。</p>
    </UiPanel>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import axios from "axios";
import request from "@/utils/request";
import { UiButton, UiPanel } from "@/ui";
import { filterGraph, kindLabels, relationLabels, statusLabels, type TopologyEdge, type TopologySnapshot } from "./topology";

const snapshot = ref<TopologySnapshot | null>(null);
const loading = ref(false);
const error = ref("");
const search = ref("");
const selectedId = ref("");
const focused = ref(false);
const zoom = ref(1);
const online = ref(navigator.onLine);
const viewport = ref<HTMLElement>();
const WIDTH = 1320, COLUMN = 264, CARD = 216;
const columnTitles = ["01 · 订阅", "02 · 分组", "03 · 节点 / 自建入站", "04 · 代理服务器", "05 · 服务器监控"];
const emptySnapshot: TopologySnapshot = { nodes: [], edges: [], warnings: [], generatedAt: "" };
const graph = computed(() => filterGraph(snapshot.value || emptySnapshot, search.value, focused.value ? selectedId.value : ""));
const selected = computed(() => snapshot.value?.nodes.find(n => n.id === selectedId.value));
const positioned = computed(() => graph.value.columns.flatMap((column, col) => column.map((n, row) => ({ ...n, x: col * COLUMN + 20, y: 56 + row * 92 }))));
const positions = computed(() => new Map(positioned.value.map(n => [n.id, n])));
const height = computed(() => Math.max(340, 76 + Math.max(...graph.value.columns.map(c => c.length), 0) * 92));
const connections = computed(() => (snapshot.value?.edges || []).filter(e => e.source === selectedId.value || e.target === selectedId.value).map(edge => {
  const incoming = edge.target === selectedId.value;
  const other = incoming ? edge.source : edge.target;
  return { edge, incoming, other, node: snapshot.value?.nodes.find(n => n.id === other) };
}));
function edgePath(edge: TopologyEdge) {
  const from = positions.value.get(edge.source)!, to = positions.value.get(edge.target)!;
  const x1 = from.x + CARD, y1 = from.y + 38, y2 = to.y + 38;
  if (from.x === to.x) { const bend = x1 + 24; return `M${x1},${y1} C${bend},${y1} ${bend},${y2} ${to.x + CARD},${y2}`; }
  const x2 = to.x, mid = (x1 + x2) / 2;
  return `M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}`;
}
function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? "未知" : date.toLocaleString(); }
function resetView() { zoom.value = 1; viewport.value?.scrollTo({ top: 0, left: 0 }); }
function selectRelated(id: string) { selectedId.value = id; search.value = ""; focused.value = true; resetView(); }
let controller: AbortController | undefined;
async function reload() {
  if (loading.value) return;
  loading.value = true; error.value = "";
  controller = new AbortController();
  try {
    // Shared authentication, expired-session handling and API base URL.
    const response = await request.get<unknown, { code: string; data: TopologySnapshot }>("/api/v1/network/topology", { signal: controller.signal, timeout: 20000 });
    if (response.code !== "00000" || !Array.isArray(response.data?.nodes) || !Array.isArray(response.data?.edges)) throw new Error("invalid response");
    snapshot.value = response.data;
    if (selectedId.value && !selected.value) { selectedId.value = ""; focused.value = false; }
  } catch (cause) {
    if (!axios.isCancel(cause)) error.value = axios.isAxiosError(cause) && cause.response?.status === 403 ? "没有读取拓扑的管理员权限。" : "无法读取最新拓扑。";
  } finally { loading.value = false; }
}
function updateOnline() { online.value = navigator.onLine; }
onMounted(() => { window.addEventListener("online", updateOnline); window.addEventListener("offline", updateOnline); reload(); });
onBeforeUnmount(() => { controller?.abort(); window.removeEventListener("online", updateOnline); window.removeEventListener("offline", updateOnline); });
</script>

<style scoped>
.topology-page { display: grid; gap: 18px; min-width: 0; max-width: 100%; color: var(--c-ink); }
.page-heading, .detail-heading { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 16px; }
h1 { font-size: 26px; margin: 4px 0 8px; } h3, p { margin: 0; } .page-heading p, .muted, .snapshot-state { color: var(--c-muted); font-size: 13px; line-height: 1.7; }
.eyebrow { font: 11px var(--font-mono); letter-spacing: .12em; }
.toolbar { display: flex; flex-wrap: wrap; align-items: center; gap: 12px; padding: 16px; border-bottom: 1px solid var(--c-line); }
.search-label { display: flex; align-items: center; gap: 10px; font-size: 13px; } input[type=search] { min-width: 160px; width: 240px; max-width: 100%; padding: 9px 12px; background: var(--c-surface-2); border: 1px solid var(--c-line); border-radius: 8px; color: var(--c-ink); }
.focus-toggle { display: flex; gap: 6px; align-items: center; font-size: 13px; } input[type=checkbox] { accent-color: var(--c-signal); } .zoom-controls { display: flex; gap: 6px; margin-left: auto; }
.legend { display: flex; flex-wrap: wrap; gap: 10px 18px; padding: 12px 16px; color: var(--c-muted); font-size: 12px; } .legend span { display: inline-flex; gap: 5px; align-items: center; } .legend i { width: 8px; height: 8px; border-radius: 2px; background: var(--type-color); }
.kind-subscription { --type-color: #9681d2; } .kind-group { --type-color: #5d9fc4; } .kind-node { --type-color: #54a28f; } .kind-inbound { --type-color: #bf9a55; } .kind-server { --type-color: #b68a6c; } .kind-monitor { --type-color: #b97c9f; }
.graph-scroll { overflow: auto; max-height: 640px; width: 100%; background: var(--c-surface-2); border-block: 1px solid var(--c-line); overscroll-behavior: contain; }
.graph-canvas { position: relative; transform-origin: top left; background-image: radial-gradient(var(--c-line) 1px, transparent 1px); background-size: 18px 18px; }
.column-title { position: absolute; top: 18px; font-size: 12px; color: var(--c-muted); } .column-title span { font-family: var(--font-mono); margin-left: 8px; }
.connections { position: absolute; inset: 0; overflow: visible; color: var(--c-muted); pointer-events: none; } .connections > path { fill: none; stroke: var(--c-muted); stroke-width: 1.2; opacity: .35; } .connections > path.relay { stroke-dasharray: 4 4; } .connections > path.highlighted { stroke: var(--c-signal); opacity: .95; stroke-width: 2; }
.node-card { position: absolute; width: 216px; height: 76px; padding: 9px 11px; text-align: left; background: var(--c-surface); border: 1px solid var(--c-line); border-left: 3px solid var(--type-color); border-radius: 9px; color: var(--c-ink); cursor: pointer; box-shadow: var(--shadow-panel); }
.node-card:hover, .node-card.selected { border-color: var(--type-color); background: var(--c-surface); } .node-card.selected { outline: 2px solid var(--type-color); outline-offset: 2px; } .node-card strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 13px; margin: 3px 0; } .node-card small { color: var(--c-muted); font: 10px var(--font-mono); } .card-meta { display: flex; justify-content: space-between; font-size: 10px; color: var(--c-muted); } .good { color: var(--c-good, var(--c-signal)); }
button:focus-visible, input:focus-visible, a:focus-visible, .graph-scroll:focus-visible { outline: 2px solid var(--c-signal); outline-offset: 3px; }
.graph-footer { padding: 12px 16px; color: var(--c-muted); font-size: 12px; line-height: 1.8; } .notice { padding: 10px 12px; background: var(--c-surface-2); border: 1px solid var(--c-line); border-radius: 8px; font-size: 13px; line-height: 1.8; overflow-wrap: anywhere; } .graph-footer .notice { margin-bottom: 10px; } .error { border-left: 3px solid var(--c-bad); }
.empty { padding: 50px 20px; text-align: center; display: grid; justify-items: center; gap: 12px; } .empty p { color: var(--c-muted); max-width: 500px; font-size: 13px; } a { color: var(--c-signal); font-size: 13px; }
.detail-heading { margin-bottom: 12px; } .detail-heading h3 { font-size: 16px; overflow-wrap: anywhere; } .detail-heading p { font-size: 12px; color: var(--c-muted); margin-top: 6px; }
.relation-list { padding: 0; list-style: none; margin: 16px 0 0; } .relation-list li { padding: 9px 0; border-top: 1px solid var(--c-line); display: flex; flex-wrap: wrap; gap: 8px 20px; font-size: 12px; } .relation-list li > span { color: var(--c-muted); } .relation-list button { color: var(--c-signal); text-align: left; overflow-wrap: anywhere; cursor: pointer; } .relation-list small { color: var(--c-muted); margin-left: 8px; }
.loading-grid { display: flex; overflow: hidden; gap: 30px; padding: 24px 20px; height: 340px; background: var(--c-surface-2); } .skeleton-column { min-width: 216px; } .skeleton-card { height: 76px; margin-bottom: 16px; border-radius: 9px; background: var(--c-line); opacity: .45; }
@media (max-width: 767px) { .toolbar { align-items: flex-start; } .search-label { width: 100%; } .search-label input { flex: 1; min-width: 0; } .zoom-controls { margin-left: 0; } .graph-scroll { max-height: 520px; } }
</style>
