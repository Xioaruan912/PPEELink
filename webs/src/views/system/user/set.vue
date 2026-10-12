<script setup lang='ts'>
import { ref,onMounted } from 'vue'
import {useUserStore} from "@/store"
import {updateUserPassword} from "@/api/user"
import { useI18n } from 'vue-i18n'
// 创建 i18n 实例
const { t } = useI18n()
const userinfo = ref()
const userStore = useUserStore()
// 获取用户信息
onMounted( async() => {
  userinfo.value = await userStore.getUserInfo()
})
const username:Ref<string> = ref('')
const password:Ref<string> = ref('')


/** 重置密码 */
function resetPassword(row: { [key: string]: any }) {
  if (!username.value || !password.value) {
        ElMessage.error(t('userset.message.xx1'))
        return
      }
      if ((password.value.length < 6)) {
        ElMessage.error(t('userset.message.xx2'))
        return
      }
  ElMessageBox.confirm(
    t('userset.message.xx3'),
    t('userset.message.title'),
    {
      confirmButtonText: 'OK',
      cancelButtonText: 'Cancel',
      type: 'warning',
    }
  )
    .then(() => {
      updateUserPassword({
        username:username.value.trim(),
        password:password.value.trim()
      
      }
       ).then(() => {
      ElMessage.success(t('userset.message.xx4') + password.value);
      window.location.reload();
    });
    })
}
</script>

<template>
  <div class="account-page">
    <PageHero title="账户设置" desc="更新管理员用户名与密码；修改后请用新凭据重新登录。" />
    <section class="account-card">
      <div class="account-id">
        <span class="account-avatar">{{ (userinfo?.username || 'U').charAt(0).toUpperCase() }}</span>
        <div class="account-id-copy">
          <b>{{ userinfo?.username || '管理员' }}</b>
          <small>管理员账户</small>
        </div>
      </div>
      <div class="account-form">
        <label class="account-label">新用户名</label>
        <el-input v-model="username" :placeholder="$t('userset.newUsername')" />
        <label class="account-label">新密码</label>
        <el-input type="password" v-model="password" show-password :placeholder="$t('userset.newPassword')" />
        <el-button class="account-submit" type="primary" @click="resetPassword">保存修改</el-button>
      </div>
    </section>
  </div>
</template>


<style scoped>
.set-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: var(--el-color-primary);
  color: #fff;
  font-size: 22px;
  font-weight: 600;
}
.el-col {
  margin-bottom: 10px
}
</style>