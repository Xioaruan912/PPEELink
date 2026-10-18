<template>
  <section class="group-board" aria-labelledby="group-board-title">
    <header class="board-head">
      <div>
        <h3 id="group-board-title">策略分组</h3>
        <p>
          把右侧策略拖进分组；拖动卡片调整顺序。分组按筛选正则自动纳入节点。
        </p>
      </div>
      <UiButton :icon="Plus" @click="addGroup()">添加分组</UiButton>
    </header>
    <div class="bulk-zones" aria-hidden="true">
      <div
        class="bulk-zone"
        :class="{ 'is-over': over === 'remove-all' }"
        @dragover.prevent="dragging?.kind === 'policy' && (over = 'remove-all')"
        @dragleave="over = ''"
        @drop.prevent="dropAll(false)"
      >
        从所有分组移除
      </div>
      <div
        class="bulk-zone"
        :class="{ 'is-over': over === 'add-all' }"
        @dragover.prevent="dragging?.kind === 'policy' && (over = 'add-all')"
        @dragleave="over = ''"
        @drop.prevent="dropAll(true)"
      >
        添加到所有分组
      </div>
    </div>
    <div class="board-layout">
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
              ><Rank
            /></span>
            <el-input
              v-model="group.name"
              class="card-name"
              :aria-label="`分组 ${index + 1} 名称`"
              placeholder="分组名称"
            />
            <el-dropdown
              trigger="click"
              @command="(cmd: string) => cardCommand(cmd, index)"
              ><RowIconButton
                :icon="MoreFilled"
                :label="`${group.name || '分组'} 更多操作`"
              /><template #dropdown
                ><el-dropdown-menu
                  ><el-dropdown-item command="up" :disabled="index === 0"
                    >上移</el-dropdown-item
                  ><el-dropdown-item
                    command="down"
                    :disabled="index === form.groups.length - 1"
                    >下移</el-dropdown-item
                  ><el-dropdown-item command="delete" divided
                    ><span class="danger-text">删除分组</span></el-dropdown-item
                  ></el-dropdown-menu
                ></template
              ></el-dropdown
            >
          </header>
          <div class="card-meta">
            <el-select
              v-model="group.type"
              class="card-type"
              :aria-label="`${group.name || '分组'} 类型`"
            >
              <el-option label="手动选择" value="select" />
              <el-option label="自动测速" value="url-test" />
              <el-option label="故障转移" value="fallback" />
            </el-select>
            <el-checkbox v-model="group.include_all_providers"
              >纳入全部节点</el-checkbox
            >
          </div>
          <el-input
            v-model="group.filter"
            class="card-filter"
            :aria-label="`${group.name || '分组'} 节点筛选正则`"
            placeholder="节点筛选正则，如 (?i)香港|HK"
          />
          <ul class="chip-list" :aria-label="`${group.name || '分组'} 策略`">
            <li v-for="(policy, pi) in group.proxies" :key="policy">
              <span
                class="chip"
                draggable="true"
                @dragstart="startPolicy($event, policy, index)"
                @dragend="endDrag"
                ><span class="chip-dot" :class="dotClass(policy)"></span
                >{{ policy
                }}<button
                  class="chip-remove"
                  :aria-label="`从 ${group.name} 移除 ${policy}`"
                  @click="group.proxies.splice(pi, 1)"
                >
                  <Close /></button
              ></span>
            </li>
            <li v-if="!group.proxies.length" class="chip-empty">
              拖入策略，或仅靠正则匹配节点
            </li>
          </ul>
          <el-select
            class="chip-add"
            :model-value="''"
            placeholder="添加策略"
            :aria-label="`为 ${group.name || '分组'} 添加策略`"
            @change="(value: string) => addPolicy(index, value)"
            ><el-option
              v-for="policy in policiesFor(index)"
              :key="policy"
              :label="policy"
              :value="policy"
          /></el-select>
        </article>
      </TransitionGroup>
      <aside class="policy-palette" aria-label="可用策略">
        <h4>可用策略</h4>
        <p>{{ palette.length }} 个</p>
        <ul>
          <li v-for="policy in palette" :key="policy">
            <span
              class="chip chip--palette"
              draggable="true"
              @dragstart="startPolicy($event, policy)"
              @dragend="endDrag"
              ><span class="chip-dot" :class="dotClass(policy)"></span
              >{{ policy }}</span
            >
          </li>
        </ul>
      </aside>
    </div>
  </section>
</template>
<script setup lang="ts">
import { computed, ref } from "vue";
import { Close, MoreFilled, Plus, Rank } from "@element-plus/icons-vue";
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
// Stable keys survive renames and reorders so cards animate instead of remounting.
const keys = new WeakMap<GroupItem, number>();
let nextKey = 0;
const keyOf = (group: GroupItem) => {
  if (!keys.has(group)) keys.set(group, nextKey++);
  return keys.get(group)!;
};
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
  // Dragging a chip between cards moves it rather than copying it.
  if (drag.from !== undefined) {
    const source = form.groups[drag.from];
    source.proxies = source.proxies.filter((p) => p !== drag.policy);
  }
}
function dropAll(add: boolean) {
  const drag = dragging.value;
  endDrag();
  if (drag?.kind !== "policy") return;
  form.groups.forEach((group, index) => {
    if (add) addPolicy(index, drag.policy);
    else group.proxies = group.proxies.filter((p) => p !== drag.policy);
  });
}
function cardCommand(command: string, index: number) {
  if (command === "delete") return removeGroup(index);
  const target = command === "up" ? index - 1 : index + 1;
  if (target < 0 || target >= form.groups.length) return;
  const [moved] = form.groups.splice(index, 1);
  form.groups.splice(target, 0, moved);
}
</script>
<style scoped>
.group-board {
  display: grid;
  gap: 12px;
  margin-bottom: 20px;
}
.board-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
h3 {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
}
.board-head p {
  color: var(--c-muted);
  font-size: 13px;
}
.bulk-zones {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}
.bulk-zone {
  display: grid;
  place-items: center;
  min-height: 44px;
  border: 1px dashed var(--c-line);
  border-radius: var(--radius-control);
  color: var(--c-muted);
  font-size: 13px;
  transition:
    border-color var(--duration-fast),
    background-color var(--duration-fast),
    color var(--duration-fast);
}
.bulk-zone.is-over {
  border-color: var(--c-focus);
  background: var(--c-selected);
  color: var(--c-ink);
}
.board-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 168px;
  gap: 12px;
  align-items: start;
}
.group-cards {
  position: relative;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
  gap: 10px;
}
.group-card {
  display: grid;
  align-content: start;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
  transition:
    border-color var(--duration-fast),
    box-shadow var(--duration-fast),
    opacity var(--duration-fast);
}
.group-card.is-over {
  border-color: var(--c-focus);
  box-shadow: 0 0 0 3px var(--c-selected);
}
.group-card.is-dragging {
  opacity: 0.45;
}
.card-head {
  display: flex;
  align-items: center;
  gap: 4px;
}
.drag-handle {
  display: grid;
  place-items: center;
  flex: 0 0 auto;
  width: 24px;
  height: 32px;
  color: var(--c-muted);
  cursor: grab;
}
.drag-handle:active {
  cursor: grabbing;
}
.drag-handle svg {
  width: 16px;
  height: 16px;
}
.card-name {
  flex: 1;
  min-width: 0;
}
.card-name :deep(.el-input__inner) {
  font-weight: 600;
}
.card-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px 10px;
}
.card-type {
  flex: 1 1 110px;
  min-width: 0;
}
.card-meta :deep(.el-checkbox) {
  margin: 0;
}
.chip-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  min-height: 34px;
  padding: 6px;
  border-radius: var(--radius-control);
  background: var(--c-canvas);
}
.chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 26px;
  padding: 0 4px 0 8px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  font-size: 13px;
  cursor: grab;
  user-select: none;
}
.chip--palette {
  padding-right: 8px;
}
.chip:hover {
  border-color: var(--c-muted);
}
.chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-data);
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
  border-radius: 4px;
  color: var(--c-muted);
}
.chip-remove:hover {
  background: var(--c-surface-2);
  color: var(--c-bad);
}
.chip-remove svg {
  width: 12px;
  height: 12px;
}
.chip-empty {
  align-self: center;
  color: var(--c-muted);
  font-size: 12px;
}
.chip-add {
  width: 100%;
}
.policy-palette {
  position: sticky;
  top: 84px;
  display: grid;
  gap: 6px;
  padding: 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-panel);
  background: var(--c-surface);
}
.policy-palette h4 {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
}
.policy-palette p {
  color: var(--c-muted);
  font-size: 12px;
}
.policy-palette ul {
  display: grid;
  gap: 6px;
  max-height: 50dvh;
  overflow: auto;
}
.policy-palette .chip {
  width: 100%;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.card-move,
.card-enter-active,
.card-leave-active {
  transition:
    transform var(--duration-slow) var(--ease-spring),
    opacity var(--duration-medium);
}
.card-enter-from,
.card-leave-to {
  opacity: 0;
  transform: scale(0.94);
}
.card-leave-active {
  position: absolute;
}
@media (max-width: 767px) {
  .board-layout {
    grid-template-columns: minmax(0, 1fr);
  }
  .bulk-zones,
  .policy-palette {
    display: none;
  }
}
</style>
