<template>
  <div class="speed-page">
    <PageHero
      title="真实测速"
      desc="对节点出口做分块下载测速，按速率排名；下方为近 7 天质量时段趋势。"
    >
      <template #actions>
        <el-button :disabled="nodesLoading || loading" @click="selectAll"
          >全选可达</el-button
        >
        <el-button type="primary" :loading="loading" @click="startTest"
          >开始测速</el-button
        >
        <el-button v-if="loading" type="warning" plain @click="abortTest"
          >停止</el-button
        >
      </template>
      <template #stats>
        <StatStrip :items="speedStats" />
      </template>
    </PageHero>

    <div class="speed-layout">
      <aside class="config-panel">
        <h3>测速配置</h3>

        <div class="field-label">
          目标节点 <span>{{ selectedIds.length }} / {{ nodes.length }}</span>
        </div>
        <div class="node-chips">
          <button
            v-for="n in nodes"
            :key="n.id"
            class="chip"
            :class="{ on: selectedIds.includes(n.id) }"
            @click="toggleNode(n.id)"
          >
            {{ n.name }}<small>{{ n.rtt }}ms</small>
          </button>
        </div>
        <EmptyState
          v-if="!nodesLoading && !nodes.length"
          title="暂无可测节点"
          desc="先在节点列表完成一次质量检测，再回到这里测速。"
        />

        <div class="field-label">下载地址</div>
        <el-input v-model="target" placeholder="https://…" />

        <div class="field-label">测速大小 <b>{{ sizeMB }} MB</b></div>
        <el-slider v-model="sizeMB" :min="1" :max="100" />

        <div class="field-label">超时 <b>{{ timeoutSec }} s</b></div>
        <el-slider v-model="timeoutSec" :min="3" :max="120" />
      </aside>

      <section class="results-panel">
        <header class="results-head">
          <h3>测速结果</h3>
          <StatusPill
            v-if="loading"
            tone="live"
            :text="`进行中 ${results.length}/${selectedIds.length}`"
          />
          <StatusPill
            v-else-if="results.length"
            tone="ok"
            :text="`完成 ${results.length} 个`"
          />
          <StatusPill v-else tone="info" text="待开始" />
        </header>

        <div class="result-list" v-loading="loading">
          <article
            v-for="(row, i) in rankedResults"
            :key="row.id ?? i"
            class="result-card"
          >
            <span class="rank">#{{ i + 1 }}</span>
            <div class="result-main">
              <div class="result-name">
                {{ row.name }}
                <small>TCP {{ row.rtt >= 0 ? row.rtt + "ms" : "超时" }}</small>
              </div>
              <div class="result-bar">
                <i
                  :style="{ width: barPercent(row) + '%' }"
                  :class="row.ok ? rateClass(row.mbps) : 'fail'"
                ></i>
              </div>
            </div>
            <div class="result-value">
              <b v-if="row.ok">{{ row.mbps }}</b>
              <b v-else class="fail">—</b>
              <small>{{ row.ok ? "Mbps" : row.error ? "失败" : "等待" }}</small>
            </div>
            <div class="result-meta">
              <span v-if="row.ok"
                >{{ (row.bytes / 1048576).toFixed(1) }} MB · {{ row.seconds }}s</span
              >
              <span v-else>{{ row.error || "" }}</span>
            </div>
          </article>
          <EmptyState
            v-if="!results.length && !loading"
            title="还没有测速结果"
            desc="在左侧选择目标节点，然后点击右上角“开始测速”。"
          />
        </div>
      </section>
    </div>

    <PanelSection
      title="质量时段趋势"
      desc="按小时聚合（近 7 天），用于观察晚高峰"
    >
      <div id="qualityTrend" class="trend-chart"></div>
      <EmptyState
        v-if="!trendLoading && trendPoints.length === 0"
        title="暂无质量样本"
        desc="服务端质量采样每 10 分钟运行一次，稍后再来看看。"
      />
    </PanelSection>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from "vue";
import * as echarts from "echarts/core";
import {
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from "echarts/components";
import { LineChart } from "echarts/charts";
import { CanvasRenderer } from "echarts/renderers";
import { getNodeOverview } from "@/api/subcription/node";

defineOptions({ name: "SpeedTest" });

echarts.use([
  TooltipComponent,
  GridComponent,
  LegendComponent,
  LineChart,
  CanvasRenderer,
]);

interface NodeItem {
  id: number;
  name: string;
  link: string;
  rtt: number;
}

const nodes = ref<NodeItem[]>([]);
const nodesLoading = ref(true);
const selectedIds = ref<number[]>([]);
const target = ref("https://speed.cloudflare.com/__down?bytes=10000000");
const sizeMB = ref(10);
const timeoutSec = ref(20);
const loading = ref(false);
const results = ref<any[]>([]);
const abortCtrl = ref<AbortController | null>(null);

const trendPoints = ref<any[]>([]);
const trendLoading = ref(false);
let trendChart: any = null;

const getToken = () => {
  try {
    return localStorage.getItem("accessToken") || "";
  } catch {
    return "";
  }
};

const parseSSE = async (
  res: Response,
  onEvent: (type: string, data: any) => void
) => {
  const reader = res.body?.getReader();
  if (!reader) return;
  const decoder = new TextDecoder();
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const events = buf.split("\n\n");
    buf = events.pop() || "";
    for (const ev of events) {
      const type = ev.match(/^event:\s*(\S+)/m)?.[1] || "";
      const dataLine = ev.match(/^data:\s*(.*)$/m);
      if (!dataLine) continue;
      try {
        onEvent(type, JSON.parse(dataLine[1]));
      } catch {
        /* ignore */
      }
    }
  }
};

const selectAll = () => {
  selectedIds.value = nodes.value
    .filter((n) => n.rtt >= 0)
    .map((n) => n.id);
};

const toggleNode = (id: number) => {
  const index = selectedIds.value.indexOf(id);
  if (index >= 0) selectedIds.value.splice(index, 1);
  else selectedIds.value.push(id);
};

const rankedResults = computed(() => {
  const ok = results.value
    .filter((r) => r.ok)
    .slice()
    .sort((a, b) => (b.mbps || 0) - (a.mbps || 0));
  const rest = results.value.filter((r) => !r.ok);
  return [...ok, ...rest];
});

const bestMbps = computed(() =>
  Math.max(0, ...results.value.filter((r) => r.ok).map((r) => r.mbps || 0))
);

const barPercent = (row: any) => {
  if (!row.ok || !bestMbps.value) return 0;
  return Math.max(4, Math.round(((row.mbps || 0) / bestMbps.value) * 100));
};

const rateClass = (mbps: number) =>
  mbps >= 20 ? "good" : mbps >= 5 ? "warn" : "bad";

const speedStats = computed(() => {
  const done = results.value.filter((r) => r.ok);
  const best = done.length
    ? Math.max(...done.map((r) => r.mbps || 0))
    : 0;
  const avg = done.length
    ? Math.round(done.reduce((a, r) => a + (r.mbps || 0), 0) / done.length)
    : 0;
  return [
    { label: "已选节点", value: selectedIds.value.length },
    { label: "最快", value: best, unit: "Mbps", tone: "ok" as const },
    { label: "平均", value: avg, unit: "Mbps" },
    { label: "结果数", value: results.value.length },
  ];
});

const startTest = async () => {
  if (!selectedIds.value.length) {
    ElMessage.warning("请先选择节点");
    return;
  }
  loading.value = true;
  results.value = [];
  abortCtrl.value = new AbortController();
  const payload = new URLSearchParams();
  payload.append("ids", selectedIds.value.join(","));
  payload.append("target", target.value);
  payload.append("size", String(sizeMB.value * 1048576));
  payload.append("timeout", String(timeoutSec.value));
  try {
    const res = await fetch(
      import.meta.env.VITE_APP_BASE_API + "/api/v1/nodes/speedtest/stream",
      {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          Authorization: getToken(),
        },
        body: payload.toString(),
        signal: abortCtrl.value.signal,
      }
    );
    if (!res.ok) {
      const text = await res.text();
      let msg = "请求失败: " + res.status;
      try {
        msg = JSON.parse(text).msg || msg;
      } catch {
        /* ignore */
      }
      ElMessage.error(msg);
      return;
    }
    await parseSSE(res, (type, data) => {
      if (type === "node") results.value.push(data);
      else if (type === "done") ElMessage.success("测速完成");
    });
  } catch (e: any) {
    if (e?.name !== "AbortError") ElMessage.error(e?.message || "测速失败");
  } finally {
    loading.value = false;
    abortCtrl.value = null;
  }
};

const abortTest = () => {
  abortCtrl.value?.abort();
  loading.value = false;
};

const cssVar = (name: string, fallback: string) =>
  getComputedStyle(document.documentElement).getPropertyValue(name).trim() ||
  fallback;

const loadTrend = async () => {
  trendLoading.value = true;
  try {
    const res = await fetch(
      import.meta.env.VITE_APP_BASE_API +
        "/api/v1/nodes/quality/trend?hours=168",
      { headers: { Authorization: getToken() } }
    );
    const json = await res.json();
    trendPoints.value = json?.data || [];
  } catch {
    trendPoints.value = [];
  } finally {
    trendLoading.value = false;
  }
  await nextTick();
  renderTrend();
};

const renderTrend = () => {
  const el = document.getElementById("qualityTrend") as HTMLDivElement;
  if (!el || !trendPoints.value.length) return;
  if (!trendChart) trendChart = echarts.init(el);
  const accent = cssVar("--ui-accent", "#e8960f");
  const muted = cssVar("--ui-text-muted", "#9a9081");
  const border = cssVar("--ui-border", "#ecdfc8");
  const hours = trendPoints.value.map((p) => p.hour);
  const availability = trendPoints.value.map((p) =>
    p.samples ? Math.round((p.successes / p.samples) * 1000) / 10 : 0
  );
  const rtt = trendPoints.value.map((p) => Math.round(p.avgRtt || 0));
  trendChart.setOption(
    {
      color: [accent, cssVar("--ui-text-secondary", "#6b6252")],
      tooltip: {
        trigger: "axis",
        backgroundColor: cssVar("--ui-surface-strong", "#fff"),
        borderColor: border,
        textStyle: { color: cssVar("--ui-text", "#241f16") },
      },
      legend: {
        data: ["可用率(%)", "平均延迟(ms)"],
        top: 0,
        textStyle: { color: muted },
      },
      grid: { left: 50, right: 50, top: 40, bottom: 30 },
      xAxis: {
        type: "category",
        data: hours,
        axisLine: { lineStyle: { color: border } },
        axisLabel: { color: muted, formatter: (v: string) => v?.slice(5) },
      },
      yAxis: [
        {
          type: "value",
          name: "可用率%",
          min: 0,
          max: 100,
          axisLabel: { color: muted },
          splitLine: { lineStyle: { color: border, opacity: 0.5 } },
        },
        {
          type: "value",
          name: "ms",
          axisLabel: { color: muted },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: "可用率(%)",
          type: "line",
          smooth: true,
          symbol: "none",
          data: availability,
          areaStyle: { opacity: 0.12 },
        },
        {
          name: "平均延迟(ms)",
          type: "line",
          smooth: true,
          symbol: "none",
          yAxisIndex: 1,
          data: rtt,
        },
      ],
    },
    true
  );
  window.addEventListener("resize", () => trendChart?.resize());
};

onMounted(async () => {
  try {
    const { data } = await getNodeOverview();
    nodes.value = (data || [])
      .filter((n: NodeItem) => n.rtt >= 0)
      .sort((a: NodeItem, b: NodeItem) => a.rtt - b.rtt);
  } catch {
    nodes.value = [];
  } finally {
    nodesLoading.value = false;
  }
  await loadTrend();
});
</script>

<style lang="scss" scoped>
.speed-layout {
  display: grid;
  grid-template-columns: 340px minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

/* ---------- config panel ---------- */
.config-panel {
  position: sticky;
  top: 12px;
  display: flex;
  flex-direction: column;
  padding: 18px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-lg);
  background: var(--ui-surface-strong);
  box-shadow: 0 10px 30px color-mix(in srgb, var(--ui-accent-shadow) 30%, transparent);

  h3 {
    margin: 0 0 12px;
    color: var(--ui-text);
    font-size: 14px;
    font-weight: 700;
  }
}

.field-label {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 14px 0 7px;
  color: var(--ui-text-secondary);
  font-size: 12px;
  font-weight: 600;

  span,
  b {
    color: var(--ui-accent-strong);
    font-family: var(--font-mono);
    font-size: 11.5px;
  }
}

.node-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  max-height: 190px;
  overflow-y: auto;
  padding-right: 2px;
}

.chip {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  padding: 6px 10px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-pill);
  background: var(--ui-surface);
  color: var(--ui-text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background var(--dur-fast) ease,
    color var(--dur-fast) ease,
    border-color var(--dur-fast) ease,
    transform var(--dur-mid) var(--ease-spring);

  small {
    color: var(--ui-text-muted);
    font-family: var(--font-mono);
    font-size: 10px;
  }

  &:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--ui-accent) 40%, var(--ui-border));
  }

  &.on {
    border-color: color-mix(in srgb, var(--ui-accent) 55%, transparent);
    background: var(--ui-accent-soft);
    color: var(--ui-accent-strong);
    box-shadow: 0 4px 14px color-mix(in srgb, var(--ui-accent-shadow) 60%, transparent);

    small {
      color: var(--ui-accent-strong);
    }
  }
}

/* ---------- results ---------- */
.results-panel {
  min-width: 0;
  padding: 18px;
  border: 1px solid var(--ui-border);
  border-radius: var(--r-lg);
  background: var(--ui-surface-strong);
  box-shadow: 0 10px 30px color-mix(in srgb, var(--ui-accent-shadow) 30%, transparent);
}

.results-head {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;

  h3 {
    margin: 0;
    color: var(--ui-text);
    font-size: 14px;
    font-weight: 700;
  }
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-height: 320px;
}

.result-card {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr) 86px minmax(120px, 170px);
  gap: 12px;
  align-items: center;
  padding: 12px 14px;
  border: 1px solid color-mix(in srgb, var(--ui-border) 80%, transparent);
  border-radius: var(--r-md);
  background: var(--ui-surface);
  transition:
    transform var(--dur-mid) var(--ease-out),
    box-shadow var(--dur-mid) ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: 0 12px 26px color-mix(in srgb, var(--ui-accent-shadow) 45%, transparent);
  }
}

.rank {
  color: var(--ui-text-muted);
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 700;
}

.result-main {
  min-width: 0;
}

.result-name {
  display: flex;
  align-items: baseline;
  gap: 8px;
  overflow: hidden;
  color: var(--ui-text);
  font-size: 13px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;

  small {
    color: var(--ui-text-muted);
    font-family: var(--font-mono);
    font-size: 10.5px;
    font-weight: 500;
  }
}

.result-bar {
  height: 5px;
  margin-top: 8px;
  overflow: hidden;
  border-radius: var(--r-pill);
  background: color-mix(in srgb, var(--ui-border) 70%, transparent);

  i {
    display: block;
    height: 100%;
    border-radius: var(--r-pill);
    transition: width 700ms var(--ease-out);

    &.good {
      background: var(--ui-accent);
    }

    &.warn {
      background: var(--el-color-warning);
    }

    &.bad,
    &.fail {
      background: var(--el-color-danger);
    }
  }
}

.result-value {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  line-height: 1.1;

  b {
    color: var(--ui-text);
    font-family: var(--font-mono);
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.02em;

    &.fail {
      color: var(--ui-text-muted);
    }
  }

  small {
    color: var(--ui-text-muted);
    font-size: 10px;
  }
}

.result-meta {
  overflow: hidden;
  color: var(--ui-text-muted);
  font-size: 11px;
  text-align: right;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ---------- trend ---------- */
.trend-chart {
  width: 100%;
  height: 320px;
}

/* ---------- responsive ---------- */
@media (max-width: 960px) {
  .speed-layout {
    grid-template-columns: 1fr;
  }

  .config-panel {
    position: static;
  }
}

@media (max-width: 720px) {
  .result-card {
    grid-template-columns: 26px minmax(0, 1fr) 70px;
  }

  .result-meta {
    display: none;
  }
}
</style>
