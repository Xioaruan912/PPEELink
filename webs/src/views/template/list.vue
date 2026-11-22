<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from "vue";
import gsap from "gsap";
import { Back, DocumentChecked, Plus } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import { provideTemplates } from "./list/useTemplates";
import TemplateList from "./list/TemplateList.vue";
import EditorWorkspace from "./list/EditorWorkspace.vue";

defineOptions({ name: "TemplateListPage" });

const rootEl = ref<HTMLElement>();
let gsapCtx: gsap.Context | null = null;

const {
  tableData,
  dialogVisible,
  TempTitle,
  Tempname,
  dirty,
  saving,
  handleAddTemp,
  closeEditor,
  addtemp,
} = provideTemplates();

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
  <div ref="rootEl" class="templates-gallery">
    <!-- Pure Gallery Masthead -->
    <header class="templates-masthead">
      <div class="masthead-left">
        <div class="title-mask">
          <h1 class="gallery-title" data-title-reveal>
            {{ dialogVisible ? "模板编辑器" : "配置模板" }}
          </h1>
        </div>
        <span v-if="!dialogVisible" class="meta-pill mono">
          {{ tableData.length }} 份模板
        </span>
        <span
          v-else
          class="meta-pill mono"
          :class="{ 'is-dirty': dirty }"
        >
          <i aria-hidden="true"></i>
          {{ Tempname || TempTitle }}{{ dirty ? " · 未保存" : "" }}
        </span>
      </div>

      <div class="masthead-actions">
        <template v-if="dialogVisible">
          <UiButton
            variant="ghost"
            :icon="Back"
            :disabled="saving"
            @click="closeEditor"
          >
            返回列表
          </UiButton>
          <UiButton
            variant="primary"
            :icon="DocumentChecked"
            :loading="saving"
            @click="addtemp"
          >
            {{ TempTitle === "添加模板" ? "保存模板" : "保存修改" }}
          </UiButton>
        </template>
        <UiButton
          v-else
          variant="primary"
          :icon="Plus"
          @click="handleAddTemp"
        >
          新建模板
        </UiButton>
      </div>
    </header>

    <span class="top-rule" data-top-rule aria-hidden="true"></span>

    <Transition name="view-swap" mode="out-in">
      <EditorWorkspace v-if="dialogVisible" key="editor" />
      <TemplateList v-else key="list" />
    </Transition>
  </div>
</template>

<style scoped>
.templates-gallery {
  display: grid;
  gap: 20px;
  max-width: 1180px;
  min-width: 0;
}

.templates-masthead {
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

.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 5px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  font-size: 12px;
  color: var(--c-muted);
}

.meta-pill i {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
}

.meta-pill.is-dirty {
  color: var(--c-warn);
  border-color: color-mix(in srgb, var(--c-warn) 45%, var(--c-line));
}

.meta-pill.is-dirty i {
  background: var(--c-warn);
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

.view-swap-enter-active,
.view-swap-leave-active {
  transition:
    opacity 200ms var(--ease-out),
    transform 240ms cubic-bezier(0.16, 1, 0.3, 1);
}

.view-swap-enter-from {
  opacity: 0;
  transform: translateY(8px);
}

.view-swap-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

@media (prefers-reduced-motion: reduce) {
  .view-swap-enter-active,
  .view-swap-leave-active {
    transition: none;
  }
}
</style>
