<template>
  <div v-if="traffic" class="traffic-meter" :class="`is-${traffic.state}`">
    <div class="traffic-line">
      <span class="mono"
        >{{ formatBytes(traffic.used)
        }}<template v-if="traffic.total">
          / {{ formatBytes(traffic.total) }}</template
        ></span
      ><span v-if="traffic.total" class="mono traffic-pct"
        >{{ Math.round(traffic.ratio * 100) }}%</span
      >
    </div>
    <div
      v-if="traffic.total"
      class="traffic-track"
      role="meter"
      :aria-valuenow="Math.round(traffic.ratio * 100)"
      aria-valuemin="0"
      aria-valuemax="100"
      :aria-label="`已用流量 ${Math.round(traffic.ratio * 100)}%`"
    >
      <i :style="{ '--fill': traffic.ratio }"></i>
    </div>
    <span class="traffic-expire">{{ expireText }}</span>
  </div>
  <span v-else class="traffic-empty">未提供</span>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { formatBytes, type trafficOf } from "@/utils/traffic";
import { formatDateTime } from "@/utils/date";
const props = defineProps<{ traffic: ReturnType<typeof trafficOf> }>();
const expireText = computed(() => {
  const t = props.traffic;
  if (!t?.expire) return "长期有效";
  if (t.daysLeft !== null && t.daysLeft <= 0) return "已到期";
  return `${formatDateTime(t.expire).slice(0, 10)} · 剩 ${t.daysLeft} 天`;
});
</script>
<style scoped>
.traffic-meter {
  display: grid;
  gap: 4px;
  min-width: 0;
  font-size: 13px;
}
.traffic-line {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  white-space: nowrap;
}
.mono {
  font: 13px/1.4 var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.traffic-pct {
  color: var(--c-muted);
}
.traffic-track {
  height: 4px;
  border-radius: 999px;
  background: var(--c-surface-2);
  overflow: hidden;
}
.traffic-track i {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: var(--c-ok);
  transform-origin: left;
  transform: scaleX(var(--fill));
  animation: traffic-grow var(--duration-slow) var(--ease-out);
}
.is-warn .traffic-track i {
  background: var(--c-warn);
}
.is-bad .traffic-track i {
  background: var(--c-bad);
}
.traffic-expire,
.traffic-empty {
  color: var(--c-muted);
  font-size: 12px;
  white-space: nowrap;
}
.is-bad .traffic-expire {
  color: var(--c-bad);
}
@keyframes traffic-grow {
  from {
    transform: scaleX(0);
  }
}
</style>
