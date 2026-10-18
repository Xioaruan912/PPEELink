<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { useUserStore } from "@/store";
import { updateUserPassword } from "@/api/user";
import { PageHeader, UiButton } from "@/ui";
import { useI18n } from "vue-i18n";
const { t } = useI18n();
const userinfo = ref<any>();
const userStore = useUserStore();
const username = ref("");
const password = ref("");
const saving = ref(false);
const userError = ref("");
const passwordError = ref("");
const loadError = ref("");
const strength = computed(() => {
  const value = password.value.trim();
  if (!value) return "至少 6 位，建议混合字母、数字和符号。";
  if (value.length < 6) return "长度不足：至少需要 6 位。";
  const types = [
    /[a-z]/i.test(value),
    /[0-9]/.test(value),
    /[^a-z0-9]/i.test(value),
  ].filter(Boolean).length;
  return value.length >= 12 && types === 3
    ? "强度较高：长度与字符组合良好。"
    : value.length >= 8 && types >= 2
      ? "强度适中：可增加长度或加入符号。"
      : "强度较低：建议至少 8 位并混合字符。";
});
async function loadUser() {
  try {
    userinfo.value = await userStore.getUserInfo();
    loadError.value = "";
  } catch {
    loadError.value = "当前账户信息读取失败。";
  }
}
async function resetPassword() {
  if (saving.value) return;
  userError.value = username.value.trim() ? "" : "请输入新用户名。";
  passwordError.value = !password.value.trim()
    ? "请输入新密码。"
    : password.value.trim().length < 6
      ? "密码去除首尾空格后至少需要 6 位。"
      : "";
  if (userError.value || passwordError.value) {
    ElMessage.error(
      !username.value.trim() || !password.value.trim()
        ? t("userset.message.xx1")
        : t("userset.message.xx2")
    );
    return;
  }
  try {
    await ElMessageBox.confirm(
      t("userset.message.xx3"),
      t("userset.message.title"),
      {
        confirmButtonText: "确认修改",
        cancelButtonText: "取消",
        type: "warning",
      }
    );
  } catch {
    return;
  }
  saving.value = true;
  try {
    await updateUserPassword({
      username: username.value.trim(),
      password: password.value.trim(),
    });
    password.value = "";
    ElMessage.success("账户信息已更新，请重新登录。");
    window.location.reload();
  } catch {
    /* API 层显示错误，保留表单供用户重试。 */
  } finally {
    saving.value = false;
  }
}
onMounted(loadUser);
</script>
<template>
  <div class="account-page">
    <PageHeader title="账户设置" description="更新登录用户名与密码。" />
    <section class="account-content" data-enter>
      <div v-if="userinfo" class="current-account">
        <span class="account-avatar" aria-hidden="true">{{
          (userinfo.username || "U").charAt(0).toUpperCase()
        }}</span>
        <div>
          <p>当前登录</p>
          <strong>{{ userinfo.username }}</strong>
        </div>
      </div>
      <div v-if="loadError" class="account-error">
        <p role="alert">{{ loadError }}</p>
        <UiButton variant="ghost" @click="loadUser">重新读取</UiButton>
      </div>
      <el-form
        class="account-form"
        label-position="top"
        @submit.prevent="resetPassword"
      >
        <el-form-item label="新用户名" required :error="userError"
          ><el-input
            v-model="username"
            :placeholder="t('userset.newUsername')"
            name="username"
            autocomplete="username"
            :disabled="saving"
            aria-label="新用户名"
            @input="userError = ''"
        /></el-form-item>
        <el-form-item label="新密码" required :error="passwordError"
          ><el-input
            v-model="password"
            type="password"
            show-password
            :placeholder="t('userset.newPassword')"
            name="password"
            autocomplete="new-password"
            :disabled="saving"
            aria-describedby="password-strength"
            @input="passwordError = ''"
        /></el-form-item>
        <p
          id="password-strength"
          class="password-strength"
          :class="{
            'password-strength--warn':
              password.trim().length > 0 && password.trim().length < 6,
          }"
          aria-live="polite"
        >
          {{ strength }}
        </p>
        <p class="account-note">
          用户名和密码会去除首尾空格。保存后页面将刷新。
        </p>
        <UiButton variant="primary" type="submit" :loading="saving"
          >修改账户</UiButton
        >
      </el-form>
    </section>
  </div>
</template>
<style scoped>
.account-page {
  min-width: 0;
}
.account-content {
  width: 100%;
  max-width: 480px;
}
.current-account {
  display: flex;
  align-items: center;
  gap: 24px;
  padding-bottom: 32px;
  margin-bottom: 32px;
  border-bottom: 1px solid var(--c-line);
}
.account-avatar {
  display: inline-grid;
  place-items: center;
  flex-shrink: 0;
  width: 64px;
  height: 64px;
  border: 1px solid var(--c-line);
  border-radius: 16px;
  background: var(--c-surface-2);
  color: var(--c-signal-text);
  font: 400 32px/1 var(--font-display);
}
.current-account p {
  margin: 0 0 4px;
  color: var(--c-muted);
  font-size: 14px;
}
.current-account strong {
  font-size: 18px;
  font-weight: 500;
  overflow-wrap: anywhere;
}
.current-account > div {
  min-width: 0;
}
.account-form :deep(.el-form-item) {
  margin-bottom: 32px;
}
.password-strength {
  margin: -16px 0 24px;
  color: var(--c-muted);
  font-size: 14px;
}
.password-strength--warn,
.account-error {
  color: var(--c-bad);
}
.account-note {
  font-size: 14px;
  color: var(--c-muted);
  margin: 0 0 32px;
}
.account-error {
  margin-bottom: 24px;
  font-size: 14px;
}
@media (max-width: 767px) {
  .account-form > .ui-button {
    width: 100%;
  }
}
</style>
