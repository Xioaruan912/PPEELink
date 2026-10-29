<script setup lang="ts">
import { computed, onActivated, onBeforeUnmount, onDeactivated, onMounted, ref, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { Refresh, Position } from '@element-plus/icons-vue';
import { PageHeader, UiButton, UiPanel, EmptyState, UiSkeleton } from '@/ui';
import { getTraceSources,getTraceHistory,getTraceDetail,createTrace,cancelTrace,prepareTraceAgents,type TraceSources,type TraceRun,type TraceDetail,type TraceView } from '@/api/network/trace';
import PathTopology from './PathTopology.vue';
import { formatDateTime } from '@/utils/date';
const router=useRouter();const route=useRoute();
const target=ref('');const landingId=ref('');const includePanel=ref(true);const agentIds=ref<number[]>([]);const probeIds=ref<number[]>([]);
const protocol=ref<'icmp'|'tcp'|'udp'>('icmp');const port=ref(443);const family=ref<'auto'|'4'|'6'>('4');
const legacyOptions=computed(()=>protocol.value==='udp'&&port.value===33434&&family.value==='auto');
function supportsOptions(source:{available:boolean;optionsSupported?:boolean;preparing?:boolean}){return source.available&&!source.preparing&&(source.optionsSupported||legacyOptions.value)}
function prefill(){
 const q=route.query;if(typeof q.target==='string')target.value=q.target;
 const raw=q.sourceAgentId||q.agentId||q.agentIds;
 if(typeof raw==='string'){agentIds.value=raw.split(',').map(Number).filter(id=>Number.isInteger(id)&&id>0);includePanel.value=false}
 if(q.family==='4'||q.family==='6')family.value=q.family;
}
watch(()=>route.query,prefill,{immediate:true});
watch(protocol,(value)=>{if(value==='tcp')port.value=443;if(value==='udp')port.value=33434});
watch([protocol,port,family],()=>{agentIds.value=agentIds.value.filter(id=>sources.value.agents.some(a=>a.id===id&&supportsOptions(a)));probeIds.value=probeIds.value.filter(id=>sources.value.probes.some(p=>p.id===id&&supportsOptions(p)))});
const sources=ref<TraceSources>({panelTool:'',probes:[],agents:[]});const history=ref<TraceRun[]>([]);
const detail=ref<TraceDetail|null>(null);const selectedId=ref<number>();
const loading=ref(true),starting=ref(false),cancelling=ref(false),detailLoading=ref(false);
const sourceError=ref(''),detailError=ref(''),historyError=ref('');
const preparingId=ref<number>();let prepareTimer:ReturnType<typeof setTimeout>|undefined;
function clearPrepareTimer(){if(prepareTimer)clearTimeout(prepareTimer);prepareTimer=undefined}
function schedulePreparation(){clearPrepareTimer();if(visible&&sources.value.agents.some(a=>a.preparing))prepareTimer=setTimeout(()=>void refreshPreparation(),3000)}
async function refreshPreparation(){try{const res=await getTraceSources();sources.value=res.data}catch{detailError.value='准备状态读取失败，请刷新来源重试。'}finally{schedulePreparation()}}
async function prepareAgent(id:number){preparingId.value=id;detailError.value='';try{await prepareTraceAgents([id]);await refreshPreparation()}catch{detailError.value='无法安排准备 NextTrace，请确认 agent 已更新并在线。'}finally{preparingId.value=undefined}}
let visible=true;let timer:ReturnType<typeof setTimeout>|undefined;let detailSequence=0;
const running=computed(()=>detail.value?.views.some(v=>v.status==='queued'||v.status==='running')||false);
const remoteCount=computed(()=>agentIds.value.length+probeIds.value.length);
const canStart=computed(()=>!loading.value&&!sourceError.value&&!starting.value&&!running.value&&target.value.trim().length>0&&(includePanel.value||remoteCount.value>0));
const landings=computed(()=>sources.value.agents.filter(agent=>agent.publicHost?.trim()));
function chooseLanding(){const landing=sources.value.agents.find(agent=>String(agent.id)===landingId.value);if(landing?.publicHost){target.value=landing.publicHost;if(landing.publicHost.includes(':'))family.value='6';else if(/^\d{1,3}(\.\d{1,3}){3}$/.test(landing.publicHost))family.value='4'}}
watch(target,(value)=>{const landing=sources.value.agents.find(agent=>String(agent.id)===landingId.value);if(landing?.publicHost!==value)landingId.value=''});
const statusText:Record<TraceView['status'],string>={queued:'等待来源执行',running:'追踪中',completed:'追踪结束',failed:'执行失败',timeout:'等待超时',cancelled:'已取消',unsupported:'来源未就绪'};
function clearTimer(){if(timer)clearTimeout(timer);timer=undefined}
function schedule(){clearTimer();if(visible&&running.value)timer=setTimeout(()=>void loadDetail(false),2000)}
async function loadSources(){
 loading.value=true;sourceError.value='';historyError.value='';
 const [s,h]=await Promise.allSettled([getTraceSources(),getTraceHistory()]);
 if(s.status==='fulfilled'){sources.value=s.value.data;agentIds.value=agentIds.value.filter(id=>sources.value.agents.some(a=>a.id===id&&supportsOptions(a)));probeIds.value=probeIds.value.filter(id=>sources.value.probes.some(p=>p.id===id&&supportsOptions(p)));if(!sources.value.panelTool)includePanel.value=false}else sourceError.value='无法读取追踪来源，请检查连接后重试。';
 if(h.status==='fulfilled')history.value=h.value.data||[];else historyError.value='历史记录暂时无法加载。';
 loading.value=false;schedulePreparation();
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
 try{const res=await createTrace({target:target.value.trim(),includePanel:includePanel.value,probeIds:probeIds.value,agentIds:agentIds.value,protocol:protocol.value,port:protocol.value==='icmp'?0:port.value,family:family.value});history.value=[res.data,...history.value].slice(0,30);await chooseRun(res.data.id)}
 catch{detailError.value='无法创建追踪，请检查目标与来源后重试。'}finally{starting.value=false}
}
async function cancel(){if(!selectedId.value)return;cancelling.value=true;try{await cancelTrace(selectedId.value);await loadDetail(false)}catch{detailError.value='取消失败，请重试。'}finally{cancelling.value=false}}
onMounted(()=>void loadSources());
onActivated(()=>{visible=true;schedule();schedulePreparation()});onDeactivated(()=>{visible=false;clearTimer();clearPrepareTimer()});onBeforeUnmount(()=>{visible=false;detailSequence++;clearTimer();clearPrepareTimer()});
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
      <template v-if="landings.length"><label for="trace-landing">选择已有落地（可选）</label><select id="trace-landing" v-model="landingId" class="landing-select" @change="chooseLanding"><option value="">选择服务器以填写目标地址</option><option v-for="landing in landings" :key="landing.id" :value="String(landing.id)">{{landing.name}} · {{landing.publicHost}}</option></select></template>
      <div class="trace-options">
       <label>探测协议<el-select v-model="protocol" aria-label="探测协议"><el-option label="ICMP" value="icmp" /><el-option label="TCP" value="tcp" /><el-option label="UDP" value="udp" /></el-select></label>
       <label>IP 版本<el-select v-model="family" aria-label="IP 版本"><el-option label="IPv4" value="4" /><el-option label="IPv6" value="6" /><el-option label="自动" value="auto" /></el-select></label>
      </div>
      <label v-if="protocol!=='icmp'">目标端口<el-input-number v-model="port" :min="1" :max="65535" controls-position="right" aria-label="目标端口" /></label>
      <div class="source-heading"><strong>追踪来源</strong><span>最多 8 个远程来源</span></div>
      <UiSkeleton v-if="loading" :lines="3" height="28px" />
      <div v-else-if="sourceError" class="inline-error" role="alert">{{sourceError}}<UiButton variant="ghost" @click="loadSources">重试</UiButton></div>
      <div v-else class="sources">
       <label class="source-row"><input v-model="includePanel" type="checkbox" :disabled="!sources.panelTool" /><span><strong>面板服务器</strong><small>{{sources.panelTool||'未安装追踪工具'}}</small></span></label>
       <template v-if="sources.agents.length"><p class="source-group">自建服务器 agent</p><label v-for="agent in sources.agents" :key="`agent-${agent.id}`" class="source-row"><input v-model="agentIds" :value="agent.id" type="checkbox" :disabled="!supportsOptions(agent) || (remoteCount>=8&&!agentIds.includes(agent.id))" /><span><strong>{{agent.name}}</strong><small>{{!agent.available?agent.reason:!supportsOptions(agent)?'需要更新 agent 以选择协议、端口和 IP 版本':agent.tool}}</small><small v-if="agent.prepareError" class="inline-error" role="status">{{agent.prepareError}}</small><UiButton v-if="agent.optionsSupported&&!agent.enriched" variant="ghost" type="button" :loading="agent.preparing||preparingId===agent.id" :disabled="!!agent.preparing||preparingId!==undefined" @click.prevent.stop="prepareAgent(agent.id)">{{agent.preparing?'准备 NextTrace 中':'准备 NextTrace'}}</UiButton></span></label></template>
       <template v-if="sources.probes.length"><p class="source-group">检测探针</p><label v-for="probe in sources.probes" :key="`probe-${probe.id}`" class="source-row"><input v-model="probeIds" :value="probe.id" type="checkbox" :disabled="!supportsOptions(probe)||(remoteCount>=8&&!probeIds.includes(probe.id))" /><span><strong>{{probe.name}}</strong><small>{{!probe.available?probe.reason:!supportsOptions(probe)?'需要更新探针以选择协议、端口和 IP 版本':probe.tool}}</small></span></label></template>
       <p v-if="!sources.agents.length" class="muted">尚未接入自建服务器 agent。可以先使用面板来源。</p>
      </div>
      <p class="help">NextTrace v{{sources.nextTraceVersion||'1.7.4'}} 提供 ASN、运营商与位置。面板{{sources.panelEnriched?'已就绪':'尚未安装匹配版本'}}；可点击来源的“准备 NextTrace”，由 agent 下载并校验官方 SHA256 后安装（Linux amd64 / arm64）；traceroute / tracepath 仍可追踪，tracepath 仅支持 UDP。旧版来源可选择 UDP / 33434 / 自动，或更新后使用完整选项。</p>
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
     <div class="result-heading"><div><h2>{{detail.run.target}}</h2><p>{{formatDateTime(detail.run.createdAt)}} · {{detail.views.length}} 个来源 · {{(detail.run.protocol||'udp').toUpperCase()}}{{detail.run.port?` / ${detail.run.port}`:''}} · {{detail.run.family==='4'?'IPv4':detail.run.family==='6'?'IPv6':'自动 IP 版本'}}</p></div><UiButton v-if="running" :loading="cancelling" @click="cancel">取消追踪</UiButton><UiButton v-else :icon="Refresh" @click="loadDetail()">刷新结果</UiButton></div>
     <p class="interpretation">“无应答”只代表该跳未返回探测响应，不等于链路断开或业务丢包。追踪结束也不保证到达目标；请结合最后跳点与原始输出判断。取消后远程命令最多等待 75 秒停止，结果不会再写入。</p>
     <div class="result-grid">
      <UiPanel v-for="view in detail.views" :key="view.id" :title="view.sourceName" :description="view.tool||'来源环境检查'">
       <template #actions><span class="trace-status" :class="view.status" role="status">{{statusText[view.status]}}</span></template>
       <p v-if="view.error" class="inline-error">{{view.error}}</p>
       <p v-if="view.tool==='nexttrace'&&view.hops.some(h=>h.responded)&&!view.hops.some(h=>h.asn)" class="muted">该来源未返回 ASN / 位置信息，路径仍可查看。</p>
       <p v-if="view.status==='queued'||view.status==='running'" class="muted">{{view.status==='queued'?'等待来源领取任务…':'正在逐跳探测，结果完成后返回…'}}</p>
       <PathTopology v-if="view.hops.length" :source="view.sourceName" :source-id="view.id" :hops="view.hops" />
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
.trace-page{min-width:0}.trace-layout{display:grid;grid-template-columns:300px minmax(0,1fr);gap:14px;align-items:start}.trace-side,.trace-results{min-width:0;display:grid;gap:12px}.trace-form{display:grid;gap:10px}.landing-select{width:100%;min-width:0;max-width:100%;padding:8px 10px;border:1px solid var(--c-line);border-radius:8px;background:var(--c-surface);color:var(--c-ink);font-size:12px;min-height:36px;text-overflow:ellipsis}.trace-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}.trace-options label{display:grid;gap:6px;min-width:0;font-size:12px}.trace-form .el-input-number{max-width:100%;display:flex;margin-top:6px}.trace-form>label,.source-heading strong{font-size:13px;font-weight:500}.source-heading{display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:4px;margin-top:4px}.source-heading>span{font-size:11px;color:var(--c-muted)}.source-group{margin:4px 0 0;color:var(--c-muted);font-size:11px}.sources{display:grid;gap:8px}.source-row{display:flex;align-items:flex-start;gap:9px;padding:8px;border:1px solid var(--c-line);border-radius:10px}.source-row input{margin-top:4px;accent-color:var(--c-signal-text);width:16px;height:16px;flex-shrink:0}.source-row span{min-width:0;display:grid;gap:2px}.source-row strong{font-size:13px;font-weight:500;overflow-wrap:anywhere}.source-row small,.help,.muted{color:var(--c-muted);font-size:12px;line-height:1.7}.source-row em{font-size:11px;color:var(--c-muted);font-style:normal;margin-left:6px}.help,.muted{margin:0}.inline-error{font-size:13px;line-height:1.6;color:var(--c-warn-ink);overflow-wrap:anywhere}.result-error{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:12px;border:1px solid var(--c-line);border-radius:12px;background:var(--c-surface)}.history-list{display:grid;max-height:300px;overflow:auto;gap:4px}.history-list button{display:grid;text-align:left;gap:2px;padding:8px;border-radius:8px;min-width:0}.history-list button:hover,.history-list button.is-active{background:var(--c-selected)}.history-list strong{font-size:13px;font-weight:500;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.history-list small{font-size:11px;color:var(--c-muted)}button:focus-visible,summary:focus-visible{outline:2px solid var(--c-focus);outline-offset:2px}.result-heading{display:flex;align-items:center;justify-content:space-between;gap:12px}.result-heading h2{font-size:18px;overflow-wrap:anywhere;margin:0}.result-heading p{font-size:12px;color:var(--c-muted);margin:3px 0 0}.interpretation{font-size:12px;line-height:1.7;color:var(--c-muted);margin:0}.result-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,320px),1fr));gap:12px;align-items:start}.trace-status{font-size:12px;color:var(--c-muted)}.trace-status.running,.trace-status.queued{color:var(--c-data)}.trace-status.failed,.trace-status.timeout,.trace-status.unsupported{color:var(--c-warn-ink)}.trace-status.completed{color:var(--c-ok)}.hop-table{display:grid;min-width:0;margin-top:8px}.hop-head,.hop-row{display:grid;grid-template-columns:32px minmax(0,1fr) 78px;gap:8px;align-items:start;padding:7px 0;border-bottom:1px solid var(--c-line);font-size:12px}.hop-head{color:var(--c-muted)}.hop-row code{font-size:12px;overflow-wrap:anywhere}.hop-row>span:last-child{text-align:right;font-variant-numeric:tabular-nums}.raw-output{margin-top:12px;font-size:12px}.raw-output summary{cursor:pointer;color:var(--c-muted);padding:4px 0}.raw-output pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--c-surface-2);border-radius:8px;padding:10px;max-height:260px;overflow:auto;font-size:11px}
@media(max-width:1000px){.trace-layout{grid-template-columns:1fr}.trace-side{grid-template-columns:repeat(2,minmax(0,1fr))}}@media(max-width:600px){.trace-side{grid-template-columns:1fr}.result-heading{align-items:flex-start}.trace-layout{gap:12px}}
</style>
