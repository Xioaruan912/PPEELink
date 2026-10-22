<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { Refresh, Position } from '@element-plus/icons-vue';
import { PageHeader, UiButton, UiPanel, EmptyState, UiSkeleton } from '@/ui';
import { getTraceSources,getTraceHistory,getTraceDetail,createTrace,cancelTrace,type TraceSources,type TraceRun,type TraceDetail,type TraceView } from '@/api/network/trace';
import { formatDateTime } from '@/utils/date';
const router=useRouter();
const target=ref('');const includePanel=ref(true);const agentIds=ref<number[]>([]);
const sources=ref<TraceSources>({panelTool:'',probes:[],agents:[]});const history=ref<TraceRun[]>([]);
const detail=ref<TraceDetail|null>(null);const selectedId=ref<number>();
const loading=ref(true),starting=ref(false),cancelling=ref(false),detailLoading=ref(false);
const sourceError=ref(''),detailError=ref(''),historyError=ref('');
let visible=true;let timer:ReturnType<typeof setTimeout>|undefined;let detailSequence=0;
const running=computed(()=>detail.value?.views.some(v=>v.status==='queued'||v.status==='running')||false);
const remoteCount=computed(()=>agentIds.value.length);
const canStart=computed(()=>!loading.value&&!sourceError.value&&!starting.value&&!running.value&&target.value.trim().length>0&&(includePanel.value||remoteCount.value>0));
const statusText:Record<TraceView['status'],string>={queued:'等待来源执行',running:'追踪中',completed:'追踪结束',failed:'执行失败',timeout:'等待超时',cancelled:'已取消',unsupported:'来源未就绪'};
function clearTimer(){if(timer)clearTimeout(timer);timer=undefined}
function schedule(){clearTimer();if(visible&&running.value)timer=setTimeout(()=>void loadDetail(false),2000)}
async function loadSources(){
 loading.value=true;sourceError.value='';historyError.value='';
 const [s,h]=await Promise.allSettled([getTraceSources(),getTraceHistory()]);
 if(s.status==='fulfilled'){sources.value=s.value.data;agentIds.value=agentIds.value.filter(id=>sources.value.agents.some(a=>a.id===id&&a.available));if(!sources.value.panelTool)includePanel.value=false}else sourceError.value='无法读取追踪来源，请检查连接后重试。';
 if(h.status==='fulfilled')history.value=h.value.data||[];else historyError.value='历史记录暂时无法加载。';
 loading.value=false;
}
async function loadDetail(showLoading=true){
 if(!selectedId.value)return;const id=selectedId.value,sequence=++detailSequence;
 if(showLoading)detailLoading.value=true;detailError.value='';
 try{const res=await getTraceDetail(id);if(sequence!==detailSequence||id!==selectedId.value)return;detail.value=res.data}
 catch{if(sequence===detailSequence)detailError.value='追踪详情读取失败；已收到的结果仍保留，可重试。'}
 finally{if(sequence===detailSequence){detailLoading.value=false;schedule()}}
}
async function chooseRun(id:number|undefined){clearTimer();detailSequence++;selectedId.value=id;detail.value=null;if(id)await loadDetail()}
async function start(){
 if(!canStart.value)return;starting.value=true;detailError.value='';
 try{const res=await createTrace({target:target.value.trim(),includePanel:includePanel.value,probeIds:[],agentIds:agentIds.value});history.value=[res.data,...history.value].slice(0,30);await chooseRun(res.data.id)}
 catch{detailError.value='无法创建追踪，请检查目标与来源后重试。'}finally{starting.value=false}
}
async function cancel(){if(!selectedId.value)return;cancelling.value=true;try{await cancelTrace(selectedId.value);await loadDetail(false)}catch{detailError.value='取消失败，请重试。'}finally{cancelling.value=false}}
onMounted(()=>void loadSources());
onActivated(()=>{visible=true;schedule()});onDeactivated(()=>{visible=false;clearTimer()});onBeforeUnmount(()=>{visible=false;detailSequence++;clearTimer()});
</script>
<template>
 <div class="trace-page">
  <PageHeader title="多地路由追踪" description="对比面板、自建服务器 agent 与探针到目标的直接网络路径；不经过代理节点。">
   <template #actions><UiButton :icon="Refresh" :loading="loading" @click="loadSources">刷新来源</UiButton></template>
  </PageHeader>
  <div class="trace-layout">
   <aside class="trace-side">
    <UiPanel title="发起追踪" description="域名或 IPv4 / IPv6 地址，最多 24 跳。">
     <form class="trace-form" @submit.prevent="start">
      <label for="trace-target">目标地址</label>
      <el-input id="trace-target" v-model="target" placeholder="example.com 或 1.1.1.1" maxlength="253" :disabled="starting" />
      <div class="source-heading"><strong>追踪来源</strong><span>最多 8 个远程来源</span></div>
      <UiSkeleton v-if="loading" :lines="3" height="28px" />
      <div v-else-if="sourceError" class="inline-error" role="alert">{{sourceError}}<UiButton variant="ghost" @click="loadSources">重试</UiButton></div>
      <div v-else class="sources">
       <label class="source-row"><input v-model="includePanel" type="checkbox" :disabled="!sources.panelTool" /><span><strong>面板服务器</strong><small>{{sources.panelTool||'未安装 traceroute / tracepath'}}</small></span></label>
       <template v-if="sources.agents.length"><p class="source-group">自建服务器 agent</p><label v-for="agent in sources.agents" :key="`agent-${agent.id}`" class="source-row"><input v-model="agentIds" :value="agent.id" type="checkbox" :disabled="!agent.available || (remoteCount>=8&&!agentIds.includes(agent.id))" /><span><strong>{{agent.name}}</strong><small>{{agent.available?agent.tool:agent.reason}}</small></span></label></template>
       <p v-if="!sources.agents.length" class="muted">尚未接入自建服务器 agent。可以先使用面板来源。</p>
      </div>
      <p class="help">来源缺少工具时，请安装系统软件包 <code>traceroute</code>。旧版 agent/探针需先更新；新版每 5 秒轮询追踪任务。</p>
      <UiButton variant="ghost" type="button" @click="router.push('/system/settings')">管理检测探针</UiButton>
      <UiButton variant="primary" :icon="Position" :loading="starting" :disabled="!canStart" type="submit">开始追踪</UiButton>
      <p v-if="!canStart&&!starting" class="help">{{running?'请等待当前追踪结束，或先取消。':'填写目标并选择至少一个已就绪的来源。'}}</p>
     </form>
    </UiPanel>
    <UiPanel title="最近追踪" description="保留 7 天，显示最近 30 次。">
     <p v-if="historyError" class="inline-error" role="status">{{historyError}}</p>
     <EmptyState v-if="!loading&&!history.length" size="inline" description="尚无追踪记录。" />
     <div class="history-list"><button v-for="run in history" :key="run.id" :class="{'is-active':selectedId===run.id}" @click="chooseRun(run.id)"><strong>{{run.target}}</strong><small>{{formatDateTime(run.createdAt)}}</small></button></div>
    </UiPanel>
   </aside>
   <section class="trace-results" aria-label="路由追踪结果" :aria-busy="detailLoading">
    <div v-if="detailError" class="result-error" role="alert"><span>{{detailError}}</span><UiButton v-if="selectedId" @click="loadDetail()">重试</UiButton></div>
    <UiSkeleton v-if="detailLoading&&!detail" :lines="5" height="48px" />
    <UiPanel v-else-if="!detail" title="路径对比"><EmptyState size="section" title="从不同网络看同一条路" description="填写目标并选择来源后，各来源会独立追踪。结果逐步返回，也可打开历史记录。" /></UiPanel>
    <template v-else>
     <div class="result-heading"><div><h2>{{detail.run.target}}</h2><p>{{formatDateTime(detail.run.createdAt)}} · {{detail.views.length}} 个来源</p></div><UiButton v-if="running" :loading="cancelling" @click="cancel">取消追踪</UiButton><UiButton v-else :icon="Refresh" @click="loadDetail()">刷新结果</UiButton></div>
     <p class="interpretation">“无应答”只代表该跳未返回探测响应，不等于链路断开或业务丢包。追踪结束也不保证到达目标；请结合最后跳点与原始输出判断。取消后远程命令最多等待 75 秒停止，结果不会再写入。</p>
     <div class="result-grid">
      <UiPanel v-for="view in detail.views" :key="view.id" :title="view.sourceName" :description="view.tool||'来源环境检查'">
       <template #actions><span class="trace-status" :class="view.status" role="status">{{statusText[view.status]}}</span></template>
       <p v-if="view.error" class="inline-error">{{view.error}}</p>
       <p v-if="view.status==='queued'||view.status==='running'" class="muted">{{view.status==='queued'?'等待来源领取任务…':'正在逐跳探测，结果完成后返回…'}}</p>
       <div v-if="view.hops.length" class="hop-table"><div class="hop-head"><span>跳数</span><span>地址</span><span>耗时</span></div><div v-for="(hop,index) in view.hops" :key="index" class="hop-row"><span>{{hop.ttl}}</span><code>{{hop.ip||'无应答'}}</code><span>{{hop.rtt===null?'—':`${hop.rtt.toFixed(2)} ms`}}</span></div></div>
       <p v-else-if="view.status==='completed'" class="muted">未解析到标准跳点，请展开原始输出。</p>
       <details v-if="view.output" class="raw-output"><summary>原始输出</summary><pre>{{view.output}}</pre></details>
      </UiPanel>
     </div>
    </template>
   </section>
  </div>
 </div>
</template>
<style scoped>
.trace-page{min-width:0}.trace-layout{display:grid;grid-template-columns:300px minmax(0,1fr);gap:14px;align-items:start}.trace-side,.trace-results{min-width:0;display:grid;gap:12px}.trace-form{display:grid;gap:10px}.trace-form>label,.source-heading strong{font-size:13px;font-weight:500}.source-heading{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:4px;margin-top:4px}.source-heading>span{font-size:11px;color:var(--c-muted)}.source-group{margin:4px 0 0;color:var(--c-muted);font-size:11px}.sources{display:grid;gap:8px}.source-row{display:flex;align-items:flex-start;gap:9px;padding:8px;border:1px solid var(--c-line);border-radius:10px}.source-row input{margin-top:4px;accent-color:var(--c-signal-text);width:16px;height:16px;flex-shrink:0}.source-row span{min-width:0;display:grid;gap:2px}.source-row strong{font-size:13px;font-weight:500;overflow-wrap:anywhere}.source-row small,.help,.muted{color:var(--c-muted);font-size:12px;line-height:1.7}.source-row em{font-size:11px;color:var(--c-muted);font-style:normal;margin-left:6px}.help,.muted{margin:0}.inline-error{font-size:13px;line-height:1.6;color:var(--c-warn-ink);overflow-wrap:anywhere}.result-error{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:12px;border:1px solid var(--c-line);border-radius:12px;background:var(--c-surface)}.history-list{display:grid;max-height:300px;overflow:auto;gap:4px}.history-list button{display:grid;text-align:left;gap:2px;padding:8px;border-radius:8px;min-width:0}.history-list button:hover,.history-list button.is-active{background:var(--c-selected)}.history-list strong{font-size:13px;font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.history-list small{font-size:11px;color:var(--c-muted)}button:focus-visible,summary:focus-visible{outline:2px solid var(--c-focus);outline-offset:2px}.result-heading{display:flex;align-items:center;justify-content:space-between;gap:12px}.result-heading h2{font-size:18px;overflow-wrap:anywhere;margin:0}.result-heading p{font-size:12px;color:var(--c-muted);margin:3px 0 0}.interpretation{font-size:12px;line-height:1.7;color:var(--c-muted);margin:0}.result-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,320px),1fr));gap:12px;align-items:start}.trace-status{font-size:12px;color:var(--c-muted)}.trace-status.running,.trace-status.queued{color:var(--c-data)}.trace-status.failed,.trace-status.timeout,.trace-status.unsupported{color:var(--c-warn-ink)}.trace-status.completed{color:var(--c-ok)}.hop-table{display:grid;min-width:0;margin-top:8px}.hop-head,.hop-row{display:grid;grid-template-columns:32px minmax(0,1fr) 78px;gap:8px;align-items:start;padding:7px 0;border-bottom:1px solid var(--c-line);font-size:12px}.hop-head{color:var(--c-muted)}.hop-row code{font-size:12px;overflow-wrap:anywhere}.hop-row>span:last-child{text-align:right;font-variant-numeric:tabular-nums}.raw-output{margin-top:12px;font-size:12px}.raw-output summary{cursor:pointer;color:var(--c-muted);padding:4px 0}.raw-output pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--c-surface-2);border-radius:8px;padding:10px;max-height:260px;overflow:auto;font-size:11px}
@media(max-width:1000px){.trace-layout{grid-template-columns:1fr}.trace-side{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.trace-side{grid-template-columns:1fr}.result-heading{align-items:flex-start}.trace-layout{gap:12px}}
</style>
