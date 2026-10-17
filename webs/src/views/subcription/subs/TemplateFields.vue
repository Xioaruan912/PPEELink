<script setup lang="ts">
import { UiButton, EmptyState } from "@/ui";
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
  templist,
} = useSubscriptions();
</script>
<template>
  <!-- 分区二：模板与参数 -->
  <el-divider content-position="left">模板与参数</el-divider>
  <EmptyState
    v-if="!templist.length"
    size="section"
    title="暂无本地模板"
    description="可填写远程模板 URL。"
  />
  <el-row :gutter="16">
    <el-col :span="8">
      <el-form-item label="Clash 模板">
        <el-radio-group v-model="clashMode" class="src-tabs" size="small">
          <el-radio-button value="1">本地</el-radio-button>
          <el-radio-button value="2">URL</el-radio-button>
        </el-radio-group>
        <el-select
          v-if="clashMode === '1'"
          v-model="Clash"
          placeholder="选择本地 clash 模板"
          class="full"
        >
          <el-option
            v-for="t in templist"
            :key="t.file"
            :label="t.file"
            :value="'./template/' + t.file"
          />
        </el-select>
        <el-input
          v-else
          v-model="Clash"
          placeholder="粘贴远程模板链接 https://…"
          class="full"
        />
      </el-form-item>
    </el-col>
    <el-col :span="8">
      <el-form-item label="Surge 模板">
        <el-radio-group v-model="surgeMode" class="src-tabs" size="small">
          <el-radio-button value="1">本地</el-radio-button>
          <el-radio-button value="2">URL</el-radio-button>
        </el-radio-group>
        <el-select
          v-if="surgeMode === '1'"
          v-model="Surge"
          placeholder="选择本地 surge 模板"
          class="full"
        >
          <el-option
            v-for="t in templist"
            :key="t.file"
            :label="t.file"
            :value="'./template/' + t.file"
          />
        </el-select>
        <el-input
          v-else
          v-model="Surge"
          placeholder="粘贴远程模板链接 https://…"
          class="full"
        />
      </el-form-item>
    </el-col>
    <el-col :span="8">
      <el-form-item label="Loon 模板">
        <el-radio-group v-model="loonMode" class="src-tabs" size="small">
          <el-radio-button value="1">本地</el-radio-button>
          <el-radio-button value="2">URL</el-radio-button>
        </el-radio-group>
        <el-select
          v-if="loonMode === '1'"
          v-model="Loon"
          placeholder="选择本地 loon 模板"
          class="full"
        >
          <el-option
            v-for="t in templist"
            :key="t.file"
            :label="t.file"
            :value="'./template/' + t.file"
          />
        </el-select>
        <el-input
          v-else
          v-model="Loon"
          placeholder="粘贴远程模板链接 https://…"
          class="full"
        />
      </el-form-item>
    </el-col>
  </el-row>
  <el-row :gutter="16">
    <el-col :span="12">
      <div class="switch-item">
        <div class="switch-main">
          <span class="switch-label">UDP</span>
          <el-switch v-model="udpOn" />
        </div>
        <div class="switch-hint">提升 UDP 转发支持</div>
      </div>
    </el-col>
    <el-col :span="12">
      <div class="switch-item">
        <div class="switch-main">
          <span class="switch-label">跳过证书</span>
          <el-switch v-model="certOn" />
        </div>
        <div class="switch-hint">关闭 TLS 证书校验</div>
      </div>
    </el-col>
  </el-row>
</template>
<style scoped src="./forms.scss"></style>
