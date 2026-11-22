<script setup lang="ts">
import IconFolder from "~icons/ph/folder";
import IconFile from "~icons/ph/file-text";
import IconLink from "~icons/ph/link";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { ElMessageBox } from "element-plus";
import { UiButton, UiDialog, UiSkeleton } from "@/ui";
import {
  downloadFile,
  listFiles,
  mutateFile,
  readFile,
  writeFile,
  type RemoteFile,
} from "@/api/monitor/operations";

const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading");
const path = ref("/");
const inputPath = ref("/");
const parent = ref("/");
const files = ref<RemoteFile[]>([]);
const error = ref("");
const status = ref("");
const busy = ref(false);
const truncated = ref(false);

const locked = computed(
  () => busy.value || phase.value === "loading" || props.offline
);
const orderedFiles = computed(() =>
  [...files.value].sort(
    (a, b) =>
      Number(b.kind === "directory") - Number(a.kind === "directory") ||
      a.name.localeCompare(b.name, undefined, { numeric: true })
  )
);
const crumbs = computed(() =>
  path.value
    .split("/")
    .filter(Boolean)
    .map((name, i, all) => ({ name, path: "/" + all.slice(0, i + 1).join("/") }))
);

let request = 0;
let disposed = false;
const message = (e: unknown) =>
  e instanceof Error ? e.message : "操作失败，请检查连接后重试。";

async function load(target = path.value) {
  if (props.offline) return;
  const seq = ++request;
  phase.value = "loading";
  error.value = "";
  try {
    const data = await listFiles(props.id, target);
    if (disposed || seq !== request) return;
    files.value = data.items;
    path.value = inputPath.value = data.path;
    parent.value = data.parent;
    truncated.value = data.truncated;
    phase.value = "ready";
  } catch (e) {
    if (disposed || seq !== request) return;
    phase.value = "error";
    error.value = message(e);
  }
}

async function perform(
  action: () => Promise<unknown>,
  success: string,
  refresh = true
) {
  if (locked.value) return;
  busy.value = true;
  error.value = "";
  status.value = "";
  try {
    await action();
    if (disposed) return;
    status.value = success;
    if (refresh) await load();
  } catch (e) {
    if (!disposed) error.value = message(e);
  } finally {
    busy.value = false;
  }
}

const uploadInput = ref<HTMLInputElement>();
async function upload(event: Event) {
  const file = (event.target as HTMLInputElement).files?.[0];
  (event.target as HTMLInputElement).value = "";
  if (!file) return;
  if (file.size > 64 * 1024 * 1024) {
    error.value = "上传最多 64 MiB。";
    return;
  }
  const target = (path.value === "/" ? "" : path.value) + "/" + file.name;
  if (truncated.value || files.value.some((item) => item.path === target)) {
    try {
      await ElMessageBox.confirm(
        `将原子替换 ${file.name}。此操作不可撤销。`,
        "确认覆盖文件",
        { type: "warning", confirmButtonText: "覆盖", cancelButtonText: "取消" }
      );
    } catch {
      return;
    }
  }
  await perform(
    () => writeFile(props.id, target, file, true),
    `已上传 ${file.name}`
  );
}

const mutation = ref<"mkdir" | "rename" | "delete" | "">("");
const selected = ref<RemoteFile>();
const name = ref("");
const dialogError = ref("");

function openMutation(action: "mkdir" | "rename" | "delete", file?: RemoteFile) {
  mutation.value = action;
  selected.value = file;
  name.value = action === "rename" ? file?.name || "" : "";
  dialogError.value = "";
}

async function submitMutation() {
  if (locked.value) return;
  if (
    mutation.value !== "delete" &&
    (!name.value.trim() ||
      name.value === "." ||
      name.value === ".." ||
      /[\/\x00-\x1f\x7f]/.test(name.value))
  ) {
    dialogError.value = "请输入单个名称，不能包含 /、控制字符或 ..。";
    return;
  }
  const target = (path.value === "/" ? "" : path.value) + "/" + name.value;
  const action = mutation.value;
  if (!action) return;
  busy.value = true;
  dialogError.value = "";
  error.value = "";
  try {
    await mutateFile(
      props.id,
      action,
      action === "mkdir" ? target : selected.value!.path,
      action === "rename" ? target : undefined
    );
    mutation.value = "";
    status.value = "操作已完成。";
    await load();
  } catch (e) {
    dialogError.value = message(e);
  } finally {
    busy.value = false;
  }
}

const editor = ref(false);
const editorPath = ref("");
const text = ref("");
const original = ref("");
const editorLoading = ref(false);
const editorLoaded = ref(false);
const editorError = ref("");
let editorGeneration = 0;

const dirty = computed(() => editor.value && text.value !== original.value);

async function editFile(file: RemoteFile) {
  if (locked.value) return;
  const seq = ++editorGeneration;
  editor.value = true;
  editorPath.value = file.path;
  text.value = original.value = "";
  editorLoading.value = true;
  editorLoaded.value = false;
  editorError.value = "";
  try {
    const data = await readFile(props.id, file.path);
    if (!disposed && seq === editorGeneration) {
      text.value = original.value = data.content;
      editorLoaded.value = true;
    }
  } catch (e) {
    if (!disposed && seq === editorGeneration) editorError.value = message(e);
  } finally {
    if (seq === editorGeneration) editorLoading.value = false;
  }
}

async function confirmDiscard() {
  if (!dirty.value) return true;
  try {
    await ElMessageBox.confirm(
      "文本尚未保存，离开将丢失这些更改。",
      "放弃编辑？",
      {
        type: "warning",
        confirmButtonText: "放弃更改",
        cancelButtonText: "继续编辑",
      }
    );
    return true;
  } catch {
    return false;
  }
}

async function closeEditor(done?: () => void) {
  if (busy.value) return;
  if (await confirmDiscard()) {
    editorGeneration++;
    editor.value = false;
    done?.();
  }
}

async function save() {
  if (locked.value || editorLoading.value || !editorLoaded.value) return;
  if (new TextEncoder().encode(text.value).length > 1024 * 1024) {
    editorError.value = "编辑内容最多 1 MiB。";
    return;
  }
  busy.value = true;
  editorError.value = "";
  try {
    await writeFile(props.id, editorPath.value, text.value);
    original.value = text.value;
    status.value = "文件已保存。";
    editor.value = false;
    await load();
  } catch (e) {
    editorError.value = message(e);
  } finally {
    busy.value = false;
  }
}

function beforeUnload(event: BeforeUnloadEvent) {
  if (dirty.value) {
    event.preventDefault();
    event.returnValue = "";
  }
}

onMounted(() => {
  void load();
  window.addEventListener("beforeunload", beforeUnload);
});
onBeforeUnmount(() => {
  disposed = true;
  request++;
  window.removeEventListener("beforeunload", beforeUnload);
});
defineExpose({ confirmDiscard });

const bytes = (n: number) =>
  n < 1024
    ? `${n} B`
    : n < 1048576
      ? `${(n / 1024).toFixed(1)} KiB`
      : `${(n / 1048576).toFixed(1)} MiB`;
</script>

<template>
  <section class="files-gallery" aria-label="文件管理">
    <!-- Unified Single-Row Path & Action Bar -->
    <div class="files-toolbar">
      <form class="path-form" @submit.prevent="load(inputPath)">
        <button
          type="button"
          class="path-nav-btn"
          :disabled="locked || path === '/'"
          @click="load(parent)"
        >
          ↑ 上一级
        </button>
        <label class="sr-only" for="remote-path">远程绝对路径</label>
        <input
          id="remote-path"
          v-model="inputPath"
          class="path-input mono"
          :disabled="locked"
          placeholder="/etc"
          spellcheck="false"
        />
        <UiButton size="sm" variant="ghost" type="submit" :disabled="locked">
          前往
        </UiButton>
      </form>

      <div class="files-actions">
        <UiButton
          size="sm"
          variant="ghost"
          :disabled="locked"
          @click="openMutation('mkdir')"
        >
          新建目录
        </UiButton>
        <UiButton
          size="sm"
          variant="primary"
          :disabled="locked"
          @click="uploadInput?.click()"
        >
          上传文件
        </UiButton>
        <UiButton size="sm" variant="ghost" :disabled="locked" @click="load()">
          刷新
        </UiButton>
        <input ref="uploadInput" type="file" hidden @change="upload" />
      </div>
    </div>

    <nav class="crumbs-bar mono" aria-label="文件路径">
      <button type="button" :disabled="locked" @click="load('/')">/</button>
      <template v-for="crumb in crumbs" :key="crumb.path">
        <span aria-hidden="true">/</span>
        <button type="button" :disabled="locked" @click="load(crumb.path)">
          {{ crumb.name }}
        </button>
      </template>
      <span class="item-count">{{ files.length }} 项</span>
    </nav>

    <p v-if="offline" class="notice" role="status">
      网络已断开，恢复连接后可刷新。
    </p>
    <div v-if="error" class="notice error" role="alert">
      <span>{{ error }}</span>
      <UiButton size="sm" variant="ghost" :disabled="locked" @click="load()">
        重试
      </UiButton>
    </div>
    <p v-if="status" class="notice ok" role="status">{{ status }}</p>

    <UiSkeleton
      v-if="phase === 'loading' && !files.length"
      :lines="6"
      height="48px"
    />

    <div v-else class="file-ledger" :aria-busy="phase === 'loading'">
      <div class="file-columns" aria-hidden="true">
        <span>名称</span>
        <span>大小 / 修改时间</span>
        <span class="is-right">操作</span>
      </div>

      <ul v-if="orderedFiles.length" :aria-label="path + ' 的文件列表'">
        <li v-for="file in orderedFiles" :key="file.path" class="file-row">
          <div class="file-identity">
            <span
              class="file-icon"
              :class="{ folder: file.kind === 'directory' }"
              aria-hidden="true"
            >
              <component
                :is="
                  file.kind === 'directory'
                    ? IconFolder
                    : file.kind === 'symlink'
                      ? IconLink
                      : IconFile
                "
              />
            </span>
            <div class="file-title">
              <button
                v-if="file.kind === 'directory'"
                type="button"
                class="file-name"
                :disabled="locked"
                @click="load(file.path)"
              >
                {{ file.name }}
              </button>
              <button
                v-else-if="file.kind === 'file' && file.size <= 1048576"
                type="button"
                class="file-name"
                :disabled="locked"
                @click="editFile(file)"
              >
                {{ file.name }}
              </button>
              <span v-else class="file-name">{{ file.name }}</span>
              <small class="mono">
                {{
                  file.kind === "directory"
                    ? "目录"
                    : file.kind === "symlink"
                      ? `→ ${file.target}`
                      : file.kind === "special"
                        ? "只读"
                        : "文件"
                }}
                · {{ file.mode }}
              </small>
            </div>
          </div>

          <div class="file-meta mono">
            <span>{{ file.kind === "directory" ? "—" : bytes(file.size) }}</span>
            <small>{{ new Date(file.modified * 1000).toLocaleString() }}</small>
          </div>

          <div class="row-actions">
            <template v-if="file.kind === 'file'">
              <UiButton
                size="sm"
                variant="ghost"
                :disabled="locked || file.size > 1048576"
                :aria-label="'编辑 ' + file.name"
                @click="editFile(file)"
              >
                编辑
              </UiButton>
              <UiButton
                size="sm"
                variant="ghost"
                :disabled="locked || file.size > 67108864"
                :aria-label="'下载 ' + file.name"
                @click="
                  perform(
                    () => downloadFile(id, file.path),
                    '下载已开始。',
                    false
                  )
                "
              >
                下载
              </UiButton>
            </template>
            <template v-if="file.kind !== 'special'">
              <UiButton
                size="sm"
                variant="ghost"
                :disabled="locked"
                :aria-label="'重命名 ' + file.name"
                @click="openMutation('rename', file)"
              >
                重命名
              </UiButton>
              <UiButton
                size="sm"
                variant="ghost"
                :disabled="locked"
                :aria-label="'删除 ' + file.name"
                @click="openMutation('delete', file)"
              >
                <span class="danger">删除</span>
              </UiButton>
            </template>
          </div>
        </li>
      </ul>

      <div v-else-if="phase === 'ready'" class="empty-dir">
        <strong>空目录</strong>
        <p>上传文件或新建子目录。</p>
      </div>
    </div>

    <p v-if="truncated" class="notice" role="status">
      当前目录超过 2000 项，仅显示前 2000 项。
    </p>
  </section>

  <!-- Mutation Dialog -->
  <UiDialog
    :model-value="!!mutation"
    :title="
      mutation === 'mkdir'
        ? '新建目录'
        : mutation === 'rename'
          ? '重命名'
          : '确认删除'
    "
    width="min(480px, 94vw)"
    :close-on-click-modal="false"
    :show-close="!busy"
    :close-on-press-escape="!busy"
    @update:model-value="!busy && (mutation = '')"
  >
    <form id="file-mutation" class="mutation-form" @submit.prevent="submitMutation">
      <p v-if="selected" class="mono path-preview">{{ selected.path }}</p>
      <p v-if="mutation === 'delete'" class="muted">
        此操作不可撤销；仅可删除空目录，符号链接只删除链接本身。
      </p>
      <label v-else class="field">
        <span>{{ mutation === "rename" ? "新名称" : "目录名称" }}</span>
        <input v-model="name" autofocus :disabled="busy" maxlength="255" />
      </label>
      <p v-if="dialogError" class="notice error" role="alert">
        {{ dialogError }}
      </p>
    </form>
    <template #footer>
      <UiButton variant="ghost" :disabled="busy" @click="mutation = ''">
        取消
      </UiButton>
      <UiButton
        form="file-mutation"
        type="submit"
        :variant="mutation === 'delete' ? 'danger' : 'primary'"
        :disabled="offline"
        :loading="busy"
      >
        确认{{ mutation === "delete" ? "删除" : "" }}
      </UiButton>
    </template>
  </UiDialog>

  <!-- Obsidian Code Editor Dialog -->
  <UiDialog
    v-model="editor"
    title="编辑文件"
    width="min(960px, 96vw)"
    :before-close="closeEditor"
    :close-on-click-modal="false"
    :show-close="!busy"
    :close-on-press-escape="!busy"
  >
    <p class="mono path-preview">
      {{ editorPath }} <strong v-if="dirty">· 未保存</strong>
    </p>
    <UiSkeleton v-if="editorLoading" :lines="6" />
    <p v-if="editorError" class="notice error" role="alert">
      {{ editorError }}
    </p>
    <textarea
      v-if="!editorLoading"
      v-model="text"
      class="code-editor mono"
      aria-label="UTF-8 文件内容"
      :readonly="busy || offline || !editorLoaded"
      spellcheck="false"
    />
    <template #footer>
      <UiButton variant="ghost" :disabled="busy" @click="closeEditor()">
        关闭
      </UiButton>
      <UiButton
        variant="primary"
        :loading="busy"
        :disabled="offline || editorLoading || !editorLoaded || !dirty"
        @click="save"
      >
        保存文件
      </UiButton>
    </template>
  </UiDialog>
</template>

<style scoped>
.files-gallery {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.files-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.path-form {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1 1 320px;
  min-width: 0;
}

.path-nav-btn {
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
  white-space: nowrap;
}

.path-nav-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.path-input {
  flex: 1;
  min-width: 0;
  padding: 7px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
}

.path-input:focus {
  outline: none;
  border-color: var(--c-ink);
}

.files-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.crumbs-bar {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--c-muted);
}

.crumbs-bar button {
  padding: 3px 6px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
  font: inherit;
}

.crumbs-bar button:hover {
  background: var(--c-surface-2);
  color: var(--c-ink);
}

.crumbs-bar button:last-of-type {
  color: var(--c-ink);
  font-weight: 500;
}

.item-count {
  margin-left: auto;
}

.notice {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin: 0;
  padding: 10px 14px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
  font-size: 13px;
  color: var(--c-muted);
}

.notice.error {
  border-color: rgb(223 41 41 / 0.28);
  color: var(--c-bad);
}

.notice.ok {
  border-color: rgb(21 184 90 / 0.28);
  color: var(--c-ok);
}

.file-ledger {
  display: grid;
}

.file-columns {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) 180px auto;
  gap: 16px;
  padding: 10px 4px;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
  font-size: 11.5px;
  color: var(--c-muted);
}

.file-columns .is-right {
  text-align: right;
}

.file-ledger ul {
  list-style: none;
  margin: 0;
  padding: 0;
}

.file-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) 180px auto;
  align-items: center;
  gap: 16px;
  padding: 14px 10px;
  margin: 0 -10px;
  border-bottom: 1px solid var(--c-line);
  transition: opacity 240ms var(--ease-out);
}

.file-row::before {
  content: "";
  position: absolute;
  inset: 0;
  border-radius: 10px;
  background: rgb(10 10 10 / 0.028);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 340ms var(--ease-out);
  pointer-events: none;
}

.file-row::after {
  content: "";
  position: absolute;
  left: 10px;
  right: 10px;
  bottom: -1px;
  height: 1px;
  background: var(--c-ink);
  transform: scaleX(0);
  transform-origin: left;
  transition: transform 320ms var(--ease-out);
  pointer-events: none;
}

.file-row > * {
  position: relative;
  z-index: 1;
}

@media (hover: hover) and (pointer: fine) {
  .file-ledger ul:hover .file-row:not(:hover) {
    opacity: 0.48;
  }
  .file-row:hover::before,
  .file-row:hover::after {
    transform: scaleX(1);
  }
}

.file-identity {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.file-icon {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 9px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  color: var(--c-muted);
  flex: none;
}

.file-icon.folder {
  color: var(--c-ink);
  background: var(--c-surface-2);
}

.file-icon svg {
  width: 17px;
  height: 17px;
}

.file-title {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.file-name {
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--c-ink);
  font-size: 14px;
  font-weight: 500;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

button.file-name {
  cursor: pointer;
}

.file-title small {
  font-size: 11px;
  color: var(--c-muted);
}

.file-meta {
  display: grid;
  gap: 2px;
  font-size: 12px;
  color: var(--c-ink);
}

.file-meta small {
  font-size: 11px;
  color: var(--c-muted);
}

.row-actions {
  display: inline-flex;
  align-items: center;
  justify-content: flex-end;
  gap: 4px;
}

.danger {
  color: var(--c-bad);
}

.empty-dir {
  padding: 48px 0;
  text-align: center;
  color: var(--c-muted);
}

.empty-dir strong {
  display: block;
  margin-bottom: 4px;
  color: var(--c-ink);
  font-size: 15px;
}

.mutation-form {
  display: grid;
  gap: 12px;
}

.path-preview {
  margin: 0 0 10px;
  font-size: 12px;
  color: var(--c-muted);
  word-break: break-all;
}

.field {
  display: grid;
  gap: 6px;
  font-size: 13px;
}

.field input {
  padding: 10px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface);
  color: var(--c-ink);
  font: inherit;
}

.code-editor {
  width: 100%;
  height: 56vh;
  min-height: 260px;
  padding: 16px 18px;
  border: 1px solid rgb(255 255 255 / 0.12);
  border-radius: 14px;
  background: #0b0b0e;
  color: #f0f0ed;
  font-size: 13px;
  line-height: 1.7;
  resize: vertical;
  tab-size: 2;
}

.muted {
  margin: 0;
  font-size: 13px;
  color: var(--c-muted);
}

.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
}

@media (max-width: 820px) {
  .file-columns {
    display: none;
  }
  .file-row {
    grid-template-columns: 1fr;
    gap: 10px;
    padding: 14px 0;
    margin: 0;
  }
  .file-row::after {
    left: 0;
    right: 0;
  }
  .row-actions {
    justify-content: flex-start;
  }
}
</style>
