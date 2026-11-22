<script setup lang="ts">
import { UiButton } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";

const { appRules, NodesList } = useSubscriptions();

function move(index: number, direction: number) {
  const next = index + direction;
  if (next < 0 || next >= appRules.value.length) return;
  const [rule] = appRules.value.splice(index, 1);
  appRules.value.splice(next, 0, rule);
}
</script>

<template>
  <section class="app-rules">
    <header class="apps-head">
      <div>
        <h3 class="apps-title">按应用代理</h3>
        <p class="apps-note">
          仅 Mihomo / sing-box 1.11+ 完整订阅生效（需配合 TUN），规则按顺序匹配，iOS 不支持。
        </p>
      </div>
      <UiButton
        variant="ghost"
        :disabled="appRules.length >= 64"
        @click="appRules.push({ platform: 'desktop', app: '', target: 'PROXY' })"
      >
        + 添加规则
      </UiButton>
    </header>

    <div v-if="appRules.length" class="rule-list">
      <div v-for="(rule, index) in appRules" :key="index" class="rule-card">
        <div class="rule-top">
          <div class="platform-pills">
            <button
              type="button"
              class="plat-pill"
              :class="{ 'is-active': rule.platform === 'desktop' }"
              @click="rule.platform = 'desktop'"
            >
              桌面进程
            </button>
            <button
              type="button"
              class="plat-pill"
              :class="{ 'is-active': rule.platform === 'android' }"
              @click="rule.platform = 'android'"
            >
              Android 应用
            </button>
          </div>

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
              @click="appRules.splice(index, 1)"
            >
              ×
            </button>
          </div>
        </div>

        <div class="rule-inputs">
          <el-input
            v-model="rule.app"
            :placeholder="
              rule.platform === 'desktop'
                ? '进程名，例如 chrome.exe'
                : '包名，例如 org.telegram.messenger'
            "
          />
          <el-select v-model="rule.target" filterable placeholder="选择出站策略">
            <el-option label="代理（客户端选择）" value="PROXY" />
            <el-option label="直连" value="DIRECT" />
            <el-option label="阻断" value="REJECT" />
            <el-option
              v-for="node in NodesList"
              :key="node.ID"
              :label="node.Name"
              :value="node.Name"
            />
          </el-select>
        </div>
      </div>
    </div>

    <p v-else class="empty-apps">
      暂无应用代理规则，点击右上角「+ 添加规则」按需配置。
    </p>
  </section>
</template>

<style scoped>
.app-rules {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.apps-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.apps-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}

.apps-note {
  margin: 4px 0 0;
  font-size: 12.5px;
  line-height: 1.5;
  color: var(--c-muted);
}

.rule-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.rule-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border-radius: 12px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
}

.rule-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.platform-pills {
  display: inline-flex;
  padding: 2px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
}

.plat-pill {
  padding: 3px 10px;
  border: none;
  border-radius: 999px;
  background: transparent;
  font-size: 12px;
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
  border-radius: 6px;
  border: 1px solid var(--c-line);
  background: transparent;
  color: var(--c-ink);
  font-size: 13px;
  cursor: pointer;
}

.op-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}

.op-btn.is-del:hover {
  border-color: #ef4444;
  color: #dc2626;
}

.rule-inputs {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.rule-inputs :deep(.el-select) {
  width: 100%;
}

.empty-apps {
  margin: 0;
  padding: 24px 0;
  text-align: center;
  font-size: 13px;
  color: var(--c-muted);
  border-top: 1px solid var(--c-line);
}

@media (max-width: 640px) {
  .rule-inputs {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
