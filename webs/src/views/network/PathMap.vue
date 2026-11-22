<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import type { TraceHop, TraceView } from "@/api/network/trace";
import { Close } from "@element-plus/icons-vue";
import { loadWorldCountries, type CountryFeature } from "@/utils/worldGeo";

const props = defineProps<{ views: TraceView[]; target: string; running?: boolean }>();

const colors = [
  "#15b85a",
  "#2563eb",
  "#9333ea",
  "#ea580c",
  "#db2777",
  "#0891b2",
];

const latLines = [83.3, 166.7, 250, 333.3, 416.7];
const lngLines = [166.7, 333.3, 500, 666.7, 833.3];

const hidden = ref<number[]>([]);
const selected = ref("");
const countries = ref<CountryFeature[]>([]);
const zoom = ref(1.06);
const offset = ref({ x: 0, y: 0 });
const svg = ref<SVGSVGElement>();
let disposed = false;

interface Point {
  key: string;
  viewId: number;
  source: string;
  hop: TraceHop;
  ttls: number[];
  x: number;
  y: number;
  color: string;
  index: number;
}

function formatTtlRange(ttls: number[]) {
  if (!ttls.length) return "#?";
  if (ttls.length === 1) return `#${ttls[0]}`;
  const first = ttls[0];
  const last = ttls[ttls.length - 1];
  return last - first + 1 === ttls.length
    ? `#${first}–${last}`
    : `#${ttls.join(",")}`;
}

function positioned(h: TraceHop): boolean {
  return (
    h.responded &&
    typeof h.latitude === "number" &&
    typeof h.longitude === "number" &&
    Number.isFinite(h.latitude) &&
    Number.isFinite(h.longitude) &&
    Math.abs(h.latitude) <= 90 &&
    Math.abs(h.longitude) <= 180 &&
    (h.latitude !== 0 || h.longitude !== 0)
  );
}

const routes = computed(() =>
  props.views.map((view, i) => {
    const color = colors[i % colors.length];
    const pts: Point[] = [];
    const byIp = new Map<string, Point>();
    view.hops.forEach((hop, index) => {
      if (!positioned(hop)) return;
      const ipKey = (hop.ip || "").trim().toLowerCase();
      if (ipKey && byIp.has(ipKey)) {
        const existing = byIp.get(ipKey)!;
        if (!existing.ttls.includes(hop.ttl)) existing.ttls.push(hop.ttl);
        return;
      }
      const pt: Point = {
        key: `${view.id}-${index}`,
        viewId: view.id,
        source: view.sourceName,
        hop,
        ttls: [hop.ttl],
        index,
        color,
        x: ((hop.longitude! + 180) / 360) * 1000,
        y: ((90 - hop.latitude!) / 180) * 500,
      };
      if (ipKey) byIp.set(ipKey, pt);
      pts.push(pt);
    });
    return { ...view, color, points: pts };
  })
);

const visibleRoutes = computed(() =>
  routes.value.filter((v) => !hidden.value.includes(v.id))
);
const points = computed(() => visibleRoutes.value.flatMap((v) => v.points));
const current = computed(() =>
  selected.value ? points.value.find((p) => p.key === selected.value) : undefined
);
const located = computed(() =>
  routes.value.reduce((sum, v) => sum + v.points.length, 0)
);

const transform = computed(
  () =>
    `translate(${offset.value.x} ${offset.value.y}) translate(500 240) scale(${zoom.value}) translate(-500 -240)`
);

const location = (h: TraceHop) =>
  [h.country, h.province, h.city]
    .filter((v, i, a) => v && a.indexOf(v) === i)
    .join(" · ");

function toggle(id: number) {
  hidden.value = hidden.value.includes(id)
    ? hidden.value.filter((v) => v !== id)
    : [...hidden.value, id];
}

function reset() {
  zoom.value = 1.06;
  offset.value = { x: 0, y: 0 };
}

function fit() {
  const ps = points.value;
  if (!ps.length) {
    reset();
    return;
  }
  const xs = ps.map((p) => p.x);
  const ys = ps.map((p) => p.y);
  const minX = Math.min(...xs);
  const maxX = Math.max(...xs);
  const minY = Math.min(...ys);
  const maxY = Math.max(...ys);
  zoom.value = Math.max(
    1.06,
    Math.min(
      6,
      Math.min(
        820 / Math.max(120, maxX - minX),
        340 / Math.max(70, maxY - minY)
      )
    )
  );
  offset.value = {
    x: (500 - (minX + maxX) / 2) * zoom.value,
    y: (240 - (minY + maxY) / 2) * zoom.value,
  };
}

function setZoom(delta: number) {
  zoom.value = Math.max(1, Math.min(8, Number((zoom.value + delta).toFixed(2))));
}

function onWheel(e: WheelEvent) {
  setZoom(e.deltaY < 0 ? 0.3 : -0.3);
}

function choose(key: string) {
  selected.value = selected.value === key ? "" : key;
}

function segment(
  a: Point,
  b: Point
): { path: string; shift: number; gap: boolean }[] {
  let end = b.x;
  const diff = end - a.x;
  if (diff > 500) end -= 1000;
  else if (diff < -500) end += 1000;
  const mid = (a.x + end) / 2;
  const bend = Math.min(36, Math.max(8, Math.abs(end - a.x) * 0.12));
  const path = `M${a.x},${a.y} Q${mid},${(a.y + b.y) / 2 - bend} ${end},${b.y}`;
  const result = [{ path, shift: 0, gap: b.index - a.index > 1 }];
  if (end < 0) result.push({ path, shift: 1000, gap: b.index - a.index > 1 });
  if (end > 1000)
    result.push({ path, shift: -1000, gap: b.index - a.index > 1 });
  return result;
}

const lines = computed(() =>
  visibleRoutes.value.flatMap((v) =>
    v.points
      .slice(1)
      .flatMap((p, i) =>
        segment(v.points[i], p).map((line, j) => ({
          ...line,
          key: `${v.id}-${i}-${j}`,
          color: v.color,
        }))
      )
  )
);

interface JumpOrb {
  key: string;
  x: number;
  y: number;
  color: string;
}

const isPlaying = ref(false);
const jumpOrbs = ref<JumpOrb[]>([]);
const activeJumpKeys = ref<Set<string>>(new Set());
const jumpBanner = ref("");
let rafId: number | null = null;

function stopHopAnimation() {
  if (rafId !== null) {
    cancelAnimationFrame(rafId);
    rafId = null;
  }
  isPlaying.value = false;
  jumpOrbs.value = [];
  activeJumpKeys.value = new Set();
  jumpBanner.value = "";
}

function evalArcPoint(a: Point, b: Point, t: number): { x: number; y: number } {
  let endX = b.x;
  const diff = endX - a.x;
  if (diff > 500) endX -= 1000;
  else if (diff < -500) endX += 1000;
  const ctrlX = (a.x + endX) / 2;
  const bend = Math.min(36, Math.max(8, Math.abs(endX - a.x) * 0.12));
  const ctrlY = (a.y + b.y) / 2 - bend;
  const u = 1 - t;
  let x = u * u * a.x + 2 * u * t * ctrlX + t * t * endX;
  const y = u * u * a.y + 2 * u * t * ctrlY + t * t * b.y;
  if (x < 0) x += 1000;
  if (x > 1000) x -= 1000;
  return { x, y };
}

function playHopAnimation() {
  stopHopAnimation();
  if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
  const playable = visibleRoutes.value.filter((v) => v.points.length >= 2);
  if (!playable.length) return;

  isPlaying.value = true;
  const segMs = 620;
  const maxSegs = Math.max(...playable.map((v) => v.points.length - 1));
  const totalMs = maxSegs * segMs + 400;
  const startAt = performance.now();

  const tick = (now: number) => {
    const elapsed = now - startAt;
    if (elapsed >= totalMs) {
      stopHopAnimation();
      return;
    }
    const orbs: JumpOrb[] = [];
    const arrived = new Set<string>();
    let latestBanner = "";

    for (const r of playable) {
      const segCount = r.points.length - 1;
      const segFloat = Math.min(segCount, elapsed / segMs);
      const segIdx = Math.min(segCount - 1, Math.floor(segFloat));
      const localT = Math.min(1, Math.max(0, segFloat - segIdx));
      const eased =
        localT < 0.5
          ? 2 * localT * localT
          : 1 - Math.pow(-2 * localT + 2, 2) / 2;

      for (let k = 0; k <= segIdx; k++) {
        arrived.add(r.points[k].key);
      }
      if (localT >= 0.88) {
        arrived.add(r.points[segIdx + 1].key);
      }

      const pos = evalArcPoint(r.points[segIdx], r.points[segIdx + 1], eased);
      orbs.push({ key: `orb-${r.id}`, x: pos.x, y: pos.y, color: r.color });

      const targetPt = r.points[Math.min(r.points.length - 1, segIdx + 1)];
      if (!latestBanner && targetPt) {
        const loc = location(targetPt.hop) || targetPt.hop.ip || "跳点";
        const rtt =
          targetPt.hop.rtt === null ? "" : ` · ${targetPt.hop.rtt.toFixed(1)} ms`;
        latestBanner = `${formatTtlRange(targetPt.ttls)} ${loc}${rtt}`;
      }
    }

    jumpOrbs.value = orbs;
    activeJumpKeys.value = arrived;
    jumpBanner.value = latestBanner;
    rafId = requestAnimationFrame(tick);
  };

  rafId = requestAnimationFrame(tick);
}

watch(
  () => props.running,
  (now, prev) => {
    if (prev && !now && visibleRoutes.value.some((v) => v.points.length >= 2)) {
      fit();
      playHopAnimation();
    }
  }
);

watch(
  () => props.views,
  (next, old) => {
    if (next.map((v) => v.id).join(",") !== old?.map((v) => v.id).join(",")) {
      stopHopAnimation();
      hidden.value = [];
      selected.value = "";
      reset();
    }
  }
);

let panStart: { x: number; y: number; pointer: number } | undefined;
function startPan(e: PointerEvent) {
  if ((e.target as Element).closest(".map-hop")) return;
  if (e.pointerType === "mouse" && e.button !== 0) return;
  panStart = { x: e.clientX, y: e.clientY, pointer: e.pointerId };
  (e.currentTarget as SVGSVGElement).setPointerCapture(e.pointerId);
}
function pan(e: PointerEvent) {
  if (!panStart || panStart.pointer !== e.pointerId || !svg.value) return;
  const scale = 1000 / svg.value.getBoundingClientRect().width;
  offset.value = {
    x: offset.value.x + (e.clientX - panStart.x) * scale,
    y: offset.value.y + (e.clientY - panStart.y) * scale,
  };
  panStart = { x: e.clientX, y: e.clientY, pointer: e.pointerId };
}
function stopPan(e: PointerEvent) {
  const el = e.currentTarget as SVGSVGElement;
  if (el.hasPointerCapture(e.pointerId)) el.releasePointerCapture(e.pointerId);
  panStart = undefined;
}

onMounted(async () => {
  try {
    const list = await loadWorldCountries();
    if (!disposed) countries.value = list;
  } catch {
    /* keep silent */
  }
});

onBeforeUnmount(() => {
  disposed = true;
  stopHopAnimation();
});
</script>

<template>
  <section class="studio-map" aria-label="全球路由路径地图">
    <div class="map-stage" :class="{ 'is-running': running }">
      <svg
        ref="svg"
        class="world"
        viewBox="0 15 1000 445"
        preserveAspectRatio="xMidYMid slice"
        tabindex="0"
        role="group"
        aria-label="全球路由路径地图"
        @wheel.prevent="onWheel"
        @pointerdown="startPan"
        @pointermove="pan"
        @pointerup="stopPan"
        @pointercancel="stopPan"
      >
        <defs>
          <radialGradient id="pm-ocean-light" cx="50%" cy="46%" r="68%">
            <stop offset="0%" stop-color="#e3eef8" />
            <stop offset="60%" stop-color="#d5e4f2" />
            <stop offset="100%" stop-color="#c4d7ea" />
          </radialGradient>
          <radialGradient id="pm-ocean-dark" cx="50%" cy="46%" r="68%">
            <stop offset="0%" stop-color="#112032" />
            <stop offset="65%" stop-color="#0b1623" />
            <stop offset="100%" stop-color="#070e17" />
          </radialGradient>
          <linearGradient id="pm-land-light" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#f7f5ee" />
            <stop offset="55%" stop-color="#efeadd" />
            <stop offset="100%" stop-color="#e4decb" />
          </linearGradient>
          <linearGradient id="pm-land-dark" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stop-color="#223041" />
            <stop offset="60%" stop-color="#1a2533" />
            <stop offset="100%" stop-color="#141d28" />
          </linearGradient>
          <filter id="pm-relief" x="-5%" y="-5%" width="110%" height="112%">
            <feDropShadow
              dx="0"
              dy="1.6"
              stdDeviation="1.8"
              flood-color="#1e3a5f"
              flood-opacity="0.18"
            />
          </filter>
        </defs>

        <rect x="0" y="0" width="1000" height="500" class="ocean-bg" />

        <g :transform="transform">
          <g class="graticules" aria-hidden="true">
            <line
              v-for="y in latLines"
              :key="`lat-${y}`"
              x1="0"
              :y1="y"
              x2="1000"
              :y2="y"
              :class="{ 'is-equator': y === 250 }"
              vector-effect="non-scaling-stroke"
            />
            <line
              v-for="x in lngLines"
              :key="`lng-${x}`"
              :x1="x"
              y1="0"
              :x2="x"
              y2="500"
              vector-effect="non-scaling-stroke"
            />
          </g>

          <g class="ocean-labels" aria-hidden="true">
            <text x="148" y="290">PACIFIC OCEAN</text>
            <text x="385" y="245">ATLANTIC OCEAN</text>
            <text x="678" y="335">INDIAN OCEAN</text>
          </g>

          <g class="land-layer" filter="url(#pm-relief)">
            <path
              v-for="country in countries"
              :key="country.name"
              :d="country.d"
              class="country"
              vector-effect="non-scaling-stroke"
            >
              <title>{{ country.name }}</title>
            </path>
          </g>

          <!-- Multi-layer Curved Kinetic Route Arcs -->
          <path
            v-for="line in lines"
            :key="`glow-${line.key}`"
            :d="line.path"
            :transform="`translate(${line.shift} 0)`"
            class="route-glow"
            :stroke="line.color"
            vector-effect="non-scaling-stroke"
          />
          <path
            v-for="line in lines"
            :key="line.key"
            :d="line.path"
            :transform="`translate(${line.shift} 0)`"
            class="route-arc"
            :class="{ 'is-gap': line.gap }"
            :stroke="line.color"
            vector-effect="non-scaling-stroke"
          />

          <!-- 3D-Elevated Hop Dots -->
          <g
            v-for="p in points"
            :key="p.key"
            class="map-hop"
            :class="{
              'is-selected': current?.key === p.key,
              'is-arrived': activeJumpKeys.has(p.key),
            }"
            :style="{ '--hop-color': p.color }"
            tabindex="0"
            role="button"
            @click.stop="choose(p.key)"
            @keydown.enter.prevent="choose(p.key)"
          >
            <title>{{ p.source }} {{ formatTtlRange(p.ttls) }} {{ p.hop.ip }}</title>
            <circle :cx="p.x" :cy="p.y" :r="15 / zoom" class="hop-hit" />
            <circle :cx="p.x" :cy="p.y" :r="9.5 / zoom" class="hop-halo" />
            <circle :cx="p.x" :cy="p.y" :r="5 / zoom" class="hop-dot" />
          </g>

          <!-- Animated Hop-by-Hop Jumping Tracer Orbs -->
          <g v-if="jumpOrbs.length" class="jump-layer" aria-hidden="true">
            <g v-for="orb in jumpOrbs" :key="orb.key">
              <circle
                :cx="orb.x"
                :cy="orb.y"
                :r="14 / zoom"
                :fill="orb.color"
                opacity="0.24"
              />
              <circle
                :cx="orb.x"
                :cy="orb.y"
                :r="7.5 / zoom"
                :fill="orb.color"
                opacity="0.52"
              />
              <circle
                :cx="orb.x"
                :cy="orb.y"
                :r="4.2 / zoom"
                fill="#ffffff"
                :stroke="orb.color"
                :stroke-width="2 / zoom"
              />
            </g>
          </g>
        </g>
      </svg>

      <!-- Floating Top-Right Controls -->
      <div class="floating-controls">
        <button
          v-if="located >= 2"
          type="button"
          aria-label="重播跳点动画"
          @click="playHopAnimation"
        >
          {{ isPlaying ? "播放中" : "▶ 重播" }}
        </button>
        <button type="button" aria-label="放大" @click="setZoom(0.5)">+</button>
        <button type="button" aria-label="缩小" @click="setZoom(-0.5)">−</button>
        <button type="button" aria-label="适应路径" @click="fit">适应</button>
        <button type="button" aria-label="重置" @click="reset">全图</button>
      </div>

      <!-- Live Jumping Hop Banner -->
      <div v-if="isPlaying && jumpBanner" class="map-hint is-jumping">
        <i aria-hidden="true"></i>
        <span class="mono">{{ jumpBanner }}</span>
      </div>
      <!-- Subtle Hint When No Hop Selected -->
      <div v-else-if="!current && !located" class="map-hint">
        <i aria-hidden="true"></i>
        <span>{{ running ? "正在追踪全球路由跳点…" : "选择目标并点击开始追踪" }}</span>
      </div>
      <div v-else-if="!current && located" class="map-hint">
        <i aria-hidden="true"></i>
        <span>点击地图跳点查看 IP / ASN / 延迟</span>
      </div>

      <!-- Floating Frosted Glass Hop Card -->
      <Transition name="hop-pop" mode="out-in">
        <aside v-if="current" :key="current.key" class="hop-card">
          <div class="hop-card-top">
            <span class="hop-ttl mono">
              <i :style="{ background: current.color }"></i>
              {{ formatTtlRange(current.ttls) }} · {{ current.source }}
            </span>
            <button
              type="button"
              class="hop-close"
              aria-label="关闭跳点卡片"
              @click="selected = ''"
            >
              <el-icon><Close /></el-icon>
            </button>
          </div>
          <strong class="hop-loc">{{ location(current.hop) || "未知地区" }}</strong>
          <code class="hop-ip mono">{{ current.hop.ip }}</code>
          <div class="hop-meta mono">
            <span>{{ current.hop.asn ? `AS${current.hop.asn.replace(/^AS/i, "")}` : "—" }}</span>
            <span>{{ current.hop.owner || current.hop.isp || "—" }}</span>
            <strong class="hop-rtt">
              {{ current.hop.rtt === null ? "—" : `${current.hop.rtt.toFixed(1)} ms` }}
            </strong>
          </div>
        </aside>
      </Transition>
    </div>

    <div v-if="routes.length" class="legend-rail">
      <button
        v-for="v in routes"
        :key="v.id"
        type="button"
        class="legend-pill"
        :class="{ 'is-muted': hidden.includes(v.id) }"
        @click="toggle(v.id)"
      >
        <i :style="{ background: v.color }"></i>
        <span>{{ v.sourceName }}</span>
        <small class="mono">{{ v.points.length }} 跳</small>
      </button>
    </div>
  </section>
</template>

<style scoped>
.studio-map {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.map-stage {
  position: relative;
  border: 1px solid var(--c-line);
  border-radius: 22px;
  background: var(--c-surface);
  overflow: hidden;
}

.map-stage.is-running::after {
  content: "";
  position: absolute;
  inset: 0;
  width: 24%;
  pointer-events: none;
  background: linear-gradient(90deg, transparent, rgb(21 184 90 / 0.16), transparent);
  animation: mapScan 2.2s linear infinite;
}

.world {
  display: block;
  width: 100%;
  height: clamp(380px, 52dvh, 560px);
  cursor: grab;
  touch-action: none;
  outline: none;
}

.world:active {
  cursor: grabbing;
}

.ocean-bg {
  fill: url(#pm-ocean-light);
}

:global(html.dark) .ocean-bg {
  fill: url(#pm-ocean-dark);
}

.graticules line {
  stroke: rgb(30 64 115 / 0.08);
  stroke-width: 0.7;
  stroke-dasharray: 2 4;
}

.graticules line.is-equator {
  stroke: rgb(30 64 115 / 0.16);
  stroke-dasharray: none;
}

:global(html.dark) .graticules line {
  stroke: rgb(148 184 230 / 0.08);
}

.ocean-labels text {
  fill: rgb(42 82 130 / 0.28);
  font-family: var(--font-display);
  font-size: 9.5px;
  font-style: italic;
  letter-spacing: 0.24em;
  user-select: none;
  pointer-events: none;
}

:global(html.dark) .ocean-labels text {
  fill: rgb(148 184 230 / 0.22);
}

.country {
  fill: url(#pm-land-light);
  stroke: #c6bfb0;
  stroke-width: 0.65;
  stroke-linejoin: round;
  transition: fill 160ms ease-out;
}

.country:hover {
  fill: #fbfaf5;
}

:global(html.dark) .country {
  fill: url(#pm-land-dark);
  stroke: #31445a;
}

:global(html.dark) .country:hover {
  fill: #2a3c52;
}

.route-glow {
  fill: none;
  stroke-width: 5;
  opacity: 0.2;
  stroke-linecap: round;
}

.route-arc {
  fill: none;
  stroke-width: 2.2;
  stroke-linecap: round;
  opacity: 0.92;
  stroke-dasharray: 6 4;
  animation: arcFlow 1.8s linear infinite;
}

.route-arc.is-gap {
  opacity: 0.5;
  stroke-dasharray: 3 6;
}

.map-hop {
  cursor: pointer;
  outline: none;
}

.hop-hit {
  fill: transparent;
}

.hop-halo {
  fill: none;
  stroke: var(--hop-color);
  stroke-width: 1.4;
  opacity: 0.35;
  vector-effect: non-scaling-stroke;
}

.hop-dot {
  fill: var(--hop-color);
  stroke: #ffffff;
  stroke-width: 1.8;
  vector-effect: non-scaling-stroke;
}

.map-hop.is-selected .hop-halo {
  opacity: 0.9;
  stroke-width: 2.2;
}

.floating-controls {
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 8;
  display: inline-flex;
  align-items: center;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: color-mix(in srgb, var(--c-surface) 90%, transparent);
  backdrop-filter: blur(16px);
  overflow: hidden;
  box-shadow: 0 8px 20px -8px rgb(0 0 0 / 0.14);
}

.floating-controls button {
  padding: 6px 11px;
  border: 0;
  border-left: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.floating-controls button:first-child {
  border-left: 0;
}

.floating-controls button:hover {
  background: var(--c-surface-muted);
}

.map-hint {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 6;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: color-mix(in srgb, var(--c-surface) 90%, transparent);
  backdrop-filter: blur(14px);
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

.hop-card {
  position: absolute;
  left: 20px;
  bottom: 20px;
  z-index: 10;
  width: min(320px, calc(100% - 40px));
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: color-mix(in srgb, var(--c-surface) 92%, transparent);
  backdrop-filter: blur(20px) saturate(160%);
  box-shadow: 0 20px 44px -16px rgb(0 0 0 / 0.18);
}

.hop-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.hop-ttl {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  font-size: 11.5px;
  color: var(--c-muted);
}

.hop-ttl i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.hop-close {
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
}

.hop-close:hover {
  background: var(--c-surface-muted);
  color: var(--c-ink);
}

.hop-loc {
  font-size: 15.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.hop-ip {
  font-size: 12px;
  color: var(--c-muted);
}

.hop-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 8px;
  margin-top: 4px;
  border-top: 1px solid var(--c-line);
  font-size: 11.5px;
  color: var(--c-muted);
}

.hop-rtt {
  color: var(--c-ok);
}

.legend-rail {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.legend-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-ink);
  cursor: pointer;
}

.legend-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}

.legend-pill small {
  color: var(--c-muted);
  font-size: 11px;
}

.legend-pill.is-muted {
  opacity: 0.45;
}

.map-hop.is-arrived .hop-halo {
  opacity: 0.42;
  transition: opacity 260ms ease;
}

.map-hint.is-jumping {
  color: var(--c-ink);
  font-weight: 500;
}

.map-hint.is-jumping i {
  background: #10b981;
}

.hop-pop-enter-active,
.hop-pop-leave-active {
  transition: opacity 200ms var(--ease-out), transform 200ms var(--ease-out);
}

.hop-pop-enter-from,
.hop-pop-leave-to {
  opacity: 0;
  transform: translateY(6px);
}

@keyframes arcFlow {
  to {
    stroke-dashoffset: -20;
  }
}

@keyframes mapScan {
  from {
    transform: translateX(-100%);
  }
  to {
    transform: translateX(460%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .route-arc,
  .map-stage.is-running::after {
    animation: none;
  }
}

@media (max-width: 760px) {
  .world {
    height: 300px;
  }
  .hop-card {
    position: static;
    width: 100%;
    border-radius: 0;
    border-width: 1px 0 0;
    box-shadow: none;
  }
}
</style>
