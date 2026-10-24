<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { UiButton, UiDialog, UiTag } from "@/ui";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { getMonitorSSHTicket } from "@/api/monitor";
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
let socket: WebSocket | undefined;
let observer: ResizeObserver | undefined;
let connectionGeneration = 0;
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
  if (socket?.readyState !== WebSocket.OPEN) return;
  socket.send(JSON.stringify({ type: "input", data }));
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

const categories = computed(() => {
  const map = new Map<string, QuickCommand[]>();
  for (const command of quickCommands) {
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
    scrollback: 2000,
    theme: {
      background: "#17191d",
      foreground: "#e2e5eb",
      cursor: "#73c9f4",
      selectionBackground: "rgba(96, 165, 250, 0.3)",
      black: "#1b211a",
      red: "#e06c75",
      green: "#9ccc3c",
      yellow: "#e5c07b",
      blue: "#61afef",
      magenta: "#c678dd",
      cyan: "#56b6c2",
      white: "#e2e5eb",
    },
  });
  fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminalHost.value);
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
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "resize", cols, rows }));
    }
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
  connectionGeneration++;
  connecting.value = false;
  controlPressed.value = false;
  if (socket) {
    socket.onopen = null;
    socket.onmessage = null;
    socket.onerror = null;
    socket.onclose = null;
    socket.close();
    socket = undefined;
  }
  connected.value = false;
  selectionAvailable.value = false;
  setStatus("未连接");
}

async function connect() {
  if (!props.monitor || !visible.value || connecting.value) return;
  if (!props.monitor.proxyServerId) {
    setStatus("该监控未关联自建服务器 agent");
    return;
  }
  disconnect();
  const generation = connectionGeneration;
  connecting.value = true;
  setStatus("正在连接…");
  try {
    const { data } = await getMonitorSSHTicket(props.monitor.id);
    if (generation !== connectionGeneration || !visible.value) return;
    const ticket = data?.ticket;
    if (!ticket) throw new Error("没有拿到 WebSSH 票据");
    const protocol = location.protocol === "https:" ? "wss" : "ws";
    const url = `${protocol}://${location.host}/api/v1/monitors/ssh/ws?ticket=${encodeURIComponent(ticket)}`;
    socket = new WebSocket(url);
    socket.onopen = () => {
      connecting.value = false;
      connected.value = true;
      setStatus("已连接");
      fit?.fit();
      if (term) {
        socket?.send(
          JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })
        );
      }
      if (!mobile.value) term?.focus();
    };
    socket.onmessage = (event) => {
      term?.write(typeof event.data === "string" ? event.data : "");
    };
    socket.onerror = () => setStatus("连接出错");
    socket.onclose = () => {
      connected.value = false;
      connecting.value = false;
      setStatus("已断开 · 重连将建立新会话");
    };
  } catch (err: any) {
    if (generation !== connectionGeneration) return;
    connecting.value = false;
    setStatus(err?.message || "连接失败");
  }
}

function open() {
  connect();
}

watch(visible, async (value) => {
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

function sendCommand(command: string) {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    ElMessage.warning("请先连接终端，再执行指令");
    return;
  }
  socket.send(JSON.stringify({ type: "input", data: `${command}\r` }));
  term?.focus();
}

async function runCommand(command: QuickCommand) {
  if (!props.monitor) return;
  try {
    await ElMessageBox.confirm(
      `将在「${props.monitor.name}」执行：\n${command.name}\n\n${command.command}`,
      command.dangerous ? "高风险脚本" : "执行脚本",
      {
        type: command.dangerous ? "warning" : "info",
        confirmButtonText: "执行",
        cancelButtonText: "取消",
      }
    );
  } catch {
    return;
  }
  sendCommand(command.command);
}
</script>
<template>
  <UiDialog
    v-model="visible"
    :title="`Agent 终端 · ${monitor?.name || ''}`"
    width="1120px"
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
            connected ? "在线" : "未连接"
          }}</UiTag>
          <span class="spacer"></span>
          <UiButton :disabled="connecting" @click="connect">{{
            connecting ? "连接中" : "重连"
          }}</UiButton>
        </div>
        <div ref="terminalHost" class="terminal"></div>
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
          终端会直接操作服务器。脚本来自
          <a
            href="https://github.com/Xioaruan912/Xcript"
            target="_blank"
            rel="noreferrer"
            >Xioaruan912/Xcript</a
          >，交互式脚本可以直接在终端里输入。
        </p>
      </section>
      <aside class="command-pane" :class="{ 'is-expanded': showCommands }">
        <header>
          <strong>快速指令</strong>
          <small>点击后在终端执行</small>
        </header>
        <div class="command-scroll">
          <section v-for="[category, list] in categories" :key="category">
            <h4>{{ category }}</h4>
            <button
              v-for="command in list"
              :key="command.name"
              type="button"
              class="command-item"
              :title="command.description"
              @click="runCommand(command)"
            >
              <span>{{ command.name }}</span>
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
.ssh-shell {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 276px;
  gap: 12px;
  min-height: 520px;
}
.terminal-pane {
  display: flex;
  min-width: 0;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: #17191d;
}
.terminal-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: #202329;
  color: #b7bec9;
  font-size: 12px;
}
.terminal-bar .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #6b7563;
}
.terminal-bar .dot.is-on {
  background: #73c9f4;
  box-shadow: 0 0 0 3px rgba(96, 165, 250, 0.18);
}
.terminal-bar .spacer {
  flex: 1;
}
.terminal {
  flex: 1;
  min-height: 420px;
  padding: 8px 6px 4px 10px;
}
.terminal-note {
  margin: 0;
  padding: 6px 10px 10px;
  color: #a0a8b6;
  font-size: 11px;
}
.terminal-note a {
  color: #73c9f4;
}
.command-pane {
  display: flex;
  min-height: 0;
  flex-direction: column;
  border: 1px solid var(--c-line);
  border-radius: 12px;
  background: var(--c-surface);
}
.command-pane > header {
  display: grid;
  gap: 2px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--c-line);
}
.command-pane strong {
  font-size: 13px;
  font-weight: 500;
}
.command-pane small {
  color: var(--c-muted);
  font-size: 11px;
}
.command-scroll {
  min-height: 0;
  overflow: auto;
  padding: 8px;
}
.command-scroll section + section {
  margin-top: 10px;
}
.command-scroll h4 {
  margin: 0 0 4px;
  padding: 0 6px;
  color: var(--c-muted);
  font-size: 11px;
  font-weight: 500;
}
.command-item {
  display: grid;
  width: 100%;
  gap: 1px;
  padding: 7px 8px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--c-ink);
  cursor: pointer;
  text-align: left;
}
.command-item:hover {
  background: var(--c-surface-2);
}
.command-item span {
  font-size: 12.5px;
}
.command-item small {
  overflow: hidden;
  color: var(--c-muted);
  font-size: 10.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
@media (max-width: 900px) {
  .ssh-shell {
    grid-template-columns: 1fr;
  }
  .command-pane {
    max-height: 240px;
  }
}

.terminal-keys,
.terminal-actions {
  display: flex;
  gap: 4px;
  padding: 6px;
  overflow-x: auto;
  flex-shrink: 0;
  border-top: 1px solid #343840;
}
.terminal-keys button,
.terminal-actions button {
  flex-shrink: 0;
  min-width: 44px;
  min-height: 44px;
  padding: 0 10px;
  border: 1px solid #414752;
  border-radius: 6px;
  color: #e2e5eb;
  background: #252930;
  touch-action: manipulation;
  font-size: 13px;
}
.terminal-keys button[aria-pressed="true"] {
  background: #2563eb;
  color: white;
}
.terminal-keys button:disabled,
.terminal-actions button:disabled {
  opacity: 0.4;
}
.terminal-keys button:focus-visible,
.terminal-actions button:focus-visible {
  outline: 2px solid #93c5fd;
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
    max-height: 160px;
    flex-shrink: 0;
  }
  .command-pane.is-expanded {
    display: flex;
  }
}
</style>
