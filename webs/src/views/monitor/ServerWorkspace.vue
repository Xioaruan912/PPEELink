<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { onBeforeRouteLeave, onBeforeRouteUpdate, useRoute, useRouter } from "vue-router";
import { PageHeader, UiButton, UiPanel, UiSkeleton } from "@/ui";
import { getMonitors, type MonitorItem } from "@/api/monitor";
import ServerFiles from "./ServerFiles.vue";
import ServerDocker from "./ServerDocker.vue";

defineOptions({ name: "MonitorWorkspace" });
// State inventory: loading / first-use empty / invalid selection / missing SSH /
// ready / partial child errors / offline retained data / busy-disabled / read-only links.
const route = useRoute(); const router = useRouter();
const phase = ref<"loading" | "ready" | "error">("loading"); const error = ref(""); const items = ref<MonitorItem[]>([]);
const offline = ref(!navigator.onLine); const tab = computed(() => route.query.tab === "docker" ? "docker" : "files");
const id = computed(() => { const raw = route.query.id; return typeof raw === "string" && /^[1-9]\d*$/.test(raw) ? Number(raw) : 0; });
const selected = computed(() => items.value.find(item => item.id === id.value));
const visitedFiles = ref(tab.value === "files"); const visitedDocker = ref(tab.value === "docker");
const files = ref<InstanceType<typeof ServerFiles>>();
watch(tab, value => { if (value === "files") visitedFiles.value = true; else visitedDocker.value = true; });
async function load() {
 phase.value = "loading"; error.value = "";
 try { const { data } = await getMonitors(); items.value = data?.items || []; phase.value = "ready"; if (!route.query.id && items.value.length) await router.replace({ query: { ...route.query, id: String(items.value[0].id), tab: tab.value } }); }
 catch (e: any) { phase.value = "error"; error.value = e?.message || String(e || "无法读取服务器，请检查网络并重试。"); }
}
function chooseServer(event: Event) { router.push({ query: { ...route.query, id: (event.target as HTMLSelectElement).value, tab: tab.value } }); }
function chooseTab(value: "files" | "docker") { router.replace({ query: { ...route.query, tab: value } }); }
async function canLeave() { return await files.value?.confirmDiscard() ?? true; }
onBeforeRouteLeave(canLeave);
onBeforeRouteUpdate(async to => { if (to.query.id !== route.query.id) return canLeave(); return true; });
function connectionChanged() { offline.value = !navigator.onLine; }
onMounted(() => { load(); window.addEventListener("online", connectionChanged); window.addEventListener("offline", connectionChanged); });
onBeforeUnmount(() => { window.removeEventListener("online", connectionChanged); window.removeEventListener("offline", connectionChanged); });
</script>

<template>
 <main class="workspace">
  <PageHeader title="服务器工作区" description="复用 Monitor SSH 配置，管理远程文件和 Docker 容器。"><template #actions><RouterLink to="/monitor/index" class="back-link">返回服务器监控 / SSH 设置</RouterLink></template></PageHeader>
  <UiPanel title="连接服务器" description="仅管理员可操作。文件和容器操作使用 SSH 权限，不依赖探针在线状态。">
   <UiSkeleton v-if="phase === 'loading'" :lines="2" aria-label="正在加载服务器" />
   <div v-else-if="phase === 'error'" role="alert" class="state"><p>{{ error }}</p><UiButton :disabled="offline" @click="load">重试加载</UiButton></div>
   <div v-else-if="!items.length" class="state"><h3>还没有服务器</h3><p>先在服务器监控中添加服务器并保存 SSH 配置，即可使用文件和 Docker 管理。</p><RouterLink to="/monitor/index">添加服务器并配置 SSH</RouterLink></div>
   <template v-else><div class="server-line"><label for="workspace-server">服务器</label><select id="workspace-server" :value="id || ''" @change="chooseServer"><option disabled value="">选择服务器</option><option v-for="item in items" :key="item.id" :value="item.id">{{ item.name }}{{ item.sshConfigured ? '' : ' · 未配置 SSH' }}</option></select><span v-if="selected" class="connection">{{ selected.sshUser || 'SSH' }}@{{ selected.sshHost || '未设置' }}:{{ selected.sshPort || 22 }} · {{ selected.online ? '探针在线' : '探针离线（SSH 可独立连接）' }}</span><UiButton :disabled="offline" @click="load">刷新服务器</UiButton></div>
    <p v-if="!selected" class="state" role="status">未找到所选服务器。请选择上方服务器，或返回监控页面确认服务器仍存在。</p>
    <div v-else-if="!selected.sshConfigured" class="state" role="status"><h3>需要先配置 SSH</h3><p>在服务器监控中打开此服务器的 SSH 设置，保存主机、用户和密码后返回刷新。</p><RouterLink to="/monitor/index">前往 SSH 设置</RouterLink></div>
   </template>
  </UiPanel>
  <template v-if="selected?.sshConfigured">
   <nav class="tabs" aria-label="工作区功能"><UiButton :variant="tab === 'files' ? 'primary' : 'secondary'" :aria-current="tab === 'files' ? 'page' : undefined" @click="chooseTab('files')">SFTP 文件管理</UiButton><UiButton :variant="tab === 'docker' ? 'primary' : 'secondary'" :aria-current="tab === 'docker' ? 'page' : undefined" @click="chooseTab('docker')">Docker 管理</UiButton></nav>
   <ServerFiles v-if="visitedFiles" v-show="tab === 'files'" :key="`files-${id}`" ref="files" :id="id" :offline="offline" />
   <ServerDocker v-if="visitedDocker" v-show="tab === 'docker'" :key="`docker-${id}`" :id="id" :offline="offline" />
  </template>
 </main>
</template>

<style scoped>
.workspace{display:grid;gap:18px}.workspace :deep(.page-header){margin-bottom:0}.server-line{display:flex;align-items:center;gap:12px;flex-wrap:wrap}.server-line label{font-weight:600;font-size:13px}select{padding:9px 12px;border:1px solid var(--c-line);border-radius:8px;background:var(--c-surface);color:var(--c-ink);font:inherit;min-width:200px;max-width:100%}select:focus-visible{outline:2px solid var(--c-signal);outline-offset:2px}.connection{font-size:12px;color:var(--c-muted);overflow-wrap:anywhere;flex:1}.state{padding:18px 0;color:var(--c-muted);line-height:1.7}.state h3{margin:0;color:var(--c-ink);font-size:15px}.state p{margin:8px 0}.tabs{display:flex;gap:8px;flex-wrap:wrap}.back-link{font-size:13px;color:var(--c-ink)}@media(max-width:600px){.server-line select{width:100%}.server-line .connection{flex-basis:100%}.tabs :deep(button){flex:1}.workspace{gap:12px}}
</style>
