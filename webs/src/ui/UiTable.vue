<template>
  <section ref="root" class="ui-table" :aria-label="label" :aria-busy="loading">
    <UiSkeleton v-if="loading" :lines="4" /><template v-else
      ><div class="ui-table__desktop">
        <el-table v-bind="$attrs" :data="rows" :row-key="rowKey"
          ><slot></slot
          ><template #empty
            ><slot name="empty"><EmptyState /></slot></template
        ></el-table>
      </div>
      <div class="ui-table__mobile">
        <TransitionGroup name="table-card" tag="div"
          ><article
            v-for="(row, index) in rows"
            :key="getKey(row, index)"
            class="ui-table__card"
          >
            <slot name="mobile-card" :row="row" :index="index"
              ><dl>
                <template v-for="(value, key) in row" :key="key"
                  ><dt>{{ key }}</dt>
                  <dd>{{ value }}</dd></template
                >
              </dl></slot
            >
          </article></TransitionGroup
        ><slot v-if="!rows.length" name="empty"><EmptyState /></slot></div
    ></template>
  </section>
</template>
<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from "vue";
import { useMotion } from "@/motion";
import UiSkeleton from "./UiSkeleton.vue";
import EmptyState from "./EmptyState.vue";
defineOptions({ inheritAttrs: false });
const props = withDefaults(
  defineProps<{
    rows: Record<string, any>[];
    rowKey?: string;
    loading?: boolean;
    label?: string;
  }>(),
  { rowKey: "id", label: "数据列表" }
);
const root = ref<HTMLElement>();
const { animate, stop } = useMotion();
const getKey = (row: Record<string, any>, index: number) =>
  row[props.rowKey] ?? index;
async function reveal() {
  await nextTick();
  stop();
  const rows = root.value?.querySelectorAll(".el-table__row, .ui-table__card");
  if (rows?.length)
    animate(
      Array.from(rows).slice(0, 12),
      { y: 12, opacity: 0 },
      {
        y: 0,
        opacity: 1,
        duration: 0.32,
        stagger: 0.04,
        clearProps: "transform,opacity",
      }
    );
}
onMounted(reveal);
watch(() => [props.rows, props.loading], reveal);
</script>
<style scoped>
.ui-table {
  min-width: 0;
}
.ui-table__mobile {
  display: none;
}
.ui-table__card {
  padding: 24px 0;
  border-bottom: 1px solid var(--c-line);
  overflow-wrap: anywhere;
}
.ui-table__card dl {
  margin: 0;
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 2fr);
  gap: 8px;
}
.ui-table__card dt {
  color: var(--c-muted);
  font-size: 14px;
}
.ui-table__card dd {
  margin: 0;
}
.table-card-move,
.table-card-enter-active,
.table-card-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-spring),
    opacity var(--duration-medium);
}
.table-card-enter-from,
.table-card-leave-to {
  opacity: 0;
  transform: translateY(16px);
}
.table-card-leave-active {
  position: absolute;
}
@media (max-width: 767px) {
  .ui-table__desktop {
    display: none;
  }
  .ui-table__mobile {
    display: block;
  }
}
</style>
