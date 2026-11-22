<script setup lang="ts">
import { UiDrawer, UiButton, EmptyState, StatusDot } from "@/ui";
import GroupFields from "./GroupFields.vue";
import CountryPicker from "./CountryPicker.vue";
import { useNodes } from "./useNodes";

const {
  nodeDialog,
  groupDialog,
  dialogMode,
  nodeForm,
  saving,
  submitNode,
  busy,
  groupDetails,
  fullNodes,
  manageGroupNodes,
  manageGroupId,
  setDelivery,
  selectionNode,
  bindGroup,
  delGroupName,
  deleteGroup,
} = useNodes();
</script>

<template>
  <UiDrawer
    v-model="nodeDialog"
    :title="dialogMode === 'add' ? '添加节点' : '编辑节点'"
    size="min(620px, 100vw)"
    :close-on-click-modal="!saving"
  >
    <el-form class="gallery-form" label-position="top" @submit.prevent="submitNode">
      <section class="drawer-section">
        <header class="section-head">
          <h3>连接信息</h3>
          <p>
            {{
              dialogMode === "add"
                ? "每行粘贴一个节点链接，支持批量导入或英文逗号分隔。"
                : "更新节点名称与底层连接 URI。"
            }}
          </p>
        </header>
        <el-form-item label="节点链接" required>
          <el-input
            v-model="nodeForm.Link"
            class="mono-textarea"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 12 }"
            placeholder="vless://... / hysteria2://... / ss://..."
          />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'edit'" label="节点名称">
          <el-input v-model="nodeForm.Name" placeholder="输入节点名称" />
        </el-form-item>
      </section>

      <section class="drawer-section">
        <header class="section-head">
          <h3>归属地区</h3>
          <p>用于在节点名录与地图中标识出口国家或地区。</p>
        </header>
        <el-form-item label="国家 / 地区">
          <CountryPicker
            v-model="nodeForm.Country"
            :name="dialogMode === 'edit' ? nodeForm.Name : ''"
            :multiple="dialogMode === 'add'"
          />
        </el-form-item>
      </section>

      <section class="drawer-section is-last">
        <header class="section-head">
          <h3>节点分组</h3>
          <p>将节点归入一个或多个分组，供订阅动态引用。</p>
        </header>
        <GroupFields />
      </section>
    </el-form>

    <template #footer>
      <UiButton variant="ghost" :disabled="saving" @click="nodeDialog = false">
        取消
      </UiButton>
      <UiButton variant="primary" :loading="saving" @click="submitNode">
        {{ dialogMode === "add" ? "添加节点" : "保存修改" }}
      </UiButton>
    </template>
  </UiDrawer>

  <UiDrawer v-model="groupDialog" title="分组管理" size="min(660px, 100vw)">
    <section class="drawer-section">
      <header class="section-head">
        <h3>现有分组</h3>
        <p>共 {{ groupDetails.length }} 个分组。</p>
      </header>
      <EmptyState
        v-if="!groupDetails.length"
        size="inline"
        description="暂无分组，可在下方创建并绑定节点。"
      />
      <div v-else class="group-chips">
        <div v-for="group in groupDetails" :key="group.id" class="group-chip">
          <strong>{{ group.name }}</strong>
          <span class="mono">{{ group.node_count }}</span>
        </div>
      </div>
    </section>

    <section class="drawer-section">
      <header class="section-head">
        <h3>分组内下发控制</h3>
        <p>关闭下发后节点仍保留在列表中，可继续管理和检测。</p>
      </header>
      <el-select
        v-model="manageGroupId"
        placeholder="选择分组查看节点"
        aria-label="查看分组内节点"
      >
        <el-option
          v-for="group in groupDetails"
          :key="group.id"
          :label="`${group.name} (${group.node_count})`"
          :value="group.id"
        />
      </el-select>
      <div class="managed-nodes">
        <div
          v-for="node in manageGroupNodes"
          :key="node.ID"
          class="visibility-row"
        >
          <strong class="visibility-name">{{ node.Name }}</strong>
          <StatusDot
            :state="node.deliveryDisabled ? 'idle' : 'ok'"
            :label="node.deliveryDisabled ? '不下发' : '允许下发'"
          />
          <UiButton
            size="sm"
            variant="ghost"
            :disabled="busy"
            @click="setDelivery(node.ID, !!node.deliveryDisabled)"
          >
            {{ node.deliveryDisabled ? "开启下发" : "关闭下发" }}
          </UiButton>
        </div>
        <EmptyState
          v-if="!manageGroupNodes.length"
          size="inline"
          description="该分组暂无节点。"
        />
      </div>
    </section>

    <section class="drawer-section">
      <header class="section-head">
        <h3>快速绑定节点</h3>
        <p>保留原分组，追加所选分组。</p>
      </header>
      <el-form label-position="top" @submit.prevent="bindGroup">
        <el-form-item label="选择节点" required>
          <el-select
            v-model="selectionNode"
            filterable
            placeholder="搜索并选择节点"
            aria-label="绑定的节点"
          >
            <el-option
              v-for="node in fullNodes"
              :key="node.ID"
              :label="node.Name"
              :value="node.Name"
            />
          </el-select>
        </el-form-item>
        <GroupFields />
        <UiButton variant="primary" :loading="busy" @click="bindGroup">
          绑定分组
        </UiButton>
      </el-form>
    </section>

    <section class="drawer-section is-last">
      <header class="section-head">
        <h3>删除分组</h3>
        <p>仅解除节点与分组的关联，节点本身不会被删除。</p>
      </header>
      <div class="delete-group">
        <el-select
          v-model="delGroupName"
          placeholder="选择要删除的分组"
          aria-label="删除的分组"
        >
          <el-option
            v-for="group in groupDetails"
            :key="group.id"
            :label="group.name"
            :value="group.name"
          />
        </el-select>
        <UiButton variant="danger" :disabled="busy" @click="deleteGroup">
          删除所选
        </UiButton>
      </div>
    </section>

    <template #footer>
      <UiButton @click="groupDialog = false">关闭</UiButton>
    </template>
  </UiDrawer>
</template>

<style scoped>
.drawer-section {
  padding: 0 0 22px;
  margin-bottom: 22px;
  border-bottom: 1px solid var(--c-line);
}

.drawer-section.is-last {
  padding-bottom: 0;
  margin-bottom: 0;
  border-bottom: 0;
}

.section-head {
  display: grid;
  gap: 3px;
  margin-bottom: 14px;
}

.section-head h3 {
  margin: 0;
  font-size: 14.5px;
  font-weight: 500;
  letter-spacing: -0.015em;
  color: var(--c-ink);
}

.section-head p {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-muted);
  line-height: 1.5;
}

.mono-textarea :deep(textarea) {
  font-family: var(--font-mono);
  font-size: 12.5px;
  line-height: 1.65;
}

:deep(.el-select) {
  width: 100%;
}

.group-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.group-chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-tray);
  font-size: 13px;
}

.group-chip strong {
  font-weight: 500;
  color: var(--c-ink);
}

.group-chip span {
  font-size: 11.5px;
  color: var(--c-muted);
}

.managed-nodes {
  margin-top: 12px;
}

.visibility-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  min-height: 52px;
  padding: 8px 0;
  border-bottom: 1px solid var(--c-line);
  font-size: 13.5px;
}

.visibility-row:last-child {
  border-bottom: 0;
}

.visibility-name {
  flex: 1 1 180px;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
  color: var(--c-ink);
}

.delete-group {
  display: flex;
  gap: 10px;
  align-items: center;
}

.delete-group :deep(.el-select) {
  flex: 1;
}
</style>
