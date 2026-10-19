<script setup lang="ts">
import { UiDrawer, UiButton, EmptyState, StatusDot } from "@/ui";
import GroupFields from "./GroupFields.vue";
import CountryPicker from "./CountryPicker.vue";
import { useNodes, nodeEffectiveHidden } from "./useNodes";
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
  hiddenNodeCount,
  toggleGroupHidden,
  toggleNodeHidden,
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
    size="560px"
    :close-on-click-modal="!saving"
  >
    <el-form label-position="top" @submit.prevent="submitNode">
      <section class="form-section">
        <h3>连接信息</h3>
        <el-form-item label="节点链接" required
          ><el-input
            v-model="nodeForm.Link"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 12 }"
            placeholder="粘贴链接，支持换行或英文逗号分隔"
        /></el-form-item>
        <p class="field-hint">添加时每行一个链接；编辑时修改当前节点。</p>
        <el-form-item v-if="dialogMode === 'edit'" label="节点名称"
          ><el-input
            v-model="nodeForm.Name"
            placeholder="例如 东京 01、Nosla-JP、AKKO_USA"
        /></el-form-item>
      </section>
      <section class="form-section">
        <h3>国旗</h3>
        <el-form-item label="国家/地区">
          <CountryPicker
            v-model="nodeForm.Country"
            :name="dialogMode === 'edit' ? nodeForm.Name : ''"
            :multiple="dialogMode === 'add'"
          />
        </el-form-item>
      </section>
      <section class="form-section">
        <h3>节点分组</h3>
        <GroupFields />
      </section>
    </el-form>
    <template #footer
      ><UiButton :disabled="saving" @click="nodeDialog = false">取消</UiButton
      ><UiButton variant="primary" :loading="saving" @click="submitNode">{{
        dialogMode === "add" ? "添加节点" : "保存修改"
      }}</UiButton></template
    >
  </UiDrawer>
  <UiDrawer v-model="groupDialog" title="分组管理" size="640px">
    <p class="field-hint">
      当前隐藏
      <span class="mono">{{ hiddenNodeCount }}</span>
      个节点。隐藏影响全局展示、检测、推荐及订阅下发，数据仍保留。
    </p>
    <section class="form-section">
      <h3>分组可见性</h3>
      <EmptyState
        v-if="!groupDetails.length"
        size="inline"
        description="暂无分组，可在下方创建并绑定节点。"
      />
      <div v-for="group in groupDetails" :key="group.id" class="visibility-row">
        <div class="visibility-name">
          <strong>{{ group.name }}</strong
          ><span class="mono">{{ group.node_count }} 个节点</span>
        </div>
        <StatusDot
          :state="group.hidden ? 'idle' : 'ok'"
          :label="group.hidden ? '已隐藏' : '显示'"
        />
        <UiButton
          variant="ghost"
          :disabled="busy"
          @click="toggleGroupHidden(group)"
          >{{ group.hidden ? "恢复分组" : "隐藏整组" }}</UiButton
        >
      </div>
    </section>
    <section class="form-section">
      <h3>节点可见性</h3>
      <el-select
        v-model="manageGroupId"
        placeholder="选择分组查看节点"
        aria-label="查看分组内节点"
        ><el-option
          v-for="group in groupDetails"
          :key="group.id"
          :label="group.name"
          :value="group.id"
      /></el-select>
      <div class="managed-nodes">
        <div
          v-for="node in manageGroupNodes"
          :key="node.ID"
          class="visibility-row"
        >
          <strong class="visibility-name">{{ node.Name }}</strong
          ><StatusDot
            :state="nodeEffectiveHidden(node) ? 'idle' : 'ok'"
            :label="nodeEffectiveHidden(node) ? '全局隐藏' : '显示'"
          />
          <UiButton
            variant="ghost"
            :disabled="busy"
            @click="toggleNodeHidden(node)"
            >{{ node.Hidden ? "恢复节点" : "隐藏节点" }}</UiButton
          >
        </div>
        <EmptyState
          v-if="!manageGroupNodes.length"
          size="inline"
          description="该分组暂无节点。"
        />
      </div>
      <p class="field-hint">整组隐藏时，需先恢复分组，组内节点才会显示。</p>
    </section>
    <section class="form-section">
      <h3>绑定节点</h3>
      <p class="field-hint">保留原分组，添加所选分组。</p>
      <el-form label-position="top" @submit.prevent="bindGroup">
        <el-form-item label="选择节点" required
          ><el-select
            v-model="selectionNode"
            filterable
            placeholder="搜索并选择节点"
            aria-label="绑定的节点"
            ><el-option
              v-for="node in fullNodes"
              :key="node.ID"
              :label="node.Name"
              :value="node.Name" /></el-select
        ></el-form-item>
        <GroupFields /><UiButton :loading="busy" @click="bindGroup"
          >绑定分组</UiButton
        >
      </el-form>
    </section>
    <section class="form-section">
      <h3>删除分组</h3>
      <p class="field-hint">仅解除节点绑定，节点仍保留。</p>
      <div class="delete-group">
        <el-select
          v-model="delGroupName"
          placeholder="选择要删除的分组"
          aria-label="删除的分组"
          ><el-option
            v-for="group in groupDetails"
            :key="group.id"
            :label="group.name"
            :value="group.name"
        /></el-select>
        <UiButton variant="danger" :disabled="busy" @click="deleteGroup"
          >删除所选</UiButton
        >
      </div>
    </section>
    <template #footer
      ><UiButton @click="groupDialog = false">关闭</UiButton></template
    >
  </UiDrawer>
</template>
<style scoped>
.form-section {
  padding: 8px 0 16px;
  border-bottom: 1px solid var(--c-line);
  margin-bottom: 16px;
}
h3 {
  margin: 0 0 16px;
  font: 600 17px/1.4 var(--font-ui);
}
.field-hint {
  color: var(--c-muted);
  font-size: 14px;
  line-height: 1.6;
  margin: 10px 0 16px;
}
.mono {
  font: 400 14px/1.6 var(--font-mono);
}
:deep(.el-select) {
  width: 100%;
}
.visibility-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px 12px;
  min-height: 64px;
  border-bottom: 1px solid var(--c-line);
  padding: 8px 0;
  font-size: 14px;
}
.visibility-name {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-width: 100px;
  overflow-wrap: anywhere;
  gap: 4px;
}
.visibility-name .mono {
  color: var(--c-muted);
  font-size: 14px;
}
.managed-nodes {
  max-height: 320px;
  overflow-y: auto;
  margin-top: 12px;
}
.delete-group {
  display: flex;
  align-items: center;
  gap: 10px;
}
.delete-group :deep(.el-select) {
  flex: 1;
  min-width: 0;
}
@media (max-width: 767px) {
  .delete-group {
    flex-wrap: wrap;
  }
  .delete-group :deep(.el-select) {
    flex-basis: 100%;
  }
}
</style>
