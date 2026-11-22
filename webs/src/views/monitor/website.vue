<template>
  <div ref="rootEl" class="traffic-gallery" :aria-busy="state.status === 'loading'">
    <!-- Pure Gallery Masthead -->
    <header class="traffic-masthead" aria-labelledby="traffic-title">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 id="traffic-title" class="gallery-title" data-title-reveal>
            流量排行
          </h1>
        </div>
        <span v-if="state.data" class="live-pill mono">
          <i aria-hidden="true"></i>
          {{ filteredItems.length }} 项记录 · {{ windowLabel }}
        </span>
      </div>

      <div class="masthead-actions">
        <UiButton
          variant="ghost"
          :icon="Refresh"
          :loading="state.status === 'loading'"
          :disabled="Boolean(disabledReason)"
          @click="load"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Notices (Only when offline or error) -->
    <div v-if="offline" class="inline-notice" role="status">
      <StatusDot state="warn" label="当前离线，恢复网络后可刷新最新流量。" />
    </div>
    <div v-if="state.status === 'error'" class="inline-notice" role="alert">
      <StatusDot state="bad" label="流量数据加载失败，请重试。" />
      <UiButton size="sm" variant="ghost" :disabled="Boolean(disabledReason)" @click="load">
        重试
      </UiButton>
    </div>

    <!-- Hairline 3-Column KPI Strip -->
    <dl v-if="state.data" class="kpi-strip">
      <div class="kpi-cell is-primary">
        <dt>总流量</dt>
        <dd class="mono">{{ formatBytes(state.data.summary.total) }}</dd>
      </div>
      <div class="kpi-cell">
        <dt>下载下行</dt>
        <dd class="mono">{{ formatBytes(state.data.summary.down) }}</dd>
      </div>
      <div class="kpi-cell">
        <dt>上传上行</dt>
        <dd class="mono">{{ formatBytes(state.data.summary.up) }}</dd>
      </div>
    </dl>

    <!-- Single-Line Control Bar: Window + Sort + Search -->
    <div class="control-bar">
      <div class="pill-groups">
        <div class="pill-group" role="group" aria-label="时间范围">
          <button
            v-for="opt in windowOptions"
            :key="opt.hours"
            type="button"
            class="seg-pill"
            :class="{ 'is-active': windowHours === opt.hours }"
            @click="setWindow(opt.hours)"
          >
            {{ opt.label }}
          </button>
        </div>

        <span class="bar-divider" aria-hidden="true"></span>

        <div class="pill-group" role="group" aria-label="排序方式">
          <button
            v-for="opt in orderOptions"
            :key="opt.value"
            type="button"
            class="seg-pill"
            :class="{ 'is-active': sortOrder === opt.value }"
            @click="setOrder(opt.value)"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <label class="search-wrap">
        <el-icon class="search-icon"><Search /></el-icon>
        <input
          v-model="keyword"
          type="search"
          class="search-input"
          placeholder="搜索域名或节点..."
          aria-label="搜索域名或节点"
        />
      </label>
    </div>

    <!-- Loading Skeleton -->
    <div
      v-if="state.status === 'loading' && !state.data"
      class="loading-wrap"
      role="status"
      aria-label="正在加载流量排行"
    >
      <UiSkeleton :lines="6" />
    </div>

    <!-- Empty State -->
    <div
      v-else-if="state.data && !filteredItems.length"
      class="empty-gallery"
      role="status"
    >
      <p class="empty-title">
        {{ keyword.trim() ? "未匹配到相关域名" : "当前时间窗口暂无代理流量" }}
      </p>
      <UiButton v-if="keyword.trim()" variant="ghost" @click="keyword = ''">
        清空搜索
      </UiButton>
    </div>

    <!-- Full-Width Pure Gallery Rows (No 5-Row Collapse, Click Any Row for Detail Card) -->
    <div v-else-if="state.data" class="traffic-list" role="list">
      <button
        v-for="(item, idx) in filteredItems"
        :key="`${item.serverId}:${item.domain}`"
        type="button"
        class="traffic-row"
        :class="{
          'is-selected':
            selectedItem?.domain === item.domain &&
            selectedItem?.serverId === item.serverId,
        }"
        role="listitem"
        @click="openDetail(item)"
      >
        <span class="row-rank mono">{{ String(idx + 1).padStart(2, "0") }}</span>

        <div class="row-main">
          <div class="row-domain-line">
            <strong class="row-domain">{{ displayDomain(item.domain) }}</strong>
            <span class="row-cat">{{ classifyTraffic(item).tag }}</span>
          </div>
          <span class="row-node">{{ item.serverName || `节点 #${item.serverId}` }}</span>
        </div>

        <div class="row-io mono">
          <span class="io-down">↓ {{ formatBytes(item.down) }}</span>
          <span class="io-up">↑ {{ formatBytes(item.up) }}</span>
        </div>

        <div class="row-total">
          <div class="total-head mono">
            <strong>{{ formatBytes(item.total) }}</strong>
            <span>{{ share(item.total).toFixed(1) }}%</span>
          </div>
          <span class="share-track" aria-hidden="true">
            <i :style="{ width: Math.max(2, share(item.total)) + '%' }"></i>
          </span>
        </div>

        <span class="row-arrow" aria-hidden="true">
          <el-icon><ArrowRight /></el-icon>
        </span>
      </button>
    </div>

    <!-- Click-to-Inspect URL / Traffic Detail Card -->
    <UiDialog
      v-model="detailOpen"
      :title="selectedItem ? displayDomain(selectedItem.domain) : '流量详情'"
      width="560px"
    >
      <div v-if="selectedItem" class="detail-card">
        <div class="detail-header-meta">
          <span class="meta-chip">{{ classifyTraffic(selectedItem).tag }}</span>
          <span class="meta-chip is-node">
            {{ selectedItem.serverName || `节点 #${selectedItem.serverId}` }}
          </span>
          <span class="meta-chip mono">排名 #{{ selectedItem.rank }}</span>
          <button
            type="button"
            class="copy-domain-btn"
            :class="{ 'is-done': copiedDomain === selectedItem.domain }"
            @click="copyDomain(selectedItem.domain)"
          >
            <el-icon>
              <Check v-if="copiedDomain === selectedItem.domain" />
              <CopyDocument v-else />
            </el-icon>
            <span>{{ copiedDomain === selectedItem.domain ? "已复制" : "复制域名" }}</span>
          </button>
        </div>

        <!-- Download vs Upload Visual Ratio -->
        <section class="detail-section">
          <div class="ratio-numbers mono">
            <div>
              <small>合计流量</small>
              <strong>{{ formatBytes(selectedItem.total) }}</strong>
              <em>占全局 {{ share(selectedItem.total).toFixed(1) }}%</em>
            </div>
            <div>
              <small>下载下行</small>
              <strong>{{ formatBytes(selectedItem.down) }}</strong>
              <em>{{ downRatio(selectedItem).toFixed(1) }}%</em>
            </div>
            <div>
              <small>上传上行</small>
              <strong>{{ formatBytes(selectedItem.up) }}</strong>
              <em>{{ (100 - downRatio(selectedItem)).toFixed(1) }}%</em>
            </div>
          </div>
          <div class="ratio-bar" aria-hidden="true">
            <span class="ratio-down" :style="{ width: downRatio(selectedItem) + '%' }"></span>
          </div>
        </section>

        <!-- Path & Content Type (MP4 / M3U8 / API) Inspection Section -->
        <section class="detail-section">
          <h3 class="section-label">路径与内容特征</h3>

          <!-- If backend provides path/content details in future, render them directly -->
          <div v-if="selectedPaths.length" class="path-list">
            <div v-for="(entry, i) in selectedPaths" :key="i" class="path-row">
              <span class="ext-badge mono">{{ entry.ext || entry.contentType || "REQ" }}</span>
              <span class="path-url mono">{{ entry.path }}</span>
              <span class="path-size mono">{{ formatBytes(entry.bytes) }}</span>
            </div>
          </div>

          <!-- Honest TLS SNI inspection + heuristic media/traffic profile -->
          <div v-else class="profile-box">
            <div class="profile-grid">
              <div>
                <dt>访问目标</dt>
                <dd class="mono">{{ displayDomain(selectedItem.domain) }}</dd>
              </div>
              <div>
                <dt>内容特征推断</dt>
                <dd>{{ classifyTraffic(selectedItem).hint }}</dd>
              </div>
              <div>
                <dt>常见资源类型</dt>
                <dd class="mono">{{ classifyTraffic(selectedItem).mime }}</dd>
              </div>
              <div>
                <dt>采集粒度</dt>
                <dd>TLS SNI 域名级汇总</dd>
              </div>
            </div>
            <p class="sni-note">
              HTTPS 代理链路默认端到端加密，节点当前仅采集域名与上下行字节，不解密具体 URL 路径与 .mp4 等明文内容。
            </p>
          </div>
        </section>

        <!-- Same Domain Across Other Nodes (if present) -->
        <section v-if="sameDomainNodes.length > 1" class="detail-section">
          <h3 class="section-label">同域名节点分布（{{ sameDomainNodes.length }}）</h3>
          <div class="node-breakdown">
            <div
              v-for="row in sameDomainNodes"
              :key="row.serverId"
              class="node-row"
            >
              <span>{{ row.serverName || `节点 #${row.serverId}` }}</span>
              <span class="mono">{{ formatBytes(row.total) }}</span>
            </div>
          </div>
        </section>
      </div>
    </UiDialog>
  </div>
</template>

<script setup lang="ts">
import {
  computed,
  onActivated,
  onBeforeUnmount,
  onDeactivated,
  onMounted,
  reactive,
  ref,
} from "vue";
import gsap from "gsap";
import {
  WebsiteTrafficState,
  type WebsiteTrafficData,
} from "@/api/proxy/websiteTraffic";
import { StatusDot, UiButton, UiDialog, UiSkeleton } from "@/ui";
import { formatBytes } from "@/utils/traffic";
import { recentHourWindow } from "@/utils/datetime";
import {
  ArrowRight,
  Check,
  CopyDocument,
  Refresh,
  Search,
} from "@element-plus/icons-vue";

defineOptions({ name: "MonitorWebsite" });

type TrafficItem = WebsiteTrafficData["items"][number] & {
  paths?: { path: string; contentType?: string; ext?: string; bytes: number }[];
};

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const state = reactive(new WebsiteTrafficState());
const offline = ref(typeof navigator !== "undefined" && !navigator.onLine);
const windowHours = ref(24);
const sortOrder = ref<"total" | "down" | "up">("total");
const keyword = ref("");
const selectedItem = ref<TrafficItem | null>(null);
const detailOpen = ref(false);
const copiedDomain = ref("");
let copyTimer: ReturnType<typeof setTimeout> | null = null;

const windowOptions = [
  { label: "24 小时", hours: 24 },
  { label: "7 天", hours: 24 * 7 },
  { label: "30 天", hours: 24 * 30 },
];

const orderOptions: { label: string; value: "total" | "down" | "up" }[] = [
  { label: "按总流量", value: "total" },
  { label: "按下载", value: "down" },
  { label: "按上传", value: "up" },
];

const windowLabel = computed(
  () => windowOptions.find((o) => o.hours === windowHours.value)?.label ?? "24 小时"
);

const disabledReason = computed(() =>
  offline.value
    ? "当前离线"
    : state.status === "loading"
      ? "正在刷新"
      : ""
);

function displayDomain(domain: string) {
  return domain === state.data?.unknownDomain
    ? "未知（IP 直连 / 无法识别）"
    : domain;
}

const filteredItems = computed<TrafficItem[]>(() => {
  const items = (state.data?.items ?? []) as TrafficItem[];
  const q = keyword.value.trim().toLowerCase();
  if (!q) return items;
  return items.filter(
    (item) =>
      displayDomain(item.domain).toLowerCase().includes(q) ||
      (item.serverName || "").toLowerCase().includes(q)
  );
});

const selectedPaths = computed(() => selectedItem.value?.paths ?? []);

const sameDomainNodes = computed(() => {
  if (!selectedItem.value || !state.data) return [];
  return state.data.items.filter(
    (item) => item.domain === selectedItem.value?.domain
  );
});

function share(total: number) {
  return state.data?.summary.total
    ? Math.min(100, Math.max(0, (total / state.data.summary.total) * 100))
    : 0;
}

function downRatio(item: TrafficItem) {
  if (!item.total) return 50;
  return Math.min(100, Math.max(0, (item.down / item.total) * 100));
}

function classifyTraffic(item: TrafficItem) {
  const d = item.domain.toLowerCase();
  const dr = downRatio(item);
  if (
    /youtube|googlevideo|netflix|nflxvideo|bilibili|bilivideo|twitch|vimeo|tiktok|bytedance|disney|hulu|hbomax|iqiyi|youku/.test(
      d
    )
  ) {
    return {
      tag: "视频流媒体",
      hint: "高下行流媒体分发",
      mime: "MP4 / M3U8 / TS / WEBM",
    };
  }
  if (/openai|anthropic|claude|chatgpt|gemini|googleapis|mistral|perplexity/.test(d)) {
    return {
      tag: "AI / API",
      hint: "模型推理与流式接口",
      mime: "JSON / SSE / HTTPS",
    };
  }
  if (/github|githubusercontent|docker|npmjs|pypi|jsdelivr|unpkg|gitlab/.test(d)) {
    return {
      tag: "开发 / 制品",
      hint: "代码仓库与包镜像下载",
      mime: "GIT / TAR / GZ / JSON",
    };
  }
  if (/cloudflare|akamai|fastly|cloudfront|edgesuite|cdn|amazonaws|s3/.test(d)) {
    return {
      tag: "CDN / 对象存储",
      hint: dr >= 85 ? "静态资源或媒体分片下载" : "混合资源分发",
      mime: dr >= 85 ? "MP4 / IMG / JS / BIN" : "HTTPS / STATIC",
    };
  }
  if (/telegram|whatsapp|discord|twitter|twimg|x\.com|instagram|fbcdn|reddit/.test(d)) {
    return {
      tag: "社交 / 媒体",
      hint: "即时通讯与图片视频流",
      mime: "JPG / WEBP / MP4 / WS",
    };
  }
  if (item.domain === state.data?.unknownDomain) {
    return {
      tag: "IP 直连",
      hint: "无 SNI 或原始 IP 连接",
      mime: "TCP / UDP RAW",
    };
  }
  if (dr >= 90 && item.total >= 50 * 1024 * 1024) {
    return {
      tag: "大流量下行",
      hint: "疑似大文件或音视频拉流",
      mime: "BIN / MP4 / ARCHIVE",
    };
  }
  if (dr <= 25 && item.total >= 10 * 1024 * 1024) {
    return {
      tag: "高上行传输",
      hint: "疑似文件上传、备份或推流",
      mime: "UPLOAD / STREAM",
    };
  }
  return {
    tag: "常规站点",
    hint: "网页浏览与接口请求",
    mime: "HTML / JSON / ASSETS",
  };
}

function openDetail(item: TrafficItem) {
  selectedItem.value = item;
  detailOpen.value = true;
}

async function copyDomain(domain: string) {
  try {
    await navigator.clipboard.writeText(domain);
    copiedDomain.value = domain;
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => {
      copiedDomain.value = "";
    }, 1800);
  } catch {
    /* ignore clipboard permission error */
  }
}

async function load() {
  if (disabledReason.value) return;
  const base = recentHourWindow();
  const end = base.end;
  const start = end - windowHours.value * 3600 * 1000;
  await state.load({
    start: new Date(start).toISOString(),
    end: new Date(end).toISOString(),
    order: sortOrder.value,
    limit: 100,
  });
}

function setWindow(hours: number) {
  if (windowHours.value === hours) return;
  windowHours.value = hours;
  void load();
}

function setOrder(order: "total" | "down" | "up") {
  if (sortOrder.value === order) return;
  sortOrder.value = order;
  void load();
}

let active = false;
let refreshTimer: ReturnType<typeof setTimeout> | undefined;
const REFRESH_MS = 60000;

function stopRefresh() {
  active = false;
  clearTimeout(refreshTimer);
}

function scheduleRefresh() {
  clearTimeout(refreshTimer);
  if (!active || offline.value || document.visibilityState === "hidden") return;
  refreshTimer = setTimeout(async () => {
    if (!active || offline.value || document.visibilityState === "hidden") return;
    await load();
    scheduleRefresh();
  }, REFRESH_MS);
}

function resumeRefresh() {
  active = true;
  scheduleRefresh();
  refreshIfStale();
}

function refreshIfStale() {
  if (
    active &&
    !offline.value &&
    document.visibilityState !== "hidden" &&
    (!state.updatedAt || Date.now() - Date.parse(state.updatedAt) >= REFRESH_MS)
  ) {
    void load();
  }
}

function connectionChanged() {
  offline.value = !navigator.onLine;
  refreshIfStale();
  scheduleRefresh();
}

function visibilityChanged() {
  refreshIfStale();
  scheduleRefresh();
}

onMounted(() => {
  window.addEventListener("online", connectionChanged);
  window.addEventListener("offline", connectionChanged);
  document.addEventListener("visibilitychange", visibilityChanged);
  resumeRefresh();
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

onActivated(resumeRefresh);
onDeactivated(stopRefresh);

onBeforeUnmount(() => {
  if (copyTimer) clearTimeout(copyTimer);
  stopRefresh();
  state.cancel();
  document.removeEventListener("visibilitychange", visibilityChanged);
  window.removeEventListener("online", connectionChanged);
  window.removeEventListener("offline", connectionChanged);
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<style scoped>
.traffic-gallery {
  display: grid;
  gap: 20px;
  max-width: 1180px;
  min-width: 0;
}

.traffic-masthead {
  display: flex;
  align-items: flex-end;
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
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.live-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.live-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.inline-notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 14px;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}

/* Hairline KPI Strip */
.kpi-strip {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 0;
  margin: 0;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--c-line);
}

.kpi-cell {
  display: grid;
  gap: 6px;
  padding: 0 24px;
  border-left: 1px solid var(--c-line);
  min-width: 0;
}

.kpi-cell:first-child {
  padding-left: 0;
  border-left: 0;
}

.kpi-cell dt {
  font-size: 12px;
  color: var(--c-muted);
}

.kpi-cell dd {
  margin: 0;
  font-size: clamp(22px, 2.2vw, 28px);
  font-weight: 500;
  letter-spacing: -0.03em;
  color: var(--c-ink);
  font-variant-numeric: tabular-nums;
}

.kpi-cell.is-primary dd {
  color: var(--c-ok);
}

/* Control Bar */
.control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.pill-groups {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.pill-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.bar-divider {
  width: 1px;
  height: 16px;
  background: var(--c-line);
}

.seg-pill {
  padding: 6px 12px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.seg-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-line);
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.search-wrap {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  width: min(260px, 100%);
  padding: 6px 0;
  border-bottom: 1px solid var(--c-line);
  transition: border-color var(--duration-fast) var(--ease-out);
}

.search-wrap:focus-within {
  border-bottom-color: var(--c-ink);
}

.search-icon {
  color: var(--c-muted);
  flex: none;
}

.search-input {
  width: 100%;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 13px;
  outline: none !important;
}

.search-input::placeholder {
  color: var(--c-muted);
}

/* Full-Width Borderless Gallery Rows */
.traffic-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.traffic-list:hover .traffic-row:not(:hover) {
  opacity: 0.56;
}

.traffic-row {
  position: relative;
  display: grid;
  grid-template-columns: 36px minmax(0, 1.5fr) 170px 180px 24px;
  align-items: center;
  gap: 16px;
  width: 100%;
  padding: 16px 10px;
  border: 0;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  text-align: left;
  cursor: pointer;
  overflow: hidden;
  transition:
    opacity 220ms var(--ease-out),
    background 220ms var(--ease-out);
}

.traffic-row::before {
  content: "";
  position: absolute;
  inset: 0;
  background: var(--c-surface);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 320ms cubic-bezier(0.16, 1, 0.3, 1);
  z-index: 0;
}

.traffic-row:hover::before,
.traffic-row.is-selected::before {
  transform: scaleX(1);
}

.traffic-row > * {
  position: relative;
  z-index: 1;
}

.row-rank {
  font-size: 12px;
  color: var(--c-muted);
}

.row-main {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.row-domain-line {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.row-domain {
  font-size: 14.5px;
  font-weight: 500;
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.row-cat {
  flex: none;
  padding: 2px 8px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  color: var(--c-muted);
  font-size: 11px;
}

.row-node {
  font-size: 12px;
  color: var(--c-muted);
}

.row-io {
  display: grid;
  gap: 2px;
  text-align: right;
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.io-down {
  color: var(--c-ink);
}

.io-up {
  color: var(--c-muted);
}

.row-total {
  display: grid;
  gap: 6px;
}

.total-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
  font-variant-numeric: tabular-nums;
}

.total-head strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.total-head span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.share-track {
  display: block;
  height: 2px;
  border-radius: 2px;
  background: var(--c-line);
  overflow: hidden;
}

.share-track i {
  display: block;
  height: 100%;
  background: var(--c-ink);
  transition: width 360ms var(--ease-out);
}

.traffic-row:hover .share-track i {
  background: var(--c-ok);
}

.row-arrow {
  display: inline-flex;
  justify-content: flex-end;
  color: var(--c-muted);
  transition: transform 220ms var(--ease-out), color 220ms var(--ease-out);
}

.traffic-row:hover .row-arrow {
  color: var(--c-ink);
  transform: translateX(3px);
}

.empty-gallery {
  display: grid;
  justify-items: center;
  gap: 12px;
  padding: 56px 16px;
  border-top: 1px solid var(--c-line);
  color: var(--c-muted);
}

.empty-title {
  margin: 0;
  font-size: 14px;
}

/* Detail Card Dialog */
.detail-card {
  display: grid;
  gap: 20px;
}

.detail-header-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.meta-chip {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  font-size: 11.5px;
  color: var(--c-ink);
  background: var(--c-surface);
}

.meta-chip.is-node {
  color: var(--c-muted);
}

.copy-domain-btn {
  margin-left: auto;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.copy-domain-btn:hover {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.copy-domain-btn.is-done {
  border-color: var(--c-ok);
  color: var(--c-ok);
}

.detail-section {
  display: grid;
  gap: 10px;
}

.section-label {
  margin: 0;
  font-size: 12px;
  font-weight: 500;
  color: var(--c-muted);
}

.ratio-numbers {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  padding: 14px 0;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
}

.ratio-numbers > div {
  display: grid;
  gap: 3px;
}

.ratio-numbers small {
  font-size: 11px;
  color: var(--c-muted);
}

.ratio-numbers strong {
  font-size: 17px;
  font-weight: 600;
  color: var(--c-ink);
}

.ratio-numbers em {
  font-style: normal;
  font-size: 11px;
  color: var(--c-muted);
}

.ratio-bar {
  height: 4px;
  border-radius: 999px;
  background: var(--c-line);
  overflow: hidden;
}

.ratio-down {
  display: block;
  height: 100%;
  background: var(--c-ok);
}

.profile-box {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-muted);
}

.profile-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px 16px;
}

.profile-grid dt {
  font-size: 11px;
  color: var(--c-muted);
  margin-bottom: 3px;
}

.profile-grid dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-ink);
  overflow-wrap: anywhere;
}

.sni-note {
  margin: 0;
  padding-top: 10px;
  border-top: 1px solid var(--c-line);
  font-size: 11.5px;
  line-height: 1.55;
  color: var(--c-muted);
}

.path-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.path-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 10px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 12px;
}

.ext-badge {
  padding: 2px 7px;
  border-radius: 6px;
  background: var(--c-ink);
  color: var(--c-surface);
  font-size: 10.5px;
  font-weight: 600;
}

.path-url {
  color: var(--c-ink);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.path-size {
  color: var(--c-muted);
}

.node-breakdown {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.node-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 9px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 12.5px;
  color: var(--c-ink);
}

@media (max-width: 820px) {
  .kpi-strip {
    grid-template-columns: 1fr;
    gap: 12px;
  }
  .kpi-cell {
    padding: 0;
    border-left: 0;
  }
  .traffic-row {
    grid-template-columns: 28px minmax(0, 1fr) 20px;
    gap: 8px 10px;
  }
  .row-io,
  .row-total {
    grid-column: 2 / 3;
    text-align: left;
  }
  .row-io {
    display: flex;
    gap: 12px;
  }
  .profile-grid,
  .ratio-numbers {
    grid-template-columns: 1fr;
  }
}
</style>
