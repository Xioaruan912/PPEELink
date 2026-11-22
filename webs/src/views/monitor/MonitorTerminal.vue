<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { UiButton, UiDialog, UiTag } from "@/ui";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import {
  getMonitorSSHTicket,
  getTerminalSessions,
  createTerminalSession,
  endTerminalSession,
  prepareTerminal,
  type TerminalSession,
} from "@/api/monitor";
import {
  createTerminalConnection,
  type TerminalEvent,
} from "./terminalConnection";
import { quickCommands, type QuickCommand } from "./commands";

const props = defineProps<{
  modelValue: boolean;
  monitor: {
    id: number;
    name: string;
    proxyServerId?: number | null;
  } | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const terminalHost = ref<HTMLDivElement>();
const status = ref("未连接");
const connected = ref(false);
let term: Terminal | undefined;
let fit: FitAddon | undefined;
const sessions = ref<TerminalSession[]>([]);
const selectedSession = ref("");
const sessionReady = ref(false),
  sessionSupported = ref(false),
  preparing = ref(false);
const sessionLoading = ref(false);
let loadGeneration = 0;
const commandTab = ref<"builtin" | "nodescripts">("builtin");
const selectedCategory = ref("全部");
const commandQuery = ref("");
const commandDraft = ref("");
const draftTitle = ref("");
const selectedCommand = ref<QuickCommand>();
const selectedVariant = ref(0);
const sessionKey = () => `ppeelink-terminal:${props.monitor?.id}`;
const connection = createTerminalConnection({
  ticket: async (id, sessionId) => {
    const { data } = await getMonitorSSHTicket(id, sessionId);
    if (!data?.ticket) throw new Error("没有拿到终端票据");
    return data.ticket;
  },
  event: handleTerminalEvent,
});
function handleTerminalEvent(event: TerminalEvent) {
  if (event.type === "output") {
    if (event.data) term?.write(event.data);
    return;
  }
  if (event.type === "ready") {
    connecting.value = false;
    connected.value = true;
    setStatus("已连接 · 任务保护已启用");
    term?.reset();
    fit?.fit();
    if (term) connection.resize(term.cols, term.rows);
    if (!mobile.value) term?.focus();
    return;
  }
  connected.value = false;
  connecting.value = ["connecting", "reconnecting"].includes(event.type);
  if (event.type === "connecting") setStatus("正在连接会话…");
  else if (event.type === "reconnecting")
    setStatus("连接中断 · 正在恢复原会话…");
  else if (event.type === "exit") {
    setStatus(event.error || "会话已结束，请新建会话");
    void refreshSessions(false);
  } else if (event.type === "unauthorized")
    setStatus(event.error || "终端权限已失效");
  else if (event.type === "error") setStatus(event.error || "连接失败");
  else setStatus("已离开 · 会话保留在服务器");
}
let observer: ResizeObserver | undefined;

const connecting = ref(false);
const controlPressed = ref(false);
const showCommands = ref(false);
const selectionAvailable = ref(false);
const viewportHeight = ref(window.visualViewport?.height || window.innerHeight);
const mobile = ref(window.innerWidth <= 640);
const keys = [
  { label: "Tab", data: "\t" },
  { label: "Esc", data: "\x1b" },
  { label: "↑", data: "\x1b[A" },
  { label: "↓", data: "\x1b[B" },
  { label: "←", data: "\x1b[D" },
  { label: "→", data: "\x1b[C" },
  { label: "Ctrl+C", data: "\x03" },
  { label: "Ctrl+D", data: "\x04" },
];
function updateViewport() {
  viewportHeight.value = window.visualViewport?.height || window.innerHeight;
  mobile.value = window.innerWidth <= 640;
}
function sendInput(data: string) {
  connection.input(data);
}
function pressKey(data: string) {
  sendInput(data);
}
function focusKeyboard() {
  term?.focus();
}
async function copySelection() {
  const text = term?.getSelection();
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("浏览器未允许复制，请使用查看文字手动复制");
  }
}
async function pasteText() {
  try {
    const { value } = await ElMessageBox.prompt(
      "粘贴内容后发送到终端；多行内容可能执行命令。",
      "粘贴文字",
      {
        inputType: "textarea",
        confirmButtonText: "发送",
        cancelButtonText: "取消",
      }
    );
    if (!connected.value) {
      ElMessage.warning("连接已断开，请重连后再粘贴");
      return;
    }
    term?.paste(value || "");
  } catch {
    /* cancelled */
  }
}
const outputVisible = ref(false);
const outputText = ref("");
function viewOutput() {
  const buffer = term?.buffer.active;
  if (!buffer) return;
  const lines: string[] = [];
  for (let i = Math.max(0, buffer.length - 300); i < buffer.length; i++) {
    lines.push(buffer.getLine(i)?.translateToString(true) || "");
  }
  outputText.value = lines.join("\n");
  outputVisible.value = true;
}

const nodeScriptsCount = computed(
  () => quickCommands.filter((c) => c.source === "nodescripts").length
);

const tabCommands = computed(() =>
  quickCommands.filter((command) =>
    commandTab.value === "nodescripts"
      ? command.source === "nodescripts"
      : command.source !== "nodescripts"
  )
);

const availableCategories = computed(() => {
  const counts = new Map<string, number>();
  for (const command of tabCommands.value) {
    counts.set(command.category, (counts.get(command.category) || 0) + 1);
  }
  return [
    ...[...counts.entries()].map(([name, count]) => ({ name, count })),
    { name: "全部", count: tabCommands.value.length },
  ];
});

function switchCommandTab(tab: "builtin" | "nodescripts") {
  commandTab.value = tab;
  commandQuery.value = "";
  selectedCategory.value = tab === "nodescripts" ? "测速与检测" : "全部";
}

const categories = computed(() => {
  const q = commandQuery.value.trim().toLowerCase();
  const map = new Map<string, QuickCommand[]>();
  for (const command of tabCommands.value) {
    if (
      !q &&
      selectedCategory.value !== "全部" &&
      command.category !== selectedCategory.value
    )
      continue;
    if (
      q &&
      !`${command.name} ${command.category} ${command.description} ${command.command}`
        .toLowerCase()
        .includes(q)
    )
      continue;
    const list = map.get(command.category) || [];
    list.push(command);
    map.set(command.category, list);
  }
  return [...map.entries()];
});

function setStatus(text: string) {
  status.value = text;
}

function createTerminal() {
  if (!terminalHost.value || term) return;
  term = new Terminal({
    cursorBlink: true,
    fontSize: 13,
    fontFamily:
      '"JetBrains Mono", "SFMono-Regular", Consolas, "Liberation Mono", monospace',
    scrollback: 5000,
    theme: {
      background: "#0b0b0e",
      foreground: "#f0f0ed",
      cursor: "#29df7e",
      selectionBackground: "rgba(41, 223, 126, 0.22)",
      black: "#141418",
      red: "#f04848",
      green: "#29df7e",
      yellow: "#f59e0b",
      blue: "#5b92f0",
      magenta: "#c084fc",
      cyan: "#38bdf8",
      white: "#f0f0ed",
    },
  });
  fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminalHost.value);
  term.attachCustomWheelEventHandler(() => {
    // Prevent xterm.js alternate-screen fallback from sending Up/Down arrow keys
    // (which cycles bash command history instead of scrolling terminal output).
    if (
      term?.buffer.active.type === "alternate" &&
      term.modes.mouseTrackingMode === "none"
    ) {
      return false;
    }
    return true;
  });
  term.onData((data) => {
    if (controlPressed.value) {
      controlPressed.value = false;
      if (/^[a-z@\[\]\\^_]$/i.test(data))
        data = String.fromCharCode(data.toUpperCase().charCodeAt(0) & 31);
    }
    sendInput(data);
  });
  term.onSelectionChange(() => {
    selectionAvailable.value = !!term?.hasSelection();
  });
  term.onResize(({ cols, rows }) => {
    connection.resize(cols, rows);
  });
  fit.fit();
  observer = new ResizeObserver(() => {
    try {
      fit?.fit();
    } catch {
      /* ignore */
    }
  });
  observer.observe(terminalHost.value);
  if (!mobile.value) term.focus();
}

function disconnect() {
  loadGeneration++;
  connection.leave();
  controlPressed.value = false;
  connected.value = false;
  connecting.value = false;
  selectionAvailable.value = false;
}
async function connect() {
  if (
    !props.monitor ||
    !visible.value ||
    connecting.value ||
    !selectedSession.value
  )
    return;
  await connection.connect(props.monitor.id, selectedSession.value);
}
async function refreshSessions(initial = false) {
  if (!props.monitor || !visible.value) return;
  const id = props.monitor.id,
    generation = ++loadGeneration;
  sessionLoading.value = true;
  try {
    const { data } = await getTerminalSessions(id);
    if (
      generation !== loadGeneration ||
      !visible.value ||
      props.monitor?.id !== id
    )
      return;
    sessions.value = data.sessions || [];
    sessionReady.value = data.ready;
    sessionSupported.value = data.supported;
    if (!data.ready) {
      setStatus(data.error || "任务保护尚未就绪");
      return;
    }
    const previous =
      selectedSession.value || localStorage.getItem(sessionKey());
    if (previous && sessions.value.some((s) => s.id === previous))
      selectedSession.value = previous;
    else if (previous) {
      selectedSession.value = "";
      setStatus("上次会话已结束，请选择其他会话或新建");
    } else if (sessions.value.length)
      selectedSession.value = sessions.value[0].id;
    else if (initial) {
      const created = await createTerminalSession(id, "终端会话");
      if (
        generation !== loadGeneration ||
        !visible.value ||
        props.monitor?.id !== id
      )
        return;
      sessions.value = [created.data];
      selectedSession.value = created.data.id;
    }
    if (selectedSession.value)
      localStorage.setItem(sessionKey(), selectedSession.value);
  } catch (error: any) {
    if (generation === loadGeneration)
      setStatus(error?.message || "无法加载会话");
  } finally {
    if (generation === loadGeneration) sessionLoading.value = false;
  }
}
async function open() {
  await refreshSessions(true);
  if (visible.value) await connect();
}
async function selectSession() {
  disconnect();
  localStorage.setItem(sessionKey(), selectedSession.value);
  await connect();
}
async function newSession() {
  if (!props.monitor || sessionLoading.value) return;
  const id = props.monitor.id,
    generation = loadGeneration;
  try {
    const { value } = await ElMessageBox.prompt(
      "为新的终端会话命名。现有会话继续运行。",
      "新建会话",
      {
        inputValue: "终端会话",
        confirmButtonText: "创建",
        cancelButtonText: "取消",
      }
    );
    if (
      generation !== loadGeneration ||
      !visible.value ||
      props.monitor?.id !== id
    )
      return;
    sessionLoading.value = true;
    const { data } = await createTerminalSession(id, value);
    if (
      generation !== loadGeneration ||
      !visible.value ||
      props.monitor?.id !== id
    )
      return;
    sessionLoading.value = false;
    disconnect();
    sessions.value.push(data);
    selectedSession.value = data.id;
    localStorage.setItem(sessionKey(), data.id);
    await connect();
  } catch (error: any) {
    if (error?.message) ElMessage.warning(error.message);
  } finally {
    if (generation === loadGeneration) sessionLoading.value = false;
  }
}
async function terminateSession() {
  if (!props.monitor || !selectedSession.value) return;
  const id = props.monitor.id,
    sessionId = selectedSession.value,
    generation = loadGeneration;
  try {
    await ElMessageBox.confirm(
      "这会结束此会话及其中正在运行的任务。关闭窗口可保留任务。",
      "结束会话",
      {
        type: "warning",
        confirmButtonText: "结束任务",
        cancelButtonText: "取消",
      }
    );
    if (
      generation !== loadGeneration ||
      !visible.value ||
      props.monitor?.id !== id ||
      selectedSession.value !== sessionId
    )
      return;
    await endTerminalSession(id, sessionId);
    if (
      generation !== loadGeneration ||
      !visible.value ||
      props.monitor?.id !== id ||
      selectedSession.value !== sessionId
    )
      return;
    disconnect();
    selectedSession.value = "";
    setStatus("会话已结束");
    await refreshSessions(false);
  } catch (error: any) {
    if (error?.message) ElMessage.warning(error.message);
  }
}
async function enableProtection() {
  if (!props.monitor || preparing.value) return;
  const id = props.monitor.id;
  preparing.value = true;
  setStatus("正在准备任务保护…");
  try {
    await prepareTerminal(id);
    if (visible.value && props.monitor?.id === id) await open();
  } catch (error: any) {
    setStatus(error?.message || "启用失败，请稍后重试");
  } finally {
    preparing.value = false;
  }
}

watch([visible, () => props.monitor?.id], async ([value]) => {
  loadGeneration++;
  disconnect();
  selectedSession.value = "";
  sessions.value = [];
  sessionReady.value = false;
  sessionSupported.value = false;
  if (value) {
    updateViewport();
    window.visualViewport?.addEventListener("resize", updateViewport);
    window.addEventListener("resize", updateViewport);
    await nextTick();
    if (!visible.value) return;
    createTerminal();
    open();
  } else {
    window.visualViewport?.removeEventListener("resize", updateViewport);
    window.removeEventListener("resize", updateViewport);
    outputVisible.value = false;
    showCommands.value = false;
    disconnect();
    observer?.disconnect();
    observer = undefined;
    term?.dispose();
    term = undefined;
    fit = undefined;
  }
});

onBeforeUnmount(() => {
  window.visualViewport?.removeEventListener("resize", updateViewport);
  window.removeEventListener("resize", updateViewport);
  disconnect();
  observer?.disconnect();
  term?.dispose();
});

function runCommand(command: QuickCommand) {
  selectedCommand.value = command;
  selectedVariant.value = 0;
  draftTitle.value = command.name;
  commandDraft.value = command.command;
}
function changeVariant() {
  const variant = selectedCommand.value?.variants?.[selectedVariant.value];
  if (variant) commandDraft.value = variant.command;
}
async function executeDraft() {
  if (!connected.value || !commandDraft.value.trim()) {
    ElMessage.warning("请先连接会话");
    return;
  }
  const command = commandDraft.value;
  if (selectedCommand.value?.dangerous || selectedCommand.value?.advanced) {
    try {
      await ElMessageBox.confirm(
        `将在「${props.monitor?.name}」执行：\n${command}`,
        "执行指令",
        { type: "warning", confirmButtonText: "执行", cancelButtonText: "取消" }
      );
    } catch {
      return;
    }
  }
  if (!connection.input(command + "\r")) {
    ElMessage.warning("连接已断开，指令未发送");
    return;
  }
  commandDraft.value = "";
  term?.focus();
}
</script>
<template>
  <UiDialog
    v-model="visible"
    :title="`Agent 终端 · ${monitor?.name || ''}`"
    width="1180px"
    :destroy-on-close="false"
    :fullscreen="mobile"
    :close-on-press-escape="false"
    :close-on-click-modal="false"
  >
    <div
      class="ssh-shell"
      :style="{ '--terminal-viewport': `${viewportHeight}px` }"
    >
      <section class="terminal-pane">
        <div class="terminal-bar">
          <span class="dot" :class="{ 'is-on': connected }"></span>
          <span class="status">{{ status }}</span>
          <UiTag :tone="connected ? 'ok' : 'neutral'">{{
            connected ? "任务保护已启用" : "未连接"
          }}</UiTag>
          <span class="spacer"></span>
          <UiButton
            :disabled="connecting || connected || !selectedSession"
            @click="connect"
            >{{ connecting ? "连接中" : "重连" }}</UiButton
          >
        </div>
        <div class="session-bar">
          <select
            v-model="selectedSession"
            aria-label="终端会话"
            :disabled="sessionLoading || !sessionReady"
            @change="selectSession"
          >
            <option value="" disabled>选择会话</option>
            <option
              v-for="session in sessions"
              :key="session.id"
              :value="session.id"
            >
              {{ session.name }} ·
              {{ session.attached ? "已连接" : "后台保留" }}
            </option>
          </select>
          <UiButton
            :disabled="sessionLoading || !sessionReady"
            @click="newSession"
            >新建</UiButton
          >
          <UiButton :loading="sessionLoading" @click="refreshSessions(false)"
            >会话列表</UiButton
          >
          <UiButton :disabled="!selectedSession" @click="terminateSession"
            >结束会话</UiButton
          >
          <UiButton
            v-if="sessionSupported && !sessionReady"
            :loading="preparing"
            @click="enableProtection"
            >启用任务保护</UiButton
          >
        </div>
        <div ref="terminalHost" class="terminal"></div>
        <div v-if="commandDraft" class="command-draft">
          <label for="terminal-command-draft"
            >{{ draftTitle }} · 确认终端已回到命令行后执行</label
          >
          <select
            v-if="selectedCommand?.variants"
            v-model="selectedVariant"
            aria-label="指令方式"
            @change="changeVariant"
          >
            <option
              v-for="(variant, index) in selectedCommand.variants"
              :key="variant.label"
              :value="index"
            >
              {{ variant.label }}
            </option>
          </select>
          <textarea
            id="terminal-command-draft"
            v-model="commandDraft"
            rows="3"
            spellcheck="false"
          ></textarea>
          <div>
            <UiButton
              variant="primary"
              :disabled="!connected"
              @click="executeDraft"
              >发送并执行</UiButton
            ><UiButton @click="commandDraft = ''">取消</UiButton>
          </div>
        </div>
        <div class="terminal-keys" role="group" aria-label="终端快捷键">
          <button
            type="button"
            :disabled="!connected"
            :aria-pressed="controlPressed"
            @pointerdown.prevent
            @click="controlPressed = !controlPressed"
          >
            Ctrl
          </button>
          <button
            v-for="key in keys"
            :key="key.label"
            type="button"
            :disabled="!connected"
            @pointerdown.prevent
            @click="pressKey(key.data)"
          >
            {{ key.label }}
          </button>
        </div>
        <div class="terminal-actions" role="group" aria-label="终端工具">
          <button type="button" :disabled="!connected" @click="focusKeyboard">
            键盘
          </button>
          <button
            type="button"
            :disabled="!selectionAvailable"
            @click="copySelection"
          >
            复制选中
          </button>
          <button type="button" :disabled="!connected" @click="pasteText">
            粘贴
          </button>
          <button type="button" @click="viewOutput">查看文字</button>
          <button
            type="button"
            :aria-expanded="showCommands"
            @click="showCommands = !showCommands"
          >
            快速指令
          </button>
        </div>
        <p class="terminal-note">
          关闭窗口会保留会话；滚轮可上下翻看历史输出（按住 Shift 拖动可框选复制）。
        </p>
        <UiButton @click="visible = false">离开并保留会话</UiButton>
      </section>
      <aside class="command-pane" :class="{ 'is-expanded': showCommands }">
        <header>
          <div class="command-head">
            <strong>快速指令</strong>
            <small>{{
              commandTab === "nodescripts"
                ? "来源 nodescripts.net · 确认后执行"
                : "点选填入草稿 · 确认后执行"
            }}</small>
          </div>
          <div class="command-tabs" role="tablist" aria-label="指令来源">
            <button
              type="button"
              role="tab"
              :aria-selected="commandTab === 'builtin'"
              :class="{ 'is-active': commandTab === 'builtin' }"
              @click="switchCommandTab('builtin')"
            >
              内置诊断
            </button>
            <button
              type="button"
              role="tab"
              :aria-selected="commandTab === 'nodescripts'"
              :class="{ 'is-active': commandTab === 'nodescripts' }"
              @click="switchCommandTab('nodescripts')"
            >
              脚本库
              <span class="tab-count">{{ nodeScriptsCount }}</span>
            </button>
          </div>
          <input
            v-model="commandQuery"
            type="search"
            class="command-search"
            aria-label="搜索指令或脚本"
            :placeholder="
              commandTab === 'nodescripts'
                ? '搜索全部 32 个脚本…'
                : '搜索内置诊断指令…'
            "
          />
          <div class="category-pills" role="group" aria-label="分类筛选">
            <button
              v-for="cat in availableCategories"
              :key="cat.name"
              type="button"
              class="cat-pill"
              :class="{
                'is-active': !commandQuery.trim() && selectedCategory === cat.name,
              }"
              @click="
                commandQuery = '';
                selectedCategory = cat.name;
              "
            >
              {{ cat.name }}
              <span class="pill-count">{{ cat.count }}</span>
            </button>
          </div>
        </header>
        <div class="command-scroll">
          <p v-if="!categories.length" class="command-empty">
            没有匹配的指令或脚本
          </p>
          <section v-for="[category, list] in categories" :key="category">
            <h4>
              {{ category }}
              <span>{{ list.length }}</span>
            </h4>
            <button
              v-for="command in list"
              :key="command.name"
              type="button"
              class="command-item"
              :class="{ 'is-dangerous': command.dangerous }"
              :title="command.description"
              @click="runCommand(command)"
            >
              <div class="command-title-row">
                <span>{{ command.name }}</span>
                <em v-if="command.dangerous" class="risk-badge">高风险</em>
                <em v-else-if="command.variants?.length" class="variant-badge"
                  >{{ command.variants.length }} 方式</em
                >
              </div>
              <small>{{ command.description }}</small>
            </button>
          </section>
        </div>
      </aside>
    </div>
  </UiDialog>
  <UiDialog
    v-model="outputVisible"
    title="最近 300 行 · 长按选择复制"
    width="720px"
  >
    <textarea
      class="terminal-output"
      :value="outputText"
      readonly
      aria-label="终端输出文字"
      spellcheck="false"
    ></textarea>
  </UiDialog>
</template>
<style scoped>
.session-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 10px 12px;
  background: #121216;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
}
.session-bar select {
  flex: 1;
  min-width: 120px;
  max-width: 100%;
  background: #0b0b0e;
  color: #f0f0ed;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  padding: 7px 10px;
}
.command-draft {
  display: grid;
  gap: 8px;
  padding: 12px;
  color: #f0f0ed;
  background: #121216;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 12px;
}
.command-draft textarea {
  width: 100%;
  box-sizing: border-box;
  background: #0b0b0e;
  color: #f0f0ed;
  font: 12.5px/1.6 var(--font-mono);
  padding: 10px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
}
.command-draft > div {
  display: flex;
  gap: 8px;
}
.advanced-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.ssh-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  grid-template-rows: minmax(0, 1fr);
  gap: 14px;
  height: clamp(540px, 74vh, 720px);
  overflow: hidden;
}
.terminal-pane {
  display: flex;
  min-width: 0;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: #0b0b0e;
}
.terminal-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.07);
  background: #121216;
  color: #c2c5ce;
  font-size: 12px;
}
.terminal-bar .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #525560;
}
.terminal-bar .dot.is-on {
  background: #29df7e;
  box-shadow: 0 0 0 3px rgba(41, 223, 126, 0.18);
}
.terminal-bar .spacer {
  flex: 1;
}
.terminal {
  flex: 1;
  min-height: 180px;
  overflow: hidden;
  padding: 8px 6px 4px 10px;
}
.terminal-note {
  margin: 0;
  padding: 6px 12px 10px;
  color: rgba(240, 240, 237, 0.56);
  font-size: 11px;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
}
.terminal-note a {
  color: #29df7e;
  text-decoration: none;
}
.terminal-note a:hover {
  text-decoration: underline;
}
.command-pane {
  display: flex;
  min-height: 0;
  height: 100%;
  flex-direction: column;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  overflow: hidden;
}
.command-pane > header {
  flex-shrink: 0;
  display: grid;
  gap: 8px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--c-line);
}
.command-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
.command-tabs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 4px;
  padding: 3px;
  border: 1px solid var(--c-line);
  border-radius: 9px;
  background: var(--c-surface-2);
}
.command-tabs button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 6px 8px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: background var(--t-fast) var(--ease-expo), color var(--t-fast) var(--ease-expo);
}
.command-tabs button.is-active {
  background: var(--c-surface);
  color: var(--c-ink);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.08);
}
.tab-count {
  padding: 1px 5px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-family: var(--font-mono);
  font-size: 10px;
}
.command-search {
  width: 100%;
  box-sizing: border-box;
  padding: 7px 10px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  outline: none;
}
.command-search:focus {
  border-color: var(--c-ink);
}
.category-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}
.cat-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11px;
  cursor: pointer;
  transition: background var(--t-fast) var(--ease-expo), color var(--t-fast) var(--ease-expo), border-color var(--t-fast) var(--ease-expo);
}
.pill-count {
  font-family: var(--font-mono);
  font-size: 10px;
  opacity: 0.65;
}
.cat-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}
.cat-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}
.cat-pill.is-active .pill-count {
  opacity: 0.85;
}
.script-notice {
  margin: 0;
  color: var(--c-muted);
  font-size: 11px;
  line-height: 1.45;
}
.command-empty {
  margin: 24px 0;
  text-align: center;
  color: var(--c-muted);
  font-size: 12px;
}
.command-pane strong {
  font-size: 13px;
  font-weight: 500;
  letter-spacing: -0.01em;
}
.command-pane small {
  color: var(--c-muted);
  font-size: 11px;
}
.command-scroll {
  min-height: 0;
  flex: 1;
  overflow: auto;
  padding: 10px;
}
.command-scroll section + section {
  margin-top: 12px;
}
.command-scroll h4 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 0 0 6px;
  padding: 0 8px;
  color: var(--c-subtle);
  font-size: 10.5px;
  font-weight: 500;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.command-scroll h4 span {
  font-family: var(--font-mono);
  font-size: 10px;
  opacity: 0.7;
}
.command-item {
  display: grid;
  width: 100%;
  gap: 2px;
  padding: 8px 10px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--c-ink);
  cursor: pointer;
  text-align: left;
  transition: background var(--t-fast) var(--ease-expo), border-color var(--t-fast) var(--ease-expo), transform var(--t-fast) var(--ease-expo);
}
.command-item:hover {
  background: var(--c-surface-2);
  border-color: var(--c-line);
  transform: translateX(2px);
}
.command-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
}
.command-item span {
  font-size: 12.5px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.risk-badge,
.variant-badge {
  flex-shrink: 0;
  padding: 1px 5px;
  border-radius: 4px;
  font-style: normal;
  font-size: 10px;
  font-weight: 500;
}
.risk-badge {
  background: rgba(239, 68, 68, 0.12);
  color: #dc2626;
}
.variant-badge {
  background: var(--c-surface-2);
  color: var(--c-muted);
}
.command-item small {
  overflow: hidden;
  color: var(--c-muted);
  font-family: var(--font-mono);
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
@media (max-width: 900px) {
  .ssh-shell {
    grid-template-columns: 1fr;
    grid-template-rows: minmax(320px, 1fr) 260px;
    height: min(82vh, 760px);
  }
  .command-pane {
    height: 100%;
    max-height: 260px;
  }
}

.terminal-keys,
.terminal-actions {
  display: flex;
  gap: 6px;
  padding: 8px 10px;
  overflow-x: auto;
  flex-shrink: 0;
  background: #101014;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}
.terminal-keys button,
.terminal-actions button {
  flex-shrink: 0;
  min-width: 44px;
  min-height: 44px;
  padding: 0 12px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 8px;
  color: #f0f0ed;
  background: #17171c;
  touch-action: manipulation;
  font-family: var(--font-mono);
  font-size: 12.5px;
  cursor: pointer;
  transition: background var(--t-fast) var(--ease-expo), border-color var(--t-fast) var(--ease-expo), color var(--t-fast) var(--ease-expo);
}
.terminal-keys button:hover:not(:disabled),
.terminal-actions button:hover:not(:disabled) {
  background: #222229;
  border-color: rgba(255, 255, 255, 0.24);
}
.terminal-keys button[aria-pressed="true"] {
  background: #29df7e;
  border-color: #29df7e;
  color: #070707;
  font-weight: 600;
}
.terminal-keys button:disabled,
.terminal-actions button:disabled {
  opacity: 0.36;
  cursor: not-allowed;
}
.terminal-keys button:focus-visible,
.terminal-actions button:focus-visible {
  outline: 2px solid #29df7e;
  outline-offset: -2px;
}
.terminal-output {
  width: 100%;
  min-height: 45dvh;
  resize: vertical;
  font: 13px/1.6 monospace;
  color: var(--c-ink);
  background: var(--c-surface);
}
@media (max-width: 640px) {
  .ssh-shell {
    display: flex;
    flex-direction: column;
    min-height: 0;
    height: max(240px, calc(var(--terminal-viewport) - 90px));
    gap: 6px;
  }
  .terminal-pane {
    flex: 1;
    min-height: 0;
    border-radius: 8px;
  }
  .terminal {
    min-height: 60px;
    padding: 6px 4px;
    overflow: hidden;
  }
  .terminal :deep(.xterm-helper-textarea) {
    font-size: 16px;
  }
  .terminal-note {
    display: none;
  }
  .terminal-bar {
    flex-wrap: wrap;
    gap: 5px;
  }
  .terminal-bar .status {
    flex: 1;
    min-width: 80px;
    overflow-wrap: anywhere;
  }
  .terminal-bar .spacer {
    display: none;
  }
  .terminal-actions {
    padding-bottom: max(6px, env(safe-area-inset-bottom));
  }
  .command-pane {
    display: none;
    max-height: 280px;
    flex-shrink: 0;
  }
  .command-pane.is-expanded {
    display: flex;
  }
}
</style>
