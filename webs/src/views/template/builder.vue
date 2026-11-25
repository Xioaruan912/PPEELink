<script setup lang="ts">
import {
  computed,
  nextTick,
  onBeforeUnmount,
  onMounted,
  ref,
  watch,
} from "vue";
import gsap from "gsap";
import {
  ArrowLeft,
  ArrowRight,
  DocumentChecked,
  DocumentCopy,
  Refresh,
} from "@element-plus/icons-vue";
import { UiButton, UiSkeleton } from "@/ui";
import KineticNotice from "@/ui/KineticNotice.vue";
import TemplateMonacoEditor from "@/components/TemplateMonacoEditor.vue";
import { useMotion } from "@/motion";
import ClashFields from "./builder/ClashFields.vue";
import LoonFields from "./builder/LoonFields.vue";
import { provideTemplateBuilder } from "./builder/useTemplateBuilder";

defineOptions({ name: "TemplateBuilder" });

const {
  loading,
  sourceLoading,
  sourceError,
  buildError,
  savedFilename,
  savedRevision,
  isSavedPreview,
  templateSource,
  templateFiles,
  target,
  form,
  loonForm,
  preview,
  loadDefault,
  onSourceChange,
  build,
  copyYaml,
} = provideTemplateBuilder();

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const step = ref<number>(0);
const stageWrap = ref<HTMLElement>();
const outline = ref<HTMLElement>();
const { animate, reduced, stop } = useMotion();

const steps = computed(() =>
  target.value === "loon"
    ? [
        { num: "01", label: "模板与基础", value: 0 },
        { num: "02", label: "节点与策略", value: 1 },
        { num: "03", label: "规则与插件", value: 2 },
        { num: "04", label: "预览与生成", value: 3 },
      ]
    : [
        { num: "01", label: "模板与基础", value: 0 },
        { num: "02", label: "策略分组", value: 1 },
        { num: "03", label: "分流规则", value: 2 },
        { num: "04", label: "预览与生成", value: 3 },
      ]
);

const activeFilename = computed({
  get: () =>
    target.value === "loon" ? loonForm.value.filename : form.filename,
  set: (val: string) => {
    if (target.value === "loon") loonForm.value.filename = val;
    else form.filename = val;
  },
});

const groupCount = computed(() =>
  target.value === "loon"
    ? loonForm.value.groups.length
    : form.groups.length
);

function selectSource(val: string) {
  if (templateSource.value === val) return;
  templateSource.value = val;
  void onSourceChange(val);
}

function goStep(nextStep: number) {
  if (nextStep < 0 || nextStep >= steps.value.length || nextStep === step.value)
    return;
  const dir = nextStep > step.value ? 1 : -1;
  step.value = nextStep;
  void nextTick(() => {
    if (stageWrap.value && !reduced.value) {
      animate(
        stageWrap.value,
        { opacity: 0, x: dir * 18 },
        {
          opacity: 1,
          x: 0,
          duration: 0.24,
          clearProps: "opacity,transform",
          ease: "power3.out",
        }
      );
    }
  });
}

watch(savedRevision, async () => {
  await nextTick();
  if (!outline.value || reduced.value) return;
  stop();
  animate(
    outline.value,
    { opacity: 1, clipPath: "inset(0 100% 100% 0)" },
    {
      opacity: 1,
      clipPath: "inset(0 0 0 0)",
      duration: 0.52,
      ease: "power3.out",
      onComplete: () => {
        if (outline.value)
          animate(
            outline.value,
            { opacity: 1 },
            { opacity: 0, duration: 0.3 }
          );
      },
    }
  );
});

onMounted(() => {
  if (rootEl.value) {
    gsapCtx = gsap.context(() => {
      const mm = gsap.matchMedia();
      mm.add("(prefers-reduced-motion: no-preference)", () => {
        const tl = gsap.timeline({ defaults: { ease: "expo.out" } });
        tl.from("[data-title-reveal]", {
          yPercent: 108,
          duration: 0.62,
          clearProps: "transform",
        }).from(
          "[data-top-rule]",
          {
            scaleX: 0,
            duration: 0.68,
            clearProps: "transform",
          },
          "-=0.45"
        );
      });
    }, rootEl.value);
  }
});

onBeforeUnmount(() => {
  gsapCtx?.revert();
  gsapCtx = null;
});
</script>

<template>
  <div ref="rootEl" class="studio-builder">
    <!-- Pure Gallery Masthead -->
    <header class="builder-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>分流编排</h1>
        </div>
        <span class="file-pill mono">
          <i aria-hidden="true"></i>
          {{ activeFilename || "未命名模板" }}
        </span>
      </div>

      <div class="masthead-actions">
        <UiButton
          v-if="step < 3"
          variant="ghost"
          @click="goStep(3)"
        >
          预览配置
        </UiButton>
        <UiButton
          variant="primary"
          :icon="DocumentChecked"
          :loading="loading"
          :disabled="sourceLoading"
          @click="build"
        >
          生成并保存
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <!-- Apple Studio 4-Stage Stepper Rail -->
    <nav class="stepper-rail" aria-label="构建流程">
      <button
        v-for="(s, idx) in steps"
        :key="s.value"
        type="button"
        class="step-tab"
        :class="{
          'is-active': step === idx,
          'is-done': step > idx,
        }"
        :aria-current="step === idx ? 'step' : undefined"
        @click="goStep(idx)"
      >
        <span class="step-num mono">{{ s.num }}</span>
        <strong class="step-label">{{ s.label }}</strong>
      </button>
    </nav>

    <KineticNotice :raw="sourceError || buildError">
      <template v-if="sourceError" #action>
        <UiButton
          size="sm"
          :icon="Refresh"
          variant="ghost"
          @click="loadDefault(templateSource === 'smart' ? 'smart' : 'default')"
        >
          重试
        </UiButton>
      </template>
    </KineticNotice>

    <!-- Focused Single-Stage Theater (Zero Side-by-Side Clutter) -->
    <main ref="stageWrap" class="stage-theater">
      <UiSkeleton v-if="sourceLoading" :lines="5" height="60px" />

      <template v-else>
        <!-- Stage 01 Top Bar: Client Pills + Source Pills + Filename (No Select Dropdowns!) -->
        <section v-if="step === 0" class="identity-deck">
          <div class="deck-row">
            <div class="deck-group">
              <span class="deck-label">目标客户端</span>
              <div class="pill-group" role="group" aria-label="目标客户端">
                <button
                  type="button"
                  class="seg-pill"
                  :class="{ 'is-active': target === 'clash' }"
                  @click="target = 'clash'"
                >
                  Clash · Mihomo
                </button>
                <button
                  type="button"
                  class="seg-pill"
                  :class="{ 'is-active': target === 'loon' }"
                  @click="target = 'loon'"
                >
                  Loon
                </button>
              </div>
            </div>

            <div class="deck-group is-file">
              <label class="deck-label" for="studio-filename">保存文件名</label>
              <input
                id="studio-filename"
                v-model="activeFilename"
                type="text"
                class="filename-input mono"
                :placeholder="target === 'loon' ? 'loon.conf' : 'my_clash.yaml'"
              />
            </div>
          </div>

          <div class="deck-group">
            <span class="deck-label">模板基底</span>
            <div class="source-pills" role="group" aria-label="模板基底">
              <button
                type="button"
                class="source-chip"
                :class="{ 'is-active': templateSource === 'smart' }"
                @click="selectSource('smart')"
              >
                智能分流 Clash
              </button>
              <button
                type="button"
                class="source-chip"
                :class="{ 'is-active': templateSource === 'default' }"
                @click="selectSource('default')"
              >
                内置默认配置
              </button>
              <button
                type="button"
                class="source-chip"
                :class="{ 'is-active': templateSource === 'blank' }"
                @click="selectSource('blank')"
              >
                新建空白
              </button>
              <button
                v-for="t in templateFiles"
                :key="t.file"
                type="button"
                class="source-chip mono"
                :class="{ 'is-active': templateSource === 'file:' + t.file }"
                @click="selectSource('file:' + t.file)"
              >
                {{ t.file }}
              </button>
            </div>
          </div>
        </section>

        <!-- Stages 01–03 Content -->
        <div v-if="step < 3" class="stage-body">
          <LoonFields v-if="target === 'loon'" :step="step" />
          <ClashFields v-else :step="step" />
        </div>

        <!-- Stage 04: Final Configuration Preview & Save Stage -->
        <section v-else class="final-preview-stage" aria-label="配置预览与保存">
          <div class="preview-summary">
            <dl class="summary-kpis">
              <div>
                <dt>客户端</dt>
                <dd class="mono">{{ target === "loon" ? "Loon (.conf)" : "Clash / Mihomo (.yaml)" }}</dd>
              </div>
              <div>
                <dt>文件名</dt>
                <dd class="mono">{{ activeFilename }}</dd>
              </div>
              <div>
                <dt>策略分组</dt>
                <dd class="mono">{{ groupCount }} 组</dd>
              </div>
              <div>
                <dt>状态</dt>
                <dd :class="{ 'tone-ok': isSavedPreview }">
                  {{ isSavedPreview ? `已保存 (${savedFilename})` : "待保存" }}
                </dd>
              </div>
            </dl>

            <div class="preview-actions">
              <UiButton
                variant="ghost"
                :icon="DocumentCopy"
                :disabled="!preview"
                @click="copyYaml"
              >
                复制配置
              </UiButton>
              <UiButton
                variant="primary"
                :icon="DocumentChecked"
                :loading="loading"
                @click="build"
              >
                生成并保存模板
              </UiButton>
            </div>
          </div>

          <div class="editor-vessel">
            <div class="preview-editor">
              <TemplateMonacoEditor
                :model-value="preview"
                :language="target === 'loon' ? 'ini' : 'yaml'"
                :read-only="true"
                :minimap="false"
              />
            </div>
            <div ref="outline" class="preview-outline" aria-hidden="true"></div>
          </div>
        </section>
      </template>
    </main>

    <!-- Calm Bottom Stage Navigation Bar -->
    <footer class="stage-footer">
      <UiButton
        variant="ghost"
        :icon="ArrowLeft"
        :disabled="step === 0"
        @click="goStep(step - 1)"
      >
        上一步
      </UiButton>

      <span class="footer-progress mono">
        STEP {{ String(step + 1).padStart(2, "0") }} / 04
      </span>

      <UiButton
        v-if="step < 3"
        variant="primary"
        @click="goStep(step + 1)"
      >
        <span>下一步：{{ steps[step + 1]?.label }}</span>
        <el-icon class="el-icon--right"><ArrowRight /></el-icon>
      </UiButton>
      <UiButton
        v-else
        variant="primary"
        :icon="DocumentChecked"
        :loading="loading"
        @click="build"
      >
        生成并保存
      </UiButton>
    </footer>
  </div>
</template>

<style scoped>
.studio-builder {
  display: grid;
  gap: 14px;
  max-width: 1220px;
  min-width: 0;
}

.builder-masthead {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
}

.masthead-left {
  display: flex;
  align-items: baseline;
  gap: 14px;
  flex-wrap: wrap;
}

.title-mask {
  overflow: hidden;
  padding-bottom: 4px;
  margin-bottom: -4px;
}

.gallery-title {
  margin: 0;
  font-family: var(--font-display);
  font-size: clamp(26px, 2.8vw, 36px);
  font-weight: 500;
  line-height: 1.08;
  letter-spacing: -0.04em;
  color: var(--c-ink);
}

.file-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-ink);
}

.file-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.masthead-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.top-rule {
  display: block;
  height: 1px;
  background: var(--c-ink);
  transform-origin: left;
}

/* 4-Stage Stepper Rail */
.stepper-rail {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
}

.step-tab {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  color: var(--c-muted);
  text-align: left;
  cursor: pointer;
  transition:
    transform 100ms ease-out,
    border-color 180ms var(--ease-out),
    background 180ms var(--ease-out),
    color 180ms var(--ease-out);
}

.step-tab:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.step-tab:active {
  transform: scale(0.98);
}

.step-tab.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.step-tab.is-done {
  border-color: color-mix(in srgb, var(--c-ok) 45%, var(--c-line));
  color: var(--c-ink);
}

.step-num {
  font-size: 11.5px;
  opacity: 0.65;
}

.step-tab.is-done .step-num {
  color: var(--c-ok);
  opacity: 1;
}

.step-label {
  font-size: 13.5px;
  font-weight: 500;
}

/* Identity Deck on Stage 01 */
.stage-theater {
  display: grid;
  gap: 14px;
  min-width: 0;
}

.identity-deck {
  display: grid;
  gap: 10px;
  padding: 14px 18px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.deck-row {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 20px;
  flex-wrap: wrap;
}

.deck-group {
  display: grid;
  gap: 7px;
}

.deck-group.is-file {
  flex: 1 1 260px;
  max-width: 380px;
}

.deck-label {
  font-size: 11.5px;
  color: var(--c-muted);
}

.pill-group {
  display: inline-flex;
  padding: 3px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-muted);
  width: fit-content;
}

.seg-pill {
  padding: 6px 14px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12.5px;
  cursor: pointer;
}

.seg-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.filename-input {
  width: 100%;
  padding: 7px 12px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 13px;
  outline: none !important;
}

.filename-input:focus {
  border-color: var(--c-ink);
}

.source-pills {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.source-chip {
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.source-chip:hover {
  border-color: var(--c-ink);
}

.source-chip.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

/* Stage 04 Final Preview */
.final-preview-stage {
  display: grid;
  gap: 16px;
}

.preview-summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
  flex-wrap: wrap;
}

.summary-kpis {
  display: flex;
  align-items: center;
  gap: 28px;
  margin: 0;
  flex-wrap: wrap;
}

.summary-kpis > div {
  display: grid;
  gap: 3px;
}

.summary-kpis dt {
  font-size: 11px;
  color: var(--c-muted);
}

.summary-kpis dd {
  margin: 0;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.tone-ok {
  color: var(--c-ok) !important;
}

.preview-actions {
  display: flex;
  gap: 8px;
}

.editor-vessel {
  position: relative;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  overflow: hidden;
  background: var(--c-surface);
}

.preview-editor {
  height: clamp(320px, 43dvh, 420px);
  min-width: 0;
}

.preview-outline {
  position: absolute;
  inset: 0;
  border: 2px solid var(--c-ok);
  border-radius: 18px;
  pointer-events: none;
  opacity: 0;
}

/* Bottom Navigation Bar */
.stage-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 14px;
  border-top: 1px solid var(--c-line);
}

.footer-progress {
  font-size: 12px;
  color: var(--c-muted);
  letter-spacing: 0.06em;
}

@media (max-width: 820px) {
  .stepper-rail {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
