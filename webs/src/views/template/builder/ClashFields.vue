<script setup lang="ts">
import { computed, ref } from "vue";
import { Delete, Plus, Setting } from "@element-plus/icons-vue";
import { UiButton, UiDrawer } from "@/ui";
import RowIconButton from "@/ui/RowIconButton.vue";
import { useTemplateBuilder } from "./useTemplateBuilder";
import GroupBoard from "./GroupBoard.vue";
import RulePicker from "./RulePicker.vue";

defineProps<{ step: string | number }>();
const { form, rulesText, addRP, removeRP, router } = useTemplateBuilder();

const showPortEditor = ref(false);
const customRulesDrawer = ref(false);

const modes = [
  { label: "规则分流 (Rule)", value: "rule", desc: "按规则集智能分流国内外流量" },
  { label: "全局代理 (Global)", value: "global", desc: "所有连接统一走主策略组" },
  { label: "全局直连 (Direct)", value: "direct", desc: "所有流量直接连接不走代理" },
];

const capabilities = computed(() => [
  { key: "allow_lan", label: "局域网共享", desc: "允许同局域网设备接入" },
  { key: "ipv6", label: "IPv6 支持", desc: "解析并转发 IPv6 流量" },
  { key: "udp", label: "UDP 转发", desc: "支持 QUIC / 游戏与语音" },
  { key: "tun_enable", label: "TUN 虚拟网卡", desc: "接管系统全局三层流量" },
  { key: "sniffer_enable", label: "TLS 域名嗅探", desc: "还原纯 IP 连接真实域名" },
  { key: "tcp_concurrent", label: "TCP 并发连接", desc: "多 IP 并发择优握手" },
] as const);

const ruleLineCount = computed(
  () =>
    rulesText.value
      .split("\n")
      .filter((l) => l.trim() && !l.trim().startsWith("#")).length
);

interface ParsedRuleRow {
  lineIndex: number;
  type: string;
  match: string;
  policy: string;
  extra: string;
  raw: string;
}

const availablePolicies = computed(() => {
  const names = form.groups.map((g) => g.name.trim()).filter(Boolean);
  return [...new Set([...names, "DIRECT", "REJECT"])];
});

const parsedRules = computed<ParsedRuleRow[]>(() => {
  const lines = rulesText.value.split("\n");
  const rows: ParsedRuleRow[] = [];
  lines.forEach((rawLine, lineIndex) => {
    const trimmed = rawLine.trim();
    if (!trimmed || trimmed.startsWith("#")) return;
    const parts = trimmed.split(",").map((p) => p.trim());
    const type = parts[0] || "";
    if (/^(MATCH|FINAL)$/i.test(type)) {
      rows.push({
        lineIndex,
        type: type.toUpperCase(),
        match: "兜底匹配",
        policy: parts[1] || "DIRECT",
        extra: parts.slice(2).join(","),
        raw: trimmed,
      });
      return;
    }
    if (parts.length >= 3) {
      rows.push({
        lineIndex,
        type: type.toUpperCase(),
        match: parts[1] || "",
        policy: parts[2] || "DIRECT",
        extra: parts.slice(3).join(","),
        raw: trimmed,
      });
    }
  });
  return rows;
});

function updateRulePolicy(row: ParsedRuleRow, nextPolicy: string) {
  const lines = rulesText.value.split("\n");
  const rebuilt = /^(MATCH|FINAL)$/i.test(row.type)
    ? [row.type, nextPolicy, row.extra].filter(Boolean).join(",")
    : [row.type, row.match, nextPolicy, row.extra].filter(Boolean).join(",");
  lines[row.lineIndex] = rebuilt;
  rulesText.value = lines.join("\n");
}

function removeRuleRow(row: ParsedRuleRow) {
  const lines = rulesText.value.split("\n");
  lines.splice(row.lineIndex, 1);
  rulesText.value = lines.join("\n");
}

const newRuleType = ref("RULE-SET");
const newRuleMatch = ref("");
const newRulePolicy = ref("");

const ruleTypeOptions = [
  "RULE-SET",
  "DOMAIN-SUFFIX",
  "DOMAIN-KEYWORD",
  "DOMAIN",
  "IP-CIDR",
  "GEOIP",
  "MATCH",
];

function addQuickRule() {
  const type = newRuleType.value;
  const policy = newRulePolicy.value || availablePolicies.value[0] || "DIRECT";
  const match = newRuleMatch.value.trim();
  if (type !== "MATCH" && !match) return;
  const line = type === "MATCH" ? `MATCH,${policy}` : `${type},${match},${policy}`;
  const lines = rulesText.value
    .split("\n")
    .filter((l) => l.trim().length > 0);
  if (type === "MATCH") {
    lines.push(line);
  } else {
    const tailIndex = lines.findIndex((l) =>
      /^(GEOIP|MATCH|FINAL)\s*,/i.test(l.trim())
    );
    if (tailIndex >= 0) lines.splice(tailIndex, 0, line);
    else lines.push(line);
  }
  rulesText.value = lines.join("\n");
  newRuleMatch.value = "";
}
</script>

<template>
  <div class="clash-stage">
    <!-- Stage 01: 基础配置（零滚轮单屏架构） -->
    <section v-if="step === 0" class="stage-panel">
      <div class="block-group">
        <span class="block-label">运行模式</span>
        <div class="mode-grid">
          <button
            v-for="m in modes"
            :key="m.value"
            type="button"
            class="mode-card"
            :class="{ 'is-active': form.mode === m.value }"
            @click="form.mode = m.value"
          >
            <div class="mode-top">
              <strong>{{ m.label }}</strong>
              <span class="mode-dot"></span>
            </div>
            <small>{{ m.desc }}</small>
          </button>
        </div>
      </div>

      <div class="block-group">
        <span class="block-label">核心网络特性</span>
        <div class="cap-grid">
          <button
            v-for="cap in capabilities"
            :key="cap.key"
            type="button"
            class="cap-card"
            :class="{ 'is-on': form[cap.key] }"
            @click="form[cap.key] = !form[cap.key]"
          >
            <div class="cap-copy">
              <strong>{{ cap.label }}</strong>
              <small>{{ cap.desc }}</small>
            </div>
            <span class="cap-indicator">
              {{ form[cap.key] ? "已开启" : "关闭" }}
            </span>
          </button>
        </div>
      </div>

      <!-- Compact Port Summary Bar + On-Demand Inline Editor -->
      <div class="port-strip">
        <div class="port-summary mono">
          <span>HTTP <strong>{{ form.port }}</strong></span>
          <span>SOCKS <strong>{{ form.socks_port }}</strong></span>
          <span>Mixed <strong>{{ form.mixed_port }}</strong></span>
          <span>DNS <strong>{{ form.dns_listen_port }}</strong></span>
          <span>控制面板 <strong>{{ form.external_controller }}</strong></span>
        </div>
        <button
          type="button"
          class="port-toggle"
          @click="showPortEditor = !showPortEditor"
        >
          <el-icon><Setting /></el-icon>
          <span>{{ showPortEditor ? "收起端口" : "微调端口" }}</span>
        </button>
      </div>

      <div v-if="showPortEditor" class="port-editor">
        <label>
          <span>HTTP 端口</span>
          <el-input-number v-model="form.port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label>
          <span>SOCKS 端口</span>
          <el-input-number v-model="form.socks_port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label>
          <span>Mixed 端口</span>
          <el-input-number v-model="form.mixed_port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label>
          <span>Redir 端口</span>
          <el-input-number v-model="form.redir_port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label>
          <span>TProxy 端口</span>
          <el-input-number v-model="form.tproxy_port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label>
          <span>DNS 端口</span>
          <el-input-number v-model="form.dns_listen_port" :min="1" :max="65535" controls-position="right" />
        </label>
        <label class="is-wide">
          <span>外部控制器 (external-controller)</span>
          <el-input v-model="form.external_controller" placeholder="127.0.0.1:9090" />
        </label>
      </div>
    </section>

    <!-- Stage 02: 策略分组（先定分组，再配分流规则） -->
    <section v-else-if="step === 1" class="stage-panel">
      <GroupBoard />
    </section>

    <!-- Stage 03: 分流规则（点选分类 + 逐条指定目标分组 + 自定义规则抽屉） -->
    <section v-else-if="step === 2" class="stage-panel">
      <RulePicker />

      <div class="rule-routing-card">
        <header class="routing-head">
          <div>
            <strong>分流规则与目标分组</strong>
            <small>按上一步建好的策略分组，逐条指定每条规则的去向</small>
          </div>
          <span class="mono routing-count">
            {{ form.rule_providers.length }} 个规则集 · {{ ruleLineCount }} 条规则
          </span>
        </header>

        <div class="rule-add-bar">
          <select v-model="newRuleType" class="rule-select is-type" aria-label="规则类型">
            <option v-for="t in ruleTypeOptions" :key="t" :value="t">
              {{ t }}
            </option>
          </select>
          <input
            v-model="newRuleMatch"
            type="text"
            class="rule-match-input mono"
            :disabled="newRuleType === 'MATCH'"
            :placeholder="
              newRuleType === 'MATCH'
                ? '兜底规则无需填写匹配项'
                : newRuleType === 'RULE-SET'
                  ? '规则集名（如 Xbox、Discord、China）'
                  : '匹配值（如 CN、openai.com）'
            "
            @keydown.enter.prevent="addQuickRule"
          />
          <select
            v-model="newRulePolicy"
            class="rule-select is-policy"
            aria-label="目标分组"
          >
            <option value="" disabled>选择目标分组</option>
            <option v-for="p in availablePolicies" :key="p" :value="p">
              {{ p }}
            </option>
          </select>
          <UiButton
            size="sm"
            variant="primary"
            :icon="Plus"
            :disabled="newRuleType !== 'MATCH' && !newRuleMatch.trim()"
            @click="addQuickRule"
          >
            添加规则
          </UiButton>
        </div>

        <div class="routing-list">
          <p v-if="!parsedRules.length" class="routing-empty">
            暂无分流规则，可在上方勾选预设分类或手动添加规则
          </p>
          <div
            v-for="row in parsedRules"
            :key="`${row.lineIndex}-${row.raw}`"
            class="routing-row"
          >
            <span class="rule-type-badge mono">{{ row.type }}</span>
            <span class="rule-match-text mono" :title="row.match">
              {{ row.match }}
              <em v-if="row.extra" class="rule-extra">,{{ row.extra }}</em>
            </span>
            <select
              class="rule-select is-row-policy"
              :value="row.policy"
              :aria-label="`${row.type} ${row.match} 目标分组`"
              @change="
                updateRulePolicy(
                  row,
                  ($event.target as HTMLSelectElement).value
                )
              "
            >
              <option
                v-if="!availablePolicies.includes(row.policy)"
                :value="row.policy"
              >
                {{ row.policy }}（未定义分组）
              </option>
              <option v-for="p in availablePolicies" :key="p" :value="p">
                {{ p }}
              </option>
            </select>
            <RowIconButton
              :icon="Delete"
              :label="`删除规则 ${row.raw}`"
              @click="removeRuleRow(row)"
            />
          </div>
        </div>
      </div>

      <div class="custom-rule-bar">
        <span class="mono">
          需要批量编辑原始规则文本或增删远程规则集 URL？
        </span>
        <div class="custom-actions">
          <UiButton variant="ghost" @click="router.push('/template/rules')">
            前往规则仓库
          </UiButton>
          <UiButton variant="ghost" :icon="Setting" @click="customRulesDrawer = true">
            高级自定义规则
          </UiButton>
        </div>
      </div>

      <UiDrawer
        v-model="customRulesDrawer"
        title="自定义规则集与分流规则"
        size="min(620px, 100vw)"
      >
        <div class="drawer-stack">
          <div class="drawer-block">
            <div class="drawer-head">
              <strong>远程规则集 (rule-providers)</strong>
              <UiButton size="sm" :icon="Plus" @click="addRP">添加规则集</UiButton>
            </div>
            <div class="rp-list">
              <div v-for="(rp, i) in form.rule_providers" :key="i" class="rp-row">
                <el-input v-model="rp.name" placeholder="名称（如 AI）" />
                <el-input v-model="rp.url" placeholder="规则集 URL" />
                <RowIconButton
                  :icon="Delete"
                  :label="`删除 ${rp.name || i + 1}`"
                  @click="removeRP(i)"
                />
              </div>
            </div>
          </div>

          <div class="drawer-block">
            <strong>分流规则行 (rules)</strong>
            <el-input
              v-model="rulesText"
              type="textarea"
              :rows="10"
              placeholder="每行一条规则，如：RULE-SET,AI,AI / GEOIP,CN,DIRECT"
            />
          </div>
        </div>
        <template #footer>
          <UiButton variant="primary" @click="customRulesDrawer = false">
            完成
          </UiButton>
        </template>
      </UiDrawer>
    </section>
  </div>
</template>

<style scoped>
.clash-stage,
.stage-panel {
  display: grid;
  gap: 14px;
}

.block-group {
  display: grid;
  gap: 7px;
}

.block-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--c-muted);
}

/* 3-Column Mode Cards */
.mode-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.mode-card {
  display: grid;
  gap: 6px;
  padding: 16px 18px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
  text-align: left;
  cursor: pointer;
  transition:
    transform 100ms ease-out,
    border-color 180ms var(--ease-out),
    background 180ms var(--ease-out);
}

.mode-card:hover {
  border-color: var(--c-ink);
}

.mode-card:active {
  transform: scale(0.98);
}

.mode-card.is-active {
  background: var(--c-ink);
  border-color: var(--c-ink);
  color: var(--c-surface);
}

.mode-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.mode-top strong {
  font-size: 14px;
  font-weight: 500;
}

.mode-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  border: 1.5px solid currentColor;
}

.mode-card.is-active .mode-dot {
  background: var(--c-ok);
  border-color: var(--c-ok);
}

.mode-card small {
  font-size: 12px;
  opacity: 0.72;
}

/* 3x2 Tactile Capability Cards */
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
  transition:
    transform 100ms ease-out,
    border-color 180ms var(--ease-out),
    background 180ms var(--ease-out);
}

.cap-card:active {
  transform: scale(0.98);
}

.cap-card.is-on {
  border-color: var(--c-ink);
  background: color-mix(in srgb, var(--c-ok) 5%, var(--c-surface));
}

.cap-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.cap-copy strong {
  font-size: 13.5px;
  font-weight: 500;
  color: var(--c-ink);
}

.cap-copy small {
  font-size: 11.5px;
  color: var(--c-muted);
}

.cap-indicator {
  flex: none;
  padding: 3px 9px;
  border-radius: 999px;
  background: var(--c-surface-muted);
  font-size: 11px;
  color: var(--c-muted);
}

.cap-card.is-on .cap-indicator {
  background: rgb(21 184 90 / 0.14);
  color: var(--c-ok);
  font-weight: 600;
}

/* Compact Port Bar */
.port-strip {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  flex-wrap: wrap;
}

.port-summary {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12px;
  color: var(--c-muted);
}

.port-summary strong {
  color: var(--c-ink);
}

.port-toggle {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 5px 11px;
  border: 1px solid var(--c-line);
  border-radius: 999px;
  background: transparent;
  color: var(--c-ink);
  font-size: 12px;
  cursor: pointer;
}

.port-toggle:hover {
  border-color: var(--c-ink);
}

.port-editor {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}

.port-editor label {
  display: grid;
  gap: 6px;
  font-size: 12px;
  color: var(--c-muted);
}

.port-editor .is-wide {
  grid-column: 1 / -1;
}

/* Custom Rule Bar */
.custom-rule-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--c-line);
  border-radius: 14px;
  background: var(--c-surface);
  font-size: 12.5px;
  color: var(--c-muted);
  flex-wrap: wrap;
}

.custom-rule-bar strong {
  color: var(--c-ink);
}

.custom-actions {
  display: flex;
  gap: 8px;
}

.drawer-stack {
  display: grid;
  gap: 20px;
}

.drawer-block {
  display: grid;
  gap: 10px;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rp-list {
  display: grid;
  gap: 8px;
  max-height: 280px;
  overflow-y: auto;
}

.rp-row {
  display: grid;
  grid-template-columns: 140px minmax(0, 1fr) auto;
  gap: 8px;
  align-items: center;
}

/* Rule-to-Group Routing Table */
.rule-routing-card {
  display: grid;
  gap: 12px;
  padding: 16px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface);
}

.routing-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.routing-head strong {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--c-ink);
}

.routing-head small {
  font-size: 12px;
  color: var(--c-muted);
}

.routing-count {
  font-size: 12px;
  color: var(--c-muted);
}

.rule-add-bar {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr) 168px auto;
  gap: 8px;
  align-items: center;
}

.rule-select,
.rule-match-input {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  height: 34px;
  padding: 0 10px;
  border: 1px solid var(--c-line);
  border-radius: 9px;
  background: var(--c-surface-muted);
  color: var(--c-ink);
  font-size: 12.5px;
  outline: none;
}

.rule-select:focus,
.rule-match-input:focus {
  border-color: var(--c-ink);
}

.routing-list {
  display: grid;
  gap: 6px;
  max-height: 360px;
  overflow-y: auto;
  padding-right: 4px;
}

.routing-empty {
  margin: 18px 0;
  text-align: center;
  font-size: 12.5px;
  color: var(--c-muted);
}

.routing-row {
  display: grid;
  grid-template-columns: 118px minmax(0, 1fr) 180px auto;
  gap: 10px;
  align-items: center;
  padding: 7px 10px;
  border: 1px solid var(--c-line);
  border-radius: 10px;
  background: var(--c-surface-muted);
}

.rule-type-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 3px 8px;
  border-radius: 6px;
  background: var(--c-surface);
  border: 1px solid var(--c-line);
  color: var(--c-ink);
  font-size: 11px;
  font-weight: 500;
}

.rule-match-text {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 12.5px;
  color: var(--c-ink);
}

.rule-extra {
  font-style: normal;
  color: var(--c-muted);
  font-size: 11px;
}

.rule-select.is-row-policy {
  height: 30px;
  background: var(--c-surface);
  font-size: 12px;
}

@media (max-width: 820px) {
  .mode-grid,
  .cap-grid,
  .port-editor {
    grid-template-columns: 1fr;
  }

  .rule-add-bar,
  .routing-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
