<script setup lang="ts">
import {
  UiDrawer,
  UiButton,
  UiTable,
  UiSkeleton,
  UiTag,
  EmptyState,
} from "@/ui";
import { useTemplates } from "./useTemplates";
const {
  regressionVisible,
  regressionLoading,
  regressionSaving,
  regressionError,
  regressionForm,
  regressionCases,
  regressionDiff,
  regressionResults,
  persistRegression,
  editRegression,
  removeRegression,
  openRegression,
} = useTemplates();
</script>
<template>
  <UiDrawer v-model="regressionVisible" title="分流回归用例" size="860px">
    <p class="muted">设置预期策略与地区，比较编辑前后的命中变化。</p>
    <el-form
      label-position="top"
      class="regression-form"
      @submit.prevent="persistRegression"
    >
      <el-form-item label="名称" required
        ><el-input
          v-model="regressionForm.name"
          aria-label="回归用例名称"
          placeholder="例如 Gemini 必须走 AI" /></el-form-item
      ><el-form-item label="域名" required
        ><el-input
          v-model="regressionForm.domain"
          aria-label="回归域名"
          placeholder="gemini.google.com" /></el-form-item
      ><el-form-item label="预期策略"
        ><el-input
          v-model="regressionForm.expectedPolicy"
          aria-label="预期策略"
          placeholder="可选" /></el-form-item
      ><el-form-item label="预期地区"
        ><el-input
          v-model="regressionForm.expectedCountry"
          aria-label="预期地区"
          placeholder="JP / US（可选）" /></el-form-item
      ><el-form-item label="禁止策略"
        ><el-input
          v-model="regressionForm.forbiddenPolicy"
          aria-label="禁止策略"
          placeholder="例如 DIRECT" /></el-form-item
      ><el-form-item label="协议"
        ><el-select v-model="regressionForm.protocol" aria-label="回归协议"
          ><el-option label="TCP" value="tcp" /><el-option
            label="UDP"
            value="udp" /></el-select></el-form-item
      ><el-form-item label="端口"
        ><el-input-number
          v-model="regressionForm.port"
          :min="1"
          :max="65535"
          aria-label="回归端口" /></el-form-item
      ><el-form-item label="启用状态"
        ><el-switch
          v-model="regressionForm.enabled"
          aria-label="启用回归用例"
          active-text="启用"
      /></el-form-item>
    </el-form>
    <div class="actions">
      <UiButton
        variant="primary"
        :loading="regressionSaving"
        @click="persistRegression"
        >{{ regressionForm.id ? "更新用例" : "保存用例" }}</UiButton
      ><UiButton :loading="regressionLoading" @click="openRegression"
        >刷新比较</UiButton
      >
    </div>
    <p v-if="regressionError" class="danger-text" role="alert">
      {{ regressionError }}
    </p>
    <UiSkeleton v-if="regressionLoading" :lines="3" height="56px" />
    <template v-else>
      <UiTable :rows="regressionCases" row-key="id" label="回归用例列表"
        ><el-table-column
          prop="name"
          label="名称"
          min-width="130" /><el-table-column
          prop="domain"
          label="域名"
          min-width="170" /><el-table-column
          prop="expectedPolicy"
          label="预期策略"
          min-width="120" /><el-table-column
          prop="expectedCountry"
          label="地区"
          width="80" /><el-table-column
          prop="forbiddenPolicy"
          label="禁止"
          width="90" /><el-table-column label="操作" width="160"
          ><template #default="{ row }"
            ><UiButton variant="ghost" @click="editRegression(row)"
              >编辑</UiButton
            ><UiButton
              variant="ghost"
              class="danger-text"
              @click="removeRegression(row)"
              >删除</UiButton
            ></template
          ></el-table-column
        ><template #mobile-card="{ row }"
          ><div class="card-title">
            <strong>{{ row.name }}</strong
            ><UiTag>{{ row.enabled ? "启用" : "停用" }}</UiTag>
          </div>
          <dl>
            <dt>域名</dt>
            <dd class="mono">{{ row.domain }}</dd>
            <dt>预期策略</dt>
            <dd>{{ row.expectedPolicy || "未指定" }}</dd>
            <dt>地区</dt>
            <dd>{{ row.expectedCountry || "未指定" }}</dd>
            <dt>禁止</dt>
            <dd>{{ row.forbiddenPolicy || "未指定" }}</dd>
            <dt>协议 / 端口</dt>
            <dd class="mono">{{ row.protocol }} / {{ row.port }}</dd>
          </dl>
          <div class="actions">
            <UiButton @click="editRegression(row)">编辑</UiButton
            ><UiButton variant="danger" @click="removeRegression(row)"
              >删除</UiButton
            >
          </div></template
        ><template #empty
          ><EmptyState
            size="section"
            title="暂无回归用例"
            description="填写上方表单创建用例。" /></template
      ></UiTable>
      <section v-if="regressionDiff.length">
        <h3>编辑前后命中差异</h3>
        <UiTable
          :rows="regressionDiff"
          row-key="domain"
          label="编辑前后命中差异"
          ><el-table-column
            prop="domain"
            label="域名"
            min-width="170"
          /><el-table-column
            prop="beforePolicy"
            label="修改前"
            min-width="120"
          /><el-table-column
            prop="afterPolicy"
            label="修改后"
            min-width="120"
          /><el-table-column label="变化" width="100"
            ><template #default="{ row }"
              ><UiTag :tone="row.changed ? 'warn' : 'ok'">{{
                row.changed ? "变化" : "一致"
              }}</UiTag></template
            ></el-table-column
          ><template #mobile-card="{ row }"
            ><strong class="mono">{{ row.domain }}</strong>
            <p>
              {{ row.beforePolicy || "未命中" }} →
              {{ row.afterPolicy || "未命中" }}
            </p>
            <UiTag :tone="row.changed ? 'warn' : 'ok'">{{
              row.changed ? "变化" : "一致"
            }}</UiTag></template
          ></UiTable
        >
      </section>
      <section v-if="regressionResults.length">
        <h3>修改后回归结果</h3>
        <UiTable
          :rows="regressionResults"
          row-key="domain"
          label="修改后回归结果"
          ><el-table-column label="用例" min-width="140"
            ><template #default="{ row }">{{
              row.case?.name
            }}</template></el-table-column
          ><el-table-column
            prop="policy"
            label="实际策略"
            min-width="120"
          /><el-table-column label="结果" width="100"
            ><template #default="{ row }"
              ><UiTag :tone="row.passed ? 'ok' : 'bad'">{{
                row.passed ? "通过" : "失败"
              }}</UiTag></template
            ></el-table-column
          ><el-table-column
            prop="reason"
            label="原因"
            min-width="180"
          /><template #mobile-card="{ row }"
            ><div class="card-title">
              <strong>{{ row.case?.name }}</strong
              ><UiTag :tone="row.passed ? 'ok' : 'bad'">{{
                row.passed ? "通过" : "失败"
              }}</UiTag>
            </div>
            <p>实际策略：{{ row.policy || "未命中" }}</p>
            <p class="muted">{{ row.reason }}</p></template
          ></UiTable
        >
      </section>
    </template>
    <template #footer
      ><UiButton @click="regressionVisible = false">关闭</UiButton></template
    >
  </UiDrawer>
</template>
<style scoped>
.regression-form {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  column-gap: 16px;
  margin-top: 16px;
}
.regression-form :deep(.el-input-number),
.regression-form :deep(.el-select) {
  width: 100%;
}
.actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin: 12px 0 16px;
}
.danger-text {
  color: var(--c-bad);
}
.muted {
  color: var(--c-muted);
}
p {
  font-size: 14px;
  overflow-wrap: anywhere;
}
.mono {
  font-family: var(--font-mono);
}
section {
  margin-top: 20px;
  padding-top: 16px;
  border-top: 1px solid var(--c-line);
}
h3 {
  font-size: 16px;
}
.card-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
dl {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr);
  gap: 8px;
  font-size: 14px;
}
dt {
  color: var(--c-muted);
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
@media (max-width: 767px) {
  .regression-form {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
