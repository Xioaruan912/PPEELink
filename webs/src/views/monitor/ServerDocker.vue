<script setup lang="ts">
import IconRefresh from "~icons/ph/arrows-clockwise";
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import { UiButton, UiDialog, UiSkeleton, UiTag } from "@/ui";
import {
  actOnContainer,
  containerLogs,
  containerStats,
  listContainers,
  type DockerAction,
  type DockerContainer,
  type DockerStats,
} from "@/api/monitor/operations";

const props = defineProps<{ id: number; offline: boolean }>();
const phase = ref<"loading" | "ready" | "error">("loading");
const dockerUnavailable = ref(false);
const items = ref<DockerContainer[]>([]);
const stats = ref<DockerStats[]>([]);
const error = ref("");
const statsError = ref("");
const status = ref("");
const busy = ref(false);
const statsBusy = ref(false);
const updated = ref("");

const locked = computed(
  () => busy.value || phase.value === "loading" || props.offline
);
const running = computed(
  () => items.value.filter((item) => item.State === "running").length
);
const statIndex = computed(
  () =>
    new Map(
      stats.value.flatMap((item) =>
        [item.ID, item.Container, item.Name]
          .filter(Boolean)
          .map((key) => [key, item] as const)
      )
    )
);
const statFor = (container: DockerContainer) =>
  statIndex.value.get(container.ID) || statIndex.value.get(container.Names);

let disposed = false;
let generation = 0;

const isDockerUnavailable = (text: string) =>
  /Docker\s*操作失败|未安装|未运行|docker\.sock|permission denied|Cannot connect to the Docker daemon|executable file not found|no such file|not found|unavailable|503/i.test(
    text
  );

const message = (e: unknown) =>
  e instanceof Error ? e.message : "操作失败，请稍后重试。";

async function load() {
  if (props.offline) return;
  const seq = ++generation;
  phase.value = "loading";
  error.value = "";
  statsError.value = "";
  dockerUnavailable.value = false;
  try {
    const data = await listContainers(props.id);
    if (disposed || seq !== generation) return;
    items.value = data.items;
    updated.value = new Date().toLocaleTimeString();
    phase.value = "ready";
    if (items.value.length > 0) {
      void refreshStats();
    } else {
      stats.value = [];
    }
  } catch (e) {
    if (disposed || seq !== generation) return;
    const msg = message(e);
    if (isDockerUnavailable(msg)) {
      dockerUnavailable.value = true;
      items.value = [];
      stats.value = [];
      phase.value = "ready";
    } else {
      error.value = msg;
      phase.value = "error";
    }
  }
}

async function refreshStats() {
  if (props.offline || statsBusy.value || dockerUnavailable.value) return;
  statsBusy.value = true;
  statsError.value = "";
  try {
    const data = await containerStats(props.id);
    if (!disposed) stats.value = data.items;
  } catch (e) {
    if (!disposed) {
      const msg = message(e);
      if (!isDockerUnavailable(msg)) statsError.value = msg;
    }
  } finally {
    statsBusy.value = false;
  }
}

const pending = ref<{ item: DockerContainer; action: DockerAction }>();
const confirmError = ref("");
const actionName = (action: DockerAction) =>
  ({ start: "启动", stop: "停止", restart: "重启" })[action];

function confirm(item: DockerContainer, action: DockerAction) {
  pending.value = { item, action };
  confirmError.value = "";
}

async function act() {
  if (!pending.value || locked.value) return;
  const { item, action } = pending.value;
  busy.value = true;
  confirmError.value = "";
  try {
    await actOnContainer(props.id, item.ID, action);
    if (disposed) return;
    pending.value = undefined;
    status.value = `已${actionName(action)} ${item.Names || item.ID.slice(0, 12)}。`;
    await load();
  } catch (e) {
    confirmError.value = message(e);
  } finally {
    busy.value = false;
  }
}

const logsOpen = ref(false);
const logsItem = ref<DockerContainer>();
const logs = ref("");
const logsError = ref("");
const logsBusy = ref(false);
const logsTruncated = ref(false);

async function showLogs(item: DockerContainer) {
  if (props.offline || logsBusy.value) return;
  logsItem.value = item;
  logsOpen.value = true;
  logsBusy.value = true;
  logsError.value = "";
  logs.value = "";
  logsTruncated.value = false;
  try {
    const data = await containerLogs(props.id, item.ID);
    if (!disposed) {
      logs.value = data.content;
      logsTruncated.value = data.truncated;
    }
  } catch (e) {
    if (!disposed) logsError.value = message(e);
  } finally {
    logsBusy.value = false;
  }
}

onMounted(() => {
  void load();
});
onBeforeUnmount(() => {
  disposed = true;
  generation++;
});
</script>

<template>
  <section class="docker-gallery" aria-label="Docker 容器">
    <header class="docker-toolbar">
      <div class="docker-summary mono">
        <template v-if="dockerUnavailable">
          <span>Docker 环境：无</span>
        </template>
        <template v-else>
          <span>容器 {{ items.length }}</span>
          <span class="is-ok">运行中 {{ running }}</span>
          <span v-if="updated">更新于 {{ updated }}</span>
        </template>
      </div>
      <div class="docker-actions">
        <UiButton
          v-if="!dockerUnavailable && items.length"
          size="sm"
          variant="ghost"
          :loading="statsBusy"
          :disabled="locked"
          @click="refreshStats"
        >
          刷新资源
        </UiButton>
        <UiButton
          size="sm"
          variant="ghost"
          :icon="IconRefresh"
          :disabled="locked"
          @click="load"
        >
          刷新
        </UiButton>
      </div>
    </header>

    <p v-if="status" class="notice ok" role="status">{{ status }}</p>
    <div v-if="error" class="notice error" role="alert">
      <span>{{ error }}</span>
      <UiButton size="sm" variant="ghost" @click="load">重试</UiButton>
    </div>

    <UiSkeleton v-if="phase === 'loading'" :lines="4" height="58px" />

    <!-- Calm Minimalist "无" Empty State when Server Has No Docker -->
    <div v-else-if="dockerUnavailable" class="quiet-empty">
      <strong class="quiet-none">无</strong>
      <p>当前服务器未检测到运行中的 Docker 环境。</p>
      <UiButton size="sm" variant="ghost" @click="load">重新检测</UiButton>
    </div>

    <div v-else-if="!items.length" class="quiet-empty">
      <strong class="quiet-none">无</strong>
      <p>当前服务器尚无容器。</p>
    </div>

    <!-- Borderless Swiss Editorial Container Rows -->
    <div v-else class="container-list">
      <article
        v-for="item in items"
        :key="item.ID"
        class="container-row"
      >
        <div class="container-main">
          <div class="container-head">
            <h3>{{ item.Names || item.ID.slice(0, 12) }}</h3>
            <UiTag :tone="item.State === 'running' ? 'ok' : 'neutral'">
              {{ item.State === "running" ? "运行中" : item.State }}
            </UiTag>
          </div>
          <p class="container-image mono">
            {{ item.Image }} · {{ item.Status }}
          </p>
        </div>

        <dl v-if="statFor(item)" class="container-stats mono">
          <div>
            <dt>CPU</dt>
            <dd>{{ statFor(item)?.CPUPerc || "—" }}</dd>
          </div>
          <div>
            <dt>内存</dt>
            <dd>{{ statFor(item)?.MemUsage || "—" }}</dd>
          </div>
        </dl>
        <span v-else class="container-stats-empty mono">—</span>

        <div class="container-actions">
          <UiButton
            size="sm"
            variant="ghost"
            :disabled="locked"
            @click="showLogs(item)"
          >
            日志
          </UiButton>
          <UiButton
            v-if="item.State !== 'running'"
            size="sm"
            variant="ghost"
            :disabled="locked"
            @click="confirm(item, 'start')"
          >
            启动
          </UiButton>
          <UiButton
            v-else
            size="sm"
            variant="ghost"
            :disabled="locked"
            @click="confirm(item, 'stop')"
          >
            停止
          </UiButton>
          <UiButton
            size="sm"
            variant="ghost"
            :disabled="locked"
            @click="confirm(item, 'restart')"
          >
            重启
          </UiButton>
        </div>
      </article>
    </div>

    <!-- Confirm Action Dialog -->
    <UiDialog
      :model-value="!!pending"
      :title="pending ? `确认${actionName(pending.action)}容器` : ''"
      width="440px"
      @update:model-value="!busy && (pending = undefined)"
    >
      <p v-if="pending" class="confirm-text">
        确定要{{ actionName(pending.action) }}
        <strong class="mono">{{
          pending.item.Names || pending.item.ID.slice(0, 12)
        }}</strong>
        ？
      </p>
      <p v-if="confirmError" class="notice error" role="alert">
        {{ confirmError }}
      </p>
      <template #footer>
        <UiButton variant="ghost" :disabled="busy" @click="pending = undefined">
          取消
        </UiButton>
        <UiButton
          :variant="pending?.action === 'stop' ? 'danger' : 'primary'"
          :loading="busy"
          @click="act"
        >
          确认{{ pending ? actionName(pending.action) : "" }}
        </UiButton>
      </template>
    </UiDialog>

    <!-- Obsidian Container Logs Dialog -->
    <UiDialog
      v-model="logsOpen"
      :title="
        logsItem
          ? `容器日志 · ${logsItem.Names || logsItem.ID.slice(0, 12)}`
          : '容器日志'
      "
      width="min(920px, 95vw)"
    >
      <UiSkeleton v-if="logsBusy" :lines="6" />
      <p v-else-if="logsError" class="notice error" role="alert">
        {{ logsError }}
      </p>
      <pre v-else class="log-console mono">{{ logs || "暂无日志输出" }}</pre>
      <template #footer>
        <UiButton variant="ghost" @click="logsOpen = false">关闭</UiButton>
      </template>
    </UiDialog>
  </section>
</template>

<style scoped>
.docker-gallery {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.docker-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.docker-summary {
  display: inline-flex;
  align-items: baseline;
  gap: 16px;
  font-size: 12.5px;
  color: var(--c-muted);
}

.docker-summary .is-ok {
  color: var(--c-ok);
}

.docker-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
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
}

.notice.ok {
  border-color: rgb(21 184 90 / 0.28);
  color: var(--c-ok);
}

.notice.error {
  border-color: rgb(223 41 41 / 0.28);
  color: var(--c-bad);
}

.quiet-empty {
  display: grid;
  justify-items: center;
  gap: 8px;
  padding: 56px 20px;
  border-top: 1px solid var(--c-line);
  border-bottom: 1px solid var(--c-line);
  text-align: center;
}

.quiet-none {
  font-family: var(--font-display);
  font-size: 36px;
  font-weight: 450;
  color: var(--c-ink);
  line-height: 1;
}

.quiet-empty p {
  margin: 0 0 6px;
  font-size: 13px;
  color: var(--c-muted);
}

.container-list {
  display: grid;
  border-top: 1px solid var(--c-line);
}

.container-row {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) minmax(0, 0.9fr) auto;
  align-items: center;
  gap: 18px;
  padding: 18px 10px;
  margin: 0 -10px;
  border-bottom: 1px solid var(--c-line);
  transition: opacity 240ms var(--ease-out);
}

.container-row::after {
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

@media (hover: hover) and (pointer: fine) {
  .container-list:hover .container-row:not(:hover) {
    opacity: 0.48;
  }
  .container-row:hover::after {
    transform: scaleX(1);
  }
}

.container-main {
  display: grid;
  gap: 5px;
  min-width: 0;
}

.container-head {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.container-head h3 {
  margin: 0;
  font-size: 15.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.container-image {
  margin: 0;
  font-size: 12px;
  color: var(--c-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.container-stats {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.container-stats dt {
  font-family: var(--font-ui);
  font-size: 11px;
  color: var(--c-muted);
  margin-bottom: 2px;
}

.container-stats dd {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-ink);
}

.container-stats-empty {
  font-size: 12px;
  color: var(--c-muted);
}

.container-actions {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.confirm-text {
  margin: 0;
  font-size: 14px;
  color: var(--c-ink);
  line-height: 1.6;
}

.log-console {
  margin: 0;
  padding: 16px 18px;
  max-height: 58vh;
  overflow: auto;
  border-radius: 14px;
  background: #0b0b0e;
  color: #e6e6e3;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

@media (max-width: 860px) {
  .container-row {
    grid-template-columns: 1fr;
    gap: 12px;
    padding: 16px 0;
    margin: 0;
  }
  .container-row::after {
    left: 0;
    right: 0;
  }
}
</style>
