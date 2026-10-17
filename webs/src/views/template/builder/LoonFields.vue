<script setup lang="ts">
import { Delete, Plus } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useTemplateBuilder } from "./useTemplateBuilder";
defineProps<{ step: string | number }>();
const { loonForm } = useTemplateBuilder();
</script>
<template>
  <el-form label-position="top" class="builder-form">
    <section v-show="step === 0" class="builder-section">
      <h2>General 基础配置</h2>
      <el-form-item label="IP 模式">
        <el-select v-model="loonForm.general.ip_mode" class="full">
          <el-option label="仅 IPv4 (v4-only)" value="v4-only" />
          <el-option label="双栈 (dual)" value="dual" />
          <el-option label="优先 IPv4" value="ipv4-preferred" />
          <el-option label="优先 IPv6" value="ipv6-preferred" />
        </el-select>
      </el-form-item>
      <el-form-item label="DNS 服务器">
        <el-input
          v-model="loonForm.general.dns_server"
          placeholder="system,1.1.1.1"
        />
      </el-form-item>
      <el-form-item label="SNI 嗅探">
        <el-switch v-model="loonForm.general.sni_sniffing" />
      </el-form-item>
      <el-form-item label="禁用 STUN">
        <el-switch v-model="loonForm.general.disable_stun" />
      </el-form-item>
      <el-form-item label="局域网访问">
        <el-switch v-model="loonForm.general.allow_wifi_access" />
      </el-form-item>
      <el-row :gutter="12">
        <el-col :span="12" :xs="24"
          ><el-form-item label="HTTP 端口"
            ><el-input-number
              v-model="loonForm.general.wifi_http_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
        <el-col :span="12" :xs="24"
          ><el-form-item label="SOCKS5 端口"
            ><el-input-number
              v-model="loonForm.general.wifi_socks_port"
              :min="1"
              :max="65535"
              class="full" /></el-form-item
        ></el-col>
      </el-row>
      <el-form-item label="测速超时(秒)">
        <el-input-number
          v-model="loonForm.general.test_timeout"
          :min="1"
          :max="60"
          class="full"
        />
      </el-form-item>
      <el-form-item label="网络测试 URL">
        <el-input v-model="loonForm.general.internet_test_url" />
      </el-form-item>
      <el-form-item label="节点测速 URL">
        <el-input v-model="loonForm.general.proxy_test_url" />
      </el-form-item>
      <el-form-item label="资源解析器">
        <el-input v-model="loonForm.general.resource_parser" />
      </el-form-item>
      <el-form-item label="GeoIP URL">
        <el-input v-model="loonForm.general.geoip_url" />
      </el-form-item>
      <el-form-item label="ASN URL">
        <el-input v-model="loonForm.general.ipasn_url" />
      </el-form-item>
      <el-form-item label="UDP 兜底">
        <el-select v-model="loonForm.general.udp_fallback_mode" class="full">
          <el-option label="DIRECT" value="DIRECT" />
          <el-option label="REJECT" value="REJECT" />
        </el-select>
      </el-form-item>
      <el-form-item label="域名拒绝阶段">
        <el-select v-model="loonForm.general.domain_reject_mode" class="full">
          <el-option label="DNS" value="DNS" />
          <el-option label="Request" value="Request" />
        </el-select>
      </el-form-item>
      <el-form-item label="DNS 拒绝方式">
        <el-select v-model="loonForm.general.dns_reject_mode" class="full">
          <el-option label="回环 IP" value="LoopbackIP" />
          <el-option label="空响应" value="NOANSWER" />
          <el-option label="NXDomain" value="NXDOMAIN" />
        </el-select>
      </el-form-item>
    </section>

    <section v-show="step === 1" class="builder-section">
      <h2>节点筛选 (Remote Filter)</h2>
      <div v-for="(f, i) in loonForm.filters" :key="i" class="loon-row">
        <el-input
          v-model="f.name"
          aria-label="筛选名称"
          placeholder="筛选名（如 香港）"
          class="loon-name"
        />
        <el-input
          v-model="f.regex"
          aria-label="节点筛选正则"
          placeholder="NameRegex 正则"
          class="loon-regex"
        />
        <RowIconButton
          :icon="Delete"
          :label="`删除筛选 ${f.name || i + 1}`"
          @click="loonForm.filters.splice(i, 1)"
        />
      </div>
      <UiButton
        :icon="Plus"
        @click="loonForm.filters.push({ name: '', regex: '' })"
        >添加筛选</UiButton
      >
    </section>

    <section v-show="step === 1" class="builder-section">
      <h2>策略组 (Proxy Group)</h2>
      <div v-for="(gr, i) in loonForm.groups" :key="i" class="loon-group-row">
        <el-input
          v-model="gr.name"
          aria-label="策略组名称"
          placeholder="组名（如 US）"
          class="loon-name"
        />
        <el-select v-model="gr.type" aria-label="策略组类型" class="loon-type">
          <el-option label="select" value="select" />
          <el-option label="url-test" value="url-test" />
          <el-option label="fallback" value="fallback" />
        </el-select>
        <el-input
          v-model="gr.policies"
          aria-label="策略列表"
          placeholder="策略列表（逗号分隔）"
          class="loon-regex"
        />
        <RowIconButton
          :icon="Delete"
          :label="`删除策略组 ${gr.name || i + 1}`"
          @click="loonForm.groups.splice(i, 1)"
        />
      </div>
      <UiButton
        :icon="Plus"
        @click="
          loonForm.groups.push({
            name: '',
            type: 'select',
            policies: '',
            url: '',
            img_url: '',
          })
        "
        >添加组</UiButton
      >
    </section>

    <section v-show="step === 2" class="builder-section">
      <h2>远程规则 (Remote Rule)</h2>
      <el-input
        v-model="loonForm.remote_rules"
        type="textarea"
        :rows="8"
        placeholder="每行一条：https://xxx, policy=组名, tag=标签, enabled=true"
      />
    </section>

    <section v-show="step === 2" class="builder-section">
      <h2>插件 (Plugin)</h2>
      <el-input
        v-model="loonForm.plugins"
        type="textarea"
        :rows="6"
        placeholder="每行一条：https://xxx.lpx, enabled=true"
      />
    </section>

    <section v-show="step === 2" class="builder-section">
      <h2>本地规则 (Rule)</h2>
      <el-input
        v-model="loonForm.rules"
        type="textarea"
        :rows="4"
        placeholder="本地规则，自动追加 DOMAIN-SUFFIX,local,DIRECT / GEOIP,CN,DIRECT / FINAL,FINAL"
      />
    </section>
  </el-form>
</template>
<style scoped lang="scss" src="./fields.scss"></style>
