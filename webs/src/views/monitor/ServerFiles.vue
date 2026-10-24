<script setup lang="ts">
import IconFolder from "~icons/ph/folder";
import IconFile from "~icons/ph/file-text";
import IconLink from "~icons/ph/link";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { UiButton, UiDialog, UiSkeleton } from "@/ui";
import { downloadFile, listFiles, mutateFile, readFile, writeFile, type RemoteFile } from "@/api/monitor/operations";
const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading");
const path = ref("/"); const inputPath = ref("/"); const parent = ref("/");
const files = ref<RemoteFile[]>([]); const error = ref(""); const status = ref(""); const busy = ref(false); const truncated = ref(false);
const locked = computed(() => busy.value || phase.value === "loading" || props.offline);
const orderedFiles = computed(() => [...files.value].sort((a, b) => Number(b.kind === "directory") - Number(a.kind === "directory") || a.name.localeCompare(b.name, undefined, { numeric: true })));
const directoryCount = computed(() => files.value.filter(file => file.kind === "directory").length);
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
 <section class="file-console" aria-label="文件管理">
  <header class="console-heading"><div><span class="eyebrow">FILE EXPLORER</span><h2>文件管理</h2><p>浏览目录、传输文件，直接编辑文本。</p></div><div class="toolbar-actions"><UiButton :disabled="locked" @click="openMutation('mkdir')">新建目录</UiButton><UiButton variant="primary" :disabled="locked" @click="uploadInput?.click()">上传文件</UiButton><input ref="uploadInput" type="file" hidden @change="upload" /></div></header>
  <p v-if="offline" class="notice" role="status">网络已断开，当前展示上次目录。恢复连接后可刷新。</p>
  <p v-if="busy" class="notice" role="status">正在处理…</p>
  <div v-if="error" class="notice error" role="alert">{{ error }} <UiButton :disabled="locked" @click="load()">重试</UiButton></div>
  <p v-if="status" class="notice" role="status">{{ status }}</p>
  <div class="explorer">
   <aside class="directory-sidebar" aria-label="目录快捷入口"><span class="sidebar-label">位置</span><button :disabled="locked" :class="{ active: path === '/' }" @click="load('/')">根目录 <code>/</code></button><button :disabled="locked || path === '/'" @click="load(parent)">返回上一级</button><div class="sidebar-caption"><span>当前目录</span><strong>{{ directoryCount }} <small>个文件夹</small></strong><strong>{{ files.length - directoryCount }} <small>个文件</small></strong></div></aside>
   <div class="explorer-main">
    <form class="path-toolbar" @submit.prevent="load(inputPath)"><label class="sr-only" for="remote-path">远程绝对路径</label><input id="remote-path" v-model="inputPath" :disabled="locked" placeholder="输入绝对路径" spellcheck="false" /><UiButton type="submit" :disabled="locked">前往</UiButton><UiButton :disabled="locked" @click="load()">刷新</UiButton></form>
    <nav class="crumbs" aria-label="文件路径"><button :disabled="locked" @click="load('/')">根目录</button><template v-for="crumb in crumbs" :key="crumb.path"><span>/</span><button :disabled="locked" @click="load(crumb.path)">{{ crumb.name }}</button></template><span class="file-count">{{ files.length }} 项</span></nav>
    <UiSkeleton v-if="phase === 'loading' && !files.length" :lines="5" aria-label="正在读取目录" />
    <div v-else class="file-list" :aria-busy="phase === 'loading'">
     <div class="file-columns" aria-hidden="true"><span>名称</span><span>大小 / 修改时间</span><span>操作</span></div>
     <ul :aria-label="path + ' 的文件列表'">
      <li v-for="file in orderedFiles" :key="file.path" class="file-row">
       <div class="file-identity"><span class="file-icon" :class="{ folder: file.kind === 'directory' }" aria-hidden="true"><component :is="file.kind === 'directory' ? IconFolder : file.kind === 'symlink' ? IconLink : IconFile" /></span><div class="file-title"><button v-if="file.kind === 'directory'" class="file-name" :disabled="locked" @click="load(file.path)">{{ file.name }}</button><button v-else-if="file.kind === 'file' && file.size <= 1048576" class="file-name" :disabled="locked" @click="editFile(file)">{{ file.name }}</button><span v-else class="file-name">{{ file.name }}</span><small>{{ file.kind === 'directory' ? '文件夹' : file.kind === 'symlink' ? '链接 → ' + file.target : file.kind === 'special' ? '特殊文件 · 只读' : '文件' }} <code>{{ file.mode }}</code></small></div></div>
       <div class="file-metadata"><span>{{ file.kind === 'directory' ? '—' : bytes(file.size) }}</span><small>{{ new Date(file.modified * 1000).toLocaleString() }}</small></div>
       <div class="row-actions"><template v-if="file.kind === 'file'"><UiButton size="sm" :disabled="locked || file.size > 1048576" :aria-label="'编辑 ' + file.name" @click="editFile(file)">编辑</UiButton><UiButton size="sm" :disabled="locked || file.size > 67108864" :aria-label="'下载 ' + file.name" @click="perform(() => downloadFile(id, file.path), '下载已开始。', false)">下载</UiButton></template><template v-if="file.kind !== 'special'"><UiButton size="sm" :disabled="locked" :aria-label="'重命名 ' + file.name" @click="openMutation('rename', file)">重命名</UiButton><UiButton size="sm" variant="ghost" class="delete-action" :disabled="locked" :aria-label="'删除 ' + file.name" @click="openMutation('delete', file)">删除</UiButton></template></div>
      </li>
     </ul>
     <div v-if="!files.length && phase === 'ready'" class="empty"><strong>这个目录是空的</strong><p>上传文件，或新建一个文件夹。</p><UiButton :disabled="locked" @click="uploadInput?.click()">上传文件</UiButton></div>
    </div>
    <footer class="explorer-footer"><span>上传 / 下载 ≤ 64 MiB · 文本编辑 ≤ 1 MiB</span><details><summary>操作说明</summary><p>符号链接仅显示，不跟随编辑或下载。只能删除空目录，重命名不覆盖目标。保存和覆盖采用原子替换，可能改变所有者及扩展属性。</p></details></footer>
   </div>
  </div>
  <p v-if="truncated" class="notice" role="status">仅显示前 2000 项。请输入更具体的子目录路径。</p>
 </section>
 <UiDialog :model-value="!!mutation" :title="mutation === 'mkdir' ? '新建目录' : mutation === 'rename' ? '重命名' : '删除确认'" width="min(520px, 94vw)" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy" @update:model-value="!busy && (mutation = '')">
  <form id="file-mutation" @submit.prevent="submitMutation"><p v-if="selected" class="wrap">{{ selected.path }}</p><p v-if="mutation === 'delete'">确认删除？此操作不可撤销。目录必须为空，符号链接仅删除链接本身。</p><label v-else class="field">{{ mutation === 'rename' ? '新名称' : '目录名称' }}<input v-model="name" autofocus :disabled="busy" maxlength="255" /></label><p v-if="dialogError" class="notice error" role="alert">{{ dialogError }}</p></form>
  <template #footer><UiButton :disabled="busy" @click="mutation = ''">取消</UiButton><UiButton form="file-mutation" type="submit" :variant="mutation === 'delete' ? 'danger' : 'primary'" :disabled="offline" :loading="busy">确认{{ mutation === 'delete' ? '删除' : '' }}</UiButton></template>
 </UiDialog>
 <UiDialog v-model="editor" title="编辑文件" width="min(960px, 96vw)" :before-close="closeEditor" :close-on-click-modal="false" :show-close="!busy" :close-on-press-escape="!busy">
  <p class="wrap">{{ editorPath }} <strong v-if="dirty">· 未保存</strong></p><p class="muted">最多 1 MiB。保存使用同目录临时文件 + 原子重命名；不保留 ACL/扩展属性。请勿同时在其他工具修改此文件。</p>
  <UiSkeleton v-if="editorLoading" :lines="5" /><p v-if="editorError" class="notice error" role="alert">{{ editorError }}</p>
  <textarea v-if="!editorLoading" v-model="text" class="editor" aria-label="UTF-8 文件内容" :readonly="busy || offline || !editorLoaded" spellcheck="false" />
  <template #footer><UiButton :disabled="busy" @click="closeEditor()">关闭</UiButton><UiButton variant="primary" :loading="busy" :disabled="offline || editorLoading || !editorLoaded || !dirty" title="修改内容后可保存；离线时只读" @click="save">保存文件</UiButton></template>
 </UiDialog>
</template>

<style scoped>
.console-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap;margin-bottom:22px}.eyebrow{font:11px var(--font-mono);letter-spacing:.16em;color:var(--c-muted)}h2{font-size:24px;margin:6px 0;font-weight:600;letter-spacing:-.04em}.console-heading p{margin:0;color:var(--c-muted);font-size:13px}.toolbar-actions,.row-actions,.crumbs{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.explorer{display:grid;grid-template-columns:165px minmax(0,1fr);border:1px solid var(--c-line);border-radius:14px;background:var(--c-surface);overflow:hidden}.directory-sidebar{padding:20px 12px;background:var(--c-canvas);border-right:1px solid var(--c-line);display:flex;flex-direction:column;gap:6px}.sidebar-label{font-size:11px;color:var(--c-muted);margin:0 10px 10px}.directory-sidebar button{display:flex;justify-content:space-between;text-align:left;font-size:13px;padding:12px 10px;border:0;border-radius:7px;background:transparent;color:var(--c-ink);cursor:pointer}.directory-sidebar button.active{background:var(--c-surface);box-shadow:0 1px 3px #0000000a}.directory-sidebar button:hover{background:var(--c-surface-2)}.sidebar-caption{margin:22px 10px;display:grid;gap:12px;font-size:12px;color:var(--c-muted)}.sidebar-caption strong{font:500 20px var(--font-mono);color:var(--c-ink)}.sidebar-caption small{font:12px var(--font-ui);color:var(--c-muted)}.explorer-main{min-width:0}.path-toolbar{display:flex;gap:8px;align-items:center;padding:16px;border-bottom:1px solid var(--c-line)}.path-toolbar input{flex:1;width:0;font:12px var(--font-mono);background:var(--c-canvas)}input,textarea{border:1px solid var(--c-line);background:var(--c-surface);color:var(--c-ink);border-radius:7px;padding:10px;font:inherit;min-width:0}.crumbs{padding:10px 16px;gap:6px;font-size:12px;color:var(--c-muted);overflow-wrap:anywhere}.crumbs button{background:none;border:0;color:var(--c-muted);padding:6px;min-height:32px;font:inherit;cursor:pointer;overflow-wrap:anywhere}.crumbs button:last-of-type{color:var(--c-ink);font-weight:600}.file-count{margin-left:auto;white-space:nowrap}.file-columns,.file-row{display:grid;grid-template-columns:minmax(140px,1fr) 160px minmax(180px,auto);gap:16px;padding:14px 16px;align-items:center}.file-columns{font-size:11px;color:var(--c-muted);background:var(--c-canvas);border-block:1px solid var(--c-line)}.file-list ul{list-style:none;padding:0;margin:0}.file-row{border-bottom:1px solid var(--c-line);font-size:13px;transition:background .16s}.file-row:last-child{border-bottom:0}.file-row:hover{background:var(--c-canvas)}.file-identity{display:flex;gap:12px;min-width:0;align-items:center}.file-icon{display:grid;place-items:center;width:32px;height:36px;flex-shrink:0;font:24px var(--font-mono);color:var(--c-muted);background:var(--c-surface-2);border:1px solid var(--c-line);border-radius:7px}.file-icon svg{width:22px;height:22px}.file-icon.folder{font-size:30px;color:var(--c-signal-text)}.file-title{min-width:0}.file-name{display:block;background:none;border:0;color:var(--c-ink);font:500 13px var(--font-ui);padding:0;text-align:left;overflow-wrap:anywhere;line-height:1.5}button.file-name{cursor:pointer}button.file-name:hover{color:var(--c-signal-text)}small{display:block;color:var(--c-muted);font-size:11px;margin-top:5px;line-height:1.5;overflow-wrap:anywhere}.file-title code{margin-left:5px}.file-metadata{font:12px var(--font-mono);color:var(--c-muted)}.row-actions{gap:5px;justify-content:flex-end}.delete-action{color:var(--c-bad)}.explorer-footer{padding:14px 16px;border-top:1px solid var(--c-line);display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;font-size:11px;color:var(--c-muted)}details{max-width:460px}summary{cursor:pointer}details p{line-height:1.8}.muted{color:var(--c-muted);font-size:12px;line-height:1.6}.notice{padding:12px;background:var(--c-surface-2);border:1px solid var(--c-line);border-radius:8px;line-height:1.6;font-size:13px;overflow-wrap:anywhere}.error{border-color:var(--c-bad);color:var(--c-bad)}.empty{padding:48px 16px;text-align:center;color:var(--c-muted)}.empty strong{color:var(--c-ink)}.field{display:grid;gap:8px}.wrap{overflow-wrap:anywhere}.editor{box-sizing:border-box;width:100%;height:55vh;min-height:220px;font:13px/1.8 var(--font-mono);resize:vertical;tab-size:2;background:var(--c-canvas);padding:18px}.sr-only{position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0,0,0,0)}button:focus-visible,input:focus-visible,textarea:focus-visible,summary:focus-visible{outline:2px solid var(--c-focus);outline-offset:2px}button:disabled{cursor:not-allowed;opacity:.55}@media(max-width:1100px){.file-columns,.file-row{grid-template-columns:minmax(120px,1fr) 130px}.file-columns>span:last-child{display:none}.row-actions{grid-column:1/-1;justify-content:flex-start;padding-left:44px}}@media(max-width:700px){.explorer{grid-template-columns:minmax(0,1fr)}.directory-sidebar{padding:8px 12px;flex-direction:row;align-items:center;border-right:0;border-bottom:1px solid var(--c-line)}.sidebar-label,.sidebar-caption{display:none}.directory-sidebar button{min-height:44px;gap:16px}.path-toolbar{padding:12px;gap:6px}.path-toolbar :deep(button){padding:8px 10px;min-height:44px}.file-columns{display:none}.file-row{grid-template-columns:minmax(0,1fr);gap:10px;padding:16px}.file-metadata{padding-left:44px;display:flex;flex-wrap:wrap;gap:8px;align-items:center}.file-metadata small{margin:0}.row-actions{padding-left:0}.row-actions :deep(button){min-height:44px;flex:1;padding:7px 8px}.file-name{font-size:14px}.crumbs{padding-inline:10px}.console-heading h2{font-size:22px}}
@media(prefers-reduced-motion:reduce){.file-row{transition:none}}
</style>
