<script setup lang="ts">
import { computed, ref } from "vue";
import { ElMessage } from "element-plus";
import { UiButton } from "@/ui";
import { useSubscriptions, type AppRule } from "./useSubscriptions";
import {
  APP_PRESETS,
  APP_PRESET_CATEGORIES,
  extractClashProxyGroups,
  type AppPreset,
  type AppPresetCategory,
} from "./appPresets";

const { appRules, NodesList, Clash, templist, udpOn } = useSubscriptions();

const activeCategory = ref<AppPresetCategory>("ai");
const expandedIndex = ref<number | null>(null);
const customEntryDraft = ref<Record<number, string>>({});

const RECOMMENDED_PRESET_IDS = [
  "chatgpt",
  "claude",
  "dev-cli",
  "telegram",
  "wechat",
  "qq",
  "baidu-pan",
  "ali-quark-pan",
  "thunder",
];

const hasBrowserRule = computed(() =>
  appRules.value.some((r) =>
    ["chrome", "edge", "firefox", "arc-brave"].includes(r.preset || "")
  )
);

const templateGroups = computed(() => {
  if (!Clash.value.startsWith("./template/")) return [];
  const fname = Clash.value.slice("./template/".length);
  const tpl = templist.value.find((t) => t.file === fname);
  return extractClashProxyGroups(tpl?.text);
});

const categoryPresets = computed(() =>
  APP_PRESETS.filter((p) => p.category === activeCategory.value)
);

function findPresetIndex(presetId: string): number {
  return appRules.value.findIndex((r) => r.preset === presetId);
}

function isPresetActive(presetId: string): boolean {
  return findPresetIndex(presetId) >= 0;
}

function resolveDefaultTarget(preset: AppPreset): string {
  if (preset.preferredGroups?.length && templateGroups.value.length) {
    for (const grp of preset.preferredGroups) {
      if (templateGroups.value.includes(grp)) return grp;
    }
  }
  return preset.defaultTarget;
}

function togglePreset(preset: AppPreset) {
  const existing = findPresetIndex(preset.id);
  if (existing >= 0) {
    appRules.value.splice(existing, 1);
    if (expandedIndex.value === existing) expandedIndex.value = null;
    return;
  }
  if (appRules.value.length >= 64) {
    ElMessage.warning("应用规则最多 64 条");
    return;
  }
  appRules.value.push({
    platform: "all",
    preset: preset.id,
    label: preset.label,
    processes: [...preset.processes],
    packages: preset.packages ? [...preset.packages] : [],
    target: resolveDefaultTarget(preset),
  });
  if (preset.category === "browser") {
    ElMessage.info(
      `已添加「${preset.label}」：该浏览器内全部网站将强制走所选出站，不再按域名分流`
    );
  }
}

function applyRecommendedPresets() {
  let added = 0;
  for (const id of RECOMMENDED_PRESET_IDS) {
    if (isPresetActive(id)) continue;
    const preset = APP_PRESETS.find((p) => p.id === id);
    if (!preset || appRules.value.length >= 64) continue;
    appRules.value.push({
      platform: "all",
      preset: preset.id,
      label: preset.label,
      processes: [...preset.processes],
      packages: preset.packages ? [...preset.packages] : [],
      target: resolveDefaultTarget(preset),
    });
    added++;
  }
  if (!udpOn.value) {
    udpOn.value = true;
  }
  if (added > 0) {
    ElMessage.success(`已添加 ${added} 组常用推荐应用（不含浏览器）并开启 UDP`);
  } else {
    ElMessage.info("常用推荐应用已全部在列表中");
  }
}

function enableUdpNow() {
  udpOn.value = true;
  ElMessage.success("已开启 UDP 转发，记得点击底部保存订阅");
}

function addCustomRule() {
  if (appRules.value.length >= 64) {
    ElMessage.warning("应用规则最多 64 条");
    return;
  }
  appRules.value.push({
    platform: "desktop",
    app: "",
    target: "PROXY",
  });
}

function move(index: number, direction: number) {
  const next = index + direction;
  if (next < 0 || next >= appRules.value.length) return;
  const [rule] = appRules.value.splice(index, 1);
  appRules.value.splice(next, 0, rule);
  if (expandedIndex.value === index) expandedIndex.value = next;
  else if (expandedIndex.value === next) expandedIndex.value = index;
}

function removeRule(index: number) {
  appRules.value.splice(index, 1);
  if (expandedIndex.value === index) expandedIndex.value = null;
  else if (expandedIndex.value !== null && expandedIndex.value > index) {
    expandedIndex.value -= 1;
  }
}

function isBundleRule(rule: AppRule): boolean {
  return !!(
    rule.preset ||
    (rule.processes && rule.processes.length > 0) ||
    (rule.packages && rule.packages.length > 0)
  );
}

function bundleItemCount(rule: AppRule): number {
  return (rule.processes?.length || 0) + (rule.packages?.length || 0);
}

function toggleExpand(index: number) {
  expandedIndex.value = expandedIndex.value === index ? null : index;
}

function removeBundleToken(
  rule: AppRule,
  kind: "processes" | "packages",
  token: string
) {
  if (bundleItemCount(rule) <= 1) {
    ElMessage.warning("至少保留一个进程或包名");
    return;
  }
  const list = rule[kind];
  if (!list) return;
  rule[kind] = list.filter((item) => item !== token);
}

function addBundleToken(index: number, rule: AppRule) {
  const raw = (customEntryDraft.value[index] || "").trim();
  if (!raw) return;
  const isAndroidPkg =
    /^[a-zA-Z][a-zA-Z0-9_]*(\.[a-zA-Z][a-zA-Z0-9_]*)+$/.test(raw) &&
    !/\.(exe|app|bin|sh|bat|cmd)$/i.test(raw);
  const key = isAndroidPkg ? "packages" : "processes";
  const current = rule[key] || [];
  if (!current.includes(raw)) {
    rule[key] = [...current, raw];
  }
  customEntryDraft.value[index] = "";
}
</script>

<template>
  <section class="app-rules">
    <!-- Top Header & Single-Line Reminder -->
    <header class="apps-head">
      <div class="apps-head-main">
        <h3 class="apps-title">应用代理</h3>
        <span class="apps-tip" role="note">
          <i class="tip-dot" aria-hidden="true"></i>
          需开启客户端 TUN 才能接管全部进程（iOS 不支持）
        </span>
        <button
          v-if="appRules.length && !udpOn"
          type="button"
          class="apps-tip is-warn-btn"
          @click="enableUdpNow"
        >
          <i class="tip-dot is-warn" aria-hidden="true"></i>
          建议开启 UDP 转发 · 点此开启
        </button>
      </div>
      <div class="apps-head-ops">
        <UiButton
          size="sm"
          variant="ghost"
          :disabled="appRules.length >= 64"
          @click="applyRecommendedPresets"
        >
          推荐配置
        </UiButton>
        <UiButton
          size="sm"
          variant="ghost"
          :disabled="appRules.length >= 64"
          @click="addCustomRule"
        >
          + 自定义进程
        </UiButton>
      </div>
    </header>

    <!-- Apple Tactile Preset Picker -->
    <div class="preset-deck">
      <nav class="cat-rail" role="tablist" aria-label="常用应用分类">
        <button
          v-for="cat in APP_PRESET_CATEGORIES"
          :key="cat.key"
          type="button"
          role="tab"
          class="cat-pill"
          :aria-selected="activeCategory === cat.key"
          :class="{ 'is-active': activeCategory === cat.key }"
          @click="activeCategory = cat.key"
        >
          {{ cat.label }}
        </button>
      </nav>

      <div
        v-if="activeCategory === 'browser' || hasBrowserRule"
        class="browser-warn"
        role="note"
      >
        <i class="tip-dot is-warn" aria-hidden="true"></i>
        整浏览器代理会覆盖域名智能分流（国内网站也会走代理），日常浏览建议不选浏览器
      </div>

      <div class="preset-chips">
        <button
          v-for="preset in categoryPresets"
          :key="preset.id"
          type="button"
          class="preset-chip"
          :class="{
            'is-active': isPresetActive(preset.id),
            'is-direct': preset.defaultTarget === 'DIRECT',
          }"
          :aria-pressed="isPresetActive(preset.id)"
          @click="togglePreset(preset)"
        >
          <span v-if="isPresetActive(preset.id)" class="chip-check" aria-hidden="true">✓</span>
          <span>{{ preset.label }}</span>
        </button>
      </div>
    </div>

    <!-- Active Rules Ledger -->
    <div v-if="appRules.length" class="rule-list">
      <article
        v-for="(rule, index) in appRules"
        :key="rule.preset ? `preset-${rule.preset}` : `custom-${index}`"
        class="rule-card"
      >
        <!-- Preset Bundle Row (Zero Exe Clutter By Default) -->
        <template v-if="isBundleRule(rule)">
          <div class="bundle-row">
            <div class="bundle-identity">
              <strong class="bundle-name">{{ rule.label || rule.preset }}</strong>
              <button
                type="button"
                class="count-pill mono"
                :aria-expanded="expandedIndex === index"
                @click="toggleExpand(index)"
              >
                {{ bundleItemCount(rule) }} 项进程
                <span class="chev" :class="{ 'is-open': expandedIndex === index }">▾</span>
              </button>
            </div>

            <div class="bundle-controls">
              <el-select
                v-model="rule.target"
                filterable
                size="small"
                placeholder="选择出站"
                class="target-select"
              >
                <el-option-group label="基础策略">
                  <el-option label="代理（客户端选节点）" value="PROXY" />
                  <el-option label="直连 (DIRECT)" value="DIRECT" />
                  <el-option label="阻断 (REJECT)" value="REJECT" />
                </el-option-group>
                <el-option-group
                  v-if="templateGroups.length"
                  label="当前模板策略组"
                >
                  <el-option
                    v-for="grp in templateGroups"
                    :key="`grp-${grp}`"
                    :label="grp"
                    :value="grp"
                  />
                </el-option-group>
                <el-option-group v-if="NodesList.length" label="指定节点">
                  <el-option
                    v-for="node in NodesList"
                    :key="node.ID"
                    :label="node.Name"
                    :value="node.Name"
                  />
                </el-option-group>
              </el-select>

              <div class="rule-ops">
                <button
                  type="button"
                  class="op-btn"
                  :disabled="index === 0"
                  aria-label="上移"
                  @click="move(index, -1)"
                >
                  ↑
                </button>
                <button
                  type="button"
                  class="op-btn"
                  :disabled="index === appRules.length - 1"
                  aria-label="下移"
                  @click="move(index, 1)"
                >
                  ↓
                </button>
                <button
                  type="button"
                  class="op-btn is-del"
                  aria-label="删除"
                  @click="removeRule(index)"
                >
                  ×
                </button>
              </div>
            </div>
          </div>

          <!-- Optional Expandable Process Drawer -->
          <div v-if="expandedIndex === index" class="bundle-detail">
            <div class="token-list mono">
              <span
                v-for="p in rule.processes || []"
                :key="`p-${p}`"
                class="token-chip"
              >
                {{ p }}
                <button
                  type="button"
                  aria-label="移除"
                  @click="removeBundleToken(rule, 'processes', p)"
                >
                  ×
                </button>
              </span>
              <span
                v-for="pkg in rule.packages || []"
                :key="`pkg-${pkg}`"
                class="token-chip is-android"
              >
                {{ pkg }}
                <button
                  type="button"
                  aria-label="移除"
                  @click="removeBundleToken(rule, 'packages', pkg)"
                >
                  ×
                </button>
              </span>
            </div>
            <div class="token-add">
              <input
                v-model="customEntryDraft[index]"
                type="text"
                class="token-input mono"
                placeholder="追加进程名或 Android 包名，回车添加…"
                @keydown.enter.prevent="addBundleToken(index, rule)"
              />
              <button
                type="button"
                class="token-add-btn"
                @click="addBundleToken(index, rule)"
              >
                追加
              </button>
            </div>
          </div>
        </template>

        <!-- Custom Single-Process Row -->
        <template v-else>
          <div class="custom-row">
            <div class="platform-pills">
              <button
                type="button"
                class="plat-pill"
                :class="{ 'is-active': rule.platform === 'desktop' }"
                @click="rule.platform = 'desktop'"
              >
                桌面
              </button>
              <button
                type="button"
                class="plat-pill"
                :class="{ 'is-active': rule.platform === 'android' }"
                @click="rule.platform = 'android'"
              >
                Android
              </button>
            </div>

            <el-input
              v-model="rule.app"
              size="small"
              class="custom-app-input"
              :placeholder="
                rule.platform === 'desktop'
                  ? '进程名，如 chrome.exe'
                  : '包名，如 org.telegram.messenger'
              "
            />

            <el-select
              v-model="rule.target"
              filterable
              size="small"
              placeholder="选择出站"
              class="target-select"
            >
              <el-option-group label="基础策略">
                <el-option label="代理（客户端选节点）" value="PROXY" />
                <el-option label="直连 (DIRECT)" value="DIRECT" />
                <el-option label="阻断 (REJECT)" value="REJECT" />
              </el-option-group>
              <el-option-group
                v-if="templateGroups.length"
                label="当前模板策略组"
              >
                <el-option
                  v-for="grp in templateGroups"
                  :key="`cgrp-${grp}`"
                  :label="grp"
                  :value="grp"
                />
              </el-option-group>
              <el-option-group v-if="NodesList.length" label="指定节点">
                <el-option
                  v-for="node in NodesList"
                  :key="node.ID"
                  :label="node.Name"
                  :value="node.Name"
                />
              </el-option-group>
            </el-select>

            <div class="rule-ops">
              <button
                type="button"
                class="op-btn"
                :disabled="index === 0"
                aria-label="上移"
                @click="move(index, -1)"
              >
                ↑
              </button>
              <button
                type="button"
                class="op-btn"
                :disabled="index === appRules.length - 1"
                aria-label="下移"
                @click="move(index, 1)"
              >
                ↓
              </button>
              <button
                type="button"
                class="op-btn is-del"
                aria-label="删除"
                @click="removeRule(index)"
              >
                ×
              </button>
            </div>
          </div>
        </template>
      </article>
    </div>

    <div v-else class="empty-apps">
      点选上方常用应用一键配置，或添加自定义进程
    </div>
  </section>
</template>

<style scoped>
.app-rules {
  display: grid;
  gap: 16px;
}

.apps-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.apps-head-main {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.apps-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--c-ink);
}

.apps-head-ops {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.apps-tip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 3px 10px;
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11.5px;
}

.apps-tip.is-warn-btn {
  border: 1px solid color-mix(in srgb, var(--c-warn, #d97706) 35%, var(--c-line));
  background: color-mix(in srgb, var(--c-warn, #d97706) 10%, var(--c-surface));
  color: var(--c-warn-ink, #b45309);
  cursor: pointer;
  transition: transform 100ms ease-out;
}

.apps-tip.is-warn-btn:active {
  transform: scale(0.97);
}

.tip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--c-ok);
  flex: none;
}

.tip-dot.is-warn {
  background: var(--c-warn, #d97706);
}

.browser-warn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 6px 11px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--c-warn, #d97706) 9%, var(--c-surface-2));
  color: var(--c-warn-ink, #b45309);
  font-size: 11.5px;
  width: fit-content;
}

/* Tactile Apple Preset Picker */
.preset-deck {
  display: grid;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}

.cat-rail {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-wrap: wrap;
}

.cat-pill {
  padding: 5px 12px;
  border: 1px solid transparent;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition:
    background 160ms ease,
    color 160ms ease,
    border-color 160ms ease;
}

.cat-pill:hover {
  color: var(--c-ink);
  border-color: var(--c-line);
}

.cat-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.preset-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.preset-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 6px 13px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12.5px;
  font-weight: 500;
  cursor: pointer;
  transition:
    transform 100ms ease-out,
    background 160ms ease,
    border-color 160ms ease,
    color 160ms ease;
}

.preset-chip:hover {
  border-color: var(--c-ink);
}

.preset-chip:active {
  transform: scale(0.97);
}

.preset-chip.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.chip-check {
  font-size: 11px;
  font-weight: 700;
}

/* Active Rules Ledger */
.rule-list {
  display: grid;
  gap: 10px;
}

.rule-card {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  border-radius: 14px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
}

.bundle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.bundle-identity {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.bundle-name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--c-ink);
}

.count-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface-2);
  color: var(--c-muted);
  font-size: 11px;
  cursor: pointer;
  transition: border-color 140ms ease, color 140ms ease;
}

.count-pill:hover {
  border-color: var(--c-ink);
  color: var(--c-ink);
}

.chev {
  display: inline-block;
  transition: transform 160ms ease;
}

.chev.is-open {
  transform: rotate(180deg);
}

.bundle-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.target-select {
  width: 176px;
}

.bundle-detail {
  display: grid;
  gap: 10px;
  padding-top: 10px;
  border-top: 1px solid var(--c-line);
}

.token-list {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.token-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 8px;
  border-radius: 8px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 11.5px;
}

.token-chip.is-android {
  background: color-mix(in srgb, var(--c-ok) 11%, transparent);
  color: var(--c-ok);
}

.token-chip button {
  border: 0;
  padding: 0 2px;
  background: transparent;
  color: inherit;
  opacity: 0.6;
  cursor: pointer;
}

.token-chip button:hover {
  opacity: 1;
}

.token-add {
  display: flex;
  align-items: center;
  gap: 8px;
}

.token-input {
  flex: 1;
  min-width: 0;
  height: 30px;
  padding: 0 10px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  background: var(--c-surface-2);
  color: var(--c-ink);
  font-size: 12px;
  outline: none;
}

.token-input:focus {
  border-color: var(--c-ink);
}

.token-add-btn {
  height: 30px;
  padding: 0 12px;
  border: 1px solid var(--c-line);
  border-radius: 8px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.token-add-btn:hover {
  border-color: var(--c-ink);
}

/* Custom Single-Process Row */
.custom-row {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) 176px auto;
  align-items: center;
  gap: 8px;
}

.platform-pills {
  display: inline-flex;
  padding: 2px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface-2);
}

.plat-pill {
  padding: 3px 9px;
  border: none;
  border-radius: 999px;
  background: transparent;
  font-size: 11.5px;
  color: var(--c-muted);
  cursor: pointer;
}

.plat-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.rule-ops {
  display: inline-flex;
  gap: 4px;
}

.op-btn {
  width: 26px;
  height: 26px;
  border-radius: 7px;
  border: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
  transition: border-color 140ms ease, color 140ms ease;
}

.op-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.op-btn.is-del:hover {
  border-color: var(--c-bad);
  color: var(--c-bad);
}

.empty-apps {
  padding: 24px 12px;
  text-align: center;
  font-size: 12.5px;
  color: var(--c-muted);
  border: 1px dashed var(--c-line);
  border-radius: 14px;
}

@media (max-width: 640px) {
  .custom-row {
    grid-template-columns: minmax(0, 1fr);
  }
  .target-select {
    width: 100%;
  }
  .bundle-controls {
    width: 100%;
    justify-content: space-between;
  }
}
</style>
