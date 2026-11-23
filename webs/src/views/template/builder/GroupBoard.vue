<script setup lang="ts">
import { computed, ref } from "vue";
import { Close, Delete, Plus, Rank } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useMotion } from "@/motion";
import { useTemplateBuilder, type GroupItem } from "./useTemplateBuilder";

const { form, addGroup, removeGroup } = useTemplateBuilder();
const { reduced } = useMotion();

type Drag =
  | { kind: "group"; index: number }
  | { kind: "policy"; policy: string; from?: number };

const dragging = ref<Drag | null>(null);
const over = ref("");

const keys = new WeakMap<GroupItem, number>();
let nextKey = 0;
const keyOf = (group: GroupItem) => {
  if (!keys.has(group)) keys.set(group, nextKey++);
  return keys.get(group)!;
};

const groupTypes = [
  { label: "手动", value: "select" },
  { label: "测速", value: "url-test" },
  { label: "容灾", value: "fallback" },
];

const BUILTIN = ["DIRECT", "REJECT"];
const palette = computed(() => [
  ...BUILTIN,
  ...form.groups.map((g) => g.name).filter(Boolean),
]);

const policiesFor = (index: number) =>
  palette.value.filter(
    (p) =>
      p !== form.groups[index].name && !form.groups[index].proxies.includes(p)
  );

const dotClass = (policy: string) =>
  policy === "DIRECT" ? "is-direct" : policy === "REJECT" ? "is-reject" : "";

function addPolicy(index: number, policy: string) {
  const group = form.groups[index];
  if (policy && policy !== group.name && !group.proxies.includes(policy))
    group.proxies.push(policy);
}

function startGroup(event: DragEvent, index: number) {
  dragging.value = { kind: "group", index };
  event.dataTransfer?.setData("text/plain", form.groups[index].name);
  if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
}

function startPolicy(event: DragEvent, policy: string, from?: number) {
  dragging.value = { kind: "policy", policy, from };
  event.dataTransfer?.setData("text/plain", policy);
  if (event.dataTransfer)
    event.dataTransfer.effectAllowed = from === undefined ? "copy" : "move";
}

function endDrag() {
  dragging.value = null;
  over.value = "";
}

function onCardOver(index: number) {
  if (dragging.value) over.value = `group-${index}`;
}

function onCardDrop(index: number) {
  const drag = dragging.value;
  endDrag();
  if (!drag) return;
  if (drag.kind === "group") {
    if (drag.index === index) return;
    const [moved] = form.groups.splice(drag.index, 1);
    form.groups.splice(index, 0, moved);
    return;
  }
  if (drag.from === index) return;
  addPolicy(index, drag.policy);
  if (drag.from !== undefined) {
    const source = form.groups[drag.from];
    source.proxies = source.proxies.filter((p) => p !== drag.policy);
  }
}
</script>

<template>
  <section class="group-board" aria-label="策略分组编排">
    <header class="board-bar">
      <div class="palette-bar">
        <span class="bar-label">可用策略（可拖入卡片）</span>
        <span
          v-for="policy in palette"
          :key="policy"
          class="chip chip--palette"
          draggable="true"
          @dragstart="startPolicy($event, policy)"
          @dragend="endDrag"
        >
          <span class="chip-dot" :class="dotClass(policy)"></span>
          {{ policy }}
        </span>
      </div>

      <UiButton variant="ghost" :icon="Plus" @click="addGroup('新策略组')">
        添加分组
      </UiButton>
    </header>

    <TransitionGroup
      tag="div"
      class="group-cards"
      :name="reduced ? '' : 'card'"
    >
      <article
        v-for="(group, index) in form.groups"
        :key="keyOf(group)"
        class="group-card"
        :class="{
          'is-over': over === `group-${index}`,
          'is-dragging':
            dragging?.kind === 'group' && dragging.index === index,
        }"
        @dragover.prevent="onCardOver(index)"
        @dragleave="over = ''"
        @drop.prevent="onCardDrop(index)"
      >
        <header class="card-head">
          <span
            class="drag-handle"
            draggable="true"
            title="拖动排序"
            aria-hidden="true"
            @dragstart="startGroup($event, index)"
            @dragend="endDrag"
          >
            <Rank />
          </span>
          <input
            v-model="group.name"
            type="text"
            class="card-name-input"
            :aria-label="`分组 ${index + 1} 名称`"
            placeholder="分组名称"
          />
          <RowIconButton
            :icon="Delete"
            :label="`删除 ${group.name || '分组'}`"
            @click="removeGroup(index)"
          />
        </header>

        <!-- Direct 1-Click Mode Pills + All Nodes Toggle (Zero Dropdown) -->
        <div class="card-controls">
          <div class="mini-pills" role="group" aria-label="分组类型">
            <button
              v-for="t in groupTypes"
              :key="t.value"
              type="button"
              class="mini-pill"
              :class="{ 'is-active': group.type === t.value }"
              @click="group.type = t.value"
            >
              {{ t.label }}
            </button>
          </div>

          <button
            type="button"
            class="all-nodes-pill"
            :class="{ 'is-on': group.include_all_providers }"
            @click="group.include_all_providers = !group.include_all_providers"
          >
            全部节点
          </button>
        </div>

        <input
          v-model="group.filter"
          type="text"
          class="filter-input mono"
          :aria-label="`${group.name || '分组'} 节点筛选正则`"
          placeholder="正则筛选（如 (?i)香港|HK，留空不筛）"
        />

        <!-- Active Policies -->
        <div class="chip-box">
          <span
            v-for="(policy, pi) in group.proxies"
            :key="policy"
            class="chip"
            draggable="true"
            @dragstart="startPolicy($event, policy, index)"
            @dragend="endDrag"
          >
            <span class="chip-dot" :class="dotClass(policy)"></span>
            {{ policy }}
            <button
              type="button"
              class="chip-remove"
              :aria-label="`移除 ${policy}`"
              @click="group.proxies.splice(pi, 1)"
            >
              <Close />
            </button>
          </span>
          <span v-if="!group.proxies.length" class="chip-empty">
            仅按节点匹配或点击下方添加策略
          </span>
        </div>

        <!-- Direct 1-Click Add Policy Chips (Zero Dropdown!) -->
        <div v-if="policiesFor(index).length" class="quick-add-rail">
          <button
            v-for="p in policiesFor(index).slice(0, 5)"
            :key="p"
            type="button"
            class="add-chip"
            @click="addPolicy(index, p)"
          >
            + {{ p }}
          </button>
        </div>
      </article>
    </TransitionGroup>
  </section>
</template>

<style scoped>
.group-board {
  display: grid;
  gap: 16px;
}

.board-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.palette-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.bar-label {
  font-size: 12px;
  color: var(--c-muted);
  margin-right: 4px;
}

.group-cards {
  position: relative;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 12px;
}

.group-card {
  display: grid;
  align-content: start;
  gap: 10px;
  min-width: 0;
  box-sizing: border-box;
  overflow: hidden;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  transition:
    border-color 180ms var(--ease-out),
    box-shadow 180ms var(--ease-out),
    opacity 180ms var(--ease-out);
}

.group-card.is-over {
  border-color: var(--c-ok);
  box-shadow: 0 0 0 3px rgb(21 184 90 / 0.14);
}

.group-card.is-dragging {
  opacity: 0.45;
}

.card-head {
  display: flex;
  align-items: center;
  gap: 6px;
}

.drag-handle {
  display: grid;
  place-items: center;
  width: 22px;
  height: 28px;
  color: var(--c-muted);
  cursor: grab;
}

.drag-handle svg {
  width: 15px;
  height: 15px;
}

.card-name-input {
  flex: 1;
  min-width: 0;
  padding: 4px 6px;
  border: 0;
  border-bottom: 1px solid transparent;
  background: transparent;
  color: var(--c-ink);
  font-size: 14.5px;
  font-weight: 600;
  outline: none !important;
}

.card-name-input:focus {
  border-bottom-color: var(--c-ink);
}

.card-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  min-width: 0;
  gap: 8px;
}

.mini-pills {
  display: inline-flex;
  flex-shrink: 0;
  padding: 2px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-muted);
}

.mini-pill {
  padding: 3px 10px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11.5px;
  white-space: nowrap;
  cursor: pointer;
}

.mini-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.all-nodes-pill {
  flex-shrink: 0;
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11.5px;
  white-space: nowrap;
  cursor: pointer;
}

.all-nodes-pill.is-on {
  border-color: var(--c-ok);
  color: var(--c-ok);
  font-weight: 500;
}

.filter-input {
  width: 100%;
  max-width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 7px 10px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 11.5px;
  outline: none !important;
}

.filter-input:focus {
  border-color: var(--c-ink);
}

.chip-box {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-height: 36px;
  padding: 8px;
  border-radius: 12px;
  background: var(--c-surface-muted);
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 5px 0 9px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-ink);
  cursor: grab;
  user-select: none;
}

.chip--palette {
  padding-right: 10px;
}

.chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ink);
}

.chip-dot.is-direct {
  background: var(--c-ok);
}

.chip-dot.is-reject {
  background: var(--c-bad);
}

.chip-remove {
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border: 0;
  border-radius: 50%;
  background: transparent;
  color: var(--c-muted);
  cursor: pointer;
}

.chip-remove:hover {
  background: var(--c-surface-muted);
  color: var(--c-bad);
}

.chip-remove svg {
  width: 11px;
  height: 11px;
}

.chip-empty {
  align-self: center;
  font-size: 11.5px;
  color: var(--c-muted);
}

.quick-add-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.add-chip {
  padding: 3px 8px;
  border: 1px dashed var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11px;
  cursor: pointer;
}

.add-chip:hover {
  border-style: solid;
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.card-move,
.card-enter-active,
.card-leave-active {
  transition:
    transform 260ms cubic-bezier(0.16, 1, 0.3, 1),
    opacity 200ms var(--ease-out);
}

.card-enter-from,
.card-leave-to {
  opacity: 0;
  transform: scale(0.96);
}

.card-leave-active {
  position: absolute;
}

@media (max-width: 640px) {
  .group-cards {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
