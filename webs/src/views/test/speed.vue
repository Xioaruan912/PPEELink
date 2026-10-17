<template>
  <div class="speedtest-page" ref="speedPageRef">
    <!-- 头部 Hero -->
    <header class="speed-hero">
      <div class="hero-copy">
        <span class="hero-eyebrow">HIGH-PRECISION BENCHMARK</span>
        <h1>真实节点出口测速</h1>
        <p>
          通过服务端建立代理隧道，并发测量目标节点到真实公共 CDN (Cloudflare) 的
          TCP 建立延迟、真实下载速度与耗时。
        </p>
      </div>

      <div class="hero-stats" v-if="results.length">
        <div class="stat-pill">
          <span class="pill-label">测试节点</span>
          <span class="pill-val font-mono">{{ results.length }}</span>
        </div>
        <div class="stat-pill">
          <span class="pill-label">峰值吞吐</span>
          <span class="pill-val font-mono text-cyan"
            >{{ maxThroughput }} <small>Mbps</small></span
          >
        </div>
        <div class="stat-pill">
          <span class="pill-label">平均速率</span>
          <span class="pill-val font-mono"
            >{{ avgThroughput }} <small>Mbps</small></span
          >
        </div>
      </div>
    </header>

    <!-- 控制面板 Bento Grid -->
    <section class="control-panel">
      <!-- 目标节点选择 -->
      <div class="control-block node-picker spotlight-card">
        <div class="block-title">
          <span>选择节点</span>
          <button
            class="select-all-btn"
            :disabled="nodesLoading"
            @click="selectAll"
          >
            选择全部可用节点 ({{ reachableNodesCount }})
          </button>
        </div>
        <el-select
          v-model="selectedIds"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          placeholder="请在下拉框中选择要测试的节点…"
          class="custom-select"
          :loading="nodesLoading"
        >
          <el-option
            v-for="n in nodes"
            :key="n.id"
            :label="n.name"
            :value="n.id"
          >
            <div class="option-row">
              <span class="opt-name">{{ n.name }}</span>
              <span class="opt-rtt" :class="rttClass(n.rtt)">{{
                n.rtt >= 0 ? n.rtt + "ms" : "超时"
              }}</span>
            </div>
          </el-option>
        </el-select>
      </div>
    </section>
    <details class="speed-advanced">
      <summary>更多测速设置</summary>
      <section class="control-panel">
        <!-- 测速目标 URL -->
        <div class="control-block target-picker spotlight-card">
          <div class="block-title">
            <span>测速地址</span>
            <button
              class="reset-target-btn"
              @click="
                target = 'https://speed.cloudflare.com/__down?bytes=10000000'
              "
            >
              恢复默认
            </button>
          </div>
          <el-input
            v-model="target"
            placeholder="https://speed.cloudflare.com/__down?bytes=10000000"
            class="custom-input"
          />
        </div>

        <!-- 样本大小快捷选择 -->
        <div class="control-block size-picker spotlight-card">
          <div class="block-title">
            <span>测试下载量</span>
            <span class="current-label">{{ sizeMB }} MB</span>
          </div>
          <div class="preset-chips">
            <button
              v-for="s in [10, 25, 50, 100]"
              :key="s"
              class="chip-btn"
              :class="{ active: sizeMB === s }"
              @click="setSize(s)"
            >
              {{ s }} MB
            </button>
          </div>
        </div>

        <!-- 超时控制 -->
        <div class="control-block timeout-picker spotlight-card">
          <div class="block-title">
            <span>最长等待时间</span>
            <span class="current-label">{{ timeoutSec }} 秒</span>
          </div>
          <div class="preset-chips">
            <button
              v-for="t in [10, 20, 30, 60]"
              :key="t"
              class="chip-btn"
              :class="{ active: timeoutSec === t }"
              @click="timeoutSec = t"
            >
              {{ t }}s
            </button>
          </div>
        </div>
      </section>
    </details>

    <!-- 操作执行条 -->
    <div class="action-bar">
      <div class="run-info">
        <template v-if="loading">
          <div class="progress-pulse"></div>
          <span
            >正在测速中… 已完成 {{ results.length }} /
            {{ selectedIds.length }} 个节点</span
          >
        </template>
        <template v-else>
          <span class="ready-dot"></span>
          <span>已选择 {{ selectedIds.length }} 个节点</span>
        </template>
      </div>

      <div class="btn-group">
        <button v-if="loading" class="abort-btn" @click="abortTest">
          中断测速
        </button>
        <button
          class="launch-btn"
          :disabled="loading || !selectedIds.length"
          @click="startTest"
        >
          <span v-if="loading" class="spin-icon">测量中…</span>
          <span v-else>开始测速</span>
        </button>
      </div>
    </div>

    <!-- 实时测速结果列表 -->
    <section class="results-card spotlight-card">
      <div class="card-head">
        <div class="title-with-count">
          <h3>测速结果</h3>
          <span class="count-badge" v-if="results.length"
            >{{ results.length }} 条结果</span
          >
        </div>
      </div>

      <el-table
        :data="sortedResults"
        row-key="name"
        size="small"
        class="modern-table"
        v-loading="loading"
      >
        <el-table-column type="index" width="55" label="#" align="center" />
        <el-table-column
          prop="name"
          label="节点名称"
          min-width="260"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <span class="node-name-text">{{ row.name }}</span>
          </template>
        </el-table-column>
        <el-table-column label="连接响应" width="120">
          <template #default="{ row }">
            <span class="tcp-pill" :class="rttClass(row.rtt)">
              {{ row.rtt >= 0 ? row.rtt + " ms" : "超时" }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="下载速度" width="160">
          <template #default="{ row }">
            <div v-if="row.ok" class="mbps-badge" :class="rateType(row.mbps)">
              <span class="mbps-val font-mono">{{ row.mbps }}</span>
              <span class="mbps-unit">Mbps</span>
            </div>
            <span v-else-if="row.error" class="fail-badge">测试失败</span>
            <span v-else class="text-muted">-</span>
          </template>
        </el-table-column>
        <el-table-column label="下载量" width="120">
          <template #default="{ row }">
            <span v-if="row.ok" class="font-mono text-muted">
              {{ (row.bytes / 1048576).toFixed(1) }} MB
            </span>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column label="耗时" width="100">
          <template #default="{ row }">
            <span v-if="row.seconds" class="font-mono text-muted"
              >{{ row.seconds }}s</span
            >
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column
          prop="error"
          label="状态说明"
          min-width="200"
          show-overflow-tooltip
        >
          <template #default="{ row }">
            <span v-if="row.error" class="error-text">{{ row.error }}</span>
            <span v-else-if="row.ok" class="success-text">✓ 连接正常</span>
            <span v-else class="text-muted">待测</span>
          </template>
        </el-table-column>
      </el-table>

      <div v-if="!loading && !results.length" class="empty-placeholder">
        <span class="empty-icon"><WorkIcon name="pulse" /></span>
        <p>选择节点后，点击「开始测速」。</p>
      </div>
    </section>

    <!-- 质量时段趋势图 -->
    <section class="trend-card spotlight-card">
      <div class="card-head">
        <div>
          <h3>近 7 天连接趋势</h3>
        </div>
      </div>
      <div id="qualityTrend" class="trend-chart"></div>
      <div
        v-if="!trendLoading && trendPoints.length === 0"
        class="empty-placeholder"
      >
        <p>暂无历史质量采样数据</p>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from "vue";
import { ElMessage } from "element-plus";
import WorkIcon from "@/components/legacy/WorkIcon.vue";
import * as echarts from "echarts/core";
import {
  TooltipComponent,
  GridComponent,
  LegendComponent,
} from "echarts/components";
import { LineChart } from "echarts/charts";
import { CanvasRenderer } from "echarts/renderers";
import { getNodeOverview } from "@/api/subcription/node";
import { initSpotlight, useLegacyStagger } from "@/motion/compat";
const staggerReveal = useLegacyStagger();

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
const speedPageRef = ref<HTMLElement | null>(null);
let cleanupSpotlight: (() => void) | null = null;

const trendPoints = ref<any[]>([]);
const trendLoading = ref(false);
let trendChart: any = null;

const reachableNodesCount = computed(
  () => nodes.value.filter((n) => n.rtt >= 0).length
);

const sortedResults = computed(() => {
  return [...results.value].sort((a, b) => (b.mbps || 0) - (a.mbps || 0));
});

const maxThroughput = computed(() => {
  if (!results.value.length) return "0";
  const max = Math.max(...results.value.map((r) => r.mbps || 0));
  return max > 0 ? max.toFixed(1) : "0";
});

const avgThroughput = computed(() => {
  const valid = results.value.filter((r) => r.ok && r.mbps > 0);
  if (!valid.length) return "0";
  const total = valid.reduce((acc, curr) => acc + curr.mbps, 0);
  return (total / valid.length).toFixed(1);
});

const setSize = (mb: number) => {
  sizeMB.value = mb;
  // 同步对齐 target URL 中的 bytes 参数
  if (target.value.includes("speed.cloudflare.com")) {
    const bytes = mb >= 100 ? 99999999 : mb * 1048576;
    target.value = `https://speed.cloudflare.com/__down?bytes=${bytes}`;
  }
};

const rateType = (mbps: number) => {
  if (mbps >= 50) return "rate-blazing";
  if (mbps >= 20) return "rate-fast";
  if (mbps >= 5) return "rate-moderate";
  return "rate-slow";
};

const rttClass = (rtt: number) => {
  if (rtt < 0) return "rtt-timeout";
  if (rtt < 100) return "rtt-fast";
  if (rtt < 250) return "rtt-mid";
  return "rtt-slow";
};

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
  selectedIds.value = nodes.value.filter((n) => n.rtt >= 0).map((n) => n.id);
};

const startTest = async () => {
  if (!selectedIds.value.length) {
    ElMessage.warning("请先勾选需要测速的节点");
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
      else if (type === "done") ElMessage.success("测速矩阵测试完成");
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
  ElMessage.info("已中断测试");
};

const loadTrend = async () => {
  trendLoading.value = true;
  try {
    const res = await fetch(
      import.meta.env.VITE_APP_BASE_API +
        "/api/v1/nodes/quality/trend?hours=168",
      {
        headers: { Authorization: getToken() },
      }
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
  if (!el) return;
  if (!trendChart) trendChart = echarts.init(el);
  const hours = trendPoints.value.map((p) => p.hour);
  const availability = trendPoints.value.map((p) =>
    p.samples ? Math.round((p.successes / p.samples) * 1000) / 10 : 0
  );
  const rtt = trendPoints.value.map((p) => Math.round(p.avgRtt || 0));

  trendChart.setOption(
    {
      tooltip: {
        trigger: "axis",
        backgroundColor: "rgba(19, 25, 38, 0.95)",
        borderColor: "rgba(255, 255, 255, 0.1)",
        textStyle: { color: "#f1f5f9" },
      },
      legend: {
        data: ["可用率(%)", "平均延迟(ms)"],
        textStyle: { color: "#94a3b8" },
      },
      grid: { left: 45, right: 45, top: 40, bottom: 25 },
      xAxis: {
        type: "category",
        data: hours,
        axisLine: { lineStyle: { color: "rgba(255, 255, 255, 0.1)" } },
        axisLabel: { color: "#64748b" },
      },
      yAxis: [
        {
          type: "value",
          name: "可用率%",
          min: 0,
          max: 100,
          splitLine: { lineStyle: { color: "rgba(255, 255, 255, 0.05)" } },
          axisLabel: { color: "#64748b" },
        },
        {
          type: "value",
          name: "延迟 (ms)",
          splitLine: { show: false },
          axisLabel: { color: "#64748b" },
        },
      ],
      series: [
        {
          name: "可用率(%)",
          type: "line",
          smooth: true,
          data: availability,
          itemStyle: { color: "#10b981" },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: "rgba(16, 185, 129, 0.25)" },
              { offset: 1, color: "rgba(16, 185, 129, 0)" },
            ]),
          },
        },
        {
          name: "平均延迟(ms)",
          type: "line",
          smooth: true,
          yAxisIndex: 1,
          data: rtt,
          itemStyle: { color: "#6d9370" },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: "rgba(56, 189, 248, 0.2)" },
              { offset: 1, color: "rgba(56, 189, 248, 0)" },
            ]),
          },
        },
      ],
    },
    true
  );
  window.addEventListener("resize", () => trendChart?.resize());
};

onMounted(async () => {
  if (speedPageRef.value) {
    cleanupSpotlight = initSpotlight(speedPageRef.value);
  }
  staggerReveal(".control-block, .results-card, .trend-card", 0.05, 0.05);
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

onBeforeUnmount(() => {
  if (cleanupSpotlight) cleanupSpotlight();
  if (trendChart) {
    trendChart.dispose();
    trendChart = null;
  }
});
</script>

<style lang="scss" scoped>
.speedtest-page {
  width: min(1200px, 100%);
  margin: 0 auto;
  padding: 32px 28px 60px;
  color: var(--ui-text);
}

.speed-hero {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  gap: 24px;
  margin-bottom: 28px;
  flex-wrap: wrap;
}

.hero-eyebrow {
  display: inline-block;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 15px;
  font-weight: 800;
  letter-spacing: 0.16em;
  color: var(--ui-accent);
  margin-bottom: 8px;
}

.hero-copy h1 {
  font-size: clamp(28px, 4vw, 36px);
  font-weight: 750;
  line-height: 1.15;
  letter-spacing: -0.03em;
  margin: 0 0 10px;
  color: var(--ui-text);
}

.hero-copy p {
  margin: 0;
  color: var(--ui-text-secondary);
  font-size: 14px;
  max-width: 660px;
  line-height: 1.5;
}

.hero-stats {
  display: flex;
  gap: 12px;
}

.stat-pill {
  display: flex;
  flex-direction: column;
  padding: 12px 18px;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border);
  border-radius: 14px;
  box-shadow: var(--ui-panel-shadow);

  .pill-label {
    font-size: 15px;
    color: var(--ui-text-muted);
    font-weight: 600;
    margin-bottom: 4px;
  }
  .pill-val {
    font-size: 22px;
    font-weight: 800;
    color: var(--ui-text);
    line-height: 1;

    small {
      font-size: 15px;
      color: var(--ui-text-muted);
      font-weight: 500;
    }
  }
  .text-cyan {
    color: var(--ui-accent);
  }
}

/* 控制台 Bento Grid */
.control-panel {
  display: grid;
  grid-template-columns: 1.4fr 1.6fr 1fr 1fr;
  gap: 16px;
  margin-bottom: 18px;
}

.control-block {
  display: flex;
  flex-direction: column;
  padding: 16px 18px;
  background: var(--ui-surface);
  border: 1px solid var(--ui-border);
  border-radius: 16px;
  box-shadow: var(--ui-panel-shadow);
}

.block-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
  font-size: 16px;
  font-weight: 700;
  color: var(--ui-text-secondary);

  .current-label {
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: var(--ui-accent);
  }
}

.select-all-btn,
.reset-target-btn {
  background: transparent;
  border: none;
  font-size: 15px;
  color: var(--ui-accent);
  font-weight: 600;
  cursor: pointer;
  padding: 0;

  &:hover {
    text-decoration: underline;
  }
  &:disabled {
    color: var(--ui-text-muted);
    cursor: not-allowed;
  }
}

.custom-select,
.custom-input {
  width: 100%;

  :deep(.el-input__wrapper) {
    background: var(--ui-canvas);
    border: 1px solid var(--ui-border);
    border-radius: 10px;
    box-shadow: none;
    padding: 4px 10px;

    &.is-focus {
      border-color: var(--ui-accent);
      box-shadow: 0 0 0 3px var(--ui-focus-ring);
    }
  }
}

.option-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;

  .opt-name {
    font-size: 16px;
    font-weight: 500;
    max-width: 220px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .opt-rtt {
    font-size: 15px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    padding: 1px 6px;
    border-radius: 4px;
  }
}

.preset-chips {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 6px;
}

.chip-btn {
  padding: 8px 4px;
  border: 1px solid var(--ui-border);
  background: var(--ui-canvas);
  border-radius: 8px;
  color: var(--ui-text-secondary);
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  transition: all 140ms ease;
  text-align: center;

  &:hover {
    color: var(--ui-text);
    border-color: var(--ui-accent);
  }

  &.active {
    background: var(--ui-accent);
    color: #ffffff;
    border-color: var(--ui-accent);
    box-shadow: 0 2px 8px var(--ui-accent-shadow);
  }
}

/* 操作执行条 */
.action-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  background: var(--ui-surface-strong);
  border: 1px solid var(--ui-border);
  border-radius: 14px;
  margin-bottom: 24px;
}

.run-info {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 16px;
  color: var(--ui-text-secondary);
}

.ready-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ui-success);
  box-shadow: 0 0 0 3px var(--ui-success-soft);
}

.progress-pulse {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ui-accent);
  animation: pulse-ring 1.5s cubic-bezier(0.215, 0.61, 0.355, 1) infinite;
}

@keyframes pulse-ring {
  0% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 var(--ui-focus-ring);
  }
  70% {
    transform: scale(1.1);
    box-shadow: 0 0 0 8px transparent;
  }
  100% {
    transform: scale(0.95);
    box-shadow: 0 0 0 0 transparent;
  }
}

.btn-group {
  display: flex;
  gap: 10px;
}

.launch-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 10px 24px;
  background: var(--ui-accent);
  color: #ffffff;
  border: none;
  border-radius: 10px;
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  transition: all 160ms ease;
  box-shadow: 0 4px 14px var(--ui-accent-shadow);

  &:hover:not(:disabled) {
    transform: translateY(-1px);
    filter: brightness(1.1);
  }
  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
    box-shadow: none;
  }
}

.abort-btn {
  padding: 10px 18px;
  background: var(--ui-danger-soft);
  color: var(--ui-danger);
  border: 1px solid var(--ui-danger);
  border-radius: 10px;
  font-size: 16px;
  font-weight: 700;
  cursor: pointer;
  transition: all 140ms ease;

  &:hover {
    background: var(--ui-danger);
    color: #ffffff;
  }
}

/* 结果列表卡片 */
.results-card,
.trend-card {
  background: var(--ui-surface);
  border: 1px solid var(--ui-border);
  border-radius: 18px;
  padding: 22px;
  margin-bottom: 24px;
  box-shadow: var(--ui-panel-shadow);
}

.card-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  margin-bottom: 18px;
}

.title-with-count {
  display: flex;
  align-items: center;
  gap: 10px;

  h3 {
    margin: 0;
    font-size: 18px;
    font-weight: 750;
    color: var(--ui-text);
  }
}

.count-badge {
  font-size: 15px;
  font-weight: 700;
  color: var(--ui-accent);
  background: var(--ui-accent-soft);
  padding: 2px 8px;
  border-radius: 999px;
}

.sub-muted {
  font-size: 16px;
  color: var(--ui-text-muted);
}

.modern-table {
  background: transparent;

  :deep(.el-table__header-wrapper) th {
    background: var(--ui-canvas);
    color: var(--ui-text-muted);
    font-size: 16px;
    font-weight: 700;
    border-bottom: 1px solid var(--ui-border);
  }

  :deep(.el-table__row) td {
    background: transparent;
    border-bottom: 1px solid var(--ui-border-subtle);
  }
}

.node-name-text {
  font-weight: 600;
  color: var(--ui-text);
  font-size: 16px;
}

.tcp-pill {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 15px;
  padding: 2px 8px;
  border-radius: 6px;
  font-weight: 600;

  &.rtt-fast {
    background: var(--ui-success-soft);
    color: var(--ui-success);
  }
  &.rtt-mid {
    background: var(--ui-warning-soft);
    color: var(--ui-warning);
  }
  &.rtt-slow {
    background: var(--ui-danger-soft);
    color: var(--ui-danger);
  }
  &.rtt-timeout {
    background: var(--ui-hover);
    color: var(--ui-text-muted);
  }
}

.mbps-badge {
  display: inline-flex;
  align-items: baseline;
  gap: 4px;
  padding: 3px 10px;
  border-radius: 8px;

  .mbps-val {
    font-size: 15px;
    font-weight: 800;
  }
  .mbps-unit {
    font-size: 14px;
    font-weight: 600;
  }

  &.rate-blazing {
    background: rgba(16, 185, 129, 0.15);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }
  &.rate-fast {
    background: var(--ui-accent-soft);
    color: var(--ui-accent);
    border: 1px solid var(--ui-accent);
  }
  &.rate-moderate {
    background: var(--ui-warning-soft);
    color: var(--ui-warning);
  }
  &.rate-slow {
    background: var(--ui-danger-soft);
    color: var(--ui-danger);
  }
}

.fail-badge {
  font-size: 15px;
  color: var(--ui-danger);
  font-weight: 600;
}

.success-text {
  font-size: 16px;
  color: var(--ui-success);
}

.error-text {
  font-size: 16px;
  color: var(--ui-danger);
}

.text-muted {
  color: var(--ui-text-muted);
}

.empty-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
  color: var(--ui-text-muted);

  .empty-icon {
    font-size: 32px;
    margin-bottom: 8px;
  }
  p {
    font-size: 16px;
    margin: 0;
  }
}

.trend-chart {
  width: 100%;
  height: 320px;
}

@media (max-width: 900px) {
  .control-panel {
    grid-template-columns: 1fr 1fr;
  }
}

@media (max-width: 600px) {
  .speedtest-page {
    padding: 20px 14px;
  }
  .control-panel {
    grid-template-columns: 1fr;
  }
  .hero-stats {
    width: 100%;
    justify-content: space-between;
  }
  .action-bar {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }
  .btn-group {
    width: 100%;
  }
  .launch-btn,
  .abort-btn {
    flex: 1;
    text-align: center;
    justify-content: center;
  }
}
</style>
