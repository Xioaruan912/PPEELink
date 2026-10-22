<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from "vue";
import { ElMessageBox } from "element-plus";
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
    sshConfigured?: boolean;
  } | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  "open-settings": [];
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
let pendingCommand = "";

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
      background: "#0f140f",
      foreground: "#d8e2ce",
      cursor: "#9ccc3c",
      selectionBackground: "rgba(156, 204, 60, 0.28)",
      black: "#1b211a",
      red: "#e06c75",
      green: "#9ccc3c",
      yellow: "#e5c07b",
      blue: "#61afef",
      magenta: "#c678dd",
      cyan: "#56b6c2",
      white: "#d8e2ce",
    },
  });
  fit = new FitAddon();
  term.loadAddon(fit);
  term.open(terminalHost.value);
  term.onData((data) => {
    if (socket?.readyState === WebSocket.OPEN) {
      socket.send(JSON.stringify({ type: "input", data }));
    }
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
  term.focus();
}

function disconnect() {
  pendingCommand = "";
  if (socket) {
    socket.onclose = null;
    socket.close();
    socket = undefined;
  }
  connected.value = false;
  setStatus("未连接");
}

async function connect() {
  if (!props.monitor) return;
  if (!props.monitor.sshConfigured) {
    setStatus("还没有配置 SSH");
    return;
  }
  disconnect();
  setStatus("正在连接…");
  try {
    const { data } = await getMonitorSSHTicket(props.monitor.id);
    const ticket = data?.ticket;
    if (!ticket) throw new Error("没有拿到 WebSSH 票据");
    const protocol = location.protocol === "https:" ? "wss" : "ws";
    const url = `${protocol}://${location.host}/api/v1/monitors/ssh/ws?ticket=${encodeURIComponent(ticket)}`;
    socket = new WebSocket(url);
    socket.onopen = () => {
      connected.value = true;
      setStatus("已连接");
      fit?.fit();
      if (term) {
        socket?.send(
          JSON.stringify({ type: "resize", cols: term.cols, rows: term.rows })
        );
      }
      if (pendingCommand) {
        const command = pendingCommand;
        pendingCommand = "";
        sendCommand(command);
      }
      term?.focus();
    };
    socket.onmessage = (event) => {
      term?.write(typeof event.data === "string" ? event.data : "");
    };
    socket.onerror = () => setStatus("连接出错");
    socket.onclose = () => {
      connected.value = false;
      setStatus("连接已断开");
    };
  } catch (err: any) {
    setStatus(err?.message || "连接失败");
  }
}

function open() {
  connect();
}

watch(visible, async (value) => {
  if (value) {
    await nextTick();
    createTerminal();
    open();
  } else {
    disconnect();
    observer?.disconnect();
    observer = undefined;
    term?.dispose();
    term = undefined;
    fit = undefined;
  }
});

onBeforeUnmount(() => {
  disconnect();
  observer?.disconnect();
  term?.dispose();
});

function sendCommand(command: string) {
  if (!socket || socket.readyState !== WebSocket.OPEN) {
    pendingCommand = command;
    setStatus("等待连接后执行…");
    connect();
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
    :title="`WebSSH · ${monitor?.name || ''}`"
    width="1120px"
    :destroy-on-close="false"
  >
    <div class="ssh-shell">
      <section class="terminal-pane">
        <div class="terminal-bar">
          <span class="dot" :class="{ 'is-on': connected }"></span>
          <span class="status">{{ status }}</span>
          <UiTag :tone="connected ? 'ok' : 'neutral'">{{
            connected ? "在线" : "未连接"
          }}</UiTag>
          <span class="spacer"></span>
          <UiButton @click="connect">重连</UiButton>
          <UiButton @click="emit('open-settings')">SSH 设置</UiButton>
        </div>
        <div ref="terminalHost" class="terminal"></div>
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
      <aside class="command-pane">
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
  background: #0f140f;
}
.terminal-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: #141a12;
  color: #aeb8a5;
  font-size: 12px;
}
.terminal-bar .dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #6b7563;
}
.terminal-bar .dot.is-on {
  background: #9ccc3c;
  box-shadow: 0 0 0 3px rgba(156, 204, 60, 0.18);
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
  color: #77836d;
  font-size: 11px;
}
.terminal-note a {
  color: #9ccc3c;
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
</style>
