<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import {
  onBeforeRouteLeave,
  onBeforeRouteUpdate,
  useRoute,
  useRouter,
} from "vue-router";
import gsap from "gsap";
import IconTerminal from "~icons/ph/terminal-window";
import IconFolder from "~icons/ph/folder-open";
import IconCube from "~icons/ph/cube";
import IconRefresh from "~icons/ph/arrows-clockwise";
import { UiButton, UiSkeleton, EmptyState } from "@/ui";
import { getMonitors, type MonitorItem } from "@/api/monitor";
import ServerFiles from "./ServerFiles.vue";
import ServerDocker from "./ServerDocker.vue";
import MonitorTerminal from "./MonitorTerminal.vue";

defineOptions({ name: "MonitorWorkspace" });

const route = useRoute();
const router = useRouter();
const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const phase = ref<"loading" | "ready" | "error">("loading");
const error = ref("");
const items = ref<MonitorItem[]>([]);
const offline = ref(!navigator.onLine);

type WorkspaceTab = "terminal" | "files" | "docker";
const tab = computed<WorkspaceTab>(() => {
  if (route.query.tab === "docker") return "docker";
  if (route.query.tab === "terminal") return "terminal";
  return "files";
});

const id = computed(() => {
  const raw = route.query.id;
  return typeof raw === "string" && /^[1-9]\d*$/.test(raw) ? Number(raw) : 0;
});

const selected = computed(() =>
  items.value.find((item) => item.id === id.value)
);

const visitedFiles = ref(tab.value === "files");
const visitedDocker = ref(tab.value === "docker");
const files = ref<InstanceType<typeof ServerFiles>>();
const terminalVisible = ref(false);

watch(tab, (value) => {
  if (value === "files") visitedFiles.value = true;
  if (value === "docker") visitedDocker.value = true;
});

async function load() {
  phase.value = "loading";
  error.value = "";
  try {
    const { data } = await getMonitors();
    items.value = data?.items || [];
    phase.value = "ready";
    if (!route.query.id && items.value.length) {
      await router.replace({
        query: {
          ...route.query,
          id: String(items.value[0].id),
          tab: tab.value,
        },
      });
    }
  } catch (e: any) {
    phase.value = "error";
    error.value =
      e?.message || String(e || "无法读取服务器，请检查网络并重试。");
  }
}

function chooseServer(serverId: number) {
  if (serverId === id.value) return;
  void router.push({
    query: {
      ...route.query,
      id: String(serverId),
      tab: tab.value,
    },
  });
}

function chooseTab(next: WorkspaceTab) {
  void router.replace({
    query: { ...route.query, tab: next },
  });
}

async function canLeave() {
  return (await files.value?.confirmDiscard()) ?? true;
}
onBeforeRouteLeave(canLeave);
onBeforeRouteUpdate(async (to) => {
  if (to.query.id !== route.query.id) return canLeave();
  return true;
});

function connectionChanged() {
  offline.value = !navigator.onLine;
}

onMounted(() => {
  void load();
  window.addEventListener("online", connectionChanged);
  window.addEventListener("offline", connectionChanged);
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  window.removeEventListener("online", connectionChanged);
  window.removeEventListener("offline", connectionChanged);
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="server-hub">
    <!-- Compact Editorial Masthead -->
    <header class="hub-masthead" aria-labelledby="hub-title">
      <div class="masthead-left">
        <div class="title-cluster">
          <div class="title-mask">
            <h1 id="hub-title" class="gallery-title" data-title-reveal>
              服务器管理
            </h1>
          </div>
          <span v-if="selected" class="selected-meta mono">
            {{ selected.name }} · {{ selected.online ? "在线" : "离线" }}
          </span>
        </div>
        <p class="gallery-lead">
          WebShell 终端、远程文件与 Docker 容器一体化工作台。
        </p>
      </div>

      <div class="masthead-actions">
        <UiButton
          v-if="selected?.proxyServerId"
          variant="primary"
          :icon="IconTerminal"
          :disabled="offline"
          @click="terminalVisible = true"
        >
          打开 WebShell
        </UiButton>
        <UiButton
          variant="ghost"
          :icon="IconRefresh"
          :disabled="offline"
          :loading="phase === 'loading'"
          @click="load"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Refactored Host Switcher Rail + 3-in-1 Mode Tabs -->
    <section v-if="phase === 'loading'" class="hub-state">
      <UiSkeleton :lines="3" height="56px" />
    </section>

    <div v-else-if="phase === 'error'" class="hub-error" role="alert">
      <span>{{ error }}</span>
      <UiButton :disabled="offline" @click="load">重试</UiButton>
    </div>

    <EmptyState
      v-else-if="!items.length"
      size="section"
      title="还没有接入服务器"
      description="请先在「主机与 Agent」添加服务器，即可使用终端、文件与容器管理。"
    >
      <template #action>
        <UiButton variant="primary" @click="router.push('/monitor/index')">
          前往主机与 Agent
        </UiButton>
      </template>
    </EmptyState>

    <template v-else>
      <div class="hub-control-bar">
        <!-- Tactile Host Pill Switcher (Replaces Ugly Raw Select) -->
        <div class="host-rail" role="tablist" aria-label="选择主机">
          <button
            v-for="item in items"
            :key="item.id"
            type="button"
            role="tab"
            class="host-pill"
            :class="{ 'is-active': item.id === id }"
            :aria-selected="item.id === id"
            @click="chooseServer(item.id)"
          >
            <i
              class="host-dot"
              :class="item.online ? 'is-on' : 'is-off'"
              aria-hidden="true"
            ></i>
            <span class="host-name">{{ item.name }}</span>
            <small v-if="item.os" class="host-os mono">{{ item.os }}</small>
          </button>
        </div>

        <!-- 3-in-1 Unified Workspace Tabs -->
        <nav
          v-if="selected?.proxyServerId"
          class="mode-tabs"
          role="tablist"
          aria-label="管理模块"
        >
          <button
            type="button"
            role="tab"
            class="mode-tab"
            :class="{ 'is-active': tab === 'terminal' }"
            :aria-selected="tab === 'terminal'"
            @click="chooseTab('terminal')"
          >
            <IconTerminal aria-hidden="true" />
            <span>WebShell</span>
          </button>
          <button
            type="button"
            role="tab"
            class="mode-tab"
            :class="{ 'is-active': tab === 'files' }"
            :aria-selected="tab === 'files'"
            @click="chooseTab('files')"
          >
            <IconFolder aria-hidden="true" />
            <span>文件管理</span>
          </button>
          <button
            type="button"
            role="tab"
            class="mode-tab"
            :class="{ 'is-active': tab === 'docker' }"
            :aria-selected="tab === 'docker'"
            @click="chooseTab('docker')"
          >
            <IconCube aria-hidden="true" />
            <span>Docker 容器</span>
          </button>
        </nav>
      </div>

      <!-- Stage Content -->
      <div class="hub-stage">
        <EmptyState
          v-if="!selected"
          size="section"
          title="请选择一台主机"
          description="点击上方任意主机胶囊即可切换工作台。"
        />

        <EmptyState
          v-else-if="!selected.proxyServerId"
          size="section"
          title="未关联 Agent 通道"
          description="该主机当前仅开启基础监控；终端、文件与 Docker 需要通过自建 Agent 通道。"
        >
          <template #action>
            <UiButton variant="primary" @click="router.push('/monitor/index')">
              返回主机与 Agent
            </UiButton>
          </template>
        </EmptyState>

        <template v-else>
          <!-- WebShell Launch & Studio Card -->
          <section
            v-show="tab === 'terminal'"
            class="webshell-stage"
            aria-label="WebShell 终端"
          >
            <div class="terminal-launch-card">
              <div class="launch-top">
                <span class="launch-status mono">
                  <i
                    :class="selected.online ? 'is-on' : 'is-off'"
                    aria-hidden="true"
                  ></i>
                  {{ selected.online ? "AGENT ONLINE" : "AGENT OFFLINE" }}
                </span>
                <span class="launch-arch mono">
                  {{
                    selected.os && selected.arch
                      ? `${selected.os}/${selected.arch}`
                      : "linux"
                  }}
                </span>
              </div>

              <div class="launch-body">
                <h2>{{ selected.name }}</h2>
                <p>
                  基于 Agent 加密通道的交互式 WebShell，支持多会话切换、常用诊断命令与触控快捷键。
                </p>
              </div>

              <div class="launch-actions">
                <UiButton
                  variant="primary"
                  :icon="IconTerminal"
                  :disabled="offline"
                  @click="terminalVisible = true"
                >
                  连接 WebShell 终端
                </UiButton>
              </div>
            </div>
          </section>

          <div v-if="visitedFiles" v-show="tab === 'files'">
            <ServerFiles
              :key="`files-${id}`"
              ref="files"
              :id="id"
              :offline="offline"
            />
          </div>

          <div v-if="visitedDocker" v-show="tab === 'docker'">
            <ServerDocker
              :key="`docker-${id}`"
              :id="id"
              :offline="offline"
            />
          </div>
        </template>
      </div>
    </template>

    <!-- Preserved Battle-Tested WebShell Terminal with Upgraded Studio Styling -->
    <MonitorTerminal
      v-if="selected?.proxyServerId"
      v-model="terminalVisible"
      :monitor="selected"
    />
  </div>
</template>

<style scoped>
.server-hub {
  display: grid;
  gap: 20px;
  max-width: 1200px;
  min-width: 0;
  padding-bottom: 56px;
}

.hub-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.masthead-left {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.title-cluster {
  display: flex;
  align-items: baseline;
  gap: 16px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.selected-meta {
  font-size: 12.5px;
  color: var(--c-muted);
}

.gallery-lead {
  margin: 0;
  font-size: 13.5px;
  color: var(--c-muted);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

.hub-state {
  padding: 20px 0;
}

.hub-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 18px;
  border: 1px solid rgb(223 41 41 / 0.25);
  border-radius: 14px;
  background: rgb(223 41 41 / 0.05);
  color: var(--c-bad);
  font-size: 13.5px;
}

.hub-control-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px 20px;
  flex-wrap: wrap;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--c-line);
}

.host-rail {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
  min-width: 0;
}

.host-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition:
    border-color var(--duration-fast) var(--ease-out),
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.host-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.host-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.host-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-muted);
  flex: none;
}

.host-dot.is-on {
  background: var(--c-ok);
}

.host-pill.is-active .host-dot.is-on {
  background: #29df7e;
}

.host-os {
  font-size: 11px;
  opacity: 0.65;
}

.mode-tabs {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
}

.mode-tab {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out);
}

.mode-tab svg {
  width: 15px;
  height: 15px;
}

.mode-tab:hover {
  color: var(--c-ink);
}

.mode-tab.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.hub-stage {
  min-width: 0;
}

/* Obsidian WebShell Studio Launch Card */
.terminal-launch-card {
  display: grid;
  gap: 22px;
  padding: 28px 30px;
  border-radius: 20px;
  background:
    radial-gradient(
      circle at 90% 10%,
      rgb(41 223 126 / 0.12),
      transparent 44%
    ),
    #0b0b0e;
  border: 1px solid rgb(255 255 255 / 0.1);
  color: #f0f0ed;
  box-shadow: 0 26px 60px -22px rgb(0 0 0 / 0.32);
}

.launch-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: 11.5px;
  letter-spacing: 0.06em;
  color: rgb(255 255 255 / 0.56);
}

.launch-status {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.launch-status i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #666;
}

.launch-status i.is-on {
  background: #29df7e;
  box-shadow: 0 0 0 3px rgb(41 223 126 / 0.22);
}

.launch-body {
  display: grid;
  gap: 8px;
}

.launch-body h2 {
  margin: 0;
  font-family: var(--font-display);
  font-size: 26px;
  font-weight: 500;
  letter-spacing: -0.03em;
  color: #ffffff;
}

.launch-body p {
  margin: 0;
  max-width: 56ch;
  font-size: 13.5px;
  line-height: 1.6;
  color: rgb(255 255 255 / 0.64);
}

.launch-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.launch-actions :deep(.el-button--primary) {
  background: #ffffff;
  border-color: #ffffff;
  color: #0b0b0e;
}

.launch-actions :deep(.el-button--primary:hover) {
  background: #29df7e;
  border-color: #29df7e;
  color: #070707;
}

@media (max-width: 720px) {
  .mode-tabs {
    width: 100%;
    justify-content: space-between;
  }
  .mode-tab {
    flex: 1;
    justify-content: center;
    padding: 7px 10px;
  }
  .terminal-launch-card {
    padding: 22px;
  }
}
</style>
