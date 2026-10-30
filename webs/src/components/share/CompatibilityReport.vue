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
  {
    immediate: true,
  }
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
        <h3>客户端兼容</h3>
        <p>检查节点参数能否完整导出；实际连接可使用节点一键诊断。</p>
      </div>
      <UiButton size="sm" :loading="loading" @click="load">重新检查</UiButton>
    </div>
    <UiSkeleton v-if="loading" :lines="3" />
    <EmptyState v-else-if="error" size="inline" :description="error" />
    <template v-else-if="current">
      <p v-if="limited" class="compat-empty">
        链接较多，本次仅检查前 10000 条。请分批选择节点检查剩余链接。
      </p>
      <el-select
        v-if="!clientKey"
        v-model="selected"
        aria-label="检查的客户端"
        class="compat-select"
        ><el-option
          v-for="report in reports"
          :key="report.key"
          :label="report.name"
          :value="report.key"
      /></el-select>
      <div class="compat-summary" role="status">
        <strong>{{ current.included }} / {{ current.total }} 可导出</strong
        ><span>{{ current.warnings }} 个需注意</span
        ><span :class="{ excluded: current.excluded > 0 }"
          >{{ current.excluded }} 个已排除</span
        >
      </div>
      <p class="compat-note">{{ current.note }}</p>
      <p v-if="!current.included && current.total" class="compat-empty">
        当前没有可导出的节点，此客户端的订阅更新会被拒绝。可查看下方原因并选择其他客户端。
      </p>
      <p v-if="!current.total" class="compat-note">
        当前没有可检查的节点。订阅已暂停或关闭下发的节点不会计入。
      </p>
      <ul v-if="issues.length" class="compat-items">
        <li v-for="(item, index) in issues" :key="`${item.id}-${index}`">
          <div>
            <strong>{{ item.name }}</strong
            ><span class="compat-state" :data-state="item.status">{{
              item.status === "unsupported" ? "已排除" : "需注意"
            }}</span>
          </div>
          <small>{{ item.protocol || "未知协议" }}</small>
          <p>{{ item.reason }}</p>
        </li>
      </ul>
      <details v-if="supported.length" class="compat-supported">
        <summary>{{ supported.length }} 个节点参数可完整导出</summary>
        <ul class="compat-items">
          <li v-for="(item, index) in supported" :key="`${item.id}-${index}`">
            <div>
              <strong>{{ item.name }}</strong
              ><span>{{ item.protocol }}</span>
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
  gap: 14px;
  padding: 18px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.compat-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}
.compat-heading h3 {
  margin: 0 0 6px;
  font: 600 16px/1.4 var(--font-ui);
}
.compat-heading p,
.compat-note,
.compat-items p {
  margin: 0;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.compat-select {
  width: 100%;
}
.compat-summary {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 18px;
  font-size: 13px;
  padding-block: 12px;
  border-block: 1px solid var(--c-line);
}
.compat-summary strong {
  font-size: 15px;
  color: var(--c-signal-text);
}
.excluded,
.compat-empty {
  color: var(--el-color-danger);
}
.compat-empty {
  margin: 0;
  font-size: 13px;
  line-height: 1.6;
}
.compat-items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
}
.compat-items li {
  min-width: 0;
  padding: 12px 0;
  border-bottom: 1px solid var(--c-line);
}
.compat-items li:last-child {
  border-bottom: 0;
  padding-bottom: 0;
}
.compat-items li > div {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
}
.compat-items strong {
  min-width: 0;
  overflow-wrap: anywhere;
  font-size: 14px;
  font-weight: 550;
}
.compat-items small {
  display: block;
  color: var(--c-muted);
  font-size: 12px;
  margin: 3px 0;
}
.compat-items li > div > span {
  flex-shrink: 0;
  color: var(--c-muted);
  font-size: 12px;
}
.compat-state[data-state="unsupported"] {
  color: var(--el-color-danger);
}
.compat-state[data-state="warning"] {
  color: var(--el-color-warning);
}
.compat-supported summary {
  cursor: pointer;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
  padding-block: 6px;
}
.compat-supported summary:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 3px;
}
@media (max-width: 599px) {
  .compatibility {
    padding: 14px;
  }
  .compat-heading {
    flex-wrap: wrap;
  }
  .compat-heading > div {
    flex: 1 1 180px;
  }
  .compat-summary {
    gap: 8px 12px;
  }
}
</style>
