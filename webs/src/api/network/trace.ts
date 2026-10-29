import request from '@/utils/request';
export interface TraceProbe { id:number; name:string; region:string; supported:boolean; available:boolean; tool:string; reason:string; optionsSupported?:boolean; enriched?:boolean; lastSeenAt?:string }
export interface TraceAgent { publicHost?:string; preparing?:boolean; prepareError?:string; id:number; name:string; supported:boolean; available:boolean; tool:string; reason:string; optionsSupported?:boolean; enriched?:boolean; lastSeenAt?:string }
export interface TraceSources { panelTool:string; panelEnriched?:boolean; nextTraceVersion?:string; probes:TraceProbe[]; agents:TraceAgent[] }
export interface TraceOptions { protocol:'icmp'|'tcp'|'udp'; port:number; family:'auto'|'4'|'6' }
export interface TraceRun { id:number; target:string; protocol?:string; port?:number; family?:string; createdAt:string }
export interface TraceHop { ttl:number; ip:string; rtt:number|null; responded:boolean; text:string; hostname?:string; asn?:string; country?:string; province?:string; city?:string; owner?:string; isp?:string; latitude?:number; longitude?:number }
export interface TraceView { id:number; probeId:number; agentServerId?:number; sourceName:string; status:'queued'|'running'|'completed'|'failed'|'timeout'|'cancelled'|'unsupported'; tool:string; output:string; error:string; startedAt?:string; finishedAt?:string; hops:TraceHop[] }
export interface TraceDetail { run:TraceRun; views:TraceView[] }
export const getTraceSources=()=>request<TraceSources>({url:'/api/v1/network/traces/sources',method:'get'});
export const getTraceHistory=()=>request<TraceRun[]>({url:'/api/v1/network/traces/list',method:'get'});
export const getTraceDetail=(id:number)=>request<TraceDetail>({url:'/api/v1/network/traces/detail',method:'get',params:{id}});
export const createTrace=(data:TraceOptions & {target:string;includePanel:boolean;probeIds:number[];agentIds:number[]})=>request<TraceRun>({url:'/api/v1/network/traces/create',method:'post',data});
export const cancelTrace=(id:number)=>request({url:'/api/v1/network/traces/cancel',method:'post',data:{id}});

export const prepareTraceAgents=(agentIds:number[])=>request({url:'/api/v1/network/traces/prepare',method:'post',data:{agentIds}});
