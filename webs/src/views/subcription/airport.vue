<script setup lang="ts">
import { computed, nextTick, ref, watch } from "vue";
import { Plus, View, MoreFilled, Refresh } from "@element-plus/icons-vue";
import {
  PageHeader,
  UiButton,
  UiDrawer,
  UiDialog,
  UiPanel,
  UiStat,
  UiSkeleton,
  EmptyState,
  StatusDot,
} from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useMotion, useStagger } from "@/motion";
import DetailDrawer from "./airport/DetailDrawer.vue";
import { useAirports, displayTime, maskSource } from "./airport/useAirports";
import type { Airport } from "./airport/useAirports";
import TrafficMeter from "./airport/TrafficMeter.vue";
import { formatBytes, trafficOf } from "@/utils/traffic";
defineOptions({ name: "AirportPage" });
const {
  tableData,
  loading,
  loadError,
  search,
  filtered,
  nodeCount,
  loadData,
  syncStates,
  activeSyncCount,
  handleSync,
  editorVisible,
  saving,
  formError,
  form,
  handleAdd,
  handleEdit,
  submitForm,
  deleteTarget,
  deleting,
  deleteError,
  handleDel,
  confirmDelete,
  drawerVisible,
  drawerLoading,
  drawerDetail,
  detailError,
  detailTarget,
  openDetail,
  selectedNodeNames,
  savingNodes,
  nodeSaveError,
  rttMap,
  rttLoading,
  rttError,
  loadNodeRtt,
  toggleNode,
  allSelected,
  toggleAllNodes,
  saveNodeSelection,
} = useAirports();
const trafficSummary = computed(() => {
  const reports = tableData.value.map(trafficOf).filter((t) => !!t);
  const days = reports
    .map((t) => t!.daysLeft)
    .filter((d): d is number => d !== null && d >= 0);
  return {
    reported: reports.length > 0,
    remaining: reports.reduce((sum, t) => sum + t!.remaining, 0),
    nearestDays: days.length ? Math.min(...days) : null,
  };
});
// Split "12.3 GB" so UiStat can count up the number and print the unit.
const remainingValue = computed(() => {
  const [value, unit] = formatBytes(trafficSummary.value.remaining).split(" ");
  return { value: Number(value), unit };
});
const root = ref<HTMLElement>();
const { reduced } = useMotion();
const stagger = useStagger();
watch(
  loading,
  async (value) => {
    if (!value) {
      await nextTick();
      if (root.value) stagger(root.value.querySelectorAll(".airport-row"));
    }
  },
  { immediate: true }
);
function command(action: string, row: Airport) {
  if (action === "edit") handleEdit(row);
  else if (action === "delete") handleDel(row);
}
const status = (row: Airport) => {
  const sync = syncStates.value[row.ID];
  if (sync?.active) return { state: "warn" as const, text: "同步中" };
  if (sync?.failed) return { state: "bad" as const, text: "同步失败" };
  if (sync?.unknown) return { state: "warn" as const, text: "结果待确认" };
  return row.NodeCount
    ? { state: "ok" as const, text: "已有节点" }
    : { state: "idle" as const, text: "待导入节点" };
};
</script>
<template>
  <div class="airport-page">
    <PageHeader title="机场" description="管理订阅来源，测活并同步节点。">
      <template #actions
        ><UiButton variant="primary" :icon="Plus" @click="handleAdd"
          >添加机场</UiButton
        ></template
      >
    </PageHeader>
    <section :aria-busy="loading">
      <div class="stat-tiles" data-enter>
        <UiStat size="large" label="机场来源" :value="tableData.length" />
        <UiStat size="large" label="保留节点" :value="nodeCount" />
        <UiStat
          size="large"
          label="剩余流量"
          :value="trafficSummary.reported ? remainingValue.value : null"
          :decimals="1"
          :unit="remainingValue.unit"
        />
        <UiStat
          size="large"
          label="最近到期"
          :value="trafficSummary.nearestDays"
          unit="天"
          :state="
            trafficSummary.nearestDays === null
              ? 'default'
              : trafficSummary.nearestDays <= 7
                ? 'bad'
                : 'default'
          "
        />
      </div>
      <UiPanel
        title="机场列表"
        :count="filtered.length"
        description="同步时读取机场返回的流量与到期信息；正在同步的会显示进度。"
        flush
        data-enter
        ><template #actions
          ><el-input
            v-model="search"
            class="list-search"
            placeholder="搜索机场名称"
            aria-label="搜索机场名称"
            clearable
          /><UiButton :loading="loading" :icon="Refresh" @click="loadData"
            >刷新</UiButton
          ></template
        >
        <EmptyState
          v-if="loadError"
          size="section"
          title="无法加载机场"
          :description="loadError"
          ><template #action
            ><UiButton @click="loadData">重新加载</UiButton></template
          ></EmptyState
        >
        <UiSkeleton v-else-if="loading" :lines="4" height="80px" />
        <div v-else ref="root">
          <EmptyState
            v-if="!tableData.length"
            size="page"
            title="接入机场来源"
            description="添加机场订阅，再同步可用节点。"
            ><template #action
              ><UiButton @click="handleAdd">添加首个机场</UiButton></template
            ></EmptyState
          >
          <EmptyState
            v-else-if="!filtered.length"
            size="section"
            title="没有匹配机场"
            description="试试其他名称。"
            ><template #action
              ><UiButton @click="search = ''">清除搜索</UiButton></template
            ></EmptyState
          >
          <div v-if="filtered.length" class="list-heading" aria-hidden="true">
            <span>名称 / 来源</span><span>节点数</span><span>流量 / 到期</span
            ><span>状态</span><span class="actions-heading">操作</span>
          </div>
          <TransitionGroup
            tag="div"
            class="airport-list"
            :name="reduced ? '' : 'airport'"
          >
            <article
              v-for="row in filtered"
              :key="row.ID"
              class="airport-row"
              :class="{ 'is-syncing': syncStates[row.ID]?.active }"
            >
              <button class="name-button" @click="openDetail(row)">
                <span class="airport-name" :title="row.Name">{{
                  row.Name
                }}</span
                ><span class="source-url">{{ maskSource(row.URL) }}</span
                ><span class="source-url"
                  >同步于 {{ displayTime(row.LastSync) }}</span
                >
              </button>
              <div class="row-value">
                <span class="mobile-label">节点数</span
                ><span class="mono">{{ row.NodeCount || 0 }}</span>
              </div>
              <div class="row-value traffic-cell">
                <span class="mobile-label">流量 / 到期</span>
                <TrafficMeter :traffic="trafficOf(row)" />
              </div>
              <div class="row-value">
                <span class="mobile-label">状态</span
                ><StatusDot
                  :state="status(row).state"
                  :label="status(row).text"
                  :running="syncStates[row.ID]?.active"
                />
              </div>
              <div class="row-actions">
                <UiButton
                  :loading="syncStates[row.ID]?.active"
                  :aria-label="`测活并同步 ${row.Name}`"
                  @click="handleSync(row)"
                  >{{
                    syncStates[row.ID]?.active ? "同步中" : "测活同步"
                  }}</UiButton
                >
                <RowIconButton
                  :icon="View"
                  :label="`查看 ${row.Name}`"
                  @click="openDetail(row)"
                />
                <el-dropdown trigger="click" @command="command($event, row)"
                  ><RowIconButton
                    :icon="MoreFilled"
                    :label="`${row.Name} 的更多操作`"
                    aria-haspopup="menu"
                  /><template #dropdown
                    ><el-dropdown-menu
                      ><el-dropdown-item command="edit"
                        >编辑机场</el-dropdown-item
                      ><el-dropdown-item
                        command="delete"
                        :disabled="syncStates[row.ID]?.active"
                        ><span class="danger-text"
                          >删除机场</span
                        ></el-dropdown-item
                      ></el-dropdown-menu
                    ></template
                  ></el-dropdown
                >
              </div>
              <div
                v-if="syncStates[row.ID]"
                class="sync-feedback"
                role="status"
              >
                <span :class="{ 'danger-text': syncStates[row.ID].failed }">{{
                  syncStates[row.ID].message
                }}</span>
                <template
                  v-if="
                    syncStates[row.ID].active &&
                    syncStates[row.ID].progress !== undefined
                  "
                  ><span class="mono">{{ syncStates[row.ID].progress }}%</span
                  ><progress
                    :value="syncStates[row.ID].progress"
                    max="100"
                    :aria-label="`${row.Name} 同步任务进度`"
                  ></progress
                ></template>
              </div>
            </article>
          </TransitionGroup>
        </div>
      </UiPanel>
    </section>
    <UiDrawer
      v-model="editorVisible"
      :title="form.ID ? '编辑机场' : '添加机场'"
      size="min(600px, 100vw)"
      :close-on-click-modal="!saving"
      :close-on-press-escape="!saving"
      :show-close="!saving"
    >
      <el-form
        class="airport-form"
        label-position="top"
        @submit.prevent="submitForm"
      >
        <el-form-item label="机场名称" required
          ><el-input
            v-model="form.Name"
            placeholder="填写机场名称"
            :disabled="saving"
          />
          <p class="field-hint">同步后，节点归入该名称的分组。</p></el-form-item
        >
        <el-form-item label="机场订阅链接" required
          ><el-input
            v-model="form.URL"
            type="password"
            show-password
            placeholder="https://…"
            autocomplete="off"
            :disabled="saving"
        /></el-form-item>
        <div class="switch-field">
          <div>
            <label for="airport-cleanup">自动清理死节点</label>
            <p class="field-hint">同步时进行 TCP 测活，丢弃无法连接的节点。</p>
          </div>
          <el-switch
            id="airport-cleanup"
            v-model="form.AutoCleanup"
            :disabled="saving"
            aria-label="自动清理死节点"
          />
        </div>
        <p v-if="formError" class="error-text" role="alert">{{ formError }}</p>
      </el-form>
      <template #footer
        ><UiButton :disabled="saving" @click="editorVisible = false"
          >取消</UiButton
        ><UiButton variant="primary" :loading="saving" @click="submitForm">{{
          form.ID ? "保存修改" : "添加机场"
        }}</UiButton></template
      >
    </UiDrawer>
    <DetailDrawer
      v-model="drawerVisible"
      :detail="drawerDetail"
      :loading="drawerLoading"
      :error="detailError"
      :selected="selectedNodeNames"
      :saving="savingNodes"
      :save-error="nodeSaveError"
      :all-selected="allSelected"
      :rtt="rttMap"
      :rtt-loading="rttLoading"
      :rtt-error="rttError"
      @retry="detailTarget && openDetail({ ID: detailTarget })"
      @toggle="toggleNode"
      @toggle-all="toggleAllNodes"
      @save="saveNodeSelection"
      @test-latency="loadNodeRtt"
    />
    <UiDialog
      :model-value="!!deleteTarget"
      title="删除机场"
      width="min(560px, calc(100vw - 32px))"
      :close-on-click-modal="!deleting"
      :close-on-press-escape="!deleting"
      :show-close="!deleting"
      @update:model-value="!$event && (deleteTarget = undefined)"
    >
      <p class="delete-copy">
        删除「{{ deleteTarget?.Name }}」后，将解除订阅对该机场分组的引用。
      </p>
      <p class="field-hint">选择保留节点，或同时删除机场所属节点。</p>
      <p v-if="deleteError" class="error-text" role="alert">
        {{ deleteError }}
      </p>
      <template #footer
        ><UiButton :disabled="deleting" @click="deleteTarget = undefined"
          >取消</UiButton
        ><UiButton :loading="deleting" @click="confirmDelete(false)"
          >仅删除机场</UiButton
        ><UiButton
          variant="danger"
          :loading="deleting"
          @click="confirmDelete(true)"
          >删除机场和节点</UiButton
        ></template
      >
    </UiDialog>
  </div>
</template>
<style scoped src="./airport/airport.scss"></style>
