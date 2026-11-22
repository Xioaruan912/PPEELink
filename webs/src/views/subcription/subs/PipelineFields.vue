<script setup lang="ts">
import { ref } from "vue";
import { UiButton } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";

const step = ref<"filter" | "organize">("filter");
const protocols = ["ss", "ssr", "vmess", "vless", "trojan", "hysteria2", "tuic"];
const sortOptions = [
  { label: "保留原顺序", value: "original" },
  { label: "按名称", value: "name" },
  { label: "按国家/地区", value: "country" },
  { label: "有效节点优先", value: "valid" },
];

const {
  pipeline,
  emojiRulesText,
  pipelinePreview,
  pipelineLoading,
  runPipelinePreview,
} = useSubscriptions();

function toggleProtocol(proto: string) {
  const idx = pipeline.protocols.indexOf(proto);
  if (idx >= 0) {
    pipeline.protocols.splice(idx, 1);
  } else {
    pipeline.protocols.push(proto);
  }
}
</script>

<template>
  <section class="pipe-layout">
    <div class="pipe-head">
      <div class="sub-pills" role="tablist" aria-label="流水线规则分区">
        <button
          type="button"
          role="tab"
          class="sub-pill"
          :class="{ 'is-active': step === 'filter' }"
          @click="step = 'filter'"
        >
          筛选与去重
        </button>
        <button
          type="button"
          role="tab"
          class="sub-pill"
          :class="{ 'is-active': step === 'organize' }"
          @click="step = 'organize'"
        >
          命名与排序
        </button>
      </div>

      <UiButton
        variant="ghost"
        :loading="pipelineLoading"
        @click="runPipelinePreview"
      >
        预览处理结果
      </UiButton>
    </div>

    <!-- Step 1: Filter -->
    <div v-show="step === 'filter'" class="pipe-fields">
      <div class="field-grid">
        <el-form-item label="包含名称（正则）">
          <el-input
            v-model="pipeline.include"
            placeholder="例如 香港|日本"
            clearable
          />
        </el-form-item>
        <el-form-item label="排除名称（正则）">
          <el-input
            v-model="pipeline.exclude"
            placeholder="例如 剩余|官网"
            clearable
          />
        </el-form-item>
      </div>

      <div class="field-block">
        <span class="field-cap">仅保留协议（不选表示全部协议）</span>
        <div class="chip-rail">
          <button
            v-for="p in protocols"
            :key="p"
            type="button"
            class="chip-btn mono"
            :class="{ 'is-active': pipeline.protocols.includes(p) }"
            @click="toggleProtocol(p)"
          >
            {{ p }}
          </button>
        </div>
      </div>

      <div class="field-grid">
        <el-form-item label="最大下发数量（0 表示不限）">
          <el-input-number
            v-model="pipeline.maxNodes"
            :min="0"
            :max="9999"
            controls-position="right"
            class="full"
          />
        </el-form-item>
      </div>

      <div class="check-row">
        <el-checkbox v-model="pipeline.excludePlaceholder">
          剔除官网 / 到期 / 流量等占位节点
        </el-checkbox>
        <el-checkbox v-model="pipeline.dedupe">
          按节点链接自动去重
        </el-checkbox>
      </div>
    </div>

    <!-- Step 2: Organize -->
    <div v-show="step === 'organize'" class="pipe-fields">
      <div class="field-block">
        <span class="field-cap">下发排序方式</span>
        <div class="chip-rail">
          <button
            v-for="opt in sortOptions"
            :key="opt.value"
            type="button"
            class="chip-btn"
            :class="{ 'is-active': pipeline.sort === opt.value }"
            @click="pipeline.sort = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div class="field-grid">
        <el-form-item label="重命名正则">
          <el-input
            v-model="pipeline.renamePattern"
            placeholder="例如 ^\[上游A\]\s*"
            clearable
          />
        </el-form-item>
        <el-form-item label="替换为">
          <el-input
            v-model="pipeline.renameReplacement"
            placeholder="留空表示删除匹配内容"
            clearable
          />
        </el-form-item>
      </div>

      <div class="field-block">
        <span class="field-cap">国旗 Emoji</span>
        <div class="check-row">
          <el-checkbox v-model="pipeline.emoji">自动补齐国旗</el-checkbox>
          <el-checkbox
            v-model="pipeline.emojiRemoveOld"
            :disabled="!pipeline.emoji"
          >
            先移除旧国旗
          </el-checkbox>
        </div>
        <el-input
          v-model="emojiRulesText"
          type="textarea"
          :rows="2"
          placeholder="自定义映射（可选），每行一个：香港=🇭🇰"
        />
      </div>
    </div>

    <!-- Preview Result Banner (only after click) -->
    <div v-if="pipelinePreview" class="preview-strip">
      <strong class="mono">
        {{ pipelinePreview.before }} → {{ pipelinePreview.after }} 节点
      </strong>
      <div class="preview-tags">
        <span
          v-for="(count, reason) in pipelinePreview.rejected"
          :key="reason"
          class="preview-tag"
        >
          {{ reason }} {{ count }}
        </span>
      </div>
    </div>
  </section>
</template>

<style scoped>
.pipe-layout {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.mono {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}

.pipe-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.sub-pills {
  display: inline-flex;
  gap: 6px;
}

.sub-pill {
  padding: 6px 14px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 13px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.16s ease;
}

.sub-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.pipe-fields {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.field-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.field-block {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field-cap {
  font-size: 13px;
  font-weight: 500;
  color: var(--c-ink);
}

.chip-rail {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.chip-btn {
  padding: 6px 12px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.16s ease;
}

.chip-btn:hover {
  color: var(--c-ink);
  border-color: color-mix(in srgb, var(--c-ink) 30%, transparent);
}

.chip-btn.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.check-row {
  display: flex;
  flex-wrap: wrap;
  gap: 10px 24px;
}

.full,
.pipe-fields :deep(.el-input-number) {
  width: 100%;
}

.preview-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 10px;
  padding: 13px 16px;
  border-radius: 12px;
  border: 1px solid color-mix(in srgb, #10b981 30%, transparent);
  background: color-mix(in srgb, #10b981 7%, var(--c-surface));
  font-size: 13.5px;
  color: var(--c-ink);
}

.preview-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.preview-tag {
  font-size: 12px;
  color: var(--c-muted);
}

@media (max-width: 640px) {
  .field-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
