<script setup lang="ts">
import { ref, watch, nextTick } from "vue";
import {
  UiButton,
  UiSegmented,
  UiStat,
  UiSkeleton,
  EmptyState,
  StatusDot,
} from "@/ui";
import { useMotion, useStagger } from "@/motion";
import { useSubscriptions } from "./useSubscriptions";
import { Grid, Share, View, MoreFilled } from "@element-plus/icons-vue";
import CopyLinkButton from "./CopyLinkButton.vue";
import RowIconButton from "@/ui/RowIconButton.vue";
const {
  tableData,
  filteredSubs,
  loading,
  loadError,
  getsubs,
  searchKey,
  statusFilter,
  expiredCount,
  activeClient,
  clientOptions,
  currentUrl,
  handleQrcode,
  openImport,
  openDrawer,
  onCardCommand,
  fmtTime,
  handleAddSub,
  nodeCounts,
} = useSubscriptions();
const root = ref<HTMLElement>();
const { reduced } = useMotion();
const stagger = useStagger();
watch(
  () => loading.value,
  async (value) => {
    if (!value) {
      await nextTick();
      if (root.value) stagger(root.value.querySelectorAll(".subscription-row"));
    }
  },
  { immediate: true }
);
const filters = [
  { label: "全部", value: "all" },
  { label: "有效", value: "active" },
  { label: "已过期", value: "expired" },
];
const expired = (value: string | null) =>
  !!value && new Date(value).getTime() <= Date.now();
</script>
<template>
  <section :aria-busy="loading">
    <div class="subscription-summary" data-enter>
      <UiStat size="large" label="订阅总数" :value="tableData.length" />
      <div class="summary-secondary">
        <UiStat
          size="small"
          label="有效订阅"
          :value="tableData.length - expiredCount"
        /><UiStat size="small" label="已过期" :value="expiredCount" />
      </div>
    </div>
    <div class="list-toolbar" data-enter>
      <el-input
        v-model="searchKey"
        placeholder="搜索订阅名称"
        aria-label="搜索订阅名称"
        clearable
      />
      <UiSegmented
        v-model="statusFilter"
        :options="filters"
        label="订阅状态筛选"
      />
      <UiButton :loading="loading" @click="getsubs">刷新</UiButton>
    </div>
    <EmptyState
      v-if="loadError"
      size="section"
      title="无法加载订阅"
      :description="loadError"
      ><template #action
        ><UiButton @click="getsubs">重新加载</UiButton></template
      ></EmptyState
    >
    <UiSkeleton v-else-if="loading" :lines="4" height="80px" />
    <div v-else ref="root">
      <EmptyState
        v-if="!tableData.length"
        size="page"
        title="连接你的设备"
        description="新建订阅，选择节点并生成配置。"
        ><template #action
          ><UiButton @click="handleAddSub">创建第一份订阅</UiButton></template
        ></EmptyState
      >
      <EmptyState
        v-else-if="!filteredSubs.length"
        size="section"
        title="没有匹配订阅"
        description="试试其他名称或状态。"
        ><template #action
          ><UiButton
            @click="
              searchKey = '';
              statusFilter = 'all';
            "
            >清除筛选</UiButton
          ></template
        ></EmptyState
      >
      <div v-if="filteredSubs.length" class="list-heading" aria-hidden="true">
        <span>名称 / 状态</span><span>节点数</span><span>过期时间</span
        ><span class="actions-heading">操作</span>
      </div>
      <TransitionGroup
        tag="div"
        :name="reduced ? '' : 'subscription'"
        class="subscription-list"
      >
        <article
          v-for="sub in filteredSubs"
          :key="sub.ID"
          class="subscription-row"
        >
          <button
            class="name-button"
            :title="sub.Name"
            @click="openDrawer(sub)"
          >
            <span class="name-text">{{ sub.Name }}</span>
            <span class="name-meta"
              ><StatusDot
                :state="expired(sub.ExpiresAt) ? 'bad' : 'ok'"
                :label="expired(sub.ExpiresAt) ? '已过期' : '有效'"
            /></span>
          </button>
          <div class="row-value">
            <span class="mobile-label">节点数</span
            ><span class="mono">{{
              nodeCounts[sub.ID] ?? `${sub.Nodes.length} 固定`
            }}</span>
          </div>
          <div class="row-value">
            <span class="mobile-label">过期时间</span
            ><time class="mono" :datetime="sub.ExpiresAt || undefined">{{
              sub.ExpiresAt ? fmtTime(sub.ExpiresAt) : "永不过期"
            }}</time>
          </div>
          <div class="row-actions">
            <CopyLinkButton
              :model-value="activeClient[sub.ID] || 'auto'"
              :value="currentUrl(sub)"
              :options="clientOptions"
              :label="sub.Name"
              @update:model-value="activeClient[sub.ID] = $event"
            />
            <RowIconButton
              :icon="Grid"
              :label="`${sub.Name} 二维码`"
              @click="handleQrcode(currentUrl(sub), sub.Name)"
            />
            <RowIconButton
              :icon="Share"
              :label="`分享 ${sub.Name}`"
              @click="openImport(sub)"
            />
            <RowIconButton
              :icon="View"
              :label="`${sub.Name} 详情`"
              @click="openDrawer(sub)"
            />
            <el-dropdown
              trigger="click"
              @command="(command: string) => onCardCommand(command, sub)"
              ><RowIconButton
                :icon="MoreFilled"
                :label="`${sub.Name} 更多操作`"
              /><template #dropdown
                ><el-dropdown-menu>
                  <el-dropdown-item command="edit">编辑订阅</el-dropdown-item
                  ><el-dropdown-item command="reset" divided
                    >重置订阅链接</el-dropdown-item
                  ><el-dropdown-item command="delete"
                    ><span class="danger-text">删除订阅</span></el-dropdown-item
                  >
                </el-dropdown-menu></template
              ></el-dropdown
            >
          </div>
        </article>
      </TransitionGroup>
    </div>
  </section>
</template>
<style scoped src="./lists.scss"></style>
