<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import QrcodeVue from "qrcode.vue";
import {
  nodeBatch,
  nodeDedupe,
  nodeTempLink,
  type DedupeGroup,
  type NodeChange,
} from "@/api/subcription/node";
import { UiButton, UiDialog, UiSegmented, EmptyState, UiSkeleton } from "@/ui";
import SecretLink from "../subs/SecretLink.vue";
import { useNodes } from "./useNodes";
import { formatDateTime } from "@/utils/date";

const { tool, loadAll, overviewList, clearSelection } = useNodes();
const visible = computed({
  get: () => !!tool.value,
  set: (open) => {
    if (!open) tool.value = null;
  },
});
const kind = computed(() => tool.value?.kind);
const targets = computed(() => tool.value?.targets || []);
const ids = computed(() => targets.value.map((n) => n.id));
const titles = {
  rename: "批量改名",
  flags: "连接开关",
  chain: "链式代理",
  dedupe: "去除重复节点",
  temp: "临时订阅",
};
const title = computed(() => (kind.value ? titles[kind.value] : ""));
const busy = ref(false);
const error = ref("");

// 改名 / 国旗
const renameMode = ref<string | number>("regex");
const rename = reactive({ pattern: "", replacement: "" });
const preview = ref<NodeChange[] | null>(null);
// 开关
const flags = reactive({ udp: true, tfo: false, skipCertVerify: false });
// 中转
const relayId = ref<number>();
const relayOptions = computed(() =>
  overviewList.value.filter((n) => !ids.value.includes(n.id))
);
// 去重
const dedupe = ref<{ groups: DedupeGroup[]; removed: number } | null>(null);
// 临时订阅
const hours = ref<string | number>(24);
const temp = ref<{ links: Record<string, string>; expiresAt: string } | null>(
  null
);
const tempClient = ref<string | number>("clash");

watch(tool, async (value) => {
  error.value = "";
  preview.value = null;
  dedupe.value = null;
  temp.value = null;
  if (!value) return;
  if (value.kind === "flags" && value.targets.length === 1) {
    const n = value.targets[0];
    Object.assign(flags, {
      udp: !!n.udp,
      tfo: !!n.tfo,
      skipCertVerify: !!n.skipCertVerify,
    });
  }
  if (value.kind === "chain")
    relayId.value = value.targets[0]?.relayId || undefined;
  if (value.kind === "dedupe") await loadDedupe();
});

function renameAction() {
  return renameMode.value === "emoji-add" || renameMode.value === "emoji-remove"
    ? (renameMode.value as "emoji-add" | "emoji-remove")
    : "rename";
}
async function runPreview() {
  busy.value = true;
  error.value = "";
  try {
    const { data } = await nodeBatch({
      ids: ids.value,
      action: renameAction(),
      pattern: renameMode.value === "regex" ? rename.pattern : "",
      replacement: rename.replacement,
      preview: true,
    });
    preview.value = data?.changes || [];
  } catch (e: any) {
    error.value = e?.message || "预览失败";
  } finally {
    busy.value = false;
  }
}
async function apply(
  payload: Parameters<typeof nodeBatch>[0],
  success: string
) {
  busy.value = true;
  error.value = "";
  try {
    await nodeBatch(payload);
    ElMessage.success(success);
    tool.value = null;
    clearSelection();
    await loadAll();
  } catch (e: any) {
    error.value = e?.message || "操作失败";
  } finally {
    busy.value = false;
  }
}
const applyRename = () =>
  apply(
    {
      ids: ids.value,
      action: renameAction(),
      pattern: renameMode.value === "regex" ? rename.pattern : "",
      replacement: rename.replacement,
    },
    "已更新名称"
  );
const applyFlags = () =>
  apply({ ids: ids.value, action: "flags", flags: { ...flags } }, "开关已保存");
const applyChain = (clear = false) =>
  apply(
    {
      ids: ids.value,
      action: "chain",
      relayId: clear ? 0 : relayId.value || 0,
    },
    clear ? "已取消中转" : "已设置中转"
  );
async function loadDedupe() {
  busy.value = true;
  try {
    dedupe.value = (await nodeDedupe(true)).data;
  } catch (e: any) {
    error.value = e?.message || "检查失败";
  } finally {
    busy.value = false;
  }
}
async function applyDedupe() {
  busy.value = true;
  try {
    const { data } = await nodeDedupe(false);
    ElMessage.success(`已移除 ${data?.removed || 0} 个重复节点`);
    tool.value = null;
    await loadAll();
  } catch (e: any) {
    error.value = e?.message || "去重失败";
  } finally {
    busy.value = false;
  }
}
async function makeTemp() {
  busy.value = true;
  error.value = "";
  try {
    temp.value = (await nodeTempLink(ids.value[0], Number(hours.value))).data;
  } catch (e: any) {
    error.value = e?.message || "生成失败";
  } finally {
    busy.value = false;
  }
}
const tempLink = computed(
  () => temp.value?.links[String(tempClient.value)] || ""
);
</script>
<template>
  <UiDialog v-model="visible" :title="title" width="560px">
    <p v-if="kind && kind !== 'dedupe'" class="scope">
      作用于
      <strong>{{
        targets.length === 1 ? targets[0].name : `${targets.length} 个节点`
      }}</strong>
    </p>

    <template v-if="kind === 'rename'">
      <UiSegmented
        v-model="renameMode"
        label="改名方式"
        :options="[
          { label: '正则替换', value: 'regex' },
          { label: '加国旗', value: 'emoji-add' },
          { label: '去国旗', value: 'emoji-remove' },
        ]"
      />
      <el-form
        v-if="renameMode === 'regex'"
        label-position="top"
        class="tool-form"
      >
        <el-form-item label="匹配（正则，留空则整体替换）">
          <el-input
            v-model="rename.pattern"
            class="mono-input"
            placeholder="例如 ^(.*?)\s*\|.*$"
          />
        </el-form-item>
        <el-form-item label="替换为（可用 $1 引用分组）">
          <el-input
            v-model="rename.replacement"
            class="mono-input"
            placeholder="例如 $1"
          />
        </el-form-item>
      </el-form>
      <p v-else class="hint">
        {{
          renameMode === "emoji-add"
            ? "按 GeoIP 或名称识别国家，在名称前加国旗。"
            : "去掉名称开头的国旗。"
        }}
      </p>
      <TransitionGroup v-if="preview" tag="ul" name="change" class="changes">
        <li v-if="!preview.length" key="none" class="hint">
          没有会改变的名称。
        </li>
        <li v-for="c in preview" :key="c.id">
          <span class="before">{{ c.before }}</span
          ><span class="arrow" aria-hidden="true">→</span
          ><span class="after">{{ c.after }}</span>
        </li>
      </TransitionGroup>
    </template>

    <template v-else-if="kind === 'flags'">
      <div class="switches">
        <label><el-switch v-model="flags.udp" /> UDP 转发</label>
        <label><el-switch v-model="flags.tfo" /> TCP Fast Open</label>
        <label><el-switch v-model="flags.skipCertVerify" /> 跳过证书校验</label>
      </div>
      <p class="hint">
        按节点覆盖订阅的默认开关，生成 Clash、Surge、Loon 配置时生效。
      </p>
    </template>

    <template v-else-if="kind === 'chain'">
      <div class="chain-visual" aria-hidden="true">
        <span>你的设备</span><i></i
        ><span class="relay">{{
          relayOptions.find((n) => n.id === relayId)?.name || "中转节点"
        }}</span
        ><i></i
        ><span class="landing">{{
          targets.length === 1 ? targets[0].name : "落地节点"
        }}</span>
      </div>
      <el-form label-position="top" class="tool-form">
        <el-form-item label="中转节点">
          <el-select
            v-model="relayId"
            filterable
            placeholder="选择先连接的节点"
            class="full"
          >
            <el-option
              v-for="n in relayOptions"
              :key="n.id"
              :label="n.name"
              :value="n.id"
            />
          </el-select>
        </el-form-item>
      </el-form>
      <p class="hint">
        流量先到中转再到落地节点，网站看到的是落地 IP。Clash（dialer-proxy）与
        Surge（underlying-proxy）支持；中转节点不在订阅里时会自动补上。
      </p>
    </template>

    <template v-else-if="kind === 'dedupe'">
      <UiSkeleton v-if="busy && !dedupe" :lines="3" height="40px" />
      <EmptyState
        v-else-if="dedupe && !dedupe.removed"
        size="inline"
        title="没有重复节点"
        description="协议、服务器和端口都相同才算重复。"
      />
      <template v-else-if="dedupe">
        <p class="hint">
          发现
          <strong>{{ dedupe.removed }}</strong>
          个重复节点。保留最早添加的一个，分组与订阅引用会转到它上面。
        </p>
        <ul class="changes">
          <li v-for="g in dedupe.groups" :key="g.keep.ID">
            <span class="after">{{ g.keep.Name }}</span
            ><span class="arrow" aria-hidden="true">←</span
            ><span class="before">{{
              g.removed.map((r) => r.Name).join("、")
            }}</span>
          </li>
        </ul>
      </template>
    </template>

    <template v-else-if="kind === 'temp'">
      <template v-if="!temp">
        <UiSegmented
          v-model="hours"
          label="有效期"
          :options="[
            { label: '1 小时', value: 1 },
            { label: '1 天', value: 24 },
            { label: '7 天', value: 168 },
            { label: '30 天', value: 720 },
          ]"
        />
        <p class="hint">
          生成只含这个节点的订阅链接，到期自动失效，无需手动回收。
        </p>
      </template>
      <div v-else class="temp-result">
        <QrcodeVue
          v-if="tempLink"
          class="temp-qr"
          :value="tempLink"
          :size="148"
          level="M"
        />
        <div class="temp-copy">
          <UiSegmented
            v-model="tempClient"
            label="客户端"
            :options="
              Object.keys(temp.links).map((k) => ({ label: k, value: k }))
            "
          />
          <SecretLink :value="tempLink" label="临时订阅链接" />
          <p class="hint">有效至 {{ formatDateTime(temp.expiresAt) }}</p>
        </div>
      </div>
    </template>

    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <template #footer>
      <UiButton @click="visible = false">{{ temp ? "完成" : "取消" }}</UiButton>
      <template v-if="kind === 'rename'">
        <UiButton :loading="busy && !preview" @click="runPreview"
          >预览</UiButton
        >
        <UiButton
          variant="primary"
          :loading="busy"
          :disabled="preview !== null && !preview.length"
          @click="applyRename"
          >应用</UiButton
        >
      </template>
      <UiButton
        v-else-if="kind === 'flags'"
        variant="primary"
        :loading="busy"
        @click="applyFlags"
        >保存</UiButton
      >
      <template v-else-if="kind === 'chain'">
        <UiButton
          v-if="targets.some((t) => t.relayId)"
          :disabled="busy"
          @click="applyChain(true)"
          >取消中转</UiButton
        >
        <UiButton
          variant="primary"
          :loading="busy"
          :disabled="!relayId"
          @click="applyChain()"
          >设置中转</UiButton
        >
      </template>
      <UiButton
        v-else-if="kind === 'dedupe'"
        variant="danger"
        :loading="busy"
        :disabled="!dedupe?.removed"
        @click="applyDedupe"
        >移除 {{ dedupe?.removed || 0 }} 个</UiButton
      >
      <UiButton
        v-else-if="kind === 'temp' && !temp"
        variant="primary"
        :loading="busy"
        @click="makeTemp"
        >生成链接</UiButton
      >
    </template>
  </UiDialog>
</template>
<style scoped>
.scope {
  margin-bottom: 14px;
  color: var(--c-muted);
  font-size: 13px;
}
.scope strong {
  color: var(--c-ink);
}
.tool-form {
  margin-top: 14px;
}
.mono-input :deep(input) {
  font-family: var(--font-mono);
}
.hint {
  margin-top: 10px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
}
.full {
  width: 100%;
}
.changes {
  position: relative;
  display: grid;
  gap: 6px;
  max-height: 260px;
  margin-top: 14px;
  overflow: auto;
}
.changes li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr);
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border-radius: var(--radius-control);
  background: var(--c-surface-2);
  font-size: 13px;
}
.before {
  overflow: hidden;
  color: var(--c-muted);
  text-decoration: line-through;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.after {
  overflow: hidden;
  font-weight: 500;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.arrow {
  color: var(--c-signal-text);
}
.change-enter-active {
  transition:
    opacity var(--duration-medium) var(--ease-out),
    transform var(--duration-medium) var(--ease-spring);
}
.change-enter-from {
  opacity: 0;
  transform: translateX(-10px);
}
.switches {
  display: grid;
  gap: 4px;
}
.switches label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}
.chain-visual {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
  font-size: 12px;
}
.chain-visual span {
  padding: 6px 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface-2);
  white-space: nowrap;
  max-width: 34%;
  overflow: hidden;
  text-overflow: ellipsis;
}
.chain-visual .relay {
  border-color: var(--c-focus);
}
.chain-visual .landing {
  background: var(--c-signal);
  border-color: var(--c-signal);
  color: var(--c-on-signal);
}
.chain-visual i {
  flex: 1;
  height: 2px;
  min-width: 12px;
  background: repeating-linear-gradient(
    90deg,
    var(--c-focus) 0 6px,
    transparent 6px 10px
  );
  background-size: 20px 2px;
  animation: chain-flow 0.8s linear infinite;
}
@keyframes chain-flow {
  to {
    background-position: 20px 0;
  }
}
.temp-result {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 16px;
}
.temp-qr {
  padding: 8px;
  border-radius: var(--radius-control);
  background: #fff;
}
.temp-copy {
  display: grid;
  gap: 10px;
  min-width: 0;
}
.error {
  margin-top: 12px;
  color: var(--c-bad);
  font-size: 13px;
}
@media (max-width: 599px) {
  .temp-result {
    grid-template-columns: minmax(0, 1fr);
    justify-items: center;
  }
}
@media (prefers-reduced-motion: reduce) {
  .chain-visual i {
    animation: none;
  }
}
</style>
