<script setup lang="ts">
defineProps<{
  route?: { name: string; protocol: string; transport: string }[];
}>();
</script>
<template>
  <div
    v-if="route && route.length > 1"
    class="diagnostic-route"
    aria-label="本次实际检测路径"
  >
    <strong>中转检测路径</strong>
    <ol>
      <li>面板网络</li>
      <li v-for="(hop, i) in route" :key="i">
        <span class="hop-role">{{
          i === route.length - 1 ? "落地" : "中转"
        }}</span
        ><span>{{ hop.name }}</span
        ><small>{{ hop.protocol }} / {{ hop.transport }}</small>
      </li>
      <li>目标网站</li>
    </ol>
    <p>按此顺序发起实际请求。入口连通不等于整条路径可用，以 HTTPS 结果为准。</p>
  </div>
</template>
<style scoped>
.diagnostic-route {
  padding: 16px;
  margin-block: 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface-2);
  font-size: 13px;
  overflow-wrap: anywhere;
}
.diagnostic-route ol {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 0;
  margin: 12px 0;
  list-style: none;
}
.diagnostic-route li {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 8px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
  min-width: 0;
  max-width: 100%;
}
.diagnostic-route li + li {
  border-left: 3px solid var(--c-focus);
}
.hop-role,
.diagnostic-route small,
.diagnostic-route p {
  color: var(--c-muted);
  font-size: 11px;
}
.diagnostic-route p {
  line-height: 1.7;
  margin: 0;
}
</style>
