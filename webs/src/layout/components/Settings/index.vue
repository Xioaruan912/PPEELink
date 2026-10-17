<template>
  <el-drawer
    v-model="visible"
    title="显示设置"
    size="360px"
    class="workspace-preferences"
  >
    <section class="preference-section">
      <h3>界面外观</h3>
      <div class="appearance-options">
        <button
          v-for="mode in appearances"
          :key="mode.value"
          :class="{ selected: settings.theme === mode.value }"
          :aria-pressed="settings.theme === mode.value"
          @click="settings.changeTheme(mode.value)"
        >
          <span class="appearance-preview" :class="mode.value"
            ><i></i><b></b><em></em></span
          ><span
            >{{ mode.label
            }}<WorkIcon v-if="settings.theme === mode.value" name="check"
          /></span>
        </button>
      </div>
    </section>
    <section class="preference-section">
      <div class="preference-option">
        <div>
          <h3>减少动态效果</h3>
          <p>暂停自动旋转和持续动效。</p>
        </div>
        <el-switch v-model="settings.motionReduced" aria-label="减少动态效果" />
      </div>
      <div class="preference-option">
        <div>
          <h3>工作区水印</h3>
          <p>在内容区域显示 PPEELink 水印。</p>
        </div>
        <el-switch
          v-model="settings.watermarkEnabled"
          aria-label="工作区水印"
        />
      </div>
    </section>
    <section class="preference-section">
      <h3>快捷键</h3>
      <div class="shortcut-reference">
        <span>搜索页面与功能</span><kbd>Ctrl K</kbd>
      </div>
      <div class="shortcut-reference">
        <span>旋转地球</span><span>拖动 / ← →</span>
      </div>
      <div class="shortcut-reference"><span>关闭弹窗</span><kbd>Esc</kbd></div>
    </section>
  </el-drawer>
</template>
<script setup lang="ts">
import { computed } from "vue";
import { useSettingsStore } from "@/store";
import { ThemeEnum } from "@/enums/ThemeEnum";
import WorkIcon from "@/components/workspace/WorkIcon.vue";
const settings = useSettingsStore();
const visible = computed({
  get: () => settings.settingsVisible,
  set: (value) => (settings.settingsVisible = value),
});
const appearances = [
  { value: ThemeEnum.LIGHT, label: "浅色" },
  { value: ThemeEnum.DARK, label: "深色" },
];
</script>
<style scoped>
.preference-section {
  padding: 5px 0 24px;
  margin-bottom: 22px;
  border-bottom: 1px solid var(--ui-border);
}
.preference-section h3 {
  font-size: 16px;
  font-weight: 600;
  margin: 0 0 7px;
}
.preference-section p {
  font-size: 14px;
  color: var(--ui-text-muted);
  margin: 0;
  line-height: 1.7;
}
.appearance-options {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  margin-top: 18px;
}
.appearance-options button {
  padding: 7px;
  border: 1px solid var(--ui-border);
  border-radius: 8px;
  background: none;
  text-align: left;
}
.appearance-options button.selected {
  border-color: var(--ui-accent);
  background: var(--ui-accent-soft);
}
.appearance-preview {
  position: relative;
  display: block;
  height: 85px;
  border-radius: 4px;
  background: #f2f4f1;
  overflow: hidden;
}
.appearance-preview > i {
  display: block;
  height: 13px;
  background: #fcfdfb;
  border-bottom: 1px solid #d8e0d6;
}
.appearance-preview > b {
  position: absolute;
  left: 9px;
  top: 25px;
  width: 28px;
  height: 37px;
  border-radius: 3px;
  background: #fcfdfb;
}
.appearance-preview > em {
  position: absolute;
  right: 13px;
  top: 26px;
  height: 37px;
  width: 37px;
  border-radius: 50%;
  border: 1px solid #9ab196;
  background: #e0e9dc;
}
.appearance-preview.dark {
  background: #14221b;
}
.appearance-preview.dark > i {
  background: #1b2d23;
  border-color: #354b38;
}
.appearance-preview.dark > b {
  background: #243a2c;
}
.appearance-preview.dark > em {
  background: #304633;
  border-color: #516b50;
}
.appearance-options button > span:last-child {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 14px;
  padding: 10px 3px 3px;
}
.appearance-options svg {
  width: 12px;
  height: 12px;
}
.preference-option {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 15px;
  padding-block: 8px;
}
.preference-option + .preference-option {
  margin-top: 20px;
}
.shortcut-reference {
  display: flex;
  justify-content: space-between;
  padding-top: 15px;
  font-size: 14px;
  color: var(--ui-text-secondary);
}
.preference-note {
  display: flex;
  align-items: center;
  gap: 7px;
  font-size: 14px;
  color: var(--ui-text-muted);
}
.preference-note svg {
  height: 13px;
  width: 13px;
}
</style>
