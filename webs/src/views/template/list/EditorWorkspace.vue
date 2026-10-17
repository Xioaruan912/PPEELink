<script setup lang="ts">
import { UiButton, UiSegmented, StatusDot, EmptyState } from "@/ui";
import TemplateMonacoEditor from "@/components/TemplateMonacoEditor.vue";
import VersionTimeline from "./VersionTimeline.vue";
import { useTemplates } from "./useTemplates";
const {
  Tempname,
  TempText,
  Tempoldname,
  editorRef,
  editorLanguage,
  validationErrors,
  validating,
  validationError,
  validationCurrent,
  preflightLoading,
  preflightReport,
  preflightCurrent,
  preflightVisible,
  visibleOutline,
  outline,
  outlineExpanded,
  inspectTab,
  saving,
  runValidation,
  runPreflight,
  openVersions,
  openRegression,
  queueTemplateValidation,
} = useTemplates();
const tabs = [
  { label: "检查", value: "checks" },
  { label: "结构", value: "outline" },
  { label: "版本", value: "versions" },
];
</script>
<template>
  <section data-enter>
    <div class="workspace-toolbar">
      <el-form label-position="top" class="filename-form"
        ><el-form-item label="模板文件名" required
          ><el-input
            v-model="Tempname"
            :disabled="saving"
            aria-label="模板文件名"
            placeholder="my_clash.yaml / loon.conf"
            clearable /></el-form-item
      ></el-form>
      <div class="workspace-actions">
        <UiButton
          :loading="validating"
          :disabled="saving"
          @click="runValidation"
          >校验语法</UiButton
        ><UiButton
          :loading="preflightLoading"
          :disabled="saving"
          @click="runPreflight(true)"
          >发布前预检</UiButton
        ><el-dropdown trigger="click"
          ><UiButton>更多操作</UiButton
          ><template #dropdown
            ><el-dropdown-menu
              ><el-dropdown-item v-if="Tempoldname" @click="openVersions"
                >版本历史</el-dropdown-item
              ><el-dropdown-item @click="openRegression"
                >回归用例</el-dropdown-item
              ><el-dropdown-item @click="queueTemplateValidation"
                >后台验证已保存模板</el-dropdown-item
              ></el-dropdown-menu
            ></template
          ></el-dropdown
        >
      </div>
    </div>
    <div class="workspace-grid">
      <main class="editor-pane">
        <div class="editor-heading">
          <span class="mono">{{ editorLanguage.toUpperCase() }}</span
          ><span>编辑内容</span>
        </div>
        <TemplateMonacoEditor
          ref="editorRef"
          v-model="TempText"
          :language="editorLanguage"
          :errors="validationCurrent ? validationErrors : []"
          :read-only="saving"
          :minimap="false"
        />
      </main>
      <aside class="inspect-pane" aria-label="模板检查与历史">
        <UiSegmented v-model="inspectTab" :options="tabs" label="模板侧栏" />
        <div v-show="inspectTab === 'checks'" class="inspect-body">
          <h2>语法校验</h2>
          <StatusDot
            :state="
              validating
                ? 'warn'
                : !validationCurrent
                  ? 'idle'
                  : validationErrors.length
                    ? 'bad'
                    : 'ok'
            "
            :running="validating"
            :label="
              validating
                ? '正在检查'
                : !validationCurrent
                  ? '等待校验'
                  : validationErrors.length
                    ? '检查未通过'
                    : '语法通过'
            "
          />
          <p v-if="validationError" class="danger-text" role="alert">
            {{ validationError }}
          </p>
          <ul
            v-if="validationCurrent && validationErrors.length"
            class="error-list"
          >
            <li v-for="message in validationErrors" :key="message">
              {{ message }}
            </li>
          </ul>
          <p v-else class="muted">
            {{
              validationCurrent
                ? "服务端语法检查未发现问题。"
                : "内容改变后需重新检查。"
            }}
          </p>
          <div class="inspect-section">
            <h2>发布前预检</h2>
            <StatusDot
              :state="
                preflightLoading
                  ? 'warn'
                  : !preflightCurrent
                    ? 'idle'
                    : preflightReport?.valid
                      ? 'ok'
                      : 'bad'
              "
              :running="preflightLoading"
              :label="
                preflightLoading
                  ? '正在预检'
                  : !preflightCurrent
                    ? '等待预检'
                    : preflightReport?.valid
                      ? '预检通过'
                      : '阻止保存'
              "
            />
            <dl v-if="preflightCurrent" class="check-stats">
              <dt>错误 / 警告</dt>
              <dd>
                {{ preflightReport?.summary?.errors || 0 }} /
                {{ preflightReport?.summary?.warnings || 0 }}
              </dd>
              <dt>策略组 / 规则</dt>
              <dd>
                {{ preflightReport?.summary?.groups || 0 }} /
                {{ preflightReport?.summary?.rules || 0 }}
              </dd>
            </dl>
            <p class="muted">保存时会检查最新内容。</p>
            <UiButton @click="preflightVisible = true">查看完整报告</UiButton>
          </div>
          <div class="inspect-section">
            <h2>版本时间线</h2>
            <VersionTimeline v-if="Tempoldname" />
            <p v-else class="muted">保存后开始记录版本。</p>
          </div>
        </div>
        <div v-show="inspectTab === 'outline'" class="inspect-body">
          <div class="outline-heading">
            <h2>结构导航</h2>
            <UiButton
              v-if="outline.some((item) => item.level > 0)"
              variant="ghost"
              @click="outlineExpanded = !outlineExpanded"
              >{{ outlineExpanded ? "仅一级" : "展开全部" }}</UiButton
            >
          </div>
          <p class="muted mono">
            {{ visibleOutline.length }} / {{ outline.length }} 项
          </p>
          <EmptyState
            v-if="!outline.length"
            size="section"
            title="暂无结构"
            description="校验语法后生成导航。"
          /><button
            v-for="item in visibleOutline"
            :key="`${item.line}-${item.key}`"
            class="outline-item"
            :style="{ paddingLeft: `${8 + Math.min(item.level, 4) * 12}px` }"
            @click="editorRef?.revealLine(item.line)"
          >
            <span>{{ item.key }}</span
            ><small>{{ item.line }}</small>
          </button>
        </div>
        <div v-show="inspectTab === 'versions'" class="inspect-body">
          <h2>版本时间线</h2>
          <VersionTimeline v-if="Tempoldname" /><EmptyState
            v-else
            size="section"
            title="尚未保存"
            description="保存后开始记录版本。"
          />
        </div>
      </aside>
    </div>
  </section>
</template>
<style scoped>
.workspace-toolbar {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 24px;
}
.filename-form {
  flex: 1;
  min-width: 0;
  max-width: 480px;
}
.filename-form :deep(.el-form-item) {
  margin-bottom: 0;
}
.workspace-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.workspace-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 30%);
  align-items: start;
  gap: 24px;
}
.editor-pane,
.inspect-pane {
  min-width: 0;
}
.editor-pane {
  height: max(600px, 68vh);
  display: flex;
  flex-direction: column;
}
.editor-pane :deep(.monaco-host) {
  flex: 1;
  min-height: 0;
}
.editor-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 0 12px;
  font-size: 13px;
  color: var(--c-muted);
}
.inspect-pane > .ui-segmented {
  width: 100%;
}
.inspect-body {
  padding-top: 24px;
}
.inspect-body h2 {
  font-size: 18px;
  line-height: 1.4;
  margin: 0 0 16px;
}
.inspect-body p {
  font-size: 14px;
  margin: 16px 0;
}
.inspect-section {
  border-top: 1px solid var(--c-line);
  padding-top: 24px;
  margin-top: 24px;
}
.mono,
.check-stats dd {
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
}
.muted {
  color: var(--c-muted);
}
.check-stats {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 12px;
  font-size: 14px;
  margin-top: 20px;
}
.check-stats dt {
  color: var(--c-muted);
}
.check-stats dd {
  margin: 0;
}
.error-list {
  font-size: 14px;
  padding-left: 20px;
  overflow-wrap: anywhere;
  color: var(--c-bad);
}
.danger-text {
  color: var(--c-bad);
}
.outline-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
.outline-heading .ui-button {
  padding: 8px;
  font-size: 14px;
}
.outline-item {
  display: flex;
  width: 100%;
  min-height: 44px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px;
  border: 0;
  border-bottom: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  text-align: left;
  font-size: 14px;
}
.outline-item:hover {
  background: var(--c-surface-2);
}
.outline-item span {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.outline-item small {
  font: 400 13px var(--font-mono);
  color: var(--c-muted);
}
@media (max-width: 1023px) {
  .workspace-toolbar {
    align-items: stretch;
    flex-direction: column;
    gap: 16px;
  }
  .filename-form {
    max-width: none;
  }
}
@media (max-width: 1023px) {
  .workspace-grid {
    grid-template-columns: minmax(0, 1fr);
    gap: 32px;
  }
  .editor-pane {
    height: 65vh;
    min-height: 420px;
  }
  .inspect-pane {
    border-top: 1px solid var(--c-line);
    padding-top: 24px;
  }
  .workspace-actions {
    gap: 8px;
  }
  .workspace-actions .ui-button {
    padding: 8px 12px;
  }
}
</style>
