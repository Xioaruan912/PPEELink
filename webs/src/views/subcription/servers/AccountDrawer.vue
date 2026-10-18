<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { saveProxyAccount, type ProxyAccount } from "@/api/proxy";
import { UiButton, UiDrawer } from "@/ui";
import { formatBytes } from "@/utils/traffic";

const props = defineProps<{ account: ProxyAccount | null }>();
const visible = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();
const GB = 1 << 30;
const form = reactive({
  enabled: true,
  limitGB: 0,
  resetDay: 0,
  expireAt: null as string | null,
  deviceLimit: 0,
  resetUsage: false,
  regenerate: false,
  unblock: false,
});
const saving = ref(false);
const error = ref("");
watch(visible, (open) => {
  if (!open || !props.account) return;
  const a = props.account;
  Object.assign(form, {
    enabled: a.uuid ? a.enabled : true,
    limitGB: Math.round((a.trafficLimit / GB) * 10) / 10,
    resetDay: a.resetDay,
    expireAt: a.expireAt,
    deviceLimit: a.deviceLimit,
    resetUsage: false,
    regenerate: false,
    unblock: false,
  });
  error.value = "";
});
const blocked = computed(
  () =>
    !!props.account?.blockedUntil &&
    new Date(props.account.blockedUntil).getTime() > Date.now()
);
async function save() {
  if (!props.account) return;
  saving.value = true;
  error.value = "";
  try {
    await saveProxyAccount({
      userId: props.account.userId,
      enabled: form.enabled,
      trafficLimit: Math.round(form.limitGB * GB),
      resetDay: form.resetDay,
      expireAt: form.expireAt,
      deviceLimit: form.deviceLimit,
      resetUsage: form.resetUsage,
      regenerate: form.regenerate,
      unblock: form.unblock,
    });
    ElMessage.success("已保存，服务器将在一分钟内应用");
    visible.value = false;
    emit("saved");
  } catch (e: any) {
    error.value = e?.message || "保存失败";
  } finally {
    saving.value = false;
  }
}
</script>
<template>
  <UiDrawer
    v-model="visible"
    :title="`自建节点 · ${account?.nickname || account?.username || ''}`"
    size="min(500px, 100vw)"
  >
    <template v-if="account">
      <div class="usage">
        <span>已用</span>
        <strong class="mono">{{ formatBytes(account.trafficUsed) }}</strong>
        <span v-if="account.trafficLimit"
          >/ {{ formatBytes(account.trafficLimit) }}</span
        >
      </div>
      <p v-if="blocked" class="blocked" role="alert">
        临时停用中：{{ account.blockedReason }}
      </p>
      <el-form label-position="top" @submit.prevent="save">
        <div class="switches">
          <label><el-switch v-model="form.enabled" /> 开通自建节点</label>
        </div>
        <div class="grid">
          <el-form-item label="流量额度（GB，0 为不限）">
            <el-input-number
              v-model="form.limitGB"
              :min="0"
              :max="100000"
              :precision="1"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="每月清零日（0 不清零）">
            <el-input-number
              v-model="form.resetDay"
              :min="0"
              :max="28"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="同时在线 IP 上限（0 不限）">
            <el-input-number
              v-model="form.deviceLimit"
              :min="0"
              :max="100"
              controls-position="right"
            />
          </el-form-item>
          <el-form-item label="到期时间">
            <el-date-picker
              v-model="form.expireAt"
              type="datetime"
              value-format="YYYY-MM-DDTHH:mm:ssZ"
              placeholder="长期有效"
            />
          </el-form-item>
        </div>
        <p class="hint">
          超过额度、到期或在线 IP 超过上限（临时停用 10
          分钟）时，该账户会自动从服务器配置中移除。
        </p>
        <div class="switches">
          <label><el-switch v-model="form.resetUsage" /> 清零已用流量</label>
          <label v-if="blocked"
            ><el-switch v-model="form.unblock" /> 解除临时停用</label
          >
          <label
            ><el-switch v-model="form.regenerate" />
            更换凭据（旧链接与已导入的配置立即失效）</label
          >
        </div>
        <p v-if="error" class="error" role="alert">{{ error }}</p>
      </el-form>
    </template>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" @click="save"
        >保存</UiButton
      >
    </template>
  </UiDrawer>
</template>
<style scoped>
.usage {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 14px;
  color: var(--c-muted);
  font-size: 13px;
}
.usage strong {
  color: var(--c-ink);
  font-size: 22px;
  font-weight: 600;
}
.blocked {
  margin-bottom: 14px;
  padding: 8px 12px;
  border-radius: var(--radius-control);
  background: color-mix(in srgb, var(--c-warn) 14%, transparent);
  color: var(--c-warn-ink);
  font-size: 13px;
}
.grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0 12px;
  margin-top: 12px;
}
.grid :deep(.el-input-number),
.grid :deep(.el-date-editor) {
  width: 100%;
}
.switches {
  display: grid;
  gap: 6px;
}
.switches label {
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 14px;
}
.hint {
  margin: 4px 0 14px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.6;
}
.mono {
  font-family: var(--font-mono);
}
.error {
  margin-top: 12px;
  color: var(--c-bad);
  font-size: 13px;
}
@media (max-width: 599px) {
  .grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
