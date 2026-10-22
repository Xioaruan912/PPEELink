<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { UiButton, UiDialog, UiPanel, UiSkeleton } from "@/ui";
import { actOnContainer, containerLogs, containerStats, listContainers, type DockerAction, type DockerContainer, type DockerStats } from "@/api/monitor/operations";
const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading"); const items = ref<DockerContainer[]>([]); const stats = ref<DockerStats[]>([]);
const error = ref(""); const statsError = ref(""); const status = ref(""); const busy = ref(false); const statsBusy = ref(false); const filter = ref(""); const updated = ref("");
const locked = computed(() => busy.value || phase.value === "loading" || props.offline);
const shown = computed(() => items.value.filter(item => `${item.Names} ${item.Image} ${item.ID}`.toLowerCase().includes(filter.value.toLowerCase())));
const statFor = (container: DockerContainer) => stats.value.find(item => item.ID === container.ID || item.Container === container.ID || item.Name === container.Names);
let disposed = false; let generation = 0;
const message = (e: unknown) => e instanceof Error ? e.message : "操作失败，请检查 Docker 服务和 SSH 权限后重试。";
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
 if (props.offline || logsBusy.value) return; logsItem.value = item; logsOpen.value = true; logsBusy.value = true; logsError.value = ""; logs.value = "";
 try { const data = await containerLogs(props.id, item.ID); if (!disposed) { logs.value = data.content; logsTruncated.value = data.truncated; } }
 catch (e) { if (!disposed) logsError.value = message(e); } finally { logsBusy.value = false; }
}
onMounted(() => { load(); refreshStats(); }); onBeforeUnmount(() => { disposed = true; generation++; });
</script>

<template>
 <UiPanel title="Docker 容器" description="通过 SSH 执行固定 Docker 命令，不支持任意 Shell 输入">
  <template #actions><UiButton :disabled="locked" @click="load()">刷新列表</UiButton><UiButton :loading="statsBusy" :disabled="offline || busy" @click="refreshStats">刷新资源</UiButton></template>
  <p v-if="offline" class="notice" role="status">网络已断开。保留上次结果供查看；连接恢复后请刷新。</p>
  <p v-if="busy" class="muted" role="status">正在操作容器，请等待完成。停止/重启可能中断服务。</p>
  <div v-if="error" class="notice error" role="alert">{{ error }} <UiButton :disabled="locked" @click="load()">重试列表</UiButton></div>
  <div v-if="statsError" class="notice error" role="alert">资源数据不可用：{{ statsError }} <UiButton :disabled="offline || statsBusy" @click="refreshStats">重试资源</UiButton></div>
  <p v-if="status" class="notice" role="status">{{ status }}</p>
  <div class="toolbar"><label>筛选容器<input v-model="filter" placeholder="名称、镜像或 ID" /></label><span class="muted">{{ shown.length }} / {{ items.length }} 个容器<span v-if="updated"> · 列表更新于 {{ updated }}</span> · 资源按需刷新</span></div>
  <UiSkeleton v-if="phase === 'loading' && !items.length" :lines="5" aria-label="正在读取 Docker 容器" />
  <div v-else class="table-wrap" :aria-busy="phase === 'loading'"><table><caption class="sr-only">Docker 容器列表及资源使用情况</caption><thead><tr><th>容器 / 镜像</th><th>状态 / 端口</th><th>资源快照</th><th>操作</th></tr></thead><tbody>
   <tr v-for="item in shown" :key="item.ID"><td><strong>{{ item.Names || '未命名容器' }}</strong><small><code>{{ item.ID.slice(0, 12) }}</code></small><small>{{ item.Image }}</small></td><td><span class="state" :class="{ running: item.State === 'running' }">{{ item.State }}</span><small>{{ item.Status }}</small><small>{{ item.Ports || '未发布端口' }}</small></td><td><template v-if="statFor(item)"><span>CPU {{ statFor(item)!.CPUPerc }} · 内存 {{ statFor(item)!.MemPerc }}</span><small>{{ statFor(item)!.MemUsage }}</small><small>网络 {{ statFor(item)!.NetIO }} · I/O {{ statFor(item)!.BlockIO }}</small><small>进程 {{ statFor(item)!.PIDs }}</small></template><span v-else class="muted">{{ statsBusy ? '正在读取资源…' : item.State === 'running' ? '无资源快照，点击刷新资源' : '未运行，无实时资源数据' }}</span></td><td><div class="row-actions"><UiButton :disabled="locked || logsBusy" @click="showLogs(item)">日志</UiButton><UiButton v-if="item.State !== 'running'" :disabled="locked" @click="confirm(item, 'start')">启动</UiButton><UiButton v-else :disabled="locked" @click="confirm(item, 'stop')">停止</UiButton><UiButton :disabled="locked" @click="confirm(item, 'restart')">重启</UiButton></div></td></tr>
   <tr v-if="!shown.length && phase === 'ready'"><td colspan="4" class="empty">{{ items.length ? '没有匹配的容器。请清除或缩短筛选条件。' : '此服务器没有容器。先通过 SSH 部署容器，然后刷新此列表。' }}</td></tr>
  </tbody></table></div>
  <p class="muted">命令限时 45 秒，输出最多 2 MiB。SSH 用户须有 Docker 权限（不自动 sudo）。容器日志可能包含敏感信息，请谨慎分享。</p>
 </UiPanel>
 <UiDialog :model-value="!!pending" :title="pending ? '确认' + actionName(pending.action) + '容器' : '确认操作'" width="min(520px, 94vw)" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy" @update:model-value="!busy && (pending = undefined)">
  <template v-if="pending"><p class="wrap">{{ pending.item.Names }} · {{ pending.item.ID.slice(0, 12) }}</p><p>{{ pending.action === 'start' ? '启动后容器将使用服务器资源。是否继续？' : '这将中断容器内的服务及活跃连接。是否继续？' }}</p><p v-if="confirmError" class="notice error" role="alert">{{ confirmError }}</p></template>
  <template #footer><UiButton :disabled="busy" @click="pending = undefined">取消</UiButton><UiButton variant="danger" :loading="busy" :disabled="offline" @click="act">确认{{ pending ? actionName(pending.action) : '' }}</UiButton></template>
 </UiDialog>
 <UiDialog v-model="logsOpen" title="容器日志 · 最近 500 行" width="min(1000px, 96vw)">
  <div class="toolbar"><code>{{ logsItem?.Names }} · {{ logsItem?.ID.slice(0, 12) }}</code><UiButton :disabled="offline || logsBusy" @click="logsItem && showLogs(logsItem)">刷新日志</UiButton></div>
  <UiSkeleton v-if="logsBusy" :lines="5" /><p v-else-if="logsError" class="notice error" role="alert">{{ logsError }} 请检查日志权限后点击刷新。</p><pre v-else class="logs" tabindex="0" aria-label="容器只读日志">{{ logs || '暂无日志输出。容器可能尚未启动，或使用不支持读取的日志驱动。' }}</pre><p v-if="logsTruncated" class="notice" role="status">输出已截断至 2 MiB，可通过 SSH 查看完整日志。</p>
  <template #footer><UiButton @click="logsOpen = false">关闭</UiButton></template>
 </UiDialog>
</template>

<style scoped>
.toolbar,.row-actions{display:flex;gap:8px;align-items:center;flex-wrap:wrap}.toolbar{margin:14px 0}.toolbar label{display:flex;align-items:center;gap:8px}input{border:1px solid var(--c-line);background:var(--c-surface);color:var(--c-ink);border-radius:8px;padding:9px;max-width:100%;min-width:0}input:focus-visible,.logs:focus-visible{outline:2px solid var(--c-signal);outline-offset:2px}.table-wrap{overflow:auto}table{width:100%;border-collapse:collapse;text-align:left;font-size:13px}td,th{padding:12px 8px;border-bottom:1px solid var(--c-line);vertical-align:top}th{font-weight:500;color:var(--c-muted)}td:first-child{min-width:160px;max-width:300px;overflow-wrap:anywhere}td:nth-child(2){max-width:280px;overflow-wrap:anywhere}td:nth-child(3){min-width:170px}small{display:block;color:var(--c-muted);margin-top:5px;line-height:1.5}.row-actions{min-width:180px}.row-actions :deep(button){font-size:12px;padding:5px 9px;min-height:32px}.state{display:inline-block;border:1px solid var(--c-line);border-radius:20px;padding:3px 9px}.running{color:var(--el-color-success)}.muted{color:var(--c-muted);font-size:12px;line-height:1.6}.notice{padding:10px 12px;border:1px solid var(--c-line);border-radius:8px;background:var(--c-surface-2);overflow-wrap:anywhere;line-height:1.6}.error{color:var(--el-color-danger);border-color:var(--el-color-danger)}.empty{padding:32px;text-align:center;color:var(--c-muted)}.logs{background:var(--c-surface-2);color:var(--c-ink);border:1px solid var(--c-line);padding:14px;border-radius:8px;white-space:pre-wrap;overflow-wrap:anywhere;max-height:55vh;overflow:auto;font:12px/1.7 monospace}.wrap{overflow-wrap:anywhere}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}
</style>
