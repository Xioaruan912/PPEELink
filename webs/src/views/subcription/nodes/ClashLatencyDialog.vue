<script setup lang="ts">
import { reactive, ref, watch } from "vue";
import { UiButton, UiDialog } from "@/ui";
import {
  clashVersion,
  ClashError,
  CONTROLLER_PRESETS,
  DEFAULT_TEST_URL,
  type ClashController,
} from "@/utils/clashController";
import { useNodes } from "./useNodes";

/** 连接用户电脑上的 Clash 外部控制接口，用它测出的延迟替换面板服务器的 TCP 延迟。 */
const {
  clashDialog,
  clashError,
  clashController,
  setClashController,
  runClashLatency,
} = useNodes();
const form = reactive<ClashController>({
  url: CONTROLLER_PRESETS[0].url,
  secret: "",
  testUrl: DEFAULT_TEST_URL,
});
const checking = ref(false);
// Clash Verge 只放行自带面板的来源，需要把本站加入“外部控制跨域设置 → 允许的来源”
const origin = window.location.origin;
async function copyOrigin() {
  try {
    await navigator.clipboard.writeText(origin);
    ElMessage.success("已复制");
  } catch {
    ElMessage.warning("复制失败，请手动选中复制");
  }
}
const result = ref<{ ok: boolean; text: string } | null>(null);
watch(clashDialog, (open) => {
  if (!open) return;
  Object.assign(
    form,
    clashController.value || {
      url: CONTROLLER_PRESETS[0].url,
      secret: "",
      testUrl: DEFAULT_TEST_URL,
    }
  );
  result.value = clashError.value
    ? { ok: false, text: clashError.value }
    : null;
});
async function check() {
  checking.value = true;
  result.value = null;
  try {
    result.value = {
      ok: true,
      text: `已连接 ${await clashVersion({ ...form })}`,
    };
    return true;
  } catch (e) {
    result.value = {
      ok: false,
      text: e instanceof ClashError ? e.message : "连接失败",
    };
    return false;
  } finally {
    checking.value = false;
  }
}
async function save() {
  if (!(await check())) return;
  setClashController({
    ...form,
    url: form.url.trim(),
    testUrl: form.testUrl.trim() || DEFAULT_TEST_URL,
  });
  clashDialog.value = false;
  void runClashLatency();
}
</script>
<template>
  <UiDialog v-model="clashDialog" title="连接我的 Clash" width="520px">
    <p class="hint">
      延迟由你电脑上运行的 Clash 经节点访问测试地址得出，和 Clash
      面板里的数字一致。浏览器只访问本机的控制接口，地址和密钥只保存在这个浏览器里，不会发给面板。
    </p>
    <el-form label-position="top" @submit.prevent="save">
      <el-form-item label="外部控制地址">
        <el-input v-model="form.url" placeholder="http://127.0.0.1:9090" />
        <div class="presets">
          <button
            v-for="p in CONTROLLER_PRESETS"
            :key="p.url"
            type="button"
            class="preset"
            :class="{ on: form.url === p.url }"
            @click="form.url = p.url"
          >
            {{ p.label }} <span class="mono">{{ p.url.split(":").pop() }}</span>
          </button>
        </div>
      </el-form-item>
      <el-form-item label="密钥（secret）">
        <el-input
          v-model="form.secret"
          type="password"
          show-password
          autocomplete="off"
          placeholder="Clash 外部控制里设置的密钥，没有可留空"
        />
      </el-form-item>
      <el-form-item label="测试地址">
        <el-input v-model="form.testUrl" :placeholder="DEFAULT_TEST_URL" />
      </el-form-item>
    </el-form>
    <p
      v-if="result"
      class="result"
      :class="result.ok ? 'is-ok' : 'is-bad'"
      role="status"
    >
      {{ result.text }}
    </p>
    <section class="verge" aria-label="Clash Verge 设置说明">
      <strong>Clash Verge 需要先放行本站</strong>
      <ol>
        <li>
          设置 → Clash 设置 → 外部控制：打开“启用外部控制器”，记下地址和密钥。
        </li>
        <li>
          点“外部控制”右边的齿轮进入“外部控制跨域设置”，打开“允许专用网络访问”，在“允许的来源”里添加：
          <span class="origin">
            <code class="mono">{{ origin }}</code>
            <button type="button" class="copy" @click="copyOrigin">复制</button>
          </span>
        </li>
      </ol>
      <p>
        只用 mihomo 内核时，对应配置里的
        <span class="mono">external-controller</span>、<span class="mono"
          >secret</span
        >
        与
        <span class="mono">external-controller-cors</span>（默认放行所有来源）。
      </p>
    </section>
    <template #footer>
      <UiButton :loading="checking" @click="check">测试连接</UiButton>
      <UiButton variant="primary" :loading="checking" @click="save"
        >保存并测延迟</UiButton
      >
    </template>
  </UiDialog>
</template>
<style scoped>
.hint {
  margin: 0 0 14px;
  color: var(--c-muted);
  font-size: 13px;
  line-height: 1.6;
}
.hint.small {
  margin: 12px 0 0;
  font-size: 12px;
}
.verge {
  margin-top: 12px;
  padding: 12px 14px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-control);
  background: var(--c-surface-2);
  font-size: 12px;
  line-height: 1.7;
  color: var(--c-muted);
}
.verge strong {
  display: block;
  margin-bottom: 4px;
  color: var(--c-ink);
  font-size: 13px;
}
.verge ol {
  margin: 0;
  padding-left: 18px;
}
.verge p {
  margin: 6px 0 0;
}
.origin {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 100%;
  margin-top: 2px;
}
.origin code {
  overflow-wrap: anywhere;
  padding: 1px 6px;
  border-radius: 6px;
  background: var(--c-surface);
  color: var(--c-ink);
}
.copy {
  flex: 0 0 auto;
  padding: 1px 8px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  color: var(--c-ink);
  font: inherit;
  cursor: pointer;
}
.copy:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.presets {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}
.preset {
  padding: 4px 10px;
  border: 1px solid var(--c-line);
  border-radius: var(--radius-pill);
  background: var(--c-surface);
  color: var(--c-muted);
  font: inherit;
  font-size: 12px;
  cursor: pointer;
}
.preset.on {
  border-color: var(--c-ink);
  color: var(--c-ink);
}
.preset:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.result {
  margin: 4px 0 0;
  padding: 8px 12px;
  border-radius: var(--radius-control);
  font-size: 13px;
  line-height: 1.5;
}
.result.is-ok {
  background: color-mix(in srgb, var(--c-ok) 12%, transparent);
  color: var(--c-ok);
}
.result.is-bad {
  background: color-mix(in srgb, var(--c-bad) 10%, transparent);
  color: var(--c-bad);
}
.mono {
  font-family: var(--font-mono);
}
</style>
