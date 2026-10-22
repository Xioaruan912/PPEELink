<script setup lang="ts">
import { computed } from "vue";
import type { DailyTraffic } from "@/api/proxy";
import { formatBytes } from "@/utils/traffic";

/** 最近 30 天每日总流量柱状图（全部服务器合计），柱子依次升起。 */
const props = defineProps<{ rows: DailyTraffic[] }>();
const days = computed(() => {
  const totals = new Map<string, number>();
  for (const r of props.rows)
    totals.set(r.day, (totals.get(r.day) || 0) + r.up + r.down);
  const out: { day: string; total: number }[] = [];
  const now = new Date();
  for (let i = 29; i >= 0; i--) {
    const d = new Date(now.getFullYear(), now.getMonth(), now.getDate() - i);
    const key = `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
    out.push({ day: key, total: totals.get(key) || 0 });
  }
  return out;
});
const max = computed(() => Math.max(1, ...days.value.map((d) => d.total)));
const sum = computed(() => days.value.reduce((s, d) => s + d.total, 0));
</script>
<template>
  <div class="traffic-bars">
    <div v-if="!sum" class="traffic-empty" role="status">
      <strong>最近 30 天暂无流量</strong>
      <p>服务器回报流量后，这里会显示每日用量。</p>
    </div>
    <div
      v-else
      class="bars"
      role="img"
      :aria-label="`最近 30 天共 ${formatBytes(sum)}`"
    >
      <el-tooltip
        v-for="(d, i) in days"
        :key="d.day"
        :content="`${d.day.slice(5)} · ${formatBytes(d.total)}`"
        placement="top"
        :trigger-keys="[]"
      >
        <span class="bar" :style="{ '--h': d.total / max, '--i': i }"
          ><i></i
        ></span>
      </el-tooltip>
    </div>
    <div v-if="sum" class="axis">
      <span>{{ days[0].day.slice(5) }}</span>
      <span
        >30 天合计 <b class="mono">{{ formatBytes(sum) }}</b></span
      >
      <span>今天</span>
    </div>
  </div>
</template>
<style scoped>
.traffic-empty {
  padding: 8px 0;
}
.traffic-empty strong {
  font-size: 14px;
  font-weight: 500;
}
.traffic-empty p {
  margin: 4px 0 0;
  color: var(--c-muted);
  font-size: 13px;
}
.bars {
  display: grid;
  grid-template-columns: repeat(30, minmax(0, 1fr));
  align-items: end;
  gap: 4px;
  height: 96px;
}
.bar {
  display: flex;
  align-items: flex-end;
  height: 100%;
}
.bar i {
  width: 100%;
  height: max(3px, calc(var(--h) * 100%));
  border-radius: 4px 4px 2px 2px;
  background: linear-gradient(
    180deg,
    var(--c-signal),
    color-mix(in srgb, var(--c-signal) 30%, var(--c-data))
  );
  transform-origin: bottom;
  animation: bar-rise 0.8s var(--ease-out) both;
  animation-delay: calc(var(--i) * 18ms);
}
.bar:hover i {
  filter: brightness(1.15);
}
@keyframes bar-rise {
  from {
    transform: scaleY(0);
  }
}
.axis {
  display: flex;
  justify-content: space-between;
  margin-top: 8px;
  color: var(--c-muted);
  font-size: 12px;
}
.axis b {
  color: var(--c-ink);
  font-weight: 500;
}
@media (prefers-reduced-motion: reduce) {
  .bar i {
    animation: none;
  }
}
</style>
