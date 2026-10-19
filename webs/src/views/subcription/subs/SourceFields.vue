<script setup lang="ts">
import { UiButton, EmptyState } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
import { VueDraggable } from "vue-draggable-plus";
import { usePrefersReducedMotion } from "@/motion";
const reduced = usePrefersReducedMotion();
const {
  sourceUrl,
  isSourceMode,
  NodesList,
  value1,
  allGroups,
  selectedGroups,
  openedNodeGroups,
  nodeQuality,
  nodeQualityByName,
  nodeQualityLabel,
  validLabel,
  groupedNodes,
  removeNode,
  selectAllNodes,
  selectGroupNodes,
  clearAllNodes,
  moveNode,
} = useSubscriptions();
</script>
<template>
  <!-- 分区三：节点来源（本地节点/分组，或上游订阅链接） -->
  <el-divider content-position="left">节点来源</el-divider>
  <div class="mb-4">
    <el-radio-group v-model="isSourceMode" class="w-full" style="display: flex">
      <el-radio-button :value="false" style="flex: 1" class="text-center"
        >本地节点与分组</el-radio-button
      >
      <el-radio-button :value="true" style="flex: 1" class="text-center"
        >上游订阅链接</el-radio-button
      >
    </el-radio-group>
  </div>

  <template v-if="isSourceMode">
    <el-form-item label="上游订阅链接（支持主源 + 备用源）">
      <el-input
        v-model="sourceUrl"
        type="textarea"
        :rows="3"
        placeholder="每行一个订阅链接；主源失败时自动尝试下一行"
        clearable
      />
      <div class="field-hint">
        保存时立即拉取，之后每 6
        小时自动刷新；节点放进与订阅同名的分组，流量与到期取自上游。
      </div>
    </el-form-item>
  </template>

  <template v-else>
    <el-row :gutter="24">
      <el-col :span="12">
        <el-form-item label="选择具体节点">
          <div class="node-pick-tools">
            <UiButton variant="ghost" @click="selectAllNodes"
              >全选节点</UiButton
            >
            <UiButton
              variant="danger"
              @click="clearAllNodes"
              :disabled="!value1.length"
              >清空已选</UiButton
            >
          </div>
          <EmptyState
            v-if="!NodesList.length"
            size="section"
            title="暂无节点"
            description="添加节点后可在此选择。"
          />
          <el-collapse v-model="openedNodeGroups" class="node-group-picker">
            <el-collapse-item
              v-for="group in groupedNodes"
              :key="group.name"
              :name="group.name"
            >
              <template #title>
                <span class="picker-group-title">{{ group.name }}</span>
                <span class="picker-group-count"
                  >{{ group.nodes.length }} 个节点</span
                >
                <span
                  class="picker-group-confidence"
                  :class="{ empty: !group.validCount }"
                  >{{
                    group.validCount
                      ? `${group.validCount} 个有效`
                      : "暂无有效节点"
                  }}</span
                >
              </template>
              <div class="picker-node-list">
                <el-checkbox
                  v-for="item in group.nodes"
                  :key="item.Name"
                  :model-value="value1.includes(item.Name)"
                  @change="
                    (checked: any) =>
                      checked ? value1.push(item.Name) : removeNode(item.Name)
                  "
                >
                  <span class="picker-node-label">
                    <span class="picker-node-name">{{ item.Name }}</span>
                    <span
                      class="picker-node-confidence"
                      :class="{ empty: !nodeQuality(item)?.valid }"
                      >{{ nodeQualityLabel(item) }}</span
                    >
                  </span>
                </el-checkbox>
              </div>
              <UiButton variant="ghost" @click="selectGroupNodes(group.nodes)"
                >选择本组全部</UiButton
              >
            </el-collapse-item>
          </el-collapse>
        </el-form-item>
        <div class="field-label">已选节点（可拖拽排序）</div>
        <VueDraggable
          v-model="value1"
          :animation="reduced ? 0 : 150"
          ghost-class="ghost"
          class="order-list"
        >
          <div
            v-for="(nodeName, index) in value1"
            :key="nodeName"
            class="order-item"
          >
            <span class="order-badge">{{ index + 1 }}</span>
            <span class="order-name">{{ nodeName }}</span>
            <span
              class="order-confidence"
              :class="{ empty: !nodeQualityByName(nodeName)?.valid }"
              >{{ validLabel(nodeQualityByName(nodeName)) }}</span
            >
            <UiButton
              variant="ghost"
              :disabled="index === 0"
              :aria-label="`上移 ${nodeName}`"
              @click="moveNode(index, -1)"
              >上移</UiButton
            >
            <UiButton
              variant="ghost"
              :disabled="index === value1.length - 1"
              :aria-label="`下移 ${nodeName}`"
              @click="moveNode(index, 1)"
              >下移</UiButton
            >
            <UiButton
              variant="ghost"
              :aria-label="`移除 ${nodeName}`"
              @click="removeNode(nodeName)"
              >移除</UiButton
            >
          </div>
        </VueDraggable>
        <EmptyState
          v-if="!value1.length"
          size="section"
          title="尚未选择节点"
          description="选择具体节点或关联分组。"
        />
      </el-col>

      <el-col :span="12">
        <el-form-item label="关联节点分组">
          <div class="group-pick" style="width: 100%">
            <el-checkbox-group
              v-model="selectedGroups"
              class="group-checkbox-list"
            >
              <el-checkbox
                v-for="g in allGroups"
                :key="g.ID"
                :value="g.ID"
                class="group-checkbox"
              >
                <span class="gc-name">{{ g.Name }}</span>
                <span class="gc-count">{{ g.NodeCount }} 节点</span>
              </el-checkbox>
            </el-checkbox-group>
            <EmptyState
              v-if="!allGroups.length"
              size="section"
              title="暂无分组"
              description="创建分组后可自动跟进节点。"
            />
            <div v-if="selectedGroups.length" class="field-hint">
              已关联
              {{ selectedGroups.length }}
              个分组，拉取时自动展开最新节点。
            </div>
          </div>
        </el-form-item>
      </el-col>
    </el-row>
  </template>
</template>
<style scoped src="./forms.scss"></style>
