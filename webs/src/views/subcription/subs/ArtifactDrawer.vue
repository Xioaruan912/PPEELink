<script setup lang="ts">
import { UiDrawer, UiButton, UiSkeleton, EmptyState, StatusDot } from "@/ui";
import { useSubscriptions } from "./useSubscriptions";
const {
  artifactDrawer,
  artifactSub,
  artifactClient,
  artifactLoading,
  artifactError,
  artifactItems,
  loadArtifacts,
  lkgId,
  setLkg,
  fmtTime,
} = useSubscriptions();
const clients = [
  "clash",
  "surge",
  "loon",
  "v2ray",
  "singbox",
  "qx",
  "shadowrocket",
];
</script>
<template>
  <UiDrawer
    v-model="artifactDrawer"
    :title="`${artifactSub?.Name || ''} · 版本与回滚`"
    size="min(820px, 100vw)"
    ><div class="artifact-toolbar">
      <el-select
        v-model="artifactClient"
        aria-label="产物客户端"
        :disabled="artifactLoading"
        @change="loadArtifacts"
        ><el-option
          v-for="client in clients"
          :key="client"
          :label="client"
          :value="client" /></el-select
      ><span class="mono">LKG #{{ lkgId || "暂无" }}</span
      ><UiButton :loading="artifactLoading" @click="loadArtifacts"
        >刷新</UiButton
      >
    </div>
    <p class="field-hint">
      产物内容不可变；回滚仅切换可用版本指针。订阅生成或校验失败时自动回退。
    </p>
    <UiSkeleton v-if="artifactLoading" :lines="4" />
    <EmptyState
      v-else-if="artifactError"
      size="section"
      title="产物加载失败"
      :description="artifactError"
      ><template #action
        ><UiButton @click="loadArtifacts">重新加载</UiButton></template
      ></EmptyState
    >
    <div v-else-if="artifactItems.length">
      <article
        v-for="item in artifactItems"
        :key="item.id"
        class="artifact-item"
      >
        <header>
          <h3 class="mono">#{{ item.id }}</h3>
          <StatusDot
            :state="item.validationStatus === 'valid' ? 'ok' : 'bad'"
            :label="item.validationStatus"
          /><span>{{ item.testStatus }}</span>
        </header>
        <dl class="config-list">
          <dt>输入摘要</dt>
          <dd class="mono">{{ item.inputDigest?.slice(0, 12) }}</dd>
          <dt>模板</dt>
          <dd>{{ item.templateName || "无模板" }}</dd>
          <dt>产物</dt>
          <dd class="mono">
            {{ item.byteSize }} B · {{ item.contentChecksum?.slice(0, 12) }}
          </dd>
          <dt>生成时间</dt>
          <dd class="mono">{{ fmtTime(item.createdAt) }}</dd>
        </dl>
        <StatusDot
          v-if="item.id === lkgId"
          state="ok"
          label="当前 LKG"
        /><UiButton
          v-else-if="item.validationStatus === 'valid'"
          variant="danger"
          @click="setLkg(item)"
          >设为 LKG</UiButton
        >
      </article>
    </div>
    <EmptyState
      v-else
      size="section"
      title="暂无产物"
      description="生成产物或完成发布验证后显示版本。"
  /></UiDrawer>
</template>
<style scoped src="./forms.scss"></style>
