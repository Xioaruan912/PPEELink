<template>
  <div
    ref="stage"
    class="network-globe"
    :class="{ 'is-dragging': dragging, 'contrast-view': contrast }"
  >
    <canvas
      ref="canvas"
      role="img"
      tabindex="0"
      aria-label="全球节点地理分布航图，可拖动或使用左右方向键旋转"
      @keydown.left.prevent="rotateBy(-0.12)"
      @keydown.right.prevent="rotateBy(0.12)"
      @pointerdown="startDrag"
      @pointermove="drag"
      @pointerup="stopDrag"
      @pointercancel="stopDrag"
    ></canvas>
    <div class="globe-caption">
      <span>{{ points.length ? `${points.length} 个节点` : "暂无节点" }}</span>
    </div>
    <button
      class="globe-play"
      :aria-label="paused ? '旋转航图' : '暂停航图'"
      @click="togglePause"
    >
      <WorkIcon :name="paused ? 'play' : 'pause'" />
    </button>
  </div>
</template>
<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from "vue";
import WorkIcon from "./WorkIcon.vue";
const props = withDefaults(
  defineProps<{
    points?: { lat: number; lng: number; name?: string }[];
    contrast?: boolean;
  }>(),
  { points: () => [], contrast: false }
);
const stage = ref<HTMLElement>();
const canvas = ref<HTMLCanvasElement>();
const paused = ref(true);
const dragging = ref(false);
let rotation = -1.8,
  lastX = 0,
  frame = 0,
  lastTime = 0,
  visible = true;
let themeObserver: MutationObserver | undefined;
let observer: IntersectionObserver | undefined,
  resize: ResizeObserver | undefined;
let lands: [number, number][] = [];
let reduced: MediaQueryList;
let context: CanvasRenderingContext2D | null = null;
let size = { w: 600, h: 430 };
const radians = Math.PI / 180;
function project(lon: number, lat: number) {
  const a = lon * radians + rotation,
    b = lat * radians;
  const z = Math.cos(b) * Math.cos(a);
  const r = Math.min(size.w * 0.39, size.h * 0.44);
  return {
    x: size.w / 2 + r * Math.cos(b) * Math.sin(a),
    y: size.h / 2 - r * (Math.sin(b) * 0.94 - z * 0.2),
    z,
    r,
  };
}
function render() {
  if (!context) return;
  const c = context;
  const tokens = getComputedStyle(document.documentElement);
  const ink = tokens.getPropertyValue("--c-muted").trim();
  const color = (name: string) => tokens.getPropertyValue(name).trim();
  c.clearRect(0, 0, size.w, size.h);
  const r = Math.min(size.w * 0.39, size.h * 0.44),
    cx = size.w / 2,
    cy = size.h / 2;
  c.beginPath();
  c.arc(cx, cy, r, 0, Math.PI * 2);
  c.fillStyle = color("--c-surface-2");
  c.fill();
  c.strokeStyle = `color-mix(in srgb, ${ink} 20%, transparent)`;
  c.lineWidth = 0.7;
  for (let lat = -60; lat <= 60; lat += 30) {
    c.beginPath();
    let started = false;
    for (let lon = -180; lon <= 180; lon += 3) {
      const p = project(lon, lat);
      if (p.z > 0) {
        started ? c.lineTo(p.x, p.y) : c.moveTo(p.x, p.y);
        started = true;
      } else started = false;
    }
    c.stroke();
  }
  for (let lon = -180; lon < 180; lon += 30) {
    c.beginPath();
    let started = false;
    for (let lat = -90; lat <= 90; lat += 3) {
      const p = project(lon, lat);
      if (p.z > 0) {
        started ? c.lineTo(p.x, p.y) : c.moveTo(p.x, p.y);
        started = true;
      } else started = false;
    }
    c.stroke();
  }
  for (const [lon, lat] of lands) {
    const p = project(lon, lat);
    if (p.z > 0) {
      c.fillStyle = `color-mix(in srgb, ${ink} ${Math.round((0.15 + p.z * 0.5) * 100)}%, transparent)`;
      c.beginPath();
      c.arc(
        p.x,
        p.y,
        Math.max(0.65, r / 150) * (0.6 + p.z * 0.4),
        0,
        Math.PI * 2
      );
      c.fill();
    }
  }
  const locations = props.points.filter(
    (p) => Number.isFinite(p.lat) && Number.isFinite(p.lng)
  );
  locations.slice(0, 80).forEach((node, i) => {
    const p = project(node.lng, node.lat);
    if (p.z <= 0.05) return;
    c.beginPath();
    c.arc(p.x, p.y, 7, 0, Math.PI * 2);
    c.fillStyle = color("--c-selected");
    c.fill();
    c.beginPath();
    c.arc(p.x, p.y, 3, 0, Math.PI * 2);
    c.fillStyle = color("--c-signal");
    c.fill();
    if (i < 6) {
      c.font = "14px sans-serif";
      c.fillStyle = color("--c-ink");
      c.fillText((node.name || "").slice(0, 14), p.x + 10, p.y + 3);
    }
  });
  // The orbit describes the atlas geometry; only API-backed node markers appear above.
  c.save();
  c.translate(cx, cy);
  c.rotate(-0.3);
  c.beginPath();
  c.ellipse(0, 0, r * 1.19, r * 0.33, 0, 0, Math.PI * 2);
  c.strokeStyle = `color-mix(in srgb, ${ink} 28%, transparent)`;
  c.stroke();
  c.restore();
}
function tick(time: number) {
  if (time - lastTime > 32) {
    if (!paused.value && !dragging.value) rotation += 0.0019;
    render();
    lastTime = time;
  }
  frame = requestAnimationFrame(tick);
}
function syncAnimation() {
  cancelAnimationFrame(frame);
  render();
  if (
    visible &&
    !document.hidden &&
    !reduced.matches &&
    document.documentElement.dataset.motion !== "reduced" &&
    !paused.value
  )
    frame = requestAnimationFrame(tick);
}
function togglePause() {
  paused.value = !paused.value;
  syncAnimation();
}
function startDrag(e: PointerEvent) {
  dragging.value = true;
  lastX = e.clientX;
  (e.target as HTMLElement).setPointerCapture(e.pointerId);
}
function drag(e: PointerEvent) {
  if (!dragging.value) return;
  rotation += (e.clientX - lastX) * 0.009;
  lastX = e.clientX;
  render();
}
function stopDrag() {
  dragging.value = false;
}
function rotateBy(amount: number) {
  rotation += amount;
  render();
}
const controller = new AbortController();
onMounted(async () => {
  context = canvas.value!.getContext("2d");
  themeObserver = new MutationObserver(render);
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ["class"],
  });
  reduced = matchMedia("(prefers-reduced-motion: reduce)");
  // Rotation starts only after an explicit user action.
  paused.value = true;
  resize = new ResizeObserver(([entry]) => {
    const rect = entry.contentRect;
    size = { w: rect.width, h: rect.height };
    const dpr = Math.min(devicePixelRatio, 2);
    canvas.value!.width = rect.width * dpr;
    canvas.value!.height = rect.height * dpr;
    context?.setTransform(dpr, 0, 0, dpr, 0, 0);
    render();
  });
  resize.observe(stage.value!);
  observer = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting;
    syncAnimation();
  });
  observer.observe(stage.value!);
  document.addEventListener("visibilitychange", syncAnimation);
  reduced.addEventListener("change", syncAnimation);
  window.addEventListener("workspace-motion-change", syncAnimation);
  try {
    // Pre-sampled from the project's world GeoJSON: a small static asset, no
    // megabyte download or polygon computation on the rendering thread.
    const response = await fetch(`${import.meta.env.BASE_URL}globe-land.json`, {
      signal: controller.signal,
    });
    if (response.ok) lands = await response.json();
    render();
  } catch {
    /* Geographic grid remains available when the map cannot be loaded. */
  }
});
watch(() => props.points, render);
onBeforeUnmount(() => {
  controller.abort();
  cancelAnimationFrame(frame);
  resize?.disconnect();
  themeObserver?.disconnect();
  observer?.disconnect();
  document.removeEventListener("visibilitychange", syncAnimation);
  reduced?.removeEventListener("change", syncAnimation);
  window.removeEventListener("workspace-motion-change", syncAnimation);
  lands = [];
});
</script>
<style scoped>
.network-globe {
  position: relative;
  width: 100%;
  height: 100%;
  min-height: 280px;
  isolation: isolate;
}
.network-globe canvas {
  position: absolute;
  inset: 0;
  display: block;
  width: 100%;
  height: 100%;
  cursor: grab;
  touch-action: pan-y;
}
.is-dragging canvas {
  cursor: grabbing;
}
.globe-caption {
  position: absolute;
  left: 20px;
  bottom: 20px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  color: var(--ui-text-secondary);
}
.geo-cross {
  font-size: 20px;
  font-weight: 300;
}
.globe-play {
  position: absolute;
  right: 20px;
  bottom: 18px;
  width: 44px;
  height: 44px;
  border: 1px solid var(--ui-border);
  border-radius: 50%;
  background: var(--ui-surface);
  display: grid;
  place-items: center;
  cursor: pointer;
}
.globe-play svg {
  width: 13px;
  height: 13px;
}
.globe-hint {
  position: absolute;
  right: 60px;
  bottom: 25px;
  font-size: 14px;
  color: var(--ui-text-muted);
}
</style>
