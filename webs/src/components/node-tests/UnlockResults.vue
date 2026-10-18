<template>
  <p v-if="result?.error" class="result-error" role="alert">
    {{ result.error }}
  </p>
  <section v-for="group in visibleGroups" :key="group.key" class="test-section">
    <div class="test-section-heading">
      <h3>{{ group.label }}</h3>
      <span>{{ resultByGroup(group.key).length }} 项服务</span>
    </div>
    <TransitionGroup
      tag="div"
      class="unlock-results"
      name="result-list"
      appear
      @enter="enter"
      @leave="leave"
      ><article
        v-for="(item, index) in resultByGroup(group.key)"
        :key="item.key"
        :data-index="index"
        class="unlock-item"
      >
        <div class="unlock-item-heading">
          <strong>{{ item.name }}</strong
          ><StatusDot
            :state="item.ok ? 'ok' : 'bad'"
            :label="item.ok ? '已解锁' : '未解锁'"
          />
        </div>
        <div class="unlock-meta">
          <span v-if="item.rtt > 0" class="test-mono">{{ item.rtt }} ms</span
          ><span v-if="item.region">{{ item.region }}</span
          ><span v-if="item.status">{{ item.status }}</span>
        </div>
        <p v-if="item.note" class="item-note">{{ item.note }}</p>
      </article></TransitionGroup
    >
  </section>
</template>
<script setup lang="ts">
import { computed } from "vue";
import StatusDot from "@/ui/StatusDot.vue";
import { useResultMotion } from "./useNodeTestSession";
import type { UnlockResult } from "./unlock";
const props = defineProps<{ result: UnlockResult }>();
const { enter, leave } = useResultMotion();
const groups = [
  { key: "ai", label: "AI 服务" },
  { key: "video", label: "影视流媒体" },
  { key: "forum", label: "论坛 / 其它" },
];
const visibleGroups = computed(() => {
  const extra = [
    ...new Set((props.result?.results || []).map((item) => item.group)),
  ]
    .filter((key) => !groups.some((group) => group.key === key))
    .map((key) => ({ key, label: key || "其它服务" }));
  return [...groups, ...extra].filter(
    (group) => resultByGroup(group.key).length
  );
});
const resultByGroup = (group: string) =>
  (props.result?.results || []).filter((item) => item.group === group);
</script>
<style scoped lang="scss">
@use "./dialog.scss";
.result-error {
  padding: 12px;
  border-left: 3px solid var(--c-bad);
  color: var(--c-bad);
  font-size: 14px;
  overflow-wrap: anywhere;
}
.unlock-results {
  position: relative;
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(0, 1fr);
  gap: 12px 16px;
}
.unlock-item {
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}
.unlock-item-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}
.unlock-item-heading strong {
  font-size: 14px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.unlock-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  margin-top: 12px;
  color: var(--c-muted);
  font-size: 14px;
}
.item-note {
  margin: 8px 0 0;
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
@media (max-width: 767px) {
  .unlock-results {
    grid-template-columns: 1fr;
  }
  .unlock-item {
    padding: 12px;
  }
}
</style>
