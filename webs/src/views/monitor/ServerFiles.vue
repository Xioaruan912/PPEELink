<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { UiButton, UiDialog, UiPanel, UiSkeleton } from "@/ui";
import { downloadFile, listFiles, mutateFile, readFile, writeFile, type RemoteFile } from "@/api/monitor/operations";
const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading");
const path = ref("/"); const inputPath = ref("/"); const parent = ref("/");
const files = ref<RemoteFile[]>([]); const error = ref(""); const status = ref(""); const busy = ref(false); const truncated = ref(false);
const locked = computed(() => busy.value || phase.value === "loading" || props.offline);
const crumbs = computed(() => path.value.split("/").filter(Boolean).map((name, i, all) => ({ name, path: "/" + all.slice(0, i + 1).join("/") })));
let request = 0; let disposed = false;
const message = (e: unknown) => e instanceof Error ? e.message : "操作失败，请检查连接后重试。";
async function load(target = path.value) {
  if (props.offline) return;
  const seq = ++request; phase.value = "loading"; error.value = "";
  try { const data = await listFiles(props.id, target); if (disposed || seq !== request) return; files.value = data.items; path.value = inputPath.value = data.path; parent.value = data.parent; truncated.value = data.truncated; phase.value = "ready"; }
  catch (e) { if (disposed || seq !== request) return; phase.value = "error"; error.value = message(e); }
}
async function perform(action: () => Promise<unknown>, success: string, refresh = true) {
  if (locked.value) return; busy.value = true; error.value = ""; status.value = "";
  try { await action(); if (disposed) return; status.value = success; if (refresh) await load(); }
  catch (e) { if (!disposed) error.value = message(e); }
  finally { busy.value = false; }
}
const uploadInput = ref<HTMLInputElement>();
async function upload(event: Event) {
 const file = (event.target as HTMLInputElement).files?.[0]; (event.target as HTMLInputElement).value = ""; if (!file) return;
 if (file.size > 64 * 1024 * 1024) { error.value = "上传最多 64 MiB。"; return; }
 const target = (path.value === "/" ? "" : path.value) + "/" + file.name;
 if (truncated.value || files.value.some(item => item.path === target)) {
   try { await ElMessageBox.confirm(`将原子替换 ${file.name}。此操作不可撤销。`, "确认覆盖文件", { type: "warning", confirmButtonText: "覆盖", cancelButtonText: "取消" }); } catch { return; }
 }
 await perform(() => writeFile(props.id, target, file, true), `已上传 ${file.name}`);
}
const mutation = ref<"mkdir" | "rename" | "delete" | "">(""); const selected = ref<RemoteFile>(); const name = ref(""); const dialogError = ref("");
function openMutation(action: "mkdir" | "rename" | "delete", file?: RemoteFile) { mutation.value = action; selected.value = file; name.value = action === "rename" ? file?.name || "" : ""; dialogError.value = ""; }
async function submitMutation() {
 if (locked.value) return;
 if (mutation.value !== "delete" && (!name.value.trim() || name.value === "." || name.value === ".." || /[\/\x00-\x1f\x7f]/.test(name.value))) { dialogError.value = "请输入单个名称，不能包含 /、控制字符或 ..。"; return; }
 const target = (path.value === "/" ? "" : path.value) + "/" + name.value;
 const action = mutation.value; if (!action) return;
 busy.value = true; dialogError.value = ""; error.value = "";
 try { await mutateFile(props.id, action, action === "mkdir" ? target : selected.value!.path, action === "rename" ? target : undefined); mutation.value = ""; status.value = "操作已完成。"; await load(); }
 catch (e) { dialogError.value = message(e); } finally { busy.value = false; }
}
const editor = ref(false); const editorPath = ref(""); const text = ref(""); const original = ref(""); const editorLoading = ref(false); const editorLoaded = ref(false); const editorError = ref("");
let editorGeneration = 0;
const dirty = computed(() => editor.value && text.value !== original.value);
async function editFile(file: RemoteFile) {
 if (locked.value) return; const seq = ++editorGeneration; editor.value = true; editorPath.value = file.path; text.value = original.value = ""; editorLoading.value = true; editorLoaded.value = false; editorError.value = "";
 try { const data = await readFile(props.id, file.path); if (!disposed && seq === editorGeneration) { text.value = original.value = data.content; editorLoaded.value = true; } }
 catch (e) { if (!disposed && seq === editorGeneration) editorError.value = message(e); } finally { if (seq === editorGeneration) editorLoading.value = false; }
}
async function confirmDiscard() { if (!dirty.value) return true; try { await ElMessageBox.confirm("文本尚未保存，离开将丢失这些更改。", "放弃编辑？", { type: "warning", confirmButtonText: "放弃更改", cancelButtonText: "继续编辑" }); return true; } catch { return false; } }
async function closeEditor(done?: () => void) { if (busy.value) return; if (await confirmDiscard()) { editorGeneration++; editor.value = false; done?.(); } }
async function save() {
 if (locked.value || editorLoading.value || !editorLoaded.value) return;
 if (new TextEncoder().encode(text.value).length > 1024 * 1024) { editorError.value = "编辑内容最多 1 MiB。"; return; }
 busy.value = true; editorError.value = "";
 try { await writeFile(props.id, editorPath.value, text.value); original.value = text.value; status.value = "文件已原子保存。"; editor.value = false; await load(); }
 catch (e) { editorError.value = message(e); } finally { busy.value = false; }
}
function beforeUnload(event: BeforeUnloadEvent) { if (dirty.value) { event.preventDefault(); event.returnValue = ""; } }
onMounted(() => { load(); window.addEventListener("beforeunload", beforeUnload); });
onBeforeUnmount(() => { disposed = true; request++; window.removeEventListener("beforeunload", beforeUnload); });
defineExpose({ confirmDiscard });
const bytes = (n: number) => n < 1024 ? `${n} B` : n < 1048576 ? `${(n / 1024).toFixed(1)} KiB` : `${(n / 1048576).toFixed(1)} MiB`;
</script>

<template>
 <UiPanel title="文件管理" description="通过 agent 本地读写 · 上传/下载 64 MiB · UTF-8 编辑 1 MiB">
  <template #actions><UiButton :disabled="locked" @click="load()">刷新</UiButton></template>
  <p v-if="offline" class="notice" role="status">网络已断开。保留上次目录供查看，恢复网络后请刷新。</p>
  <p v-if="busy" class="muted" role="status">正在处理，请等待完成后再进行其他操作。</p>
  <div v-if="error" class="notice error" role="alert">{{ error }} <UiButton :disabled="locked" @click="load()">重试</UiButton></div>
  <p v-if="status" class="notice" role="status">{{ status }}</p>
  <nav class="crumbs" aria-label="文件路径"><button :disabled="locked" @click="load('/')">根目录 /</button><template v-for="crumb in crumbs" :key="crumb.path"><span>/</span><button :disabled="locked" @click="load(crumb.path)">{{ crumb.name }}</button></template></nav>
  <form class="toolbar" @submit.prevent="load(inputPath)"><label class="path-label">绝对路径<input v-model="inputPath" :disabled="locked" placeholder="/" aria-label="远程绝对路径" /></label><UiButton type="submit" :disabled="locked">前往</UiButton><UiButton :disabled="locked || path === '/'" title="根目录没有上一级" @click="load(parent)">上一级</UiButton><UiButton :disabled="locked" @click="openMutation('mkdir')">新建目录</UiButton><UiButton :disabled="locked" @click="uploadInput?.click()">上传文件</UiButton><input ref="uploadInput" type="file" hidden @change="upload" /></form>
  <p class="muted">符号链接仅显示，不跟随编辑或下载。删除目录仅限空目录；重命名不覆盖目标。编辑和覆盖会原子替换文件，可能改变所有者或扩展属性。</p>
  <UiSkeleton v-if="phase === 'loading' && !files.length" :lines="5" aria-label="正在读取目录" />
  <div v-else class="table-wrap" :aria-busy="phase === 'loading'">
   <table><caption class="sr-only">{{ path }} 的文件列表</caption><thead><tr><th>名称 / 类型</th><th>大小</th><th>权限 / 修改时间</th><th>操作</th></tr></thead><tbody>
    <tr v-for="file in files" :key="file.path"><td><button v-if="file.kind === 'directory'" class="file-name" :disabled="locked" @click="load(file.path)">{{ file.name }}/</button><span v-else class="file-name">{{ file.name }}</span><small>{{ file.kind === 'directory' ? '目录' : file.kind === 'symlink' ? '符号链接 → ' + file.target : file.kind === 'special' ? '特殊文件（只读）' : '文件' }}</small></td><td>{{ bytes(file.size) }}</td><td><code>{{ file.mode }}</code><small>{{ new Date(file.modified * 1000).toLocaleString() }}</small></td><td><div class="row-actions"><template v-if="file.kind === 'file'"><UiButton :disabled="locked || file.size > 1048576" title="仅支持不超过 1 MiB 的 UTF-8 文本" @click="editFile(file)">编辑</UiButton><UiButton :disabled="locked || file.size > 67108864" title="下载最多 64 MiB" @click="perform(() => downloadFile(id, file.path), '下载已开始。', false)">下载</UiButton></template><template v-if="file.kind !== 'special'"><UiButton :disabled="locked" @click="openMutation('rename', file)">重命名</UiButton><UiButton variant="danger" :disabled="locked" @click="openMutation('delete', file)">删除</UiButton></template></div></td></tr>
    <tr v-if="!files.length && phase === 'ready'"><td colspan="4" class="empty">此目录为空。上传文件或新建目录开始管理。</td></tr>
   </tbody></table>
  </div>
  <p v-if="truncated" class="notice" role="status">仅显示前 2000 项。请输入更具体的子目录路径。</p>
 </UiPanel>
 <UiDialog :model-value="!!mutation" :title="mutation === 'mkdir' ? '新建目录' : mutation === 'rename' ? '重命名' : '删除确认'" width="min(520px, 94vw)" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy" @update:model-value="!busy && (mutation = '')">
  <form id="file-mutation" @submit.prevent="submitMutation"><p v-if="selected" class="wrap">{{ selected.path }}</p><p v-if="mutation === 'delete'">确认删除？此操作不可撤销。目录必须为空，符号链接仅删除链接本身。</p><label v-else class="field">{{ mutation === 'rename' ? '新名称' : '目录名称' }}<input v-model="name" autofocus :disabled="busy" maxlength="255" /></label><p v-if="dialogError" class="notice error" role="alert">{{ dialogError }}</p></form>
  <template #footer><UiButton :disabled="busy" @click="mutation = ''">取消</UiButton><UiButton form="file-mutation" type="submit" :variant="mutation === 'delete' ? 'danger' : 'primary'" :disabled="offline" :loading="busy">确认{{ mutation === 'delete' ? '删除' : '' }}</UiButton></template>
 </UiDialog>
 <UiDialog v-model="editor" title="UTF-8 文本编辑器" width="min(960px, 96vw)" :before-close="closeEditor" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy">
  <p class="wrap">{{ editorPath }} <strong v-if="dirty">· 未保存</strong></p><p class="muted">最多 1 MiB。保存使用同目录临时文件 + 原子重命名；不保留 ACL/扩展属性。请勿同时在其他工具修改此文件。</p>
  <UiSkeleton v-if="editorLoading" :lines="5" /><p v-if="editorError" class="notice error" role="alert">{{ editorError }}</p>
  <textarea v-if="!editorLoading" v-model="text" class="editor" aria-label="UTF-8 文件内容" :readonly="busy || offline || !editorLoaded" spellcheck="false" />
  <template #footer><UiButton :disabled="busy" @click="closeEditor()">关闭</UiButton><UiButton variant="primary" :loading="busy" :disabled="offline || editorLoading || !editorLoaded || !dirty" title="修改内容后可保存；离线时只读" @click="save">原子保存</UiButton></template>
 </UiDialog>
</template>

<style scoped>
.toolbar,.row-actions,.crumbs{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.toolbar{margin:14px 0}.path-label{display:flex;align-items:center;gap:8px;flex:1;min-width:200px}.path-label input{width:100%}input,textarea{border:1px solid var(--c-line);background:var(--c-surface);color:var(--c-ink);border-radius:8px;padding:9px;font:inherit;min-width:0}.crumbs{overflow-wrap:anywhere}.crumbs button,.file-name{background:none;border:0;color:var(--c-ink);font:inherit;text-align:left;overflow-wrap:anywhere}.crumbs button,button.file-name{text-decoration:underline;cursor:pointer;min-height:32px}.table-wrap{overflow:auto}table{width:100%;border-collapse:collapse;text-align:left;font-size:13px}th,td{padding:12px 8px;border-bottom:1px solid var(--c-line);vertical-align:top}th{color:var(--c-muted);font-weight:500}td:first-child{min-width:150px;max-width:320px}small{display:block;color:var(--c-muted);margin-top:5px;overflow-wrap:anywhere}.row-actions{min-width:250px}.row-actions :deep(button){font-size:12px;padding:5px 9px;min-height:32px}.muted{color:var(--c-muted);font-size:12px;line-height:1.6}.notice{padding:10px 12px;background:var(--c-surface-2);border:1px solid var(--c-line);border-radius:8px;line-height:1.6;overflow-wrap:anywhere}.error{border-color:var(--el-color-danger);color:var(--el-color-danger)}.empty{padding:32px;text-align:center;color:var(--c-muted)}.field{display:grid;gap:8px}.wrap{overflow-wrap:anywhere}.editor{box-sizing:border-box;width:100%;height:48vh;min-height:220px;font:13px/1.6 monospace;resize:vertical;tab-size:2}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}button:focus-visible,input:focus-visible,textarea:focus-visible{outline:2px solid var(--c-signal);outline-offset:2px}button:disabled{cursor:not-allowed;opacity:.55}@media(max-width:600px){.path-label{flex-basis:100%}.toolbar :deep(button){flex:1}.row-actions{min-width:180px}}
</style>
