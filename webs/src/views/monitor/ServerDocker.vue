<script setup lang="ts">
import IconContainer from "~icons/ph/cube";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { UiButton, UiDialog, UiSkeleton } from "@/ui";
import { actOnContainer, containerLogs, containerStats, listContainers, type DockerAction, type DockerContainer, type DockerStats } from "@/api/monitor/operations";
const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading"); const items = ref<DockerContainer[]>([]); const stats = ref<DockerStats[]>([]);
const error = ref(""); const statsError = ref(""); const status = ref(""); const busy = ref(false); const statsBusy = ref(false); const updated = ref("");
const locked = computed(() => busy.value || phase.value === "loading" || props.offline);
const running = computed(() => items.value.filter(item => item.State === "running").length);
const statIndex = computed(() => new Map(stats.value.flatMap(item => [item.ID, item.Container, item.Name].filter(Boolean).map(key => [key, item] as const))));
const statFor = (container: DockerContainer) => statIndex.value.get(container.ID) || statIndex.value.get(container.Names);
let disposed = false; let generation = 0;
const message = (e: unknown) => e instanceof Error ? e.message : "操作失败，请检查 Docker 服务和 agent 权限后重试。";
async function load() {
 if (props.offline) return; const seq = ++generation; phase.value = "loading"; error.value = "";
 try { const data = await listContainers(props.id); if (disposed || seq !== generation) return; items.value = data.items; updated.value = new Date().toLocaleTimeString(); phase.value = "ready"; }
 catch (e) { if (!disposed && seq === generation) { error.value = message(e); phase.value = "error"; } }
}
async function refreshStats() {
 if (props.offline || statsBusy.value) return; statsBusy.value = true; statsError.value = "";
 try { const data = await containerStats(props.id); if (!disposed) stats.value = data.items; }
 catch (e) { if (!disposed) statsError.value = message(e); } finally { statsBusy.value = false; }
}
const pending = ref<{ item: DockerContainer; action: DockerAction }>(); const confirmError = ref("");
const actionName = (action: DockerAction) => ({ start: "启动", stop: "停止", restart: "重启" })[action];
function confirm(item: DockerContainer, action: DockerAction) { pending.value = { item, action }; confirmError.value = ""; }
async function act() {
 if (!pending.value || locked.value) return; const { item, action } = pending.value; busy.value = true; confirmError.value = "";
 try { await actOnContainer(props.id, item.ID, action); if (disposed) return; pending.value = undefined; status.value = `已${actionName(action)} ${item.Names || item.ID.slice(0, 12)}。`; await load(); await refreshStats(); }
 catch (e) { confirmError.value = message(e); } finally { busy.value = false; }
}
const logsOpen = ref(false); const logsItem = ref<DockerContainer>(); const logs = ref(""); const logsError = ref(""); const logsBusy = ref(false); const logsTruncated = ref(false);
async function showLogs(item: DockerContainer) {
 if (props.offline || logsBusy.value) return; logsItem.value = item; logsOpen.value = true; logsBusy.value = true; logsError.value = ""; logs.value = ""; logsTruncated.value = false;
 try { const data = await containerLogs(props.id, item.ID); if (!disposed) { logs.value = data.content; logsTruncated.value = data.truncated; } }
 catch (e) { if (!disposed) logsError.value = message(e); } finally { logsBusy.value = false; }
}
onMounted(() => { load(); refreshStats(); }); onBeforeUnmount(() => { disposed = true; generation++; });
</script>

<template>
 <section class="docker-console" aria-label="Docker 容器">
  <header class="console-heading"><div><span class="eyebrow">CONTAINERS</span><h2>容器工作台</h2><p>查看运行状态、资源占用与日志。</p></div><div class="row-actions"><UiButton :disabled="locked" @click="load()">刷新列表</UiButton><UiButton :loading="statsBusy" :disabled="offline || busy" @click="refreshStats">刷新资源</UiButton></div></header>
  <div class="docker-overview" aria-label="容器概览"><div><span>全部容器</span><strong>{{ phase === 'loading' && !items.length ? '—' : items.length }}</strong></div><div><span><i class="status-dot" />运行中</span><strong>{{ running }}</strong></div><div><span>未运行</span><strong>{{ items.length - running }}</strong></div><div class="snapshot-time"><span>列表更新</span><strong>{{ updated || '等待连接' }}</strong></div></div>
  <p v-if="offline" class="notice" role="status">网络已断开，当前展示上次结果。恢复连接后可刷新。</p>
  <p v-if="busy" class="notice" role="status">正在操作容器…</p>
  <div v-if="error" class="notice error" role="alert">{{ error }} <UiButton :disabled="locked" @click="load()">重试列表</UiButton></div>
  <div v-if="statsError" class="notice error" role="alert">资源暂不可用：{{ statsError }} <UiButton :disabled="offline || statsBusy" @click="refreshStats">重试资源</UiButton></div>
  <p v-if="status" class="notice" role="status">{{ status }}</p>
  <UiSkeleton v-if="phase === 'loading' && !items.length" :lines="5" aria-label="正在读取 Docker 容器" />
  <div v-else class="container-grid" :aria-busy="phase === 'loading'">
   <article v-for="item in items" :key="item.ID" class="container-card" :aria-label="item.Names || item.ID">
    <header class="container-head"><div class="container-mark" aria-hidden="true"><IconContainer /></div><div class="container-title"><h3>{{ item.Names || '未命名容器' }}</h3><code>{{ item.ID.slice(0, 12) }}</code></div><span class="state" :class="{ running: item.State === 'running' }">{{ item.State === 'running' ? '运行中' : item.State === 'exited' ? '已停止' : item.State }}</span></header>
    <div class="image-name"><span>镜像</span><code>{{ item.Image }}</code></div>
    <div class="resource-pair"><div><span>CPU</span><strong>{{ statFor(item)?.CPUPerc || '—' }}</strong></div><div><span>内存</span><strong>{{ statFor(item)?.MemPerc || '—' }}</strong></div></div>
    <p v-if="!statFor(item)" class="muted snapshot-note">{{ statsBusy ? '正在读取资源…' : item.State === 'running' ? '暂无资源快照，可刷新资源。' : '容器未运行，暂无资源数据。' }}</p>
    <dl class="container-details"><div><dt>状态</dt><dd>{{ item.Status }}</dd></div><div><dt>端口</dt><dd>{{ item.Ports || '未发布' }}</dd></div><template v-if="statFor(item)"><div><dt>内存用量</dt><dd>{{ statFor(item)!.MemUsage }}</dd></div><div><dt>网络 / 磁盘</dt><dd>{{ statFor(item)!.NetIO }} / {{ statFor(item)!.BlockIO }}</dd></div><div><dt>进程</dt><dd>{{ statFor(item)!.PIDs }}</dd></div></template></dl>
    <footer class="container-actions"><UiButton :disabled="locked || logsBusy" @click="showLogs(item)">查看日志</UiButton><div class="row-actions"><UiButton v-if="item.State !== 'running'" variant="primary" :disabled="locked" @click="confirm(item, 'start')">启动</UiButton><UiButton v-else :disabled="locked" @click="confirm(item, 'stop')">停止</UiButton><UiButton :disabled="locked" @click="confirm(item, 'restart')">重启</UiButton></div></footer>
   </article>
   <div v-if="!items.length && phase === 'ready'" class="empty"><strong>还没有容器</strong><p>在这台服务器部署容器后，刷新即可查看。</p><UiButton :disabled="locked" @click="load()">刷新容器</UiButton></div>
  </div>
 </section>
 <UiDialog :model-value="!!pending" :title="pending ? '确认' + actionName(pending.action) + '容器' : '确认操作'" width="min(520px, 94vw)" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy" @update:model-value="!busy && (pending = undefined)">
  <template v-if="pending"><p class="wrap">{{ pending.item.Names }} · {{ pending.item.ID.slice(0, 12) }}</p><p>{{ pending.action === 'start' ? '启动后容器将使用服务器资源。是否继续？' : '这将中断容器内的服务及活跃连接。是否继续？' }}</p><p v-if="confirmError" class="notice error" role="alert">{{ confirmError }}</p></template>
  <template #footer><UiButton :disabled="busy" @click="pending = undefined">取消</UiButton><UiButton :variant="pending?.action === 'start' ? 'primary' : 'danger'" :loading="busy" :disabled="offline" @click="act">确认{{ pending ? actionName(pending.action) : '' }}</UiButton></template>
 </UiDialog>
 <UiDialog v-model="logsOpen" title="容器日志 · 最近 500 行" width="min(1000px, 96vw)">
  <div class="toolbar"><code>{{ logsItem?.Names }} · {{ logsItem?.ID.slice(0, 12) }}</code><UiButton :disabled="offline || logsBusy" @click="logsItem && showLogs(logsItem)">刷新日志</UiButton></div>
  <UiSkeleton v-if="logsBusy" :lines="5" /><p v-else-if="logsError" class="notice error" role="alert">{{ logsError }} 请检查日志权限后点击刷新。</p><pre v-else class="logs" tabindex="0" aria-label="容器只读日志">{{ logs || '暂无日志输出。容器可能尚未启动，或使用不支持读取的日志驱动。' }}</pre><p v-if="logsTruncated" class="notice" role="status">输出已截断至 2 MiB，可通过 Agent 终端查看完整日志。</p>
  <template #footer><UiButton @click="logsOpen = false">关闭</UiButton></template>
 </UiDialog>
</template>

<style scoped>
.console-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:22px}.eyebrow{font:11px var(--font-mono);letter-spacing:.16em;color:var(--c-muted)}h2{font-size:24px;margin:6px 0;font-weight:600;letter-spacing:-.04em}.console-heading p{margin:0;color:var(--c-muted);font-size:13px}.docker-overview{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));border:1px solid var(--c-line);border-radius:12px;margin-bottom:22px;background:var(--c-surface)}.docker-overview>div{padding:18px 22px;display:grid;gap:12px;border-right:1px solid var(--c-line)}.docker-overview>div:last-child{border:0}.docker-overview span{font-size:12px;color:var(--c-muted)}.docker-overview strong{font:500 28px var(--font-mono)}.docker-overview .snapshot-time strong{font-size:13px;align-self:center}.status-dot{display:inline-block;width:6px;height:6px;border-radius:50%;background:var(--c-ok);margin-right:6px}.container-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px}.container-card{min-width:0;border:1px solid var(--c-line);border-radius:14px;background:var(--c-surface);display:flex;flex-direction:column;overflow:hidden;transition:border-color .2s,box-shadow .2s}.container-card:hover{border-color:var(--c-muted);box-shadow:var(--shadow-panel)}.container-head{display:flex;align-items:center;gap:12px;padding:20px}.container-mark{display:grid;place-items:center;width:38px;height:38px;flex-shrink:0;border:1px solid var(--c-line);border-radius:9px;background:var(--c-surface-2);font-size:24px}.container-mark svg{width:22px;height:22px}.container-title{min-width:0;flex:1}.container-title h3{margin:0 0 5px;font-size:15px;overflow-wrap:anywhere}.container-title code{font-size:11px;color:var(--c-muted)}.state{font-size:11px;white-space:nowrap;border:1px solid var(--c-line);border-radius:6px;padding:4px 7px;color:var(--c-muted)}.running{color:var(--c-ok);background:color-mix(in srgb,var(--c-ok) 7%,transparent);border-color:color-mix(in srgb,var(--c-ok) 25%,var(--c-line))}.image-name{display:grid;gap:6px;margin:0 20px 18px;font-size:12px;overflow-wrap:anywhere}.image-name span{color:var(--c-muted)}.resource-pair{display:grid;grid-template-columns:1fr 1fr;margin:0 20px;padding:16px 0;border-block:1px solid var(--c-line)}.resource-pair>div{display:grid;gap:8px}.resource-pair span{font-size:11px;color:var(--c-muted)}.resource-pair strong{font:500 24px var(--font-mono)}.container-details{margin:16px 20px;display:grid;gap:9px;font-size:12px;flex:1}.container-details>div{display:grid;grid-template-columns:76px minmax(0,1fr);gap:10px}.container-details dt{color:var(--c-muted)}.container-details dd{margin:0;overflow-wrap:anywhere;line-height:1.5}.container-actions{display:flex;justify-content:space-between;gap:8px;padding:14px 20px;border-top:1px solid var(--c-line);background:var(--c-canvas)}.toolbar,.row-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.toolbar{margin:14px 0}.toolbar code{overflow-wrap:anywhere;min-width:0}.muted{color:var(--c-muted);font-size:12px;line-height:1.6}.snapshot-note{margin:12px 20px 0}.notice{padding:12px;border:1px solid var(--c-line);border-radius:8px;background:var(--c-surface-2);overflow-wrap:anywhere;line-height:1.6;font-size:13px}.error{color:var(--c-bad);border-color:var(--c-bad)}.empty{grid-column:1/-1;padding:48px 16px;text-align:center;border:1px dashed var(--c-line);border-radius:12px;color:var(--c-muted)}.empty strong{color:var(--c-ink)}.logs{background:#17191d;color:#e4e7ec;border:1px solid var(--c-line);padding:18px;border-radius:8px;white-space:pre-wrap;overflow-wrap:anywhere;max-height:60vh;overflow:auto;font:12px/1.8 var(--font-mono)}.wrap{overflow-wrap:anywhere}.logs:focus-visible{outline:2px solid var(--c-focus);outline-offset:2px}@media(max-width:900px){.container-grid{grid-template-columns:1fr}}@media(max-width:600px){.docker-overview{grid-template-columns:1fr 1fr}.docker-overview>div{padding:14px}.docker-overview>div:nth-child(2){border-right:0}.docker-overview>div:nth-child(n+3){border-top:1px solid var(--c-line)}.container-head{padding:16px;gap:8px}.container-mark{width:30px;height:30px}.container-actions{padding:12px;flex-wrap:wrap}.container-actions :deep(button){min-height:44px;padding:8px 12px}.image-name,.resource-pair{margin-inline:16px}.console-heading h2{font-size:22px}}
@media(prefers-reduced-motion:reduce){.container-card{transition:none}}
</style>
