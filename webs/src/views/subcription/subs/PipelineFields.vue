<script setup lang="ts">
import { ref } from "vue";
import { UiButton, UiSegmented, EmptyState } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
const step = ref<string | number>("filter");
const steps = [
  { label: "筛选", value: "filter" },
  { label: "命名与排序", value: "organize" },
];
const {
  pipeline,
  emojiRulesText,
  pipelinePreview,
  pipelineLoading,
  runPipelinePreview,
  loading,
} = useSubscriptions();
</script>
<template>
  <section class="pipeline-section">
    <div class="pipeline-section-head">
      <div>
        <div class="pipeline-title">
          节点处理
          <el-tag size="small" type="info" effect="plain">可选</el-tag>
        </div>
        <div class="pipeline-desc">筛选、整理并预览节点处理结果。</div>
      </div>
      <span class="pipeline-hint">按顺序应用</span>
    </div>
    <UiSegmented v-model="step" :options="steps" label="流水线规则分区" />
    <div class="pipeline-card">
      <el-row :gutter="12">
        <el-col v-show="step === 'filter'" :span="12" :xs="24"
          ><el-form-item label="包含名称（正则）"
            ><el-input
              v-model="pipeline.include"
              placeholder="例如 香港|日本"
              clearable /></el-form-item
        ></el-col>
        <el-col v-show="step === 'filter'" :span="12" :xs="24"
          ><el-form-item label="排除名称（正则）"
            ><el-input
              v-model="pipeline.exclude"
              placeholder="例如 剩余|官网"
              clearable /></el-form-item
        ></el-col>
        <el-col v-show="step === 'organize'" :span="12" :xs="24"
          ><el-form-item label="重命名正则"
            ><el-input
              v-model="pipeline.renamePattern"
              placeholder="例如 ^\[上游A\]\s*"
              clearable /></el-form-item
        ></el-col>
        <el-col v-show="step === 'organize'" :span="12" :xs="24"
          ><el-form-item label="替换为"
            ><el-input
              v-model="pipeline.renameReplacement"
              placeholder="留空表示删除匹配内容"
              clearable /></el-form-item
        ></el-col>
        <el-col v-show="step === 'filter'" :span="12" :xs="24"
          ><el-form-item label="协议过滤"
            ><el-select
              v-model="pipeline.protocols"
              multiple
              clearable
              placeholder="全部协议"
              class="full"
              ><el-option
                v-for="p in [
                  'ss',
                  'ssr',
                  'vmess',
                  'vless',
                  'trojan',
                  'hysteria2',
                  'tuic',
                ]"
                :key="p"
                :label="p"
                :value="p" /></el-select></el-form-item
        ></el-col>
        <el-col v-show="step === 'organize'" :span="8" :xs="16"
          ><el-form-item label="排序"
            ><el-select v-model="pipeline.sort" class="full"
              ><el-option label="保留原顺序" value="original" /><el-option
                label="名称"
                value="name" /><el-option
                label="国家/地区"
                value="country" /><el-option
                label="低延迟优先"
                value="latency" /><el-option
                label="质量分优先"
                value="quality" /></el-select></el-form-item
        ></el-col>
        <el-col v-show="step === 'filter'" :span="4" :xs="8"
          ><el-form-item label="最多节点"
            ><el-input-number
              v-model="pipeline.maxNodes"
              :min="0"
              :max="9999"
              controls-position="right" /></el-form-item
        ></el-col>
        <el-col v-show="step === 'filter'" :span="12" :xs="24"
          ><el-form-item
            ><el-checkbox v-model="pipeline.excludePlaceholder"
              >剔除官网/到期/流量等占位节点</el-checkbox
            ></el-form-item
          ></el-col
        >
        <el-col v-show="step === 'organize'" :span="24"
          ><el-form-item label="国旗 Emoji">
            <div class="flag-row">
              <el-checkbox v-model="pipeline.emoji">自动补国旗</el-checkbox>
              <el-checkbox
                v-model="pipeline.emojiRemoveOld"
                :disabled="!pipeline.emoji"
                >先移除旧国旗</el-checkbox
              >
            </div>
            <el-input
              v-model="emojiRulesText"
              type="textarea"
              :rows="2"
              class="emoji-rules"
              placeholder="自定义映射（可选），每行一个：关键字=国旗，例如 香港=🇭🇰"
            /> </el-form-item
        ></el-col>
      </el-row>
      <div class="pipeline-footer">
        <el-checkbox v-model="pipeline.dedupe">按节点链接去重</el-checkbox
        ><UiButton
          variant="secondary"
          :loading="pipelineLoading"
          @click="runPipelinePreview"
          >预览处理结果</UiButton
        >
      </div>
      <EmptyState
        v-if="!pipelinePreview && !pipelineLoading"
        size="section"
        title="尚未预览"
        description="保存订阅后可预览真实处理结果。"
      />
      <el-alert
        v-if="pipelinePreview"
        type="success"
        :closable="false"
        show-icon
        ><template #title
          >处理前 {{ pipelinePreview.before }} 个 → 处理后
          {{ pipelinePreview.after }} 个</template
        ><template #default
          ><span
            v-for="(count, reason) in pipelinePreview.rejected"
            :key="reason"
            class="reject-stat"
            >{{ reason }} {{ count }}</span
          ></template
        ></el-alert
      >
    </div>
  </section>
</template>
<style scoped src="./forms.scss"></style>
