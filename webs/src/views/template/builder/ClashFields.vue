<script setup lang="ts">
import { Delete, Plus } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useTemplateBuilder } from "./useTemplateBuilder";
defineProps<{ step: string | number }>();
const { form, rulesText, addGroup, removeGroup, addRP, removeRP, router } =
  useTemplateBuilder();
</script>
<template>
  <el-form label-position="top" class="builder-form">
    <!-- 端口区块 -->

    <section v-show="step === 0" class="builder-section">
      <h2>端口配置</h2>
      <el-row :gutter="12">
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="HTTP 端口"
            ><el-input-number
              v-model="form.port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="SOCKS 端口"
            ><el-input-number
              v-model="form.socks_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="Mixed 端口"
            ><el-input-number
              v-model="form.mixed_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="Redir 端口"
            ><el-input-number
              v-model="form.redir_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="TProxy 端口"
            ><el-input-number
              v-model="form.tproxy_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="DNS 端口"
            ><el-input-number
              v-model="form.dns_listen_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
      </el-row>
      <el-form-item label="外部控制">
        <el-input
          v-model="form.external_controller"
          placeholder="0.0.0.0:9090"
          class="full"
        />
      </el-form-item>
      <el-form-item label="端口偏移联动">
        <el-switch v-model="form.port_offset" />
        <span class="field-hint"
          >开启后修改主端口，mixed/redir/tproxy/socks 端口自动偏移</span
        >
      </el-form-item>
    </section>

    <section v-show="step === 0" class="builder-section">
      <h2>基础配置</h2>
      <el-row :gutter="12">
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="允许局域网"
            ><el-switch v-model="form.allow_lan" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="IPv6"
            ><el-switch v-model="form.ipv6" /></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="UDP"
            ><el-switch v-model="form.udp" /></el-form-item
        ></el-col>
      </el-row>
      <el-row :gutter="12">
        <el-col :span="8" :xs="24" :sm="12">
          <el-form-item label="模式">
            <el-select v-model="form.mode" class="full">
              <el-option label="Rule（规则）" value="rule" />
              <el-option label="Global（全局）" value="global" />
              <el-option label="Direct（直连）" value="direct" />
            </el-select>
          </el-form-item>
        </el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="日志级别"
            ><el-select v-model="form.log_level" class="full"
              ><el-option label="info" value="info" /><el-option
                label="debug"
                value="debug" /><el-option
                label="warning"
                value="warning" /><el-option
                label="error"
                value="error" /></el-select></el-form-item
        ></el-col>
        <el-col :span="8" :xs="24" :sm="12"
          ><el-form-item label="客户端指纹"
            ><el-input
              v-model="form.global_client_fingerprint"
              placeholder="chrome"
              class="full" /></el-form-item
        ></el-col>
      </el-row>
    </section>

    <section v-show="step === 2" class="builder-section">
      <h2>分组与规则</h2>
      <!-- 策略分组 -->
      <el-form-item label="策略分组">
        <div class="group-list">
          <div v-for="(g, i) in form.groups" :key="i" class="group-row">
            <el-input
              v-model="g.name"
              aria-label="分组名称"
              placeholder="分组名（如 AI）"
              class="g-name"
            />
            <el-select v-model="g.type" aria-label="分组类型" class="g-type">
              <el-option label="select 手动选择" value="select" />
              <el-option label="url-test 自动测速" value="url-test" />
              <el-option label="fallback 故障转移" value="fallback" />
            </el-select>
            <el-input
              v-model="g.filter"
              aria-label="节点筛选正则"
              placeholder="filter 正则（如 (?i)US|美国）"
              class="g-filter"
            />
            <el-checkbox
              v-model="g.include_all_providers"
              aria-label="匹配全部节点"
              title="匹配全部节点"
              >全部</el-checkbox
            >
            <RowIconButton
              :icon="Delete"
              :label="`删除分组 ${g.name || i + 1}`"
              @click="removeGroup(i)"
            />
          </div>
          <UiButton :icon="Plus" @click="addGroup">添加分组</UiButton>
        </div>
        <div class="group-hint">
          类型说明：<b>select</b>=手动选择；<b>url-test</b>=自动测速；<b>fallback</b>=故障转移。filter
          为正则，匹配节点自动填入。
        </div>
      </el-form-item>

      <!-- 规则集 -->
      <el-form-item label="规则集 (rule-providers)">
        <div class="group-list">
          <div v-for="(rp, i) in form.rule_providers" :key="i" class="rp-row">
            <el-input
              v-model="rp.name"
              aria-label="规则集名称"
              placeholder="名称（如 AI）"
              class="rp-name"
            />
            <el-input
              v-model="rp.url"
              aria-label="规则集 URL"
              placeholder="规则集 URL"
              class="rp-url"
            />
            <el-select
              v-model="rp.behavior"
              aria-label="规则集行为"
              class="rp-behavior"
            >
              <el-option label="classical" value="classical" />
              <el-option label="domain" value="domain" />
              <el-option label="ipcidr" value="ipcidr" />
            </el-select>
            <RowIconButton
              :icon="Delete"
              :label="`删除规则集 ${rp.name || i + 1}`"
              @click="removeRP(i)"
            />
          </div>
          <UiButton :icon="Plus" @click="addRP">添加规则集</UiButton>
          <UiButton variant="ghost" @click="router.push('/template/rules')"
            >从规则中心添加</UiButton
          >
        </div>
      </el-form-item>

      <!-- 规则文本 -->
      <el-form-item label="规则 (rules)">
        <el-input
          v-model="rulesText"
          type="textarea"
          :rows="8"
          placeholder="每行一条规则，如：RULE-SET,AI,AI / GEOIP,CN,DIRECT / MATCH,日常使用"
        />
      </el-form-item></section
  ></el-form>
</template>
<style scoped lang="scss" src="./fields.scss"></style>
