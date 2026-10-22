import request from '@/utils/request';
export interface TraceProbe { id:number; name:string; region:string; supported:boolean; available:boolean; tool:string; reason:string; lastSeenAt?:string }
export interface TraceSources { panelTool:string; probes:TraceProbe[] }
export interface TraceRun { id:number; target:string; createdAt:string }
export interface TraceHop { ttl:number; ip:string; rtt:number|null; responded:boolean; text:string }
export interface TraceView { id:number; probeId:number; sourceName:string; status:'queued'|'running'|'completed'|'failed'|'timeout'|'cancelled'|'unsupported'; tool:string; output:string; error:string; startedAt?:string; finishedAt?:string; hops:TraceHop[] }
export interface TraceDetail { run:TraceRun; views:TraceView[] }
export const getTraceSources=()=>request<TraceSources>({url:'/api/v1/network/traces/sources',method:'get'});
export const getTraceHistory=()=>request<TraceRun[]>({url:'/api/v1/network/traces/list',method:'get'});
export const getTraceDetail=(id:number)=>request<TraceDetail>({url:'/api/v1/network/traces/detail',method:'get',params:{id}});
export const createTrace=(data:{target:string;includePanel:boolean;probeIds:number[]})=>request<TraceRun>({url:'/api/v1/network/traces/create',method:'post',data});
export const cancelTrace=(id:number)=>request({url:'/api/v1/network/traces/cancel',method:'post',data:{id}});
