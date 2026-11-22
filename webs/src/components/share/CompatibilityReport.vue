<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import request from "@/utils/request";
import { UiButton, UiSkeleton, EmptyState } from "@/ui";
import type {
  ClientCompatibility as Report,
  CompatibilityData,
} from "./compatibility";

const props = defineProps<{
  subscriptionId?: number;
  nodeIds?: number[];
  clientKey?: string;
  preloaded?: CompatibilityData;
}>();

const reports = ref<Report[]>([]);
const selected = ref("clash");
const loading = ref(false);
const limited = ref(false);
const error = ref("");
let generation = 0;
let alive = true;

onBeforeUnmount(() => {
  alive = false;
  generation++;
});

async function load() {
  const current = ++generation;
  loading.value = true;
  error.value = "";
  try {
    if (props.preloaded) {
      reports.value = props.preloaded.clients;
      limited.value = !!props.preloaded.limited;
      return;
    }
    const { data } = await request(
      props.subscriptionId
        ? {
            url: "/api/v1/subcription/compatibility",
            method: "get",
            params: { id: props.subscriptionId },
          }
        : {
            url: "/api/v1/nodes/compatibility",
            method: "post",
            data: { ids: props.nodeIds ?? [] },
          }
    );
    if (alive && current === generation) {
      reports.value = data.clients;
      limited.value = !!data.limited;
      if (
        reports.value.length &&
        !reports.value.some((r) => r.key === selected.value)
      ) {
        selected.value = reports.value[0].key;
      }
    }
  } catch {
    if (alive && current === generation)
      error.value = "兼容信息加载失败，请重新加载。";
  } finally {
    if (alive && current === generation) loading.value = false;
  }
}

watch(
  () => [
    props.subscriptionId,
    JSON.stringify(props.nodeIds ?? []),
    props.preloaded,
  ],
  load,
  { immediate: true }
);

const current = computed(() =>
  reports.value.find((r) => r.key === (props.clientKey || selected.value))
);
const issues = computed(() =>
  [...(current.value?.items ?? [])]
    .filter((i) => i.status !== "supported")
    .sort(
      (a, b) =>
        Number(b.status === "unsupported") - Number(a.status === "unsupported")
    )
);
const supported = computed(() =>
  (current.value?.items ?? []).filter((i) => i.status === "supported")
);
</script>

<template>
  <section
    class="compatibility"
    aria-label="客户端兼容检查"
    :aria-busy="loading"
  >
    <div class="compat-heading">
      <div>
        <h3>客户端导出兼容性</h3>
        <p>检查节点协议与参数能否完整导出；实际连通性请使用节点一键诊断。</p>
      </div>
      <UiButton size="sm" variant="ghost" :loading="loading" @click="load">
        重新检查
      </UiButton>
    </div>

    <UiSkeleton v-if="loading" :lines="4" height="44px" />
    <EmptyState v-else-if="error" size="inline" :description="error" />

    <template v-else-if="current">
      <p v-if="limited" class="compat-alert">
        链接较多，本次仅检查前 10000 条。请分批选择节点检查剩余链接。
      </p>

      <!-- Direct Client Pill Switcher -->
      <div
        v-if="!clientKey && reports.length"
        class="client-pills"
        role="tablist"
        aria-label="检查的客户端"
      >
        <button
          v-for="report in reports"
          :key="report.key"
          type="button"
          role="tab"
          class="client-pill"
          :class="{ 'is-active': selected === report.key }"
          :aria-selected="selected === report.key"
          @click="selected = report.key"
        >
          <span>{{ report.name }}</span>
          <small v-if="report.excluded > 0" class="pill-badge is-bad mono">
            {{ report.excluded }}
          </small>
          <small v-else-if="report.warnings > 0" class="pill-badge is-warn mono">
            {{ report.warnings }}
          </small>
        </button>
      </div>

      <!-- Hairline Metric Ruler -->
      <dl class="compat-ruler" role="status">
        <div>
          <dt>可导出</dt>
          <dd class="mono is-ok">
            {{ current.included }} <small>/ {{ current.total }}</small>
          </dd>
        </div>
        <div>
          <dt>需注意</dt>
          <dd class="mono" :class="{ 'is-warn': current.warnings > 0 }">
            {{ current.warnings }}
          </dd>
        </div>
        <div>
          <dt>已排除</dt>
          <dd class="mono" :class="{ 'is-bad': current.excluded > 0 }">
            {{ current.excluded }}
          </dd>
        </div>
      </dl>

      <p v-if="current.note" class="compat-note">{{ current.note }}</p>
      <p v-if="!current.included && current.total" class="compat-alert">
        当前没有可导出的节点，此客户端的订阅更新会被拒绝。可查看下方原因并切换其他客户端。
      </p>
      <p v-if="!current.total" class="compat-note">
        当前没有可检查的节点。已暂停或关闭下发的节点不会计入。
      </p>

      <ul v-if="issues.length" class="compat-items">
        <li v-for="(item, index) in issues" :key="`${item.id}-${index}`">
          <div class="item-top">
            <strong>{{ item.name }}</strong>
            <span class="compat-state" :data-state="item.status">
              {{ item.status === "unsupported" ? "已排除" : "需注意" }}
            </span>
          </div>
          <small class="mono">{{ item.protocol || "未知协议" }}</small>
          <p>{{ item.reason }}</p>
        </li>
      </ul>

      <details v-if="supported.length" class="compat-supported">
        <summary>
          <span>{{ supported.length }} 个节点参数可完整导出</span>
        </summary>
        <ul class="compat-items">
          <li v-for="(item, index) in supported" :key="`${item.id}-${index}`">
            <div class="item-top">
              <strong>{{ item.name }}</strong>
              <span class="proto-tag mono">{{ item.protocol }}</span>
            </div>
            <p>{{ item.reason }}</p>
          </li>
        </ul>
      </details>
    </template>
  </section>
</template>

<style scoped>
.compatibility {
  min-width: 0;
  display: grid;
  gap: 18px;
}

.compat-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.compat-heading h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 500;
  letter-spacing: -0.02em;
  color: var(--c-ink);
}

.compat-heading p {
  margin: 4px 0 0;
  font-size: 12.5px;
  color: var(--c-muted);
}

.client-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.client-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 13px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 13px;
  cursor: pointer;
  transition:
    background var(--duration-fast) var(--ease-out),
    color var(--duration-fast) var(--ease-out),
    border-color var(--duration-fast) var(--ease-out);
}

.client-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-ink);
}

.client-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.pill-badge {
  padding: 1px 6px;
  border-radius: 999px;
  font-size: 11px;
  line-height: 1.3;
}

.pill-badge.is-bad {
  background: rgb(223 41 41 / 0.14);
  color: var(--c-bad);
}

.pill-badge.is-warn {
  background: rgb(180 83 9 / 0.14);
  color: var(--c-warn);
}

.client-pill.is-active .pill-badge {
  background: rgb(255 255 255 / 0.18);
  color: var(--c-surface);
}

.compat-ruler {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  margin: 0;
  border-top: 1px solid var(--c-ink);
  border-bottom: 1px solid var(--c-line);
}

.compat-ruler > div {
  display: grid;
  gap: 4px;
  padding: 14px 16px 14px 0;
  border-right: 1px solid var(--c-line);
}

.compat-ruler > div:not(:first-child) {
  padding-left: 16px;
}

.compat-ruler > div:last-child {
  border-right: 0;
  padding-right: 0;
}

.compat-ruler dt {
  font-size: 11.5px;
  color: var(--c-muted);
}

.compat-ruler dd {
  margin: 0;
  font-size: 22px;
  font-weight: 500;
  color: var(--c-ink);
}

.compat-ruler dd small {
  font-size: 13px;
  color: var(--c-muted);
  font-weight: 400;
}

.compat-ruler dd.is-ok {
  color: var(--c-ok);
}

.compat-ruler dd.is-warn {
  color: var(--c-warn);
}

.compat-ruler dd.is-bad {
  color: var(--c-bad);
}

.compat-note {
  margin: 0;
  font-size: 13px;
  color: var(--c-muted);
  line-height: 1.6;
}

.compat-alert {
  margin: 0;
  padding: 12px 14px;
  border: 1px solid rgb(180 83 9 / 0.28);
  border-radius: 12px;
  background: rgb(180 83 9 / 0.06);
  color: var(--c-warn);
  font-size: 13px;
  line-height: 1.55;
}

.compat-items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
}

.compat-items li {
  display: grid;
  gap: 4px;
  padding: 14px 0;
  border-bottom: 1px solid var(--c-line);
}

.item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.item-top strong {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.compat-items small {
  font-size: 11.5px;
  color: var(--c-muted);
}

.compat-items p {
  margin: 2px 0 0;
  font-size: 12.5px;
  color: var(--c-muted);
  line-height: 1.55;
}

.compat-state {
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 500;
}

.compat-state[data-state="unsupported"] {
  background: rgb(223 41 41 / 0.1);
  color: var(--c-bad);
}

.compat-state[data-state="warning"] {
  background: rgb(180 83 9 / 0.1);
  color: var(--c-warn);
}

.proto-tag {
  font-size: 11.5px;
  color: var(--c-muted);
}

.compat-supported summary {
  cursor: pointer;
  padding: 10px 0;
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
  border-bottom: 1px solid var(--c-line);
}
</style>
