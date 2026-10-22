<script setup lang="ts">
import { computed, reactive, ref, watch } from "vue";
import { UiButton, UiDialog } from "@/ui";
import { saveMonitorSSH } from "@/api/monitor";

interface SSHTarget {
  id: number;
  name: string;
  sshHost?: string;
  sshPort?: number;
  sshUser?: string;
  sshConfigured?: boolean;
}

const props = defineProps<{
  modelValue: boolean;
  monitor: SSHTarget | null;
}>();
const emit = defineEmits<{
  "update:modelValue": [value: boolean];
  saved: [];
}>();

const visible = computed({
  get: () => props.modelValue,
  set: (value) => emit("update:modelValue", value),
});

const form = reactive({ host: "", port: 22, user: "root", password: "" });
const error = ref("");
const saving = ref(false);

watch(visible, (open) => {
  if (!open) return;
  form.host = props.monitor?.sshHost || "";
  form.port = props.monitor?.sshPort || 22;
  form.user = props.monitor?.sshUser || "root";
  form.password = "";
  error.value = "";
});

async function submit() {
  if (!props.monitor || saving.value) return;
  if (!form.host.trim() || !form.user.trim()) {
    error.value = "请填写 SSH 地址和用户名。";
    return;
  }
  saving.value = true;
  error.value = "";
  try {
    await saveMonitorSSH({
      id: props.monitor.id,
      host: form.host.trim(),
      port: Number(form.port) || 22,
      user: form.user.trim(),
      password: form.password,
    });
    emit("saved");
    visible.value = false;
  } catch (err: any) {
    error.value = err?.message || "保存失败，请稍后重试。";
  } finally {
    saving.value = false;
  }
}
</script>
<template>
  <UiDialog v-model="visible" title="SSH 连接设置" width="520px">
    <p class="ssh-note">
      面板会用这里的凭据连接服务器，仅用于 WebSSH 和快速指令。密码加密保存，不会回显。
    </p>
    <el-form label-position="top" @submit.prevent="submit">
      <el-form-item label="SSH 地址" required>
        <el-input v-model="form.host" placeholder="例如 216.23.84.93" />
      </el-form-item>
      <div class="ssh-row">
        <el-form-item label="端口" required>
          <el-input v-model.number="form.port" type="number" min="1" max="65535" />
        </el-form-item>
        <el-form-item label="用户名" required>
          <el-input v-model="form.user" placeholder="root" />
        </el-form-item>
      </div>
      <el-form-item :label="monitor?.sshConfigured ? '密码（留空保持不变）' : '密码'">
        <el-input
          v-model="form.password"
          type="password"
          show-password
          autocomplete="new-password"
          :placeholder="monitor?.sshConfigured ? '留空沿用已保存的密码' : 'SSH 密码'"
        />
      </el-form-item>
      <p v-if="error" class="field-error" role="alert">{{ error }}</p>
    </el-form>
    <template #footer>
      <UiButton @click="visible = false">取消</UiButton>
      <UiButton variant="primary" :loading="saving" @click="submit"
        >保存并连接</UiButton
      >
    </template>
  </UiDialog>
</template>
<style scoped>
.ssh-note {
  margin: 0 0 14px;
  color: var(--c-muted);
  font-size: 12px;
  line-height: 1.7;
}
.ssh-row {
  display: grid;
  grid-template-columns: 120px minmax(0, 1fr);
  gap: 12px;
}
</style>
