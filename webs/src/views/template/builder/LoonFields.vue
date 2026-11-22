<script setup lang="ts">
import { ref } from "vue";
import { Delete, Plus, Setting } from "@element-plus/icons-vue";
import { UiButton } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useTemplateBuilder } from "./useTemplateBuilder";

defineProps<{ step: string | number }>();
const { loonForm } = useTemplateBuilder();

const showAdvancedUrls = ref(false);
const ruleTab = ref<"remote" | "plugin" | "local">("remote");

const ipModes = [
  { label: "仅 IPv4", value: "v4-only" },
  { label: "双栈 (Dual)", value: "dual" },
  { label: "优先 IPv4", value: "ipv4-preferred" },
  { label: "优先 IPv6", value: "ipv6-preferred" },
];

const groupTypes = [
  { label: "手动", value: "select" },
  { label: "测速", value: "url-test" },
  { label: "容灾", value: "fallback" },
];
</script>

<template>
  <div class="loon-stage">
    <!-- Stage 01: Loon 基础配置（零下拉单屏架构） -->
    <section v-if="step === 0" class="stage-panel">
      <div class="block-group">
        <span class="block-label">IP 协议栈模式</span>
        <div class="pill-row">
          <button
            v-for="m in ipModes"
            :key="m.value"
            type="button"
            class="opt-pill"
            :class="{ 'is-active': loonForm.general.ip_mode === m.value }"
            @click="loonForm.general.ip_mode = m.value"
          >
            {{ m.label }}
          </button>
        </div>
      </div>

      <div class="cap-grid">
        <button
          type="button"
          class="cap-card"
          :class="{ 'is-on': loonForm.general.sni_sniffing }"
          @click="loonForm.general.sni_sniffing = !loonForm.general.sni_sniffing"
        >
          <div>
            <strong>SNI 域名嗅探</strong>
            <small>识别纯 IP TLS 流量域名</small>
          </div>
          <span class="cap-tag">{{ loonForm.general.sni_sniffing ? "开启" : "关闭" }}</span>
        </button>

        <button
          type="button"
          class="cap-card"
          :class="{ 'is-on': loonForm.general.disable_stun }"
          @click="loonForm.general.disable_stun = !loonForm.general.disable_stun"
        >
          <div>
            <strong>禁用 STUN 穿透</strong>
            <small>防止 WebRTC 暴露真实 IP</small>
          </div>
          <span class="cap-tag">{{ loonForm.general.disable_stun ? "开启" : "关闭" }}</span>
        </button>

        <button
          type="button"
          class="cap-card"
          :class="{ 'is-on': loonForm.general.allow_wifi_access }"
          @click="loonForm.general.allow_wifi_access = !loonForm.general.allow_wifi_access"
        >
          <div>
            <strong>局域网 Wi-Fi 共享</strong>
            <small>HTTP {{ loonForm.general.wifi_http_port }} / SOCKS {{ loonForm.general.wifi_socks_port }}</small>
          </div>
          <span class="cap-tag">{{ loonForm.general.allow_wifi_access ? "开启" : "关闭" }}</span>
        </button>
      </div>

      <div class="inline-bar">
        <label class="inline-field">
          <span>DNS 服务器</span>
          <el-input v-model="loonForm.general.dns_server" placeholder="system,1.1.1.1" />
        </label>
        <div class="inline-field">
          <span>UDP 兜底</span>
          <div class="pill-row">
            <button
              v-for="u in ['REJECT', 'DIRECT']"
              :key="u"
              type="button"
              class="opt-pill mono"
              :class="{ 'is-active': loonForm.general.udp_fallback_mode === u }"
              @click="loonForm.general.udp_fallback_mode = u"
            >
              {{ u }}
            </button>
          </div>
        </div>
        <button
          type="button"
          class="adv-btn"
          @click="showAdvancedUrls = !showAdvancedUrls"
        >
          <el-icon><Setting /></el-icon>
          <span>{{ showAdvancedUrls ? "收起资源地址" : "解析器与 GeoIP" }}</span>
        </button>
      </div>

      <div v-if="showAdvancedUrls" class="adv-urls">
        <label>
          <span>Sub-Store 资源解析器</span>
          <el-input v-model="loonForm.general.resource_parser" />
        </label>
        <label>
          <span>GeoIP 数据库 URL</span>
          <el-input v-model="loonForm.general.geoip_url" />
        </label>
        <label>
          <span>ASN 数据库 URL</span>
          <el-input v-model="loonForm.general.ipasn_url" />
        </label>
      </div>
    </section>

    <!-- Stage 02: 节点筛选与策略组（双栏并排单屏） -->
    <section v-else-if="step === 1" class="loon-split">
      <div class="split-col">
        <div class="col-head">
          <strong>节点正则筛选 (Remote Filter)</strong>
          <UiButton
            size="sm"
            variant="ghost"
            :icon="Plus"
            @click="loonForm.filters.push({ name: '', regex: '' })"
          >
            添加筛选
          </UiButton>
        </div>
        <div class="scroll-box">
          <div v-for="(f, i) in loonForm.filters" :key="i" class="item-row is-filter">
            <el-input v-model="f.name" placeholder="筛选名（如 香港）" />
            <el-input v-model="f.regex" placeholder="正则 NameRegex" />
            <RowIconButton
              :icon="Delete"
              :label="`删除 ${f.name || i + 1}`"
              @click="loonForm.filters.splice(i, 1)"
            />
          </div>
        </div>
      </div>

      <div class="split-col">
        <div class="col-head">
          <strong>策略分组 (Proxy Group)</strong>
          <UiButton
            size="sm"
            variant="ghost"
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
          >
            添加策略组
          </UiButton>
        </div>
        <div class="scroll-box">
          <div v-for="(gr, i) in loonForm.groups" :key="i" class="item-row is-group">
            <el-input v-model="gr.name" placeholder="组名" />
            <div class="mini-pills">
              <button
                v-for="t in groupTypes"
                :key="t.value"
                type="button"
                class="mini-pill"
                :class="{ 'is-active': gr.type === t.value }"
                @click="gr.type = t.value"
              >
                {{ t.label }}
              </button>
            </div>
            <el-input v-model="gr.policies" placeholder="策略（逗号分隔）" />
            <RowIconButton
              :icon="Delete"
              :label="`删除 ${gr.name || i + 1}`"
              @click="loonForm.groups.splice(i, 1)"
            />
          </div>
        </div>
      </div>
    </section>

    <!-- Stage 03: 规则与插件（3 标签切换，告别 3 个大文本框纵向堆叠） -->
    <section v-else-if="step === 2" class="stage-panel">
      <div class="pill-row">
        <button
          type="button"
          class="opt-pill"
          :class="{ 'is-active': ruleTab === 'remote' }"
          @click="ruleTab = 'remote'"
        >
          远程规则 (Remote Rule)
        </button>
        <button
          type="button"
          class="opt-pill"
          :class="{ 'is-active': ruleTab === 'plugin' }"
          @click="ruleTab = 'plugin'"
        >
          Loon 插件 (Plugin)
        </button>
        <button
          type="button"
          class="opt-pill"
          :class="{ 'is-active': ruleTab === 'local' }"
          @click="ruleTab = 'local'"
        >
          本地规则 (Rule)
        </button>
      </div>

      <el-input
        v-if="ruleTab === 'remote'"
        v-model="loonForm.remote_rules"
        type="textarea"
        :rows="10"
        placeholder="每行一条：https://xxx, policy=组名, tag=标签, enabled=true"
      />
      <el-input
        v-else-if="ruleTab === 'plugin'"
        v-model="loonForm.plugins"
        type="textarea"
        :rows="10"
        placeholder="每行一条：https://xxx.lpx, enabled=true"
      />
      <el-input
        v-else
        v-model="loonForm.rules"
        type="textarea"
        :rows="10"
        placeholder="本地规则，自动追加 DOMAIN-SUFFIX,local,DIRECT / GEOIP,CN,DIRECT / FINAL,FINAL"
      />
    </section>
  </div>
</template>

<style scoped>
.loon-stage,
.stage-panel {
  display: grid;
  gap: 18px;
}

.block-group {
  display: grid;
  gap: 8px;
}

.block-label {
  font-size: 12px;
  color: var(--c-muted);
}

.pill-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.opt-pill {
  padding: 6px 14px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: var(--c-surface);
  color: var(--c-ink);
  font-size: 12.5px;
  cursor: pointer;
}

.opt-pill.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
  font-weight: 500;
}

.cap-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.cap-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
}

.cap-card.is-on {
  border-color: var(--c-ink);
}

.cap-card strong {
  display: block;
  font-size: 13.5px;
  color: var(--c-ink);
}

.cap-card small {
  font-size: 11.5px;
  color: var(--c-muted);
}

.cap-tag {
  padding: 3px 9px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 11px;
  color: var(--c-muted);
}

.cap-card.is-on .cap-tag {
  background: rgb(21 184 90 / 0.14);
  color: var(--c-ok);
  font-weight: 600;
}

.inline-bar {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  flex-wrap: wrap;
}

.inline-field {
  display: grid;
  gap: 6px;
  font-size: 12px;
  color: var(--c-muted);
  flex: 1 1 200px;
}

.adv-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 7px 12px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.adv-urls {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
}

.adv-urls label {
  display: grid;
  gap: 6px;
  font-size: 12px;
  color: var(--c-muted);
}

/* Split Stage 02 */
.loon-split {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1.3fr);
  gap: 16px;
}

.split-col {
  display: grid;
  gap: 10px;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 18px;
  background: var(--c-surface);
}

.col-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13.5px;
}

.scroll-box {
  display: grid;
  gap: 8px;
  max-height: 360px;
  overflow-y: auto;
  padding-right: 4px;
}

.item-row {
  display: grid;
  align-items: center;
  gap: 8px;
}

.item-row.is-filter {
  grid-template-columns: 110px minmax(0, 1fr) auto;
}

.item-row.is-group {
  grid-template-columns: 95px auto minmax(0, 1fr) auto;
}

.mini-pills {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
}

.mini-pill {
  padding: 3px 7px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--c-muted);
  font-size: 11px;
  cursor: pointer;
}

.mini-pill.is-active {
  background: var(--c-ink);
  color: var(--c-surface);
}

@media (max-width: 860px) {
  .cap-grid,
  .adv-urls,
  .loon-split {
    grid-template-columns: 1fr;
  }
}
</style>
