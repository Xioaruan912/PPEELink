<script setup lang="ts">
import { useSubscriptions } from "./useSubscriptions";

const {
  Clash,
  Surge,
  Loon,
  clashMode,
  surgeMode,
  loonMode,
  udpOn,
  certOn,
  providerOn,
  templist,
} = useSubscriptions();
</script>

<template>
  <div class="tpl-layout">
    <!-- Client Template Rows -->
    <section class="tpl-section">
      <header class="sec-head">
        <h3 class="sec-title">客户端模板</h3>
        <p class="sec-sub">为各客户端指定本地模板或远程模板链接</p>
      </header>

      <div class="client-rows">
        <!-- Clash -->
        <div class="client-row">
          <div class="client-meta">
            <strong class="client-name">Clash</strong>
            <div class="mode-pills" role="group" aria-label="Clash 模板来源">
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': clashMode === '1' }"
                @click="clashMode = '1'"
              >
                本地
              </button>
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': clashMode === '2' }"
                @click="clashMode = '2'"
              >
                URL
              </button>
            </div>
          </div>
          <div class="client-input">
            <el-select
              v-if="clashMode === '1'"
              v-model="Clash"
              placeholder="选择本地 Clash 模板"
              clearable
              class="full"
            >
              <el-option
                v-for="t in templist"
                :key="t.file"
                :label="
                  t.file === 'clash-smart.yaml'
                    ? 'clash-smart.yaml（智能分流默认）'
                    : t.file
                "
                :value="'./template/' + t.file"
              />
            </el-select>
            <el-input
              v-else
              v-model="Clash"
              placeholder="粘贴远程模板链接 https://…"
              clearable
            />
          </div>
        </div>

        <!-- Surge -->
        <div class="client-row">
          <div class="client-meta">
            <strong class="client-name">Surge</strong>
            <div class="mode-pills" role="group" aria-label="Surge 模板来源">
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': surgeMode === '1' }"
                @click="surgeMode = '1'"
              >
                本地
              </button>
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': surgeMode === '2' }"
                @click="surgeMode = '2'"
              >
                URL
              </button>
            </div>
          </div>
          <div class="client-input">
            <el-select
              v-if="surgeMode === '1'"
              v-model="Surge"
              placeholder="选择本地 Surge 模板"
              clearable
              class="full"
            >
              <el-option
                v-for="t in templist"
                :key="t.file"
                :label="
                  t.file === 'clash-smart.yaml'
                    ? 'clash-smart.yaml（智能分流默认）'
                    : t.file
                "
                :value="'./template/' + t.file"
              />
            </el-select>
            <el-input
              v-else
              v-model="Surge"
              placeholder="粘贴远程模板链接 https://…"
              clearable
            />
          </div>
        </div>

        <!-- Loon -->
        <div class="client-row">
          <div class="client-meta">
            <strong class="client-name">Loon</strong>
            <div class="mode-pills" role="group" aria-label="Loon 模板来源">
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': loonMode === '1' }"
                @click="loonMode = '1'"
              >
                本地
              </button>
              <button
                type="button"
                class="mode-pill"
                :class="{ 'is-active': loonMode === '2' }"
                @click="loonMode = '2'"
              >
                URL
              </button>
            </div>
          </div>
          <div class="client-input">
            <el-select
              v-if="loonMode === '1'"
              v-model="Loon"
              placeholder="选择本地 Loon 模板"
              clearable
              class="full"
            >
              <el-option
                v-for="t in templist"
                :key="t.file"
                :label="
                  t.file === 'clash-smart.yaml'
                    ? 'clash-smart.yaml（智能分流默认）'
                    : t.file
                "
                :value="'./template/' + t.file"
              />
            </el-select>
            <el-input
              v-else
              v-model="Loon"
              placeholder="粘贴远程模板链接 https://…"
              clearable
            />
          </div>
        </div>
      </div>
    </section>

    <!-- Protocol Toggles -->
    <section class="tpl-section">
      <header class="sec-head">
        <h3 class="sec-title">协议与下发选项</h3>
      </header>

      <div class="toggle-list">
        <div class="toggle-row">
          <div class="toggle-info">
            <strong>UDP 转发</strong>
            <span>为所有支持 UDP 的节点开启转发支持</span>
          </div>
          <el-switch v-model="udpOn" />
        </div>

        <div class="toggle-row">
          <div class="toggle-info">
            <strong>跳过证书校验</strong>
            <span>关闭 TLS 证书验证（仅自签证书节点需要）</span>
          </div>
          <el-switch v-model="certOn" />
        </div>

        <div class="toggle-row">
          <div class="toggle-info">
            <strong>Clash 节点集合（proxy-provider）</strong>
            <span>客户端按小时自动刷新节点列表，无需重新导入整份配置</span>
          </div>
          <el-switch v-model="providerOn" />
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.tpl-layout {
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.tpl-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.sec-head {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.sec-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--c-ink);
}

.sec-sub {
  margin: 0;
  font-size: 12.5px;
  color: var(--c-muted);
}

.client-rows {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--c-line);
}

.client-row {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr);
  align-items: center;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--c-line);
}

.client-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.client-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.mode-pills {
  display: inline-flex;
  padding: 2px;
  border-radius: 999px;
  border: 1px solid var(--c-line);
  background: var(--c-surface);
}

.mode-pill {
  padding: 3px 9px;
  border: none;
  border-radius: 999px;
  background: transparent;
  font-size: 12px;
  color: var(--c-muted);
  cursor: pointer;
  transition: all 0.16s ease;
}

.mode-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

.client-input .full,
.client-input :deep(.el-select) {
  width: 100%;
}

.toggle-list {
  display: flex;
  flex-direction: column;
  border-top: 1px solid var(--c-line);
}

.toggle-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 0;
  border-bottom: 1px solid var(--c-line);
}

.toggle-info {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.toggle-info strong {
  font-size: 14px;
  font-weight: 500;
  color: var(--c-ink);
}

.toggle-info span {
  font-size: 12.5px;
  color: var(--c-muted);
}

@media (max-width: 640px) {
  .client-row {
    grid-template-columns: minmax(0, 1fr);
    gap: 10px;
  }
}
</style>
