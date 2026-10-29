<script setup lang="ts">
import type { TraceHop } from '@/api/network/trace';
defineProps<{ source: string; sourceId: number; hops: TraceHop[] }>();
function location(hop:TraceHop){return [hop.country,hop.province,hop.city].filter((s,i,a)=>s&&a.indexOf(s)===i).join(' · ')}
</script>
<template>
 <div class="path-topology" :aria-label="`${source}的独立追踪路径`">
  <div class="path-source"><span class="source-dot" aria-hidden="true" /><strong>{{source}}</strong><span>出发网络</span></div>
  <ol class="path-hops">
   <li v-for="(hop,index) in hops" :key="`${sourceId}-${hop.ttl}-${index}`" :class="{unknown:!hop.responded}">
    <span class="hop-dot" aria-hidden="true" />
    <div class="hop-heading"><span class="hop-ttl">第 {{hop.ttl}} 跳</span><span class="hop-time">{{hop.rtt===null?'—':`${hop.rtt.toFixed(2)} ms`}}</span></div>
    <code class="hop-ip">{{hop.ip||'无应答'}}</code>
    <span v-if="hop.hostname" class="hop-host">{{hop.hostname}}</span>
    <div v-if="hop.asn||hop.owner||hop.isp" class="hop-network"><strong v-if="hop.asn">AS{{hop.asn.replace(/^AS/i,'')}}</strong><span>{{hop.owner||hop.isp}}</span></div>
    <span v-if="location(hop)" class="hop-location">{{location(hop)}}</span>
    <span v-if="!hop.responded" class="hop-note">该跳未返回探测响应</span>
   </li>
  </ol>
 </div>
</template>
<style scoped>
.path-topology{min-width:0;margin-top:12px}.path-source{display:grid;grid-template-columns:12px minmax(0,1fr);gap:3px 10px;align-items:center;font-size:13px;overflow-wrap:anywhere}.path-source>span:last-child{grid-column:2;font-size:11px;color:var(--c-muted)}.source-dot{width:10px;height:10px;border-radius:50%;background:var(--c-data)}.path-hops{list-style:none;margin:0 0 0 4px;padding:10px 0 0 0;border-left:2px solid var(--c-line)}.path-hops li{position:relative;display:grid;gap:4px;min-width:0;padding:8px 0 14px 18px;overflow-wrap:anywhere}.hop-dot{position:absolute;left:-5px;top:13px;width:8px;height:8px;background:var(--c-ok);border:1px solid var(--c-surface);border-radius:50%}.unknown .hop-dot{background:var(--c-surface);border-color:var(--c-muted)}.hop-heading{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:4px;font-size:11px;color:var(--c-muted)}.hop-time{font-variant-numeric:tabular-nums}.hop-ip{font-size:13px;color:var(--c-text);white-space:normal;overflow-wrap:anywhere}.hop-host,.hop-location,.hop-note{font-size:11px;color:var(--c-muted);line-height:1.5}.hop-network{display:flex;flex-wrap:wrap;gap:4px 8px;font-size:11px;line-height:1.5}.hop-network strong{font-weight:600;color:var(--c-data)}.hop-network span{color:var(--c-muted)}.unknown .hop-ip{color:var(--c-muted)}
</style>
