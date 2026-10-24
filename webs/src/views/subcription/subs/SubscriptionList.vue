<script setup lang="ts">
import { ref } from "vue";
import {
  UiButton,
  UiPanel,
  UiSegmented,
  UiSkeleton,
  EmptyState,
  StatusDot,
} from "@/ui";
import { useMotion } from "@/motion";
import { useSubscriptions } from "./useSubscriptions";
import { Grid, Share, MoreFilled } from "@element-plus/icons-vue";
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
    <div class="subs-summary" data-enter>
      <div>
        <span>订阅总数</span>
        <strong>{{ loading || loadError ? "—" : tableData.length }}</strong>
      </div>
      <div>
        <span>有效</span>
        <strong class="ok">{{
          loading || loadError ? "—" : tableData.length - expiredCount
        }}</strong>
      </div>
      <div>
        <span>已过期</span>
        <strong :class="{ bad: expiredCount }">{{
          loading || loadError ? "—" : expiredCount
        }}</strong>
      </div>
    </div>
    <UiPanel
      title="订阅列表"
      :count="filteredSubs.length"
      description="先选择客户端，再复制链接或扫码导入。"
      flush
      data-enter
      ><template #actions
        ><el-input
          v-model="searchKey"
          class="list-search"
          placeholder="搜索订阅名称"
          aria-label="搜索订阅名称"
          clearable
        /><UiSegmented
          v-model="statusFilter"
          :options="filters"
          label="订阅状态筛选"
        /><UiButton :loading="loading" @click="getsubs"
          >刷新</UiButton
        ></template
      >
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
          size="section"
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
        <TransitionGroup
          tag="div"
          :name="reduced ? '' : 'subscription'"
          class="subscription-list"
        >
          <article
            v-for="sub in filteredSubs"
            :key="sub.ID"
            class="subscription-row"
            :aria-label="sub.Name"
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
              <span class="mobile-label">包含节点</span
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
            <p class="source-summary">
              {{
                sub.GroupRefs?.length
                  ? `${sub.GroupRefs.length} 个动态分组`
                  : "固定节点来源"
              }}<span v-if="sub.GroupRefs?.length && sub.Nodes.length">
                · {{ sub.Nodes.length }} 个固定节点</span
              >
            </p>
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
                      ><span class="danger-text"
                        >删除订阅</span
                      ></el-dropdown-item
                    >
                  </el-dropdown-menu></template
                ></el-dropdown
              >
            </div>
          </article>
        </TransitionGroup>
      </div>
    </UiPanel>
  </section>
</template>
<style scoped src="./lists.scss"></style>
